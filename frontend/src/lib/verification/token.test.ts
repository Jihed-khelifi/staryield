// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";

import { takeVerificationToken, tokenFromHash } from "@/lib/verification/token";

afterEach(() => window.history.replaceState(null, "", "/en"));

describe("verification token in the URL fragment", () => {
  it("reads the token from the hash", () => {
    expect(tokenFromHash("#token=abc_DEF-123")).toBe("abc_DEF-123");
    expect(tokenFromHash("token=abc")).toBe("abc");
  });

  it("rejects missing, empty and oversized tokens", () => {
    expect(tokenFromHash("")).toBeNull();
    expect(tokenFromHash("#other=1")).toBeNull();
    expect(tokenFromHash("#token=")).toBeNull();
    expect(tokenFromHash(`#token=${"a".repeat(129)}`)).toBeNull();
    expect(tokenFromHash(`#token=${"a".repeat(128)}`)).toHaveLength(128);
  });

  it("takes the token once and strips the fragment, keeping the path and query", () => {
    window.history.replaceState(null, "", "/en/verify-email?utm=x#token=secret");
    expect(takeVerificationToken()).toBe("secret");
    expect(window.location.hash).toBe("");
    expect(window.location.pathname + window.location.search).toBe("/en/verify-email?utm=x");
    expect(takeVerificationToken()).toBeNull();
  });

  it("strips a fragment that is not a token too", () => {
    window.history.replaceState(null, "", "/en/verify-email#nothing");
    expect(takeVerificationToken()).toBeNull();
    expect(window.location.hash).toBe("");
  });
});
