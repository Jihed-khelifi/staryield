"use client";

/**
 * The signed-in client's invite link and counters (growth: referrals), shared
 * by every invite surface below the provider (the overview card, the
 * invite page, the post-reading prompt) so they read one request and one
 * "Copied" state.
 *
 * Server state: `GET /me/referrals` (loading, error and retry through
 * `useAsync`), only while `enabled` (the account shell enables it on the views
 * that show it). It is refreshed when a referral notification arrives
 * (`notification.created` of a `referral.*` kind: a friend joined, completed a
 * reading or minutes were added; the grant job always sends one, so
 * `wallet.updated` adds nothing), once per burst of events and at most once
 * per few seconds. A refresh keeps the last good copy on screen.
 */
import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import type { ApiError } from "@/lib/api/errors";
import { referralsApi, type MyReferrals } from "@/lib/api/referrals";
import type { AppNotification } from "@/lib/api/types";
import { useRealtimeEvents } from "@/lib/auth/session-provider";
import { canWebShare, copyText, shareInvite } from "@/lib/referrals/share";
import { useAsync } from "@/lib/use-async";
import { announce } from "@/components/feedback/live-region";
import { useToast } from "@/components/feedback/toast-provider";

/** How long the "Copied" state lasts. */
export const COPIED_MS = 2500;
/** Realtime refreshes are coalesced to one per this window. */
export const REFRESH_COALESCE_MS = 3000;
/** A refresh waits this long for the rest of a burst of events (it then reads them all at once). */
export const REFRESH_DEBOUNCE_MS = 400;

export type ReferralContextValue = {
  data: MyReferrals | null;
  error: ApiError | null;
  loading: boolean;
  reload: () => void;
  /** The link was just copied (resets after a moment). */
  copied: boolean;
  /** The browser offers the native share sheet. */
  canShare: boolean;
  /** Copy the invite link; false when the browser refused (the field then selects the link). */
  copyLink: () => Promise<boolean>;
  /** Share with the native sheet, or copy where there is none. */
  shareLink: () => Promise<void>;
};

const ReferralContext = React.createContext<ReferralContextValue | null>(null);

const REFERRAL_EVENTS = ["notification.created"] as const;
const noopSubscribe = () => () => {};

export function ReferralProvider({ children, enabled = true }: { children: React.ReactNode; enabled?: boolean }) {
  const { text } = useI18n();
  const toast = useToast();
  const state = useAsync((signal) => referralsApi.mine(signal), [], enabled);
  const { data, reload } = state;
  const [copied, setCopied] = React.useState(false);
  const copiedTimer = React.useRef<number | undefined>(undefined);
  const lastRefresh = React.useRef(0);
  const pendingRefresh = React.useRef<number | undefined>(undefined);
  // Read on the client only (the server render has no navigator).
  const canShare = React.useSyncExternalStore(noopSubscribe, () => canWebShare(), () => false);

  React.useEffect(
    () => () => {
      window.clearTimeout(copiedTimer.current);
      window.clearTimeout(pendingRefresh.current);
    },
    [],
  );

  const refresh = React.useCallback(() => {
    if (pendingRefresh.current !== undefined) return;
    const wait = Math.max(REFRESH_DEBOUNCE_MS, lastRefresh.current + REFRESH_COALESCE_MS - Date.now());
    pendingRefresh.current = window.setTimeout(() => {
      pendingRefresh.current = undefined;
      lastRefresh.current = Date.now();
      reload();
    }, wait);
  }, [reload]);

  useRealtimeEvents<{ notification?: AppNotification }>(REFERRAL_EVENTS, (type, event) => {
    if (!enabled || type !== "notification.created") return;
    if (event?.notification?.kind?.startsWith("referral.")) refresh();
  });

  const markCopied = React.useCallback(() => {
    setCopied(true);
    window.clearTimeout(copiedTimer.current);
    copiedTimer.current = window.setTimeout(() => setCopied(false), COPIED_MS);
  }, []);

  const copyLink = React.useCallback(async () => {
    if (!data) return false;
    const method = await copyText(data.link);
    if (!method) {
      toast.show({ tone: "error", message: text("Couldn't copy the link. Select it and copy it yourself.") });
      return false;
    }
    markCopied();
    announce(text("Invite link copied"));
    track("referral_link_copied");
    return true;
  }, [data, markCopied, text, toast]);

  const shareLink = React.useCallback(async () => {
    if (!data) return;
    const outcome = await shareInvite({
      url: data.link,
      title: text("Join me on Staryield"),
      text: text("I use Staryield for live readings with psychic advisors. Join with my link:"),
    });
    if (outcome.result === "cancelled") return;
    if (outcome.result === "failed") {
      toast.show({ tone: "error", message: text("Couldn't copy the link. Select it and copy it yourself.") });
      return;
    }
    if (outcome.result === "copied") {
      markCopied();
      announce(text("Invite link copied"));
    } else {
      announce(text("Thank you for sharing Staryield"));
    }
    track("referral_link_shared", { method: outcome.method });
  }, [data, markCopied, text, toast]);

  const value = React.useMemo<ReferralContextValue>(
    () => ({ data: state.data, error: state.error, loading: state.loading, reload: state.reload, copied, canShare, copyLink, shareLink }),
    [state.data, state.error, state.loading, state.reload, copied, canShare, copyLink, shareLink],
  );
  return <ReferralContext.Provider value={value}>{children}</ReferralContext.Provider>;
}

/** The invite state of the nearest ReferralProvider. */
export function useReferrals(): ReferralContextValue {
  const ctx = React.useContext(ReferralContext);
  if (!ctx) throw new Error("useReferrals must be used inside <ReferralProvider>");
  return ctx;
}

/** The nearest provider's state, or null outside any (components that mount their own). */
export function useOptionalReferrals(): ReferralContextValue | null {
  return React.useContext(ReferralContext);
}
