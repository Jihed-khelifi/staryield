/**
 * Revisions 3.0 site configuration (docs/platform-extensions.md §15): promo
 * banners, the sign-up offer and free mode.
 *
 * The real seeded banner is checked against the stack. Everything else is a
 * UI-only state, so the client's config request is answered by
 * `page.route`. The first paint comes from a server-side fetch (which a
 * browser route cannot see), so the page clock is moved past the five-minute
 * refresh to make the client load the intercepted configuration; the same
 * clock then drives banner rotation deterministically.
 */
import { expect, test, type Page } from "@playwright/test";

import { accountTrigger, CLIENT, signIn } from "./helpers";

type Banner = { id: string; text: string; cta_label: string; action: { type: "link" | "none"; href: string }; audience?: "all" | "anonymous" | "signed_in" };

function config(over: { banners?: Banner[]; rotation?: number; free?: boolean; minutes?: number; offer?: boolean } = {}) {
  return {
    banners: { rotation_seconds: over.rotation ?? 4, items: (over.banners ?? []).map((b) => ({ audience: "all", ...b })) },
    signup_offer: { enabled: over.offer ?? true, free_minutes: over.offer === false ? 0 : (over.minutes ?? 5) },
    free_mode: { active: over.free ?? false },
    generated_at: new Date().toISOString(),
  };
}

/** Answer the client's /site/config request with `body`, then make the page ask for it. */
async function mockConfig(page: Page, body: ReturnType<typeof config>, path: string) {
  await page.route("**/api/v1/site/config*", (route) => route.fulfill({ json: body }));
  await page.clock.install();
  await page.goto(path);
  await page.clock.fastForward(5 * 60_000 + 1_000);
}

const bar = (page: Page) => page.getByTestId("promo-bar");
const active = (page: Page) => page.locator('[data-testid="promo-slide"][data-active="true"]');

const THREE: Banner[] = [
  { id: "a", text: "FIRST BANNER", cta_label: "GO FIRST", action: { type: "link", href: "/signup" } },
  { id: "b", text: "SECOND BANNER", cta_label: "GO SECOND", action: { type: "link", href: "https://staryield.net/en/faq" } },
  { id: "c", text: "THIRD BANNER", cta_label: "", action: { type: "none", href: "" } },
];

test.describe("promo banners", { tag: "@responsive" }, () => {
  test("the seeded banner shows to visitors with the live free minutes and signs them up", async ({ page }) => {
    await page.goto("/en");
    const slide = bar(page).getByRole("link", { name: /MINUTES FREE \+ 80% OFF FOR NEW CUSTOMERS/ });
    await expect(slide).toBeVisible();
    await expect(slide).toHaveAttribute("href", "/en/signup");
    await expect(slide).toContainText(/^\s*\d+ MINUTES FREE/);
    await page.goto("/fr");
    await expect(bar(page).getByRole("link")).toHaveAttribute("href", "/fr/signup");
  });

  test("the visitors-only seeded banner is hidden once signed in", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto("/en");
    await expect(accountTrigger(page)).toBeVisible();
    await expect(bar(page).getByText(/MINUTES FREE/)).toHaveCount(0);
  });
});

test.describe("rotating banners (intercepted config)", () => {
  test("rotate on the interval, wrap around, and build internal, external and text-only actions", async ({ page }) => {
    await mockConfig(page, config({ banners: THREE, rotation: 4 }), "/en");
    await expect(bar(page)).toBeVisible();
    await expect(active(page)).toContainText("FIRST BANNER");
    await expect(active(page).getByRole("link")).toHaveAttribute("href", "/en/signup");
    await expect(active(page).getByText("GO FIRST")).toBeVisible();

    await page.clock.runFor(4_100);
    await expect(active(page)).toContainText("SECOND BANNER");
    const external = active(page).getByRole("link");
    await expect(external).toHaveAttribute("href", "https://staryield.net/en/faq");
    await expect(external).toHaveAttribute("target", "_blank");
    await expect(external).toHaveAttribute("rel", "noopener noreferrer");

    await page.clock.runFor(4_100);
    await expect(active(page)).toContainText("THIRD BANNER");
    await expect(active(page).getByRole("link")).toHaveCount(0);

    await page.clock.runFor(4_100);
    await expect(active(page)).toContainText("FIRST BANNER");
  });

  test("hover and the pause button stop the rotation (WCAG 2.2.2)", async ({ page }) => {
    await mockConfig(page, config({ banners: THREE, rotation: 4 }), "/en");
    await bar(page).hover();
    await page.clock.runFor(20_000);
    await expect(active(page)).toContainText("FIRST BANNER");
    await page.mouse.move(5, 400);
    const toggle = page.getByTestId("promo-toggle");
    await toggle.click();
    await expect(toggle).toHaveAccessibleName("Play promotions");
    await page.mouse.move(5, 400);
    await page.clock.runFor(20_000);
    await expect(active(page)).toContainText("FIRST BANNER");
    await toggle.click();
    await expect(toggle).toHaveAccessibleName("Pause promotions");
    await page.mouse.move(5, 400);
    await page.clock.runFor(4_100);
    await expect(active(page)).toContainText("SECOND BANNER");
  });

  test("keyboard focus on a banner link pauses it", async ({ page }) => {
    await mockConfig(page, config({ banners: THREE, rotation: 4 }), "/en");
    await active(page).getByRole("link").focus();
    await page.clock.runFor(20_000);
    await expect(active(page)).toContainText("FIRST BANNER");
  });

  test("reduced motion keeps rotating without the fade", async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await mockConfig(page, config({ banners: THREE, rotation: 4 }), "/en");
    await page.clock.runFor(4_100);
    await expect(active(page)).toContainText("SECOND BANNER");
    const duration = await active(page).evaluate((el) => getComputedStyle(el).transitionDuration);
    expect(duration).toMatch(/^0s$|^0\.01ms$|^1e-05s$/);
  });

  test("a single banner has no pause control, and a hidden bar renders nothing", async ({ page }) => {
    await mockConfig(page, config({ banners: [THREE[0]] }), "/en");
    await expect(active(page)).toContainText("FIRST BANNER");
    await expect(page.getByTestId("promo-toggle")).toHaveCount(0);
    // The admin hides the bar: the next refresh removes it.
    await page.unroute("**/api/v1/site/config*");
    await page.route("**/api/v1/site/config*", (route) => route.fulfill({ json: config({ banners: [] }) }));
    await page.clock.fastForward(5 * 60_000 + 1_000);
    await expect(bar(page)).toHaveCount(0);
  });

  test("audience filtering: a signed-in-only banner is not shown to visitors", async ({ page }) => {
    await mockConfig(
      page,
      config({ banners: [{ id: "m", text: "MEMBERS ONLY", cta_label: "", action: { type: "none", href: "" }, audience: "signed_in" }] }),
      "/en",
    );
    await expect(bar(page)).toHaveCount(0);
  });

  test("the bar stays the same height while rotating (no layout shift)", async ({ page }) => {
    await mockConfig(page, config({ banners: THREE, rotation: 4 }), "/en");
    const first = (await bar(page).boundingBox())!.height;
    await page.clock.runFor(4_100);
    await page.clock.runFor(4_100);
    expect(Math.abs((await bar(page).boundingBox())!.height - first)).toBeLessThanOrEqual(1);
  });
});

test.describe("free mode display (intercepted config)", () => {
  test("home cards, the directory, the profile and the credits dialog show it", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.route("**/api/v1/site/config*", (route) => route.fulfill({ json: config({ free: true }) }));
    await page.clock.install();

    await page.goto("/en");
    await page.clock.fastForward(5 * 60_000 + 1_000);
    await expect(page.getByTestId("featured-advisor").first().getByTestId("free-chat-badge")).toBeVisible();

    await page.goto("/en/psychics");
    await page.clock.fastForward(5 * 60_000 + 1_000);
    await expect(page.locator(".advisor-card").first().getByTestId("free-chat-badge")).toBeVisible();

    await page.goto("/en/psychics/theo");
    await page.clock.fastForward(5 * 60_000 + 1_000);
    await expect(page.getByTestId("profile-ctas").locator("xpath=..").getByTestId("free-chat-badge")).toBeVisible();
    const rate = page.getByTestId("profile-free-rate");
    await expect(rate).toContainText("Free for a limited time");
    expect(await rate.locator("s").count()).toBe(1);

    await page.goto("/en/credits");
    await page.clock.fastForward(5 * 60_000 + 1_000);
    const notice = page.getByTestId("credits-free-notice");
    await expect(notice).toContainText("Live chat is free right now.");
    await expect(notice).toContainText("custom requests");
  });

  test("no free-mode copy when it is off", async ({ page }) => {
    await mockConfig(page, config({ free: false }), "/en/psychics/theo");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.getByTestId("free-chat-badge")).toHaveCount(0);
    await expect(page.getByTestId("profile-free-rate")).toHaveCount(0);
  });
});

test.describe("sign-up offer copy (intercepted config)", () => {
  test("the reviews call to action shows the live number of free minutes and hides the claim when the offer is off", async ({ page }) => {
    await mockConfig(page, config({ minutes: 8 }), "/en/reviews");
    await expect(page.getByTestId("reviews-free-minutes")).toContainText("8 free minutes");
    await page.unroute("**/api/v1/site/config*");
    await page.route("**/api/v1/site/config*", (route) => route.fulfill({ json: config({ offer: false }) }));
    await page.clock.fastForward(5 * 60_000 + 1_000);
    await expect(page.getByTestId("reviews-free-minutes")).toHaveCount(0);
  });
});
