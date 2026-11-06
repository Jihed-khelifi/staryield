"use client";

import { FiInfo, FiMenu } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import type { ChatSession, Participant } from "@/lib/api/types";
import { formatDuration } from "@/lib/format";
import { cn } from "@/lib/utils";
import { ReaderAvatar } from "@/components/chat/reader-avatar";
import { RatePrice } from "@/components/chat/rate-price";
import { formatMoney, type RateView } from "@/components/chat/chat-model";
import { Button } from "@/components/ui/button";

type Earnings = { credits: number; cents?: number; currency?: string };

type ChatHeaderProps = {
  counterpart: Participant;
  /** The viewer's role: advisors see their client's online/offline presence and their earnings. */
  viewerRole?: "client" | "psychic";
  session: ChatSession | null;
  elapsedSeconds: number;
  /** The per-minute rate as the viewer sees it (member discount for clients). */
  rate?: RateView | null;
  /** Advisor: live earnings of the session so far. */
  earnings?: Earnings | null;
  onOpenInbox: () => void;
  onOpenDetails: () => void;
  onEnd: () => void;
  ending: boolean;
};

const statusCopy: Record<string, string> = {
  online: "Online",
  busy: "In a reading",
  away: "Away",
  offline: "Offline",
};

/** "$40.33" when the money value is known, else "24 credits". */
export function useEarningsLabel(earnings: Earnings | null | undefined): string | null {
  const { text, locale } = useI18n();
  if (!earnings) return null;
  if (typeof earnings.cents === "number") return formatMoney(earnings.cents, earnings.currency, locale);
  return text("{value0} credits", { value0: earnings.credits.toLocaleString(locale) });
}

export function ChatHeader({ counterpart, viewerRole = "client", session, elapsedSeconds, rate, earnings, onOpenInbox, onOpenDetails, onEnd, ending }: ChatHeaderProps) {
  const { text, t } = useI18n();
  const live = session && (session.status === "active" || session.status === "disconnected" || session.status === "accepted");
  const presence = viewerRole === "psychic" && counterpart.status ? (counterpart.status === "offline" ? "offline" : "online") : null;
  const statusLine =
    session?.status === "active"
      ? text("Live chat in progress")
      : session?.status === "disconnected"
        ? text("Reconnecting…")
        : counterpart.status
          ? text(statusCopy[counterpart.status] ?? counterpart.status)
          : "";
  const earned = useEarningsLabel(viewerRole === "psychic" && session && session.status !== "requested" ? earnings : null);
  const showTimer = Boolean(session && session.status !== "requested");

  return (
    <div className="chat-header flex items-center justify-between gap-3 bg-sage p-4" data-testid="chat-header">
      <div className="flex min-w-0 items-center gap-2 sm:gap-3">
        <Button variant="ghost" size="icon-sm" aria-label={t("chatroom.inbox.openLabel")} onClick={onOpenInbox} className="chat-header-menu text-ink hover:bg-cream/40 lg:hidden">
          <FiMenu className="size-5" aria-hidden />
        </Button>
        <button
          type="button"
          className="chat-header-avatar relative shrink-0 xl:pointer-events-none"
          onClick={onOpenDetails}
          aria-label={text("View {value0}'s details", { value0: counterpart.name })}
        >
          <ReaderAvatar initial={counterpart.name.charAt(0).toUpperCase()} image={counterpart.avatar_url} />
          {presence ? <span className="presence-dot presence-dot-avatar" data-online={presence === "online" || undefined} aria-hidden /> : null}
        </button>
        <div className="flex min-w-0 flex-col gap-0.5 text-ink">
          <span className="chat-header-name truncate font-serif text-[18px] font-bold">{text(counterpart.name)}</span>
          <span className="chat-reader-status font-ui text-xs opacity-80" data-testid="chat-header-status">
            <span className="chat-status-dot" aria-hidden>
              ●{" "}
            </span>
            {statusLine}
            {rate && rate.rate > 0 ? (
              <span className="chat-header-rate">
                {" "}
                · <RatePrice rate={rate} />
              </span>
            ) : null}
          </span>
        </div>
      </div>

      <div className="flex items-center gap-2">
        <div className="flex flex-col items-end gap-0.5 text-ink">
          {live ? (
            <button
              type="button"
              onClick={onEnd}
              disabled={ending}
              aria-label={t("chatroom.header.endChatLabel", { name: counterpart.name })}
              className="chat-end-button cursor-pointer rounded-sm font-serif text-base font-medium whitespace-nowrap transition-colors hover:text-cream-light focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none disabled:opacity-60"
            >
              {t("chatroom.header.endChat")}
            </button>
          ) : null}
          {showTimer ? (
            <span className="chat-header-clock flex items-baseline gap-1 font-ui text-xs whitespace-nowrap">
              <span aria-label={t("chatroom.header.elapsed")} role="timer" className="tabular-nums">
                {formatDuration(elapsedSeconds)}
              </span>
              {earned ? (
                <span className="chat-header-earnings tabular-nums" data-testid="session-earnings">
                  <span aria-hidden="true">({earned})</span>
                  <span className="sr-only">{text("Session earnings: {value0}", { value0: earned })}</span>
                </span>
              ) : null}
            </span>
          ) : null}
        </div>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("chatroom.header.openDetails")}
          onClick={onOpenDetails}
          className={cn("text-ink hover:bg-cream/40 xl:hidden", "chat-header-details")}
        >
          <FiInfo className="size-4" aria-hidden />
        </Button>
      </div>
    </div>
  );
}
