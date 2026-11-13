import { describe, expect, it, vi } from "vitest";

import { ageBlockOf, notifyAgeBlocked, onAgeBlocked, reportAgeBlock } from "@/lib/api/age-events";
import { ApiError } from "@/lib/api/errors";

// revisions 5.0: 18+ age gate (docs §24.6).
describe("age-gate events", () => {
  it("maps the two gate codes and nothing else", () => {
    expect(ageBlockOf({ code: "birth_date_required" })).toBe("birth_date_required");
    expect(ageBlockOf({ code: "age_restricted" })).toBe("restricted");
    expect(ageBlockOf({ code: "forbidden" })).toBeNull();
    expect(ageBlockOf({ code: "csrf_failed" })).toBeNull();
  });

  it("tells listeners about gate answers only, and stops after unsubscribing", () => {
    const seen = vi.fn();
    const off = onAgeBlocked(seen);
    notifyAgeBlocked(new ApiError({ code: "forbidden", message: "x", status: 403 }));
    expect(seen).not.toHaveBeenCalled();
    const error = new ApiError({ code: "birth_date_required", message: "x", status: 403, details: { next: "/signup/birth" } });
    notifyAgeBlocked(error);
    expect(seen).toHaveBeenCalledWith("birth_date_required", error);
    notifyAgeBlocked(new ApiError({ code: "age_restricted", message: "x", status: 403 }));
    expect(seen).toHaveBeenLastCalledWith("restricted", expect.any(ApiError));
    reportAgeBlock("restricted");
    expect(seen).toHaveBeenLastCalledWith("restricted");
    off();
    notifyAgeBlocked(error);
    expect(seen).toHaveBeenCalledTimes(3);
  });
});
