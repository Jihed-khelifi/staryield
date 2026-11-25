"use client";

import * as React from "react";
import Link from "next/link";
import { CheckCircle2, Loader2, MailWarning } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { emailVerificationApi, type VerificationResult } from "@/lib/api/email-verification";
import { readAuthHint } from "@/lib/api/client";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import { useSession } from "@/lib/auth/session-provider";
import { usePsychicsHref } from "@/lib/psychics-link";
import { takeVerificationToken } from "@/lib/verification/token";
import { ResendVerification } from "@/components/account/resend-verification";
import { AuthLegal, AuthShell } from "@/components/forms/auth/auth-card";
import { Button } from "@/components/ui/button";

type Phase =
  | { kind: "init" }
  | { kind: "missing" }
  | { kind: "verifying" }
  | { kind: "done"; status: VerificationResult["status"] }
  | { kind: "invalid" }
  | { kind: "error"; error: ApiError };

/**
 * Landing page of the verification link. The token is read once from the URL
 * fragment, removed from the address bar at once and posted immediately (no
 * extra click). The link may be opened on any device, signed in or not; a
 * browser that holds the session refreshes it, and every other tab hears the
 * realtime event.
 */
export function VerifyEmailPage() {
  const { text, locale } = useI18n();
  const { status, user, reload } = useSession();
  const psychicsHref = usePsychicsHref();
  const token = React.useRef<string | null>(null);
  const captured = React.useRef(false);
  const [phase, setPhase] = React.useState<Phase>({ kind: "init" });

  const run = React.useCallback(async () => {
    const value = token.current;
    if (!value) return;
    setPhase({ kind: "verifying" });
    try {
      const result = await emailVerificationApi.verify(value);
      token.current = null;
      // This browser may hold the account's session: show the verified state everywhere in it.
      if (readAuthHint()) void reload();
      setPhase({ kind: "done", status: result.status });
    } catch (raw) {
      const error = toApiError(raw);
      // A used, expired or unknown link cannot be retried; anything else (network, server, limits) can.
      if (error.code === "invalid_token") {
        token.current = null;
        setPhase({ kind: "invalid" });
      } else {
        setPhase({ kind: "error", error });
      }
    }
  }, [reload]);

  React.useEffect(() => {
    // Once per mount (Strict Mode runs effects twice; the fragment is stripped by then).
    if (captured.current) return;
    captured.current = true;
    token.current = takeVerificationToken();
    if (token.current) void run();
    else setPhase({ kind: "missing" });
  }, [run]);

  React.useEffect(() => {
    // Another link opened in this tab only changes the fragment (no new page load): take its token too.
    const onHashChange = () => {
      const next = takeVerificationToken();
      if (!next) return;
      token.current = next;
      void run();
    };
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, [run]);

  const signedIn = status === "authenticated";
  const alreadyVerified = signedIn && user?.email_verified === true;
  const loginHref = `/${locale}/login?next=${encodeURIComponent(`/${locale}/profile`)}`;
  const profileHref = `/${locale}/profile`;

  let icon: React.ReactNode = <Loader2 aria-hidden className="email-link-icon animate-spin motion-reduce:animate-none" />;
  let title = "Confirming your email…";
  let body: React.ReactNode = text("One moment while we check your link.");
  let cta: React.ReactNode = null;

  const primary = (href: string, label: string, testId: string) => (
    <Button asChild className="auth-continue gold-button w-full">
      <Link href={href} data-testid={testId}>
        {text(label)}
      </Link>
    </Button>
  );

  if (phase.kind === "done") {
    icon = <CheckCircle2 aria-hidden className="email-link-icon text-success" />;
    title = phase.status === "verified" ? "Your email is verified" : "Your email is already verified";
    body = text(
      phase.status === "verified"
        ? "Thank you. Your account is now more secure, and you will receive your reading receipts and account notices."
        : "Nothing more to do: this address was already confirmed.",
    );
    cta = (
      <>
        {primary(psychicsHref, "Find an advisor", "verify-email-advisors")}
        <Button asChild variant="outline" className="w-full">
          <Link href={profileHref} data-testid="verify-email-profile">
            {text("Go to my profile")}
          </Link>
        </Button>
      </>
    );
  } else if (phase.kind === "missing") {
    icon = <MailWarning aria-hidden className="email-link-icon text-danger" />;
    title = "This link is incomplete";
    body = text("Open the link from your email again. If it still doesn't work, we can send you a new one.");
    cta =
      status === "loading" ? null : signedIn && !alreadyVerified ? (
        <ResendVerification label="Send a new link" variant="cta" align="center" />
      ) : (
        primary(signedIn ? profileHref : loginHref, signedIn ? "Go to my profile" : "Log in to get a new link", "verify-email-next")
      );
  } else if (phase.kind === "invalid") {
    icon = <MailWarning aria-hidden className="email-link-icon text-danger" />;
    if (alreadyVerified) {
      icon = <CheckCircle2 aria-hidden className="email-link-icon text-success" />;
      title = "Your email is already verified";
      body = text("Nothing more to do: this address was already confirmed.");
      cta = primary(psychicsHref, "Find an advisor", "verify-email-advisors");
    } else {
      title = "This link has expired";
      body = text("Confirmation links work once and expire after 48 hours. Ask for a new one to confirm your email.");
      cta =
        status === "loading" ? null : signedIn ? (
          <ResendVerification label="Send a new link" variant="cta" align="center" />
        ) : (
          primary(loginHref, "Log in to get a new link", "verify-email-login")
        );
    }
  } else if (phase.kind === "error") {
    icon = <MailWarning aria-hidden className="email-link-icon text-danger" />;
    title = "We couldn't confirm your email";
    body = text(errorMessage(phase.error));
    cta = (
      <Button type="button" className="auth-continue gold-button w-full" onClick={() => void run()} data-testid="verify-email-retry">
        {text("Try again")}
      </Button>
    );
  }

  const busy = phase.kind === "init" || phase.kind === "verifying";
  return (
    <AuthShell promo={false}>
      <section
        className="auth-card paper-page email-link-card"
        aria-labelledby="verify-email-heading"
        aria-busy={busy}
        data-testid="verify-email"
        data-phase={phase.kind === "done" ? `done-${phase.status}` : phase.kind}
      >
        <div className="auth-card-body">
          {icon}
          <h1 id="verify-email-heading" className="text-center text-[28px] font-medium leading-tight" tabIndex={-1}>
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
