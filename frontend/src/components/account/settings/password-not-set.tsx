"use client";

import * as React from "react";
import { KeyRound, MailCheck } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { authApi } from "@/lib/api/endpoints";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import { useSession } from "@/lib/auth/session-provider";
import { announce } from "@/components/feedback/live-region";
import { Button } from "@/components/ui/button";

/** Seconds before the create-password link can be requested again. */
export const CREATE_PASSWORD_RESEND_SECONDS = 60;

/**
 * "Send me a link to create my password": the normal reset link
 * (`POST /auth/password/forgot`), for accounts that signed up with Google and
 * have no password yet (§23.1). One implementation for the security card, the
 * sign-in methods block and every dialog that needs the password; requests
 * are throttled client-side (the API rate-limits as well).
 */
export function useCreatePasswordLink(email?: string) {
  const { text, locale } = useI18n();
  const { user } = useSession();
  const address = email ?? user?.email;
  const [phase, setPhase] = React.useState<"idle" | "sending" | "sent">("idle");
  const [error, setError] = React.useState<ApiError | null>(null);
  // The resend wait is a deadline plus a once-a-second clock, so it stays right when timers are throttled.
  const [until, setUntil] = React.useState(0);
  const [now, setNow] = React.useState(() => Date.now());
  const wait = Math.max(0, Math.ceil((until - now) / 1000));
  const waiting = wait > 0;
  const startWait = React.useCallback((seconds: number) => {
    const at = Date.now();
    setNow(at);
    setUntil(at + seconds * 1000);
  }, []);

  React.useEffect(() => {
    if (!waiting) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [waiting]);

  const send = React.useCallback(async () => {
    if (!address || phase === "sending" || wait > 0) return;
    setPhase("sending");
    setError(null);
    try {
      await authApi.forgotPassword(address, locale);
      setPhase("sent");
      startWait(CREATE_PASSWORD_RESEND_SECONDS);
      announce(text("We've e-mailed you a link to create your password."));
    } catch (err) {
      const e = toApiError(err);
      setPhase("idle");
      setError(e);
      if (e.code === "rate_limited") startWait(Math.max(1, Math.ceil(e.retryAfter ?? CREATE_PASSWORD_RESEND_SECONDS)));
    }
  }, [address, locale, phase, startWait, text, wait]);

  return { send, phase, error, wait, ready: Boolean(address) };
}

export type CreatePasswordLink = ReturnType<typeof useCreatePasswordLink>;

/** The "Create a password" button with its confirmation / error line. */
export function CreatePasswordAction({ link, className }: { link: CreatePasswordLink; className?: string }) {
  const { text } = useI18n();
  const { phase, error, wait } = link;
  const sending = phase === "sending";
  const label = sending
    ? text("Sending…")
    : wait > 0
      ? text("Send again in {value0}s", { value0: wait })
      : text(phase === "sent" ? "Send the link again" : "Create a password");
  return (
    <div className={className ?? "grid gap-2"} data-testid="create-password-action">
      {phase === "sent" ? (
        <p className="flex items-start gap-2 text-sm" role="status" data-testid="create-password-sent">
          <MailCheck aria-hidden className="mt-0.5 size-4 shrink-0 text-success" />
          <span>{text("We've e-mailed you a link to create your password.")}</span>
        </p>
      ) : null}
      {error ? (
        <p className="text-sm text-danger" role="alert">
          {text(errorMessage(error))}
        </p>
      ) : null}
      <Button
        type="button"
        variant="outline"
        size="lg"
        className="settings-button justify-self-start"
        onClick={() => void link.send()}
        disabled={sending || wait > 0 || !link.ready}
        aria-busy={sending}
        data-testid="create-password"
      >
        <KeyRound aria-hidden />
        {label}
      </Button>
    </div>
  );
}

/**
 * Shown instead of a dead end when an action answers `password_not_set`
 * (e-mail change, deactivate, close, export, change password): the
 * explanation plus the "Create a password" action.
 */
export function PasswordNotSetNotice({ email, className }: { email?: string; className?: string }) {
  const { text } = useI18n();
  const link = useCreatePasswordLink(email);
  return (
    <div className={className ?? "settings-note grid gap-3"} role="group" aria-label={text("Create a password")} data-testid="password-not-set">
      <p>{text("This needs your account password, and you haven't created one yet.")}</p>
      <p>{text("You signed up with Google, so there is no password to confirm yet. Create one with the link we e-mail you, then come back.")}</p>
      <CreatePasswordAction link={link} />
    </div>
  );
}

/** True for the API's "this account has no password yet" answer. */
export function isPasswordNotSet(error: unknown): boolean {
  return toApiError(error).code === "password_not_set";
}
