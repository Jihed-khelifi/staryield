/** Test fixtures for the invite surfaces (growth: referrals). */
import type { MyReferrals } from "@/lib/api/referrals";

export function myReferralsFixture(over: { program?: Partial<MyReferrals["program"]>; stats?: Partial<MyReferrals["stats"]> } = {}): MyReferrals {
  return {
    code: "ABCD2345",
    link: "https://staryield.net/en?ref=ABCD2345",
    program: { enabled: true, reward_minutes: 5, max_rewards: 20, launched: false, ...over.program },
    stats: { joined: 0, qualified: 0, pending_minutes: 0, granted_minutes: 0, remaining_rewards: 20, ...over.stats },
  };
}
