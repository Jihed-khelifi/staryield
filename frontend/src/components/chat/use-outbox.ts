"use client";

import * as React from "react";

import { chatApi } from "@/lib/api/endpoints";
import type { ApiError } from "@/lib/api/errors";
import type { ChatMessage, ChatSession } from "@/lib/api/types";
import { OutboxController } from "@/lib/chat/outbox-controller";
import type { OutboxEntry } from "@/lib/chat/outbox";
import type { ConnectionState, RealtimeClient } from "@/lib/realtime/client";
import { isOpen, newClientMsgId } from "@/components/chat/chat-model";

type Options = {
  userId: string;
  /** The open session of the conversation on screen (sends go to it). */
  session: ChatSession | null;
  realtime: RealtimeClient | null;
  connection: ConnectionState;
  /** The server confirmed a message (merge it into the timeline). */
  onDelivered: (message: ChatMessage) => void;
  /** A message stopped retrying automatically. */
  onFailed?: (entry: OutboxEntry, error: ApiError) => void;
};

export type Outbox = {
  entries: OutboxEntry[];
  /** Queue a text message for the open session; returns its client id. */
  send: (body: string) => string | null;
  /** Queue a failed message again; false when no session can take it. */
  retry: (clientMsgId: string) => boolean;
  discard: (clientMsgId: string) => void;
  /** Drop entries the server already confirmed (seen in history or events). */
  confirm: (clientMsgIds: Iterable<string>) => void;
};

const NONE: OutboxEntry[] = [];

/**
 * React binding of the outbox pump: text messages are queued (and kept in
 * sessionStorage) until the server confirms them, and sent in order while
 * the connection is open and the session is active.
 */
export function useOutbox({ userId, session, realtime, connection, onDelivered, onFailed }: Options): Outbox {
  const controller = React.useMemo(() => (userId ? new OutboxController(userId) : null), [userId]);
  React.useEffect(() => {
    controller?.start();
    return () => controller?.stop();
  }, [controller]);
  // Sends go over the WebSocket when there is one (REST otherwise).
  React.useEffect(() => {
    controller?.configure(
      async (entry) => {
        const payload = { session_id: entry.session_id, body: entry.body, client_msg_id: entry.client_msg_id };
        const res = realtime ? await realtime.request<{ message: ChatMessage }>("message.send", payload) : await chatApi.send(entry.session_id, entry.body, entry.client_msg_id);
        return res.message;
      },
      { onDelivered: (message) => onDelivered(message), onFailed },
    );
  });

  const entries = React.useSyncExternalStore(
    controller?.subscribe ?? noopSubscribe,
    controller?.getSnapshot ?? emptySnapshot,
    emptySnapshot,
  );

  const sessionId = session?.id ?? null;
  const ready = connection === "open" && session?.status === "active";
  React.useEffect(() => {
    controller?.setTarget(sessionId, ready);
  }, [controller, sessionId, ready]);

  // A session that ended can no longer take what it still holds.
  const closedSession = session && !isOpen(session) ? session.id : null;
  React.useEffect(() => {
    if (closedSession) controller?.sessionClosed(closedSession);
  }, [controller, closedSession]);

  const send = React.useCallback(
    (body: string) => {
      if (!controller || !session) return null;
      return controller.send({ client_msg_id: newClientMsgId(), conversation_id: session.conversation_id, session_id: session.id, body }).client_msg_id;
    },
    [controller, session],
  );

  const retry = React.useCallback(
    (cmid: string) => {
      const entry = controller?.getSnapshot().find((e) => e.client_msg_id === cmid);
      // Resend into the conversation's open session (it may be a new one).
      if (!controller || !entry || !session || session.conversation_id !== entry.conversation_id || !isOpen(session)) return false;
      controller.retry(cmid, session.id);
      return true;
    },
    [controller, session],
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
