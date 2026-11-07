"use client";

import * as React from "react";
import { BellRing, X } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { pushApi, settingsApi } from "@/lib/api/account";
import { dismissPrompt, promptDismissed, shouldShowPrompt } from "@/lib/chat/chat-alerts";
import { enablePush, pushPermission, pushSupported, type PushOutcome } from "@/lib/push";
import { announce } from "@/components/feedback/live-region";
import { Button } from "@/components/ui/button";

const PROBLEM: Record<Exclude<PushOutcome, "subscribed">, string> = {
  denied: "Notifications are blocked for this site. Allow them in your browser settings, then try again.",
  dismissed: "You closed the permission prompt. Turn the switch on again when you're ready.",
  unsupported: "This browser doesn't support push notifications.",
  failed: "We couldn't turn on notifications in this browser. Please try again.",
};

/**
 * Dismissible prompt above the composer: turn on notifications so new
 * messages reach the phone when the chat is closed. Shown once per user,
 * after their first message, when the browser supports push and the
 * permission is still undecided.
 */
export function ChatPushPrompt({ userId, hasSent }: { userId: string; hasSent: boolean }) {
  const { text } = useI18n();
  const [supported] = React.useState(pushSupported);
  const [dismissed, setDismissed] = React.useState(() => (typeof window === "undefined" ? true : promptDismissed(userId)));
  const [publicKey, setPublicKey] = React.useState<string | null>(null);
  const [permission, setPermission] = React.useState(pushPermission);
  const [busy, setBusy] = React.useState(false);
  const [problem, setProblem] = React.useState<string | null>(null);
  const [done, setDone] = React.useState(false);
  const active = supported && permission === "default" && !dismissed && hasSent;

  React.useEffect(() => {
    if (!active || publicKey) return;
    const ctrl = new AbortController();
    pushApi.config(ctrl.signal).then(
      (c) => {
        if (c.enabled && c.public_key) setPublicKey(c.public_key);
      },
      () => undefined,
    );
    return () => ctrl.abort();
  }, [active, publicKey]);

  if (done) {
    return (
      <p className="chat-push-prompt" role="status" data-testid="chat-push-done">
        <BellRing size={16} aria-hidden />
        <span>{text("Notifications are on. We'll let you know when a new message arrives.")}</span>
      </p>
    );
  }
  if (!shouldShowPrompt({ supported, permission, dismissed, hasSent, serverEnabled: Boolean(publicKey) }) && !(problem && !dismissed && supported && hasSent)) return null;

  async function turnOn() {
    if (!publicKey) return;
    setBusy(true);
    setProblem(null);
    const outcome = await enablePush(publicKey);
    setBusy(false);
    setPermission(pushPermission());
    if (outcome === "subscribed") {
      // The server only pushes when the account switch is on.
      void settingsApi.update({ notifications: { push: true } }).catch(() => undefined);
      setDone(true);
      announce(text("Notifications are on. We'll let you know when a new message arrives."));
      window.setTimeout(() => setDone(false), 5000);
      return;
    }
    setProblem(text(PROBLEM[outcome]));
  }

  return (
    <div className="chat-push-prompt" role="region" aria-label={text("Message notifications")} data-testid="chat-push-prompt">
      <BellRing size={16} aria-hidden />
      <div className="min-w-0 flex-1">
        <p className="chat-push-title">{text("Don't miss a reply")}</p>
        <p>{text("Turn on notifications to know when a new message arrives, even when this page is closed.")}</p>
        {problem ? (
          <p role="alert" className="chat-push-problem">
            {problem}
          </p>
        ) : null}
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <Button type="button" variant="cta" className="h-8 px-4 text-xs" onClick={() => void turnOn()} disabled={busy} aria-busy={busy || undefined}>
            {text(busy ? "Turning on…" : problem ? "Try again" : "Turn on notifications")}
          </Button>
          <button
            type="button"
            className="chat-push-later"
            onClick={() => {
              dismissPrompt(userId);
              setDismissed(true);
            }}
          >
            {text("Not now")}
          </button>
        </div>
      </div>
      <button
        type="button"
        className="chat-push-close"
        aria-label={text("Dismiss")}
        onClick={() => {
          dismissPrompt(userId);
          setDismissed(true);
        }}
      >
        <X size={14} aria-hidden />
      </button>
    </div>
  );
}
