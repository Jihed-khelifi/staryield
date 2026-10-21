"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { newId } from "@/lib/api/client";
import { withdrawalsApi } from "@/lib/api/endpoints";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import type { Page, PsychicAnalytics, Withdrawal, WithdrawalMethod } from "@/lib/api/types";
import { formatDate } from "@/lib/format";
import { creditsValue, validateWithdrawal, type WithdrawalDraft, type WithdrawalErrors } from "@/lib/psychic-analytics";
import { useAsync } from "@/lib/use-async";
import { cn } from "@/lib/utils";
import { useToast } from "@/components/feedback/toast-provider";
import { Button } from "@/components/ui/button";
import { methodLabel, withdrawalStatusLabel } from "@/components/analytics/analytics-copy";

const EMPTY: WithdrawalDraft = { amount: "", method: "bank_transfer", destination: "" };

/** Map server field names onto the form's fields. */
function serverFieldErrors(error: ApiError | null): WithdrawalErrors {
  if (!error) return {};
  const f = error.fields;
  const out: WithdrawalErrors = {};
  if (f.amount_credits) out.amount = { message: f.amount_credits };
  if (f.method) out.method = { message: f.method };
  if (f.destination) out.destination = { message: f.destination };
  return out;
}

/** Available/reserved/paid balances, the payout request form and history (§8). */
export function Withdrawals({ payout, onChanged }: { payout: PsychicAnalytics["payout"] | null; onChanged: () => void }) {
  const { text, locale } = useI18n();
  const toast = useToast();
  const history = useAsync((signal) => withdrawalsApi.list(undefined, signal), []);
  const [more, setMore] = React.useState<Page<Withdrawal>[]>([]);
  const [updated, setUpdated] = React.useState<Record<string, Withdrawal>>({});
  const [created, setCreated] = React.useState<Withdrawal[]>([]);
  const [draft, setDraft] = React.useState<WithdrawalDraft>(EMPTY);
  const [errors, setErrors] = React.useState<WithdrawalErrors>({});
  const [apiError, setApiError] = React.useState<ApiError | null>(null);
  const [busy, setBusy] = React.useState(false);
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

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!payout) return;
    setApiError(null);
    const result = validateWithdrawal(draft, payout);
    setErrors(result.errors);
    if (!result.value) return;
    setBusy(true);
    try {
      const w = await withdrawalsApi.request({ ...result.value, idempotency_key: key.current });
      key.current = newId();
      setCreated((c) => [w, ...c]);
      setDraft(EMPTY);
      toast.show({ tone: "success", message: text("Withdrawal requested. We will email you once it is processed.") });
      onChanged();
    } catch (err) {
      const e2 = toApiError(err);
      setApiError(e2);
      setErrors(serverFieldErrors(e2));
    } finally {
      setBusy(false);
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

  const fieldError = (name: keyof WithdrawalDraft) =>
    errors[name] ? (
      <span id={`${ids}-${name}-error`} className="text-xs text-red-800" role="alert">
        {text(errors[name]!.message, formatValues(errors[name]!.values))}
      </span>
    ) : null;
  const formatValues = (values?: Record<string, string | number>) =>
    values && Object.fromEntries(Object.entries(values).map(([k, v]) => [k, typeof v === "number" ? number.format(v) : v]));
  const describedBy = (name: keyof WithdrawalDraft) => (errors[name] ? `${ids}-${name}-error` : undefined);

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
        <form className="withdrawal-form" onSubmit={submit} noValidate data-testid="withdrawal-form">
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
            <div className="field">
              <label htmlFor={`${ids}-amount`}>{text("Amount (credits)")}</label>
              <span className="flex gap-2">
                <input
                  id={`${ids}-amount`}
                  name="amount"
                  inputMode="numeric"
                  autoComplete="off"
                  value={draft.amount}
                  onChange={(e) => setDraft((d) => ({ ...d, amount: e.target.value }))}
                  aria-invalid={errors.amount ? true : undefined}
                  aria-describedby={describedBy("amount")}
                />
                <Button
                  type="button"
                  variant="outline"
                  className="h-auto border-gold bg-cream px-3"
                  onClick={() => payout && setDraft((d) => ({ ...d, amount: String(payout.available_credits) }))}
                >
                  {text("Max")}
                </Button>
              </span>
              {draft.amount && /^\d+$/.test(draft.amount.trim()) ? <span className="text-xs font-light">{money(Number(draft.amount))}</span> : null}
              {fieldError("amount")}
            </div>
            <fieldset className="grid gap-2" aria-describedby={describedBy("method")}>
              <legend className="mb-2 text-sm">{text("Payout method")}</legend>
              <div className="flex flex-wrap gap-2">
                {(Object.keys(methodLabel) as WithdrawalMethod[]).map((m) => (
                  <label key={m} className={cn("status-pill cursor-pointer", draft.method === m && "online")}>
                    <input
                      type="radio"
                      name="method"
                      value={m}
                      className="sr-only"
                      checked={draft.method === m}
                      onChange={() => setDraft((d) => ({ ...d, method: m }))}
                    />
                    {text(methodLabel[m])}
                  </label>
                ))}
              </div>
              {fieldError("method")}
            </fieldset>
            <div className="field">
              <label htmlFor={`${ids}-destination`}>{text(draft.method === "paypal" ? "PayPal email" : "IBAN or account number")}</label>
              <input
                id={`${ids}-destination`}
                name="destination"
                type={draft.method === "paypal" ? "email" : "text"}
                autoComplete="off"
                maxLength={200}
                value={draft.destination}
                onChange={(e) => setDraft((d) => ({ ...d, destination: e.target.value }))}
                aria-invalid={errors.destination ? true : undefined}
                aria-describedby={describedBy("destination")}
              />
              <span className="text-xs font-light">{text("Stored encrypted. Only the last characters are shown afterwards.")}</span>
              {fieldError("destination")}
            </div>
            {apiError && Object.keys(serverFieldErrors(apiError)).length === 0 ? (
              <p role="alert" className="text-sm text-red-800">
                {text(errorMessage(apiError))}
              </p>
            ) : null}
            <Button type="submit" className="gold-button h-11" aria-busy={busy}>
              {text(busy ? "Sending…" : "Request withdrawal")}
            </Button>
          </fieldset>
        </form>

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
