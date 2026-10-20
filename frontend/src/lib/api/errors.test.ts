import { describe, expect, it } from "vitest";

import { ApiError, errorCopy, errorMessage, toApiError } from "@/lib/api/errors";

describe("ApiError", () => {
  it("normalizes thrown values", () => {
    expect(toApiError(new TypeError("Failed to fetch")).code).toBe("network_error");
    expect(toApiError(new DOMException("x", "AbortError")).code).toBe("aborted");
    expect(toApiError("weird").code).toBe("internal_error");
    const e = new ApiError({ code: "conflict", message: "x" });
    expect(toApiError(e)).toBe(e);
  });

  it("exposes field errors and retryability", () => {
    const e = new ApiError({ code: "validation_failed", message: "x", status: 422, details: { fields: { email: "Enter a valid email address." } } });
    expect(e.fields.email).toBe("Enter a valid email address.");
    expect(e.retryable).toBe(false);
    expect(new ApiError({ code: "service_unavailable", message: "", status: 503 }).retryable).toBe(true);
    expect(new ApiError({ code: "rate_limited", message: "", status: 429 }).retryable).toBe(true);
  });

  it("has user copy for every error code and falls back safely", () => {
    expect(errorMessage(new ApiError({ code: "insufficient_credits", message: "server text" }))).toBe(errorCopy.insufficient_credits);
    expect(errorMessage(new ApiError({ code: "something_new", message: "x" }))).toBe(errorCopy.internal_error);
  });
});
