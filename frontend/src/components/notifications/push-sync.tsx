"use client";

import * as React from "react";

import { pushApi, settingsApi } from "@/lib/api/account";
import { useSession } from "@/lib/auth/session-provider";
import { syncPush } from "@/lib/push";

/**
 * Keeps this browser's push subscription registered with the API
 * (docs §21.3). Once per page load, for every signed-in user; never prompts.
 */
export function PushSync() {
  const { status, user } = useSession();
  const userId = status === "authenticated" ? (user?.id ?? "") : "";
  React.useEffect(() => {
    if (!userId) return;
    void syncPush({
      userId,
      pushSwitchOn: async () => (await settingsApi.get()).notifications.push === true,
      publicKey: async () => {
        const config = await pushApi.config();
        return config.enabled && config.public_key ? config.public_key : null;
      },
    }).catch(() => undefined);
  }, [userId]);
  return null;
}
