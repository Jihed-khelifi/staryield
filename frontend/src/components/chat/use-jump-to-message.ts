"use client";

import * as React from "react";

import type { ReplyPreview } from "@/lib/api/types";
import type { LoadOlderResult } from "@/components/chat/use-conversation-timeline";

/** Older pages loaded at most while looking for the message a reply quotes (§23.2). */
export const JUMP_MAX_PAGES = 10;
/** Waiting for another load of older messages to finish, at most this many times. */
export const JUMP_BUSY_TRIES = 24;
export const JUMP_BUSY_MS = 250;
/** After a page arrives React needs a moment to paint it: look for the row a few times. */
const SETTLE_TRIES = 5;
const SETTLE_MS = 60;

export type JumpFailure = "unavailable" | "failed";

type Options = {
  /** Scroll to a message that is on screen and highlight it; false when it is not loaded. */
  scrollTo: (id: string) => boolean;
  /** Load the page of older messages (the timeline's own bookkeeping). */
  loadOlder: () => Promise<LoadOlderResult>;
  /** The conversation on screen; a jump is abandoned when it changes. */
  conversationId: string | null;
  /** The search for an older message began (for a screen-reader status). */
  onSearching?: () => void;
  /** The message could not be reached; `retry` looks again (not offered for "unavailable"). */
  onFailed: (reason: JumpFailure, retry: (() => void) | null) => void;
};

/**
 * Quote → original message (§23.2). Scrolls to it when it is on screen;
 * otherwise loads older pages, bounded (`JUMP_MAX_PAGES`), until it appears.
 * `jumpingTo` is the quoted message id while that search runs (a visible
 * loading state on the quote). Failure is reported, with a retry.
 */
export function useJumpToMessage(options: Options) {
  const [jumpingTo, setJumpingTo] = React.useState<string | null>(null);
  const latest = React.useRef(options);
  React.useEffect(() => {
    latest.current = options;
  });
  const running = React.useRef(false);

  const jumpTo = React.useCallback(async function jumpTo(quote: ReplyPreview): Promise<void> {
    if (latest.current.scrollTo(quote.id)) return;
    if (running.current) return;
    const conversation = latest.current.conversationId;
    const stillHere = () => latest.current.conversationId === conversation;
    const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));
    const settle = async () => {
      for (let i = 0; i < SETTLE_TRIES; i++) {
        await sleep(SETTLE_MS);
        if (latest.current.scrollTo(quote.id)) return true;
      }
      return false;
    };
    running.current = true;
    setJumpingTo(quote.id);
    latest.current.onSearching?.();
    try {
      let busy = 0;
      for (let pages = 0; pages < JUMP_MAX_PAGES; ) {
        const result = await latest.current.loadOlder();
        if (!stillHere()) return;
        if (!result.ok) {
          if (result.reason === "busy" && ++busy <= JUMP_BUSY_TRIES) {
            // The feed's own "load earlier" is running: wait for it, then look.
            await sleep(JUMP_BUSY_MS);
            if (!stillHere()) return;
            if (latest.current.scrollTo(quote.id)) return;
            continue;
          }
          if (result.reason === "none") latest.current.onFailed("unavailable", null);
          else latest.current.onFailed("failed", () => void jumpTo(quote));
          return;
        }
        pages++;
        if (await settle()) return;
        if (!stillHere()) return;
      }
      latest.current.onFailed("failed", () => void jumpTo(quote));
    } finally {
      running.current = false;
      setJumpingTo(null);
    }
  }, []);

  return { jumpTo, jumpingTo };
}
