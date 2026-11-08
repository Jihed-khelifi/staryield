/**
 * System notifications for pending chat requests while the tab is hidden
 * (docs §21.1). Same tag as the server push (`chat.request:<session_id>`), so
 * the push and this notification never stack.
 */
import { CHAT_NOTIFICATION_ICON } from "@/lib/chat/chat-alerts";
import { pushSupported } from "@/lib/push";

export const REQUEST_NOTIFICATION_KIND = "chat.requested";
export const requestNotificationTag = (sessionId: string) => `chat.request:${sessionId}`;

async function registration(): Promise<ServiceWorkerRegistration | null> {
  if (!pushSupported()) return null;
  try {
    return (await navigator.serviceWorker.getRegistration("/")) ?? null;
  } catch {
    return null;
  }
}

export async function showRequestNotification(opts: { sessionId: string; title: string; body: string; url: string }): Promise<boolean> {
  const reg = await registration();
  if (!reg) return false;
  try {
    await reg.showNotification(opts.title, {
      body: opts.body,
      icon: CHAT_NOTIFICATION_ICON,
      badge: CHAT_NOTIFICATION_ICON,
      tag: requestNotificationTag(opts.sessionId),
      renotify: true,
      requireInteraction: true,
      data: { url: new URL(opts.url, window.location.origin).href, kind: REQUEST_NOTIFICATION_KIND },
    } as NotificationOptions);
    return true;
  } catch {
    return false;
  }
}

/** Close the request notification with this tag; a `chat.missed` one with the same tag is kept. */
export async function closeRequestNotification(sessionId: string): Promise<void> {
  const reg = await registration();
  if (!reg) return;
  try {
    const list = await reg.getNotifications({ tag: requestNotificationTag(sessionId) });
    for (const n of list) {
      if ((n.data as { kind?: string } | null)?.kind === REQUEST_NOTIFICATION_KIND) n.close();
    }
  } catch {
    /* nothing to close */
  }
}
