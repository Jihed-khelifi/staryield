// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";

import { decodeConsent, encodeConsent, readConsentCookie } from "@/lib/consent";
import { applyTheme, parseThemeCookie, themeFromCookieHeader, themeSlug } from "@/lib/theme";
import { vapidKeyBytes } from "@/lib/push";

afterEach(() => {
  document.cookie = "sy_theme=; max-age=0; path=/";
  document.cookie = "sy_consent=; max-age=0; path=/";
  delete document.documentElement.dataset.theme;
});

describe("theme", () => {
  it("maps names to slugs and rejects unknown cookie values", () => {
    expect(themeSlug("Light Cream")).toBe("light-cream");
    expect(themeSlug("Whatever")).toBe("warm-beige");
    expect(parseThemeCookie("<script>")).toBe("warm-beige");
    expect(themeFromCookieHeader("a=1; sy_theme=light-cream")).toBe("light-cream");
  });
  it("applies to <html> and persists the cookie", () => {
    applyTheme("light-cream");
    expect(document.documentElement.dataset.theme).toBe("light-cream");
    expect(document.cookie).toContain("sy_theme=light-cream");
  });
});

describe("consent cookie", () => {
  it("round-trips and rejects malformed values", () => {
    const value = encodeConsent({ analytics: false, marketing_attribution: true, personalization: true }, "2026-11");
    expect(decodeConsent(value)).toEqual({ analytics: false, marketing_attribution: true, personalization: true, version: "2026-11" });
    expect(decodeConsent("a=1")).toBeNull();
    document.cookie = `sy_consent=${encodeURIComponent(value)}; path=/`;
    expect(readConsentCookie()?.analytics).toBe(false);
  });
});

describe("push", () => {
  it("decodes the VAPID key", () => {
    const bytes = vapidKeyBytes("BMXeDXhYC7_T3HYYfqoDVpNkr0b4BCNPHL36oE4o94n6VIkLJ3uFOr0PX_s14eljM6j9rqHinA5WQ1C8pJRkedA");
    expect(bytes.length).toBe(65);
    expect(bytes[0]).toBe(4);
  });
});
