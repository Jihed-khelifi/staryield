/**
 * Client logger. Every warning and error is written to the console in
 * development and reported to the API (/api/v1/client-logs) with the request
 * correlation ID, so frontend failures appear in the same structured logs as
 * backend ones. Reports are de-duplicated and rate-limited.
 */

type Level = "debug" | "info" | "warn" | "error";

export type LogContext = {
  code?: string;
  requestId?: string;
  [key: string]: unknown;
};

type Transport = (report: {
  level: Level;
  message: string;
  stack?: string;
  code?: string;
  url?: string;
  request_id?: string;
  context?: Record<string, unknown>;
}) => Promise<void>;

let transport: Transport | null = null;
const recent = new Map<string, number>();
let windowStart = 0;
let sentInWindow = 0;
const MAX_PER_MINUTE = 10;

/** The API client installs the transport (avoids an import cycle). */
export function setLogTransport(t: Transport) {
  transport = t;
}

const isDev = process.env.NODE_ENV !== "production";

function scalarContext(ctx?: LogContext): Record<string, unknown> | undefined {
  if (!ctx) return undefined;
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(ctx)) {
    if (k === "code" || k === "requestId") continue;
    if (/^[a-z][a-z0-9_]{0,63}$/.test(k) && ["string", "number", "boolean"].includes(typeof v))
      out[k] = typeof v === "string" ? v.slice(0, 256) : v;
  }
  return out;
}

function report(level: Level, message: string, error?: unknown, ctx?: LogContext) {
  if (typeof window === "undefined" || !transport) return;
  const key = `${level}:${message}`;
  const now = Date.now();
  if ((recent.get(key) ?? 0) > now - 60_000) return;
  recent.set(key, now);
  if (recent.size > 200) recent.clear();
  if (now - windowStart > 60_000) {
    windowStart = now;
    sentInWindow = 0;
  }
  if (++sentInWindow > MAX_PER_MINUTE) return;
  const stack = error instanceof Error ? error.stack : undefined;
  transport({
    level,
    message: message.slice(0, 1000),
    stack: stack?.slice(0, 4000),
    code: ctx?.code,
    url: window.location.pathname,
    request_id: ctx?.requestId,
    context: scalarContext(ctx),
  }).catch(() => {
    // Never let log shipping failures surface or recurse.
  });
}

function write(level: Level, message: string, error?: unknown, ctx?: LogContext) {
  if (isDev || level === "warn" || level === "error") {
    const fn = level === "debug" ? console.debug : level === "info" ? console.info : level === "warn" ? console.warn : console.error;
    fn(`[staryield] ${message}`, ...(error !== undefined ? [error] : []), ...(ctx ? [ctx] : []));
  }
  if (level === "warn" || level === "error") report(level, message, error, ctx);
}

export const logger = {
  debug: (message: string, ctx?: LogContext) => write("debug", message, undefined, ctx),
  info: (message: string, ctx?: LogContext) => write("info", message, undefined, ctx),
  warn: (message: string, error?: unknown, ctx?: LogContext) => write("warn", message, error, ctx),
  error: (message: string, error?: unknown, ctx?: LogContext) => write("error", message, error, ctx),
};
