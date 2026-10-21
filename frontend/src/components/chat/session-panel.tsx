"use client";

import Link from "next/link";

import { useI18n } from "@/i18n/i18n-provider";
import { errorMessage, type ApiError } from "@/lib/api/errors";
import type { ChatSession, Participant, Review, SessionSummary, Wallet } from "@/lib/api/types";
import { formatDuration } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { endReasonCopy } from "@/components/chat/chat-model";
import { ReviewForm, ReviewThanks } from "@/components/reviews/review-form";

type Props = {
  role: "client" | "psychic";
  counterpart: Participant;
  session: ChatSession | null;
  summary: SessionSummary | null;
  wallet: Wallet | null;
  rate: number;
  secondsLeft: number;
  busy: boolean;
  error: ApiError | null;
  returnPath: string;
  onStart: () => void;
  onCancel: () => void;
  onAccept: () => void;
  onDecline: () => void;
  onDismissSummary: () => void;
  /** Client: the summary's session was rated. */
  onReviewed?: (review: Review) => void;
};

/** The state-dependent card above the composer (spec §5 and §6). */
export function SessionPanel(p: Props) {
  const { text, locale } = useI18n();
  const s = p.session;
  const name = text(p.counterpart.name);

  const errorBlock = p.error ? (
    <div role="alert" className="mt-3 text-sm text-red-800">
      <p>{text(errorMessage(p.error))}</p>
      {p.error.code === "insufficient_credits" ? (
        <Link className="underline" href={`/${locale}/credits?next=${encodeURIComponent(p.returnPath)}`}>
          {text("Top up credits")}
        </Link>
      ) : null}
    </div>
  ) : null;

  if (s?.status === "requested") {
    return p.role === "psychic" ? (
      <div className="chat-session-card" role="alert">
        <p className="font-semibold">{text("{value0} would like to start a live chat.", { value0: name })}</p>
        <p className="text-xs">{text("This request expires in {value0} seconds.", { value0: p.secondsLeft })}</p>
        <div className="mt-3 flex gap-3">
          <Button className="gold-button" onClick={p.onAccept} disabled={p.busy}>
            {text("Accept")}
          </Button>
          <Button variant="outline" onClick={p.onDecline} disabled={p.busy}>
            {text("Decline")}
          </Button>
        </div>
        {errorBlock}
      </div>
    ) : (
      <div className="chat-session-card" role="status">
        <p className="font-semibold">{text("Waiting for {value0} to accept…", { value0: name })}</p>
        <p className="text-xs">{text("This request expires in {value0} seconds.", { value0: p.secondsLeft })}</p>
        <Button variant="outline" className="mt-3" onClick={p.onCancel} disabled={p.busy}>
          {text("Cancel request")}
        </Button>
        {errorBlock}
      </div>
    );
  }
  if (s?.status === "accepted") {
    return (
      <div className="chat-session-card" role="status">
        <p className="font-semibold">{text("Connecting you with {value0}…", { value0: name })}</p>
        <p className="text-xs">{text("The chat starts as soon as you are both connected. Billing starts then.")}</p>
      </div>
    );
  }
  if (s?.status === "disconnected") {
    return (
      <div className="chat-session-card" role="status">
        <p className="font-semibold">
          {text(s.disconnected_party === (p.role === "client" ? "psychic" : "client") ? "{value0} lost connection." : "Your connection was interrupted.", { value0: name })}
        </p>
        <p className="text-xs">
          {text("Billing is paused. The chat resumes automatically if the connection returns within {value0} seconds.", { value0: p.secondsLeft })}
        </p>
      </div>
    );
  }
  if (s?.status === "active") return null;

  if (p.summary) {
    const sum = p.summary;
    return (
      <div className="chat-session-card" data-testid="chat-summary">
        <p className="font-semibold">{text("Chat summary")}</p>
        {sum.session.end_reason && endReasonCopy[sum.session.end_reason] ? <p className="text-xs">{text(endReasonCopy[sum.session.end_reason])}</p> : null}
        <dl className="mt-2 grid grid-cols-3 gap-2 text-xs">
          <div>
            <dt className="opacity-70">{text("Duration")}</dt>
            <dd className="font-semibold">{formatDuration(sum.duration_seconds)}</dd>
          </div>
          <div>
            <dt className="opacity-70">{text("Billed minutes")}</dt>
            <dd className="font-semibold">
              {sum.session.billed_minutes}
              {sum.session.free_minutes_used ? ` (${text("{value0} free", { value0: sum.session.free_minutes_used })})` : ""}
            </dd>
          </div>
          <div>
            <dt className="opacity-70">{text(p.role === "client" ? "Credits used" : "Your earnings")}</dt>
            <dd className="font-semibold">{(p.role === "client" ? sum.session.credits_charged : sum.session.psychic_earnings) ?? 0}</dd>
          </div>
        </dl>
        {p.role === "client" && sum.review ? (
          <ReviewThanks review={sum.review} />
        ) : p.role === "client" && sum.can_review ? (
          <ReviewForm sessionId={sum.session.id} advisorName={name} onSubmitted={(r) => p.onReviewed?.(r)} />
        ) : null}
        <Button variant="outline" className="mt-3" onClick={p.onDismissSummary}>
          {text("Close summary")}
        </Button>
      </div>
    );
  }

  if (p.role === "psychic") {
    return s && endReasonCopy[s.end_reason ?? ""] ? (
      <div className="chat-session-card">
        <p className="text-xs">{text(endReasonCopy[s.end_reason!])}</p>
      </div>
    ) : null;
  }

  const offline = p.counterpart.status && p.counterpart.status !== "online";
  const free = p.wallet?.free_minutes ?? 0;
  return (
    <div className="chat-session-card">
      {s && endReasonCopy[s.end_reason ?? ""] ? <p className="mb-2 text-xs">{text(endReasonCopy[s.end_reason!])}</p> : null}
      <p className="font-semibold">{text("Start a private live chat with {value0}", { value0: name })}</p>
      <p className="text-xs">
        {free > 0
          ? text("{value0} free minutes, then {value1} credits per started minute.", { value0: free, value1: p.rate })
          : text("{value0} credits per started minute. Balance: {value1} credits.", { value0: p.rate, value1: (p.wallet?.balance ?? 0).toLocaleString(locale) })}
      </p>
      <Button className="gold-button mt-3" onClick={p.onStart} disabled={p.busy || Boolean(offline)} aria-busy={p.busy}>
        {text(offline ? "Advisor unavailable" : p.busy ? "Sending request…" : "♧ Start live chat ")}
      </Button>
      {errorBlock}
    </div>
  );
}
