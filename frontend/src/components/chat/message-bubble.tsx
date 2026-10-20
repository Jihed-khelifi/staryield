"use client";

import { useI18n } from "@/i18n/i18n-provider";
import { formatMessageTime } from "@/lib/format";
import { cn } from "@/lib/utils";
import { MessageBookmark } from "@/components/chat/message-bookmark";
import { endReasonCopy, systemCopy, type UiMessage } from "@/components/chat/chat-model";

export function MessageBubble({
  message,
  mine,
  authorName,
  saved = false,
  seen = false,
  onBookmark,
  onRetry,
}: {
  message: UiMessage;
  mine: boolean;
  authorName: string;
  saved?: boolean;
  seen?: boolean;
  onBookmark?: () => void;
  onRetry?: () => void;
}) {
  const { text, locale, t } = useI18n();

  if (message.kind === "system") {
    const reason = message.body.split(":")[1];
    return (
      <li className="chat-system-message" role="note">
        <span>{text(systemCopy(message.body))}</span>
        {reason && endReasonCopy[reason] ? <span className="block opacity-70">{text(endReasonCopy[reason])}</span> : null}
        <time dateTime={message.created_at} className="block opacity-60">
          {formatMessageTime(message.created_at, locale)}
        </time>
      </li>
    );
  }

  const bubble = (
    <div
      className={cn(
        "message-bubble flex w-fit max-w-[85%] flex-col gap-2 rounded-lg bg-cream p-3 text-ink sm:max-w-[380px]",
        message.pending === "sending" && "opacity-70",
        message.pending === "failed" && "ring-1 ring-red-800",
      )}
    >
      {/* Rendered as text: React escapes it, so message content can never inject markup. */}
      <p className="w-full font-serif text-[13px] leading-[1.4] break-words whitespace-pre-line">{message.body}</p>
      <span className="flex w-full items-center justify-end gap-2 font-ui text-[10px] opacity-60">
        {message.pending === "sending" ? <span>{text("Sending…")}</span> : null}
        {mine && !message.pending && seen ? <span>{text("Seen")}</span> : null}
        <time dateTime={message.created_at}>{formatMessageTime(message.created_at, locale)}</time>
      </span>
      {message.pending === "failed" ? (
        <button type="button" className="text-left font-ui text-[11px] text-red-800 underline" onClick={onRetry}>
          {text("Not delivered — tap to retry")}
        </button>
      ) : null}
    </div>
  );

  return (
    <li className={cn("flex w-full items-end gap-[5px]", mine && "justify-end")}>
      <span className="sr-only">{mine ? t("chatroom.feed.you") : text(authorName)}</span>
      {mine ? (
        <>
          {!message.pending ? <MessageBookmark saved={saved} onToggle={onBookmark} /> : null}
          {bubble}
        </>
      ) : (
        <>
          {bubble}
          <MessageBookmark saved={saved} onToggle={onBookmark} />
        </>
      )}
    </li>
  );
}
