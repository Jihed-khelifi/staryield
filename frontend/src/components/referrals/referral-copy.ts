/**
 * Copy of the invite surfaces (growth: referrals) that depends on numbers or
 * on the program's state. Singular and plural are separate sentences so
 * every language translates them naturally. Pure functions: unit-tested.
 */
import type { TranslationValues } from "@/i18n/types";
import type { ReferralProgramView, ReferralStats } from "@/lib/api/referrals";

type Translate = <T>(value: T, values?: TranslationValues) => T;

const fmt = (locale: string, n: number) => new Intl.NumberFormat(locale).format(n);

/**
 * "For every friend who joins … you get 5 free minutes." While the program is paused
 * nothing is promised: new sign-ups and readings earn nothing then.
 */
export function rewardSentence(program: Pick<ReferralProgramView, "enabled" | "reward_minutes">, locale: string, text: Translate): string {
  if (!program.enabled) return text("Rewards for new invitations are paused for now.");
  const minutes = fmt(locale, program.reward_minutes);
  return program.reward_minutes === 1
    ? text("For every friend who joins with your link and completes a reading, you get 1 free minute.")
    : text("For every friend who joins with your link and completes a reading, you get {value0} free minutes.", { value0: minutes });
}

export function friendsJoined(n: number, locale: string, text: Translate): string {
  return n === 1 ? text("1 friend joined") : text("{value0} friends joined", { value0: fmt(locale, n) });
}

export function friendsCompleted(n: number, locale: string, text: Translate): string {
  return n === 1 ? text("1 completed a reading") : text("{value0} completed a reading", { value0: fmt(locale, n) });
}

/** Minutes earned: "added", and any still on their way. */
export function minutesProgress(stats: ReferralStats, locale: string, text: Translate): string[] {
  const out: string[] = [];
  if (stats.granted_minutes > 0)
    out.push(stats.granted_minutes === 1 ? text("1 minute added") : text("{value0} minutes added", { value0: fmt(locale, stats.granted_minutes) }));
  if (stats.pending_minutes > 0)
    out.push(
      stats.pending_minutes === 1 ? text("1 minute on its way") : text("{value0} minutes on their way", { value0: fmt(locale, stats.pending_minutes) }),
    );
  return out;
}

/** The one-line progress of the overview card and the prompt ("2 friends joined · 1 completed a reading · 5 minutes on their way"). */
export function progressLine(stats: ReferralStats, locale: string, text: Translate): string {
  if (stats.joined === 0 && stats.qualified === 0 && stats.granted_minutes === 0) return text("No friend has joined with your link yet.");
  return [friendsJoined(stats.joined, locale, text), friendsCompleted(stats.qualified, locale, text), ...minutesProgress(stats, locale, text)].join(" · ");
}

/** "Rewards left: 18 of 20". */
export function rewardsLeft(stats: ReferralStats, program: Pick<ReferralProgramView, "max_rewards">, locale: string, text: Translate): string {
  return text("{value0} of {value1}", { value0: fmt(locale, stats.remaining_rewards), value1: fmt(locale, program.max_rewards) });
}
