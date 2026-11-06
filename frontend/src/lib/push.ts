/**
 * Web Push in the browser: feature detection, the permission prompt, the
 * service worker (`/sw.js`) and the browser subscription that the API
 * stores (`POST /me/push-subscriptions`). Every step degrades gracefully:
 * unsupported browsers, blocked permissions and push-service failures are
 * reported as outcomes, never thrown at the UI.
 */
import { pushApi } from "@/lib/api/account";

export const SERVICE_WORKER_URL = "/sw.js";

export type PushOutcome = "subscribed" | "denied" | "dismissed" | "unsupported" | "failed";

/** Push needs a secure context, a service worker, PushManager and Notification. */
export function pushSupported(): boolean {
  if (typeof window === "undefined" || typeof navigator === "undefined") return false;
  return Boolean(window.isSecureContext) && "serviceWorker" in navigator && "PushManager" in window && "Notification" in window;
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

/**
 * Ask for permission (call from a click), register the service worker,
 * subscribe with the server's VAPID key and register the subscription with
 * the API.
 */
export async function enablePush(publicKey: string): Promise<PushOutcome> {
  if (!pushSupported()) return "unsupported";
  let permission = Notification.permission;
  if (permission === "default") {
    try {
      permission = await Notification.requestPermission();
    } catch {
      return "failed";
    }
  }
  if (permission === "denied") return "denied";
  if (permission !== "granted") return "dismissed";
  try {
    const registration = await navigator.serviceWorker.register(SERVICE_WORKER_URL, { scope: "/" });
    await navigator.serviceWorker.ready;
    const key = vapidKeyBytes(publicKey);
    let subscription = await registration.pushManager.getSubscription();
    if (subscription && !sameKey(subscription, key)) {
      await subscription.unsubscribe();
      subscription = null;
    }
    subscription ??= await registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: key });
    await pushApi.subscribe(subscription.toJSON());
    return "subscribed";
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
