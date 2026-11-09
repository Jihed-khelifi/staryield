"use client";

/**
 * Session context: the signed-in user (from the server — never trusted from
 * local storage), the live wallet, and the realtime connection's lifecycle.
 */
import * as React from "react";
import { usePathname } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import { endSession, onSessionExpired, onSignedOutElsewhere, readAuthHint, setAuthHint } from "@/lib/api/client";
import { authApi, meApi } from "@/lib/api/endpoints";
import { ApiError, toApiError } from "@/lib/api/errors";
import type { User, Wallet } from "@/lib/api/types";
import { track } from "@/lib/analytics";
import { logger } from "@/lib/logger";
import { RealtimeClient, type ConnectionState, type TransportKind } from "@/lib/realtime/client";
import { clearOutbox } from "@/lib/chat/outbox";
import { clearDrafts } from "@/lib/chat/drafts";
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
  /**
   * Revoke the session (or every session), clear client state and go to the
   * locale home page (or `to`, a same-locale path such as the login page).
   */
  signOut: (opts?: { everywhere?: boolean; to?: string }) => Promise<void>;
};

/** The session is definitively gone (not a network or server hiccup). */
export function isSessionEnd(code: string | number): boolean {
  return code === "unauthenticated" || code === "session_expired" || code === "account_disabled";
}

/** Two auth.revoked signals this close are one revocation (frame + 4001 close). */
export const REVOKED_DEDUPE_MS = 2000;

/** Retry delay for transient session reads: 2 s, 4 s, ... capped at 30 s. */
export function sessionRetryDelay(attempt: number): number {
  const base = Math.min(2000 * 2 ** attempt, 30_000);
  return base / 2 + Math.random() * (base / 2);
}

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
      if (isSessionEnd(err.code)) {
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
    let timer: number | undefined;
    let attempt = 0;
    const load = () => {
      meApi.get(ctrl.signal).then(
        (me) => setUser(me),
        (raw) => {
          const err = toApiError(raw);
          if (err.code === "aborted") return;
          if (isSessionEnd(err.code)) {
            setAuthHint(false);
            clear();
            return;
          }
          logger.warn("could not load the session", err, { code: String(err.code), requestId: err.requestId });
          setError(err);
          if (!readAuthHint()) {
            setStatus("anonymous");
            return;
          }
          // This browser has a session: a network or server hiccup must not
          // sign it out nor clear the hint. Keep loading and retry.
          timer = window.setTimeout(load, sessionRetryDelay(attempt++));
        },
      );
    };
    load();
    return () => {
      ctrl.abort();
      window.clearTimeout(timer);
    };
  }, [clear, setUser]);

  // Another tab signed out (or found the session gone): follow it.
  React.useEffect(
    () =>
      onSignedOutElsewhere(() => {
        if (!signingOut.current) clear();
      }),
    [clear],
  );

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
    const revocations: number[] = [];
    let lastRevoked = 0;
    let retry: number | undefined;
    let stopped = false;
    // Re-read the account after a revocation signal. Only a definitive 401
    // (after a refresh) signs out; a transient failure keeps the session and
    // tries again with backoff.
    const verify = (attempt: number) => {
      void meApi.get().then(
        (me) => {
          if (signingOut.current || stopped) return;
          setUser(me);
          client.start();
        },
        (raw: unknown) => {
          if (signingOut.current || stopped) return;
          const code = toApiError(raw).code;
          if (!isSessionEnd(code)) {
            if (attempt < 5) retry = window.setTimeout(() => verify(attempt + 1), sessionRetryDelay(attempt));
            return;
          }
          // A failed refresh has already announced the expired session.
          if (code !== "unauthenticated" && code !== "session_expired")
            toast.show({ tone: "info", message: text("You were signed out. Please sign in again.") });
          endSession();
          clear();
        },
      );
    };
    const offRevoked = client.on("auth.revoked", () => {
      if (signingOut.current) return;
      // The frame and the 4001 close of one revocation arrive back to back.
      const now = Date.now();
      if (now - lastRevoked < REVOKED_DEDUPE_MS) return;
      lastRevoked = now;
      // account settings: a password change revokes every connection, but this
      // browser already holds the new session — keep it when it still works
      // (at most a few times a minute, so a revoked session cannot loop).
      revocations.push(now);
      while (revocations.length && now - revocations[0] > 60_000) revocations.shift();
      if (revocations.length > 3) {
        toast.show({ tone: "info", message: text("You were signed out. Please sign in again.") });
        endSession();
        clear();
        return;
      }
      verify(0);
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
      stopped = true;
      window.clearTimeout(retry);
      client.stop();
    };
  }, [realtime, clear, setUser, text, toast]);

  // Page-view analytics.
  React.useEffect(() => {
    track("page_view");
  }, [pathname]);

  const signOut = React.useCallback(async (opts?: { everywhere?: boolean; to?: string }) => {
    if (signingOut.current) return;
    signingOut.current = true;
    realtime?.stop();
    try {
      await (opts?.everywhere ? authApi.logoutAll() : authApi.logout());
    } catch (err) {
      logger.warn("logout request failed", err);
    } finally {
      endSession();
      try {
        if (userId) sessionStorage.removeItem(`staryield.rt.last:${userId}`);
      } catch {
        // Storage unavailable: nothing was persisted.
      }
      // FE-CHAT (wave 3): unsent chat messages never outlive the session.
      if (userId) clearOutbox(userId);
      // revisions 3.0: composer drafts (private reading content) go with it.
      if (userId) clearDrafts(userId);
      clearAttachmentCache();
      // A full navigation (not router.push) drops every in-memory cache —
      // realtime, notifications, chat history — and lands signed out on the
      // home page, without racing pages that redirect anonymous visitors.
      // Only same-locale paths (never another origin) are accepted as `to`.
      const to = opts?.to && opts.to.startsWith(`/${locale}/`) && !opts.to.startsWith("//") ? opts.to : `/${locale}`;
      window.location.assign(to);
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
