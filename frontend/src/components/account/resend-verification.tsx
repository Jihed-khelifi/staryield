"use client";

import * as React from "react";
import { CheckCircle2, MailWarning } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { errorMessage } from "@/lib/api/errors";
import { cn } from "@/lib/utils";
import { useResendVerification } from "@/lib/verification/use-resend-verification";
import { Button } from "@/components/ui/button";

/**
 * "Resend link" with its states: sending, sent to the masked address with a
 * live countdown until another link can be requested, cooldown, hourly limit,
 * already verified and errors with a retry. The button carries the countdown
 * (it is not announced every second); the status line below it is the live
 * region.
 */
export function ResendVerification({
  label = "Resend link",
  variant = "outline",
  className,
  align = "start",
}: {
  label?: string;
  variant?: React.ComponentProps<typeof Button>["variant"];
  className?: string;
  align?: "start" | "center";
}) {
  const { text } = useI18n();
  const { state, secondsLeft, busy, canResend, resend } = useResendVerification();

  let buttonText = text(label);
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
    message = text("You asked for several links. Please try again in {value0} minutes.", { value0: Math.max(1, Math.ceil(secondsLeft / 60)) });
    tone = "warn";
  } else if (state.phase === "verified") {
    message = text("Your email address is already verified.");
  } else if (state.phase === "error") {
    message = text(errorMessage(state.error));
    tone = "error";
  }
  const Icon = tone === "ok" ? CheckCircle2 : MailWarning;

  return (
    <div className={cn("flex flex-col gap-2", align === "center" && "items-center text-center", className)} data-testid="resend-verification" data-phase={state.phase}>
      {state.phase === "verified" ? null : (
        <Button type="button" variant={variant} size="lg" onClick={() => void resend()} disabled={!canResend} aria-busy={busy} data-testid="resend-verification-button">
          {buttonText}
        </Button>
      )}
      <p role="status" aria-live="polite" className={cn("flex items-start gap-1.5 text-[13px] leading-snug", tone === "error" ? "text-danger" : "text-[var(--muted-ink)]", !message && "sr-only")} data-testid="resend-verification-status">
        {message ? <Icon aria-hidden className="mt-0.5 size-4 shrink-0" /> : null}
        <span>{message}</span>
      </p>
    </div>
  );
}
