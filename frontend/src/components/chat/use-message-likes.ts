"use client";

import * as React from "react";

import { chatApi } from "@/lib/api/endpoints";
import { toApiError, type ApiError } from "@/lib/api/errors";
import type { MessageLikes } from "@/lib/api/types";
import { isActionable, type UiMessage } from "@/components/chat/chat-model";

/** The messages with optimistic likes laid over them (`override`: message id → liked_by). */
export function withLikeOverrides(messages: UiMessage[], override: Record<string, string[]>): UiMessage[] {
  if (Object.keys(override).length === 0) return messages;
  return messages.map((m) => (m.id in override ? { ...m, liked_by: override[m.id] } : m));
}

type Options = {
  /** The signed-in user. */
  meId: string;
  /** The confirmed messages of the conversation on screen (the server state, with `liked_by`). */
  messages: UiMessage[];
  /** The API's answer is authoritative: put it on the timeline. */
  applyLikes: (likes: MessageLikes) => void;
  /** A like or unlike failed and was rolled back; `retry` repeats it. */
  onFailed: (error: ApiError, retry: () => void) => void;
};

/**
 * Client state of likes (§23.3): the server state (`liked_by`) lives on the
 * timeline messages; this holds only the optimistic overrides until the API
 * answers. Success replaces the override with the answer, failure rolls it
 * back and reports the error with a way to try again. One request per message
 * at a time (a second toggle while one runs is ignored).
 */
export function useMessageLikes({ meId, messages, applyLikes, onFailed }: Options) {
  const [override, setOverride] = React.useState<Record<string, string[]>>({});
  const latest = React.useRef({ messages, override, applyLikes, onFailed });
  React.useEffect(() => {
    latest.current = { messages, override, applyLikes, onFailed };
  });
  const busy = React.useRef(new Set<string>());

  const toggle = React.useCallback(
    function toggle(m: UiMessage) {
      if (!meId || !isActionable(m) || busy.current.has(m.id)) return;
      const { messages: known, override: pending } = latest.current;
      const current = pending[m.id] ?? known.find((x) => x.id === m.id)?.liked_by ?? m.liked_by ?? [];
      const liking = !current.includes(meId);
      const next = liking ? [...current, meId] : current.filter((id) => id !== meId);
      const clear = () =>
        setOverride((cur) => (m.id in cur ? Object.fromEntries(Object.entries(cur).filter(([id]) => id !== m.id)) : cur));
      busy.current.add(m.id);
      setOverride((cur) => ({ ...cur, [m.id]: next }));
      (liking ? chatApi.like(m.id) : chatApi.unlike(m.id))
        .then(
          (likes) => {
            latest.current.applyLikes(likes);
            clear();
          },
          (err) => {
            clear();
            latest.current.onFailed(toApiError(err), () => toggle(m));
          },
        )
        .finally(() => busy.current.delete(m.id));
    },
    [meId],
  );

  return { override, toggle };
}
