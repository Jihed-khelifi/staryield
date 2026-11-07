"use client";

import * as React from "react";
import Link from "next/link";
import { CheckCircle2, CircleDot, Clock, Flag, Lock, MessageCircleReply } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { toApiError } from "@/lib/api/errors";
import type { CustomerTicket, CustomerTicketMessage, CustomerTicketRow, TicketStatus } from "@/lib/api/types";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { supportReplySchema, type SupportReplyValues } from "@/lib/schemas/support-reply";
import { chatReportReasonLabels } from "@/lib/schemas/chat-report";
import { supportTopicLabels } from "@/lib/schemas/contact";
import { isChatReport, reportConversationHref } from "@/lib/support/chat-report";
import { classifySupportError, markClosed, REPLY_MAX, ticketStatusCopy, waitHint } from "@/lib/support/support";
import { cn } from "@/lib/utils";
import { announce } from "@/components/feedback/live-region";
import { useToast } from "@/components/feedback/toast-provider";
import { Button } from "@/components/ui/button";
import { FieldRow } from "@/components/ui/field";
import { Form, FormRootError } from "@/components/ui/form";
import { Textarea } from "@/components/ui/textarea";

const CHIP: Record<TicketStatus, { icon: typeof Clock; className: string }> = {
  open: { icon: Clock, className: "bg-cream-light" },
  pending: { icon: MessageCircleReply, className: "bg-gold-cta font-semibold" },
  resolved: { icon: CheckCircle2, className: "bg-cream-light" },
  closed: { icon: Lock, className: "bg-warm-gray/50" },
};

/** "Awaiting support" / "Awaiting your reply" / "Resolved" / "Closed": icon and words, never colour alone. */
export function StatusChip({ status, className }: { status: TicketStatus; className?: string }) {
  const { text } = useI18n();
  const { icon: Icon, className: tone } = CHIP[status];
  return (
    <span data-testid="ticket-status" data-status={status} className={cn("status-pill inline-flex items-center gap-1.5 text-ink", tone, className)}>
      <Icon size={13} aria-hidden />
      {text(ticketStatusCopy[status])}
    </span>
  );
}

/** Marks a conversation report in lists and threads: icon and words, never colour alone. */
export function ReportBadge({ className }: { className?: string }) {
  const { text } = useI18n();
  return (
    <span
      data-testid="report-badge"
      className={cn("inline-flex items-center gap-1.5 rounded-full border border-amber/70 bg-gold-cta/40 px-2.5 py-0.5 text-xs font-semibold text-ink", className)}
    >
      <Flag size={12} aria-hidden />
      {text("Conversation report")}
    </span>
  );
}

/** The title of a request: the localized reason for a conversation report, the subject otherwise. */
export function useTicketTitle() {
  const { text } = useI18n();
  return React.useCallback(
    (row: Pick<CustomerTicketRow, "kind" | "report" | "subject">) => (isChatReport(row) ? text(chatReportReasonLabels[row.report.reason] ?? chatReportReasonLabels.other) : row.subject),
    [text],
  );
}

/** "With {advisor} · {session date}" for a conversation report, or null for other requests. */
export function useReportMeta() {
  const { text } = useI18n();
  const format = useDateFormat();
  return React.useCallback(
    (row: Pick<CustomerTicketRow, "kind" | "report">): string | null => {
      if (!isChatReport(row)) return null;
      const advisor = row.report.advisor?.name || text("an advisor");
      const when = row.report.session_started_at ? format(row.report.session_started_at) : "";
      return when ? text("With {value0} · {value1}", { value0: advisor, value1: when }) : text("With {value0}", { value0: advisor });
    },
    [text, format],
  );
}

function useDateFormat() {
  const { locale } = useI18n();
  return React.useCallback(
    (iso: string) => {
      const at = new Date(iso);
      return Number.isNaN(at.getTime()) ? "" : new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(at);
    },
    [locale],
  );
}

export function useFormatTicketDate() {
  return useDateFormat();
}

/** Detail card of a conversation report: what it is about, and a way back to the conversation. */
function ReportContextCard({ ticket, showConversationLink }: { ticket: CustomerTicket; showConversationLink: boolean }) {
  const { text, locale } = useI18n();
  const format = useDateFormat();
  if (!isChatReport(ticket)) return null;
  const { report } = ticket;
  const advisor = report.advisor?.name || text("an advisor");
  const href = showConversationLink ? reportConversationHref(locale, ticket) : null;
  return (
    <section data-testid="report-context" aria-labelledby="report-context-title" className="mt-5 grid gap-3 rounded-xl border border-amber/60 bg-cream-light p-4">
      <h3 id="report-context-title" className="flex items-start gap-2 font-semibold [overflow-wrap:anywhere]">
        <Flag size={16} aria-hidden className="mt-1 shrink-0 text-amber" />
        {text("Report about your conversation with {value0}", { value0: advisor })}
      </h3>
      <dl className="grid gap-x-6 gap-y-1 text-sm sm:grid-cols-[auto_1fr]">
        <dt className="text-ink/75">{text("Reason")}</dt>
        <dd data-testid="report-context-reason">{text(chatReportReasonLabels[report.reason] ?? chatReportReasonLabels.other)}</dd>
        {report.session_started_at ? (
          <>
            <dt className="text-ink/75">{text("Session date")}</dt>
            <dd>{format(report.session_started_at)}</dd>
          </>
        ) : null}
      </dl>
      {href ? (
        <Button asChild variant="outline" size="sm" className="w-fit border-gold bg-cream">
          <Link href={href} data-testid="report-open-conversation">
            {text("Open conversation")}
          </Link>
        </Button>
      ) : null}
    </section>
  );
}

function Message({ message }: { message: CustomerTicketMessage }) {
  const { text } = useI18n();
  const format = useDateFormat();
  const mine = message.author.type === "customer";
  return (
    <li
      data-testid="ticket-message"
      data-author={message.author.type}
      className={cn(
        "max-w-full rounded-xl border p-4 sm:max-w-[88%]",
        mine ? "self-end border-gold/50 bg-cream-light" : "self-start border-gold bg-white",
      )}
    >
      <header className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5 text-sm">
        <span className="font-semibold [overflow-wrap:anywhere]">{message.author.name}</span>
        <span className="rounded-full border border-ink/25 px-2 text-xs">{text(mine ? "You" : "Support team")}</span>
        <time className="text-xs text-ink/75" dateTime={message.created_at}>
          {format(message.created_at)}
        </time>
      </header>
      {/* Plain text only: line breaks kept, long words and URLs wrap. */}
      <p className="mt-2 font-serif leading-relaxed whitespace-pre-wrap [overflow-wrap:anywhere]">{message.body}</p>
    </li>
  );
}

export function ClosedNotice({ className }: { className?: string }) {
  const { text, locale } = useI18n();
  return (
    <div role="status" data-testid="ticket-closed" className={cn("flex flex-col items-start gap-3 rounded-xl border border-gold bg-cream-light p-4", className)}>
      <p className="flex items-center gap-2 font-semibold">
        <Lock size={16} aria-hidden className="text-amber" />
        {text("This request is closed")}
      </p>
      <p className="text-sm">{text("You cannot reply to it any more. If you still need help, send us a new request.")}</p>
      <Button asChild className="gold-button">
        <Link href={`/${locale}/contact`}>{text("Send a new request")}</Link>
      </Button>
    </div>
  );
}

/** The reply composer: 1–5,000 characters, counter, valid/invalid states, loading, success toast. */
function ReplyComposer({
  ticket,
  reply,
  onTicket,
}: {
  ticket: CustomerTicket;
  reply: (body: string) => Promise<CustomerTicket>;
  onTicket: (ticket: CustomerTicket) => void;
}) {
  const { text } = useI18n();
  const toast = useToast();
  const form = useSchemaForm(supportReplySchema, { defaultValues: { body: "" } });
  const pending = form.formState.isSubmitting;

  async function submit(values: SupportReplyValues) {
    try {
      const updated = await reply(values.body.trim());
      onTicket(updated);
      form.reset({ body: "" });
      const message = text("Reply sent. We will get back to you by email.");
      toast.show({ tone: "success", message });
      announce(message);
    } catch (err) {
      const { kind, retryAfter } = classifySupportError(err, "account");
      if (kind === "closed") {
        onTicket(markClosed(ticket));
        toast.show({ tone: "error", message: text("This request is closed") });
        return;
      }
      if (kind === "rate_limited") {
        const hint = waitHint(retryAfter);
        const message =
          hint.unit === "seconds"
            ? text("You are sending replies too fast. Try again in {value0} seconds.", { value0: hint.value })
            : hint.unit === "minutes"
              ? text("You are sending replies too fast. Try again in about {value0} minutes.", { value0: hint.value })
              : text("You are sending replies too fast. Please wait a little before trying again.");
        form.setError("root.server", { type: "rate_limited", message });
        return;
      }
      applyApiError(form, toApiError(err), { text });
    }
  }

  return (
    <Form {...form}>
      <form onSubmit={(event) => void form.handleSubmit(submit)(event)} noValidate className="grid gap-4" data-testid="reply-form">
        <FieldRow
          control={form.control}
          name="body"
          label={text(ticket.status === "resolved" ? "Reply to reopen this request" : "Your reply")}
          maxLength={REPLY_MAX}
        >
          {(field) => <Textarea {...field} rows={5} maxLength={REPLY_MAX + 500} placeholder={text("Write your reply…")} className="min-h-32 resize-y bg-white" />}
        </FieldRow>
        <FormRootError />
        <Button type="submit" className="gold-button h-12 w-full sm:w-56" disabled={pending} aria-busy={pending} onMouseDown={keepFocusOnPress}>
          {text(pending ? "Sending…" : "Send reply")}
        </Button>
      </form>
    </Form>
  );
}

/** A request: header with the status chip, the conversation, then the composer or the closed notice. */
export function TicketThread({
  ticket,
  reply,
  onTicket,
  onRefresh,
  refreshing,
  showConversationLink = false,
}: {
  showConversationLink?: boolean;
  ticket: CustomerTicket;
  reply: (body: string) => Promise<CustomerTicket>;
  onTicket: (ticket: CustomerTicket) => void;
  onRefresh?: () => void;
  refreshing?: boolean;
}) {
  const { text } = useI18n();
  const format = useDateFormat();
  const titleOf = useTicketTitle();
  const metaOf = useReportMeta();
  const meta = metaOf(ticket);
  return (
    <article data-testid="ticket-thread" aria-labelledby="ticket-subject" aria-busy={refreshing || undefined}>
      <header className="grid gap-3 border-b border-gold/30 pb-5">
        <p className="font-display text-sm tracking-wider" data-testid="ticket-reference">
          {ticket.reference}
        </p>
        <h2 id="ticket-subject" className="text-2xl font-semibold [overflow-wrap:anywhere]">
          {titleOf(ticket)}
        </h2>
        {meta ? (
          <p className="text-sm text-ink/80 [overflow-wrap:anywhere]" data-testid="report-meta">
            {meta}
          </p>
        ) : null}
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm">
          <StatusChip status={ticket.status} />
          {isChatReport(ticket) ? <ReportBadge /> : null}
          <span className="inline-flex items-center gap-1.5">
            <CircleDot size={12} aria-hidden className="text-amber" />
            {text(supportTopicLabels[ticket.topic] ?? supportTopicLabels.general)}
          </span>
          <span>{text("Opened {value0}", { value0: format(ticket.created_at) })}</span>
          {onRefresh ? (
            <Button type="button" variant="outline" size="sm" className="ml-auto border-gold/60" onClick={onRefresh} disabled={refreshing}>
              {text(refreshing ? "Refreshing…" : "Refresh")}
            </Button>
          ) : null}
        </div>
      </header>
      <ReportContextCard ticket={ticket} showConversationLink={showConversationLink} />
      {ticket.messages.length === 0 ? (
        // A conversation report can be sent without details (§16.4): nothing to list yet.
        <p className="my-6 rounded-lg border border-dashed border-gold/60 px-4 py-3 text-sm text-ink/80" role="note" data-testid="ticket-no-messages">
          {text(ticket.can_reply ? "You sent this report without details. You can add them below." : "This request has no messages.")}
        </p>
      ) : (
        <ol className="my-6 flex flex-col gap-4" aria-label={text("Conversation")}>
          {ticket.messages.map((message) => (
            <Message key={message.id} message={message} />
          ))}
        </ol>
      )}
      {ticket.can_reply ? <ReplyComposer key={ticket.reference} ticket={ticket} reply={reply} onTicket={onTicket} /> : <ClosedNotice />}
    </article>
  );
}
