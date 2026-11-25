import { describe, expect, it } from "vitest";

import type { User } from "@/lib/api/types";
import { shouldAnnounceVerified, withEmailVerified } from "@/lib/auth/email-verified";

const user = (over: Partial<User> = {}) => ({ id: "u1", email: "a@b.c", ...over }) as User;

describe("withEmailVerified", () => {
  it("marks the user verified and keeps every other field", () => {
    const u = user({ email_verified: false, display_name: "A" });
    expect(withEmailVerified(u)).toEqual({ ...u, email_verified: true });
  });

  it("returns the same object when nothing changes (no re-render) and tolerates no user", () => {
    const u = user({ email_verified: true });
    expect(withEmailVerified(u)).toBe(u);
    expect(withEmailVerified(null)).toBeNull();
  });
});

describe("shouldAnnounceVerified", () => {
  const ev = { email_verified: true as const };
  it("announces the first change for the signed-in user", () => {
    expect(shouldAnnounceVerified(ev, user({ email_verified: false }), "/en/profile")).toBe(true);
    expect(shouldAnnounceVerified(ev, user(), "/en/profile/settings")).toBe(true);
  });
  it("stays quiet when already verified, without a user, for a malformed event or on the verify page", () => {
    expect(shouldAnnounceVerified(ev, user({ email_verified: true }), "/en/profile")).toBe(false);
    expect(shouldAnnounceVerified(ev, null, "/en/profile")).toBe(false);
    expect(shouldAnnounceVerified({}, user(), "/en/profile")).toBe(false);
    expect(shouldAnnounceVerified(undefined, user(), "/en/profile")).toBe(false);
    expect(shouldAnnounceVerified(ev, user({ email_verified: false }), "/fr/verify-email")).toBe(false);
  });
});
