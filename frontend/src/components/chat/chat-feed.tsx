"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { MessageBubble } from "@/components/chat/message-bubble";
import { TypingBubble } from "@/components/chat/typing-bubble";
import type { UiMessage } from "@/components/chat/chat-model";
import { ScrollArea } from "@/components/ui/scroll-area";

export function ChatFeed({
  messages,
  meId,
  authorName,
  typing,
  savedIds,
  seenUpTo,
  hasMore,
  loadingMore,
  onLoadMore,
  onBookmark,
  onRetry,
  children,
}: {
  messages: UiMessage[];
  meId: string;
  authorName: string;
  typing: boolean;
  savedIds: Set<string>;
  /** Created-at of the newest message the other side has read. */
  seenUpTo?: string;
  hasMore: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
  onBookmark: (message: UiMessage) => void;
  onRetry: (message: UiMessage) => void;
  children?: React.ReactNode;
}) {
  const { text, t } = useI18n();
  const endRef = React.useRef<HTMLDivElement>(null);
  const lastId = messages.at(-1)?.id;

  React.useEffect(() => {
    endRef.current?.scrollIntoView({ block: "end" });
  }, [lastId, typing]);

  return (
    <ScrollArea className="min-h-0 flex-1">
      <p className="chat-session-notice">
        {text("✦ Private session with ")}
        {text(authorName)} {text("· bookmarks save insights ")}
      </p>
      {hasMore ? (
        <div className="flex justify-center pt-3">
          <button type="button" className="font-ui text-xs underline" onClick={onLoadMore} disabled={loadingMore}>
            {text(loadingMore ? "Loading…" : "Load earlier messages")}
          </button>
        </div>
      ) : null}
      <ol aria-label={t("chatroom.feed.label", { name: authorName })} aria-live="polite" className="flex flex-col gap-3 p-4">
        {messages.map((message) => (
          <MessageBubble
            key={message.client_msg_id && message.pending ? message.client_msg_id : message.id}
            message={message}
            mine={message.sender_id === meId}
            authorName={authorName}
            saved={savedIds.has(message.id)}
            seen={Boolean(seenUpTo && message.created_at <= seenUpTo)}
            onBookmark={() => onBookmark(message)}
            onRetry={() => onRetry(message)}
          />
        ))}
        {typing ? <TypingBubble authorName={authorName} /> : null}
      </ol>
      {children}
      <div ref={endRef} />
    </ScrollArea>
  );
}
