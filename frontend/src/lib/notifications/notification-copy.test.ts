import { describe, expect, it } from "vitest";

import en from "@/i18n/dictionaries/copy-en.json";
import es from "@/i18n/dictionaries/copy-es.json";
import fr from "@/i18n/dictionaries/copy-fr.json";
import { createTranslateCopy } from "@/i18n/translate-copy";
import type { AppNotification } from "@/lib/api/types";
import { errorCopy } from "@/lib/api/errors";
import { attachmentProblemCopy } from "@/lib/chat/attachments";
import {
  notificationHref,
  notificationTemplates,
  notificationText,
  pickLocalized,
  relativeTime,
  requestStatusCopy,
  withdrawalStatusCopy,
} from "@/lib/notifications/notification-copy";

const n = (kind: string, data: Record<string, unknown>, link = "/profile"): AppNotification => ({
  id: "n1",
  kind,
  data,
  link,
  created_at: "2026-10-20T10:00:00Z",
});

const textEn = createTranslateCopy(en);
const textFr = createTranslateCopy(fr);
const textEs = createTranslateCopy(es);

describe("notificationText", () => {
  it.each([
    ["chat.requested", { session_id: "s", client_name: "Amélie" }, "New chat request", "Amélie would like to start a live chat."],
    ["chat.missed", { session_id: "s", client_name: "Marco" }, "Missed chat request", "Marco's chat request expired before you could answer."],
    ["chat.review_prompt", { session_id: "s", psychic_name: "Theo", psychic_slug: "theo" }, "How was your reading?", "Rate your chat with Theo."],
    ["review.received", { review_id: "r", rating: 5 }, "New review", "A client rated your chat 5 out of 5."],
    ["request.created", { request_id: "r", service_name: "Love reading", client_name: "Lucía" }, "New request", "Lucía ordered Love reading."],
    ["request.updated", { request_id: "r", service_name: "Love reading", status: "delivered" }, "Request update", "Love reading · Delivered"],
    ["purchase.succeeded", { purchase_id: "p", credits: 1200 }, "Credits added", "1,200 credits were added to your balance."],
    ["credits.adjusted", { amount: 50 }, "Credits added", "Staryield added 50 credits to your balance."],
    ["credits.adjusted", { amount: -30 }, "Balance adjusted", "30 credits were removed from your balance."],
    ["withdrawal.updated", { withdrawal_id: "w", status: "paid", amount_credits: 3000 }, "Withdrawal update", "Your withdrawal of 3,000 credits has been paid."],
    ["withdrawal.updated", { withdrawal_id: "w", status: "rejected", amount_credits: 2500 }, "Withdrawal update", "Your withdrawal of 2,500 credits was rejected. The credits are available again."],
  ])("renders %s in English", (kind, data, title, body) => {
    expect(notificationText(n(kind, data), "en", textEn)).toEqual({ title, body });
  });

  it("renders in the viewer's language from kind + data", () => {
    const prompt = n("chat.review_prompt", { psychic_name: "Theo" });
    expect(notificationText(prompt, "fr", textFr)).toEqual({ title: "Comment s'est passée votre lecture ?", body: "Notez votre chat avec Theo." });
    expect(notificationText(prompt, "es", textEs)).toEqual({ title: "¿Qué tal fue tu lectura?", body: "Valora tu chat con Theo." });
    expect(notificationText(n("purchase.succeeded", { credits: 1200 }), "fr", textFr).body).toBe(`${new Intl.NumberFormat("fr").format(1200)} crédits ont été ajoutés à votre solde.`);
    expect(notificationText(n("request.updated", { service_name: "X", status: "accepted" }), "es", textEs).body).toBe("X · En curso");
  });

  it("uses broadcast copy for the locale, falling back to English", () => {
    const b = n("broadcast", { broadcast_id: "b", title: { en: "Full moon", fr: "Pleine lune" }, body: { en: "Advisors online", es: "Asesores en línea" } });
    expect(notificationText(b, "fr", textFr)).toEqual({ title: "Pleine lune", body: "Advisors online" });
    expect(notificationText(b, "es", textEs)).toEqual({ title: "Full moon", body: "Asesores en línea" });
    expect(notificationText(b, "en", textEn)).toEqual({ title: "Full moon", body: "Advisors online" });
  });

  it("degrades gracefully on unknown kinds and missing data", () => {
    expect(notificationText(n("something.new", {}), "fr", textFr)).toEqual({ title: "Notification", body: "" });
    expect(notificationText(n("chat.requested", {}), "en", textEn).body).toBe("A client would like to start a live chat.");
    expect(notificationText(n("broadcast", { title: {}, body: null }), "en", textEn)).toEqual({ title: "Notification", body: "" });
  });

  it("pickLocalized prefers the locale, then English, then any value", () => {
    expect(pickLocalized({ es: "Hola" }, "fr")).toBe("Hola");
    expect(pickLocalized({ fr: "  ", en: "Hi" }, "fr")).toBe("Hi");
    expect(pickLocalized("plain", "fr")).toBe("plain");
    expect(pickLocalized(42, "fr")).toBe("");
  });
});

describe("relativeTime", () => {
  const now = Date.parse("2026-10-20T12:00:00Z");
  it("formats with Intl.RelativeTimeFormat in each language", () => {
    const rtf = (l: string) => new Intl.RelativeTimeFormat(l, { numeric: "auto", style: "short" });
    expect(relativeTime("2026-10-20T11:59:50Z", "en", now)).toBe(rtf("en").format(0, "second"));
    expect(relativeTime("2026-10-20T11:55:00Z", "fr", now)).toBe(rtf("fr").format(-5, "minute"));
    expect(relativeTime("2026-10-20T09:00:00Z", "es", now)).toBe(rtf("es").format(-3, "hour"));
    expect(relativeTime("2026-10-19T12:00:00Z", "en", now)).toBe(rtf("en").format(-1, "day"));
    expect(relativeTime("2026-09-01T12:00:00Z", "en", now)).toBe(rtf("en").format(-2, "month"));
    expect(relativeTime("not a date", "en", now)).toBe("");
  });
});

describe("notificationHref", () => {
  it("prefixes the current locale to app paths", () => {
    expect(notificationHref("/chatroom?reader=theo&review=abc", "fr")).toBe("/fr/chatroom?reader=theo&review=abc");
    expect(notificationHref("/profile/analytics", "es")).toBe("/es/profile/analytics");
    expect(notificationHref("/", "en")).toBe("/en");
    expect(notificationHref("/en/profile", "fr")).toBe("/fr/profile");
  });

  it.each([["https://evil.example/x"], ["//evil.example"], ["javascript:alert(1)"], ["/\\evil"], [""], ["profile"]])("rejects %s", (link) => {
    expect(notificationHref(link, "en")).toBeNull();
  });
});

describe("dictionary coverage of non-literal copy", () => {
  const all = [
    ...Object.values(notificationTemplates),
    ...Object.values(requestStatusCopy),
    ...Object.values(withdrawalStatusCopy),
    ...Object.values(attachmentProblemCopy),
    errorCopy.step_up_required,
    errorCopy.invalid_code,
  ];
  it.each(all.map((s) => [s]))("%s is translated in every locale", (key) => {
    for (const dict of [en, fr, es] as Record<string, string>[]) expect(dict[key], key).toBeTruthy();
  });
});
