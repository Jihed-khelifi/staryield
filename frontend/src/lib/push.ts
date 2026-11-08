/**
 * Web Push in the browser: feature detection, the permission prompt, the
 * service worker (`/sw.js`) and the browser subscription that the API
 * stores (`POST /me/push-subscriptions`). Every step degrades gracefully:
 * unsupported browsers, blocked permissions and push-service failures are
 * reported as outcomes, never thrown at the UI.
 */
import { pushApi } from "@/lib/api/account";
import { ApiError } from "@/lib/api/errors";

export const SERVICE_WORKER_URL = "/sw.js";

export type PushOutcome =
  | "subscribed"
  | "denied"
  | "dismissed"
  | "unsupported"
  | "ios_install"
  | "brave_blocked"
  | "service_error"
  | "server_rejected"
  | "failed";

/** English source copy for every failure outcome (docs §21.3); callers pass it through `text()`. */
const OUTCOME_MESSAGE: Record<Exclude<PushOutcome, "subscribed">, string> = {
  denied: "Notifications are blocked for this site. Allow them in your browser settings, then try again.",
  dismissed: "You closed the permission prompt. Turn the switch on again when you're ready.",
  unsupported: "This browser doesn't support push notifications.",
  ios_install:
    "On iPhone and iPad, notifications work once Staryield is on your Home Screen: tap Share, then “Add to Home Screen”, and open Staryield from there.",
  brave_blocked:
    "Brave blocks website notifications by default. Open brave://settings/privacy, turn on “Use Google services for push messaging”, restart Brave, then try again.",
  service_error: "Your browser's notification service didn't respond. Check your connection, then try again.",
  server_rejected: "This browser's notification service isn't supported. Try Chrome, Edge, Firefox or Safari.",
  failed: "We couldn't turn on notifications in this browser. Please try again.",
};

export function pushOutcomeMessage(outcome: Exclude<PushOutcome, "subscribed">): string {
  return OUTCOME_MESSAGE[outcome];
}

/** Where Brave's switch lives; web pages cannot link to brave:// URLs, so the UI offers to copy it. */
export const BRAVE_PRIVACY_URL = "brave://settings/privacy";

/** Milliseconds to wait for `navigator.serviceWorker.ready`. */
export const SERVICE_WORKER_TIMEOUT_MS = 10_000;
const RETRY_DELAY_MS = 1000;
const API_RETRY_DELAY_MS = 600;

const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

/** Push needs a secure context, a service worker, PushManager and Notification. */
export function pushSupported(): boolean {
  if (typeof window === "undefined" || typeof navigator === "undefined") return false;
  return Boolean(window.isSecureContext) && "serviceWorker" in navigator && "PushManager" in window && "Notification" in window;
}

function isIOSDevice(): boolean {
  if (typeof navigator === "undefined") return false;
  if (/iPad|iPhone|iPod/.test(navigator.userAgent || "")) return true;
  // iPadOS asks for the desktop site: it reports a Mac with a touch screen.
  return navigator.platform === "MacIntel" && (navigator.maxTouchPoints ?? 0) > 1;
}

function isStandalone(): boolean {
  if (typeof window === "undefined") return false;
  try {
    if ((navigator as Navigator & { standalone?: boolean }).standalone === true) return true;
    return typeof window.matchMedia === "function" && window.matchMedia("(display-mode: standalone)").matches;
  } catch {
    return false;
  }
}

export type PushAvailability = "available" | "ios_install" | "unsupported";

/** Whether this browser can take push now, needs the iOS Home Screen app, or cannot at all. */
export function pushAvailability(): PushAvailability {
  if (pushSupported()) return "available";
  if (isIOSDevice() && !isStandalone()) return "ios_install";
  return "unsupported";
}

async function isBrave(): Promise<boolean> {
  try {
    const brave = (navigator as Navigator & { brave?: { isBrave?: () => Promise<boolean> } }).brave;
    return Boolean(await brave?.isBrave?.());
  } catch {
    return false;
  }
}

/** The notification permission, or "unsupported". */
export function pushPermission(): NotificationPermission | "unsupported" {
  return pushSupported() ? Notification.permission : "unsupported";
}

/** VAPID public key (base64url) → the bytes `pushManager.subscribe` expects. */
export function vapidKeyBytes(base64url: string): Uint8Array<ArrayBuffer> {
  const padding = "=".repeat((4 - (base64url.length % 4)) % 4);
  const base64 = (base64url + padding).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64);
  const out = new Uint8Array(new ArrayBuffer(raw.length));
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
  return out;
}

function sameKey(subscription: PushSubscription, key: Uint8Array): boolean {
  const current = subscription.options?.applicationServerKey;
  if (!current) return true;
  const bytes = new Uint8Array(current);
  return bytes.length === key.length && bytes.every((b, i) => b === key[i]);
}

/** This browser's existing subscription, if any (never prompts). */
export async function existingSubscription(): Promise<PushSubscription | null> {
  if (!pushSupported()) return null;
  try {
    const registration = await navigator.serviceWorker.getRegistration("/");
    return (await registration?.pushManager.getSubscription()) ?? null;
  } catch {
    return null;
  }
}

function errorName(error: unknown): string {
  return typeof error === "object" && error !== null && "name" in error ? String((error as { name: unknown }).name) : "";
}

function requestPermission(): Promise<NotificationPermission> {
  return new Promise((resolve, reject) => {
    try {
      // Legacy Safari takes a callback and returns undefined; modern browsers return a promise.
      const result = Notification.requestPermission(resolve) as Promise<NotificationPermission> | undefined;
      if (result && typeof result.then === "function") result.then(resolve, reject);
    } catch (error) {
      reject(error);
    }
  });
}

/** The service worker registration, or null when `ready` does not settle in time. */
async function readyRegistration(): Promise<ServiceWorkerRegistration | null> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<null>((resolve) => {
    timer = setTimeout(() => resolve(null), SERVICE_WORKER_TIMEOUT_MS);
  });
  try {
    return await Promise.race([navigator.serviceWorker.ready, timeout]);
  } finally {
    clearTimeout(timer);
  }
}

type Subscribed = { subscription: PushSubscription } | { outcome: Exclude<PushOutcome, "subscribed"> };

/** Browser subscription with the key-mismatch, InvalidStateError and AbortError recoveries. */
async function subscribeBrowser(registration: ServiceWorkerRegistration, key: Uint8Array<ArrayBuffer>): Promise<Subscribed> {
  let subscription = await registration.pushManager.getSubscription();
  if (subscription && !sameKey(subscription, key)) {
    await subscription.unsubscribe().catch(() => false);
    subscription = null;
  }
  if (subscription) return { subscription };
  const subscribe = () => registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: key });
  let aborted = 0;
  let reset = false;
  for (;;) {
    try {
      return { subscription: await subscribe() };
    } catch (error) {
      const name = errorName(error);
      const message = error instanceof Error ? error.message : "";
      if (name === "NotAllowedError") return { outcome: "denied" };
      if (name === "InvalidStateError" && !reset) {
        reset = true;
        const stale = await registration.pushManager.getSubscription().catch(() => null);
        await stale?.unsubscribe().catch(() => false);
        continue;
      }
      if (name === "AbortError" || /push service error/i.test(message)) {
        if (aborted++ === 0) {
          await sleep(RETRY_DELAY_MS);
          continue;
        }
        return { outcome: (await isBrave()) ? "brave_blocked" : "service_error" };
      }
      return { outcome: "failed" };
    }
  }
}

/** Register with the API: 422 is a rejection, network errors and 5xx are retried once. */
async function registerWithApi(subscription: PushSubscription): Promise<PushOutcome> {
  for (let attempt = 0; ; attempt++) {
    try {
      await pushApi.subscribe(subscription.toJSON());
      return "subscribed";
    } catch (error) {
      const api = error instanceof ApiError ? error : null;
      if (api && (api.status === 422 || api.code === "validation_failed")) return "server_rejected";
      const transient = api ? api.retryable : true;
      if (!transient || attempt >= 1) return "failed";
      await sleep(API_RETRY_DELAY_MS);
    }
  }
}

/**
 * Ask for permission (call from a click), register the service worker,
 * subscribe with the server's VAPID key and register the subscription with
 * the API. Never throws; never prompts when permission is already decided.
 */
export async function enablePush(publicKey: string): Promise<PushOutcome> {
  const availability = pushAvailability();
  if (availability !== "available") return availability;
  let permission = Notification.permission;
  if (permission === "default") {
    try {
      permission = await requestPermission();
    } catch {
      return "failed";
    }
  }
  if (permission === "denied") return "denied";
  if (permission !== "granted") return "dismissed";
  try {
    await navigator.serviceWorker.register(SERVICE_WORKER_URL, { scope: "/", updateViaCache: "none" });
    const registration = await readyRegistration();
    if (!registration) return "service_error";
    const result = await subscribeBrowser(registration, vapidKeyBytes(publicKey));
    if ("outcome" in result) return result.outcome;
    return await registerWithApi(result.subscription);
  } catch {
    return "failed";
  }
}

/** Remove this browser's subscription (API first, then the browser). */
export async function disablePush(): Promise<void> {
  const subscription = await existingSubscription();
  if (!subscription) return;
  try {
    await pushApi.unsubscribe(subscription.endpoint);
  } finally {
    await subscription.unsubscribe().catch(() => false);
  }
}

/** Re-send an existing subscription so the server copy stays current (idempotent). */
export async function refreshPush(): Promise<boolean> {
  const subscription = await existingSubscription();
  if (!subscription) return false;
  try {
    await pushApi.subscribe(subscription.toJSON());
    return true;
  } catch {
    return false;
  }
}

// --- background upkeep (PushSync, docs §21.3) -------------------------------------

/** Re-send an existing subscription at most this often per user and browser. */
export const PUSH_REFRESH_INTERVAL_MS = 6 * 60 * 60 * 1000;
export const PUSH_SYNC_KEY_PREFIX = "staryield.push.synced.v1:";

/** A failed silent re-subscribe (e.g. Brave with Google push off) is retried at most this often. */
export const PUSH_RESUBSCRIBE_RETRY_MS = 60 * 60 * 1000;
export const PUSH_RESUBSCRIBE_KEY_PREFIX = "staryield.push.resubscribe.v1:";

function readTime(key: string): number {
  try {
    return Number(window.localStorage.getItem(key)) || 0;
  } catch {
    return 0;
  }
}

function writeTime(key: string): void {
  try {
    window.localStorage.setItem(key, String(Date.now()));
  } catch {
    /* private mode: tried again next page load */
  }
}

const lastSynced = (userId: string) => readTime(PUSH_SYNC_KEY_PREFIX + userId);
const markSynced = (userId: string) => writeTime(PUSH_SYNC_KEY_PREFIX + userId);

export type PushSyncResult = "skipped" | "throttled" | "refreshed" | "resubscribed" | "idle";

/**
 * Never prompts and never unsubscribes. With permission granted: an existing
 * subscription is re-sent (throttled); a missing one is recreated silently
 * when the account's push switch is on.
 */
export async function syncPush(opts: {
  userId: string;
  pushSwitchOn: () => Promise<boolean>;
  publicKey: () => Promise<string | null>;
}): Promise<PushSyncResult> {
  if (pushAvailability() !== "available" || Notification.permission !== "granted") return "skipped";
  const subscription = await existingSubscription();
  if (subscription) {
    if (Date.now() - lastSynced(opts.userId) < PUSH_REFRESH_INTERVAL_MS) return "throttled";
    if (await refreshPush()) {
      markSynced(opts.userId);
      return "refreshed";
    }
    return "idle";
  }
  // Without this, a browser whose push service refuses (Brave's default) would
  // retry two subscribes on every page load.
  const attemptKey = PUSH_RESUBSCRIBE_KEY_PREFIX + opts.userId;
  if (Date.now() - readTime(attemptKey) < PUSH_RESUBSCRIBE_RETRY_MS) return "throttled";
  try {
    if (!(await opts.pushSwitchOn())) return "idle";
    const key = await opts.publicKey();
    if (!key) return "idle";
    writeTime(attemptKey);
    if ((await enablePush(key)) === "subscribed") {
      markSynced(opts.userId);
      return "resubscribed";
    }
  } catch {
    /* best effort */
  }
  return "idle";
}
