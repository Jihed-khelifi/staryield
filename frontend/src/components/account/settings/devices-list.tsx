"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { devicesApi, type Device } from "@/lib/api/account";
import { useSession } from "@/lib/auth/session-provider";
import { usePagedList } from "@/lib/use-paged-list";
import { announce } from "@/components/feedback/live-region";
import { ErrorState } from "@/components/feedback/error-state";
import { useToast } from "@/components/feedback/toast-provider";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { CursorPager } from "@/components/ui/pagination";
import { SkeletonList } from "@/components/ui/skeleton";

import { formatDateTime, relativeTime } from "./format";
import { SettingsSubheading } from "./settings-ui";

const PAGE_SIZE = 5;

/** "Chrome on macOS" from the parsed browser and platform (localized joiner). */
export function deviceName(device: Pick<Device, "browser" | "os" | "label">, text: ReturnType<typeof useI18n>["text"]): string {
  if (device.browser && device.os) return text("{value0} on {value1}", { value0: device.browser, value1: device.os });
  return device.browser || device.os || text("Unknown device");
}

type Pending = { kind: "one"; device: Device } | { kind: "others" } | null;

/** Signed-in devices (one per refresh-token family), newest activity first. */
export function DevicesList() {
  const { text, locale } = useI18n();
  const toast = useToast();
  const { user } = useSession();
  const list = usePagedList((cursor, signal) => devicesApi.list(cursor, PAGE_SIZE, signal), [user?.id]);
  const [pending, setPending] = React.useState<Pending>(null);
  const [busy, setBusy] = React.useState(false);
  const headingId = React.useId();
  const [now] = React.useState(() => Date.now());

  const hasOthers = list.items.some((d) => !d.current) || list.hasNext || list.hasPrev;

  async function confirm() {
    if (!pending) return;
    setBusy(true);
    try {
      if (pending.kind === "one") {
        await devicesApi.revoke(pending.device.id);
        const name = deviceName(pending.device, text);
        list.removeItem((d) => d.id === pending.device.id);
        if (list.items.length <= 1 && (list.hasNext || list.hasPrev)) list.reset();
        toast.show({ tone: "success", message: text("{value0} was signed out.", { value0: name }) });
      } else {
        const res = await devicesApi.revokeOthers();
        list.reset();
        const message =
          res.signed_out === 1 ? text("Signed out of 1 other device.") : text("Signed out of {value0} other devices.", { value0: res.signed_out });
        toast.show({ tone: "success", message });
        announce(message);
      }
      setPending(null);
    } catch (err) {
      toast.showError(err);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="settings-devices" aria-labelledby={headingId} role="group">
      <SettingsSubheading id={headingId}>{text("Logged In Devices")}</SettingsSubheading>
      {list.error ? (
        <ErrorState error={list.error} onRetry={list.reload} compact />
      ) : list.loading && list.items.length === 0 ? (
        <div aria-busy="true">
          <p role="status" className="sr-only">
            {text("Loading your devices…")}
          </p>
          <SkeletonList rows={2} rowClassName="h-11" />
        </div>
      ) : (
        <ul className="settings-list" data-testid="devices-list">
          {list.items.map((device) => {
            const seen = relativeTime(device.last_seen_at, locale, now);
            const name = deviceName(device, text);
            return (
              <li key={device.id} className="settings-row settings-device" data-current={device.current || undefined} data-testid="device-row">
                <span className="settings-row-main">
                  <span aria-hidden className="settings-dot" data-tone={device.current ? "current" : "other"} />
                  <span className="settings-row-label">
                    {device.current ? text("{value0} (Current)", { value0: name }) : name}
                  </span>
                </span>
                <span className="settings-device-meta">
                  <span className="settings-row-hint" title={text("Signed in {value0}", { value0: formatDateTime(device.signed_in_at, locale) })}>
                    {device.current || !seen ? text("Active now") : text("Last seen {value0}", { value0: seen })}
                  </span>
                  {device.current ? null : (
                    <button
                      type="button"
                      className="settings-link-button"
                      onClick={() => setPending({ kind: "one", device })}
                      aria-label={text("Sign out {value0}", { value0: name })}
                    >
                      {text("Sign out")}
                    </button>
                  )}
                </span>
              </li>
            );
          })}
        </ul>
      )}
      <CursorPager
        page={list.page}
        hasPrev={list.hasPrev}
        hasNext={list.hasNext}
        onPrev={list.prev}
        onNext={list.next}
        loading={list.loading}
        label={text("Devices pages")}
      />
      {hasOthers ? (
        <Button type="button" variant="ghost" size="lg" className="settings-text-action" onClick={() => setPending({ kind: "others" })} data-testid="sign-out-others">
          {text("Sign out of all other devices")}
        </Button>
      ) : null}

      <AlertDialog open={pending !== null} onOpenChange={(open) => (!open && !busy ? setPending(null) : undefined)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pending?.kind === "one"
                ? text("Sign out {value0}?", { value0: deviceName(pending.device, text) })
                : text("Sign out of all other devices?")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {pending?.kind === "one"
                ? text("That device will need your password to sign in again.")
                : text("Every other browser and app signed in to your account will need your password to sign in again. This device stays signed in.")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy}>{text("Cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={(event) => {
                event.preventDefault();
                void confirm();
              }}
              disabled={busy}
              aria-busy={busy}
            >
              {text(busy ? "Signing out…" : "Sign out")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
