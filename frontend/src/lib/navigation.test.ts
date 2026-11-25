import { describe, expect, it } from "vitest";

import { keepNext, safeNext, signupUrl } from "@/lib/navigation";

describe("safeNext", () => {
  it("accepts same-locale relative paths", () => {
    expect(safeNext("?next=%2Fen%2Fchatroom%3Freader%3Dramone", "en")).toBe("/en/chatroom?reader=ramone");
    expect(safeNext("?next=/en", "en")).toBe("/en");
  });

  it.each([
    ["absolute URL", "?next=https://evil.example/en/x"],
    ["protocol-relative", "?next=//evil.example/en"],
    ["backslash trick", "?next=/en\\@evil.example"],
    ["other locale", "?next=/fr/profile"],
    ["control characters", "?next=/en/%0d%0aSet-Cookie"],
    ["javascript scheme", "?next=javascript:alert(1)"],
    ["missing", ""],
  ])("rejects %s", (_, search) => {
    expect(safeNext(search, "en")).toBeNull();
  });
});

describe("signupUrl and keepNext", () => {
  it("builds the sign-up URL that continues to a path", () => {
    expect(signupUrl("en")).toBe("/en/signup");
    expect(signupUrl("fr", "/fr/chatroom?reader=theo")).toBe("/fr/signup?next=%2Ffr%2Fchatroom%3Freader%3Dtheo");
  });

  it("carries a safe next to the following step and nothing else", () => {
    const next = encodeURIComponent("/en/chatroom?reader=theo");
    expect(keepNext("/en/signup/password", `?next=${next}`, "en")).toBe(`/en/signup/password?next=${next}`);
    expect(keepNext("/en/signup/password", "?utm_source=x", "en")).toBe("/en/signup/password");
    expect(keepNext("/en/signup/password", "?next=https://evil.example/en", "en")).toBe("/en/signup/password");
    expect(keepNext("/en/signup/password", `?next=${encodeURIComponent("/fr/x")}`, "en")).toBe("/en/signup/password");
  });
});
