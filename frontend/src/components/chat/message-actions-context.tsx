"use client";

import * as React from "react";

import type { ReplyPreview } from "@/lib/api/types";
import type { UiMessage } from "@/components/chat/chat-model";

/**
 * What the messages of the conversation on screen can do (§23.2, §23.3). The
 * chatroom shell owns the state (reply targets, optimistic likes, the jump to
 * an older message) and provides it here, so bubbles and the composer read it
 * from context instead of receiving it through the feed.
 */
export type MessageActions = {
  /** A chatroom provides the actions (false in isolation: bubbles then show no action buttons). */
  enabled: boolean;
  /** The signed-in user. */
  viewerId: string;
  /** The message the composer is about to answer in the conversation on screen. */
  replyTarget: ReplyPreview | null;
  /** Start answering a message (focuses the composer). */
  reply: (message: UiMessage) => void;
  /** Stop answering. */
  cancelReply: () => void;
  /** Like or unlike (optimistic; failure rolls back with a toast). */
  toggleLike: (message: UiMessage) => void;
  /** Scroll to the quoted message and highlight it, loading older pages when needed. */
  jumpTo: (quote: ReplyPreview) => void;
  /** The quote being looked for (for a loading state on its block), if any. */
  jumpingTo: string | null;
  /** Display name of a participant (advisors see their private nicknames for clients). */
  nameOf: (senderId: string | undefined) => string;
};

const NOOP = () => undefined;

/** Used outside the chatroom (and in isolated tests): nothing to act on. */
const DEFAULT: MessageActions = {
  enabled: false,
  viewerId: "",
  replyTarget: null,
  reply: NOOP,
  cancelReply: NOOP,
  toggleLike: NOOP,
  jumpTo: NOOP,
  jumpingTo: null,
  nameOf: () => "",
};

const MessageActionsContext = React.createContext<MessageActions>(DEFAULT);

export function MessageActionsProvider({ value, children }: { value: MessageActions; children: React.ReactNode }) {
  return <MessageActionsContext.Provider value={value}>{children}</MessageActionsContext.Provider>;
}

export function useMessageActions(): MessageActions {
  return React.useContext(MessageActionsContext);
}
