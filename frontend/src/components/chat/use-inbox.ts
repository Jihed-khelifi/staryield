"use client";

import * as React from "react";

import { chatApi } from "@/lib/api/endpoints";
import { toApiError, type ApiError } from "@/lib/api/errors";
import type { ChatMessage, Conversation } from "@/lib/api/types";

/** Conversations per inbox page. */
export const INBOX_PAGE_SIZE = 20;

export type InboxStatus = "idle" | "loading" | "ready" | "error";

export type Inbox = {
  items: Conversation[];
  status: InboxStatus;
  error: ApiError | null;
  hasMore: boolean;
  loadingMore: boolean;
  moreError: ApiError | null;
  /** Load the next page (infinite scroll). */
  loadMore: () => void;
  /** Refetch the first page and merge it (new conversation, resync). */
  refresh: () => void;
  /** Live update of the loaded rows (realtime events). */
  patch: (fn: (items: Conversation[]) => Conversation[]) => void;
};

const activity = (c: Conversation) => c.last_activity_at ?? c.last_message_at ?? c.created_at;

/** Newest activity first; ties by id (the API's keyset order). */
export function sortInbox(items: Conversation[]): Conversation[] {
  return [...items].sort((a, b) => activity(b).localeCompare(activity(a)) || b.id.localeCompare(a.id));
}

/** Merge a page into the loaded rows: the page's copy wins, no duplicates. */
export function mergeInbox(current: Conversation[], page: Conversation[]): Conversation[] {
  const byId = new Map(current.map((c) => [c.id, c]));
  for (const c of page) byId.set(c.id, c);
  return sortInbox([...byId.values()]);
}

/** A message arrived: refresh the row's preview, unread count and position. */
export function withMessage(items: Conversation[], message: ChatMessage, opts: { meId: string; activeId: string | null }): Conversation[] | null {
  const row = items.find((c) => c.id === message.conversation_id);
  if (!row) return null;
  const counts = message.sender_id !== opts.meId && row.id !== opts.activeId;
  const next: Conversation = {
    ...row,
    last_message: message,
    last_message_at: message.created_at,
    last_activity_at: message.created_at,
    last_seq: Math.max(row.last_seq ?? 0, message.seq ?? 0),
    unread: counts ? row.unread + 1 : row.id === opts.activeId ? 0 : row.unread,
  };
  return sortInbox(items.map((c) => (c.id === row.id ? next : c)));
}

type State = {
  items: Conversation[];
  cursor?: string;
  hasMore: boolean;
  pages: number;
  /** First-page request that last completed (or failed). */
  done: string;
  error: ApiError | null;
};

/**
 * The signed-in user's conversations, newest activity first, loaded one
 * cursor page at a time (infinite scroll). Realtime updates patch the
 * loaded rows; a conversation that is not loaded yet triggers `refresh`,
 * which merges the first page without dropping later pages.
 */
export function useInbox(enabled: boolean, { onFirstPage }: { onFirstPage?: (items: Conversation[]) => void } = {}): Inbox {
  const firstPage = React.useRef(onFirstPage);
  React.useEffect(() => {
    firstPage.current = onFirstPage;
  });
  const [state, setState] = React.useState<State>({ items: [], hasMore: false, pages: 0, done: "", error: null });
  const [more, setMore] = React.useState<{ loading: boolean; error: ApiError | null }>({ loading: false, error: null });
  const [nonce, setNonce] = React.useState(0);
  const key = enabled ? String(nonce) : "";

  React.useEffect(() => {
    if (!enabled) return;
    const request = String(nonce);
    const ctrl = new AbortController();
    chatApi.inbox(undefined, INBOX_PAGE_SIZE, ctrl.signal).then(
      (page) => {
        if (ctrl.signal.aborted) return;
        firstPage.current?.(page.items);
        setState((s) => ({
          items: mergeInbox(s.items, page.items),
          // Keep the frontier when later pages are loaded already.
          cursor: s.pages > 1 ? s.cursor : page.next_cursor,
          hasMore: s.pages > 1 ? s.hasMore : page.has_more,
          pages: Math.max(1, s.pages),
          done: request,
          error: null,
        }));
      },
      (err) => {
        const error = toApiError(err);
        if (ctrl.signal.aborted || error.code === "aborted") return;
        setState((s) => ({ ...s, done: request, error }));
      },
    );
    return () => ctrl.abort();
  }, [enabled, nonce]);

  const loadMore = React.useCallback(() => {
    if (!state.cursor || !state.hasMore || more.loading) return;
    const cursor = state.cursor;
    setMore({ loading: true, error: null });
    chatApi.inbox(cursor, INBOX_PAGE_SIZE).then(
      (page) => {
        setState((s) => (s.cursor !== cursor ? s : { ...s, items: mergeInbox(s.items, page.items), cursor: page.next_cursor, hasMore: page.has_more, pages: s.pages + 1 }));
        setMore({ loading: false, error: null });
      },
      (err) => setMore({ loading: false, error: toApiError(err) }),
    );
  }, [state.cursor, state.hasMore, more.loading]);

  const refresh = React.useCallback(() => setNonce((n) => n + 1), []);
  const patch = React.useCallback((fn: (items: Conversation[]) => Conversation[]) => setState((s) => {
    const items = fn(s.items);
    return items === s.items ? s : { ...s, items };
  }), []);

  // Loaded rows stay on screen while a refresh runs or after it fails.
  const failed = state.done === key ? state.error : null;
  const status: InboxStatus = !enabled ? "idle" : state.pages > 0 ? "ready" : failed ? "error" : "loading";
  return {
    items: state.items,
    status,
    error: failed,
    hasMore: state.hasMore,
    loadingMore: more.loading,
    moreError: more.error,
    loadMore,
    refresh,
    patch,
  };
}
