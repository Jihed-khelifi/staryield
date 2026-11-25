import { describe, expect, it } from "vitest";

import en from "@/i18n/dictionaries/copy-en.json";
import es from "@/i18n/dictionaries/copy-es.json";
import fr from "@/i18n/dictionaries/copy-fr.json";
import { createTranslateCopy } from "@/i18n/translate-copy";
import type { AppNotification } from "@/lib/api/types";
import { autoAwayNotificationTemplates, notificationDestination, notificationText } from "@/lib/notifications/notification-copy";

// follow-ups: auto-away. The advisor was set to Away after N unanswered requests.
const n = (data: Record<string, unknown>, link = "/chatroom"): AppNotification =>
  ({ id: "n1", kind: "availability.auto_away", data, link, created_at: "2026-11-25T10:00:00Z" }) as AppNotification;

describe("availability.auto_away notification", () => {
  it("says how many requests were missed, in every language", () => {
    expect(notificationText(n({ missed: 3 }), "en", createTranslateCopy(en))).toEqual({
      title: "Set to Away automatically",
      body: "You were set to Away after 3 missed requests.",
    });
    expect(notificationText(n({ missed: 5 }), "fr", createTranslateCopy(fr)).body).toBe("Votre statut est passé à Absent après 5 demandes manquées.");
    expect(notificationText(n({ missed: 2 }), "es", createTranslateCopy(es)).body).toBe("Tu estado cambió a Ausente tras 2 solicitudes sin responder.");
  });

  it("leads to the chatroom, where the availability control is, even without a link", () => {
    expect(notificationDestination(n({ missed: 3 }), "en")).toBe("/en/chatroom");
    expect(notificationDestination(n({ missed: 3 }, ""), "fr")).toBe("/fr/chatroom");
  });

  it("never shows an unfilled placeholder", () => {
    for (const text of [createTranslateCopy(en), createTranslateCopy(fr), createTranslateCopy(es)]) {
      const copy = notificationText(n({}), "en", text);
      expect(`${copy.title} ${copy.body}`).not.toMatch(/\{\w+\}/);
    }
  });

  it.each(Object.values(autoAwayNotificationTemplates).map((s) => [s]))("%s is translated in every locale", (key) => {
    for (const dict of [fr, es] as Record<string, string>[]) {
      expect(dict[key], key).toBeTruthy();
      expect(dict[key], key).not.toBe(key);
    }
  });
});
