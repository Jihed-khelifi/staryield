"use client";

/**
 * "Report an issue" with the conversation on screen (clients only, §16.4).
 *
 * The chatroom places <ReportIssueButton>; the dialog is self-contained: it
 * checks for an open report first, then asks for a reason and optional
 * details, and ends in a confirmation or an "already reported" state. The
 * live chat is never touched (no navigation, no session change).
 */
import * as React from "react";
import Link from "next/link";
import { AlertCircle, CheckCircle2, Flag, ShieldCheck } from "lucide-react";
import { FiFlag } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import { supportApi } from "@/lib/api/endpoints";
import { errorMessage, toApiError } from "@/lib/api/errors";
import { track } from "@/lib/analytics";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import {
  CHAT_REPORT_DETAILS_MAX,
  CHAT_REPORT_REASONS,
  chatReportReasonLabels,
  chatReportSchema,
  isChatReportReason,
  type ChatReportValues,
} from "@/lib/schemas/chat-report";
import { classifyReportError, existingReference, type ReportFailure } from "@/lib/support/chat-report";
import { waitHint } from "@/lib/support/support";
import { useAsync } from "@/lib/use-async";
import { cn } from "@/lib/utils";
import { announce } from "@/components/feedback/live-region";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { FieldRow } from "@/components/ui/field";
import { Form, FormControl, FormField, FormItem, FormMessage } from "@/components/ui/form";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

export type ReportIssueTarget = {
  conversationId: string;
  /** The session on screen, or the one that just ended, when known. */
  sessionId?: string;
  /** Shown in the dialog ("your conversation with …"). */
  advisorName: string;
  /** Newest message seq visible in the feed. */
  seq?: number;
};

export type ReportIssueButtonProps = {
  target: ReportIssueTarget;
  /**
   * "header": small ghost icon button in the chat header (tablet and up).
   * "tools": compact icon button in the phone session-tools row.
   */
  placement: "header" | "tools";
  className?: string;
};

/** What the dialog shows once the person has an answer: a sent report or an earlier one. */
type Outcome = { kind: "sent" | "exists"; reference: string | null };

function DialogSkeleton() {
  const { text } = useI18n();
  return (
    <div aria-busy="true" data-testid="report-checking" className="grid gap-3">
      <p role="status" className="sr-only">
        {text("Loading…")}
      </p>
      <Skeleton className="h-5 w-2/3" />
      <Skeleton className="h-10 w-full" />
      <Skeleton className="h-10 w-full" />
      <Skeleton className="h-10 w-full" />
    </div>
  );
}

/** Reference, "View report" and "Back to chat": shared by the sent and the already-reported states. */
function OutcomeActions({ reference, onClose }: { reference: string | null; onClose: () => void }) {
  const { text, locale } = useI18n();
  return (
    <div className="mt-1 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
      <Button type="button" variant="outline" className="h-11 border-gold bg-cream sm:h-10" onClick={onClose} data-testid="report-back">
        {text("Back to chat")}
      </Button>
      <Button asChild className="gold-button h-11 sm:h-10">
        <Link href={reference ? `/${locale}/support/requests/${reference}` : `/${locale}/support/requests`} data-testid="report-view">
          {text("View report")}
        </Link>
      </Button>
    </div>
  );
}

function OutcomeView({ outcome, onClose }: { outcome: Outcome; onClose: () => void }) {
  const { text } = useI18n();
  const ref = React.useRef<HTMLDivElement>(null);
  const sent = outcome.kind === "sent";
  React.useEffect(() => {
    ref.current?.focus();
    announce(text(sent ? "Thanks — your report was sent." : "You already reported this conversation"));
  }, [sent, text]);
  return (
    <div ref={ref} tabIndex={-1} role="status" data-testid={sent ? "report-success" : "report-exists"} className="grid gap-3 outline-none">
      <div className="flex items-start gap-3">
        {sent ? <CheckCircle2 aria-hidden className="mt-0.5 size-6 shrink-0 text-success" /> : <Flag aria-hidden className="mt-0.5 size-6 shrink-0 text-amber" />}
        <div className="grid gap-1">
          <p className="font-semibold">{text(sent ? "Thanks — your report was sent." : "You already reported this conversation")}</p>
          <p className="text-sm">{text(sent ? "We'll reply by e-mail and in Support requests." : "Our team is looking into it. You can follow it and add details in Support requests.")}</p>
        </div>
      </div>
      {outcome.reference ? (
        <p className="rounded-lg border border-gold/50 bg-white px-4 py-3 text-center">
          <span className="block text-xs text-ink/75">{text("Reference")}</span>
          <span className="font-display text-xl tracking-wider [overflow-wrap:anywhere]" data-testid="report-reference">
            {outcome.reference}
          </span>
        </p>
      ) : null}
      <OutcomeActions reference={outcome.reference} onClose={onClose} />
    </div>
  );
}

/** A failed submit: localized message with the call to action that fits it. */
function FailureNotice({ failure, onRetry, onClose, busy }: { failure: ReportFailure; onRetry: () => void; onClose: () => void; busy: boolean }) {
  const { text, locale } = useI18n();
  let message: string;
  let cta: React.ReactNode;
  const retry = (
    <Button type="button" size="sm" variant="outline" className="h-9 border-danger/50 bg-white" onClick={onRetry} disabled={busy} data-testid="report-retry">
      {text(busy ? "Sending…" : "Retry")}
    </Button>
  );
  const close = (
    <Button type="button" size="sm" variant="outline" className="h-9 border-danger/50 bg-white" onClick={onClose}>
      {text("Back to chat")}
    </Button>
  );
  switch (failure.kind) {
    case "rate_limited": {
      const hint = waitHint(failure.retryAfter);
      message =
        hint.unit === "seconds"
          ? text("You have sent several reports recently. Try again in {value0} seconds.", { value0: hint.value })
          : hint.unit === "minutes"
            ? text("You have sent several reports recently. Try again in about {value0} minutes.", { value0: hint.value })
            : text("You have sent several reports recently. Please wait a little before trying again.");
      cta = close;
      break;
    }
    case "sign_in": {
      message = text("Please sign in to continue.");
      const next = typeof window === "undefined" ? `/${locale}/chatroom` : `${window.location.pathname}${window.location.search}`;
      cta = (
        <Button asChild size="sm" className="gold-button h-9">
          <Link href={`/${locale}/login?next=${encodeURIComponent(next)}`}>{text("Sign in")}</Link>
        </Button>
      );
      break;
    }
    case "forbidden":
      message = text("Only clients can report a conversation.");
      cta = close;
      break;
    case "not_found":
      message = text("This conversation is no longer available.");
      cta = close;
      break;
    case "retry":
      message = text("We could not send your report. Check your connection and try again.");
      cta = retry;
      break;
    default:
      message = text("Something went wrong on our side. Please try again.");
      cta = (
        <>
          {retry}
          <Button asChild size="sm" variant="outline" className="h-9 border-danger/50 bg-white">
            <Link href={`/${locale}/contact`}>{text("Contact support")}</Link>
          </Button>
        </>
      );
  }
  return (
    <div role="alert" data-testid="report-error" data-kind={failure.kind} className="flex items-start gap-2 rounded-lg border border-danger/40 bg-danger/5 p-3 text-sm text-danger">
      <AlertCircle aria-hidden className="mt-0.5 size-4 shrink-0" />
      <div className="grid gap-2">
        <p>{message}</p>
        <div className="flex flex-wrap gap-2">{cta}</div>
      </div>
    </div>
  );
}

function ReportForm({ target, onOutcome, onClose }: { target: ReportIssueTarget; onOutcome: (outcome: Outcome) => void; onClose: () => void }) {
  const { text } = useI18n();
  const form = useSchemaForm<ReturnType<typeof chatReportSchema>, ChatReportValues>(chatReportSchema, { defaultValues: { reason: undefined, details: "" } });
  const [failure, setFailure] = React.useState<ReportFailure | null>(null);
  const inflight = React.useRef(false);
  const reasonId = React.useId();
  const reason = form.watch("reason");
  const busy = form.formState.isSubmitting;

  async function submit(values: ChatReportValues) {
    // A second press while the first request is still running is ignored.
    if (inflight.current) return;
    inflight.current = true;
    setFailure(null);
    try {
      const details = values.details?.trim();
      const ticket = await supportApi.createChatReport({
        conversation_id: target.conversationId,
        ...(target.sessionId ? { session_id: target.sessionId } : {}),
        reason: values.reason,
        ...(details ? { details } : {}),
        ...(target.seq !== undefined ? { seq: target.seq } : {}),
      });
      // Only the reason is recorded: never the free text.
      track("chat_report_submitted", { reason: values.reason });
      onOutcome({ kind: "sent", reference: ticket.reference });
    } catch (err) {
      const e = toApiError(err);
      const result = classifyReportError(e);
      if (result.kind === "exists") {
        onOutcome({ kind: "exists", reference: result.reference ?? null });
      } else if (result.kind === "fields") {
        // Field messages are already localized by the API; anything else is a generic notice.
        const known: string[] = [];
        for (const [field, message] of Object.entries(e.fields)) {
          if ((field === "reason" || field === "details") && typeof message === "string" && message) {
            form.setError(field, { type: "server", message });
            known.push(field);
          }
        }
        if (known.length === 0) {
          form.setError("root.server", { type: "validation_failed", message: text("We could not attach your report to this conversation. Close this window and try again.") });
        } else {
          try {
            form.setFocus(known[0] as "reason" | "details");
          } catch {
            // Radio groups have no single focus target.
          }
        }
        announce(text(errorMessage(e)), "assertive");
      } else {
        setFailure(result);
        announce(text(errorMessage(e)), "assertive");
      }
    } finally {
      inflight.current = false;
    }
  }

  const rootError = (form.formState.errors.root as { server?: { message?: string } } | undefined)?.server?.message;
  const other = reason === "other";

  return (
    <Form {...form}>
      <form onSubmit={(event) => void form.handleSubmit(submit)(event)} noValidate className="grid gap-5" data-testid="report-form" aria-busy={busy || undefined}>
        <FormField
          control={form.control}
          name="reason"
          render={({ field }) => (
            <FormItem>
              <span className="text-sm font-medium" id={`${reasonId}-label`}>
                {text("What went wrong?")}
              </span>
              <FormControl>
                <RadioGroup
                  name={field.name}
                  value={field.value ?? ""}
                  onValueChange={(value) => {
                    if (isChatReportReason(value)) field.onChange(value);
                  }}
                  onBlur={field.onBlur}
                  aria-labelledby={`${reasonId}-label`}
                  aria-required="true"
                  disabled={busy}
                  className="gap-2"
                >
                  {CHAT_REPORT_REASONS.map((value) => (
                    <label
                      key={value}
                      className="flex min-h-11 cursor-pointer items-center gap-3 rounded-lg border border-gold/50 bg-white px-3 py-2 text-sm transition-colors hover:border-gold has-data-checked:border-amber has-data-checked:bg-cream has-focus-visible:ring-3 has-focus-visible:ring-ring/50"
                    >
                      <RadioGroupItem value={value} data-testid={`report-reason-${value}`} />
                      <span>{text(chatReportReasonLabels[value])}</span>
                    </label>
                  ))}
                </RadioGroup>
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FieldRow
          control={form.control}
          name="details"
          label={text(other ? "Tell us what happened" : "More details (optional)")}
          maxLength={CHAT_REPORT_DETAILS_MAX}
          showValid={other}
        >
          {(field) => (
            <Textarea
              {...field}
              rows={4}
              maxLength={CHAT_REPORT_DETAILS_MAX + 200}
              disabled={busy}
              placeholder={text("What happened, and when? Please do not include passwords or card numbers.")}
              className="min-h-24 resize-y bg-white"
              autoComplete="off"
              onFocus={(event) => {
                // Keep the field visible above the phone keyboard.
                const el = event.currentTarget;
                window.setTimeout(() => el.scrollIntoView?.({ block: "nearest" }), 250);
              }}
            />
          )}
        </FieldRow>
        <p className="flex items-start gap-2 text-xs text-ink/80" data-testid="report-privacy">
          <ShieldCheck aria-hidden className="mt-0.5 size-4 shrink-0 text-amber" />
          <span>{text("Our support team may review this conversation's messages and session details to investigate. {value0} is not told about your report.", { value0: target.advisorName })}</span>
        </p>
        {rootError ? (
          <div role="alert" className="flex items-start gap-2 rounded-lg border border-danger/40 bg-danger/5 p-3 text-sm text-danger" data-testid="report-root-error">
            <AlertCircle aria-hidden className="mt-0.5 size-4 shrink-0" />
            <p>{rootError}</p>
          </div>
        ) : null}
        {failure ? <FailureNotice failure={failure} busy={busy} onClose={onClose} onRetry={() => void form.handleSubmit(submit)()} /> : null}
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button type="button" variant="outline" className="h-11 border-gold bg-cream sm:h-10" onClick={onClose} disabled={busy}>
            {text("Cancel")}
          </Button>
          <Button type="submit" className="gold-button h-11 sm:h-10" disabled={busy} aria-busy={busy} onMouseDown={keepFocusOnPress} data-testid="report-submit">
            {text(busy ? "Sending…" : "Send report")}
          </Button>
        </div>
      </form>
    </Form>
  );
}

/** Dialog body: mounted only while open, so every opening starts from a fresh check. */
function ReportDialogBody({ target, onClose }: { target: ReportIssueTarget; onClose: () => void }) {
  const { text } = useI18n();
  const check = useAsync((signal) => supportApi.openChatReport(target.conversationId, signal), [target.conversationId]);
  const [outcome, setOutcome] = React.useState<Outcome | null>(null);

  return (
    <>
      <DialogHeader>
        <DialogTitle className="text-xl">{text("Report an issue")}</DialogTitle>
        <DialogDescription>{text("About your conversation with {value0}", { value0: target.advisorName })}</DialogDescription>
      </DialogHeader>
      {outcome ? (
        <OutcomeView outcome={outcome} onClose={onClose} />
      ) : check.loading || (!check.data && !check.error) ? (
        <DialogSkeleton />
      ) : check.data?.open ? (
        <OutcomeView outcome={{ kind: "exists", reference: existingReference({ reference: check.data.open.reference }) ?? null }} onClose={onClose} />
      ) : (
        // A failed check falls back to the form: the POST answers 409 when a report is open.
        <ReportForm target={target} onOutcome={setOutcome} onClose={onClose} />
      )}
    </>
  );
}

export function ReportIssueButton({ target, placement, className }: ReportIssueButtonProps) {
  const { text } = useI18n();
  const [open, setOpen] = React.useState(false);
  const close = React.useCallback(() => setOpen(false), []);
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Tooltip>
        <TooltipTrigger asChild>
          <DialogTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={text("Report an issue")}
              data-testid="report-issue-trigger"
              data-placement={placement}
              // 32px glyph box; the ::after extends the touch target to 44px without a bigger look.
              className={cn("chat-report-trigger relative size-8 text-ink hover:bg-cream/40 after:absolute after:-inset-1.5", className)}
            >
              <FiFlag className="size-4" aria-hidden />
            </Button>
          </DialogTrigger>
        </TooltipTrigger>
        <TooltipContent>{text("Report an issue")}</TooltipContent>
      </Tooltip>
      <DialogContent data-testid="report-dialog" size="md">
        <ReportDialogBody target={target} onClose={close} />
      </DialogContent>
    </Dialog>
  );
}
