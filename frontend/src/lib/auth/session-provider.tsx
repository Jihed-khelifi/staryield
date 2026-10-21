"use client";

/**
 * Session context: the signed-in user (from the server — never trusted from
 * local storage), the live wallet, and the realtime connection's lifecycle.
 */
import * as React from "react";
import { usePathname } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import { onSessionExpired, setAuthHint } from "@/lib/api/client";
import { authApi, meApi } from "@/lib/api/endpoints";
import { ApiError, toApiError } from "@/lib/api/errors";
import type { User, Wallet } from "@/lib/api/types";
import { track } from "@/lib/analytics";
import { logger } from "@/lib/logger";
import { RealtimeClient, type ConnectionState, type TransportKind } from "@/lib/realtime/client";
import { clearAttachmentCache } from "@/lib/chat/attachments";
import { useToast } from "@/components/feedback/toast-provider";

type Status = "loading" | "authenticated" | "anonymous";

type SessionContextValue = {
  status: Status;
  user: User | null;
  wallet: Wallet | null;
  /** Last error while loading the session (network outage, server error). */
  error: ApiError | null;
  realtime: RealtimeClient | null;
  connection: { state: ConnectionState; transport: TransportKind };
  setUser: (user: User) => void;
  setWallet: (wallet: Wallet) => void;
  reload: () => Promise<User | null>;
  /** Revoke the session (or every session), clear client state and go to the locale home page. */
  signOut: (opts?: { everywhere?: boolean }) => Promise<void>;
};

const SessionContext = React.createContext<SessionContextValue | null>(null);
const IDLE = { state: "idle", transport: "ws" } as const;

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const { text, locale } = useI18n();
  const toast = useToast();
  const pathname = usePathname();
  const [status, setStatus] = React.useState<Status>("loading");
  const [user, setUserState] = React.useState<User | null>(null);
  const [wallet, setWallet] = React.useState<Wallet | null>(null);
  const [error, setError] = React.useState<ApiError | null>(null);

  const [connection, setConnection] = React.useState<{ state: ConnectionState; transport: TransportKind }>({ state: "idle", transport: "ws" });
  // While signing out, revocation events are expected: no "signed out" toast.
  const signingOut = React.useRef(false);

  const setUser = React.useCallback((u: User) => {
    setUserState(u);
    setWallet(u.wallet ?? null);
    setStatus("authenticated");
    setError(null);
    setAuthHint(true);
  }, []);

  const clear = React.useCallback(() => {
    setUserState(null);
    setWallet(null);
    setStatus("anonymous");
  }, []);

  const reload = React.useCallback(async () => {
    try {
      const me = await meApi.get();
      setUser(me);
      return me;
    } catch (raw) {
      const err = toApiError(raw);
      if (err.code === "unauthenticated" || err.code === "session_expired" || err.code === "account_disabled") {
        setAuthHint(false);
        clear();
        if (err.code === "account_disabled") toast.show({ tone: "error", message: text("This account has been deactivated. Please contact support.") });
        return null;
      }
      logger.warn("could not load the session", err, { code: String(err.code), requestId: err.requestId });
      setError(err);
      setStatus((s) => (s === "loading" ? "anonymous" : s));
      return null;
    }
  }, [clear, setUser, text, toast]);

  React.useEffect(() => {
    const ctrl = new AbortController();
    meApi.get(ctrl.signal).then(
      (me) => setUser(me),
      (raw) => {
        const err = toApiError(raw);
        if (err.code === "aborted") return;
        if (err.code === "unauthenticated" || err.code === "session_expired" || err.code === "account_disabled") {
          setAuthHint(false);
          clear();
          return;
        }
        logger.warn("could not load the session", err, { code: String(err.code), requestId: err.requestId });
        setError(err);
        setStatus("anonymous");
      },
    );
    return () => ctrl.abort();
  }, [clear, setUser]);

  // Refresh failed somewhere in the app: the session is gone.
  React.useEffect(
    () =>
      onSessionExpired(() => {
        if (status === "authenticated" && !signingOut.current) toast.show({ tone: "info", message: text("Your session has expired. Please sign in again.") });
        clear();
      }),
    [clear, status, text, toast],
  );

  // One realtime connection per signed-in user. The client object is inert
  // until start(); the effect owns its lifecycle.
  const userId = user?.id;
  const realtime = React.useMemo(() => (userId ? new RealtimeClient(userId) : null), [userId]);
  React.useEffect(() => {
    if (!realtime) return;
    const client = realtime;
    const offState = client.onState((state, transport) => {
      setConnection({ state, transport });
      if (transport === "sse" && state === "open") track("realtime_fallback");
    });
    const offWallet = client.on("wallet.updated", (e) => setWallet(e.data as Wallet));
    const offHello = client.on("hello", (e) => {
      if ((e.data as { reconnected?: boolean })?.reconnected) track("realtime_reconnected");
    });
    const offRevoked = client.on("auth.revoked", () => {
      if (signingOut.current) return;
      toast.show({ tone: "info", message: text("You were signed out. Please sign in again.") });
      setAuthHint(false);
      clear();
    });
    const offResync = client.on("resync_required", () => {
      void meApi.get().then((me) => setWallet(me.wallet ?? null)).catch(() => undefined);
    });
    client.start();
    return () => {
      offState();
      offWallet();
      offHello();
      offRevoked();
      offResync();
      client.stop();
    };
  }, [realtime, clear, text, toast]);

  // Page-view analytics.
  React.useEffect(() => {
    track("page_view");
  }, [pathname]);

  const signOut = React.useCallback(async (opts?: { everywhere?: boolean }) => {
    if (signingOut.current) return;
    signingOut.current = true;
    realtime?.stop();
    try {
      await (opts?.everywhere ? authApi.logoutAll() : authApi.logout());
    } catch (err) {
      logger.warn("logout request failed", err);
    } finally {
      setAuthHint(false);
      try {
        if (userId) sessionStorage.removeItem(`staryield.rt.last:${userId}`);
      } catch {
        // Storage unavailable: nothing was persisted.
      }
      clearAttachmentCache();
      // A full navigation (not router.push) drops every in-memory cache —
      // realtime, notifications, chat history — and lands signed out on the
      // home page, without racing pages that redirect anonymous visitors.
      // eslint-disable-next-line @next/next/no-location-assign-relative-destination
      window.location.assign(`/${locale}`);
    }
  }, [realtime, userId, locale]);

  const value = React.useMemo<SessionContextValue>(
    () => ({ status, user, wallet, error, realtime, connection: realtime ? connection : IDLE, setUser, setWallet, reload, signOut }),
    [status, user, wallet, error, realtime, connection, setUser, reload, signOut],
  );
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession(): SessionContextValue {
  const ctx = React.useContext(SessionContext);
  if (!ctx) throw new Error("useSession must be used inside <SessionProvider>");
  return ctx;
}

/** Subscribe to a realtime event type for the component's lifetime. */
export function useRealtimeEvent<T = unknown>(type: string, handler: (data: T, id?: string) => void) {
  const { realtime } = useSession();
  const ref = React.useRef(handler);
  React.useEffect(() => {
    ref.current = handler;
  });
  React.useEffect(() => {
    if (!realtime) return;
    return realtime.on(type, (e) => ref.current(e.data as T, e.id));
  }, [realtime, type]);
}

/** Subscribe one handler to several realtime event types. */
export function useRealtimeEvents<T = unknown>(types: readonly string[], handler: (type: string, data: T, id?: string) => void) {
  const { realtime } = useSession();
  const ref = React.useRef(handler);
  React.useEffect(() => {
    ref.current = handler;
  });
  const key = types.join(",");
  React.useEffect(() => {
    if (!realtime) return;
    const offs = key.split(",").map((type) => realtime.on(type, (e) => ref.current(e.type, e.data as T, e.id)));
    return () => offs.forEach((off) => off());
  }, [realtime, key]);
}
