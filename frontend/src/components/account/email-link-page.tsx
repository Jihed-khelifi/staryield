"use client";

import * as React from "react";
import Link from "next/link";
import { CheckCircle2, LinkIcon, Loader2, MailWarning } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { accountApi, emailChangeApi } from "@/lib/api/account";
import { setAuthHint } from "@/lib/api/client";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import { useSession } from "@/lib/auth/session-provider";
import { AuthLegal, AuthShell } from "@/components/forms/auth/auth-card";
import { Button } from "@/components/ui/button";

export type EmailLinkAction = "confirm-email" | "cancel-email-change" | "keep-account";

type Phase =
  | { kind: "init" }
  | { kind: "missing" }
  | { kind: "ready" }
  | { kind: "working" }
  | { kind: "done"; status: string }
  | { kind: "error"; error: ApiError; retry: boolean };

type Copy = { title: string; body: string };

const READY: Record<EmailLinkAction, Copy & { button: string; busy: string }> = {
  "confirm-email": {
    title: "Confirm your new email address",
    body: "Confirm the change to finish. You'll be signed out on every device, then you can sign in with your new address.",
    button: "Confirm my new email",
    busy: "Confirming…",
  },
  "cancel-email-change": {
    title: "Cancel the email change?",
    body: "Someone asked to change the email address of your Staryield account. If it wasn't you, cancel it now.",
    button: "Cancel the change",
    busy: "Cancelling…",
  },
  "keep-account": {
    title: "Keep your account?",
    body: "Your account is scheduled for deletion. Keep it to cancel the deletion and use Staryield again.",
    button: "Keep my account",
    busy: "Keeping your account…",
  },
};

const DONE: Record<string, Copy> = {
  confirmed: {
    title: "Your email address is updated",
    body: "Sign in with your new address. We told your previous address, with a link to undo the change if it wasn't you.",
  },
  cancelled: {
    title: "The change is cancelled",
    body: "Your email address stays the same. If you didn't ask for this change, change your password to secure your account.",
  },
  restored: {
    title: "Your previous email address is back",
    body: "We signed you out everywhere and emailed you a link to choose a new password. Use it now to secure your account.",
  },
  active: {
    title: "Your account is active again",
    body: "The deletion is cancelled. Sign in to continue.",
  },
};

const EXPIRED: Record<EmailLinkAction, string> = {
  "confirm-email": "Confirmation links work once and expire after 30 minutes. Request a new one from your settings.",
  "cancel-email-change": "This link works once and expires after 7 days. If someone else may have access to your account, contact our support team.",
  "keep-account": "This link works once. Sign in before the deletion date to keep your account.",
};

/** Read the one-time token and strip it from the address bar (history, referrers, analytics). */
function takeToken(): string | null {
  const raw = new URLSearchParams(window.location.search).get("token");
  if (raw !== null) window.history.replaceState(window.history.state, "", window.location.pathname + window.location.hash);
  return raw && raw.length <= 128 ? raw : null;
}

/**
 * Landing pages for the one-time links we e-mail: confirm a new address,
 * cancel (or undo) an e-mail change, keep an account scheduled for closure.
 * The token is read once, removed from the URL at once, kept only in
 * memory and sent in a POST body after an explicit click (link scanners
 * that open URLs cannot use it).
 */
export function EmailLinkPage({ action }: { action: EmailLinkAction }) {
  const { text, locale } = useI18n();
  const { status, reload } = useSession();
  const token = React.useRef<string | null>(null);
  const captured = React.useRef(false);
  const [phase, setPhase] = React.useState<Phase>({ kind: "init" });

  React.useEffect(() => {
    // Once per mount (Strict Mode runs effects twice; the URL is stripped by then).
    if (captured.current) return;
    captured.current = true;
    token.current = takeToken();
    setPhase(token.current ? { kind: "ready" } : { kind: "missing" });
  }, []);

  async function run() {
    const value = token.current;
    if (!value) return;
    setPhase({ kind: "working" });
    try {
      const result =
        action === "confirm-email"
          ? await emailChangeApi.confirm(value)
          : action === "cancel-email-change"
            ? await emailChangeApi.cancel(value)
            : await accountApi.cancelClosure(value);
      token.current = null;
      // Confirming or undoing a change signs every device out, this one too.
      if (result.status === "confirmed" || result.status === "restored") {
        setAuthHint(false);
        void reload();
      }
      setPhase({ kind: "done", status: result.status });
    } catch (err) {
      const error = toApiError(err);
      // A used, expired or conflicting link cannot be retried.
      if (error.code === "invalid_token" || error.code === "email_taken") token.current = null;
      setPhase({ kind: "error", error, retry: token.current !== null });
    }
  }

  const ready = READY[action];
  const settingsHref = `/${locale}/profile/settings#account`;
  const loginHref = `/${locale}/login`;
  let icon: React.ReactNode = <LinkIcon aria-hidden className="email-link-icon text-amber" />;
  let title = ready.title;
  let body: React.ReactNode = text(ready.body);
  let cta: React.ReactNode = null;

  if (phase.kind === "init") {
    icon = <Loader2 aria-hidden className="email-link-icon animate-spin motion-reduce:animate-none" />;
    title = "Checking your link…";
    body = null;
  } else if (phase.kind === "missing") {
    icon = <MailWarning aria-hidden className="email-link-icon text-danger" />;
    title = "This link is incomplete";
    body = text("Open the link from your email again, or start over from your settings.");
    cta = (
      <Button asChild className="auth-continue gold-button w-full">
        <Link href={action === "keep-account" ? loginHref : settingsHref}>{text(action === "keep-account" ? "Log in" : "Go to settings")}</Link>
      </Button>
    );
  } else if (phase.kind === "ready" || phase.kind === "working") {
    const busy = phase.kind === "working";
    cta = (
      <Button
        type="button"
        className="auth-continue gold-button w-full"
        onClick={() => void run()}
        disabled={busy}
        aria-busy={busy}
        data-testid="email-link-action"
      >
        {text(busy ? ready.busy : ready.button)}
      </Button>
    );
  } else if (phase.kind === "done") {
    const done = DONE[phase.status] ?? DONE.active;
    icon = <CheckCircle2 aria-hidden className="email-link-icon text-success" />;
    title = done.title;
    body = text(done.body);
    const signedIn = status === "authenticated" && phase.status === "cancelled";
    cta = (
      <Button asChild className="auth-continue gold-button w-full">
        <Link href={signedIn ? settingsHref : loginHref} data-testid="email-link-next">
          {text(signedIn ? "Go to settings" : "Log in")}
        </Link>
      </Button>
    );
  } else {
    const { error } = phase;
    icon = <MailWarning aria-hidden className="email-link-icon text-danger" />;
    if (error.code === "invalid_token") {
      title = "This link has expired";
      body = text(EXPIRED[action]);
    } else if (error.code === "email_taken") {
      title = "We couldn't change your email address";
      body = text("This email address now belongs to another account. Contact our support team.");
    } else {
      title = "Something went wrong";
      body = text(errorMessage(error));
    }
    const retry = phase.retry;
    const support = action === "cancel-email-change" || error.code === "email_taken";
    cta = retry ? (
      <Button type="button" className="auth-continue gold-button w-full" onClick={() => void run()} data-testid="email-link-action">
        {text("Try again")}
      </Button>
    ) : (
      <Button asChild className="auth-continue gold-button w-full">
        <Link href={support ? `/${locale}/contact` : action === "keep-account" ? loginHref : settingsHref} data-testid="email-link-next">
          {text(support ? "Contact support" : action === "keep-account" ? "Log in" : "Go to settings")}
        </Link>
      </Button>
    );
  }

  return (
    <AuthShell promo={false}>
      <section className="auth-card paper-page email-link-card" aria-labelledby="email-link-title" data-testid="email-link" data-phase={phase.kind}>
        <div className="auth-card-body">
          {icon}
          <h1 id="email-link-title" className="text-center text-[28px] font-medium leading-tight" tabIndex={-1}>
            {text(title)}
          </h1>
          <div role="status" aria-live="polite" className="text-center text-sm font-light">
            {body}
          </div>
        </div>
        {cta}
        <AuthLegal />
      </section>
    </AuthShell>
  );
}
