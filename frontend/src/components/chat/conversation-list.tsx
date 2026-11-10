"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { errorMessage, type ApiError } from "@/lib/api/errors";
import type { Conversation } from "@/lib/api/types";
import { cn } from "@/lib/utils";
import { useClientNames } from "@/lib/chat/client-alias-provider";
import { ReaderAvatar } from "@/components/chat/reader-avatar";
import { systemCopy } from "@/components/chat/chat-model";
import { ErrorState } from "@/components/feedback/error-state";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";

/** Load the next page when the list is scrolled this close to its end. */
const END_SLACK_PX = 160;

type ConversationListProps = {
  conversations: Conversation[];
  /** Advisors see each client's online/offline presence. */
  viewerRole?: "client" | "psychic";
  activeId: string | null;
  onSelect: (conversation: Conversation) => void;
  className?: string;
  /** First page: loading, failed or ready. */
  status?: "idle" | "loading" | "ready" | "error";
  error?: ApiError | null;
  onRetry?: () => void;
  /** Cursor pagination (infinite scroll with a button fallback). */
  hasMore?: boolean;
  loadingMore?: boolean;
  moreError?: ApiError | null;
  onLoadMore?: () => void;
  /** "cards": the phone switcher sheet (Figma 1261:1537). */
  variant?: "rows" | "cards";
  /** Leave this conversation out (shown separately as the current one). */
  excludeId?: string | null;
};

const statusCopy: Record<string, string> = {
  online: "Online",
  busy: "Busy",
  away: "Away",
  offline: "Offline",
};

function ListSkeleton({ label }: { label: string }) {
  return (
    <div className="flex flex-col gap-2 p-3" role="status" aria-label={label} data-testid="inbox-loading">
      {[0, 1, 2].map((i) => (
        <div key={i} className="flex items-center gap-3 rounded-lg p-3" aria-hidden>
          <Skeleton className="size-10 rounded-lg bg-cream/70" />
          <div className="flex flex-1 flex-col gap-1.5">
            <Skeleton className="h-3.5 w-1/2 bg-cream/70" />
            <Skeleton className="h-3 w-4/5 bg-cream/70" />
          </div>
        </div>
      ))}
    </div>
  );
}

export function ConversationList({
  conversations,
  viewerRole = "client",
  activeId,
  onSelect,
  className,
  status = "ready",
  error,
  onRetry,
  hasMore = false,
  loadingMore = false,
  moreError,
  onLoadMore,
  variant = "rows",
  excludeId,
}: ConversationListProps) {
  const { t, text } = useI18n();
  const cards = variant === "cards";
  const rows = excludeId ? conversations.filter((c) => c.id !== excludeId) : conversations;
  // Advisors see their private nickname for each client (§23.4).
  const clientIds = React.useMemo(() => (viewerRole === "psychic" ? rows.map((c) => c.counterpart.id) : []), [viewerRole, rows]);
  const { nameFor } = useClientNames(clientIds);

  if (status === "loading" && conversations.length === 0) return <ListSkeleton label={text("Loading your conversations…")} />;
  if (status === "error" && conversations.length === 0 && error) {
    return (
      <div className="p-3">
        <ErrorState error={error} onRetry={onRetry} compact title={text("Your conversations could not be loaded.")} className="bg-cream/60" />
      </div>
    );
  }
  if (rows.length === 0 && !hasMore) {
    return (
      <p className="p-4 text-sm font-light" data-testid="inbox-empty">
        {conversations.length > 0 ? text("No other conversations yet.") : t("chatroom.inbox.empty")}
      </p>
    );
  }

  const onScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const el = e.currentTarget;
    if (hasMore && !loadingMore && !moreError && el.scrollHeight - el.scrollTop - el.clientHeight < END_SLACK_PX) onLoadMore?.();
  };

  return (
    <div className={cn("conversation-scroll h-full overflow-y-auto overscroll-contain", className)} onScroll={onScroll} data-testid="inbox" data-variant={variant}>
      <ul aria-label={t("chatroom.inbox.srHeading")} className="flex flex-col gap-2 p-3">
        {rows.map((conversation) => {
          const isActive = conversation.id === activeId;
          const last = conversation.last_message;
          const preview = last
            ? last.kind === "system"
              ? text(systemCopy(last.body))
              : last.kind === "image"
                ? `📷 ${last.body.trim() || text("Photo")}`
                : last.body
            : "";
          const shownName = viewerRole === "psychic" ? nameFor(conversation.counterpart.id, conversation.counterpart.name) : conversation.counterpart.name;
          const live = conversation.open_session?.status;
          const status = viewerRole === "psychic" ? conversation.counterpart.status : undefined;
          const online = status ? status !== "offline" : null;
          // Cards name the advisor's availability ("Theo · Busy").
          const advisorStatus = cards && viewerRole === "client" ? conversation.counterpart.status : undefined;
          return (
            <li key={conversation.id}>
              <button
                type="button"
                onClick={() => onSelect(conversation)}
                aria-current={isActive ? "true" : undefined}
                data-testid="inbox-row"
                data-status={conversation.counterpart.status}
                className={cn(
                  "conversation-row flex w-full cursor-pointer items-center gap-3 rounded-lg p-3 text-left transition-colors focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
                  isActive ? "bg-sage" : "bg-transparent hover:bg-sage/40",
                )}
              >
                <span className="relative shrink-0">
                  <ReaderAvatar initial={shownName.charAt(0).toUpperCase()} image={conversation.counterpart.avatar_url} />
                  {online !== null ? <span className="presence-dot presence-dot-avatar" data-online={online || undefined} aria-hidden /> : null}
                </span>
                <span className="flex min-w-0 flex-1 flex-col gap-0.5 overflow-hidden text-ink">
                  <span className="conversation-name block truncate font-serif text-base font-bold">
                    {shownName}
                    {advisorStatus ? ` · ${text(statusCopy[advisorStatus] ?? advisorStatus)}` : null}
                    {online !== null ? <span className="sr-only">{` · ${text(online ? "Online" : "Offline")}`}</span> : null}
                  </span>
                  <span className="conversation-preview block w-full truncate font-ui text-xs opacity-70">
                    {live === "requested" ? text("New chat request") : live === "active" ? text("Live now") : preview}
                  </span>
                </span>
                {conversation.unread > 0 ? (
                  <Badge
                    aria-label={t("chatroom.inbox.unread", { count: conversation.unread })}
                    className="h-4 shrink-0 rounded-sm bg-deep-black px-1.5 py-0.5 font-ui text-[10px] font-bold text-white"
                  >
                    {conversation.unread}
                  </Badge>
                ) : null}
              </button>
            </li>
          );
        })}
      </ul>
      <div className="px-3 pb-3" data-testid="inbox-footer">
        {moreError ? (
          <div className="rounded-lg bg-cream/60 p-3 text-xs" role="alert">
            <p>{text(errorMessage(moreError))}</p>
            <button type="button" className="mt-1 underline" onClick={onLoadMore}>
              {text("Try again")}
            </button>
          </div>
        ) : hasMore ? (
          <button
            type="button"
            className="w-full rounded-lg py-2 font-ui text-xs underline disabled:no-underline disabled:opacity-70"
            onClick={onLoadMore}
            disabled={loadingMore}
            aria-busy={loadingMore || undefined}
            data-testid="inbox-load-more"
          >
            {text(loadingMore ? "Loading more conversations…" : "Load more conversations")}
          </button>
        ) : null}
      </div>
    </div>
  );
}
