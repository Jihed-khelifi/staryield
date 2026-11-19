"use client";
import { useCallback, useEffect, useRef } from "react";

import type { PresenceStatus } from "@/lib/api/types";
import { useRealtimeEvent } from "@/lib/auth/session-provider";

/**
 * Live advisor status on the directory and the home selection (revisions 6.0,
 * §26.3). The server orders advisors online, away, busy, offline, then by
 * rating; a status change moves the visible card between those tiers at once
 * (stable, so the server's order is kept inside a tier) and the list is
 * refetched at most every {@link STATUS_REFETCH_MS}.
 */

/** Least time between two refetches caused by status changes. */
export const STATUS_REFETCH_MS = 5_000;

/** Lower comes first. Mirrors `statusTier` in the API's catalogue. */
export const STATUS_TIER: Record<PresenceStatus, number> = { online: 0, away: 1, busy: 2, offline: 3 };

/** Orders cards by status tier; cards of one tier keep their relative order. */
export function sortByStatusTier<T extends { status: PresenceStatus }>(items: readonly T[]): T[] {
  return items
    .map((item, index) => ({ item, index }))
    .sort((a, b) => STATUS_TIER[a.item.status] - STATUS_TIER[b.item.status] || a.index - b.index)
    .map(({ item }) => item);
}

/**
 * Applies a status change to one advisor and re-sorts. `patch` adds derived
 * fields (the home card's call to action). The list is returned unchanged
 * when the advisor is not in it or already has that status.
 */
export function applyStatusChange<T extends { slug: string; status: PresenceStatus }>(
  items: readonly T[],
  slug: string,
  status: PresenceStatus,
  patch?: (item: T) => T,
): T[] {
  const known = items.find((a) => a.slug === slug);
  if (!known || known.status === status) return items as T[];
  return sortByStatusTier(items.map((a) => (a.slug === slug ? (patch ? patch({ ...a, status }) : { ...a, status }) : a)));
}

/**
 * Returns `notify`: each call schedules one run of `run`, no sooner than
 * `gapMs` after the previous run; calls made while a run is already
 * scheduled share it. The first call after a quiet period runs at once.
 */
export function useThrottledRun(run: () => void, gapMs = STATUS_REFETCH_MS): () => void {
  const latest = useRef(run);
  const lastRun = useRef(Number.NEGATIVE_INFINITY);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    latest.current = run;
  });
  useEffect(
    () => () => {
      if (timer.current !== null) clearTimeout(timer.current);
      timer.current = null;
    },
    [],
  );
  return useCallback(() => {
    if (timer.current !== null) return;
    const wait = Math.max(0, lastRun.current + gapMs - Date.now());
    timer.current = setTimeout(() => {
      timer.current = null;
      lastRun.current = Date.now();
      latest.current();
    }, wait);
  }, [gapMs]);
}

/**
 * Subscribes to `psychic.status_changed`: `update` applies each change to the
 * cards on screen and `refetch` (when given) reloads the list, throttled.
 */
export function useStatusChanges(update: (slug: string, status: PresenceStatus) => void, refetch?: () => void) {
  const notify = useThrottledRun(() => refetch?.());
  useRealtimeEvent<{ slug: string; status: PresenceStatus }>("psychic.status_changed", (d) => {
    update(d.slug, d.status);
    if (refetch) notify();
  });
}
