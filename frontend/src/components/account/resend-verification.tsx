"use client";

import * as React from "react";
import Link from "next/link";
import { CheckCircle2, MailWarning } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { errorMessage } from "@/lib/api/errors";
import { cn } from "@/lib/utils";
import { longWait } from "@/lib/verification/resend";
import { useResendVerification } from "@/lib/verification/use-resend-verification";
import { Button } from "@/components/ui/button";

/**
 * "Send me a link" / "Resend link" with its states: sending, sent to the
 * masked address with a live countdown until another link can be requested
 * (from the first render when a link is already waiting), cooldown, hourly
 * and daily limits, already verified, an address that refuses e-mail (with
 * the way to change it) and errors with a retry. The button carries the
 * countdown (it is not announced every second); the status line below it is
 * the live region.
 */
export function ResendVerification({
  label,
  variant = "outline",
  className,
  align = "start",
}: {
  /** Overrides "Resend link" (a link is waiting) / "Send me a link" (none is). */
  label?: string;
  variant?: React.ComponentProps<typeof Button>["variant"];
  className?: string;
  align?: "start" | "center";
}) {
  const { text, locale } = useI18n();
  const { state, secondsLeft, busy, canResend, pending, resend } = useResendVerification();

  let buttonText = text(label ?? (pending ? "Resend link" : "Send me a link"));
  if (busy) buttonText = text("Sending…");
  else if (secondsLeft > 0) buttonText = text("Resend in {value0}s", { value0: secondsLeft });
  else if (state.phase === "error") buttonText = text("Try again");
  else if (state.phase === "sent") buttonText = text("Resend link");

  let message: string | null = null;
  let tone: "ok" | "warn" | "error" = "ok";
  if (state.phase === "sent") {
    message = text("We sent a new link to {value0}. Check your inbox and your spam folder.", { value0: state.email });
  } else if (state.phase === "cooldown") {
    message = text("A link was just sent. Check your inbox, or ask for another in a moment.");
    tone = "warn";
  } else if (state.phase === "limited") {
    const wait = longWait(secondsLeft);
    message =
      wait.unit === "hours"
        ? text("You asked for several links today. Please try again in {value0} hours.", { value0: wait.value })
        : text("You asked for several links. Please try again in {value0} minutes.", { value0: wait.value });
    tone = "warn";
  } else if (state.phase === "verified") {
    message = text("Your email address is already verified.");
  } else if (state.phase === "undeliverable") {
    message = text("We can't deliver email to this address: earlier messages bounced or were refused. Check it, or change it in your account settings.");
    tone = "error";
  } else if (state.phase === "error") {
    message = text(errorMessage(state.error));
    tone = "error";
  }
  const Icon = tone === "ok" ? CheckCircle2 : MailWarning;
  const done = state.phase === "verified" || state.phase === "undeliverable";

  return (
    <div className={cn("flex flex-col gap-2", align === "center" && "items-center text-center", className)} data-testid="resend-verification" data-phase={state.phase}>
      {done ? null : (
        <Button type="button" variant={variant} size="lg" onClick={() => void resend()} disabled={!canResend} aria-busy={busy} data-testid="resend-verification-button">
          {buttonText}
        </Button>
      )}
      <p role="status" aria-live="polite" className={cn("flex items-start gap-1.5 text-[13px] leading-snug", tone === "error" ? "text-danger" : "text-[var(--muted-ink)]", !message && "sr-only")} data-testid="resend-verification-status">
        {message ? <Icon aria-hidden className="mt-0.5 size-4 shrink-0" /> : null}
        <span>{message}</span>
      </p>
      {state.phase === "undeliverable" ? (
        <Button asChild variant={variant} size="lg">
          <Link href={`/${locale}/profile/settings#account`} data-testid="resend-verification-change-email">
            {text("Change your email address")}
          </Link>
        </Button>
      ) : null}
    </div>
  );
}
