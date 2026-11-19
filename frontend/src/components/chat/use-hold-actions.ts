"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { newId } from "@/lib/api/client";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import { holdsApi, type ChatHold, type HoldContinueResult, type HoldRevealResult } from "@/lib/api/holds";
import type { Wallet } from "@/lib/api/types";
import { logger } from "@/lib/logger";
import { useToast } from "@/components/feedback/toast-provider";

/** How long a covered wallet waits for the hold to convert by itself before "Continue reading" is offered. */
export const CONTINUE_OFFER_DELAY_MS = 5000;
/** The dialog closes right after a purchase, a moment before the wallet shows it: wait this long before giving up the reveal. */
const TOP_UP_SETTLE_MS = 2500;

/** What the confirm dialog shows: "Reveal {count} hidden messages from {name} for {price} credits?" */
export type RevealConfirm = { holdId: string; price: number; count: number; error: string | null };
type Target = Omit<RevealConfirm, "error">;

type Options = {
  role: "client" | "psychic";
  /** The open hold of the reading on screen. */
  hold: ChatHold | null;
  /** The held reading on screen (for "Continue reading"). */
  sessionId: string | undefined;
  wallet: Wallet | null;
  holdById: (holdId: string) => ChatHold | undefined;
  applyHold: (hold: ChatHold) => void;
  setWallet: (wallet: Wallet) => void;
  /** Open the top-up dialog in place, with the advisor and the open hold. */
  openTopUp: () => void;
  topUpOpen: boolean;
  /** A reveal succeeded: put the messages on the timeline (and in the inbox preview). */
  onRevealed: (result: HoldRevealResult) => void;
  /** The reading continued (`POST …/hold/continue`). */
  onContinued: (result: HoldContinueResult) => void;
  /** The state of the reading may have changed under us (409 `invalid_state`): read it again. */
  onStale: (sessionId: string | undefined) => void;
};

export type HoldActions = {
  openLocked: (holdId: string | undefined) => void;
  confirm: RevealConfirm | null;
  revealing: boolean;
  /** A hold is being looked up. */
  loading: boolean;
  confirmReveal: () => void;
  cancelReveal: () => void;
  continueOffered: boolean;
  continuing: boolean;
  continueReading: () => void;
};

/**
 * The client's actions on a hold (§26.1). A locked message while the hold is
 * open opens the top-up. After the hold expired it asks to confirm and reveals
 * for one minute's price: a wallet that covers the price confirms at once, a
 * short one opens the top-up and confirms by itself when the wallet covers the
 * price. "Continue reading" is the fallback when a purchase succeeded but the
 * hold did not convert on its own.
 */
export function useHoldActions(o: Options): HoldActions {
  const { text } = useI18n();
  const toast = useToast();
  // The confirm dialog is opened by a click (`confirming`, which also carries a failed attempt's reason) or by the
  // wallet covering the price after a top-up that was opened for it (`awaiting`: derived below, never copied into state).
  const [confirming, setConfirming] = React.useState<RevealConfirm | null>(null);
  const [awaiting, setAwaiting] = React.useState<Target | null>(null);
  const [revealing, setRevealing] = React.useState(false);
  const [loading, setLoading] = React.useState(false);
  const [continuing, setContinuing] = React.useState(false);
  const [offeredFor, setOfferedFor] = React.useState<string | null>(null);
  // One key per reveal and per continue, kept across retries after an unknown outcome (the server replays it).
  const keys = React.useRef(new Map<string, string>());
  const latest = React.useRef(o);
  React.useEffect(() => {
    latest.current = o;
  });
  const keyFor = (scope: string) => {
    let key = keys.current.get(scope);
    if (!key) {
      key = newId();
      keys.current.set(scope, key);
    }
    return key;
  };
  const explain = (error: ApiError) => text(errorMessage(error));

  const { wallet, topUpOpen } = o;
  const covered = (target: Target | null) => Boolean(target && wallet && wallet.balance >= target.price);
  const confirm: RevealConfirm | null = confirming ?? (awaiting && covered(awaiting) ? { ...awaiting, error: null } : null);

  const reveal = async (target: Target) => {
    setRevealing(true);
    try {
      const result = await holdsApi.reveal(target.holdId, keyFor(`reveal:${target.holdId}`));
      keys.current.delete(`reveal:${target.holdId}`);
      const { applyHold, setWallet, onRevealed } = latest.current;
      applyHold(result.hold);
      if (result.wallet) setWallet(result.wallet);
      onRevealed(result);
      setConfirming(null);
      setAwaiting(null);
    } catch (err) {
      const e = toApiError(err);
      const { openTopUp, onStale } = latest.current;
      if (e.code === "insufficient_credits") {
        // The wallet is short after all (another tab spent it): take the server's balance, top up, then ask again.
        const { balance } = e.details ?? {};
        const known = latest.current.wallet;
        if (typeof balance === "number" && known) latest.current.setWallet({ ...known, balance });
        setConfirming(null);
        setAwaiting(target);
        openTopUp();
      } else if (e.code === "invalid_state") {
        // The hold is not what we thought (still open, or already settled): read it again.
        setConfirming(null);
        setAwaiting(null);
        void lookup(target.holdId).then((fresh) => {
          if (fresh?.status === "open") openTopUp();
          else {
            toast.show({ tone: "info", message: text("This reading just changed. Please try again.") });
            onStale(fresh?.session_id);
          }
        });
      } else {
        // Network and server errors keep the dialog: "Try again" repeats the same request (same key).
        setConfirming({ ...target, error: explain(e) });
        if (!e.retryable) logger.warn("hold reveal failed", e, { code: String(e.code) });
      }
    } finally {
      setRevealing(false);
    }
  };

  const lookup = async (holdId: string): Promise<ChatHold | undefined> => {
    setLoading(true);
    try {
      const fresh = await holdsApi.get(holdId);
      latest.current.applyHold(fresh);
      return fresh;
    } catch (err) {
      const e = toApiError(err);
      toast.show({ tone: "error", message: explain(e), actionLabel: e.retryable ? text("Try again") : undefined, onAction: e.retryable ? () => openLocked(holdId) : undefined });
      return undefined;
    } finally {
      setLoading(false);
    }
  };

  /** The confirm dialog for an expired hold, or the top-up when the wallet is short. */
  const proceed = (hold: ChatHold) => {
    const target: Target = { holdId: hold.id, price: hold.price_credits, count: hold.held_messages };
    const { wallet: current, openTopUp } = latest.current;
    if (current && current.balance >= target.price) {
      setConfirming({ ...target, error: null });
      return;
    }
    setAwaiting(target);
    openTopUp();
  };

  const openLocked = (holdId: string | undefined) => {
    if (o.role !== "client" || !holdId || revealing || loading) return;
    void (async () => {
      let hold = latest.current.holdById(holdId);
      // The reading ended (or the page was reloaded): the hold is only known to the server.
      if (!hold) {
        const fresh = await lookup(holdId);
        if (!fresh) return;
        hold = fresh;
      }
      const { openTopUp, onStale } = latest.current;
      if (hold.status === "open") openTopUp();
      else if (hold.status === "expired") proceed(hold);
      // Nothing to pay: the same call answers with the messages of a hold that was revealed already.
      else if (hold.status === "revealed") await reveal({ holdId: hold.id, price: hold.price_credits, count: hold.held_messages });
      else onStale(hold.session_id); // converted: the messages are readable, this copy is stale
    })();
  };

  // A top-up opened for a reveal that closes without the wallet covering the price ends the wait (a moment later: the
  // dialog closes a little before the wallet shows the purchase).
  const sawDialog = React.useRef(false);
  const waiting = awaiting !== null && !covered(awaiting);
  React.useEffect(() => {
    if (!waiting) {
      sawDialog.current = false;
      return;
    }
    if (topUpOpen) {
      sawDialog.current = true;
      return;
    }
    if (!sawDialog.current) return;
    const timer = window.setTimeout(() => setAwaiting(null), TOP_UP_SETTLE_MS);
    return () => window.clearTimeout(timer);
  }, [waiting, topUpOpen]);

  // "Continue reading": the wallet covers the minute but the hold did not convert on its own.
  const open = o.hold?.status === "open" ? o.hold : null;
  const covers = Boolean(open && wallet && wallet.balance >= open.price_credits && o.role === "client");
  const openId = open?.id ?? null;
  React.useEffect(() => {
    if (!covers || !openId) return;
    const timer = window.setTimeout(() => setOfferedFor(openId), CONTINUE_OFFER_DELAY_MS);
    return () => {
      window.clearTimeout(timer);
      setOfferedFor(null);
    };
  }, [covers, openId]);
  const continueOffered = covers && offeredFor === openId;

  const continueReading = () => {
    const { sessionId, openTopUp, onStale } = latest.current;
    if (!sessionId || continuing) return;
    setContinuing(true);
    holdsApi
      .continueReading(sessionId, keyFor(`continue:${sessionId}`))
      .then(
        (result) => {
          keys.current.delete(`continue:${sessionId}`);
          const { applyHold, setWallet, onContinued } = latest.current;
          applyHold(result.hold);
          if (result.wallet) setWallet(result.wallet);
          onContinued(result);
        },
        (err) => {
          const e = toApiError(err);
          if (e.code === "insufficient_credits") openTopUp();
          else if (e.code === "invalid_state") onStale(sessionId);
          else toast.show({ tone: "error", message: explain(e), actionLabel: e.retryable ? text("Try again") : undefined, onAction: e.retryable ? continueReading : undefined });
        },
      )
      .finally(() => setContinuing(false));
  };

  return {
    openLocked,
    confirm,
    revealing,
    loading,
    confirmReveal: () => {
      if (confirm && !revealing) void reveal({ holdId: confirm.holdId, price: confirm.price, count: confirm.count });
    },
    cancelReveal: () => {
      setConfirming(null);
      setAwaiting(null);
    },
    continueOffered,
    continuing,
    continueReading,
  };
}
