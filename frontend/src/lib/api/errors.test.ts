// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError, errorCopy, errorMessage, toApiError, unreachableCopy } from "@/lib/api/errors";
import copyEn from "@/i18n/dictionaries/copy-en.json";
import copyEs from "@/i18n/dictionaries/copy-es.json";
import copyFr from "@/i18n/dictionaries/copy-fr.json";

describe("ApiError", () => {
  it("normalizes thrown values", () => {
    expect(toApiError(new TypeError("Failed to fetch")).code).toBe("network_error");
    expect(toApiError(new DOMException("x", "AbortError")).code).toBe("aborted");
    expect(toApiError("weird").code).toBe("client_error");
    const e = new ApiError({ code: "conflict", message: "x" });
    expect(toApiError(e)).toBe(e);
  });

  it("exposes field errors and retryability", () => {
    const e = new ApiError({ code: "validation_failed", message: "x", status: 422, details: { fields: { email: "Enter a valid email address." } } });
    expect(e.fields.email).toBe("Enter a valid email address.");
    expect(e.retryable).toBe(false);
    expect(new ApiError({ code: "service_unavailable", message: "", status: 503 }).retryable).toBe(true);
    expect(new ApiError({ code: "feature_unavailable", message: "", status: 503 }).retryable).toBe(false);
    expect(new ApiError({ code: "rate_limited", message: "", status: 429 }).retryable).toBe(true);
  });

  it("has user copy for every error code and falls back safely", () => {
    expect(errorMessage(new ApiError({ code: "insufficient_credits", message: "server text" }))).toBe(errorCopy.insufficient_credits);
    expect(errorMessage(new ApiError({ code: "something_new", message: "x" }))).toBe(errorCopy.internal_error);
  });
});

describe("revisions 5.0 codes (§24.10)", () => {
  it.each([
    ["offer_unavailable", "The Welcome Pack is no longer available. Choose another pack to continue."],
    ["refund_request_exists", "You already have an open refund request for this payment."],
    ["refund_not_eligible", "This payment can't be refunded from here. Contact support if you think this is a mistake."],
  ])("%s has its own copy", (code, copy) => {
    expect(errorMessage(new ApiError({ code, message: "server text", status: 409 }))).toBe(copy);
  });

  it("none of them is retryable: the same request would fail the same way", () => {
    for (const code of ["offer_unavailable", "refund_request_exists", "refund_not_eligible"]) expect(new ApiError({ code, message: "x", status: 409 }).retryable).toBe(false);
  });
});

// revisions 5.0: 18+ age gate (docs §24.6).
describe("age-gate error codes", () => {
  it("have their own copy, not the generic fallback", () => {
    const required = errorMessage(new ApiError({ code: "birth_date_required", message: "x", status: 403 }));
    const restricted = errorMessage(new ApiError({ code: "age_restricted", message: "x", status: 403 }));
    expect(required).toBe("Enter your date of birth to continue. Staryield is for adults 18 and over.");
    expect(restricted).toContain("restricted");
    expect(restricted).toContain("Contact support");
    expect(required).not.toBe(errorCopy.internal_error);
    expect(restricted).not.toBe(errorCopy.internal_error);
  });

  it("are translated into every language", () => {
    for (const code of ["birth_date_required", "age_restricted"]) {
      const english = errorCopy[code];
      for (const copy of [copyEn, copyFr, copyEs] as Record<string, string>[]) expect(copy[english]).toBeTruthy();
      expect((copyFr as Record<string, string>)[english]).not.toBe(english);
      expect((copyEs as Record<string, string>)[english]).not.toBe(english);
    }
  });

  it("are not retryable and keep the API's next step", () => {
    const e = new ApiError({ code: "birth_date_required", message: "x", status: 403, details: { next: "/signup/birth" } });
    expect(e.retryable).toBe(false);
    expect(e.details?.next).toBe("/signup/birth");
    expect(new ApiError({ code: "age_restricted", message: "x", status: 403 }).retryable).toBe(false);
  });
});

describe("toApiError classification (docs §22.2)", () => {
  afterEach(() => vi.restoreAllMocks());

  it.each(["Failed to fetch", "Load failed", "NetworkError when attempting to fetch resource.", "Network request failed"])(
    "treats the fetch failure %j as network_error",
    (message) => {
      const e = toApiError(new TypeError(message));
      expect(e.code).toBe("network_error");
      expect(e.retryable).toBe(true);
    },
  );

  it("does not blame the network for programming errors", () => {
    for (const error of [new TypeError("Cannot read properties of undefined (reading 'x')"), new RangeError("bad"), new Error("boom"), "weird", null]) {
      const e = toApiError(error);
      expect(e.code).toBe("client_error");
      expect(e.retryable).toBe(false);
      expect(errorMessage(e)).toBe("Something went wrong on this page. Please try again.");
    }
  });

  it("logs an unexpected error once", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);
    const bug = new Error("only once");
    toApiError(bug);
    toApiError(bug);
    expect(warn.mock.calls.filter((c) => String(c[0]).includes("unexpected client error"))).toHaveLength(1);
  });

  it("words network_error by navigator.onLine", () => {
    const online = vi.spyOn(window.navigator, "onLine", "get");
    const e = new ApiError({ code: "network_error", message: "x" });
    online.mockReturnValue(false);
    expect(errorMessage(e)).toBe("You appear to be offline. Check your connection and try again.");
    online.mockReturnValue(true);
    expect(errorMessage(e)).toBe(unreachableCopy);
    expect(unreachableCopy).toBe("We couldn't reach Staryield. Please try again in a moment.");
  });
});
