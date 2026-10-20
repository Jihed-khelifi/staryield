/**
 * Product analytics for the admin dashboard. Events are batched and sent to
 * /api/v1/analytics/events; business-critical events (payments, sessions)
 * are recorded server-side and never trusted from the browser.
 *
 * Privacy: honours Do Not Track and Global Privacy Control, never sends
 * message content or form values, and strips query strings server-side.
 */
import { api, newId } from "@/lib/api/client";
import { logger } from "@/lib/logger";

export type ClientEventName =
  | "page_view"
  | "calculator_started"
  | "calculator_completed"
  | "psychic_list_viewed"
  | "psychic_profile_viewed"
  | "chat_cta_clicked"
  | "checkout_opened"
  | "checkout_pack_selected"
  | "signup_step_viewed"
  | "signup_step_completed"
  | "horoscope_viewed"
  | "chart_viewed"
  | "service_viewed"
  | "service_order_started"
  | "error_displayed"
  | "web_vital"
  | "realtime_reconnected"
  | "realtime_fallback"
  | "locale_changed"
  | "chat_opened";

type Queued = {
  event_id: string;
  name: ClientEventName;
  occurred_at: string;
  anonymous_id: string;
  session_key: string;
  path: string;
  referrer: string;
  locale: string;
  properties?: Record<string, string | number | boolean>;
};

const queue: Queued[] = [];
let timer: ReturnType<typeof setTimeout> | null = null;
let installed = false;

function optedOut(): boolean {
  if (typeof navigator === "undefined") return true;
  const gpc = (navigator as Navigator & { globalPrivacyControl?: boolean }).globalPrivacyControl;
  return navigator.doNotTrack === "1" || gpc === true;
}

function stored(storage: () => Storage, key: string): string {
  try {
    let v = storage().getItem(key);
    if (!v) {
      v = newId();
      storage().setItem(key, v);
    }
    return v;
  } catch {
    return newId();
  }
}

async function flush(keepalive = false) {
  if (timer) clearTimeout(timer);
  timer = null;
  if (queue.length === 0) return;
  const events = queue.splice(0, 50);
  try {
    await api("/api/v1/analytics/events", { method: "POST", body: { events }, keepalive, retries: 1, idempotent: true });
  } catch (error) {
    logger.debug("analytics flush failed", { code: String((error as { code?: string }).code ?? "") });
  }
  if (queue.length) void flush(keepalive);
}

function install() {
  if (installed || typeof window === "undefined") return;
  installed = true;
  const onHide = () => {
    if (document.visibilityState === "hidden") void flush(true);
  };
  document.addEventListener("visibilitychange", onHide);
  window.addEventListener("pagehide", () => void flush(true));
}

/** Record a client event (no-op when the visitor opted out). */
export function track(name: ClientEventName, properties?: Record<string, string | number | boolean>) {
  if (typeof window === "undefined" || optedOut()) return;
  install();
  queue.push({
    event_id: newId(),
    name,
    occurred_at: new Date().toISOString(),
    anonymous_id: stored(() => localStorage, "staryield.aid"),
    session_key: stored(() => sessionStorage, "staryield.sid"),
    path: window.location.pathname,
    referrer: document.referrer,
    locale: document.documentElement.lang,
    properties,
  });
  if (queue.length >= 20) void flush();
  else if (!timer) timer = setTimeout(() => void flush(), 5_000);
}
