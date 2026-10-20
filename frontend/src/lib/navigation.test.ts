import { describe, expect, it } from "vitest";

import { safeNext } from "@/lib/navigation";

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
