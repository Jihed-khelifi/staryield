"use client";
import { useCallback, useEffect } from "react";

import { psychicsApi } from "@/lib/api/endpoints";
import type { FeaturedList, PresenceStatus } from "@/lib/api/types";
import { applyStatusChange, useStatusChanges } from "@/lib/psychics-live";
import { useAsync, type AsyncState } from "@/lib/use-async";

/** How often a featured selection refreshes while the page is visible. */
export const FEATURED_REFRESH_MS = 30_000;

/** Statuses can change between refreshes: the call to action follows them. */
export function withStatus(list: FeaturedList, slug: string, status: PresenceStatus): FeaturedList {
  const cta = status === "online" ? ("chat" as const) : ("waitlist" as const);
  const items = applyStatusChange(list.items, slug, status, (a) => ({ ...a, cta }));
  return items === list.items ? list : { ...list, items };
}

export type FeaturedAdvisors = AsyncState<FeaturedList> & {
  /** The server said this advisor is online (409 `advisor_available`): the card, its badge and the order follow at once. */
  markOnline: (slug: string) => void;
};

/**
 * A live selection of advisors, the advisors who can talk now first
 * (`GET /psychics/featured`), shared by the home cards and `AdvisorStrip`.
 * It stays current for everyone: a refresh every 30 seconds while the page is
 * visible (paused in background tabs, and the only source for visitors, who
 * have no realtime connection), and for members live status changes that
 * re-sort the cards at once and refetch the selection, at most every 5 s,
 * since another advisor may now belong among the first.
 */
export function useFeaturedAdvisors(count: number): FeaturedAdvisors {
  const featured = useAsync((signal) => psychicsApi.featured(count, signal), [count]);
  const { reload, setData } = featured;

  useEffect(() => {
    const tick = () => {
      if (document.visibilityState === "visible") reload();
    };
    const id = window.setInterval(tick, FEATURED_REFRESH_MS);
    document.addEventListener("visibilitychange", tick);
    return () => {
      window.clearInterval(id);
      document.removeEventListener("visibilitychange", tick);
    };
  }, [reload]);

  useStatusChanges((slug, status) => setData((list) => (list ? withStatus(list, slug, status) : list)), reload);

  // A patch, not a reload: the API's selection may lag the 409 by a few
  // seconds and would put the stale status back on the card.
  const markOnline = useCallback((slug: string) => setData((list) => (list ? withStatus(list, slug, "online") : list)), [setData]);
  return { ...featured, markOnline };
}
