"use client";

/**
 * Unread chat messages for the header shortcut (docs/platform-extensions.md
 * §18). The total loads with the session and follows realtime `chat.unread`;
 * it is refetched after a reconnect that could not replay events, on
 * `resync_required` and when the page becomes visible. While the tab is
 * hidden the title carries "(n) ". Every failure is swallowed.
 */
import * as React from "react";

import { chatApi } from "@/lib/api/endpoints";
import { toApiError } from "@/lib/api/errors";
import type { ChatUnread } from "@/lib/api/types";
import { useRealtimeEvent, useSession } from "@/lib/auth/session-provider";
import { stripUnreadPrefix, withUnreadPrefix } from "@/lib/chat/unread-title";
import { logger } from "@/lib/logger";

type Value = { count: number; conversations: number };
const ZERO: Value = { count: 0, conversations: 0 };
const ChatUnreadContext = React.createContext<Value>(ZERO);

const num = (n: unknown) => Math.max(0, Math.floor(Number(n) || 0));

export function ChatUnreadProvider({ children }: { children: React.ReactNode }) {
  const { status, user } = useSession();
  const userId = status === "authenticated" ? (user?.id ?? null) : null;
  const [raw, setRaw] = React.useState<Value & { owner: string | null }>({ ...ZERO, owner: userId });
  // Another (or no) user always starts from zero (sign-out resets the count).
  if (raw.owner !== userId) setRaw({ ...ZERO, owner: userId });
  const value = React.useMemo<Value>(() => (raw.owner === userId && userId ? { count: raw.count, conversations: raw.conversations } : ZERO), [raw, userId]);
  const ownerRef = React.useRef(userId);
  React.useEffect(() => {
    ownerRef.current = userId;
  }, [userId]);

  const apply = React.useCallback((owner: string, d: Partial<ChatUnread> | null | undefined) => {
    if (owner !== ownerRef.current) return;
    setRaw((cur) => {
      const count = num(d?.count);
      const conversations = num(d?.conversations);
      return cur.owner === owner && cur.count === count && cur.conversations === conversations ? cur : { owner, count, conversations };
    });
  }, []);

  const fetchCount = React.useCallback(
    (signal?: AbortSignal) => {
      const owner = ownerRef.current;
      if (!owner) return;
      chatApi.unreadCount(signal).then(
        (r) => apply(owner, r),
        (err) => {
          const e = toApiError(err);
          if (e.code !== "aborted") logger.debug("chat unread failed", { code: String(e.code) });
        },
      );
    },
    [apply],
  );

  React.useEffect(() => {
    if (!userId) return;
    const ctrl = new AbortController();
    fetchCount(ctrl.signal);
    return () => ctrl.abort();
  }, [userId, fetchCount]);

  useRealtimeEvent<Partial<ChatUnread>>("chat.unread", (d) => {
    if (userId && d) apply(userId, d);
  });
  useRealtimeEvent("resync_required", () => fetchCount());
  useRealtimeEvent<{ reconnected?: boolean }>("hello", (d) => {
    if (d?.reconnected) fetchCount();
  });

  React.useEffect(() => {
    if (!userId) return;
    const onVisible = () => {
      if (document.visibilityState === "visible") fetchCount();
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => document.removeEventListener("visibilitychange", onVisible);
  }, [userId, fetchCount]);

  // Tab title: "(n) " while hidden, restored when visible.
  const count = value.count;
  React.useEffect(() => {
    const sync = () => {
      const hidden = document.visibilityState === "hidden";
      const next = hidden && count > 0 ? withUnreadPrefix(document.title, count) : stripUnreadPrefix(document.title);
      if (next !== document.title) document.title = next;
    };
    sync();
    document.addEventListener("visibilitychange", sync);
    return () => {
      document.removeEventListener("visibilitychange", sync);
      document.title = stripUnreadPrefix(document.title);
    };
  }, [count]);

  return <ChatUnreadContext.Provider value={value}>{children}</ChatUnreadContext.Provider>;
}

export function useChatUnread(): Value {
  return React.useContext(ChatUnreadContext);
}
