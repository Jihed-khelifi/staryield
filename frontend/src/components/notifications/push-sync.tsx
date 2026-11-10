"use client";

import * as React from "react";

import { pushApi, settingsApi } from "@/lib/api/account";
import { useSession } from "@/lib/auth/session-provider";
import { syncPush } from "@/lib/push";

/** Message the service worker posts when the browser replaced the push subscription. */
export const SUBSCRIPTION_CHANGED_MESSAGE = "push.subscription_changed";

/**
 * Keeps this browser's push subscription registered with the API
 * (docs §21.3, §23.6). Once per page load, for every signed-in user, and at
 * once when the service worker reports a changed subscription; the endpoint is
 * re-sent whenever it differs from the last one sent. Never prompts.
 */
export function PushSync() {
  const { status, user } = useSession();
  const userId = status === "authenticated" ? (user?.id ?? "") : "";
  React.useEffect(() => {
    if (!userId) return;
    let live = true;
    const run = (force: boolean) => {
      void syncPush({
        userId,
        force,
        pushSwitchOn: async () => (await settingsApi.get()).notifications.push === true,
        publicKey: async () => {
          const config = await pushApi.config();
          return config.enabled && config.public_key ? config.public_key : null;
        },
      }).catch(() => undefined);
    };
    run(false);
    const sw = typeof navigator !== "undefined" ? navigator.serviceWorker : undefined;
    const onMessage = (event: MessageEvent) => {
      if (live && (event.data as { type?: unknown } | null)?.type === SUBSCRIPTION_CHANGED_MESSAGE) run(true);
    };
    sw?.addEventListener?.("message", onMessage);
    return () => {
      live = false;
      sw?.removeEventListener?.("message", onMessage);
    };
  }, [userId]);
  return null;
}
