/**
 * HTTP client for the Staryield API.
 *
 * - Cookies carry the session (HttpOnly); `credentials: "include"`.
 * - Unsafe requests send the CSRF double-submit token (fetched once, rotated
 *   by the server, retried once when stale).
 * - An expired access token triggers a single-flight refresh, then the
 *   original request is retried once.
 * - Every request has a timeout and an X-Request-ID that appears in server
 *   logs and in error responses.
 * - Idempotent requests retry transient failures with exponential backoff,
 *   honouring Retry-After.
 * - Every failure is converted into an ApiError and logged.
 */
import { API_URL } from "@/lib/api/config";
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
  /** Keep the request alive while the page unloads. */
  keepalive?: boolean;
};

const AUTH_HINT = "staryield.auth-hint";
let csrfToken: string | null = null;
let csrfPromise: Promise<string> | null = null;
let refreshPromise: Promise<boolean> | null = null;
const expiredListeners = new Set<() => void>();

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

function hasAuthHint(): boolean {
  try {
    return localStorage.getItem(AUTH_HINT) === "1";
  } catch {
    return true;
  }
}

/** Subscribe to "the session is gone" (refresh failed). */
export function onSessionExpired(fn: () => void): () => void {
  expiredListeners.add(fn);
  return () => expiredListeners.delete(fn);
}

export function setCsrfToken(token: string | null) {
  csrfToken = token;
}

function currentLocale(): string {
  return typeof document !== "undefined" ? document.documentElement.lang || "en" : "en";
}

async function fetchCsrf(): Promise<string> {
  if (csrfToken) return csrfToken;
  if (!csrfPromise) {
    csrfPromise = (async () => {
      const res = await fetch(`${API_URL}/api/v1/auth/csrf`, {
        credentials: "include",
        headers: { Accept: "application/json", "X-Request-ID": newId() },
      });
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

/** Refresh the access token once for all concurrent callers. */
export function refreshSession(): Promise<boolean> {
  if (!refreshPromise) {
    refreshPromise = (async () => {
      try {
        const token = await fetchCsrf();
        const res = await fetch(`${API_URL}/api/v1/auth/refresh`, {
          method: "POST",
          credentials: "include",
          headers: { Accept: "application/json", "X-CSRF-Token": token, "X-Request-ID": newId() },
        });
        if (res.ok) {
          const body = (await res.json()) as { csrf_token?: string };
          if (body.csrf_token) csrfToken = body.csrf_token;
          setAuthHint(true);
          return true;
        }
        const err = await errorFrom(res);
        if (err.code === "refresh_conflict") {
          // Another tab rotated the cookie; the jar already holds the new one.
          await sleep(300);
          return true;
        }
        if (err.code === "csrf_failed") csrfToken = null;
        setAuthHint(false);
        expiredListeners.forEach((fn) => fn());
        return false;
      } catch (error) {
        logger.warn("session refresh failed", error);
        return false;
      }
    })().finally(() => {
      refreshPromise = null;
    });
  }
  return refreshPromise;
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
  let refreshed = false;
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
        Accept: "application/json",
        "X-Request-ID": requestId,
        "X-Locale": currentLocale(),
      };
      let body: BodyInit | undefined;
      if (opts.body instanceof FormData) body = opts.body;
      else if (opts.body !== undefined) {
        headers["Content-Type"] = "application/json";
        body = JSON.stringify(opts.body);
      }
      if (unsafe) headers["X-CSRF-Token"] = await fetchCsrf();

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
      if ((err.code === "session_expired" || err.code === "unauthenticated") && !opts.skipRefresh && !refreshed && hasAuthHint()) {
        refreshed = true;
        if (await refreshSession()) continue;
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
