"use client";

import * as React from "react";
import Link from "next/link";
import { CheckCircle2, Loader2, MailWarning, MailX } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { reportEmailApi } from "@/lib/api/calculator-report";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import { takeToken } from "@/components/account/email-link-page";
import { AuthLegal, AuthShell } from "@/components/forms/auth/auth-card";
import { Button } from "@/components/ui/button";

type Phase =
  | { kind: "init" }
  | { kind: "missing" }
  | { kind: "ready" }
  | { kind: "working" }
  | { kind: "done" }
  | { kind: "error"; error: ApiError; retry: boolean };

/**
 * `/{locale}/unsubscribe?token=…` (revisions 6.0, §26.4): the page the
 * unsubscribe link of the report e-mail opens. Like the other one-time e-mail
 * links, the token is read once, removed from the address bar at once, kept
 * in memory and sent in a POST body after an explicit click, so a link scanner
 * that merely opens the URL cannot unsubscribe anyone. The API answers
 * idempotently: using the link twice is a success; an unknown token is a 404.
 */
export function UnsubscribePage() {
  const { text, locale } = useI18n();
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
      await reportEmailApi.unsubscribe(value);
      token.current = null;
      setPhase({ kind: "done" });
    } catch (err) {
      const error = toApiError(err);
      // An unknown link cannot be retried; a network or server failure can.
      if (error.code === "not_found" || error.code === "validation_failed") token.current = null;
      setPhase({ kind: "error", error, retry: token.current !== null });
    }
  }

  const homeHref = `/${locale}`;
  let icon: React.ReactNode = <MailX aria-hidden className="email-link-icon text-amber" />;
  let title = "Unsubscribe from Staryield e-mails?";
  let body: React.ReactNode = text("You will stop receiving horoscopes, offers and news from Staryield. E-mails you ask for, like a report, still reach you.");
  let cta: React.ReactNode;

  if (phase.kind === "init") {
    icon = <Loader2 aria-hidden className="email-link-icon animate-spin motion-reduce:animate-none" />;
    title = "Checking your link…";
    body = null;
    cta = null;
  } else if (phase.kind === "missing") {
    icon = <MailWarning aria-hidden className="email-link-icon text-danger" />;
    title = "This link is incomplete";
    body = text("Open the unsubscribe link from your e-mail again.");
    cta = (
      <Button asChild className="auth-continue gold-button w-full">
        <Link href={homeHref} data-testid="unsubscribe-next">
          {text("Back to home")}
        </Link>
      </Button>
    );
  } else if (phase.kind === "done") {
    icon = <CheckCircle2 aria-hidden className="email-link-icon text-success" />;
    title = "You are unsubscribed";
    body = text("We will not send you horoscopes, offers or news anymore. You can still ask for a report by e-mail whenever you like.");
    cta = (
      <Button asChild className="auth-continue gold-button w-full">
        <Link href={homeHref} data-testid="unsubscribe-next">
          {text("Back to home")}
        </Link>
      </Button>
    );
  } else if (phase.kind === "error") {
    icon = <MailWarning aria-hidden className="email-link-icon text-danger" />;
    if (!phase.retry && (phase.error.code === "not_found" || phase.error.code === "validation_failed")) {
      title = "This link is not valid";
      body = text("The unsubscribe link may be incomplete, or this address is no longer on our list. If you still receive e-mails from us, write to our support team.");
    } else {
      title = "Something went wrong";
      body = text(errorMessage(phase.error));
    }
    cta = phase.retry ? (
      <Button type="button" className="auth-continue gold-button w-full" onClick={() => void run()} data-testid="unsubscribe-action">
        {text("Try again")}
      </Button>
    ) : (
      <Button asChild className="auth-continue gold-button w-full">
        <Link href={`/${locale}/contact`} data-testid="unsubscribe-next">
          {text("Contact support")}
        </Link>
      </Button>
    );
  } else {
    const busy = phase.kind === "working";
    cta = (
      <Button
        type="button"
        className="auth-continue gold-button w-full"
        onClick={() => void run()}
        disabled={busy}
        aria-busy={busy}
        data-testid="unsubscribe-action"
      >
        {text(busy ? "Unsubscribing…" : "Unsubscribe")}
      </Button>
    );
  }

  return (
    <AuthShell promo={false}>
      <section className="auth-card paper-page email-link-card" aria-labelledby="unsubscribe-title" data-testid="unsubscribe" data-phase={phase.kind}>
        <div className="auth-card-body">
          {icon}
          <h1 id="unsubscribe-title" className="text-center text-[28px] font-medium leading-tight" tabIndex={-1}>
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
