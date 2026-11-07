"use client";

import * as React from "react";
import Link from "next/link";
import { CheckCircle2, CircleDot, Clock, Lock, MessageCircleReply } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { toApiError } from "@/lib/api/errors";
import type { CustomerTicket, CustomerTicketMessage, TicketStatus } from "@/lib/api/types";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { supportReplySchema, type SupportReplyValues } from "@/lib/schemas/support-reply";
import { supportTopicLabels } from "@/lib/schemas/contact";
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
}: {
  ticket: CustomerTicket;
  reply: (body: string) => Promise<CustomerTicket>;
  onTicket: (ticket: CustomerTicket) => void;
  onRefresh?: () => void;
  refreshing?: boolean;
}) {
  const { text } = useI18n();
  const format = useDateFormat();
  return (
    <article data-testid="ticket-thread" aria-labelledby="ticket-subject" aria-busy={refreshing || undefined}>
      <header className="grid gap-3 border-b border-gold/30 pb-5">
        <p className="font-display text-sm tracking-wider" data-testid="ticket-reference">
          {ticket.reference}
        </p>
        <h2 id="ticket-subject" className="text-2xl font-semibold [overflow-wrap:anywhere]">
          {ticket.subject}
        </h2>
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm">
          <StatusChip status={ticket.status} />
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
      <ol className="my-6 flex flex-col gap-4" aria-label={text("Conversation")}>
        {ticket.messages.map((message) => (
          <Message key={message.id} message={message} />
        ))}
      </ol>
      {ticket.can_reply ? <ReplyComposer key={ticket.reference} ticket={ticket} reply={reply} onTicket={onTicket} /> : <ClosedNotice />}
    </article>
  );
}
