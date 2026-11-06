/**
 * Revisions 2.0 visual sweep (M7): logo pages, CTA glow, Wellness Center
 * background and the mobile tab bar. Runs on desktop and mobile (@responsive).
 */
import { expect, test, type Page } from "@playwright/test";

import { CLIENT, advisor, signIn } from "./helpers";

/** Every visible gold call to action on the page carries the CTA glow. */
async function expectGoldCtasGlow(page: Page) {
  const missing = await page.evaluate(() =>
    [...document.querySelectorAll<HTMLElement>("a, button")]
      .filter((e) => {
        const r = e.getBoundingClientRect();
        return r.width > 40 && r.height > 18 && getComputedStyle(e).visibility !== "hidden" && !e.hasAttribute("role");
      })
      .filter((e) => /rgb\((194, 154, 61|217, 160, 54)\)/.test(getComputedStyle(e).backgroundColor))
      .filter((e) => !getComputedStyle(e).boxShadow.includes("194, 154, 61"))
      .map((e) => (e.textContent ?? "").trim().slice(0, 30)),
  );
  expect(missing).toEqual([]);
}

test.describe("logo pages", { tag: "@responsive" }, () => {
  for (const path of ["/en/login", "/en/signup", "/en/forgot-password", "/en/calculators/natal-chart"]) {
    test(`${path} uses the standard wordmark, clean rays and faint sample text`, async ({ page }) => {
      await page.goto(path);
      const wordmark = page.locator(".radiant-wordmark");
      await expect(wordmark).toBeVisible();
      expect(await wordmark.evaluate((el) => getComputedStyle(el).textTransform)).toBe("uppercase");
      // One ray set, centred on the sun.
      const rays = page.locator(".radiant-rays img");
      await expect(rays).toHaveCount(1);
      await expect(rays).toHaveAttribute("src", /assets\/brand\/auth-rays-[0-9a-f]+\.svg$/);
      const sun = await page.locator(".radiant-brand .sun-logo-large").boundingBox();
      const origin = await page.locator(".radiant-rays").boundingBox();
      expect(sun).not.toBeNull();
      expect(Math.abs(origin!.x - (sun!.x + sun!.width / 2))).toBeLessThanOrEqual(1);
      expect(Math.abs(origin!.y - (sun!.y + sun!.height / 2))).toBeLessThanOrEqual(1);
      // Sample text: Source Serif 4 Regular, #000 at 16 %, with a visible label kept.
      const input = page.locator("main input[placeholder]").first();
      const style = await input.evaluate((el) => {
        const s = getComputedStyle(el, "::placeholder");
        return { color: s.color, family: s.fontFamily, weight: s.fontWeight };
      });
      expect(style.color.replace(/\s/g, "")).toMatch(/^rgba\(0,0,0,0\.16\)$/);
      expect(style.family).toMatch(/serif/i);
      expect(style.weight).toBe("400");
      await expectGoldCtasGlow(page);
    });
  }
});

test.describe("CTA glow", { tag: "@responsive" }, () => {
  for (const path of ["/en", "/en/psychics", "/en/psychics/ramone", "/en/calculators/path-of-life", "/en/calculators/astrology"]) {
    test(`gold buttons glow on ${path}`, async ({ page }) => {
      await page.goto(path);
      await page.waitForLoadState("networkidle");
      await expectGoldCtasGlow(page);
      expect(await page.locator("[data-slot=button], a, button").count()).toBeGreaterThan(0);
    });
  }

  test("account pages and the glow token", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    for (const path of ["/en/profile", "/en/profile/settings"]) {
      await page.goto(path);
      await page.waitForLoadState("networkidle");
      await expectGoldCtasGlow(page);
    }
    // Resolve the token through a real box-shadow: the CSS build may serialise
    // the colour as rgb(194 154 61 / .42) or #c29a3d6b; the computed value is normalised.
    const shadow = await page.evaluate(() => {
      const probe = document.createElement("div");
      probe.style.boxShadow = "var(--shadow-cta-glow)";
      document.body.append(probe);
      const value = getComputedStyle(probe).boxShadow;
      probe.remove();
      return value;
    });
    expect(shadow).toMatch(/rgba\(194, 154, 61, 0\.4\d*\) 0px 4px 15px/);
  });
});

test.describe("Wellness Center background", { tag: "@responsive" }, () => {
  async function expectWellness(page: Page) {
    const bg = await page.locator(".dashboard-page").first().evaluate((el) => getComputedStyle(el).backgroundImage);
    expect(bg).toContain("radial-gradient");
    expect(bg).toContain("linear-gradient");
  }

  test("client overview, chart and settings", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    for (const path of ["/en/profile", "/en/profile/chart", "/en/profile/settings"]) {
      await page.goto(path);
      await expect(page.locator(".account-wrap")).toBeVisible();
      await expectWellness(page);
    }
  });

  test("advisor overview, analytics and settings", async ({ page }) => {
    const ramone = advisor("ramone");
    await signIn(page, ramone.email, ramone.password);
    for (const path of ["/en/profile", "/en/profile/analytics", "/en/profile/settings"]) {
      await page.goto(path);
      await expect(page.locator(".account-wrap")).toBeVisible();
      await expectWellness(page);
    }
  });
});

test.describe("mobile tab bar", { tag: "@mobile-only" }, () => {
  test("floating pill with Today, Psychics, Chat and Profile; the active tab is a capsule", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto("/en/profile");
    const nav = page.getByRole("navigation", { name: "Main navigation" });
    await expect(nav.getByRole("link")).toHaveText(["Today", "Psychics", "Chat", "Profile"]);
    const bar = await nav.evaluate((el) => {
      const s = getComputedStyle(el);
      return { position: s.position, radius: parseFloat(s.borderTopLeftRadius), bottom: parseFloat(s.bottom) };
    });
    expect(bar.position).toBe("fixed");
    expect(bar.radius).toBeGreaterThanOrEqual(24);
    expect(bar.bottom).toBeGreaterThanOrEqual(12);
    const active = nav.locator("a.active");
    await expect(active).toHaveText("Today");
    expect(await active.evaluate((el) => getComputedStyle(el).backgroundColor)).not.toBe("rgba(0, 0, 0, 0)");
  });
});
