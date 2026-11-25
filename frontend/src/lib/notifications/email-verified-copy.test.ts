import { describe, expect, it } from "vitest";

import en from "@/i18n/dictionaries/copy-en.json";
import es from "@/i18n/dictionaries/copy-es.json";
import fr from "@/i18n/dictionaries/copy-fr.json";
import { createTranslateCopy } from "@/i18n/translate-copy";
import type { AppNotification } from "@/lib/api/types";
import { isAnnouncedElsewhere } from "@/lib/notifications/email-verified-copy";
import { notificationDestination, notificationText } from "@/lib/notifications/notification-copy";

const note: AppNotification = { id: "n1", kind: "account.email_verified", data: {}, link: "/profile/settings#account", created_at: "2026-11-05T10:00:00Z" };

describe("account.email_verified notification", () => {
  it("has its own title and body in every language", () => {
    const english = notificationText(note, "en", createTranslateCopy(en));
    expect(english).toEqual({ title: "Email verified", body: "Your email address is confirmed. Thank you for securing your account." });
    for (const [locale, dictionary] of [["fr", fr], ["es", es]] as const) {
      const copy = notificationText(note, locale, createTranslateCopy(dictionary));
      expect(copy.title).not.toBe(english.title);
      expect(copy.body).not.toBe(english.body);
      expect(copy.title && copy.body).toBeTruthy();
    }
  });

  it("opens the account section of the settings, also when the API sent no link", () => {
    expect(notificationDestination(note, "fr")).toBe("/fr/profile/settings#account");
    expect(notificationDestination({ ...note, link: "" }, "en")).toBe("/en/profile/settings#account");
  });

  it("is announced by its realtime event, not by a second toast", () => {
    expect(isAnnouncedElsewhere("account.email_verified")).toBe(true);
    expect(isAnnouncedElsewhere("export.ready")).toBe(false);
  });
});
