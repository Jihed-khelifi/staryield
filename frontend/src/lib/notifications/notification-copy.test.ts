import { describe, expect, it } from "vitest";

import en from "@/i18n/dictionaries/copy-en.json";
import es from "@/i18n/dictionaries/copy-es.json";
import fr from "@/i18n/dictionaries/copy-fr.json";
import { createTranslateCopy } from "@/i18n/translate-copy";
import type { AppNotification, NotificationKind } from "@/lib/api/types";
import { errorCopy } from "@/lib/api/errors";
import { attachmentProblemCopy } from "@/lib/chat/attachments";
import {
  NOTIFICATION_KINDS,
  notificationDestination,
  notificationFallbackLink,
  notificationHref,
  notificationTemplates,
  notificationText,
  pickLocalized,
  relativeTime,
  refundResolvedCopy,
  requestStatusCopy,
  subscriptionCanceledCopy,
  subscriptionCanceledPlainCopy,
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
    // advisor settings: photo review decisions
    ["photo.approved", { submission_id: "p" }, "Photo approved", "Your new profile photo is now on your profile."],
    ["photo.rejected", { submission_id: "p", reason: "Your face is not visible." }, "Photo not approved", "Your new profile photo was not approved: Your face is not visible."],
    ["photo.rejected", { submission_id: "p" }, "Photo not approved", "Your new profile photo was not approved. Upload another one from your settings."],
    ["support.replied", { reference: "SY-7K2M9Q", subject: "Billing question" }, "Support replied to your request", "SY-7K2M9Q: Billing question"],
    ["support.replied", { reference: "SY-7K2M9Q" }, "Support replied to your request", "We answered your request SY-7K2M9Q."],
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

  it("tells a refund request's resolution by status and sends the person to their billing history (§24.4)", () => {
    const resolved = (status: string) => n("refund_request.resolved", { reference: "RR-AB12CD34", status });
    expect(notificationText(resolved("approved"), "en", textEn).body).toBe("Your refund request RR-AB12CD34 was approved.");
    expect(notificationText(resolved("denied"), "en", textEn).body).toBe("Your refund request RR-AB12CD34 was declined.");
    expect(notificationText(resolved("credited"), "en", textEn).body).toContain("credits added");
    expect(notificationText(resolved("something-new"), "en", textEn).body).toBe(refundResolvedCopy.other.replace("{reference}", "RR-AB12CD34"));
    expect(notificationDestination(n("refund_request.resolved", {}, ""), "fr")).toBe("/fr/profile/settings#billing");
  });

  it("pickLocalized prefers the locale, then English, then any value", () => {
    expect(pickLocalized({ es: "Hola" }, "fr")).toBe("Hola");
    expect(pickLocalized({ fr: "  ", en: "Hi" }, "fr")).toBe("Hi");
    expect(pickLocalized("plain", "fr")).toBe("plain");
    expect(pickLocalized(42, "fr")).toBe("");
  });
});

/** Sample data per kind; typed as a Record so a new NotificationKind must be added here too. */
const samples: Record<NotificationKind, Record<string, unknown>> = {
  "chat.requested": { client_name: "Amélie" },
  "chat.missed": { client_name: "Marco" },
  "chat.review_prompt": { psychic_name: "Theo" },
  "review.received": { rating: 4 },
  "request.created": { service_name: "Love reading", client_name: "Lucía" },
  "request.updated": { service_name: "Love reading", status: "accepted" },
  "purchase.succeeded": { credits: 100 },
  "credits.adjusted": { amount: 10 },
  "withdrawal.updated": { status: "paid", amount_credits: 100 },
  "photo.approved": {},
  "photo.rejected": { reason: "Blurry." },
  "support.replied": { reference: "SY-1", subject: "Hi" },
  broadcast: { title: { en: "Full moon" }, body: { en: "Advisors online" } },
  "waitlist.available": { psychic_name: "Theo", psychic_slug: "theo" },
  "export.ready": { bytes: 10 },
  "account.email_change_requested": {},
  "account.email_changed": {},
  "account.reactivated": {},
  "account.google_linked": { email: "j•••@gmail.com" },
  "account.google_unlinked": { email: "j•••@gmail.com" },
  "subscription.renewed": { period_end: "2026-12-01T00:00:00Z" },
  "subscription.payment_failed": { reason: "declined", retry: true },
  "subscription.canceled": { reason: "period_ended" },
  // revisions 5.0 (§24.4)
  "refund_request.received": { request_id: "r1", reference: "RR-AB12CD34" },
  "refund_request.resolved": { request_id: "r1", reference: "RR-AB12CD34", status: "approved", refunded_cents: 1000, currency: "USD" },
  // revisions 5.1 (§25.5)
  "payment.refunded": { amount_cents: 2667, currency: "USD", transaction_id: "t1" },
  "subscription.started": { plan_name: "Premium", amount_cents: 999, currency: "USD" },
};

describe("copy for every kind (§23.6)", () => {
  it("has a sample for exactly the listed kinds (the list is checked against the NotificationKind union at compile time)", () => {
    expect(new Set(NOTIFICATION_KINDS)).toEqual(new Set(Object.keys(samples)));
    expect(NOTIFICATION_KINDS.length).toBe(new Set(NOTIFICATION_KINDS).size);
  });

  it.each([...NOTIFICATION_KINDS].map((k) => [k]))("%s has its own title and body in every language", (kind) => {
    for (const [locale, text] of [["en", textEn], ["fr", textFr], ["es", textEs]] as const) {
      const copy = notificationText(n(kind, samples[kind]), locale, text);
      expect(copy.title, `${kind} ${locale} title`).toBeTruthy();
      expect(copy.title, `${kind} ${locale} must not use the generic title`).not.toBe(text(notificationTemplates.fallbackTitle));
      expect(copy.body, `${kind} ${locale} body`).toBeTruthy();
      expect(`${copy.title} ${copy.body}`).not.toMatch(/\{\w+\}/);
    }
    // French and Spanish are translated, not English.
    const en2 = notificationText(n(kind, samples[kind]), "en", textEn);
    if (kind !== "broadcast") {
      expect(notificationText(n(kind, samples[kind]), "fr", textFr).title).not.toBe(en2.title);
      expect(notificationText(n(kind, samples[kind]), "es", textEs).title).not.toBe(en2.title);
    }
  });

  it("renders the new kinds with their data", () => {
    expect(notificationText(n("waitlist.available", { psychic_name: "Theo" }), "en", textEn)).toEqual({
      title: "Your advisor is online",
      body: "Theo is online now. Start your reading while they are available.",
    });
    expect(notificationText(n("waitlist.available", {}), "en", textEn).body).toBe("An advisor you waited for is online now.");
    expect(notificationText(n("account.google_linked", { email: "j•••@gmail.com" }), "en", textEn).body).toBe("You can now sign in with the Google account j•••@gmail.com.");
    expect(notificationText(n("account.google_linked", {}), "en", textEn).body).toBe("You can now sign in with Google.");
    expect(notificationText(n("account.google_unlinked", { email: "j•••@gmail.com" }), "fr", textFr).body).toBe("Le compte Google j•••@gmail.com ne permet plus de se connecter.");
    expect(notificationText(n("account.google_unlinked", {}), "es", textEs).body).toBe("Se ha eliminado el inicio de sesión con Google de tu cuenta.");
  });

  it("explains why a membership ended and when a failed payment can still be fixed", () => {
    const reasons = ["period_ended", "payment_failed", "canceled_by_user", "admin", "dispute", "account_closed"];
    const bodies = reasons.map((reason) => notificationText(n("subscription.canceled", { reason }), "en", textEn).body);
    expect(new Set(bodies).size).toBe(reasons.length);
    expect(notificationText(n("subscription.canceled", { reason: "something-new" }), "en", textEn).body).toBe(subscriptionCanceledCopy.other);
    const date = new Intl.DateTimeFormat("en", { dateStyle: "medium" }).format(Date.parse("2026-12-05T00:00:00Z"));
    expect(notificationText(n("subscription.payment_failed", { grace_until: "2026-12-05T00:00:00Z" }), "en", textEn).body).toBe(
      `We couldn't charge your payment method. Update it by ${date} to keep your benefits.`,
    );
    expect(notificationText(n("subscription.payment_failed", { grace_until: "garbage" }), "en", textEn).body).toBe(notificationTemplates.subscriptionPaymentFailedBody);
  });

  it("still reports a membership's end while memberships are off, but never invites a new one (§25.1)", () => {
    for (const reason of ["period_ended", "payment_failed", "canceled_by_user", "admin", "dispute", "account_closed", "something-new"]) {
      const on = notificationText(n("subscription.canceled", { reason }), "en", textEn).body;
      const off = notificationText(n("subscription.canceled", { reason }), "en", textEn, { memberships: false }).body;
      expect(off).toBeTruthy();
      expect(off).not.toMatch(/subscribe/i);
      expect(off.length).toBeLessThanOrEqual(on.length);
    }
    expect(notificationText(n("subscription.canceled", { reason: "period_ended" }), "en", textEn, { memberships: false }).body).toBe("Your membership has ended.");
    expect(notificationText(n("subscription.canceled", { reason: "period_ended" }), "en", textEn, { memberships: true }).body).toContain("You can subscribe again");
    // The default keeps the copy people already knew.
    expect(notificationText(n("subscription.canceled", { reason: "payment_failed" }), "en", textEn).body).toBe(subscriptionCanceledCopy.payment_failed);
    expect(notificationText(n("subscription.canceled", { reason: "canceled_by_user" }), "fr", textFr, { memberships: false }).body).toBe("Vous avez annulé votre abonnement et il est maintenant terminé.");
  });

  it("falls back to a sensible link when the API sent none, and never to an unsafe one", () => {
    expect(notificationDestination(n("waitlist.available", { psychic_slug: "the o" }, ""), "fr")).toBe("/fr/chatroom?reader=the%20o");
    expect(notificationDestination(n("subscription.canceled", {}, ""), "es")).toBe("/es/profile/settings");
    expect(notificationDestination(n("export.ready", {}, ""), "en")).toBe("/en/profile/settings#account-actions");
    expect(notificationDestination(n("account.google_linked", {}, ""), "en")).toBe("/en/profile/settings#security");
    // The API's own link wins; an unsafe one is replaced by the kind's default or nothing.
    expect(notificationDestination(n("waitlist.available", { psychic_slug: "theo" }, "/chatroom?reader=theo"), "en")).toBe("/en/chatroom?reader=theo");
    expect(notificationDestination(n("export.ready", {}, "https://evil.example"), "en")).toBe("/en/profile/settings#account-actions");
    expect(notificationDestination(n("photo.approved", {}, "//evil.example"), "en")).toBeNull();
    expect(notificationFallbackLink(n("broadcast", {}, ""))).toBeNull();
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
    ...Object.values(subscriptionCanceledCopy),
    ...Object.values(subscriptionCanceledPlainCopy),
    ...Object.values(refundResolvedCopy),
    ...Object.values(attachmentProblemCopy),
    errorCopy.step_up_required,
    errorCopy.invalid_code,
  ];
  it.each(all.map((s) => [s]))("%s is translated in every locale", (key) => {
    for (const dict of [en, fr, es] as Record<string, string>[]) expect(dict[key], key).toBeTruthy();
  });
});

// revisions 5.1 (§25.5): refunds issued by support or the provider.
describe("payment.refunded", () => {
  const english = createTranslateCopy(en);
  const refunded = (data: Record<string, unknown>) =>
    ({ id: "n1", kind: "payment.refunded", data, read: false, created_at: "2026-11-13T10:00:00Z" }) as unknown as AppNotification;

  it("states the amount in the refund's currency and the bank's posting time", () => {
    const copy = notificationText(refunded({ amount_cents: 2667, currency: "usd" }), "en", english);
    expect(copy.title).toBe("Refund issued");
    expect(copy.body).toBe("$26.67 was refunded to your payment method. Your bank may take 5–10 business days to show it.");
  });

  it("still reads well without an amount", () => {
    expect(notificationText(refunded({}), "en", english).body).toBe(
      "A refund was issued to your payment method. Your bank may take 5–10 business days to show it.",
    );
  });
});
