"use client";

import * as React from "react";
import { FiArrowDown } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import type { ApiError } from "@/lib/api/errors";
import { MessageBubble, type Delivery } from "@/components/chat/message-bubble";
import { TypingBubble } from "@/components/chat/typing-bubble";
import type { UiMessage } from "@/components/chat/chat-model";
import { Skeleton } from "@/components/ui/skeleton";

/** Within this many pixels of the end counts as "at the latest message". */
const BOTTOM_SLACK_PX = 80;
/** Older history loads when scrolled this close to the top. */
const TOP_SLACK_PX = 120;

export type Announcement = { id: string; text: string };

const keyOf = (m: UiMessage) => (m.client_msg_id && m.pending ? m.client_msg_id : m.id);

function prefersReducedMotion() {
  return typeof window !== "undefined" && window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
}

export function ChatFeed({
  messages,
  meId,
  authorName,
  typing,
  savedIds,
  peerReadSeq = 0,
  status = "ready",
  hasOlder,
  loadingOlder,
  olderError,
  onLoadOlder,
  onBookmark,
  onRetry,
  onDiscard,
  connected = true,
  announcements = [],
  scrollToken = 0,
  onAtBottomChange,
  children,
}: {
  messages: UiMessage[];
  meId: string;
  authorName: string;
  typing: boolean;
  savedIds: Set<string>;
  /** The newest seq the other participant has read ("Seen"). */
  peerReadSeq?: number;
  /** Loading state of the newest history page. */
  status?: "idle" | "loading" | "ready" | "error";
  hasOlder: boolean;
  loadingOlder: boolean;
  olderError?: ApiError | null;
  onLoadOlder: () => void;
  onBookmark: (message: UiMessage) => void;
  onRetry: (message: UiMessage) => void;
  onDiscard?: (message: UiMessage) => void;
  /** The realtime connection is open (queued messages say so when not). */
  connected?: boolean;
  /** Incoming messages for screen readers (never the viewer's own). */
  announcements?: Announcement[];
  /** Bumped when the viewer sends: always scroll to the newest message. */
  scrollToken?: number;
  onAtBottomChange?: (atBottom: boolean) => void;
  children?: React.ReactNode;
}) {
  const { text, t } = useI18n();
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const contentRef = React.useRef<HTMLDivElement>(null);
  const [atBottom, setAtBottom] = React.useState(true);
  // Newest confirmed seq seen while at the bottom; incoming ones after it are "new".
  const [seenSeq, setSeenSeq] = React.useState(0);
  const atBottomRef = React.useRef(true);
  const geometry = React.useRef<{ firstKey?: string; height: number }>({ height: 0 });
  const firstKey = messages[0] ? keyOf(messages[0]) : undefined;
  let newest = 0;
  for (const m of messages) if (!m.pending && (m.seq ?? 0) > newest) newest = m.seq!;
  const newestRef = React.useRef(newest);
  React.useEffect(() => {
    newestRef.current = newest;
  });

  // Keep the reader's place: older pages are prepended without a jump, and
  // new messages scroll into view only while the reader is at the bottom. This
  // runs only when the messages change — never on unrelated re-renders (the 1 s
  // session timer, a draft being saved…) nor when the other person starts or
  // stops typing: the typing slot has a fixed height, so it never moves the feed.
  React.useLayoutEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const prev = geometry.current;
    if (prev.firstKey && firstKey !== prev.firstKey && messages.some((m) => keyOf(m) === prev.firstKey)) {
      el.scrollTop += el.scrollHeight - prev.height;
    } else if (atBottomRef.current) {
      el.scrollTop = el.scrollHeight;
    }
    geometry.current = { firstKey, height: el.scrollHeight };
    // `firstKey` is derived from `messages`.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [messages]);

  // The visible area (phone keyboard, rotation, composer growing) or the content
  // (an image finished loading) changed size: stay on the latest message if the reader was there.
  React.useEffect(() => {
    const el = scrollRef.current;
    const content = contentRef.current;
    if (!el || !content || typeof ResizeObserver === "undefined") return;
    let height = el.clientHeight;
    let contentHeight = content.offsetHeight;
    const ro = new ResizeObserver(() => {
      if (el.clientHeight === height && content.offsetHeight === contentHeight) return;
      height = el.clientHeight;
      contentHeight = content.offsetHeight;
      if (atBottomRef.current) el.scrollTop = el.scrollHeight;
      geometry.current = { ...geometry.current, height: el.scrollHeight };
    });
    ro.observe(el);
    ro.observe(content);
    return () => ro.disconnect();
  }, []);

  // The viewer sent something: show it, wherever they were.
  React.useEffect(() => {
    if (!scrollToken) return;
    const el = scrollRef.current;
    if (!el) return;
    atBottomRef.current = true;
    el.scrollTop = el.scrollHeight;
  }, [scrollToken]);

  const onScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    const bottom = el.scrollHeight - el.scrollTop - el.clientHeight <= BOTTOM_SLACK_PX;
    // At the bottom (or just leaving it) everything up to now was seen.
    if (bottom || atBottomRef.current) setSeenSeq(newestRef.current);
    atBottomRef.current = bottom;
    if (bottom !== atBottom) {
      setAtBottom(bottom);
      onAtBottomChange?.(bottom);
    }
    if (el.scrollTop <= TOP_SLACK_PX && hasOlder && !loadingOlder && !olderError) onLoadOlder();
  };

  const jumpToLatest = () => {
    const el = scrollRef.current;
    if (!el) return;
    atBottomRef.current = true;
    el.scrollTo({ top: el.scrollHeight, behavior: prefersReducedMotion() ? "auto" : "smooth" });
    setSeenSeq(newest);
    setAtBottom(true);
    onAtBottomChange?.(true);
    // The pill disappears: keep keyboard focus in the conversation.
    el.focus({ preventScroll: true });
  };

  // Incoming messages that arrived below the reader's position.
  const unseen = atBottom ? 0 : messages.filter((m) => !m.pending && (m.seq ?? 0) > seenSeq && m.sender_id !== meId && m.kind !== "system").length;

  // "Seen" goes on the newest of the viewer's messages the other side read.
  let receiptSeq = 0;
  for (const m of messages) if (m.sender_id === meId && !m.pending && (m.seq ?? 0) > 0 && m.seq! <= peerReadSeq) receiptSeq = m.seq!;

  const delivery = (m: UiMessage): Delivery | undefined => {
    if (!m.pending) return undefined;
    if (m.pending === "failed") return "failed";
    if (m.pending === "sending") return "sending";
    return connected ? (m.failure ? "retrying" : "sending") : "waiting";
  };

  return (
    <div className="chat-feed-frame relative flex min-h-0 flex-1 flex-col">
      <div
        ref={scrollRef}
        className="chat-feed-scroll min-h-0 flex-1 overflow-y-auto overscroll-contain focus-visible:outline-none"
        onScroll={onScroll}
        tabIndex={-1}
        data-testid="chat-feed"
        data-at-bottom={atBottom || undefined}
      >
        <div ref={contentRef} className="chat-feed-content">
        <p className="chat-session-notice">
          <span aria-hidden className="chat-session-notice-star">
            ✦
          </span>{" "}
          {text("Private session with {value0} · bookmarks save insights", { value0: authorName })}
        </p>
        {hasOlder ? (
          <div className="flex justify-center pt-3">
            <button
              type="button"
              className="chat-load-older font-ui text-xs underline disabled:no-underline disabled:opacity-70"
              onClick={onLoadOlder}
              disabled={loadingOlder}
              aria-busy={loadingOlder || undefined}
              data-testid="load-older"
            >
              {text(loadingOlder ? "Loading earlier messages…" : olderError ? "Couldn't load earlier messages. Try again" : "Load earlier messages")}
            </button>
          </div>
        ) : null}
        {status === "loading" && messages.length === 0 ? (
          <div className="flex flex-col gap-3 p-4" role="status" aria-label={text("Loading messages…")}>
            <Skeleton className="h-12 w-2/3 rounded-lg bg-cream/70" />
            <Skeleton className="ml-auto h-10 w-1/2 rounded-lg bg-cream/70" />
            <Skeleton className="h-16 w-3/4 rounded-lg bg-cream/70" />
          </div>
        ) : null}
        <ol aria-label={t("chatroom.feed.label", { name: authorName })} className="chat-feed-list flex flex-col gap-3 p-4">
          {messages.map((message) => (
            <MessageBubble
              key={keyOf(message)}
              message={message}
              mine={message.sender_id === meId}
              authorName={authorName}
              saved={savedIds.has(message.id)}
              seen={receiptSeq > 0 && message.seq === receiptSeq}
              delivery={delivery(message)}
              onBookmark={() => onBookmark(message)}
              onRetry={() => onRetry(message)}
              onDiscard={onDiscard ? () => onDiscard(message) : undefined}
            />
          ))}
          {/* Always rendered with a fixed height: showing or hiding the indicator never moves the feed. */}
          <li className="chat-typing-slot" aria-hidden={typing ? undefined : true} data-testid="typing-slot">
            {typing ? <TypingBubble authorName={authorName} /> : null}
          </li>
        </ol>
        {children}
        </div>
      </div>
      {unseen > 0 ? (
        <button type="button" className="chat-jump-latest" onClick={jumpToLatest} data-testid="jump-to-latest">
          <FiArrowDown aria-hidden className="size-3.5" />
          <span>{text("Jump to latest")}</span>
          <span className="chat-jump-count">{text("{value0} new", { value0: unseen })}</span>
        </button>
      ) : null}
      {/* Screen readers hear incoming messages only — never the viewer's own or history loads. */}
      <div role="log" aria-live="polite" aria-relevant="additions" aria-label={text("New messages from {value0}", { value0: authorName })} className="sr-only" data-testid="chat-live-log">
        {announcements.map((a) => (
          <p key={a.id}>{a.text}</p>
        ))}
      </div>
    </div>
  );
}
