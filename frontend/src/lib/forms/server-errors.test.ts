import { describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api/errors";
import { applyApiError } from "@/lib/forms/server-errors";

const text = <T,>(value: T): T => (typeof value === "string" ? (`[t] ${value}` as T) : value);

type FormLike = Parameters<typeof applyApiError>[0];

function fakeForm(values: Record<string, unknown>) {
  const form = { setError: vi.fn(), getValues: vi.fn(() => values), setFocus: vi.fn() };
  return form as typeof form & FormLike;
}

describe("applyApiError", () => {
  it("puts localized field messages on matching fields and focuses the first", () => {
    const form = fakeForm({ display_name: "", birth: { day: "" } });
    const error = new ApiError({
      code: "validation_failed",
      message: "invalid",
      status: 422,
      details: { fields: { display_name: "Too long.", birth_date: "Invalid date." } },
    });
    const result = applyApiError(form, error, { text, fieldMap: { birth_date: "birth" } });
    expect(result.code).toBe("validation_failed");
    expect(form.setError).toHaveBeenCalledWith("display_name", { type: "server", message: "Too long." });
    expect(form.setError).toHaveBeenCalledWith("birth", { type: "server", message: "Invalid date." });
    expect(form.setError).not.toHaveBeenCalledWith("root.server", expect.anything());
    expect(form.setFocus).toHaveBeenCalledWith("display_name");
  });

  it("falls back to a form-level error for fields the form does not have", () => {
    const form = fakeForm({ email: "" });
    const error = new ApiError({ code: "validation_failed", message: "x", status: 422, details: { fields: { token: "Bad." } } });
    applyApiError(form, error, { text });
    expect(form.setError).toHaveBeenCalledWith("root.server", {
      type: "validation_failed",
      message: "[t] Some fields need attention.",
    });
    expect(form.setFocus).not.toHaveBeenCalled();
  });

  it("maps any other failure to the translated copy for its code", () => {
    const form = fakeForm({ email: "" });
    applyApiError(form, new ApiError({ code: "email_taken", message: "dup", status: 409 }), { text });
    expect(form.setError).toHaveBeenCalledWith("root.server", {
      type: "email_taken",
      message: "[t] An account with this email already exists.",
    });
    applyApiError(form, new TypeError("Failed to fetch"), { text });
    expect(form.setError).toHaveBeenLastCalledWith("root.server", {
      type: "network_error",
      message: "[t] You appear to be offline. Check your connection and try again.",
    });
  });

  it("ignores aborted requests", () => {
    const form = fakeForm({});
    applyApiError(form, new DOMException("aborted", "AbortError"), { text });
    expect(form.setError).not.toHaveBeenCalled();
  });
});
