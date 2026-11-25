import { describe, expect, it } from "vitest";

import en from "@/i18n/dictionaries/copy-en.json";
import es from "@/i18n/dictionaries/copy-es.json";
import fr from "@/i18n/dictionaries/copy-fr.json";
import { createTranslateCopy } from "@/i18n/translate-copy";
import type { AppNotification } from "@/lib/api/types";
import { lifecycleTemplates } from "@/lib/notifications/lifecycle-copy";
import { notificationDestination, notificationText } from "@/lib/notifications/notification-copy";

const textEn = createTranslateCopy(en);
const textFr = createTranslateCopy(fr);
const textEs = createTranslateCopy(es);

const n = (kind: string, data: Record<string, unknown> = {}, link = ""): AppNotification => ({
  id: "n1",
  kind,
  data,
  link,
  created_at: "2026-11-20T10:00:00Z",
});

// growth: the lifecycle nudges only state what was true when they were sent.
describe("lifecycle notification copy", () => {
  it.each([
    [{ free_minutes: 1, online_advisors: 4 }, "Your free minute is waiting", "You have 1 free minute to use on your first reading."],
    [{ free_minutes: 1200 }, "Your free minutes are waiting", "You have 1,200 free minutes to use on your first reading."],
    [{ free_minutes: 0, online_advisors: 2 }, "Advisors are online now", "Choose an advisor and start your first reading."],
    [{ free_minutes: "nope", online_advisors: -3 }, "Your first reading is waiting", "Choose an advisor and start your first reading whenever you are ready."],
    [{}, "Your first reading is waiting", "Choose an advisor and start your first reading whenever you are ready."],
  ])("first reading %j", (data, title, body) => {
    expect(notificationText(n("lifecycle.first_reading", data), "en", textEn)).toEqual({ title, body });
  });

  it.each([
    [{ online_advisors: 1 }, "An advisor is online now", "1 advisor is online now. Pick up where you left off."],
    [{ online_advisors: 12 }, "Advisors are online now", "12 advisors are online now. Pick up where you left off."],
    [{ online_advisors: 0 }, "Your daily horoscope is ready", "Read today's horoscope for your sign."],
  ])("win-back %j never claims advisors who are not online", (data, title, body) => {
    expect(notificationText(n("lifecycle.win_back", data), "en", textEn)).toEqual({ title, body });
  });

  it("renders every sentence in French and Spanish, with the numbers in the viewer's format", () => {
    for (const value of Object.values(lifecycleTemplates)) {
      expect(fr[value as keyof typeof fr], value).toBeTruthy();
      expect(es[value as keyof typeof es], value).toBeTruthy();
      expect(fr[value as keyof typeof fr]).not.toBe(value);
      expect(es[value as keyof typeof es]).not.toBe(value);
    }
    const free = notificationText(n("lifecycle.first_reading", { free_minutes: 1200 }), "fr", textFr);
    expect(free.body).toBe(`Vous avez ${new Intl.NumberFormat("fr").format(1200)} minutes offertes à utiliser pour votre première consultation.`);
    expect(notificationText(n("lifecycle.win_back", { online_advisors: 3 }), "es", textEs).body).toBe("Hay 3 asesores en línea. Retoma donde lo dejaste.");
    expect(notificationText(n("lifecycle.invite_friends"), "fr", textFr).title).toBe("Votre consultation vous a plu ?");
  });

  it("leads to the place where the next step is done when the API sent no link", () => {
    expect(notificationDestination(n("lifecycle.verify_reminder"), "en")).toBe("/en/profile/settings#account");
    expect(notificationDestination(n("lifecycle.first_reading"), "fr")).toBe("/fr/psychics");
    expect(notificationDestination(n("lifecycle.win_back"), "es")).toBe("/es/psychics");
    expect(notificationDestination(n("lifecycle.invite_friends"), "en")).toBe("/en/profile/invite");
    // The API's own link wins (the win-back sends people to the horoscope when nobody is online).
    expect(notificationDestination(n("lifecycle.win_back", { online_advisors: 0 }, "/profile"), "en")).toBe("/en/profile");
  });
});
