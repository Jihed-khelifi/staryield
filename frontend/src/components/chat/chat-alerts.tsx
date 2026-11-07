"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { chatApi } from "@/lib/api/endpoints";
import type { ChatMessage, Conversation } from "@/lib/api/types";
import { useRealtimeEvent, useSession } from "@/lib/auth/session-provider";
import { chatNotificationUrl, shouldShowSystemNotification, showChatNotification } from "@/lib/chat/chat-alerts";
import { pushPermission } from "@/lib/push";

/**
 * While this tab is open but hidden, a counterpart message shows a system
 * notification through the service worker (§18: same tag as the server push,
 * so they never stack; the message text is never shown). Renders nothing.
 * The counterpart's name comes from the inbox, fetched lazily and only when a
 * notification will actually be shown.
 */
export function ChatAlerts() {
  const { text, locale } = useI18n();
  const { status, user } = useSession();
  const meId = status === "authenticated" ? (user?.id ?? "") : "";
  const role = user?.role;
  const known = React.useRef(new Map<string, Conversation>());

  React.useEffect(() => {
    known.current = new Map();
  }, [meId]);

  useRealtimeEvent<{ message: ChatMessage }>("message.created", ({ message }) => {
    if (!meId || !message) return;
    const hidden = typeof document !== "undefined" && document.visibilityState === "hidden";
    if (!shouldShowSystemNotification({ message, meId, activeConversationId: null, hidden, permission: pushPermission() })) return;
    void (async () => {
      let conv = known.current.get(message.conversation_id);
      if (!conv) {
        try {
          const items = await chatApi.conversations();
          known.current = new Map(items.map((c) => [c.id, c]));
          conv = known.current.get(message.conversation_id);
        } catch {
          /* show the notification without the name */
        }
      }
      const name = conv?.counterpart.name;
      await showChatNotification({
        conversationId: message.conversation_id,
        title: name ? text("New message from {value0}", { value0: name }) : text("New message"),
        body: text("Open the chatroom to read it."),
        url: chatNotificationUrl(locale, role === "client" ? conv?.counterpart.slug : undefined),
      });
    })();
  });
  return null;
}
