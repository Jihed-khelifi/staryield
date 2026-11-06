"use client";

import * as React from "react";
import { Info, RotateCcw } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { accountApi } from "@/lib/api/account";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import type { User } from "@/lib/api/types";
import { Button } from "@/components/ui/button";

/**
 * Signing in to a paused account: the API answers 403 with a short-lived
 * reactivation token (5 minutes) after checking the password, so the user
 * can reactivate (or keep an account scheduled for closure) in one click.
 */
export type Reactivation = {
  kind: "account_deactivated" | "account_closure_pending";
  token: string;
  deletionAt?: string;
};

/** The reactivation offer carried by a sign-in error, if any. */
export function reactivationFrom(error: ApiError): Reactivation | null {
  if (error.code !== "account_deactivated" && error.code !== "account_closure_pending") return null;
  const token = error.details?.reactivation_token;
  if (typeof token !== "string" || !token) return null;
  const deletion = error.details?.deletion_scheduled_at;
  return { kind: error.code, token, deletionAt: typeof deletion === "string" ? deletion : undefined };
}

export function ReactivationOffer({ offer, onSignedIn }: { offer: Reactivation; onSignedIn: (user: User) => void }) {
  const { text, locale } = useI18n();
  const [busy, setBusy] = React.useState(false);
  const [problem, setProblem] = React.useState<string | null>(null);
  const [expired, setExpired] = React.useState(false);
  const closing = offer.kind === "account_closure_pending";
  const date = offer.deletionAt ? new Intl.DateTimeFormat(locale, { dateStyle: "long" }).format(new Date(offer.deletionAt)) : null;

  async function reactivate() {
    setBusy(true);
    setProblem(null);
    try {
      const res = await accountApi.reactivate(offer.token);
      onSignedIn(res.user);
    } catch (err) {
      const e = toApiError(err);
      if (e.code === "invalid_token") {
        setProblem(text("This offer has expired. Sign in again to continue."));
        setExpired(true);
      } else setProblem(text(errorMessage(e)));
      setBusy(false);
    }
  }

  return (
    <div className="mt-4 grid gap-3 rounded-lg border border-gold/60 bg-cream-light/70 p-3 text-sm" role="alert" data-testid="reactivation-offer" data-kind={offer.kind}>
      <p className="flex items-start gap-2">
        <Info aria-hidden className="mt-0.5 size-4 shrink-0" />
        <span>
          {closing
            ? date
              ? text("This account is scheduled for deletion on {value0}. Keep it to cancel the deletion and sign in.", { value0: date })
              : text("This account is scheduled for deletion. Keep it to cancel the deletion and sign in.")
            : text("You disabled this account. Reactivate it to sign in again; your data is waiting for you.")}
        </span>
      </p>
      {problem ? <p className="text-danger">{problem}</p> : null}
      {expired ? null : (
      <Button type="button" variant="cta" size="lg" className="justify-self-start px-4" onClick={() => void reactivate()} disabled={busy} aria-busy={busy} data-testid="reactivate">
        <RotateCcw aria-hidden />
        {text(busy ? "One moment…" : closing ? "Keep my account" : "Reactivate my account")}
      </Button>
      )}
    </div>
  );
}

const NOTICES: Record<string, string> = {
  deactivated: "Your account is disabled and you are signed out. Sign in again whenever you want to reactivate it.",
  closure_scheduled: "Your account will be deleted in 30 days. Sign in before then if you change your mind.",
};

const noSubscription = () => () => {};

/** One-off confirmation after disabling or closing the account (?notice=). */
export function LoginNotice() {
  const { text } = useI18n();
  const notice = React.useSyncExternalStore(
    noSubscription,
    () => new URLSearchParams(window.location.search).get("notice") ?? "",
    () => ""
  );
  const message = NOTICES[notice];
  if (!message) return null;
  return (
    <p className="mt-4 flex items-start gap-2 rounded-lg bg-sage/20 p-3 text-sm" role="status" data-testid="login-notice">
      <Info aria-hidden className="mt-0.5 size-4 shrink-0" />
      <span>{text(message)}</span>
    </p>
  );
}
