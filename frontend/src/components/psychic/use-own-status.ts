"use client";

import * as React from "react";

import { psychicsApi } from "@/lib/api/endpoints";
import type { Availability, PresenceStatus } from "@/lib/api/types";
import { useRealtimeEvent, useSession } from "@/lib/auth/session-provider";
import { useToast } from "@/components/feedback/toast-provider";

type StatusEvent = { psychic_id?: string; slug?: string; status?: PresenceStatus };

export type OwnStatus = {
  /** What the advisor chose: online, away or offline. */
  availability: Availability;
  /** What clients see: busy while a live chat is open. */
  status: PresenceStatus;
  /** True while a change is being saved. */
  saving: boolean;
  setAvailability: (next: Availability) => Promise<void>;
};

/** Busy is computed by the server while a session is open; it cannot be chosen. */
export function isBusy(status: PresenceStatus | undefined) {
  return status === "busy";
}

/**
 * The signed-in advisor's own availability and effective status. The chosen
 * availability lives in the session (shown optimistically while saving and
 * rolled back on failure); the effective status comes from the API and from
 * `psychic.status_changed` events, so "Busy" appears as soon as a live chat
 * starts and clears when it ends.
 */
export function useOwnStatus(): OwnStatus {
  const { user, setUser } = useSession();
  const toast = useToast();
  const self = user?.psychic;
  const slug = self?.slug;
  const [pending, setPending] = React.useState<Availability | null>(null);
  const [reported, setReported] = React.useState<PresenceStatus | null>(null);
  const availability: Availability = pending ?? self?.availability ?? "offline";
  const status: PresenceStatus = reported ?? availability;

  React.useEffect(() => {
    if (!slug) return;
    const controller = new AbortController();
    psychicsApi
      .get(slug, controller.signal)
      .then((p) => setReported(p.status))
      .catch(() => undefined); // keep showing the chosen availability
    return () => controller.abort();
  }, [slug]);

  useRealtimeEvent<StatusEvent>("psychic.status_changed", (event) => {
    if (!event || !slug || event.slug !== slug || !event.status) return;
    setReported(event.status);
  });

  const setAvailability = React.useCallback(
    async (next: Availability) => {
      if (!user?.psychic || next === availability) return;
      setPending(next);
      try {
        const p = await psychicsApi.setAvailability(next);
        setReported(p.status);
        setUser({ ...user, psychic: { ...user.psychic, availability: next } });
      } catch (err) {
        toast.showError(err);
      } finally {
        setPending(null);
      }
    },
    [availability, setUser, toast, user],
  );

  return { availability, status, saving: pending !== null, setAvailability };
}
