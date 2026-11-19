"use client";

import * as React from "react";

import type { ChatHold } from "@/lib/api/holds";
import type { ChatSession } from "@/lib/api/types";
import { EMPTY_HOLD_STORE, currentHold, holdStoreReducer } from "@/lib/chat/hold";

export type ChatHoldState = {
  /** The hold of the session on screen: its own copy or the newer one events and API answers brought. */
  hold: ChatHold | null;
  /** A hold (event or API answer): only ever moves it forward. */
  apply: (hold: ChatHold) => void;
  /** `session.hold_extended`: the deadline moved out. */
  extend: (holdId: string, expiresAt: string) => void;
  /** Newest known copy of any hold (a hold of an ended reading comes from `GET /chat/holds/:id`). */
  holdById: (holdId: string) => ChatHold | undefined;
};

/**
 * The hold of the conversation on screen, kept in one place. The session view
 * carries the hold it had when it was read; events (`session.held`,
 * `session.hold_extended`, `session.hold_ended`) and API answers (continue,
 * reveal, `GET /chat/holds/:id`) are folded into a small store, and the newer
 * copy wins (`lib/chat/hold`), so a late event can never bring an older state back.
 */
export function useChatHoldState(session: ChatSession | null): ChatHoldState {
  const [store, dispatch] = React.useReducer(holdStoreReducer, EMPTY_HOLD_STORE);
  const sessionHold = session?.hold;
  const hold = React.useMemo(() => currentHold(sessionHold, store), [sessionHold, store]);
  const latest = React.useRef({ store, sessionHold });
  React.useEffect(() => {
    latest.current = { store, sessionHold };
  });
  const apply = React.useCallback((incoming: ChatHold) => dispatch({ type: "hold", hold: incoming }), []);
  const extend = React.useCallback((holdId: string, expiresAt: string) => dispatch({ type: "extended", holdId, expiresAt }), []);
  const holdById = React.useCallback((holdId: string) => {
    const { store: s, sessionHold: own } = latest.current;
    const known = s.holds[holdId];
    if (own?.id === holdId) return currentHold(own, s) ?? undefined;
    return known;
  }, []);
  return { hold, apply, extend, holdById };
}
