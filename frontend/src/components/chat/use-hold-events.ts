"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import type { ChatHold, HoldEndedEvent, HoldExtendedEvent, MessagesRevealedEvent, SessionHeldEvent } from "@/lib/api/holds";
import { useRealtimeEvent } from "@/lib/auth/session-provider";
import { holdSecondsLeft } from "@/lib/chat/hold";
import type { Announcement } from "@/components/chat/chat-feed";
import { useToast } from "@/components/feedback/toast-provider";

type Options = {
  role: "client" | "psychic";
  /** The other participant, as the viewer sees them. */
  counterpartName: string;
  /** Whether a conversation is the one on screen. */
  isOnScreen: (conversationId: string) => boolean;
  apply: (hold: ChatHold) => void;
  extend: (holdId: string, expiresAt: string) => void;
  /** The client's top-up dialog (once per hold, when it starts). */
  openTopUp: () => void;
  /** Lines for the screen-reader log of the feed. */
  announce: (entries: Announcement[]) => void;
  /** Client: the revealed messages (after a conversion or a reveal). */
  onRevealed: (event: MessagesRevealedEvent & { messages: NonNullable<MessagesRevealedEvent["messages"]> }) => void;
  /** Advisor: these messages are no longer hidden from the client. */
  onAdvisorRevealed: (event: MessagesRevealedEvent & { message_ids: string[] }) => void;
};

/**
 * Realtime events of the free-trial hold (§26.1), folded into the hold store
 * and the chatroom. `session.held` itself also reaches the shell's session
 * handler (which stores the session view); here it opens the client's top-up
 * dialog once per hold and tells screen readers the reading is on hold.
 */
export function useHoldEvents(o: Options) {
  const { text } = useI18n();
  const toast = useToast();
  const latest = React.useRef(o);
  React.useEffect(() => {
    latest.current = o;
  });
  // Events can repeat (reconnect replays, a second tab): each hold opens the dialog and speaks once.
  const handled = React.useRef(new Set<string>());
  const once = (key: string) => {
    if (handled.current.has(key)) return false;
    handled.current.add(key);
    return true;
  };

  useRealtimeEvent<SessionHeldEvent>("session.held", (e) => {
    const hold = e?.hold;
    if (!hold) return;
    const { apply, role, isOnScreen, counterpartName, announce, openTopUp } = latest.current;
    apply(hold);
    if (!isOnScreen(hold.conversation_id) || !once(`held:${hold.id}`)) return;
    const offset = e.session?.server_time ? new Date(e.session.server_time).getTime() - Date.now() : 0;
    const seconds = holdSecondsLeft(hold.expires_at, Date.now(), offset);
    if (role === "client") {
      announce([{ id: `hold-${hold.id}`, text: text("Your reading is on hold. Top up within {value0} seconds to keep chatting with {value1}.", { value0: seconds, value1: counterpartName }) }]);
      openTopUp();
    } else {
      announce([{ id: `hold-${hold.id}`, text: text("{value0}'s reading is on hold. Your replies are hidden from them until they top up.", { value0: counterpartName }) }]);
    }
  });

  useRealtimeEvent<HoldExtendedEvent>("session.hold_extended", (e) => {
    if (e?.hold_id && e.expires_at) latest.current.extend(e.hold_id, e.expires_at);
  });

  useRealtimeEvent<HoldEndedEvent>("session.hold_ended", (e) => {
    const hold = e?.hold;
    if (!hold) return;
    const { apply, role, isOnScreen, counterpartName, announce } = latest.current;
    apply(hold);
    if (!isOnScreen(hold.conversation_id) || !once(`ended:${hold.id}`)) return;
    if (hold.status === "converted") {
      if (role === "client") {
        const message = text("Your reading with {value0} continues.", { value0: counterpartName });
        toast.show({ tone: "success", message });
        announce([{ id: `hold-end-${hold.id}`, text: message }]);
      } else {
        announce([{ id: `hold-end-${hold.id}`, text: text("{value0} topped up. Your replies are visible to them now.", { value0: counterpartName }) }]);
      }
    } else if (hold.status === "expired") {
      announce([
        {
          id: `hold-end-${hold.id}`,
          text:
            role === "client"
              ? text("The reading with {value0} ended before a top-up arrived.", { value0: counterpartName })
              : text("The reading with {value0} ended before they topped up. Your replies were not revealed.", { value0: counterpartName }),
        },
      ]);
    }
  });

  useRealtimeEvent<MessagesRevealedEvent>("messages.revealed", (e) => {
    if (!e) return;
    const { onRevealed, onAdvisorRevealed } = latest.current;
    if (e.messages) onRevealed({ ...e, messages: e.messages });
    else if (e.message_ids) onAdvisorRevealed({ ...e, message_ids: e.message_ids });
  });
}
