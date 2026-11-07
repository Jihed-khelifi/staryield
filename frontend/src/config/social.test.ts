import { describe, expect, it } from "vitest";

import registry from "@/data/r2-images.json";
import { SOCIAL_LINKS, visibleSocialLinks, type SocialLink } from "@/config/social";

describe("social links", () => {
  it("lists the eight footer platforms in order", () => {
    expect(SOCIAL_LINKS.map((link) => link.id)).toEqual([
      "facebook",
      "x",
      "instagram",
      "reddit",
      "tiktok",
      "threads",
      "google-play",
      "app-store",
    ]);
  });

  it("only renders profiles with an https URL", () => {
    expect(visibleSocialLinks().map((link) => link.id)).toEqual(["facebook", "x", "instagram", "reddit", "tiktok", "threads"]);
    const extra: SocialLink[] = [
      { id: "x", label: "X", href: "javascript:alert(1)", icon: "", size: 22 },
      { id: "x", label: "X", href: "http://insecure.example", icon: "", size: 22 },
      { id: "x", label: "X", href: "not a url", icon: "", size: 22 },
    ];
    expect(visibleSocialLinks(extra)).toEqual([]);
  });

  it("uses icons that are registered R2 assets", () => {
    const urls = new Set(registry.map((asset) => asset.url));
    for (const link of SOCIAL_LINKS) expect(urls.has(link.icon), link.icon).toBe(true);
  });
});
