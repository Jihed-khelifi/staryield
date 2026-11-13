import { describe, expect, it } from "vitest";

import en from "@/i18n/dictionaries/copy-en.json";
import fr from "@/i18n/dictionaries/copy-fr.json";
import es from "@/i18n/dictionaries/copy-es.json";
import { createTranslateCopy } from "@/i18n/translate-copy";
import { ApiError } from "@/lib/api/errors";
import type { TransactionRefundInfo } from "@/lib/api/pricing";
import {
  REFUND_NOT_ELIGIBLE_COPY,
  REFUND_REASONS,
  REFUND_REASON_COPY,
  REFUND_STATUS_COPY,
  classifyRefundFailure,
  refundMailto,
  refundRowState,
  refundSchema,
} from "@/lib/billing/refund";

const err = (code: string, status: number, details?: Record<string, unknown>) => new ApiError({ code, message: code, status, details });
const base: TransactionRefundInfo = { refundable_cents: 3200, credits_remaining: 250, window_ends_at: "2026-11-19T10:00:00Z", can_request: true };

describe("refundRowState", () => {
  it("offers a request when one can be sent", () => {
    expect(refundRowState(base)).toEqual({ kind: "request" });
  });

  it("shows nothing for other payments or when nothing can be requested", () => {
    expect(refundRowState(undefined)).toEqual({ kind: "none" });
    expect(refundRowState({ ...base, can_request: false })).toEqual({ kind: "none" });
  });

  it("shows an open request's status and reference, withdrawable", () => {
    expect(refundRowState({ ...base, can_request: false, request: { id: "r1", reference: "RR-AB12CD34", status: "open" } })).toEqual({
      kind: "status", status: "open", reference: "RR-AB12CD34", id: "r1", withdrawable: true, canRequestAgain: false,
    });
  });

  it("shows a decided request without Withdraw, and a new request only when the API allows one", () => {
    const denied = refundRowState({ ...base, can_request: false, request: { id: "r1", reference: "RR-AB12CD34", status: "denied" } });
    expect(denied).toMatchObject({ kind: "status", status: "denied", withdrawable: false, canRequestAgain: false });
    const withdrawn = refundRowState({ ...base, can_request: true, request: { id: "r1", reference: "RR-AB12CD34", status: "withdrawn" } });
    expect(withdrawn).toMatchObject({ kind: "status", status: "withdrawn", withdrawable: false, canRequestAgain: true });
  });
});

describe("classifyRefundFailure", () => {
  it("returns the existing request on refund_request_exists", () => {
    expect(classifyRefundFailure(err("refund_request_exists", 409, { request_id: "r9" }))).toEqual({ kind: "exists", requestId: "r9" });
    expect(classifyRefundFailure(err("refund_request_exists", 409))).toEqual({ kind: "exists", requestId: undefined });
  });

  it("explains every refund_not_eligible reason", () => {
    for (const reason of ["not_credit_purchase", "not_paid", "fully_refunded", "outside_window"]) {
      expect(classifyRefundFailure(err("refund_not_eligible", 409, { reason }))).toEqual({ kind: "not_eligible", reason, message: REFUND_NOT_ELIGIBLE_COPY[reason] });
    }
    expect(classifyRefundFailure(err("refund_not_eligible", 409, { reason: "new_reason" }))).toMatchObject({ kind: "not_eligible", message: expect.any(String) });
  });

  it("offers the e-mail channel when in-app requests are off", () => {
    expect(classifyRefundFailure(err("feature_unavailable", 503))).toEqual({ kind: "email" });
    expect(refundMailto()).toBe("mailto:support@staryield.net?subject=Refund%20Request");
  });

  it("tells a rate limit, and marks transient failures retryable", () => {
    expect(classifyRefundFailure(err("rate_limited", 429)).kind).toBe("rate_limited");
    expect(classifyRefundFailure(err("network_error", 0))).toMatchObject({ kind: "error", retryable: true });
    expect(classifyRefundFailure(err("validation_failed", 422))).toMatchObject({ kind: "error", retryable: false });
  });
});

describe("refundSchema", () => {
  const schema = refundSchema(createTranslateCopy(en));

  it("needs one of the six reasons", async () => {
    expect(REFUND_REASONS).toHaveLength(6);
    await expect(schema.validate({ details: "" })).rejects.toThrow("Choose a reason.");
    await expect(schema.validate({ reason: "refund_me", details: "" })).rejects.toThrow("Choose a reason.");
    await expect(schema.validate({ reason: "unused_credits", details: "" })).resolves.toMatchObject({ reason: "unused_credits" });
  });

  it("limits the details to 2000 characters", async () => {
    await expect(schema.validate({ reason: "other", details: "x".repeat(2000) })).resolves.toBeTruthy();
    await expect(schema.validate({ reason: "other", details: "x".repeat(2001) })).rejects.toThrow("Use at most 2000 characters.");
  });
});

describe("refund copy", () => {
  it("is translated in every language", () => {
    const all = [...Object.values(REFUND_REASON_COPY), ...Object.values(REFUND_STATUS_COPY), ...Object.values(REFUND_NOT_ELIGIBLE_COPY)];
    for (const message of all) for (const dict of [en, fr, es] as Record<string, string>[]) expect(dict[message], message).toBeTruthy();
    expect(createTranslateCopy(fr)(REFUND_STATUS_COPY.open)).toBe("Remboursement demandé");
  });
});
