/**
 * HTTP client for the Staryield API.
 *
 * - Cookies carry the session (HttpOnly); `credentials: "include"`.
 * - Unsafe requests send the CSRF double-submit token (fetched once, rotated
 *   by the server, retried once when stale).
 * - An expired access token triggers a single-flight refresh (one per
 *   browser: Web Lock + BroadcastChannel across tabs), then the original
 *   request is retried once. Only a definitive 401 from the refresh endpoint
 *   ends the session; network errors, timeouts, 429 and 5xx leave it intact.
 * - Every request has a timeout and an X-Request-ID that appears in server
 *   logs and in error responses.
 * - Idempotent requests retry transient failures with exponential backoff,
 *   honouring Retry-After.
 * - Every failure is converted into an ApiError and logged.
 */
import { API_URL, currentLocale } from "@/lib/api/config";
import { ApiError, toApiError } from "@/lib/api/errors";
import { logger, setLogTransport } from "@/lib/logger";

export type Query = Record<string, string | number | boolean | undefined | null>;

export type RequestOptions = {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  query?: Query;
  signal?: AbortSignal;
  timeoutMs?: number;
  /** Retries for transient failures (GET defaults to 2, others to 0). */
  retries?: number;
  /** Allow retrying a non-GET request (it carries an idempotency key). */
  idempotent?: boolean;
  /** Do not try to refresh the session on 401 (auth endpoints). */
  skipRefresh?: boolean;
  /**
   * Send no CSRF token (default true). Only for the stateless public POSTs the
   * API exempts from CSRF (calculators), so a blocked /auth/csrf cannot stop them.
   */
  csrf?: boolean;
  /** Keep the request alive while the page unloads. */
  keepalive?: boolean;
  /** Decode the body as JSON (default) or return it as a Blob (images). */
  responseType?: "json" | "blob";
  /** Extra request headers (e.g. `X-Support-Token`); cannot override the built-in ones. */
  headers?: Record<string, string>;
};

const AUTH_HINT = "staryield.auth-hint";
let csrfToken: string | null = null;
let csrfPromise: Promise<string> | null = null;
let refreshPromise: Promise<RefreshResult> | null = null;
const expiredListeners = new Set<() => void>();
const signedOutListeners = new Set<() => void>();

/** Outcome of a session refresh (docs §22.1). */
export type RefreshOutcome = "refreshed" | "expired" | "unavailable";
export type RefreshResult = { outcome: RefreshOutcome; error?: ApiError };

const REFRESH_TIMEOUT_MS = 15_000;
/** A refresh finished less than this long ago (by any tab) is not repeated. */
const REFRESH_SKIP_MS = 10_000;
const REFRESH_LOCK = "staryield:auth-refresh";
const AUTH_CHANNEL = "staryield:auth";
type AuthMessage = { type: "refreshed"; at: number } | { type: "signed_out" };
let lastRefreshAt = 0;
let authChannel: BroadcastChannel | null | undefined;

export function newId(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) return crypto.randomUUID();
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    return (c === "x" ? r : (r & 0x3) | 0x8).toString(16);
  });
}

/** Remember that this browser has (or no longer has) a session. */
export function setAuthHint(on: boolean) {
  try {
    if (on) localStorage.setItem(AUTH_HINT, "1");
    else localStorage.removeItem(AUTH_HINT);
  } catch {
    // Storage may be unavailable (private mode); refresh simply runs anyway.
  }
}

/** First-paint guess: true only when this browser remembered a session (false on the server and without storage). */
export function readAuthHint(): boolean {
  try {
    return localStorage.getItem(AUTH_HINT) === "1";
  } catch {
    return false;
  }
}

function hasAuthHint(): boolean {
  try {
    return localStorage.getItem(AUTH_HINT) === "1";
  } catch {
    return true;
  }
}

/**
 * End the local session hint and tell the other tabs: explicit sign-out,
 * account closure, or a definitive expiry. Never call it for transient
 * failures (that is what used to disable refresh for every tab).
 */
export function endSession() {
  setAuthHint(false);
  postAuth({ type: "signed_out" });
}

/** The BroadcastChannel shared by this browser's tabs (null where unsupported). */
function channel(): BroadcastChannel | null {
  if (authChannel !== undefined) return authChannel;
  authChannel = null;
  if (typeof BroadcastChannel === "undefined") return authChannel;
  try {
    const ch = new BroadcastChannel(AUTH_CHANNEL);
    ch.onmessage = (event: MessageEvent<AuthMessage>) => {
      const msg = event.data;
      if (msg?.type === "refreshed" && typeof msg.at === "number") lastRefreshAt = Math.max(lastRefreshAt, msg.at);
      else if (msg?.type === "signed_out") {
        // Another tab signed out (or found the session gone): follow it.
        csrfToken = null;
        signedOutListeners.forEach((fn) => fn());
      }
    };
    authChannel = ch;
  } catch {
    authChannel = null;
  }
  return authChannel;
}

function postAuth(msg: AuthMessage) {
  try {
    channel()?.postMessage(msg);
  } catch {
    // A closed channel only costs the cross-tab shortcut.
  }
}

/** Subscribe to "another tab signed out" (explicit sign-out or expiry). */
export function onSignedOutElsewhere(fn: () => void): () => void {
  channel(); // start listening
  signedOutListeners.add(fn);
  return () => signedOutListeners.delete(fn);
}

/** Subscribe to "the session is gone" (refresh answered 401). */
export function onSessionExpired(fn: () => void): () => void {
  expiredListeners.add(fn);
  return () => expiredListeners.delete(fn);
}

export function setCsrfToken(token: string | null) {
  csrfToken = token;
}

export { currentLocale };

/**
 * `fetch` with a timeout whose rejections are already ApiErrors
 * (network_error / timeout), so callers never see a bare TypeError.
 */
async function fetchBounded(url: string, init: RequestInit, timeoutMs: number, requestId?: string): Promise<Response> {
  const controller = new AbortController();
  let timedOut = false;
  const timer = setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, timeoutMs);
  try {
    return await fetch(url, { ...init, signal: controller.signal });
  } catch (error) {
    if (timedOut) throw new ApiError({ code: "timeout", message: "Request timed out.", requestId, cause: error });
    throw new ApiError({ code: "network_error", message: "Network request failed.", requestId, cause: error });
  } finally {
    clearTimeout(timer);
  }
}

async function fetchCsrf(): Promise<string> {
  if (csrfToken) return csrfToken;
  if (!csrfPromise) {
    csrfPromise = (async () => {
      const requestId = newId();
      const res = await fetchBounded(
        `${API_URL}/api/v1/auth/csrf`,
        { credentials: "include", headers: { Accept: "application/json", "X-Request-ID": requestId } },
        REFRESH_TIMEOUT_MS,
        requestId,
      );
      if (!res.ok) throw await errorFrom(res);
      const body = (await res.json()) as { csrf_token: string };
      csrfToken = body.csrf_token;
      return csrfToken;
    })().finally(() => {
      csrfPromise = null;
    });
  }
  return csrfPromise;
}

async function errorFrom(res: Response): Promise<ApiError> {
  const requestId = res.headers.get("X-Request-ID") ?? undefined;
  const retryAfterHeader = res.headers.get("Retry-After");
  const retryAfter = retryAfterHeader ? Number(retryAfterHeader) : undefined;
  try {
    const body = (await res.json()) as {
      error?: { code?: string; message?: string; details?: Record<string, unknown>; request_id?: string };
    };
    if (body?.error?.code) {
      return new ApiError({
        code: body.error.code,
        message: body.error.message ?? res.statusText,
        status: res.status,
        details: body.error.details,
        requestId: body.error.request_id ?? requestId,
        retryAfter: Number.isFinite(retryAfter) ? retryAfter : undefined,
      });
    }
  } catch {
    // Non-JSON error body (proxy, gateway): fall through.
  }
  const code = res.status === 429 ? "rate_limited" : res.status === 503 || res.status === 502 ? "service_unavailable" : res.status >= 500 ? "internal_error" : "bad_request";
  return new ApiError({ code, message: res.statusText || "Request failed", status: res.status, requestId, retryAfter });
}

/**
 * One refresh attempt, with one retry when the CSRF token was stale. The
 * server no longer rotates the CSRF token on refresh, but another browser
 * profile of an old deployment or a cleared cookie can still make it stale.
 */
async function runRefresh(): Promise<RefreshResult> {
  try {
    for (let attempt = 0; ; attempt++) {
      const requestId = newId();
      const token = await fetchCsrf();
      const res = await fetchBounded(
        `${API_URL}/api/v1/auth/refresh`,
        { method: "POST", credentials: "include", headers: { Accept: "application/json", "X-CSRF-Token": token, "X-Request-ID": requestId } },
        REFRESH_TIMEOUT_MS,
        requestId,
      );
      if (res.ok) {
        const rotated = res.headers.get("X-CSRF-Token");
        if (rotated) csrfToken = rotated;
        try {
          const body = (await res.json()) as { csrf_token?: string };
          if (body.csrf_token) csrfToken = body.csrf_token;
        } catch {
          // The cookies are already rotated; an unreadable body changes nothing.
        }
        return refreshed();
      }
      const err = await errorFrom(res);
      if (err.code === "csrf_failed" && attempt === 0) {
        csrfToken = null; // stale token: fetch the current one and retry once
        continue;
      }
      if (err.code === "refresh_conflict") {
        // Old API: another tab rotated the cookie; the jar already holds the new one.
        await sleep(500);
        return refreshed();
      }
      // Only a definitive "this session is over" ends it. 429, 5xx and the
      // like are transient: nothing is cleared.
      if (res.status === 401 || err.code === "account_disabled") {
        endSession();
        expiredListeners.forEach((fn) => fn());
        return { outcome: "expired", error: err };
      }
      logger.warn("session refresh unavailable", err, { code: String(err.code), requestId: err.requestId, status: res.status });
      return { outcome: "unavailable", error: err };
    }
  } catch (raw) {
    const error = toApiError(raw);
    logger.warn("session refresh unavailable", error, { code: String(error.code), requestId: error.requestId });
    return { outcome: "unavailable", error };
  }
}

function refreshed(): RefreshResult {
  lastRefreshAt = Date.now();
  setAuthHint(true);
  postAuth({ type: "refreshed", at: lastRefreshAt });
  return { outcome: "refreshed" };
}

/** Run `fn` under the cross-tab refresh lock (per-tab when Web Locks are missing). */
async function withRefreshLock<T>(fn: () => Promise<T>): Promise<T> {
  const locks = typeof navigator !== "undefined" ? navigator.locks : undefined;
  if (!locks?.request) return fn();
  try {
    return await locks.request(REFRESH_LOCK, fn);
  } catch (error) {
    // The lock itself failed (not fn): degrade to the per-tab single flight.
    logger.debug("web lock unavailable", { reason: error instanceof Error ? error.name : "unknown" });
    return fn();
  }
}

/**
 * Refresh the access token once for all concurrent callers in this tab and,
 * through a Web Lock, in every tab. A refresh any tab completed within the
 * last 10 s is not repeated unless `force` is set (the caller just saw a 401
 * after such a refresh).
 */
export function refreshSessionOutcome(opts: { force?: boolean } = {}): Promise<RefreshResult> {
  if (!refreshPromise) {
    channel(); // listen for other tabs' refreshes before waiting for the lock
    refreshPromise = withRefreshLock(async () => {
      if (!opts.force && Date.now() - lastRefreshAt < REFRESH_SKIP_MS) return { outcome: "refreshed" } as RefreshResult;
      return runRefresh();
    }).finally(() => {
      refreshPromise = null;
    });
  }
  return refreshPromise;
}

/** Refresh the session; true when the access cookie is fresh (see refreshSessionOutcome for the detail). */
export async function refreshSession(): Promise<boolean> {
  return (await refreshSessionOutcome()).outcome === "refreshed";
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const t = setTimeout(resolve, ms);
    signal?.addEventListener("abort", () => {
      clearTimeout(t);
      reject(new DOMException("Aborted", "AbortError"));
    });
  });
}

function backoff(attempt: number, retryAfter?: number): number {
  if (retryAfter && retryAfter > 0) return Math.min(retryAfter * 1000, 15_000);
  const base = Math.min(400 * 2 ** attempt, 5_000);
  return base / 2 + Math.random() * (base / 2);
}

function buildUrl(path: string, query?: Query): string {
  const url = new URL(`${API_URL}${path}`);
  if (query)
    for (const [k, v] of Object.entries(query))
      if (v !== undefined && v !== null && v !== "") url.searchParams.set(k, String(v));
  return url.toString();
}

/** Perform an API request and decode the JSON response. */
export async function api<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const method = opts.method ?? "GET";
  const unsafe = method !== "GET";
  const maxRetries = opts.retries ?? (unsafe && !opts.idempotent ? 0 : 2);
  const timeoutMs = opts.timeoutMs ?? 15_000;
  let csrfRetried = false;
  let refreshes = 0;
  let conflictRetried = false;

  for (let attempt = 0; ; attempt++) {
    const requestId = newId();
    const controller = new AbortController();
    let timedOut = false;
    const timer = setTimeout(() => {
      timedOut = true;
      controller.abort();
    }, timeoutMs);
    const onAbort = () => controller.abort();
    opts.signal?.addEventListener("abort", onAbort);

    try {
      const headers: Record<string, string> = {
        ...opts.headers,
        Accept: opts.responseType === "blob" ? "image/*, application/json;q=0.5" : "application/json",
        "X-Request-ID": requestId,
        "X-Locale": currentLocale(),
      };
      let body: BodyInit | undefined;
      if (opts.body instanceof FormData) body = opts.body;
      else if (opts.body !== undefined) {
        headers["Content-Type"] = "application/json";
        body = JSON.stringify(opts.body);
      }
      if (unsafe && opts.csrf !== false) headers["X-CSRF-Token"] = await fetchCsrf();

      let res: Response;
      try {
        res = await fetch(buildUrl(path, opts.query), {
          method,
          headers,
          body,
          credentials: "include",
          signal: controller.signal,
          keepalive: opts.keepalive,
        });
      } catch (error) {
        if (timedOut) throw new ApiError({ code: "timeout", message: "Request timed out.", requestId, cause: error });
        if (opts.signal?.aborted) throw new ApiError({ code: "aborted", message: "Request cancelled.", requestId, cause: error });
        throw new ApiError({ code: "network_error", message: "Network request failed.", requestId, cause: error });
      }

      const rotated = res.headers.get("X-CSRF-Token");
      if (rotated) csrfToken = rotated;

      if (res.ok) {
        if (opts.responseType === "blob") {
          try {
            return (await res.blob()) as T;
          } catch (error) {
            throw new ApiError({ code: "invalid_response", message: "Unreadable response body.", status: res.status, requestId, cause: error });
          }
        }
        if (res.status === 204 || res.headers.get("Content-Length") === "0") return undefined as T;
        try {
          return (await res.json()) as T;
        } catch (error) {
          throw new ApiError({ code: "invalid_response", message: "Malformed JSON response.", status: res.status, requestId, cause: error });
        }
      }

      const err = await errorFrom(res);
      if (err.code === "csrf_failed" && !csrfRetried) {
        csrfRetried = true;
        csrfToken = null;
        continue;
      }
      if ((err.code === "session_expired" || err.code === "unauthenticated") && !opts.skipRefresh && refreshes < 2 && hasAuthHint()) {
        // A second refresh (forced, never skipped) only happens when the
        // first one succeeded or was shared and the request still got a 401.
        const result = await refreshSessionOutcome({ force: refreshes === 1 });
        if (result.outcome === "refreshed") {
          refreshes++;
          continue;
        }
        // The session is intact but could not be refreshed now: surface the
        // retryable refresh error (network_error, timeout, rate_limited, ...)
        // instead of a misleading 401.
        if (result.outcome === "unavailable" && result.error) throw result.error;
      }
      if (err.code === "refresh_conflict" && !conflictRetried) {
        conflictRetried = true;
        await sleep(300, opts.signal);
        continue;
      }
      throw err;
    } catch (raw) {
      const err = toApiError(raw);
      if (err.retryable && attempt < maxRetries && !opts.signal?.aborted) {
        logger.debug(`retrying ${method} ${path}`, { code: err.code, requestId: err.requestId });
        await sleep(backoff(attempt, err.retryAfter), opts.signal);
        continue;
      }
      if (err.code !== "aborted") {
        const ctx = { code: String(err.code), requestId: err.requestId, status: err.status, path };
        if (err.status >= 500 || err.code === "network_error" || err.code === "timeout" || err.code === "invalid_response")
          logger.warn(`API ${method} ${path} failed`, err, ctx);
        else logger.debug(`API ${method} ${path} rejected`, ctx);
      }
      throw err;
    } finally {
      clearTimeout(timer);
      opts.signal?.removeEventListener("abort", onAbort);
    }
  }
}

// Ship client error reports through the API (CSRF-protected endpoint).
setLogTransport(async (report) => {
  const token = await fetchCsrf();
  await fetch(`${API_URL}/api/v1/client-logs`, {
    method: "POST",
    credentials: "include",
    keepalive: true,
    headers: { "Content-Type": "application/json", "X-CSRF-Token": token, "X-Request-ID": newId() },
    body: JSON.stringify(report),
  });
});
