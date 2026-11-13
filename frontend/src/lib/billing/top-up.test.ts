import { describe, expect, it } from "vitest";

import { createTranslateCopy } from "@/i18n/translate-copy";
import en from "@/i18n/dictionaries/copy-en.json";
import fr from "@/i18n/dictionaries/copy-fr.json";
import es from "@/i18n/dictionaries/copy-es.json";
import { ApiError } from "@/lib/api/errors";
import type { PricingPack } from "@/lib/api/pricing";
import { declineMessage } from "@/components/account/settings/billing/billing-format";
import {
  OFFER_UNAVAILABLE_COPY,
  advisorEstimate,
  classifyPurchaseFailure,
  defaultPack,
  minutesRange,
  offerUnavailableMessage,
  packEyebrow,
  standardPacks,
} from "@/lib/billing/top-up";

const text = createTranslateCopy(en);
const pack = (code: string, over: Partial<PricingPack> = {}): PricingPack => ({
  id: `id_${code}`, code, kind: "standard", name: code, tagline: "", label: "", credits: 100, price_cents: 1000, currency: "USD",
  featured: false, new_clients_only: false, sort_order: 1, badges: [], ...over,
});
const welcome = pack("welcome_600", { kind: "intro", credits: 600, badges: [{ type: "new_client_offer" }] });
const quick = pack("quick_150", { credits: 150, label: "One focused question" });
const starter = pack("starter_315", { credits: 315, badges: [{ type: "save", percent: 4 }] });
const err = (code: string, status: number, details?: Record<string, unknown>) => new ApiError({ code, message: code, status, details });

describe("defaultPack", () => {
  it("preselects the eligible Welcome Pack", () => {
    expect(defaultPack({ packs: [welcome, quick, starter], welcome_offer: { eligible: true, pack_id: welcome.id } })?.code).toBe("welcome_600");
  });

  it("is the first standard pack once the offer is redeemed (it is absent, never disabled)", () => {
    expect(defaultPack({ packs: [quick, starter], welcome_offer: { eligible: false } })?.code).toBe("quick_150");
  });

  it("skips the offer after the API refused it", () => {
    expect(defaultPack({ packs: [welcome, quick, starter], welcome_offer: { eligible: true, pack_id: welcome.id } }, true)?.code).toBe("quick_150");
  });

  it("is null for an empty catalog", () => {
    expect(defaultPack({ packs: [], welcome_offer: { eligible: false } })).toBeNull();
  });

  it("keeps the standard packs in the API's order", () => {
    expect(standardPacks([welcome, quick, starter]).map((p) => p.code)).toEqual(["quick_150", "starter_315"]);
  });
});

describe("packEyebrow", () => {
  it("joins the badges, else the pack's label, else 'One-time pack'", () => {
    expect(packEyebrow({ badges: [{ type: "most_popular" }, { type: "save", percent: 11 }], label: "" }, text)).toBe("Most popular · Save 11%");
    expect(packEyebrow(welcome, text)).toBe("New client offer");
    expect(packEyebrow({ badges: [{ type: "best_value" }, { type: "save", percent: 21 }], label: "x" }, text)).toBe("Best value · Save 21%");
    expect(packEyebrow(quick, text)).toBe("One focused question");
    expect(packEyebrow({ badges: [], label: "" }, text)).toBe("One-time pack");
  });

  it("is translated", () => {
    expect(packEyebrow({ badges: [{ type: "save", percent: 4 }], label: "" }, createTranslateCopy(fr))).toBe("Économisez 4 %");
    expect(packEyebrow({ badges: [], label: "" }, createTranslateCopy(es))).toBe("Paquete único");
  });
});

describe("minutes (§24.7)", () => {
  it("shows both ends of the advisor rate range", () => {
    expect(minutesRange(315, { min: 30, max: 60 }, "en", text)).toBe("≈ 10.5 min. at 30 credits/min. · ≈ 5.25 min. at 60 credits/min.");
    expect(minutesRange(600, { min: 30, max: 60 }, "en", text)).toBe("≈ 20 min. at 30 credits/min. · ≈ 10 min. at 60 credits/min.");
  });

  it("does not repeat a single rate", () => {
    expect(minutesRange(600, { min: 40, max: 40 }, "en", text)).toBe("≈ 15 min. at 40 credits/min.");
  });

  it("states the minutes at the live advisor's rate, with at most two decimals", () => {
    expect(advisorEstimate(600, { advisorName: "Solomon", ratePerMinute: 45 }, "en", text)).toBe(
      "At Solomon's rate of 45 credits/minute, this pack provides approximately 13.33 minutes.",
    );
    expect(advisorEstimate(675, { advisorName: "Theo", ratePerMinute: 30 }, "en", text)).toContain("approximately 22.5 minutes");
  });

  it("says nothing without an advisor or a usable rate", () => {
    expect(advisorEstimate(600, undefined, "en", text)).toBeNull();
    expect(advisorEstimate(600, { advisorName: "Theo" }, "en", text)).toBeNull();
    expect(advisorEstimate(600, { advisorName: "  ", ratePerMinute: 30 }, "en", text)).toBeNull();
    expect(advisorEstimate(600, { advisorName: "Theo", ratePerMinute: 0 }, "en", text)).toBeNull();
  });
});

describe("classifyPurchaseFailure", () => {
  it("tells a decline with the reason's copy", () => {
    expect(classifyPurchaseFailure(err("payment_failed", 402, { reason: "expired_card" }), declineMessage)).toEqual({ kind: "declined", message: "This card has expired. Use another card." });
    expect(classifyPurchaseFailure(err("payment_failed", 402), declineMessage)).toEqual({ kind: "declined", message: "Your payment was declined. Try another card." });
  });

  it("gives every offer_unavailable reason its copy", () => {
    for (const reason of Object.keys(OFFER_UNAVAILABLE_COPY)) {
      const failure = classifyPurchaseFailure(err("offer_unavailable", 409, { reason }), declineMessage);
      expect(failure).toEqual({ kind: "offer", message: OFFER_UNAVAILABLE_COPY[reason as keyof typeof OFFER_UNAVAILABLE_COPY] });
    }
    expect(offerUnavailableMessage(err("offer_unavailable", 409, { reason: "something_new" }))).toBe(OFFER_UNAVAILABLE_COPY.no_offer);
    expect(offerUnavailableMessage(err("offer_unavailable", 409))).toBe(OFFER_UNAVAILABLE_COPY.no_offer);
  });

  it("retries with the same key when the outcome is unknown (network, timeout, 5xx)", () => {
    for (const e of [err("network_error", 0), err("timeout", 0), err("service_unavailable", 503), err("internal_error", 500)]) {
      const failure = classifyPurchaseFailure(e, declineMessage);
      expect(failure.kind).toBe("retry");
    }
  });

  it("does not offer a retry when purchases are switched off", () => {
    expect(classifyPurchaseFailure(err("feature_unavailable", 503), declineMessage).kind).toBe("unavailable");
  });

  it("leaves the age gate's answers to the global handler", () => {
    expect(classifyPurchaseFailure(err("birth_date_required", 403), declineMessage)).toEqual({ kind: "age_gate" });
    expect(classifyPurchaseFailure(err("age_restricted", 403), declineMessage)).toEqual({ kind: "age_gate" });
  });

  it("falls back to the error's own copy", () => {
    expect(classifyPurchaseFailure(err("validation_failed", 422), declineMessage)).toEqual({ kind: "error", message: "Some fields need attention." });
  });

  it("has copy for every message it can return, in every language", () => {
    const messages = [
      ...Object.values(OFFER_UNAVAILABLE_COPY),
      (classifyPurchaseFailure(err("network_error", 0), declineMessage) as { message: string }).message,
      (classifyPurchaseFailure(err("feature_unavailable", 503), declineMessage) as { message: string }).message,
    ];
    for (const message of messages) for (const dict of [en, fr, es] as Record<string, string>[]) expect(dict[message], message).toBeTruthy();
  });
});
