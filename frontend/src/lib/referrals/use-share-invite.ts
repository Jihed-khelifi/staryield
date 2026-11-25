"use client";

import { referralsApi, type ReferralProgramView } from "@/lib/api/referrals";
import { useSession } from "@/lib/auth/session-provider";
import { referralProgramOf } from "@/lib/referral";
import { useSiteConfig } from "@/lib/site-config/site-config-provider";
import { useAsync } from "@/lib/use-async";

export type ShareInvite = { code: string; program: ReferralProgramView };

/**
 * The invite a signed-in client's shared results carry (growth: referrals ×
 * share images): a friend who opens the shared link and signs up counts as
 * invited. Only for clients while the program is on (public configuration, so
 * nothing is requested otherwise), and only once `active` (the share dialog is
 * open). Best effort: while it loads, or when `GET /me/referrals` fails, the
 * share goes out with the plain link and nothing is shown.
 */
export function useShareInvite(active: boolean): ShareInvite | null {
  const { user } = useSession();
  const { config } = useSiteConfig();
  const eligible = active && user?.role === "client" && referralProgramOf(config).enabled;
  const { data } = useAsync((signal) => referralsApi.mine(signal), [user?.id], eligible);
  if (!eligible || !data?.program.enabled) return null;
  return { code: data.code, program: data.program };
}
