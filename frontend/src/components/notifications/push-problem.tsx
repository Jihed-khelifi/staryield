"use client";

import * as React from "react";
import { Check, Copy } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { BRAVE_PRIVACY_URL, pushOutcomeMessage, type PushOutcome } from "@/lib/push";
import { announce } from "@/components/feedback/live-region";

/**
 * Why push could not be turned on (docs §21.3). Brave also gets the settings
 * address in a copyable form: web pages cannot link to brave:// URLs.
 */
export function PushProblem({ outcome, className }: { outcome: Exclude<PushOutcome, "subscribed">; className?: string }) {
  const { text } = useI18n();
  const [copied, setCopied] = React.useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(BRAVE_PRIVACY_URL);
      setCopied(true);
      announce(text("Copied. Paste it into a new Brave tab."));
      window.setTimeout(() => setCopied(false), 3000);
    } catch {
      announce(text("Couldn't copy. Select the address and copy it manually."));
    }
  }

  return (
    <span className={className} data-testid="push-problem" data-outcome={outcome}>
      {text(pushOutcomeMessage(outcome))}
      {outcome === "brave_blocked" ? (
        <span className="push-brave-address">
          <code data-testid="push-brave-address">{BRAVE_PRIVACY_URL}</code>
          <button type="button" className="push-copy-button" onClick={() => void copy()} data-testid="push-brave-copy">
            {copied ? <Check size={12} aria-hidden /> : <Copy size={12} aria-hidden />}
            {text(copied ? "Copied" : "Copy")}
          </button>
        </span>
      ) : null}
    </span>
  );
}
