"use client";

import * as React from "react";
import { useFormContext } from "react-hook-form";

import { useI18n } from "@/i18n/i18n-provider";
import { pushApi } from "@/lib/api/account";
import { disablePush, enablePush, existingSubscription, pushAvailability, pushPermission, refreshPush, type PushOutcome } from "@/lib/push";
import { PushProblem } from "@/components/notifications/push-problem";
import { useAsync } from "@/lib/use-async";
import { announce } from "@/components/feedback/live-region";
import { FormControl, FormField, FormItem } from "@/components/ui/form";
import { Switch } from "@/components/ui/switch";

import type { SettingsFormValues } from "./settings-model";
import { SettingsCard } from "./settings-ui";

type Toggle = Exclude<keyof SettingsFormValues["notifications"], "push">;

const PRIMARY: { name: Toggle; label: string }[] = [
  { name: "email", label: "Email Notifications" },
  { name: "marketing", label: "Marketing Updates" },
];

const TOPICS: { name: Toggle; label: string }[] = [
  { name: "horoscope", label: "Daily Horoscope" },
  { name: "events", label: "Cosmic events" },
  { name: "chart", label: "Changes in your chart" },
];

function SwitchRow({ name, label, testId }: { name: Toggle; label: string; testId?: string }) {
  const { text } = useI18n();
  const form = useFormContext<SettingsFormValues>();
  const labelId = React.useId();
  return (
    <FormField
      control={form.control}
      name={`notifications.${name}`}
      render={({ field }) => (
        <FormItem className="settings-switch-row">
          <span id={labelId} className="settings-switch-label">
            {text(label)}
          </span>
          <FormControl>
            <Switch
              ref={field.ref}
              name={field.name}
              checked={field.value}
              onCheckedChange={field.onChange}
              onBlur={field.onBlur}
              aria-labelledby={labelId}
              data-testid={testId ?? `notify-${name}`}
            />
          </FormControl>
        </FormItem>
      )}
    />
  );
}

/**
 * Push notifications: switching on asks for the browser permission,
 * registers the service worker and subscribes this browser; the account
 * preference itself is saved with the other settings. Unsupported browsers,
 * blocked permissions and push-service failures are explained, never fatal.
 */
function PushRow() {
  const { text } = useI18n();
  const form = useFormContext<SettingsFormValues>();
  const config = useAsync((signal) => pushApi.config(signal), []);
  const [availability] = React.useState(pushAvailability);
  const supported = availability === "available";
  const [permission, setPermission] = React.useState(pushPermission);
  const [subscribed, setSubscribed] = React.useState<boolean | null>(null);
  const [busy, setBusy] = React.useState(false);
  const [problem, setProblem] = React.useState<Exclude<PushOutcome, "subscribed"> | null>(null);
  const labelId = React.useId();
  const hintId = React.useId();
  const preference = form.getValues("notifications.push");

  React.useEffect(() => {
    let live = true;
    void existingSubscription().then((sub) => {
      if (!live) return;
      setSubscribed(Boolean(sub));
      // Keep the server's copy of this browser current (keys rotate).
      if (sub && preference) void refreshPush();
    });
    return () => {
      live = false;
    };
    // Checked once per visit; the switch updates `subscribed` itself.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const enabled = config.data?.enabled === true && Boolean(config.data.public_key);
  const unavailable = !supported || (config.data !== null && !enabled) || config.error !== null;

  async function turnOn(onChange: (value: boolean) => void) {
    if (!config.data?.public_key) return;
    setBusy(true);
    setProblem(null);
    const outcome = await enablePush(config.data.public_key);
    setBusy(false);
    setPermission(pushPermission());
    if (outcome === "subscribed") {
      setSubscribed(true);
      onChange(true);
      announce(text("Push notifications are on in this browser. Save your changes to keep them."));
      return;
    }
    setProblem(outcome);
  }

  async function turnOff(onChange: (value: boolean) => void) {
    onChange(false);
    setProblem(null);
    setBusy(true);
    try {
      await disablePush();
    } catch {
      // The server drops dead subscriptions on its own (404/410).
    } finally {
      setBusy(false);
      setSubscribed(false);
    }
  }

  let hint: React.ReactNode = null;
  if (availability !== "available") hint = <PushProblem outcome={availability} />;
  else if (config.error || (config.data && !enabled)) hint = text("Push notifications aren't available yet.");
  else if (problem) hint = <PushProblem outcome={problem} />;
  else if (permission === "denied") hint = <PushProblem outcome="denied" />;
  const retryable = problem === "brave_blocked" || problem === "service_error" || problem === "failed";

  return (
    <FormField
      control={form.control}
      name="notifications.push"
      render={({ field }) => (
        <FormItem className="settings-switch-row settings-switch-row-described" data-testid="push-row">
          <div>
            <span id={labelId} className="settings-switch-label">
              {text("Push Notifications")}
            </span>
            {hint ? (
              <p id={hintId} className="settings-description" role="status" data-testid="push-hint">
                {hint}
                {retryable ? (
                  <>
                    {" "}
                    <button type="button" className="settings-link-button" onClick={() => void turnOn(field.onChange)} disabled={busy} data-testid="push-retry">
                      {text("Try again")}
                    </button>
                  </>
                ) : null}
              </p>
            ) : field.value && subscribed === false && permission !== "denied" ? (
              <p id={hintId} className="settings-description" data-testid="push-hint">
                {text("Not turned on in this browser yet.")}{" "}
                <button type="button" className="settings-link-button" onClick={() => void turnOn(field.onChange)} disabled={busy}>
                  {text("Turn on here")}
                </button>
              </p>
            ) : null}
          </div>
          <FormControl>
            <Switch
              ref={field.ref}
              name={field.name}
              checked={field.value}
              disabled={busy || (!field.value && (unavailable || permission === "denied" || config.loading))}
              aria-busy={busy || undefined}
              onCheckedChange={(on) => void (on ? turnOn(field.onChange) : turnOff(field.onChange))}
              onBlur={field.onBlur}
              aria-labelledby={labelId}
              aria-describedby={hint || (field.value && subscribed === false) ? hintId : undefined}
              data-testid="notify-push"
            />
          </FormControl>
        </FormItem>
      )}
    />
  );
}

/** Notifications (Figma "Card / Notifications") plus the topic switches. */
export function NotificationsCard() {
  const { text } = useI18n();
  return (
    <SettingsCard id="notifications" icon="bell" title={text("Notifications")} testId="settings-notifications">
      <div className="settings-switches">
        <SwitchRow {...PRIMARY[0]} />
        <PushRow />
        <SwitchRow {...PRIMARY[1]} />
      </div>
      <div className="settings-switches settings-topics" role="group" aria-label={text("Topics")}>
        {TOPICS.map((topic) => (
          <SwitchRow key={topic.name} {...topic} />
        ))}
      </div>
    </SettingsCard>
  );
}
