"use client";

import { useState } from "react";

import { chatApi } from "@/lib/api/endpoints";
import type { PresenceStatus } from "@/lib/api/types";
import { useSession } from "@/lib/auth/session-provider";
import { useAsync } from "@/lib/use-async";
import { OPEN_STATUSES } from "@/components/chat/chat-model";

/** The session statuses of a reading that is still open (`requested,accepted,...`). */
const OPEN_FILTER = [...OPEN_STATUSES].join(",");

/**
 * Whether the signed-in client is in an open reading with this advisor, so
 * the profile offers the way back to it instead of the advisor's waiting
 * list. An advisor in a reading shows busy and a client has at most one open
 * session, so it is read only for a client looking at a busy advisor (one
 * small request, none for everyone else), and again each time the status
 * changes (the reading may have ended, or another one started).
 */
export function useOpenReading(slug: string, status: PresenceStatus | undefined): boolean {
  const { status: sessionStatus, user } = useSession();
  const enabled = sessionStatus === "authenticated" && user?.role === "client" && status === "busy";
  // Every status change starts a new read (a reading may have ended and another begun since).
  const [seen, setSeen] = useState(status);
  const [changes, setChanges] = useState(0);
  if (seen !== status) {
    setSeen(status);
    setChanges((n) => n + 1);
  }
  const open = useAsync(() => chatApi.sessions(OPEN_FILTER), [slug, user?.id ?? "", changes], enabled);
  return enabled && !open.loading && (open.data?.items ?? []).some((s) => s.psychic.slug === slug && OPEN_STATUSES.has(s.status));
}
