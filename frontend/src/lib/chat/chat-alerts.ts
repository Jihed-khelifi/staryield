/**
 * Decisions and helpers for chat message alerts (docs §18):
 *  - hidden tab + counterpart message + permission granted → one system
 *    notification through the service worker, tag `chat:<conversation_id>`
 *    (the same tag as the server push, so the two never stack);
 *  - a counterpart message for another conversation than the one on screen →
 *    an in-app toast;
 *  - the "turn on notifications" prompt, shown once per user.
 */
import type { ChatMessage } from "@/lib/api/types";
import { pushSupported } from "@/lib/push";

export const CHAT_NOTIFICATION_ICON = "https://assets.staryield.net/assets/brand/sun-mark-192-c139a00d54.png";

export type AlertInput = {
  message: Pick<ChatMessage, "conversation_id" | "sender_id" | "kind">;
  meId: string;
  activeConversationId: string | null;
  /** document.visibilityState === "hidden" */
  hidden: boolean;
  permission: NotificationPermission | "unsupported";
};

/** A message someone else wrote that people want to hear about (never system messages). */
export function isCounterpartMessage(m: AlertInput["message"], meId: string): boolean {
  return Boolean(m.sender_id) && m.sender_id !== meId && (m.kind === "text" || m.kind === "image");
}

export function shouldShowSystemNotification(i: AlertInput): boolean {
  return isCounterpartMessage(i.message, i.meId) && i.hidden && i.permission === "granted";
}

export function shouldToastOtherConversation(i: AlertInput): boolean {
  return isCounterpartMessage(i.message, i.meId) && !i.hidden && Boolean(i.activeConversationId) && i.message.conversation_id !== i.activeConversationId;
}

export const chatNotificationTag = (conversationId: string) => `chat:${conversationId}`;

/** `/chatroom?reader=<slug>` for clients, `/chatroom` for advisors, with the locale. */
export function chatNotificationUrl(locale: string, slug?: string): string {
  return slug && /^[a-z0-9-]{1,64}$/.test(slug) ? `/${locale}/chatroom?reader=${slug}` : `/${locale}/chatroom`;
}

/** Show the notification through the service worker. Returns false when it could not be shown. */
export async function showChatNotification(opts: { conversationId: string; title: string; body: string; url: string }): Promise<boolean> {
  if (!pushSupported()) return false;
  try {
    const registration = await navigator.serviceWorker.getRegistration("/");
    if (!registration) return false;
    await registration.showNotification(opts.title, {
      body: opts.body,
      icon: CHAT_NOTIFICATION_ICON,
      badge: CHAT_NOTIFICATION_ICON,
      tag: chatNotificationTag(opts.conversationId),
      renotify: true,
      data: { url: new URL(opts.url, window.location.origin).href, kind: "chat.message" },
    } as NotificationOptions);
    return true;
  } catch {
    return false;
  }
}

// --- the "turn on notifications" prompt -----------------------------------------

export const PROMPT_KEY_PREFIX = "staryield.chat.push-prompt.v1:";

export function promptDismissed(userId: string): boolean {
  try {
    return window.localStorage.getItem(PROMPT_KEY_PREFIX + userId) !== null;
  } catch {
    return false;
  }
}

export function dismissPrompt(userId: string): void {
  try {
    window.localStorage.setItem(PROMPT_KEY_PREFIX + userId, String(Date.now()));
  } catch {
    /* private mode: the prompt may return next visit */
  }
}

export function shouldShowPrompt(i: { supported: boolean; permission: NotificationPermission | "unsupported"; dismissed: boolean; hasSent: boolean; serverEnabled: boolean }): boolean {
  return i.supported && i.permission === "default" && !i.dismissed && i.hasSent && i.serverEnabled;
}
