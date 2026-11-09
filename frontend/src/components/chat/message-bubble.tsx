"use client";

import { FiAlertCircle, FiClock } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import { formatMessageTime } from "@/lib/format";
import { cn } from "@/lib/utils";
import { MessageBookmark } from "@/components/chat/message-bookmark";
import { MessageText } from "@/components/chat/message-text";
import { ChatImage } from "@/components/chat/chat-image";
import { endReasonCopy, systemCopy, type UiMessage } from "@/components/chat/chat-model";

/** Delivery state of one of the viewer's own messages. */
export type Delivery = "sending" | "waiting" | "retrying" | "failed";

/** Why a message could not be delivered (English source copy, translated). */
const failureCopy: Record<string, string> = {
  session_ended: "Not delivered: this chat has ended.",
  invalid_state: "Not delivered: this chat has ended.",
  rate_limited: "Not delivered: you are sending too fast.",
  validation_failed: "Not delivered: this message cannot be sent.",
  payload_too_large: "Not delivered: this message is too long.",
  off_session_limit_reached: "Not delivered: you've used today's messages outside a live chat.",
  off_session_disabled: "Not delivered: messages outside a live chat are turned off right now.",
  off_session_not_eligible: "Not delivered: you can write once the live chat starts.",
  recipient_unavailable: "Not delivered: this person can't receive messages right now.",
  network_error: "Not delivered: check your connection and try again.",
  timeout: "Not delivered: the connection was too slow. Try again.",
  unsupported_media_type: "Not delivered: this image type isn't supported.",
};
/** Image-specific wording for the codes that read differently for a photo. */
const imageFailureCopy: Record<string, string> = {
  payload_too_large: "Not delivered: this image is too large.",
  validation_failed: "Not delivered: this image cannot be sent.",
};

export function MessageBubble({
  message,
  mine,
  authorName,
  saved = false,
  seen = false,
  delivery,
  onBookmark,
  onRetry,
  onDiscard,
}: {
  message: UiMessage;
  mine: boolean;
  authorName: string;
  saved?: boolean;
  seen?: boolean;
  delivery?: Delivery;
  onBookmark?: () => void;
  onRetry?: () => void;
  onDiscard?: () => void;
}) {
  const { text, locale, t } = useI18n();

  if (message.kind === "system") {
    const reason = message.body.split(":")[1];
    return (
      <li className="chat-system-message" data-testid="chat-system-message" data-seq={message.seq || undefined}>
        <span>{text(systemCopy(message.body))}</span>
        {reason && endReasonCopy[reason] ? <span className="block opacity-70">{text(endReasonCopy[reason])}</span> : null}
        <time dateTime={message.created_at} className="block opacity-60">
          {formatMessageTime(message.created_at, locale)}
        </time>
      </li>
    );
  }

  const image = message.kind === "image";
  const attachment = message.attachments?.[0];
  const alt = message.body.trim() || (mine ? text("Image you sent") : text("Image from {value0}", { value0: authorName }));
  const status =
    delivery === "sending"
      ? text(image ? "Uploading…" : "Sending…")
      : delivery === "waiting"
        ? text("Waiting for connection…")
        : delivery === "retrying"
          ? text("Retrying…")
          : null;

  const bubble = (
    <div
      className={cn(
        "message-bubble flex w-fit max-w-[85%] flex-col gap-2 rounded-lg p-3 text-ink sm:max-w-[380px]",
        image && "message-bubble-image",
        (delivery === "sending" || delivery === "waiting" || delivery === "retrying") && "message-bubble-pending",
        delivery === "failed" && "message-bubble-failed",
      )}
      data-kind={message.kind}
      data-delivery={delivery}
    >
      {image ? <ChatImage attachment={attachment} file={message.localFile} alt={alt} caption={message.body || undefined} /> : null}
      {/* Rendered as text: React escapes it, so message content can never inject markup. */}
      {message.body ? <MessageText body={message.body} className="w-full font-serif text-[13px] leading-[1.4]" /> : null}
      <span className="message-meta flex w-full items-center justify-end gap-2 font-ui text-[10px]">
        {message.off_session ? (
          <span className="message-off-session mr-auto" data-testid="message-off-session">
            {text("Outside a live chat")}
          </span>
        ) : null}
        {status ? (
          <span className="inline-flex items-center gap-1" data-testid="message-status">
            {delivery === "waiting" ? <FiClock aria-hidden className="size-3" /> : null}
            {status}
          </span>
        ) : null}
        {mine && !delivery && seen ? <span data-testid="message-seen">{text("Seen")}</span> : null}
        <time dateTime={message.created_at}>{formatMessageTime(message.created_at, locale)}</time>
      </span>
      {delivery === "failed" ? (
        <span className="message-failure flex flex-wrap items-center gap-x-3 gap-y-1 font-ui text-[11px]" data-testid="message-failed">
          <span className="inline-flex items-center gap-1">
            <FiAlertCircle aria-hidden className="size-3.5" />
            {message.failureDetail
              ? `${text("Not delivered:")} ${message.failureDetail}`
              : text((image ? imageFailureCopy[message.failure ?? ""] : undefined) ?? failureCopy[message.failure ?? ""] ?? "Not delivered.")}
          </span>
          <button type="button" className="underline" onClick={onRetry}>
            {text("Retry")}
          </button>
          {onDiscard ? (
            <button type="button" className="underline" onClick={onDiscard}>
              {text("Remove")}
            </button>
          ) : null}
        </span>
      ) : null}
    </div>
  );

  return (
    <li className={cn("relative flex w-full items-end gap-[5px]", mine && "justify-end")} data-testid="chat-message" data-mine={mine || undefined} data-seq={message.seq || undefined}>
      <span className="sr-only">{mine ? t("chatroom.feed.you") : text(authorName)}</span>
      {mine ? (
        <>
          {!delivery ? <MessageBookmark saved={saved} onToggle={onBookmark} /> : null}
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
