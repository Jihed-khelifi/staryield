import { describe, expect, it } from "vitest";

import en from "@/i18n/dictionaries/copy-en.json";
import es from "@/i18n/dictionaries/copy-es.json";
import fr from "@/i18n/dictionaries/copy-fr.json";
import { createTranslateCopy } from "@/i18n/translate-copy";
import type { AppNotification } from "@/lib/api/types";
import { notificationDestination, notificationText, referralNotificationTemplates } from "@/lib/notifications/notification-copy";

// growth: referrals. The notifications never name the friend.
const n = (kind: string, data: Record<string, unknown>, link = ""): AppNotification =>
  ({ id: "n1", kind, data, link, read: false, created_at: "2026-11-20T10:00:00Z" }) as unknown as AppNotification;
const textEn = createTranslateCopy(en);

describe("referral notifications", () => {
  it("say what they say, never a launch", () => {
    expect(notificationText(n("referral.joined", {}), "en", textEn)).toEqual({
      title: "A friend joined with your link",
      body: "Your free minutes are earned once they verify their email and complete a reading.",
    });
    // Legacy rows only (the API no longer sends this kind): the wording never mentions a launch.
    expect(notificationText(n("referral.qualified", { minutes: 5, launched: false }), "en", textEn)).toEqual({
      title: "A friend completed a reading",
      body: "Your 5 free minutes are being added to your wallet.",
    });
    expect(notificationText(n("referral.qualified", { minutes: 1, launched: false }), "en", textEn).body).toBe("Your free minute is being added to your wallet.");
    expect(notificationText(n("referral.qualified", { minutes: 5, launched: true }), "en", textEn).body).toBe("Your 5 free minutes are being added to your wallet.");
    expect(notificationText(n("referral.qualified", { minutes: 5 }), "en", textEn).body).toBe("Your 5 free minutes are being added to your wallet.");
    expect(notificationText(n("referral.reward_granted", { minutes: 15, friends: 3 }), "fr", createTranslateCopy(fr)).body).toBe(
      "15 minutes gratuites gagnées grâce à vos invitations ont été ajoutées à votre portefeuille.",
    );
    expect(notificationText(n("referral.reward_granted", { minutes: 1 }), "es", createTranslateCopy(es)).body).toBe(
      "Se añadió a tu monedero 1 minuto gratis de tus invitaciones.",
    );
  });

  it("a legacy referral.qualified row never mentions a launch, in any locale", () => {
    for (const [locale, dict] of [["en", en], ["fr", fr], ["es", es]] as const) {
      for (const data of [{ minutes: 5, launched: false }, { minutes: 1, launched: false }, { minutes: 5, launched: true }, { minutes: 5 }]) {
        const { title, body } = notificationText(n("referral.qualified", data), locale, createTranslateCopy(dict));
        expect(`${title} ${body}`, `${locale} ${JSON.stringify(data)}`).not.toMatch(/launch|lancement|lanzamiento|lance\b|lancer|lanza|beta|bêta/i);
        expect(body).not.toMatch(/\{\w+\}/);
      }
    }
  });

  it("lead to the invite page", () => {
    for (const kind of ["referral.joined", "referral.qualified", "referral.reward_granted"]) {
      expect(notificationDestination(n(kind, {}), "es")).toBe("/es/profile/invite");
      expect(notificationDestination(n(kind, {}, "/profile/invite"), "fr")).toBe("/fr/profile/invite");
    }
  });

  it.each(Object.values(referralNotificationTemplates).map((s) => [s]))("%s is translated in every locale", (key) => {
    for (const dict of [fr, es] as Record<string, string>[]) {
      expect(dict[key], key).toBeTruthy();
      expect(dict[key], key).not.toBe(key);
    }
  });
});
