"use client";

import * as React from "react";
import { FiInfo, FiMenu } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import type { ChatSession, Participant } from "@/lib/api/types";
import { formatDate, formatDuration } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useClientName, usePrimeClientAliases } from "@/lib/chat/client-alias-provider";
import { ClientNameField } from "@/components/chat/client-name-field";
import { ReaderAvatar } from "@/components/chat/reader-avatar";
import { RatePrice } from "@/components/chat/rate-price";
import { formatMoney, type RateView } from "@/components/chat/chat-model";
import { ReportIssueButton, type ReportIssueTarget } from "@/components/chat/report-issue";
import { useClientProfile } from "@/components/chat/use-client-profile";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

type Earnings = { credits: number; cents?: number; currency?: string };

type ChatHeaderProps = {
  counterpart: Participant;
  /** The conversation on screen: advisors see the client's date of birth under the name (§22.6). */
  conversationId?: string | null;
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
  /** Phones: unread messages in other conversations or unread notifications (dot on the menu button). */
  activity?: boolean;
  /** Clients with a conversation: "Report an issue" (tablet and up; phones have it in the tools row). */
  report?: ReportIssueTarget | null;
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

/** ☰ that opens the conversation drawer; dotted when there is activity elsewhere (§20.2). */
function InboxMenuButton({ onClick, activity }: { onClick: () => void; activity: boolean }) {
  const { text, t } = useI18n();
  return (
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label={activity ? text("Open conversations, new activity") : t("chatroom.inbox.openLabel")}
      onClick={onClick}
      className="chat-header-menu relative text-ink hover:bg-cream/40 lg:hidden"
      data-activity={activity || undefined}
    >
      <FiMenu className="size-5" aria-hidden />
      {activity ? <span className="chat-menu-dot" aria-hidden data-testid="menu-activity-dot" /> : null}
    </Button>
  );
}

/**
 * Phone top bar while there is no conversation on screen (empty, loading or
 * failed inbox): the site header is hidden there, so the drawer (account,
 * navigation) stays reachable from here.
 */
export function ChatroomTopBar({ onOpenInbox, activity = false }: { onOpenInbox: () => void; activity?: boolean }) {
  const { text } = useI18n();
  return (
    <div className="chat-header chat-topbar-empty flex items-center gap-3 bg-sage p-4 md:hidden" data-testid="chat-topbar">
      <InboxMenuButton onClick={onOpenInbox} activity={activity} />
      <span className="font-serif text-[18px] font-bold text-ink">{text("Chatroom")}</span>
    </div>
  );
}

export function ChatHeader({ counterpart, conversationId = null, viewerRole = "client", session, elapsedSeconds, rate, earnings, onOpenInbox, onOpenDetails, onEnd, ending, activity = false, report }: ChatHeaderProps) {
  const { text, t, locale } = useI18n();
  // Advisors only; shares one cached request with the client panel.
  const advisor = viewerRole === "psychic";
  const clientProfile = useClientProfile(advisor ? conversationId : null);
  // Advisors see their private nickname for the client (§23.4); clients and advisors alike get the real name from `counterpart`.
  const profileAlias = clientProfile.profile;
  const aliasEntries = React.useMemo(() => (profileAlias ? [{ clientId: profileAlias.id, alias: profileAlias.alias }] : []), [profileAlias]);
  usePrimeClientAliases(advisor ? aliasEntries : []);
  const displayName = useClientName(advisor ? counterpart.id : null, counterpart.name);
  const dateOfBirth = clientProfile.profile?.date_of_birth ?? clientProfile.profile?.birth_details?.date_of_birth;
  const born = dateOfBirth ? formatDate(dateOfBirth, locale, "long") : "";
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
        <InboxMenuButton onClick={onOpenInbox} activity={activity} />
        <button
          type="button"
          className="chat-header-avatar relative shrink-0 xl:pointer-events-none"
          onClick={onOpenDetails}
          aria-label={text("View {value0}'s details", { value0: displayName })}
        >
          <ReaderAvatar initial={displayName.charAt(0).toUpperCase()} image={counterpart.avatar_url} />
          {presence ? <span className="presence-dot presence-dot-avatar" data-online={presence === "online" || undefined} aria-hidden /> : null}
        </button>
        <div className="flex min-w-0 flex-col gap-0.5 text-ink">
          {advisor ? (
            <ClientNameField key={conversationId ?? "none"} conversationId={conversationId} clientId={counterpart.id} realName={counterpart.name} />
          ) : (
            <span className="chat-header-name truncate font-serif text-[18px] font-bold">{text(counterpart.name)}</span>
          )}
          {advisor && conversationId && clientProfile.loading ? (
            <span className="chat-header-dob" aria-hidden data-testid="chat-header-dob-loading">
              <Skeleton className="h-3 w-24 bg-cream/70" />
            </span>
          ) : born ? (
            <span className="chat-header-dob truncate font-ui text-xs" data-testid="chat-header-dob">
              <span className="sr-only">{text("Born")} </span>
              <time dateTime={dateOfBirth}>{born}</time>
            </span>
          ) : null}
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
              aria-label={t("chatroom.header.endChatLabel", { name: displayName })}
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
        {report ? <ReportIssueButton placement="header" target={report} className="max-md:hidden" /> : null}
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("chatroom.header.openDetails")}
          onClick={onOpenDetails}
          className={cn("text-ink hover:bg-cream/40 max-md:hidden xl:hidden", "chat-header-details")}
        >
          <FiInfo className="size-4" aria-hidden />
        </Button>
      </div>
    </div>
  );
}
