"use client";

/**
 * In-app notifications (docs/platform-extensions.md §7.1).
 *
 * The unread count loads with the session; the list loads the first time the
 * bell opens and pages with `next_cursor`. New notifications arrive over
 * realtime (`notification.created`) and show a toast. After a reconnect that
 * could not replay missed events the count (and the loaded list) are
 * refetched. Every failure is swallowed: notifications never block the UI.
 */
import * as React from "react";
import { usePathname, useRouter } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import { notificationsApi } from "@/lib/api/endpoints";
import { toApiError, type ApiError } from "@/lib/api/errors";
import type { AppNotification } from "@/lib/api/types";
import { useRealtimeEvent, useSession } from "@/lib/auth/session-provider";
import { logger } from "@/lib/logger";
import { notificationHref, notificationText } from "@/lib/notifications/notification-copy";
import { useToast } from "@/components/feedback/toast-provider";

type NotificationsContextValue = {
  /** False when signed out or when the API does not offer notifications. */
  enabled: boolean;
  items: AppNotification[];
  unread: number;
  loaded: boolean;
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  error: ApiError | null;
  /** Load the first page if it has not been loaded yet. */
  ensureLoaded: () => void;
  loadMore: () => Promise<void>;
  refresh: () => void;
  markRead: (id: string) => Promise<void>;
  markAllRead: () => Promise<void>;
  /** Mark read and navigate to the notification's link. */
  open: (notification: AppNotification) => void;
};

const NotificationsContext = React.createContext<NotificationsContextValue | null>(null);

type State = {
  items: AppNotification[];
  cursor?: string;
  unread: number;
  loaded: boolean;
  loading: boolean;
  loadingMore: boolean;
  error: ApiError | null;
  disabled: boolean;
};

const INITIAL: State = { items: [], unread: 0, loaded: false, loading: false, loadingMore: false, error: null, disabled: false };

function unsupported(error: ApiError): boolean {
  return error.code === "not_found" || error.code === "feature_unavailable";
}

function quiet(error: unknown, what: string): ApiError {
  const e = toApiError(error);
  if (e.code !== "aborted") logger.debug(`notifications: ${what} failed`, { code: String(e.code), requestId: e.requestId });
  return e;
}

type OwnedState = State & { owner: string | null };

export function NotificationsProvider({ children }: { children: React.ReactNode }) {
  const { status, user } = useSession();
  const userId = status === "authenticated" ? (user?.id ?? null) : null;
  const { text, locale } = useI18n();
  const toast = useToast();
  const router = useRouter();
  const pathname = usePathname();
  const [raw, setRaw] = React.useState<OwnedState>({ ...INITIAL, owner: userId });
  // State belongs to one user: another (or no) user sees a fresh state.
  const state = React.useMemo<OwnedState>(() => (raw.owner === userId ? raw : { ...INITIAL, owner: userId }), [raw, userId]);
  const stateRef = React.useRef(state);
  const ownerRef = React.useRef(userId);
  const pathRef = React.useRef(pathname);
  const listInFlight = React.useRef(false);
  const seen = React.useRef(new Set<string>());
  React.useEffect(() => {
    stateRef.current = state;
    pathRef.current = pathname;
  });
  React.useEffect(() => {
    ownerRef.current = userId;
    seen.current = new Set();
    listInFlight.current = false;
  }, [userId]);

  /** Apply a change for `owner` (ignored when the signed-in user changed meanwhile). */
  const update = React.useCallback((owner: string | null, fn: (s: State) => State) => {
    if (owner !== ownerRef.current) return;
    setRaw((r) => {
      const base: OwnedState = r.owner === owner ? r : { ...INITIAL, owner };
      return { ...fn(base), owner };
    });
  }, []);

  const fetchCount = React.useCallback(
    (signal?: AbortSignal) => {
      const owner = ownerRef.current;
      notificationsApi.unreadCount(signal).then(
        (r) => update(owner, (s) => ({ ...s, unread: Math.max(0, Number(r?.count) || 0) })),
        (err) => {
          const e = quiet(err, "unread count");
          if (unsupported(e)) update(owner, (s) => ({ ...s, disabled: true }));
        },
      );
    },
    [update],
  );

  const fetchFirstPage = React.useCallback(() => {
    if (listInFlight.current) return;
    const owner = ownerRef.current;
    listInFlight.current = true;
    update(owner, (s) => ({ ...s, loading: true, error: null }));
    notificationsApi
      .list()
      .then(
        (page) => {
          if (owner !== ownerRef.current) return;
          page.items.forEach((n) => seen.current.add(n.id));
          update(owner, (s) => ({ ...s, items: page.items, cursor: page.next_cursor, loaded: true, loading: false, error: null }));
        },
        (err) => {
          const e = quiet(err, "list");
          update(owner, (s) => ({ ...s, loading: false, error: e, disabled: s.disabled || unsupported(e) }));
        },
      )
      .finally(() => {
        listInFlight.current = false;
      });
  }, [update]);

  React.useEffect(() => {
    if (!userId) return;
    const ctrl = new AbortController();
    fetchCount(ctrl.signal);
    return () => ctrl.abort();
  }, [userId, fetchCount]);

  const refresh = React.useCallback(() => {
    if (!userId || stateRef.current.disabled) return;
    fetchCount();
    if (stateRef.current.loaded) fetchFirstPage();
  }, [userId, fetchCount, fetchFirstPage]);

  const ensureLoaded = React.useCallback(() => {
    const s = stateRef.current;
    if (!userId || s.loaded || s.disabled) return;
    fetchFirstPage();
  }, [userId, fetchFirstPage]);

  const loadMore = React.useCallback(async () => {
    const { cursor, loadingMore } = stateRef.current;
    if (!cursor || loadingMore) return;
    const owner = ownerRef.current;
    update(owner, (s) => ({ ...s, loadingMore: true }));
    try {
      const page = await notificationsApi.list(cursor);
      if (owner !== ownerRef.current) return;
      const fresh = page.items.filter((n) => !seen.current.has(n.id));
      fresh.forEach((n) => seen.current.add(n.id));
      update(owner, (s) => ({ ...s, items: [...s.items, ...fresh], cursor: page.next_cursor, loadingMore: false }));
    } catch (err) {
      const e = quiet(err, "load more");
      update(owner, (s) => ({ ...s, loadingMore: false, error: e }));
    }
  }, [update]);

  const markRead = React.useCallback(
    async (id: string) => {
      const target = stateRef.current.items.find((n) => n.id === id);
      if (target?.read_at) return;
      const owner = ownerRef.current;
      const now = new Date().toISOString();
      update(owner, (s) => ({
        ...s,
        items: s.items.map((n) => (n.id === id ? { ...n, read_at: now } : n)),
        unread: target ? Math.max(0, s.unread - 1) : s.unread,
      }));
      try {
        const r = await notificationsApi.markRead([id]);
        if (typeof r?.unread === "number") update(owner, (s) => ({ ...s, unread: Math.max(0, r.unread) }));
      } catch (err) {
        quiet(err, "mark read");
      }
    },
    [update],
  );

  const markAllRead = React.useCallback(async () => {
    const owner = ownerRef.current;
    const now = new Date().toISOString();
    update(owner, (s) => ({ ...s, unread: 0, items: s.items.map((n) => (n.read_at ? n : { ...n, read_at: now })) }));
    try {
      const r = await notificationsApi.markAllRead();
      if (typeof r?.unread === "number") update(owner, (s) => ({ ...s, unread: Math.max(0, r.unread) }));
    } catch (err) {
      quiet(err, "mark all read");
      fetchCount();
    }
  }, [fetchCount, update]);

  const open = React.useCallback(
    (n: AppNotification) => {
      if (!n.read_at) void markRead(n.id);
      const href = notificationHref(n.link, locale);
      if (href) router.push(href);
    },
    [locale, markRead, router],
  );

  // --- realtime --------------------------------------------------------------
  useRealtimeEvent<{ notification: AppNotification }>("notification.created", (data) => {
    const n = data?.notification;
    if (!userId || !n?.id || seen.current.has(n.id)) return;
    seen.current.add(n.id);
    update(userId, (s) => ({ ...s, items: s.loaded ? [n, ...s.items] : s.items, unread: n.read_at ? s.unread : s.unread + 1 }));
    // The chatroom already announces chat requests itself.
    if (n.kind === "chat.requested" && pathRef.current?.includes("/chatroom")) return;
    const copy = notificationText(n, locale, text);
    const href = notificationHref(n.link, locale);
    toast.show({
      tone: "info",
      message: copy.body ? `${copy.title} — ${copy.body}` : copy.title,
      actionLabel: href ? text("View") : undefined,
      onAction: href ? () => open(n) : undefined,
    });
  });
  useRealtimeEvent("resync_required", refresh);
  useRealtimeEvent<{ reconnected?: boolean; resumed?: boolean }>("hello", (d) => {
    if (d?.reconnected && !d.resumed) refresh();
  });

  const value = React.useMemo<NotificationsContextValue>(
    () => ({
      enabled: Boolean(userId) && !state.disabled,
      items: state.items,
      unread: state.unread,
      loaded: state.loaded,
      loading: state.loading,
      loadingMore: state.loadingMore,
      hasMore: Boolean(state.cursor),
      error: state.error,
      ensureLoaded,
      loadMore,
      refresh,
      markRead,
      markAllRead,
      open,
    }),
    [userId, state, ensureLoaded, loadMore, refresh, markRead, markAllRead, open],
  );
  return <NotificationsContext.Provider value={value}>{children}</NotificationsContext.Provider>;
}

export function useNotifications(): NotificationsContextValue {
  const ctx = React.useContext(NotificationsContext);
  if (!ctx) throw new Error("useNotifications must be used inside <NotificationsProvider>");
  return ctx;
}
