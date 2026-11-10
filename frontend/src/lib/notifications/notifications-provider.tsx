"use client";

/**
 * In-app notifications (docs/platform-extensions.md §7.1, §23.6).
 *
 * The unread count loads with the session; the list loads the first time the
 * bell opens and pages with `next_cursor`. New notifications arrive over
 * realtime (`notification.created`, de-duplicated by id) and show a toast;
 * `notification.read` (another tab or device marked some or all as read) sets
 * the count and marks the listed rows. The count is refetched on every
 * realtime `hello` (first connection and reconnects), on `resync_required`
 * and when the page becomes visible (throttled). A sequence guard drops a
 * count response that started before a newer event or local change, so a slow
 * reply can never overwrite fresher state. Marking read is optimistic with a
 * rollback, an error toast and a retry. A push notification opened with
 * `?sy_n=<id>` (or the service worker's `notification.opened` message) is
 * marked read and the parameter is removed. The app badge is cleared while
 * the app is visible and when nothing is unread. Server state lives here;
 * every failure is swallowed: notifications never block the UI.
 */
import * as React from "react";
import { usePathname, useRouter } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import { notificationsApi } from "@/lib/api/endpoints";
import { toApiError, type ApiError } from "@/lib/api/errors";
import type { AppNotification, NotificationReadEvent } from "@/lib/api/types";
import { useRealtimeEvent, useSession } from "@/lib/auth/session-provider";
import { logger } from "@/lib/logger";
import { notificationDestination, notificationText } from "@/lib/notifications/notification-copy";
import { announce } from "@/components/feedback/live-region";
import { useToast } from "@/components/feedback/toast-provider";

/** Query parameter the service worker adds to the URL of a notification that was opened. */
export const OPENED_NOTIFICATION_PARAM = "sy_n";
/** Visibility refetches closer together than this are skipped. */
export const VISIBLE_REFETCH_MIN_MS = 5000;
const NOTIFICATION_ID = /^[A-Za-z0-9_-]{1,64}$/;

type BadgeNavigator = Navigator & { clearAppBadge?: () => Promise<void> };
function clearAppBadge() {
  try {
    void (navigator as BadgeNavigator).clearAppBadge?.()?.catch(() => undefined);
  } catch {
    /* Badging API unavailable */
  }
}

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
  // Bumped by every event or local change that sets the unread count: a count
  // response that started before the latest bump is stale and dropped.
  const countSeq = React.useRef(0);
  const lastVisibleFetch = React.useRef(0);
  React.useEffect(() => {
    stateRef.current = state;
    pathRef.current = pathname;
  });
  React.useEffect(() => {
    ownerRef.current = userId;
    seen.current = new Set();
    listInFlight.current = false;
    countSeq.current = 0;
    lastVisibleFetch.current = 0;
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
      if (!owner) return;
      const startedAt = countSeq.current;
      notificationsApi.unreadCount(signal).then(
        (r) => {
          // A newer event or local change arrived while this was in flight.
          if (countSeq.current !== startedAt) return;
          update(owner, (s) => ({ ...s, unread: Math.max(0, Math.floor(Number(r?.count)) || 0) }));
        },
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
    // A list that failed to load (error, not loaded) is retried too.
    if (stateRef.current.loaded || stateRef.current.error) fetchFirstPage();
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

  // The toast's "Try again" calls the latest versions of the mark-read functions.
  const retryRef = React.useRef<{ markRead: (id: string) => Promise<void>; markAllRead: () => Promise<void> }>({
    markRead: async () => undefined,
    markAllRead: async () => undefined,
  });

  const setUnread = React.useCallback(
    (owner: string | null, unread: number) => {
      countSeq.current++;
      update(owner, (s) => ({ ...s, unread: Math.max(0, Math.floor(unread) || 0) }));
    },
    [update],
  );

  const markRead = React.useCallback(
    async (id: string) => {
      const target = stateRef.current.items.find((n) => n.id === id);
      if (target?.read_at) return;
      const owner = ownerRef.current;
      if (!owner) return;
      const now = new Date().toISOString();
      countSeq.current++;
      update(owner, (s) => ({
        ...s,
        items: s.items.map((n) => (n.id === id ? { ...n, read_at: now } : n)),
        unread: target ? Math.max(0, s.unread - 1) : s.unread,
      }));
      try {
        const r = await notificationsApi.markRead([id]);
        if (typeof r?.unread === "number") setUnread(owner, r.unread);
      } catch (err) {
        quiet(err, "mark read");
        // Roll back what this call changed (only while nothing else touched the row).
        countSeq.current++;
        update(owner, (s) => ({
          ...s,
          items: s.items.map((n) => (n.id === id && n.read_at === now ? { ...n, read_at: undefined } : n)),
          unread: target && s.items.some((n) => n.id === id && n.read_at === now) ? s.unread + 1 : s.unread,
        }));
        if (!target) fetchCount();
        toast.show({
          tone: "error",
          message: text("We couldn't mark that notification as read."),
          actionLabel: text("Try again"),
          onAction: () => void retryRef.current.markRead(id),
        });
      }
    },
    [update, setUnread, fetchCount, toast, text],
  );

  const markAllRead = React.useCallback(async () => {
    const owner = ownerRef.current;
    if (!owner) return;
    const before = stateRef.current;
    const now = new Date().toISOString();
    const flipped = new Set(before.items.filter((n) => !n.read_at).map((n) => n.id));
    countSeq.current++;
    update(owner, (s) => ({ ...s, unread: 0, items: s.items.map((n) => (n.read_at ? n : { ...n, read_at: now })) }));
    try {
      const r = await notificationsApi.markAllRead();
      setUnread(owner, typeof r?.unread === "number" ? r.unread : 0);
      if (before.unread > 0) announce(text("All notifications marked as read."));
    } catch (err) {
      quiet(err, "mark all read");
      countSeq.current++;
      update(owner, (s) => ({
        ...s,
        unread: Math.max(s.unread, before.unread),
        items: s.items.map((n) => (flipped.has(n.id) && n.read_at === now ? { ...n, read_at: undefined } : n)),
      }));
      fetchCount();
      announce(text("We couldn't mark your notifications as read."));
      toast.show({
        tone: "error",
        message: text("We couldn't mark your notifications as read."),
        actionLabel: text("Try again"),
        onAction: () => void retryRef.current.markAllRead(),
      });
    }
  }, [fetchCount, update, setUnread, toast, text]);

  React.useEffect(() => {
    retryRef.current = { markRead, markAllRead };
  }, [markRead, markAllRead]);

  const open = React.useCallback(
    (n: AppNotification) => {
      if (!n.read_at) void markRead(n.id);
      const href = notificationDestination(n, locale);
      if (href) router.push(href);
    },
    [locale, markRead, router],
  );

  // --- realtime --------------------------------------------------------------
  useRealtimeEvent<{ notification: AppNotification }>("notification.created", (data) => {
    const n = data?.notification;
    if (!userId || !n?.id || seen.current.has(n.id)) return;
    seen.current.add(n.id);
    countSeq.current++;
    update(userId, (s) => ({ ...s, items: s.loaded ? [n, ...s.items] : s.items, unread: n.read_at ? s.unread : s.unread + 1 }));
    // The chatroom already announces chat requests itself.
    if (n.kind === "chat.requested" && pathRef.current?.includes("/chatroom")) return;
    const copy = notificationText(n, locale, text);
    const href = notificationDestination(n, locale);
    toast.show({
      tone: "info",
      message: copy.body ? `${copy.title} — ${copy.body}` : copy.title,
      actionLabel: href ? text("View") : undefined,
      onAction: href ? () => open(n) : undefined,
    });
  });
  // Read state changed in another tab or device (or by this tab's own request).
  useRealtimeEvent<NotificationReadEvent>("notification.read", (d) => {
    if (!userId || !d || typeof d.unread !== "number") return;
    countSeq.current++;
    const ids = new Set(Array.isArray(d.ids) ? d.ids.filter((id): id is string => typeof id === "string") : []);
    const now = new Date().toISOString();
    update(userId, (s) => ({
      ...s,
      unread: Math.max(0, Math.floor(d.unread) || 0),
      items: d.all === true ? s.items.map((n) => (n.read_at ? n : { ...n, read_at: now })) : ids.size ? s.items.map((n) => (ids.has(n.id) && !n.read_at ? { ...n, read_at: now } : n)) : s.items,
    }));
  });
  // The first hello too: a notification stored between the initial REST load
  // and the subscription would otherwise stay uncounted. After a reconnect that
  // could not replay (resync_required) the loaded list is refreshed as well.
  useRealtimeEvent<{ reconnected?: boolean; resumed?: boolean }>("hello", (d) => {
    if (!userId || stateRef.current.disabled) return;
    fetchCount();
    if (d?.reconnected && !d.resumed && stateRef.current.loaded) fetchFirstPage();
  });
  useRealtimeEvent("resync_required", refresh);

  // Back on the page: refetch the count (throttled) and clear the app badge.
  React.useEffect(() => {
    if (!userId) return;
    const onVisible = () => {
      if (document.visibilityState !== "visible") return;
      clearAppBadge();
      const now = Date.now();
      if (stateRef.current.disabled || now - lastVisibleFetch.current < VISIBLE_REFETCH_MIN_MS) return;
      lastVisibleFetch.current = now;
      fetchCount();
    };
    // The count itself loads with the session; here only the badge is cleared.
    if (document.visibilityState === "visible") clearAppBadge();
    document.addEventListener("visibilitychange", onVisible);
    return () => document.removeEventListener("visibilitychange", onVisible);
  }, [userId, fetchCount]);

  // Nothing unread: nothing to badge.
  React.useEffect(() => {
    if (userId && state.unread === 0) clearAppBadge();
  }, [userId, state.unread]);

  // A push notification was opened: `?sy_n=<id>` on load, or the service
  // worker's message when this tab was already open. Mark it read, then drop
  // the parameter so a reload does not repeat it.
  const openedRef = React.useRef(markRead);
  React.useEffect(() => {
    openedRef.current = markRead;
  });
  React.useEffect(() => {
    if (!userId) return;
    const handle = () => {
      let id: string | null = null;
      try {
        id = new URL(window.location.href).searchParams.get(OPENED_NOTIFICATION_PARAM);
      } catch {
        return;
      }
      if (id === null) return;
      if (NOTIFICATION_ID.test(id)) void openedRef.current(id);
      try {
        const url = new URL(window.location.href);
        url.searchParams.delete(OPENED_NOTIFICATION_PARAM);
        window.history.replaceState(window.history.state, "", `${url.pathname}${url.search}${url.hash}`);
      } catch {
        /* history unavailable */
      }
    };
    handle();
    const onMessage = (event: MessageEvent) => {
      const d = event.data as { type?: unknown; id?: unknown } | null;
      if (d?.type === "notification.opened" && typeof d.id === "string" && NOTIFICATION_ID.test(d.id)) void openedRef.current(d.id);
    };
    const sw = typeof navigator !== "undefined" ? navigator.serviceWorker : undefined;
    sw?.addEventListener?.("message", onMessage);
    return () => sw?.removeEventListener?.("message", onMessage);
  }, [userId]);

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
