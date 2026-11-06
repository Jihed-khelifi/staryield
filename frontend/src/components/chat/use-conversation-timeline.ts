"use client";

import * as React from "react";

import { chatApi } from "@/lib/api/endpoints";
import { toApiError, type ApiError } from "@/lib/api/errors";
import type { ChatMessage } from "@/lib/api/types";
import { logger } from "@/lib/logger";
import {
  Backfill,
  EMPTY_TIMELINE,
  hasGap,
  isGapAfter,
  oldestSeq,
  withMessages,
  withNewestPage,
  withOlderPage,
  type Timeline,
} from "@/components/chat/chat-model";

/** Messages per history page. */
export const HISTORY_PAGE_SIZE = 50;
/** Gap-fill attempts after a failure before waiting for the next trigger. */
const MAX_FILL_RETRIES = 3;
const FILL_RETRY_MS = 4_000;

export type TimelineStatus = "idle" | "loading" | "ready" | "error";

export type ConversationTimeline = {
  timeline: Timeline;
  status: TimelineStatus;
  error: ApiError | null;
  loadingOlder: boolean;
  olderError: ApiError | null;
  /** Load the page before the oldest loaded message (`before_seq`). */
  loadOlder: () => Promise<void>;
  /** Merge confirmed messages (events, acks); fills a gap if one appears. */
  receive: (messages: ChatMessage[]) => void;
  /** Fetch whatever was missed (reconnect, resync, gap). */
  fillGaps: () => void;
  /** Reload the newest page from scratch. */
  reload: () => void;
};

type State = {
  /** Conversation the timeline belongs to. */
  conv: string | null;
  timeline: Timeline;
  /** Load request (conversation|nonce) that last completed or failed. */
  done: string;
  error: ApiError | null;
};

const INITIAL: State = { conv: null, timeline: EMPTY_TIMELINE, done: "", error: null };

/**
 * The confirmed history of one conversation, kept in strict `seq` order.
 * Events merge as they arrive; a seq past the contiguous head starts one
 * gap fill over REST (`after_seq`, following `has_more`), and so does a
 * reconnect (`fillGaps`). `onBackfilled` receives the messages a fill
 * brought in that were not on screen yet (for announcements).
 */
export function useConversationTimeline(
  conversationId: string | null,
  { onBackfilled }: { onBackfilled?: (fresh: ChatMessage[]) => void } = {},
): ConversationTimeline {
  const [state, setState] = React.useState<State>(INITIAL);
  const [older, setOlder] = React.useState<{ conv: string | null; loading: boolean; error: ApiError | null }>({ conv: null, loading: false, error: null });
  const [nonce, setNonce] = React.useState(0);
  const loadKey = `${conversationId ?? ""}|${nonce}`;

  const timeline = state.conv === conversationId ? state.timeline : EMPTY_TIMELINE;
  const status: TimelineStatus = !conversationId ? "idle" : state.done === loadKey ? (state.error ? "error" : "ready") : "loading";

  // The latest state, updated eagerly: the gap filler reads the head from it
  // between two fetches, before React has rendered the previous page.
  const stateRef = React.useRef<State>(INITIAL);
  const timelineRef = React.useRef(timeline);
  const convRef = React.useRef(conversationId);
  const backfilledRef = React.useRef(onBackfilled);
  React.useEffect(() => {
    convRef.current = conversationId;
    backfilledRef.current = onBackfilled;
    // Another conversation: its own timeline (empty until it loads).
    const s = stateRef.current;
    timelineRef.current = s.conv === conversationId ? s.timeline : EMPTY_TIMELINE;
  });

  const commit = React.useCallback((next: State) => {
    stateRef.current = next;
    timelineRef.current = next.conv === convRef.current ? next.timeline : EMPTY_TIMELINE;
    setState(next);
  }, []);

  /** Apply an update to a conversation's timeline (a new conversation starts empty). */
  const update = React.useCallback(
    (key: string, fn: (t: Timeline) => Timeline) => {
      const s = stateRef.current;
      const base = s.conv === key ? s : { conv: key, timeline: EMPTY_TIMELINE, done: "", error: null };
      const next = fn(base.timeline);
      if (next !== base.timeline || base !== s) commit({ ...base, timeline: next });
    },
    [commit],
  );

  // One gap filler per conversation load (created in an effect: it reads
  // the latest timeline through refs when it runs, never during render).
  const backfillRef = React.useRef<Backfill | null>(null);
  const failures = React.useRef(0);
  const retryTimer = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  React.useEffect(() => {
    if (!conversationId) return;
    const key = conversationId;
    const filler = new Backfill({
      fetchSince: (after, signal) => chatApi.since(key, after, 200, signal),
      head: () => (convRef.current === key ? timelineRef.current.headSeq : 0),
      apply: (items) => {
        const known = new Set(timelineRef.current.messages.map((m) => m.id));
        const fresh = items.filter((m) => !known.has(m.id));
        update(key, (t) => withMessages(t, items));
        if (fresh.length) backfilledRef.current?.(fresh);
      },
      // Too much was missed: start again from the newest page.
      onOverflow: () => setNonce((n) => n + 1),
      needed: () => hasGap(timelineRef.current),
    });
    backfillRef.current = filler;
    failures.current = 0;
    return () => {
      filler.stop();
      if (retryTimer.current) clearTimeout(retryTimer.current);
      if (backfillRef.current === filler) backfillRef.current = null;
    };
  }, [conversationId, loadKey, update]);

  // Load the newest page (conversation change or reload).
  React.useEffect(() => {
    if (!conversationId) return;
    const key = conversationId;
    const request = loadKey;
    const reloading = nonce > 0;
    const ctrl = new AbortController();
    chatApi.history(key, undefined, HISTORY_PAGE_SIZE, ctrl.signal).then(
      (page) => {
        if (ctrl.signal.aborted) return;
        const s = stateRef.current;
        // Events merged while loading are kept (they may be newer than the
        // page); a reload drops older pages and starts from this one.
        const base = s.conv === key ? s.timeline : EMPTY_TIMELINE;
        const head = page.items.at(-1)?.seq ?? 0;
        const start = reloading ? { ...EMPTY_TIMELINE, messages: base.messages.filter((m) => (m.seq ?? 0) > head) } : base;
        commit({ conv: key, timeline: withNewestPage(start, page), done: request, error: null });
      },
      (err) => {
        const e = toApiError(err);
        if (ctrl.signal.aborted || e.code === "aborted") return;
        const s = stateRef.current;
        commit({ conv: key, timeline: s.conv === key ? s.timeline : EMPTY_TIMELINE, done: request, error: e });
      },
    );
    return () => ctrl.abort();
  }, [conversationId, loadKey, nonce, commit]);

  /** Fill missed messages: `force` after a reconnect, else only a visible gap. */
  const requestFill = React.useCallback((force: boolean) => {
    const run = () => {
      const filler = backfillRef.current;
      if (!filler || !timelineRef.current.loaded) return;
      filler.request(force).then(
        () => {
          failures.current = 0;
        },
        (err) => {
          const e = toApiError(err);
          if (e.code === "aborted") return;
          logger.debug("chat: gap fill failed", { code: String(e.code) });
          if (++failures.current > MAX_FILL_RETRIES) return;
          if (retryTimer.current) clearTimeout(retryTimer.current);
          retryTimer.current = setTimeout(() => {
            if (hasGap(timelineRef.current)) run();
          }, FILL_RETRY_MS * failures.current);
        },
      );
    };
    run();
  }, []);
  const fillGaps = React.useCallback(() => requestFill(true), [requestFill]);

  const receive = React.useCallback(
    (messages: ChatMessage[]) => {
      const key = convRef.current;
      if (!key) return;
      const mine = messages.filter((m) => m.conversation_id === key);
      if (!mine.length) return;
      const gap = mine.some((m) => isGapAfter(timelineRef.current, m.seq));
      update(key, (t) => withMessages(t, mine));
      if (gap || hasGap(timelineRef.current)) requestFill(false);
    },
    [update, requestFill],
  );

  const olderLoading = older.conv === conversationId && older.loading;
  const loadOlder = React.useCallback(async () => {
    const key = convRef.current;
    const t = timelineRef.current;
    if (!key || !t.loaded || !t.hasOlder || olderLoading) return;
    setOlder({ conv: key, loading: true, error: null });
    try {
      const page = await chatApi.history(key, oldestSeq(t) || undefined, HISTORY_PAGE_SIZE);
      update(key, (cur) => withOlderPage(cur, page));
      setOlder({ conv: key, loading: false, error: null });
    } catch (err) {
      setOlder({ conv: key, loading: false, error: toApiError(err) });
    }
  }, [olderLoading, update]);

  const reload = React.useCallback(() => setNonce((n) => n + 1), []);

  return {
    timeline,
    status,
    error: status === "error" ? state.error : null,
    loadingOlder: olderLoading,
    olderError: older.conv === conversationId ? older.error : null,
    loadOlder,
    receive,
    fillGaps,
    reload,
  };
}
