"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { useWatch } from "react-hook-form";

import { newId } from "@/lib/api/client";
import { withdrawalsApi } from "@/lib/api/endpoints";
import { errorMessage } from "@/lib/api/errors";
import type { Page, PsychicAnalytics, Withdrawal, WithdrawalMethod } from "@/lib/api/types";
import { formatDate } from "@/lib/format";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { creditsValue } from "@/lib/psychic-analytics";
import {
  emptyWithdrawal,
  toWithdrawalRequest,
  withdrawalSchema,
  type WithdrawalContext,
  type WithdrawalValues,
} from "@/lib/schemas/withdrawal";
import { useAsync } from "@/lib/use-async";
import { cn } from "@/lib/utils";
import { useToast } from "@/components/feedback/toast-provider";
import { Button } from "@/components/ui/button";
import { FieldRow } from "@/components/ui/field";
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage, FormRootError } from "@/components/ui/form";
import { methodLabel, withdrawalStatusLabel } from "@/components/analytics/analytics-copy";

/** Available/reserved/paid balances, the payout request form and history (§8). */
export function Withdrawals({ payout, onChanged }: { payout: PsychicAnalytics["payout"] | null; onChanged: () => void }) {
  const { text, locale } = useI18n();
  const toast = useToast();
  const history = useAsync((signal) => withdrawalsApi.list(undefined, signal), []);
  const [more, setMore] = React.useState<Page<Withdrawal>[]>([]);
  const [updated, setUpdated] = React.useState<Record<string, Withdrawal>>({});
  const [created, setCreated] = React.useState<Withdrawal[]>([]);
  // The payout limits reach the schema through the resolver context
  // (react-hook-form reads the latest options on every validation).
  const context = React.useMemo<WithdrawalContext>(
    () => ({ payout, format: (n: number) => new Intl.NumberFormat(locale).format(n) }),
    [payout, locale],
  );
  const form = useSchemaForm(withdrawalSchema, { defaultValues: emptyWithdrawal, context });
  const method = useWatch({ control: form.control, name: "method" });
  const amount = useWatch({ control: form.control, name: "amount" });
  const busy = form.formState.isSubmitting;
  const [confirming, setConfirming] = React.useState<string | null>(null);
  const [cancelling, setCancelling] = React.useState<string | null>(null);
  // One idempotency key per request form: retries and double submits never pay twice.
  const key = React.useRef(newId());
  const ids = React.useId();

  const listed = [...created, ...(history.data?.items ?? []), ...more.flatMap((p) => p.items)].filter(
    (w, i, all) => all.findIndex((x) => x.id === w.id) === i,
  );
  const items = listed.map((w) => updated[w.id] ?? w);
  const cursor = more.length ? more.at(-1)!.next_cursor : history.data?.next_cursor;
  const open = items.find((w) => w.status === "pending" || w.status === "approved");
  const money = (credits: number) => (payout ? creditsValue(credits, payout.cents_per_credit, payout.currency, locale) : "");
  const number = new Intl.NumberFormat(locale);

  async function submit(values: WithdrawalValues) {
    if (!payout) return;
    try {
      const w = await withdrawalsApi.request({ ...toWithdrawalRequest(values), idempotency_key: key.current });
      key.current = newId();
      setCreated((c) => [w, ...c]);
      form.reset(emptyWithdrawal);
      toast.show({ tone: "success", message: text("Withdrawal requested. We will email you once it is processed.") });
      onChanged();
    } catch (err) {
      applyApiError(form, err, { text, fieldMap: { amount_credits: "amount", method: "method", destination: "destination" } });
    }
  }

  async function cancel(w: Withdrawal) {
    setCancelling(w.id);
    try {
      const next = await withdrawalsApi.cancel(w.id);
      setUpdated((u) => ({ ...u, [w.id]: next }));
      setConfirming(null);
      onChanged();
    } catch (err) {
      toast.showError(err);
    } finally {
      setCancelling(null);
    }
  }

  async function loadMore() {
    if (!cursor) return;
    try {
      const page = await withdrawalsApi.list(cursor);
      setMore((m) => [...m, page]);
    } catch (err) {
      toast.showError(err);
    }
  }

  return (
    <section className="analytics-card" aria-labelledby={`${ids}-heading`} data-testid="withdrawals">
      <h2 id={`${ids}-heading`} className="mb-4 text-xl font-semibold">
        {text("Payouts")}
      </h2>
      {payout ? (
        <div className="analytics-kpis analytics-kpis-compact">
          {(
            [
              ["Available", payout.available_credits, "available"],
              ["Reserved", payout.reserved_credits, "reserved"],
              ["Paid out", payout.paid_credits, "paid"],
              ["Lifetime earnings", payout.lifetime_net_credits, "lifetime"],
            ] as const
          ).map(([label, credits, id]) => (
            <div key={id} className="stat-tile analytics-kpi" data-testid={`payout-${id}`}>
              <p className="text-[10px] uppercase">{text(label)}</p>
              <p className="mt-2 text-2xl">{number.format(credits)}</p>
              <span className="text-xs">{text("Worth {value0}", { value0: money(credits) })}</span>
            </div>
          ))}
        </div>
      ) : null}

      <div className="withdrawal-layout">
        <Form {...form}>
          <form className="withdrawal-form" onSubmit={(event) => void form.handleSubmit(submit)(event)} noValidate data-testid="withdrawal-form">
            <h3 className="text-base font-semibold">{text("Request a withdrawal")}</h3>
            {payout ? (
              <p className="text-xs font-light">
                {text("Minimum {value0} credits ({value1}). One withdrawal can be in progress at a time.", {
                  value0: number.format(payout.min_withdrawal_credits),
                  value1: money(payout.min_withdrawal_credits),
                })}
              </p>
            ) : null}
            {open ? (
              <p className="rounded-md bg-cream/70 p-3 text-sm" role="status">
                {text("You already have a withdrawal in progress. You can request another once it is processed.")}
              </p>
            ) : null}
            <fieldset disabled={busy || Boolean(open) || !payout} className="grid gap-4">
              <FormField
                control={form.control}
                name="amount"
                render={({ field }) => (
                  <FormItem showValid>
                    <FormLabel>{text("Amount (credits)")}</FormLabel>
                    <span className="flex gap-2">
                      <FormControl>
                        <input {...field} inputMode="numeric" autoComplete="off" className="min-w-0 flex-1" />
                      </FormControl>
                      <Button
                        type="button"
                        variant="outline"
                        className="h-auto border-gold bg-cream px-3"
                        onClick={() =>
                          payout && form.setValue("amount", String(payout.available_credits), { shouldValidate: true, shouldDirty: true })
                        }
                      >
                        {text("Max")}
                      </Button>
                    </span>
                    {amount && /^\d+$/.test(amount.trim()) ? <span className="text-xs font-light">{money(Number(amount))}</span> : null}
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="method"
                render={({ field }) => (
                  <FormItem>
                    <fieldset className="grid gap-2">
                      <legend className="mb-2 text-sm">{text("Payout method")}</legend>
                      <div className="flex flex-wrap gap-2">
                        {(Object.keys(methodLabel) as WithdrawalMethod[]).map((m) => (
                          <label key={m} className={cn("status-pill cursor-pointer", field.value === m && "online")}>
                            <input
                              type="radio"
                              name={field.name}
                              value={m}
                              className="sr-only"
                              checked={field.value === m}
                              onBlur={field.onBlur}
                              onChange={() => {
                                field.onChange(m);
                                if (form.getFieldState("destination").isTouched) void form.trigger("destination");
                              }}
                            />
                            {text(methodLabel[m])}
                          </label>
                        ))}
                      </div>
                    </fieldset>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FieldRow
                control={form.control}
                name="destination"
                label={text(method === "paypal" ? "PayPal email" : "IBAN or account number")}
                description={text("Stored encrypted. Only the last characters are shown afterwards.")}
              >
                {(field) => <input {...field} type={method === "paypal" ? "email" : "text"} autoComplete="off" maxLength={200} />}
              </FieldRow>
              <FormRootError />
              <Button type="submit" className="gold-button h-11" aria-busy={busy} onMouseDown={keepFocusOnPress}>
                {text(busy ? "Sending…" : "Request withdrawal")}
              </Button>
            </fieldset>
          </form>
        </Form>

        <div className="min-w-0">
          <h3 className="mb-3 text-base font-semibold">{text("History")}</h3>
          {history.error ? (
            <p role="alert" className="text-sm">
              {text(errorMessage(history.error))}{" "}
              <button type="button" className="underline" onClick={history.reload}>
                {text("Try again")}
              </button>
            </p>
          ) : history.loading && !history.data ? (
            <p className="text-sm" role="status">
              {text("Loading…")}
            </p>
          ) : items.length === 0 ? (
            <p className="text-sm font-light">{text("No withdrawals yet.")}</p>
          ) : (
            <ul className="space-y-3" data-testid="withdrawal-history">
              {items.map((w) => (
                <li key={w.id} className="withdrawal-item">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <strong>
                      {text("{value0} credits", { value0: number.format(w.amount_credits) })} ·{" "}
                      {new Intl.NumberFormat(locale, { style: "currency", currency: w.currency || "USD" }).format(w.amount_cents / 100)}
                    </strong>
                    <span className={cn("status-pill", w.status === "paid" && "online")} data-status={w.status}>
                      {text(withdrawalStatusLabel[w.status] ?? w.status)}
                    </span>
                  </div>
                  <p className="mt-1 text-xs font-light">
                    {text(methodLabel[w.method] ?? w.method)} · {w.destination_hint} · {formatDate(w.requested_at, locale)}
                    {w.processed_at ? ` · ${text("Processed {value0}", { value0: formatDate(w.processed_at, locale) })}` : ""}
                  </p>
                  {w.note ? <p className="mt-1 text-xs whitespace-pre-wrap">{w.note}</p> : null}
                  {w.status === "pending" ? (
                    confirming === w.id ? (
                      <div className="mt-2 flex flex-wrap items-center gap-2 text-xs">
                        <span>{text("Cancel this withdrawal?")}</span>
                        <Button size="sm" variant="outline" className="border-gold bg-cream" disabled={cancelling === w.id} onClick={() => void cancel(w)}>
                          {text("Yes, cancel it")}
                        </Button>
                        <Button size="sm" variant="ghost" onClick={() => setConfirming(null)}>
                          {text("Keep it")}
                        </Button>
                      </div>
                    ) : (
                      <button type="button" className="mt-2 text-xs underline" onClick={() => setConfirming(w.id)}>
                        {text("Cancel withdrawal")}
                      </button>
                    )
                  ) : null}
                </li>
              ))}
            </ul>
          )}
          {cursor ? (
            <button type="button" className="mt-3 text-xs underline" onClick={() => void loadMore()}>
              {text("Load more")}
            </button>
          ) : null}
        </div>
      </div>
    </section>
  );
}
