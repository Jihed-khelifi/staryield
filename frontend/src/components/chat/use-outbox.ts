"use client";

import * as React from "react";

import { chatApi, type SentMessage } from "@/lib/api/endpoints";
import type { ApiError } from "@/lib/api/errors";
import type { ChatMessage, OffSessionAllowance } from "@/lib/api/types";
import { OutboxController } from "@/lib/chat/outbox-controller";
import type { OutboxEntry } from "@/lib/chat/outbox";
import type { ConnectionState, RealtimeClient } from "@/lib/realtime/client";
import { newClientMsgId } from "@/components/chat/chat-model";

type Options = {
  userId: string;
  /** The conversation on screen: messages are sent to it (the API routes them into the live chat or outside one, §22.3). */
  conversationId: string | null;
  /** The live session of that conversation, when one is known (sent along for older servers). */
  liveSessionId?: string | null;
  realtime: RealtimeClient | null;
  connection: ConnectionState;
  /** The server confirmed a message (merge it into the timeline). */
  onDelivered: (message: ChatMessage, allowance?: OffSessionAllowance) => void;
  /** A message stopped retrying automatically. */
  onFailed?: (entry: OutboxEntry, error: ApiError) => void;
};

export type Outbox = {
  entries: OutboxEntry[];
  /** Queue a text message for the conversation on screen; returns its client id. */
  send: (body: string) => string | null;
  /** Queue a failed message again; false when it is unknown. */
  retry: (clientMsgId: string) => boolean;
  discard: (clientMsgId: string) => void;
  /** Drop entries the server already confirmed (seen in history or events). */
  confirm: (clientMsgIds: Iterable<string>) => void;
};

const NONE: OutboxEntry[] = [];

/**
 * React binding of the outbox pump: text messages are queued (and kept in
 * sessionStorage) until the server confirms them, and sent in order while
 * the connection is open.
 */
export function useOutbox({ userId, conversationId, liveSessionId, realtime, connection, onDelivered, onFailed }: Options): Outbox {
  const controller = React.useMemo(() => (userId ? new OutboxController(userId) : null), [userId]);
  React.useEffect(() => {
    controller?.start();
    return () => controller?.stop();
  }, [controller]);
  // Sends go over the WebSocket when there is one (REST otherwise).
  React.useEffect(() => {
    controller?.configure(
      async (entry) => {
        // `session_id` is only sent for servers that predate conversation sends.
        const payload = { conversation_id: entry.conversation_id, body: entry.body, client_msg_id: entry.client_msg_id, ...(liveSessionId ? { session_id: liveSessionId } : {}) };
        return realtime
          ? await realtime.request<SentMessage>("message.send", payload)
          : await chatApi.sendToConversation(entry.conversation_id, entry.body, entry.client_msg_id);
      },
      { onDelivered: (message, _entry, allowance) => onDelivered(message, allowance), onFailed },
    );
  });

  const entries = React.useSyncExternalStore(
    controller?.subscribe ?? noopSubscribe,
    controller?.getSnapshot ?? emptySnapshot,
    emptySnapshot,
  );

  const ready = connection === "open";
  React.useEffect(() => {
    controller?.setTarget(conversationId, ready);
  }, [controller, conversationId, ready]);

  const send = React.useCallback(
    (body: string) => {
      if (!controller || !conversationId) return null;
      return controller.send({ client_msg_id: newClientMsgId(), conversation_id: conversationId, body }).client_msg_id;
    },
    [controller, conversationId],
  );

  const retry = React.useCallback(
    (cmid: string) => {
      if (!controller?.getSnapshot().some((e) => e.client_msg_id === cmid)) return false;
      controller.retry(cmid);
      return true;
    },
    [controller],
  );

  const discard = React.useCallback((cmid: string) => controller?.discard(cmid), [controller]);
  const confirm = React.useCallback((ids: Iterable<string>) => controller?.confirm(ids), [controller]);

  return { entries, send, retry, discard, confirm };
}

function noopSubscribe() {
  return () => undefined;
}

function emptySnapshot(): OutboxEntry[] {
  return NONE;
}
