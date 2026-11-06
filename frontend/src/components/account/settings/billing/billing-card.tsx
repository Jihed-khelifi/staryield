"use client";

import * as React from "react";
import { CreditCard, Plus } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { billingApi, type PaymentMethod, type Plan, type Subscription } from "@/lib/api/billing";
import { useSession } from "@/lib/auth/session-provider";
import { useAsync } from "@/lib/use-async";
import { usePagedList } from "@/lib/use-paged-list";
import { ErrorState } from "@/components/feedback/error-state";
import { useToast } from "@/components/feedback/toast-provider";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { CursorPager } from "@/components/ui/pagination";
import { Skeleton, SkeletonList } from "@/components/ui/skeleton";

import { formatDay } from "../format";
import { SettingsCard } from "../settings-ui";
import { AddCardDialog, ChangeCardDialog, SubscribeDialog } from "./billing-dialogs";
import { TRANSACTION_KIND, TRANSACTION_STATUS, cardLabel, expiryLabel, planPrice, transactionAmount, usable } from "./billing-format";

type BillingData = { plans: Plan[]; subscription: Subscription | null; methods: PaymentMethod[] };

async function loadBilling(signal: AbortSignal): Promise<BillingData> {
  const [plans, subscription, methods] = await Promise.all([
    billingApi.plans(signal),
    billingApi.subscription(signal),
    billingApi.paymentMethods(signal),
  ]);
  return { plans, subscription, methods };
}

const OPEN: Subscription["status"][] = ["active", "past_due", "incomplete"];

function MembershipStatus({ subscription, timeZone }: { subscription: Subscription; timeZone?: string }) {
  const { text, locale } = useI18n();
  const day = (iso?: string) => formatDay(iso, locale, timeZone);
  const card = subscription.payment_method ? cardLabel(subscription.payment_method, text) : null;
  let line: string;
  let tone: "ok" | "warn" | "muted" = "ok";
  switch (subscription.status) {
    case "active":
      if (subscription.cancel_at_period_end) {
        line = text("Ends on {value0}. Your benefits last until then.", { value0: day(subscription.current_period_end) });
        tone = "warn";
      } else {
        line = card
          ? text("Renews on {value0} with {value1}.", { value0: day(subscription.renews_at ?? subscription.current_period_end), value1: card })
          : text("Renews on {value0}.", { value0: day(subscription.renews_at ?? subscription.current_period_end) });
      }
      break;
    case "past_due":
      line = subscription.next_retry_at
        ? text("Your last payment failed. We'll try again on {value0}; update your card to pay now.", { value0: day(subscription.next_retry_at) })
        : text("Your last payment failed. Update your card to keep your benefits.");
      tone = "warn";
      break;
    case "incomplete":
      line = text("Your first payment is being confirmed.");
      tone = "muted";
      break;
    case "canceled":
      line = text("Your membership ended on {value0}.", { value0: day(subscription.ended_at ?? subscription.canceled_at) });
      tone = "muted";
      break;
    default:
      line = text("Your membership has ended.");
      tone = "muted";
  }
  const status: Record<Subscription["status"], string> = {
    active: subscription.cancel_at_period_end ? "Canceling" : "Active",
    past_due: "Payment due",
    incomplete: "Pending",
    canceled: "Ended",
    expired: "Ended",
  };
  return (
    <p className="settings-membership-status" data-tone={tone} data-testid="membership-status">
      <span className="settings-status" data-status={subscription.status}>
        {text(status[subscription.status])}
      </span>{" "}
      {line}
    </p>
  );
}

function Membership({ data, timeZone, update }: { data: BillingData; timeZone?: string; update: (patch: Partial<BillingData>) => void }) {
  const { text, locale } = useI18n();
  const toast = useToast();
  const [dialog, setDialog] = React.useState<"subscribe" | "change" | "cancel" | null>(null);
  const [busy, setBusy] = React.useState(false);
  const sub = data.subscription;
  const open = sub !== null && OPEN.includes(sub.status);
  const plan = (open ? sub?.plan : null) ?? data.plans[0] ?? sub?.plan ?? null;
  const addMethod = React.useCallback(
    (method: PaymentMethod) =>
      update({ methods: [method, ...data.methods.filter((m) => m.id !== method.id)].map((m) => (method.is_default && m.id !== method.id ? { ...m, is_default: false } : m)) }),
    [data.methods, update]
  );

  if (!plan) return <p className="settings-muted-text">{text("Memberships are not available yet.")}</p>;

  async function cancelMembership() {
    setBusy(true);
    try {
      update({ subscription: await billingApi.cancel() });
      toast.show({ tone: "success", message: text("Your membership will not renew. You keep your benefits until the end of the period.") });
      setDialog(null);
    } catch (err) {
      toast.showError(err);
    } finally {
      setBusy(false);
    }
  }

  async function resumeMembership() {
    setBusy(true);
    try {
      update({ subscription: await billingApi.resume() });
      toast.show({ tone: "success", message: text("Your membership will renew as usual.") });
    } catch (err) {
      toast.showError(err);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="settings-billing-group" data-testid="membership">
      <p className="settings-label">{text("Active Plan")}</p>
      <div className="settings-row settings-plan-row">
        <span className="settings-row-label">{plan.name}</span>
        <span className="settings-price">{planPrice(plan, locale, text)}</span>
      </div>
      {sub ? <MembershipStatus subscription={sub} timeZone={timeZone} /> : <p className="settings-membership-status">{text("You're not a member yet.")}</p>}
      <div className="settings-inline-actions settings-inline-actions-start">
        {!open ? (
          <Button type="button" variant="cta" size="lg" className="px-4" onClick={() => setDialog("subscribe")} data-testid="membership-subscribe">
            {text("Subscribe")}
          </Button>
        ) : sub?.status === "active" && sub.cancel_at_period_end ? (
          <Button type="button" variant="cta" size="lg" className="px-4" onClick={() => void resumeMembership()} disabled={busy} aria-busy={busy} data-testid="membership-resume">
            {text(busy ? "Resuming…" : "Resume membership")}
          </Button>
        ) : (
          <>
            <Button type="button" className="settings-button" onClick={() => setDialog("change")} data-testid="membership-change-card">
              {text(sub?.status === "past_due" ? "Update card" : "Change card")}
            </Button>
            {sub?.status === "active" ? (
              <Button type="button" variant="ghost" size="lg" className="settings-text-action" onClick={() => setDialog("cancel")} data-testid="membership-cancel">
                {text("Cancel membership")}
              </Button>
            ) : null}
          </>
        )}
      </div>

      <SubscribeDialog
        open={dialog === "subscribe"}
        onOpenChange={(next) => setDialog(next ? "subscribe" : null)}
        plan={plan}
        methods={data.methods}
        onMethodAdded={addMethod}
        onSubscribed={(subscription) => update({ subscription })}
      />
      {sub && open ? (
        <ChangeCardDialog
          open={dialog === "change"}
          onOpenChange={(next) => setDialog(next ? "change" : null)}
          subscription={sub}
          methods={data.methods}
          onMethodAdded={addMethod}
          onChanged={(subscription) => update({ subscription })}
        />
      ) : null}
      <AlertDialog open={dialog === "cancel"} onOpenChange={(next) => (!next && !busy ? setDialog(null) : undefined)}>
        <AlertDialogContent data-testid="cancel-membership-dialog">
          <AlertDialogHeader>
            <AlertDialogTitle>{text("Cancel your membership?")}</AlertDialogTitle>
            <AlertDialogDescription>
              {text("You won't be charged again. Your benefits continue until {value0}, and you can resume before then.", {
                value0: formatDay(sub?.current_period_end, locale, timeZone),
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy}>{text("Keep membership")}</AlertDialogCancel>
            <AlertDialogAction
              destructive
              disabled={busy}
              aria-busy={busy}
              onClick={(event) => {
                event.preventDefault();
                void cancelMembership();
              }}
              data-testid="membership-cancel-confirm"
            >
              {text(busy ? "Canceling…" : "Cancel membership")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function PaymentMethods({ data, update }: { data: BillingData; update: (patch: Partial<BillingData>) => void }) {
  const { text } = useI18n();
  const toast = useToast();
  const [adding, setAdding] = React.useState(false);
  const [removing, setRemoving] = React.useState<PaymentMethod | null>(null);
  const [busy, setBusy] = React.useState<string | null>(null);
  const methods = data.methods;

  async function makeDefault(method: PaymentMethod) {
    setBusy(method.id);
    try {
      const saved = await billingApi.setDefault(method.id);
      update({ methods: methods.map((m) => (m.id === saved.id ? saved : { ...m, is_default: false })) });
      toast.show({ tone: "success", message: text("{value0} is now your default card.", { value0: cardLabel(saved, text) }) });
    } catch (err) {
      toast.showError(err);
    } finally {
      setBusy(null);
    }
  }

  async function remove() {
    if (!removing) return;
    setBusy(removing.id);
    try {
      await billingApi.removeMethod(removing.id);
      update({ methods: methods.filter((m) => m.id !== removing.id) });
      toast.show({ tone: "success", message: text("{value0} was removed.", { value0: cardLabel(removing, text) }) });
      setRemoving(null);
    } catch (err) {
      toast.showError(err);
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="settings-billing-group" data-testid="payment-methods">
      <p className="settings-label">{text("Payment Methods")}</p>
      {methods.length === 0 ? (
        <p className="settings-empty">{text("No saved cards yet.")}</p>
      ) : (
        <ul className="settings-list">
          {methods.map((method) => (
            <li key={method.id} className="settings-row settings-method" data-testid="payment-method">
              <span className="settings-row-main">
                <CreditCard aria-hidden size={20} absoluteStrokeWidth strokeWidth={2} className="shrink-0" />
                <span className="settings-row-label">{cardLabel(method, text)}</span>
                {method.is_default ? <span className="settings-badge">{text("Default")}</span> : null}
              </span>
              <span className="settings-method-side">
                <span className="settings-row-hint" data-tone={usable(method) ? undefined : "danger"}>
                  {method.status === "failed" ? text("Declined") : method.expired ? text("Expired") : expiryLabel(method, text)}
                </span>
                <span className="settings-method-actions">
                  {!method.is_default && usable(method) ? (
                    <button type="button" className="settings-link-button" onClick={() => void makeDefault(method)} disabled={busy !== null}>
                      {text("Make default")}
                      <span className="sr-only"> {cardLabel(method, text)}</span>
                    </button>
                  ) : null}
                  <button type="button" className="settings-link-button" onClick={() => setRemoving(method)} disabled={busy !== null}>
                    {text("Remove")}
                    <span className="sr-only"> {cardLabel(method, text)}</span>
                  </button>
                </span>
              </span>
            </li>
          ))}
        </ul>
      )}
      <div>
        <Button type="button" variant="ghost" size="lg" className="settings-text-action" onClick={() => setAdding(true)} data-testid="add-card">
          <Plus aria-hidden />
          {text("Add a card")}
        </Button>
      </div>
      <AddCardDialog
        open={adding}
        onOpenChange={setAdding}
        firstCard={methods.length === 0}
        onAdded={(method) =>
          update({
            methods: [method, ...methods.filter((m) => m.id !== method.id).map((m) => (method.is_default ? { ...m, is_default: false } : m))],
          })
        }
      />
      <AlertDialog open={removing !== null} onOpenChange={(next) => (!next && busy === null ? setRemoving(null) : undefined)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{text("Remove {value0}?", { value0: removing ? cardLabel(removing, text) : "" })}</AlertDialogTitle>
            <AlertDialogDescription>{text("You can add it again at any time.")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy !== null}>{text("Cancel")}</AlertDialogCancel>
            <AlertDialogAction
              destructive
              disabled={busy !== null}
              onClick={(event) => {
                event.preventDefault();
                void remove();
              }}
            >
              {text("Remove card")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function Transactions({ refreshKey, timeZone }: { refreshKey: number; timeZone?: string }) {
  const { text, locale } = useI18n();
  const { user } = useSession();
  const list = usePagedList((cursor, signal) => billingApi.transactions(cursor, 5, signal), [user?.id, refreshKey]);
  return (
    <div className="settings-billing-group" data-testid="transactions">
      <p className="settings-label">{text("Transaction History")}</p>
      {list.error ? (
        <ErrorState error={list.error} onRetry={list.reload} compact />
      ) : list.loading && list.items.length === 0 ? (
        <div aria-busy="true">
          <p role="status" className="sr-only">
            {text("Loading your payments…")}
          </p>
          <SkeletonList rows={1} rowClassName="h-11" />
        </div>
      ) : list.items.length === 0 ? (
        <p className="settings-empty">{text("No payments yet.")}</p>
      ) : (
        <ul className="settings-list">
          {list.items.map((t) => (
            <li key={t.id} className="settings-row settings-transaction" data-status={t.status} data-testid="transaction">
              <span className="grid gap-0.5">
                <span className="settings-row-label">{formatDay(t.created_at, locale, timeZone)}</span>
                <span className="settings-row-hint">{t.description || text(TRANSACTION_KIND[t.kind] ?? "Payment")}</span>
              </span>
              <span className="grid justify-items-end gap-0.5">
                <span className="settings-row-label tabular-nums">{transactionAmount(t, locale)}</span>
                {TRANSACTION_STATUS[t.status] ? (
                  <span className="settings-status" data-status={t.status}>
                    {text(TRANSACTION_STATUS[t.status])}
                  </span>
                ) : null}
              </span>
            </li>
          ))}
        </ul>
      )}
      <CursorPager
        page={list.page}
        hasPrev={list.hasPrev}
        hasNext={list.hasNext}
        onPrev={list.prev}
        onNext={list.next}
        loading={list.loading}
        label={text("Payment history pages")}
      />
    </div>
  );
}

/**
 * Billing & Subscriptions (Figma "Card / Billing"): the Premium Membership
 * (subscribe, cancel at period end, resume, change card), saved cards and
 * the payment history. Card entry goes through the provider's fields.
 */
export function BillingCard({ timeZone }: { timeZone?: string }) {
  const { text } = useI18n();
  const { user } = useSession();
  const billing = useAsync(loadBilling, [user?.id]);
  const [refreshKey, setRefreshKey] = React.useState(0);
  const setData = billing.setData;
  const update = React.useCallback(
    (patch: Partial<BillingData>) => {
      setData((current) => (current ? { ...current, ...patch } : current));
      if (patch.subscription !== undefined) setRefreshKey((n) => n + 1);
    },
    [setData]
  );

  return (
    <SettingsCard id="billing" icon="card" title={text("Billing & Subscriptions")} testId="settings-billing">
      {billing.error && billing.error.code === "feature_unavailable" ? (
        <p className="settings-muted-text">{text("Memberships are not available yet.")}</p>
      ) : billing.error ? (
        <ErrorState error={billing.error} onRetry={billing.reload} compact />
      ) : !billing.data ? (
        <div aria-busy="true" className="settings-stack">
          <p role="status" className="sr-only">
            {text("Loading your billing details…")}
          </p>
          <Skeleton className="h-4 w-24" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : (
        <div className="settings-stack">
          <Membership data={billing.data} timeZone={timeZone} update={update} />
          <PaymentMethods data={billing.data} update={update} />
          <Transactions refreshKey={refreshKey} timeZone={timeZone} />
        </div>
      )}
    </SettingsCard>
  );
}
