"use client";

/**
 * The viewer's waiting lists, shared by every booking button on a page.
 *
 * Before this, each surface (home cards, profile, chat room) read
 * `GET /me/waitlist` on its own. Here one provider reads it at most once per
 * page for a signed-in client, and only when a button actually needs it (an
 * advisor who cannot talk right now), then serves joined/pending state and
 * the join and leave actions to all buttons. Mount it where booking surfaces
 * live: the home cards, the directory, the profile, the chat room's waiting
 * card and `AdvisorStrip` (`WaitlistScope` reuses a parent's). It is not
 * app-wide because most pages have no booking button, and a provider above
 * every page would keep one list alive for the whole visit while the state
 * it holds is only accurate for the page that read it.
 */
import * as React from "react";

import { waitlistApi } from "@/lib/api/endpoints";
import { toApiError, type ApiError } from "@/lib/api/errors";
import type { AppNotification, PresenceStatus } from "@/lib/api/types";
import { useRealtimeEvent, useSession } from "@/lib/auth/session-provider";
import { useAsync } from "@/lib/use-async";

/** Open waiting lists per client are capped at 20 server-side, so one page of 50 holds them all. */
const MINE_LIMIT = 50;

/**
 * How long a 409 `advisor_available` outranks the status the page shows,
 * unless a live status change for that advisor comes first. Longer than the
 * API's advisor catalogue snapshot (10 s), so a list reloaded meanwhile cannot
 * bring the stale status back; after it, the page's own status (live or
 * polled) is the truth again.
 */
export const AVAILABLE_HOLD_MS = 15_000;

export type WaitlistResult = { ok: true } | { ok: false; error: ApiError };

export type WaitlistValue = {
  /** The viewer is a signed-in client: the only role that can wait. */
  enabled: boolean;
  /**
   * `unknown` until `GET /me/waitlist` answers (for a client). A failed read
   * counts as not joined: joining is idempotent, so the button stays usable.
   */
  joined: (slug: string) => "joined" | "not_joined" | "unknown";
  pending: (slug: string) => "join" | "leave" | null;
  join: (slug: string) => Promise<WaitlistResult>;
  leave: (slug: string) => Promise<WaitlistResult>;
  /**
   * The server said this advisor is online (409 `advisor_available`) while the
   * page still shows `shownStatus`: every button for that advisor shows the
   * chat while the page shows that status, until the advisor's next live
   * status change or for {@link AVAILABLE_HOLD_MS} at most.
   */
  markAvailable: (slug: string, shownStatus: PresenceStatus) => void;
  availableFrom: (slug: string) => PresenceStatus | null;
  /** Says a button needs the lists. The first call starts the one read; later calls do nothing. */
  demand: () => void;
};

const WaitlistContext = React.createContext<WaitlistValue | null>(null);

export function WaitlistProvider({ children }: { children: React.ReactNode }) {
  const { status, user } = useSession();
  const enabled = status === "authenticated" && user?.role === "client";
  // Latched: once a button needed the lists they are read, and only once
  // (an advisor going online and back does not read them again).
  const [wanted, setWanted] = React.useState(false);
  const mine = useAsync((signal) => waitlistApi.mine({ limit: MINE_LIMIT }, signal), [user?.id ?? ""], enabled && wanted);
  const [overrides, setOverrides] = React.useState<Record<string, boolean>>({});
  const [pendingBy, setPendingBy] = React.useState<Record<string, "join" | "leave">>({});
  const [available, setAvailable] = React.useState<Record<string, PresenceStatus>>({});
  // What this page learnt belongs to the account that learnt it (sign-out or another sign-in starts over).
  const owner = user?.id ?? "";
  const [ownerSeen, setOwnerSeen] = React.useState(owner);
  if (ownerSeen !== owner) {
    setOwnerSeen(owner);
    setOverrides({});
    setPendingBy({});
    setAvailable({});
  }

  const fetched = React.useMemo(() => new Set((mine.data?.items ?? []).map((e) => e.slug)), [mine.data]);

  const forgetAvailable = React.useCallback((slug: string) => {
    setAvailable((m) => {
      if (!(slug in m)) return m;
      const rest = { ...m };
      delete rest[slug];
      return rest;
    });
  }, []);
  // One hold timer per advisor; all cleared when the page goes.
  const holds = React.useRef(new Map<string, ReturnType<typeof setTimeout>>());
  React.useEffect(() => {
    const timers = holds.current;
    return () => timers.forEach((t) => clearTimeout(t));
  }, []);
  const markAvailable = React.useCallback(
    (slug: string, shownStatus: PresenceStatus) => {
      setAvailable((m) => (m[slug] === shownStatus ? m : { ...m, [slug]: shownStatus }));
      clearTimeout(holds.current.get(slug));
      holds.current.set(
        slug,
        setTimeout(() => {
          holds.current.delete(slug);
          forgetAvailable(slug);
        }, AVAILABLE_HOLD_MS),
      );
    },
    [forgetAvailable],
  );
  // A live status change is newer than the 409: the page's status is the truth from then on.
  useRealtimeEvent<{ slug?: string }>("psychic.status_changed", (d) => {
    if (typeof d?.slug === "string") forgetAvailable(d.slug);
  });

  // The server closes an entry when it tells the client the advisor is online
  // (kind `waitlist.available`, always in-app): every tab drops it at once.
  // Not on the status change itself: the entry stays open while the advisor
  // has not been online long enough to notify (docs §9.7).
  useRealtimeEvent<{ notification?: AppNotification }>("notification.created", (d) => {
    const n = d?.notification;
    const slug = n?.kind === "waitlist.available" ? n.data?.psychic_slug : undefined;
    if (typeof slug === "string" && slug) setOverrides((m) => (m[slug] === false ? m : { ...m, [slug]: false }));
  });

  const joined = React.useCallback(
    (slug: string): "joined" | "not_joined" | "unknown" => {
      if (slug in overrides) return overrides[slug] ? "joined" : "not_joined";
      if (mine.data) return fetched.has(slug) ? "joined" : "not_joined";
      if (!enabled || mine.error) return "not_joined";
      return "unknown";
    },
    [overrides, mine.data, mine.error, fetched, enabled],
  );

  const run = React.useCallback(async (slug: string, kind: "join" | "leave"): Promise<WaitlistResult> => {
    setPendingBy((m) => ({ ...m, [slug]: kind }));
    try {
      await (kind === "join" ? waitlistApi.join(slug) : waitlistApi.leave(slug));
      setOverrides((m) => ({ ...m, [slug]: kind === "join" }));
      return { ok: true };
    } catch (err) {
      return { ok: false, error: toApiError(err) };
    } finally {
      setPendingBy((m) => {
        const rest = { ...m };
        delete rest[slug];
        return rest;
      });
    }
  }, []);

  // Stable: buttons list it as an effect dependency.
  const demand = React.useCallback(() => setWanted(true), []);

  const value = React.useMemo<WaitlistValue>(
    () => ({
      enabled,
      joined,
      pending: (slug) => pendingBy[slug] ?? null,
      join: (slug) => run(slug, "join"),
      leave: (slug) => run(slug, "leave"),
      markAvailable,
      availableFrom: (slug) => available[slug] ?? null,
      demand,
    }),
    [enabled, joined, pendingBy, run, markAvailable, available, demand],
  );
  return <WaitlistContext.Provider value={value}>{children}</WaitlistContext.Provider>;
}

export function useWaitlist(): WaitlistValue {
  const value = React.useContext(WaitlistContext);
  if (!value) throw new Error("useWaitlist needs a WaitlistProvider above it");
  return value;
}

/** Wraps `children` in a provider unless a parent already shares one (so a strip placed on a page that has its own reuses it). */
export function WaitlistScope({ children }: { children: React.ReactNode }) {
  const parent = React.useContext(WaitlistContext);
  return parent ? <>{children}</> : <WaitlistProvider>{children}</WaitlistProvider>;
}
