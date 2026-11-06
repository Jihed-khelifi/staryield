"use client";

import * as React from "react";
import Link from "next/link";
import { CheckCircle2, Circle } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { PublicFrame } from "@/components/layout/public-frame";
import { RadiantPanel } from "@/components/forms/radiant-panel";
import { Button } from "@/components/ui/button";
import { passwordCheckLabels, passwordChecks, type PasswordCheckId } from "@/lib/schemas/auth";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { cn } from "@/lib/utils";

/** The email typed on the first sign-up step, carried to the password step. */
const SIGNUP_EMAIL = "staryield.signup.email";

export function readSignupEmail(): string {
  try {
    return sessionStorage.getItem(SIGNUP_EMAIL) ?? "";
  } catch {
    return "";
  }
}

export function writeSignupEmail(email: string) {
  try {
    sessionStorage.setItem(SIGNUP_EMAIL, email);
  } catch {
    // Storage blocked: the password step sends the visitor back to re-enter it.
  }
}

export function clearSignupEmail() {
  try {
    sessionStorage.removeItem(SIGNUP_EMAIL);
  } catch {
    // Nothing to clean up.
  }
}

/** The logo page around every auth step (radial rays + sun + STARYIELD). */
export function AuthShell({ promo = true, children }: { promo?: boolean; children: React.ReactNode }) {
  return (
    <PublicFrame promo={promo}>
      <RadiantPanel className="auth-panel">{children}</RadiantPanel>
    </PublicFrame>
  );
}

/** Terms · Contact · Privacy under each card. */
export function AuthLegal() {
  const { text, locale } = useI18n();
  return (
    <div className="auth-legal">
      <Link href={`/${locale}/terms`}>{text("Terms of Use")}</Link> ·{" "}
      <Link href={`/${locale}/contact`}>{text("Contact Us")}</Link> ·{" "}
      <Link href={`/${locale}/privacy`}>{text("Privacy Policy")}</Link>
    </div>
  );
}

/**
 * The cream card of an auth step: title, optional subtitle, fields and the
 * full-width CONTINUE bar (CTA glow). It is the <form>; the step passes
 * react-hook-form's handleSubmit.
 */
export function AuthCard({
  title,
  subtitle,
  children,
  onSubmit,
  pending = false,
  submitLabel = "Continue",
  pendingLabel = "Continuing…",
  className,
  legal = true,
  testId,
}: {
  title: string;
  subtitle?: React.ReactNode;
  children?: React.ReactNode;
  onSubmit: React.FormEventHandler<HTMLFormElement>;
  pending?: boolean;
  submitLabel?: string;
  pendingLabel?: string;
  className?: string;
  legal?: boolean;
  testId?: string;
}) {
  const { text } = useI18n();
  const titleId = React.useId();
  return (
    <form
      className={cn("auth-card paper-page", className)}
      onSubmit={onSubmit}
      noValidate
      aria-labelledby={titleId}
      data-testid={testId}
    >
      <div className="auth-card-body">
        <h1 id={titleId} className="text-center text-[32px] font-medium leading-tight">
          {text(title)}
        </h1>
        {subtitle ? <div className="mt-4 text-center text-sm font-light">{subtitle}</div> : null}
        <div className="auth-fields">{children}</div>
      </div>
      <Button type="submit" disabled={pending} aria-busy={pending} onMouseDown={keepFocusOnPress} className="auth-continue gold-button w-full">
        {text(pending ? pendingLabel : submitLabel)}
      </Button>
      {legal ? <AuthLegal /> : null}
    </form>
  );
}

/** Live password rules: each line turns green (with a check) once met. */
export function PasswordChecklist({ password, id }: { password: string; id?: string }) {
  const { text } = useI18n();
  const checks = passwordChecks(password);
  return (
    <ul id={id} className="auth-password-checks" aria-label={text("Password requirements")}>
      {(Object.keys(passwordCheckLabels) as PasswordCheckId[]).map((key) => {
        const ok = checks[key];
        const Icon = ok ? CheckCircle2 : Circle;
        return (
          <li key={key} data-met={ok || undefined}>
            <Icon aria-hidden className="size-3.5 shrink-0" />
            <span>{text(passwordCheckLabels[key])}</span>
            <span className="sr-only">{text(ok ? "Requirement met" : "Requirement not met yet")}</span>
          </li>
        );
      })}
    </ul>
  );
}

/** Seconds left of a lock-out; ticks once per second while above zero. */
export function useCountdown(seconds: number | undefined): number {
  const [left, setLeft] = React.useState(seconds ?? 0);
  const [from, setFrom] = React.useState(seconds);
  if (from !== seconds) {
    setFrom(seconds);
    setLeft(seconds ?? 0);
  }
  React.useEffect(() => {
    if (!left) return;
    const timer = window.setTimeout(() => setLeft((s) => Math.max(0, s - 1)), 1000);
    return () => window.clearTimeout(timer);
  }, [left]);
  return left;
}
