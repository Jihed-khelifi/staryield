import { describe, expect, it } from "vitest";

import en from "@/i18n/dictionaries/copy-en.json";
import es from "@/i18n/dictionaries/copy-es.json";
import fr from "@/i18n/dictionaries/copy-fr.json";
import { createTranslateCopy } from "@/i18n/translate-copy";
import { progressLine, rewardSentence, rewardsLeft } from "@/components/referrals/referral-copy";
import { myReferralsFixture } from "@/test/referral-fixtures";

const textEn = createTranslateCopy(en);

describe("invite copy", () => {
  it("promises the minutes at launch before the launch, and plainly after", () => {
    expect(rewardSentence({ enabled: true, reward_minutes: 5, launched: false }, "en", textEn)).toBe(
      "For every friend who joins with your link and completes a reading, you get 5 free minutes, added when Staryield launches.",
    );
    expect(rewardSentence({ enabled: true, reward_minutes: 5, launched: true }, "en", textEn)).toBe(
      "For every friend who joins with your link and completes a reading, you get 5 free minutes.",
    );
    expect(rewardSentence({ enabled: true, reward_minutes: 1, launched: true }, "en", textEn)).toContain("you get 1 free minute.");
  });

  it("promises nothing while the program is paused", () => {
    for (const launched of [false, true]) {
      expect(rewardSentence({ enabled: false, reward_minutes: 5, launched }, "en", textEn)).toBe("Rewards for new invitations are paused for now.");
    }
  });

  it("summarises the progress with singular and plural forms", () => {
    const { program } = myReferralsFixture();
    expect(progressLine(myReferralsFixture().stats, program, "en", textEn)).toBe("No friend has joined with your link yet.");
    expect(progressLine({ ...myReferralsFixture().stats, joined: 1 }, program, "en", textEn)).toBe("1 friend joined · 0 completed a reading");
    expect(progressLine({ joined: 3, qualified: 2, pending_minutes: 10, granted_minutes: 0, remaining_rewards: 18 }, program, "en", textEn)).toBe(
      "3 friends joined · 2 completed a reading · 10 minutes pending until launch",
    );
    expect(
      progressLine({ joined: 3, qualified: 3, pending_minutes: 5, granted_minutes: 10, remaining_rewards: 17 }, { launched: true }, "en", textEn),
    ).toBe("3 friends joined · 3 completed a reading · 10 minutes added · 5 minutes on their way");
    expect(progressLine({ joined: 1, qualified: 1, pending_minutes: 1, granted_minutes: 1, remaining_rewards: 0 }, { launched: true }, "en", textEn)).toBe(
      "1 friend joined · 1 completed a reading · 1 minute added · 1 minute on its way",
    );
  });

  it("formats numbers and translates every variant", () => {
    const stats = { joined: 1200, qualified: 2, pending_minutes: 10, granted_minutes: 0, remaining_rewards: 18 };
    expect(progressLine(stats, { launched: false }, "en", textEn)).toContain("1,200 friends joined");
    const textFr = createTranslateCopy(fr);
    const textEs = createTranslateCopy(es);
    for (const [locale, t] of [["fr", textFr], ["es", textEs]] as const) {
      const line = progressLine(stats, { launched: false }, locale, t);
      expect(line).not.toContain("friends joined");
      expect(line).not.toMatch(/\{\w+\}/);
      expect(rewardSentence({ enabled: true, reward_minutes: 5, launched: false }, locale, t)).not.toContain("For every friend");
      expect(rewardSentence({ enabled: false, reward_minutes: 5, launched: false }, locale, t)).not.toContain("Rewards for new invitations");
    }
    expect(rewardsLeft(stats, { max_rewards: 20 }, "en", textEn)).toBe("18 of 20");
  });
});
