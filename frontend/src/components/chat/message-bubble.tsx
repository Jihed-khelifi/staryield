"use client";

import * as React from "react";
import { FiAlertCircle, FiBookmark, FiClock, FiCornerUpLeft, FiHeart, FiImage } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import { formatMessageTime } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useBubbleGestures } from "@/lib/chat/gestures";
import { MessageMenu, MessageMenuTrigger, type MenuAnchor } from "@/components/chat/message-actions";
import { useMessageActions } from "@/components/chat/message-actions-context";
import { MessageText } from "@/components/chat/message-text";
import { ChatImage } from "@/components/chat/chat-image";
import { endReasonCopy, isActionable, systemCopy, type UiMessage } from "@/components/chat/chat-model";

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
  reply_target_invalid: "Not delivered: the message you replied to is no longer available.",
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
  const actions = useMessageActions();
  // The open menu: at the finger (long-press) or under the three-dot button.
  const [menu, setMenu] = React.useState<MenuAnchor | null>(null);
  const triggerRef = React.useRef<HTMLButtonElement>(null);
  const closeMenu = React.useCallback(() => setMenu(null), []);
  const openFromButton = () => {
    if (triggerRef.current) setMenu({ element: triggerRef.current, align: mine ? "start" : "end" });
  };
  // A click on the three-dot button toggles its menu (a second press closes it).
  const toggleFromButton = () => {
    const element = triggerRef.current;
    if (element) setMenu((current) => (current && "element" in current ? null : { element, align: mine ? "start" : "end" }));
  };
  const [pop, setPop] = React.useState(false);
  const likedBy = message.liked_by ?? [];
  const likedByMe = Boolean(actions.viewerId) && likedBy.includes(actions.viewerId);
  const actionable = actions.enabled && !delivery && isActionable(message);

  const like = () => {
    // The heart pops when it is given, not when it is taken back.
    if (!likedByMe) setPop(true);
    actions.toggleLike(message);
  };
  React.useEffect(() => {
    if (!pop) return;
    const timer = window.setTimeout(() => setPop(false), 700);
    return () => window.clearTimeout(timer);
  }, [pop]);
  const { attach: attachGestures, handlers: gestureHandlers } = useBubbleGestures({
    enabled: actionable,
    onReply: () => actions.reply(message),
    onLike: like,
    onMenu: setMenu,
  });

  if (message.kind === "system") {
    const reason = message.body.split(":")[1];
    return (
      <li className="chat-system-message" data-testid="chat-system-message" data-seq={message.seq || undefined}>
        <span>{text(systemCopy(message.body))}</span>
        {reason && endReasonCopy[reason] ? <span className="block opacity-70">{text(endReasonCopy[reason])}</span> : null}
        <time dateTime={message.created_at} className="block">
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

  const quote = message.reply_to;
  const quoteAuthor = quote
    ? quote.sender_id && quote.sender_id === actions.viewerId
      ? t("chatroom.feed.you")
      : text(actions.nameOf(quote.sender_id) || authorName)
    : "";
  const others = likedBy.filter((id) => id !== actions.viewerId);
  const otherName = others.length ? text(actions.nameOf(others[0]) || authorName) : "";
  const likeLabel =
    likedBy.length === 0
      ? ""
      : likedByMe && others.length
        ? text("You and {value0} liked this", { value0: otherName })
        : likedByMe
          ? text("You liked this")
          : text("{value0} liked this", { value0: otherName });

  const bubble = (
    <div
      ref={attachGestures}
      {...(actionable ? gestureHandlers : {})}
      className={cn(
        "message-bubble flex w-fit max-w-[85%] flex-col gap-2 rounded-lg p-3 text-ink sm:max-w-[380px]",
        image && "message-bubble-image",
        (delivery === "sending" || delivery === "waiting" || delivery === "retrying") && "message-bubble-pending",
        delivery === "failed" && "message-bubble-failed",
        likedBy.length > 0 && "message-bubble-liked",
      )}
      data-kind={message.kind}
      data-delivery={delivery}
      data-actionable={actionable || undefined}
      data-liked={likedBy.length > 0 || undefined}
    >
      {actionable ? (
        <span className="message-swipe-hint" aria-hidden>
          <FiCornerUpLeft className="size-3.5" />
        </span>
      ) : null}
      {actionable ? <MessageMenuTrigger ref={triggerRef} mine={mine} open={Boolean(menu && "element" in menu)} onOpen={openFromButton} onToggle={toggleFromButton} /> : null}
      {quote ? (
        <button
          type="button"
          className="message-quote"
          data-testid="message-quote"
          data-no-gesture
          aria-busy={actions.jumpingTo === quote.id || undefined}
          onClick={() => actions.jumpTo(quote)}
        >
          <span className="message-quote-author">{quoteAuthor}</span>
          <span className="message-quote-excerpt">
            {quote.kind === "image" ? <FiImage aria-hidden className="size-3 shrink-0" /> : null}
            <span className="truncate">{quote.excerpt || text("Photo")}</span>
          </span>
          {/* The visible author and excerpt name the button; the instruction follows them (WCAG 2.5.3). */}
          <span className="sr-only"> {text("Go to the original message")}</span>
        </button>
      ) : null}
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
        {saved && !delivery ? (
          <span className="message-saved-mark" data-testid="message-saved">
            <FiBookmark aria-hidden className="size-3" fill="currentColor" />
            <span className="sr-only">{text("Bookmarked")}</span>
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
      {likedBy.length > 0 ? (
        <span className="message-like-badge" role="img" aria-label={likeLabel} data-testid="message-like-badge" data-mine-liked={likedByMe || undefined}>
          <FiHeart aria-hidden className="size-3" />
        </span>
      ) : null}
      {pop ? (
        <span className="message-like-pop" aria-hidden data-testid="message-like-pop">
          <FiHeart className="size-9" />
        </span>
      ) : null}
    </div>
  );

  return (
    <li
      className={cn("message-row relative flex w-full items-end gap-[5px]", mine && "justify-end")}
      data-testid="chat-message"
      data-mine={mine || undefined}
      data-seq={message.seq || undefined}
      data-message-id={message.pending ? undefined : message.id}
    >
      <span className="sr-only">{mine ? t("chatroom.feed.you") : text(authorName)}</span>
      {bubble}
      {menu && actionable ? (
        <MessageMenu
          anchor={menu}
          liked={likedByMe}
          saved={saved}
          hasText={Boolean(message.body.trim())}
          copyText={message.body}
          onReply={() => actions.reply(message)}
          onLike={like}
          onBookmark={() => onBookmark?.()}
          onClose={closeMenu}
          returnFocusTo={"element" in menu ? menu.element : null}
        />
      ) : null}
    </li>
  );
}
