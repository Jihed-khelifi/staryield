import { describe, expect, it } from "vitest";

import { isBenignBrowserError } from "@/components/feedback/error-reporter";

describe("isBenignBrowserError", () => {
  it("ignores ResizeObserver frame notices", () => {
    expect(isBenignBrowserError("ResizeObserver loop completed with undelivered notifications.")).toBe(true);
    expect(isBenignBrowserError("ResizeObserver loop limit exceeded")).toBe(true);
  });
  it("keeps real errors", () => {
    expect(isBenignBrowserError("TypeError: x is undefined")).toBe(false);
    expect(isBenignBrowserError(undefined)).toBe(false);
    expect(isBenignBrowserError("Something about ResizeObserver loop")).toBe(false);
  });
});
