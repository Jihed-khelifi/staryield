"use client";

import * as React from "react";
import { useFormContext } from "react-hook-form";

import { useI18n } from "@/i18n/i18n-provider";
import { pushApi, settingsApi } from "@/lib/api/account";
import { errorMessage } from "@/lib/api/errors";
import { disablePush, enablePush, existingSubscription, pushAvailability, pushOutcomeMessage, pushPermission, refreshPush, type PushOutcome } from "@/lib/push";
import { PushProblem } from "@/components/notifications/push-problem";
import { useAsync } from "@/lib/use-async";
import { announce } from "@/components/feedback/live-region";
import { FormControl, FormField, FormItem } from "@/components/ui/form";
import { Switch } from "@/components/ui/switch";

import type { SettingsFormValues } from "./settings-model";
import { useSettingsSync } from "./settings-sync";
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
 * Push notifications (docs §21.3, §23.6). The switch applies at once, without
 * "Save Changes": switching on asks for the browser permission, registers the
 * service worker, subscribes this browser and persists `notify_push: true`;
 * switching off persists `false` and removes this browser's subscription (the
 * account stops receiving push here; turning it on again re-subscribes
 * without a new prompt). When persisting fails the switch goes back to where
 * it was (and a browser subscription made for it is removed). Unsupported
 * browsers, blocked permissions and push-service failures are explained,
 * never fatal; every outcome message comes from `pushOutcomeMessage`.
 */
function PushRow() {
  const { text } = useI18n();
  const form = useFormContext<SettingsFormValues>();
  const sync = useSettingsSync();
  const config = useAsync((signal) => pushApi.config(signal), []);
  const [availability] = React.useState(pushAvailability);
  const supported = availability === "available";
  const [permission, setPermission] = React.useState(pushPermission);
  const [subscribed, setSubscribed] = React.useState<boolean | null>(null);
  const [busy, setBusy] = React.useState(false);
  const [problem, setProblem] = React.useState<Exclude<PushOutcome, "subscribed"> | null>(null);
  const [saveError, setSaveError] = React.useState<string | null>(null);
  const lastAction = React.useRef<"on" | "off">("on");
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

  /** Save the account switch on its own; the form keeps the saved value as its baseline. */
  async function persist(on: boolean): Promise<boolean> {
    try {
      const saved = await settingsApi.update({ notifications: { push: on } });
      form.resetField("notifications.push", { defaultValue: saved.notifications.push });
      sync?.adopt(saved);
      return true;
    } catch (error) {
      const message = errorMessage(error);
      setSaveError(message);
      announce(text(message));
      return false;
    }
  }

  async function turnOn() {
    lastAction.current = "on";
    if (!config.data?.public_key) return;
    const alreadyOn = form.getValues("notifications.push");
    setBusy(true);
    setProblem(null);
    setSaveError(null);
    const outcome = await enablePush(config.data.public_key);
    setPermission(pushPermission());
    if (outcome !== "subscribed") {
      setBusy(false);
      setProblem(outcome);
      announce(text(pushOutcomeMessage(outcome)));
      return;
    }
    if (!alreadyOn && !(await persist(true))) {
      // The account switch stays off, so this browser must not stay subscribed.
      await disablePush().catch(() => undefined);
      setSubscribed(false);
      setBusy(false);
      return;
    }
    setSubscribed(true);
    setBusy(false);
    announce(text("Push notifications are on in this browser."));
  }

  async function turnOff() {
    lastAction.current = "off";
    setProblem(null);
    setSaveError(null);
    setBusy(true);
    form.setValue("notifications.push", false, { shouldDirty: false });
    if (!(await persist(false))) {
      form.resetField("notifications.push", { defaultValue: true });
      setBusy(false);
      return;
    }
    try {
      await disablePush();
    } catch {
      // The server drops dead subscriptions on its own (404/410).
    } finally {
      setBusy(false);
      setSubscribed(false);
    }
    announce(text("Push notifications are off."));
  }

  let hint: React.ReactNode = null;
  if (availability !== "available") hint = <PushProblem outcome={availability} />;
  else if (config.error || (config.data && !enabled)) hint = text("Push notifications aren't available yet.");
  else if (saveError) hint = text(saveError);
  else if (problem) hint = <PushProblem outcome={problem} />;
  else if (permission === "denied") hint = <PushProblem outcome="denied" />;
  const retryable = saveError !== null || problem === "brave_blocked" || problem === "service_error" || problem === "failed";
  const retry = () => void (lastAction.current === "off" ? turnOff() : turnOn());

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
                    <button type="button" className="settings-link-button" onClick={retry} disabled={busy} data-testid="push-retry">
                      {text("Try again")}
                    </button>
                  </>
                ) : null}
              </p>
            ) : field.value && subscribed === false && permission !== "denied" ? (
              <p id={hintId} className="settings-description" data-testid="push-hint">
                {text("Not turned on in this browser yet.")}{" "}
                <button type="button" className="settings-link-button" onClick={() => void turnOn()} disabled={busy}>
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
              onCheckedChange={(on) => void (on ? turnOn() : turnOff())}
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
