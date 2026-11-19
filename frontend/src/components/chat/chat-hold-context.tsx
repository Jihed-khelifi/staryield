"use client";

import * as React from "react";

import type { ChatHold } from "@/lib/api/holds";

/**
 * revisions 6.0 (§26.1): what the free-trial hold of the conversation on
 * screen offers to the messages, the banner and the session card. The
 * chatroom shell owns the state (`use-chat-hold`, `use-hold-actions`) and
 * provides it here, so a locked bubble reads it from context instead of
 * receiving it through the feed.
 */
export type ChatHoldValue = {
  /** Clients get the reveal and top-up actions; advisors only see the hold. */
  role: "client" | "psychic";
  /** The open hold of the reading on screen, else its latest expired or revealed one. */
  hold: ChatHold | null;
  /** The other participant, as the viewer sees them (advisors: their nickname for the client). */
  counterpartName: string;
  /** Server clock minus device clock in ms: countdowns run to the server's deadline. */
  clockOffsetMs: number;
  /** Client: messages that are still hidden after the hold ended (the "Top up to read" prompt), else null. */
  hidden: { holdId: string; count: number } | null;
  /** A click on a locked message or on the "Top up to read" prompt: top-up while the hold is open, reveal after it expired. */
  openLocked: (holdId: string | undefined) => void;
  /** Client: open the top-up dialog for the open hold. */
  openTopUp: () => void;
  /** A hold lookup or a reveal request is running. */
  busy: boolean;
  /** Client: the wallet covers a minute but the hold did not convert in time ("Continue reading"). */
  continueOffered: boolean;
  continuing: boolean;
  continueReading: () => void;
};

const NOOP = () => undefined;

/** Used outside the chatroom (and in isolated tests): no hold. */
const DEFAULT: ChatHoldValue = {
  role: "client",
  hold: null,
  counterpartName: "",
  clockOffsetMs: 0,
  hidden: null,
  openLocked: NOOP,
  openTopUp: NOOP,
  busy: false,
  continueOffered: false,
  continuing: false,
  continueReading: NOOP,
};

const ChatHoldContext = React.createContext<ChatHoldValue>(DEFAULT);

export function ChatHoldProvider({ value, children }: { value: ChatHoldValue; children: React.ReactNode }) {
  return <ChatHoldContext.Provider value={value}>{children}</ChatHoldContext.Provider>;
}

export function useChatHold(): ChatHoldValue {
  return React.useContext(ChatHoldContext);
}
