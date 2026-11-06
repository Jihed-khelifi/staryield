import { describe, expect, it } from "vitest";

// The R2 asset pipeline (scripts/assets.mjs) exports pure helpers.
import { assetKey, mergeRegistry, planEntry } from "../../scripts/assets.mjs";

const entry = (key: string, sha256 = "a".repeat(64), bytes = 10) => ({
  key,
  url: `https://assets.staryield.net/${key}`,
  contentType: "image/svg+xml",
  bytes,
  sha256,
});

describe("assets pipeline helpers", () => {
  it("builds plain and content-hashed keys", () => {
    expect(assetKey({ prefix: "assets", path: "social/x.svg", sha256: "0123456789abcdef" })).toBe("assets/social/x.svg");
    expect(assetKey({ prefix: "/assets/", path: "social\\x.svg", sha256: "0123456789abcdef", hashed: true })).toBe(
      "assets/social/x-0123456789.svg",
    );
    expect(assetKey({ path: "favicon.ico", sha256: "ffff" })).toBe("favicon.ico");
  });

  it("never overwrites a registered key with different content", () => {
    const registry = [entry("assets/a.svg")];
    expect(planEntry(registry, entry("assets/a.svg"))).toBe("skip");
    expect(planEntry(registry, entry("assets/a.svg", "b".repeat(64)))).toBe("conflict");
    expect(planEntry(registry, entry("assets/b.svg"))).toBe("upload");
  });

  it("merges into a key-sorted registry", () => {
    const merged = mergeRegistry([entry("assets/c.svg"), entry("assets/a.svg")], [entry("assets/b.svg"), entry("assets/a.svg", "c".repeat(64))]);
    expect(merged.map((item) => item.key)).toEqual(["assets/a.svg", "assets/b.svg", "assets/c.svg"]);
    expect(merged[0].sha256).toBe("c".repeat(64));
  });
});
