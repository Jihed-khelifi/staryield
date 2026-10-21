"use client";

import { FiInfo, FiMessageSquare } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import type { ChatSession, Participant } from "@/lib/api/types";
import { formatDuration } from "@/lib/format";
import { ReaderAvatar } from "@/components/chat/reader-avatar";
import { Button } from "@/components/ui/button";

type ChatHeaderProps = {
  counterpart: Participant;
  /** The viewer's role: advisors see their client's online/offline presence. */
  viewerRole?: "client" | "psychic";
  session: ChatSession | null;
  elapsedSeconds: number;
  rate?: number;
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

export function ChatHeader({ counterpart, viewerRole = "client", session, elapsedSeconds, rate, onOpenInbox, onOpenDetails, onEnd, ending }: ChatHeaderProps) {
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

  return (
    <div className="chat-header flex items-center justify-between gap-3 bg-sage p-4">
      <div className="flex min-w-0 items-center gap-2 sm:gap-3">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("chatroom.inbox.openLabel")}
          onClick={onOpenInbox}
          className="text-ink hover:bg-cream/40 lg:hidden"
        >
          <FiMessageSquare className="size-4" aria-hidden />
        </Button>
        <button type="button" className="relative shrink-0 xl:pointer-events-none" onClick={onOpenDetails} aria-label={text("View {value0}'s details", { value0: counterpart.name })}>
          <ReaderAvatar initial={counterpart.name.charAt(0).toUpperCase()} image={counterpart.avatar_url} />
          {presence ? <span className="presence-dot presence-dot-avatar" data-online={presence === "online" || undefined} aria-hidden /> : null}
        </button>
        <div className="flex min-w-0 flex-col gap-0.5 text-ink">
          <span className="truncate font-serif text-[18px] font-bold">{text(counterpart.name)}</span>
          <span className="chat-reader-status font-ui text-xs opacity-80" data-testid="chat-header-status">
            {statusLine}
            {rate ? (
              <span>
                {" "}
                · {rate} {text("credits/min ")}
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
              className="cursor-pointer rounded-sm font-serif text-base font-medium whitespace-nowrap transition-colors hover:text-cream-light focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none disabled:opacity-60"
            >
              {t("chatroom.header.endChat")}
            </button>
          ) : null}
          {session && session.status !== "requested" ? (
            <span aria-label={t("chatroom.header.elapsed")} role="timer" className="font-ui text-xs tabular-nums opacity-80">
              {formatDuration(elapsedSeconds)}
            </span>
          ) : null}
        </div>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("chatroom.header.openDetails")}
          onClick={onOpenDetails}
          className="text-ink hover:bg-cream/40 xl:hidden"
        >
          <FiInfo className="size-4" aria-hidden />
        </Button>
      </div>
    </div>
  );
}
