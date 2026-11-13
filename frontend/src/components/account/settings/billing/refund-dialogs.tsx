"use client";

import * as React from "react";
import Link from "next/link";
import { AlertCircle, CheckCircle2, Mail } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { newId } from "@/lib/api/client";
import type { Transaction } from "@/lib/api/billing";
import { errorMessage, toApiError } from "@/lib/api/errors";
import { refundsApi, type RefundQuote, type RefundReason, type RefundRequest } from "@/lib/api/pricing";
import {
  REFUND_REASONS,
  REFUND_REASON_COPY,
  classifyRefundFailure,
  refundMailto,
  refundSchema,
  REFUND_DETAILS_MAX,
  REFUND_SUPPORT_EMAIL,
  type RefundFailure,
  type RefundFormValues,
} from "@/lib/billing/refund";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { useAsync } from "@/lib/use-async";
import { announce } from "@/components/feedback/live-region";
import { ErrorState } from "@/components/feedback/error-state";
import { useToast } from "@/components/feedback/toast-provider";
import { LinkedCopy } from "@/components/billing/linked-copy";
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
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { FieldRow } from "@/components/ui/field";
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage, FormRootError } from "@/components/ui/form";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";

import { formatDay, formatMoney } from "../format";

function QuoteSkeleton() {
  const { text } = useI18n();
  return (
    <div aria-busy="true" className="grid gap-3" data-testid="refund-quote-loading">
      <p role="status" className="sr-only">
        {text("Checking what can be refunded…")}
      </p>
      <Skeleton className="h-5 w-2/3" />
      <Skeleton className="h-24 w-full" />
      <Skeleton className="h-10 w-full" />
    </div>
  );
}

/** What the payment bought, what is left, and what could come back (an estimate, confirmed on review). */
function QuoteSummary({ quote, timeZone }: { quote: RefundQuote; timeZone?: string }) {
  const { text, locale } = useI18n();
  const rows: [string, string][] = [
    [text("Credits purchased"), quote.credits_purchased.toLocaleString(locale)],
    [text("Credits used"), quote.credits_used.toLocaleString(locale)],
    [text("Credits remaining"), quote.credits_remaining.toLocaleString(locale)],
    [text("Refundable estimate"), text("up to {value0}", { value0: formatMoney(quote.refundable_cents, quote.currency, locale) })],
    [text("Refund window ends"), formatDay(quote.window_ends_at, locale, timeZone)],
  ];
  return (
    <div className="settings-refund-quote" data-testid="refund-quote">
      <p className="settings-row-label">
        {text("{value0} · {value1}", { value0: quote.pack_name, value1: formatMoney(quote.amount_paid_cents, quote.currency, locale) })}
      </p>
      <dl>
        {rows.map(([label, value]) => (
          <div key={label}>
            <dt>{label}</dt>
            <dd>{value}</dd>
          </div>
        ))}
      </dl>
      <p className="settings-refund-hint">{text("The estimate covers your unused credits. We confirm the amount when we review your request.")}</p>
      {!quote.within_window ? (
        <p role="note" className="settings-refund-note" data-testid="refund-outside-window">
          {text("The refund window for unused credits has ended. You can still tell us about a problem with this payment.")}
        </p>
      ) : null}
    </div>
  );
}

/** The e-mail channel, for when in-app requests are off or a request can't be sent. */
function EmailChannel() {
  const { text } = useI18n();
  return (
    <div className="settings-dialog-success" data-testid="refund-email">
      <p>{text("Refund requests can't be sent from here right now. Write to us and we will take care of it.")}</p>
      <Button asChild variant="cta" size="xl">
        <a href={refundMailto()}>
          <Mail aria-hidden />
          {text("E-mail {value0}", { value0: REFUND_SUPPORT_EMAIL })}
        </a>
      </Button>
      <p className="settings-refund-hint">{text('Use the subject "Refund Request" and tell us which payment it is about.')}</p>
    </div>
  );
}

function RefundSent({ request, onClose }: { request: RefundRequest; onClose: () => void }) {
  const { text } = useI18n();
  const ref = React.useRef<HTMLDivElement>(null);
  React.useEffect(() => {
    ref.current?.focus();
    announce(text("Your refund request was sent."));
  }, [text]);
  return (
    <div ref={ref} tabIndex={-1} role="status" className="grid gap-3 outline-none" data-testid="refund-success">
      <div className="flex items-start gap-3">
        <CheckCircle2 aria-hidden className="mt-0.5 size-6 shrink-0 text-success" />
        <div className="grid gap-1">
          <p className="font-semibold">{text("Your refund request was sent.")}</p>
          <p className="text-sm">{text("We aim to acknowledge requests within 2 business days.")}</p>
        </div>
      </div>
      <p className="rounded-lg border border-gold/50 bg-white px-4 py-3 text-center">
        <span className="block text-xs text-ink/75">{text("Reference")}</span>
        <span className="font-display text-xl tracking-wider [overflow-wrap:anywhere]" data-testid="refund-reference">
          {request.reference}
        </span>
      </p>
      <div className="flex justify-end">
        <Button type="button" variant="cta" size="xl" onClick={onClose} data-testid="refund-done">
          {text("Done")}
        </Button>
      </div>
    </div>
  );
}

function RefundForm({
  quote,
  transactionId,
  onSent,
  onClose,
  onStale,
}: {
  quote: RefundQuote;
  transactionId: string;
  onSent: (request: RefundRequest) => void;
  onClose: () => void;
  onStale: () => void;
}) {
  const { text, locale } = useI18n();
  const form = useSchemaForm<ReturnType<typeof refundSchema>, RefundFormValues>(refundSchema, { defaultValues: { details: "" } });
  // One key per attempt: a retry after an unknown outcome replays the same request.
  const attempt = React.useRef(newId());
  const [emailOnly, setEmailOnly] = React.useState(false);
  const busy = form.formState.isSubmitting;
  const serverError = form.formState.errors.root?.server;

  async function submit(values: RefundFormValues) {
    try {
      const details = values.details?.trim();
      const request = await refundsApi.create({ transaction_id: transactionId, reason: values.reason, ...(details ? { details } : {}) }, attempt.current);
      onSent(request);
    } catch (err) {
      const e = toApiError(err);
      if (e.code === "aborted") return;
      if (e.code === "validation_failed") {
        let placed = false;
        for (const [field, message] of Object.entries(e.fields)) {
          if ((field === "reason" || field === "details") && typeof message === "string" && message) {
            form.setError(field, { type: "server", message });
            placed = true;
          }
        }
        if (placed) return;
      }
      const failure: RefundFailure = classifyRefundFailure(e);
      if (failure.kind === "email") {
        setEmailOnly(true);
        return;
      }
      // Anything but an unknown outcome is final for this key.
      if (!(failure.kind === "error" && failure.retryable)) attempt.current = newId();
      if (failure.kind === "exists") {
        // The row now shows the existing request: reload it and say so.
        onStale();
        form.setError("root.server", { type: "exists", message: text("You already have an open refund request for this payment. You can follow it in your payment history.") });
        return;
      }
      if (failure.kind === "not_eligible") {
        if (failure.reason !== "outside_window") onStale();
        form.setError("root.server", { type: "not_eligible", message: text(failure.message) });
        return;
      }
      form.setError("root.server", {
        type: failure.kind === "rate_limited" ? "rate_limited" : failure.retryable ? "retry" : "error",
        message: text(failure.message),
      });
    }
  }

  if (emailOnly) return <EmailChannel />;

  return (
    <Form {...form}>
      <form onSubmit={(event) => {
        event.stopPropagation();
        void form.handleSubmit(submit)(event);
      }} noValidate className="settings-dialog-form" data-testid="refund-form" aria-busy={busy || undefined}>
        <FormField
          control={form.control}
          name="reason"
          render={({ field }) => (
            <FormItem>
              <FormLabel>{text("Why are you asking for a refund?")}</FormLabel>
              <Select value={field.value ?? ""} onValueChange={field.onChange} disabled={busy}>
                <FormControl>
                  <SelectTrigger ref={field.ref} className="h-11 data-placeholder:text-[#595959]" data-testid="refund-reason">
                    <SelectValue placeholder={text("Choose a reason")} />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  {REFUND_REASONS.map((value: RefundReason) => (
                    <SelectItem key={value} value={value} disabled={value === "unused_credits" && !quote.within_window} data-testid={`refund-reason-${value}`}>
                      {text(REFUND_REASON_COPY[value])}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )}
        />
        <FieldRow control={form.control} name="details" label={text("More details (optional)")} maxLength={REFUND_DETAILS_MAX} showValid={false}>
          {(field) => (
            <Textarea
              {...field}
              rows={4}
              maxLength={REFUND_DETAILS_MAX + 200}
              disabled={busy}
              placeholder={text("Tell us what happened. Please do not include passwords or card numbers.")}
              className="min-h-24 resize-y bg-white"
              autoComplete="off"
              data-testid="refund-details"
            />
          )}
        </FieldRow>
        <p className="settings-refund-hint">
          <LinkedCopy
            copy={text("Requests are reviewed under our [Refund Policy]. A decision can take a few days.")}
            links={[
              (label) => (
                <Link href={`/${locale}/refund-policy`} className="underline" target="_blank" rel="noopener noreferrer" data-testid="refund-policy-link">
                  {label}
                  <span className="sr-only"> {text("(opens in a new tab)")}</span>
                </Link>
              ),
            ]}
          />
        </p>
        {serverError ? (
          <FormRootError>
            {serverError.type === "retry" ? (
              <Button type="submit" variant="outline" size="lg" className="h-11" disabled={busy} data-testid="refund-retry">
                {text("Try again")}
              </Button>
            ) : null}
            {serverError.type === "rate_limited" || serverError.type === "not_eligible" ? (
              <Button asChild variant="outline" size="lg" className="h-11">
                <a href={refundMailto()}>{text("E-mail {value0}", { value0: REFUND_SUPPORT_EMAIL })}</a>
              </Button>
            ) : null}
          </FormRootError>
        ) : null}
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button type="button" variant="outline" size="xl" className="border-ink/40" onClick={onClose} disabled={busy}>
            {text("Cancel")}
          </Button>
          <Button type="submit" variant="cta" size="xl" disabled={busy} aria-busy={busy} onMouseDown={keepFocusOnPress} data-testid="refund-submit">
            {text(busy ? "Sending…" : "Send request")}
          </Button>
        </div>
      </form>
    </Form>
  );
}

/**
 * "Request a refund" (§24.4): the quote (credits used and left, the refundable
 * estimate, when the window ends), a reason, optional details and the policy
 * link. Mount it only while open so every opening fetches a fresh quote.
 */
export function RefundRequestDialog({
  transaction,
  timeZone,
  onClose,
  onRequested,
  onStale,
}: {
  transaction: Pick<Transaction, "id" | "description">;
  timeZone?: string;
  onClose: () => void;
  /** A request was created: the row shows it. */
  onRequested: (request: RefundRequest) => void;
  /** The history is out of date (an open request or a refund exists): reload it. */
  onStale: () => void;
}) {
  const { text } = useI18n();
  const quote = useAsync((signal) => refundsApi.quote(transaction.id, signal), [transaction.id]);
  const [sent, setSent] = React.useState<RefundRequest | null>(null);
  const emailOnly = quote.error?.code === "feature_unavailable";

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent size="md" className="settings-dialog" data-testid="refund-dialog">
        <DialogHeader>
          <DialogTitle>{text("Request a refund")}</DialogTitle>
          <DialogDescription>{text("Tell us why, and we will review this payment.")}</DialogDescription>
        </DialogHeader>
        {sent ? (
          <RefundSent request={sent} onClose={onClose} />
        ) : emailOnly ? (
          <EmailChannel />
        ) : quote.error ? (
          <ErrorState error={quote.error} onRetry={quote.reload} compact />
        ) : !quote.data ? (
          <QuoteSkeleton />
        ) : (
          <>
            <QuoteSummary quote={quote.data} timeZone={timeZone} />
            <RefundForm
              quote={quote.data}
              transactionId={transaction.id}
              onClose={onClose}
              onStale={onStale}
              onSent={(request) => {
                setSent(request);
                onRequested(request);
              }}
            />
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

/** Confirm withdrawing an open request. */
export function WithdrawRefundDialog({
  request,
  onClose,
  onChanged,
}: {
  request: { id: string; reference: string };
  onClose: () => void;
  /** The request left the open state (withdrawn, or already decided): reload the history. */
  onChanged: () => void;
}) {
  const { text } = useI18n();
  const toast = useToast();
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  async function withdraw() {
    setBusy(true);
    setError(null);
    try {
      await refundsApi.withdraw(request.id);
      toast.show({ tone: "success", message: text("Your refund request {value0} was withdrawn.", { value0: request.reference }) });
      onChanged();
      onClose();
    } catch (err) {
      const e = toApiError(err);
      if (e.code === "invalid_state") {
        // It was already decided (or withdrawn elsewhere): the history will say so.
        toast.show({ tone: "info", message: text("This request can no longer be withdrawn.") });
        onChanged();
        onClose();
        return;
      }
      setError(text(errorMessage(e)));
    } finally {
      setBusy(false);
    }
  }

  return (
    <AlertDialog open onOpenChange={(open) => (!open && !busy ? onClose() : undefined)}>
      <AlertDialogContent data-testid="refund-withdraw-dialog">
        <AlertDialogHeader>
          <AlertDialogTitle>{text("Withdraw this refund request?")}</AlertDialogTitle>
          <AlertDialogDescription>
            {text("Request {value0} will be closed. You can send a new one later if you still need it.", { value0: request.reference })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        {error ? (
          <p role="alert" className="flex items-start gap-2 text-sm text-danger">
            <AlertCircle aria-hidden className="mt-0.5 size-4 shrink-0" />
            {error}
          </p>
        ) : null}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={busy}>{text("Keep my request")}</AlertDialogCancel>
          <AlertDialogAction
            destructive
            disabled={busy}
            aria-busy={busy}
            onClick={(event) => {
              event.preventDefault();
              void withdraw();
            }}
            data-testid="refund-withdraw-confirm"
          >
            {text(busy ? "Withdrawing…" : "Withdraw request")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
