import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { API_URL, CLIENT, expectNoHorizontalScroll, signIn } from "./helpers";

/**
 * Public pricing page (`/pricing`, docs/platform-extensions.md §24.8), server-rendered from
 * GET /api/v1/pricing. The failure and trial-off variants need an API whose catalog can be
 * switched, so they run only against the mock in e2e/README-style setups:
 *   E2E_PRICING_MOCK=1 (POST {API}/__mock/pricing {"mode": "ok" | "fail" | "trial_off"}).
 * Each variant uses its own locale (fail: es, trial off: fr) because the server keeps a short
 * per-locale cache (30 s); re-running the mock variants within 30 s needs a restarted dev server.
 */
const CANONICAL = ["welcome_600", "quick_150", "starter_315", "popular_675", "deep_1350", "premium_2400"];
const WIDTHS = [320, 360, 390, 768, 1024, 1199, 1200, 1440, 1920, 2560];
const MOCK = Boolean(process.env.E2E_PRICING_MOCK);

const columnsFor = (width: number) => (width >= 1200 ? 3 : width >= 768 ? 2 : 1);
const cards = (page: Page) => page.getByTestId("pack-card");

async function setMock(mode: "ok" | "fail" | "trial_off") {
  const res = await fetch(`${API_URL}/__mock/pricing`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ mode }) });
  expect(res.ok).toBe(true);
}

/** Position, order and size facts of the pack grid in the page. */
async function gridFacts(page: Page) {
  return page.evaluate(() => {
    const boxes = [...document.querySelectorAll<HTMLElement>("[data-testid=pack-card]")];
    const rects = boxes.map((el) => el.getBoundingClientRect());
    const columns = getComputedStyle(document.querySelector("[data-testid=pack-grid]")!).gridTemplateColumns.split(" ").length;
    return {
      codes: boxes.map((el) => el.dataset.pack),
      columns,
      radii: boxes.map((el) => getComputedStyle(el).borderTopLeftRadius),
      // Reading order (top to bottom, then left to right) must equal the DOM (canonical) order.
      visualOrder: rects.map((r, i) => ({ i, top: Math.round(r.top + window.scrollY), left: Math.round(r.left) })).sort((a, b) => a.top - b.top || a.left - b.left).map((x) => x.i),
      ctaHeights: boxes.map((el) => el.querySelector<HTMLElement>("[data-testid=pack-cta]")!.getBoundingClientRect().height),
      right: Math.max(...rects.map((r) => r.right)),
      viewport: window.innerWidth,
    };
  });
}

test.describe("public pricing", () => {
  // revisions 5.1: card acceptance marks next to the footnote (docs/platform-extensions.md §25.6).
  test("@responsive shows the Visa and Mastercard marks after the footnote, and no subscription is offered", async ({ page }) => {
    await page.goto("/en/pricing");
    const footnote = page.getByTestId("pricing-footnote");
    await expect(footnote).toBeVisible();
    const marks = page.locator("main").getByTestId("accepted-cards");
    await expect(marks).toBeVisible();
    await expect(marks.getByText("We accept")).toBeVisible();
    await expect(marks.getByRole("img")).toHaveCount(2);
    await expect(marks.getByRole("img").first()).toHaveAttribute("alt", "Visa");
    await expect(marks.getByRole("img").nth(1)).toHaveAttribute("alt", "Mastercard");
    const [noteBox, marksBox] = [(await footnote.boundingBox())!, (await marks.boundingBox())!];
    expect(marksBox.y).toBeGreaterThan(noteBox.y);
    await expect(page.locator("main")).not.toContainText(/Premium (?:Membership|subscription)|monthly plan/i);
    await expectNoHorizontalScroll(page);
  });

  test("@responsive renders the canonical catalog with CTAs to sign-up and the FAQ linking to the Refund Policy", async ({ page }) => {
    await page.goto("/en/pricing");
    await expect(page.getByRole("heading", { level: 1 })).toHaveText("Choose your credits. Choose your advisor. Stay in control.");
    await expect(cards(page)).toHaveCount(6);
    expect((await gridFacts(page)).codes).toEqual(CANONICAL);

    const welcome = cards(page).first();
    await expect(welcome).toContainText("NEW CLIENT OFFER");
    await expect(welcome).toContainText("Available once per eligible new customer after sign-up. Limit one per customer and payment method.");
    await expect(welcome.getByTestId("pack-cta")).toHaveText("Start With the Welcome Offer");
    // "Most popular" is shown only once sales data substantiates it (§24.1): the mock sends it,
    // a fresh API does not. The featured emphasis and the savings badge are always there.
    const popular = page.locator("[data-pack=popular_675]");
    await expect(popular).toHaveAttribute("data-featured", "true");
    await expect(popular).toContainText(/SAVE \d+%/);
    if (MOCK) await expect(popular).toContainText(/MOST POPULAR/);
    await expect(page.getByTestId("pricing-footnote")).toContainText("Minute estimates are approximate");

    // Signed-out visitors go to sign-up from every call to action.
    for (const href of await page.locator("[data-testid=pack-cta], [data-testid=pricing-hero-cta], [data-testid=pricing-final-cta]").evaluateAll((links) => links.map((a) => a.getAttribute("href")))) expect(href).toBe("/en/signup");

    const refund = page.getByTestId("faq-refund-link");
    await expect(refund).toHaveAttribute("href", "/en/refund-policy");
    await refund.click();
    await expect(page).toHaveURL(/\/en\/refund-policy$/);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText("Refund & Cancellation Policy");
  });

  for (const width of WIDTHS)
    test(`canonical order, columns, 8px corners and no horizontal overflow at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await page.goto("/en/pricing");
      await expect(cards(page)).toHaveCount(6);
      await expectNoHorizontalScroll(page);
      const facts = await gridFacts(page);
      expect(facts.codes).toEqual(CANONICAL);
      expect(facts.columns).toBe(columnsFor(width));
      expect(facts.visualOrder).toEqual([0, 1, 2, 3, 4, 5]);
      expect(new Set(facts.radii)).toEqual(new Set(["8px"]));
      for (const height of facts.ctaHeights) expect(height).toBeGreaterThanOrEqual(44);
      expect(facts.right).toBeLessThanOrEqual(facts.viewport + 1);
      // Every link and button on the page is a comfortable touch target.
      const small = await page.evaluate(() =>
        [...document.querySelectorAll<HTMLElement>("main a, main button")]
          .filter((el) => el.getBoundingClientRect().height > 0 && el.getBoundingClientRect().height < 44 && !el.closest("[aria-hidden=true]") && !(el.closest("p") && el.tagName === "A"))
          .map((el) => `${el.tagName} ${(el.textContent ?? "").trim().slice(0, 30)} ${Math.round(el.getBoundingClientRect().height)}`),
      );
      expect(small).toEqual([]);
    });

  test("@responsive has no serious accessibility violations", async ({ page }) => {
    await page.goto("/en/pricing");
    await expect(cards(page)).toHaveCount(6);
    await expectNoA11yViolations(page, ["main"]);
  });

  test("signed-in visitors are sent to the top-up page and the advisor directory", async ({ page }) => {
    // A real session (a stubbed /me alone is undone by the other signed-in calls answering 401).
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto("/en/pricing");
    await expect(page.getByTestId("pricing-hero-cta")).toHaveAttribute("href", "/en/psychics");
    await expect(page.getByTestId("pack-cta").first()).toHaveAttribute("href", "/en/credits");
    for (const href of await page.getByTestId("pack-cta").evaluateAll((links) => links.map((a) => a.getAttribute("href")))) expect(href).toBe("/en/credits");
    await expect(page.getByTestId("pricing-final-cta")).toHaveAttribute("href", "/en/psychics");
  });

  test("is translated and keeps the locale on links", async ({ page }) => {
    test.skip(MOCK, "covered by the French trial-off test below (the mock variants own the fr and es server caches)");
    await page.goto("/es/pricing");
    await expect(page.getByRole("heading", { level: 1 })).toHaveText("Elija sus créditos. Elija a su asesor. Mantenga el control.");
    await expect(page.getByTestId("faq-refund-link")).toHaveAttribute("href", "/es/refund-policy");
    await expect(page.getByTestId("pricing-hero-cta")).toHaveAttribute("href", "/es/signup");
    await expect(page).toHaveTitle(/Precios|Pricing/);
  });

  test("is indexed, titled and has hreflang alternates", async ({ page }) => {
    await page.goto("/en/pricing");
    await expect(page).toHaveTitle(/^Pricing — /);
    await expect(page.locator("meta[name=robots][content*=noindex]")).toHaveCount(0);
    await expect(page.locator("meta[name=description]")).toHaveAttribute("content", /one-time credit pack/);
    for (const locale of ["en", "fr", "es"]) await expect(page.locator(`link[rel=alternate][hreflang=${locale}]`)).toHaveAttribute("href", new RegExp(`/${locale}/pricing$`));
  });

  test.describe("switchable catalog (mock API)", () => {
    test.skip(!MOCK, "needs the mock API (E2E_PRICING_MOCK=1)");
    test.afterEach(() => setMock("ok"));

    test("when the API fails the page still renders and Retry loads the prices", async ({ page }) => {
      await setMock("fail");
      await page.goto("/es/pricing");
      // Next serves a stale data-cache entry while it revalidates, so a failure only shows on a cold cache.
      test.skip((await page.getByTestId("pack-grid").count()) > 0, "server data cache is warm for /es: delete .next/dev/cache/fetch-cache and restart the dev server");
      await expect(page.getByTestId("pricing-unavailable")).toBeVisible();
      await expect(page.getByTestId("pack-grid")).toHaveCount(0);
      await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
      await expect(page.getByTestId("faq-refund-link")).toBeVisible();
      await expect(page.getByTestId("pricing-final-cta")).toBeVisible();
      await expect(page.getByText(/\$\d/)).toHaveCount(0);
      await expectNoHorizontalScroll(page);
      await expectNoA11yViolations(page, ["main"]);
      await setMock("ok");
      await page.getByTestId("pricing-retry").click();
      await expect(cards(page)).toHaveCount(6);
      await expect(page.getByTestId("pricing-unavailable")).toHaveCount(0);
    });

    test("with the trial off the page uses plain sign-up copy", async ({ page }) => {
      await setMock("trial_off");
      await page.goto("/fr/pricing");
      await expect(cards(page)).toHaveCount(6);
      test.skip((await page.getByText(/minutes offertes/).count()) > 0, "server data cache is warm for /fr: delete .next/dev/cache/fetch-cache and restart the dev server");
      await expect(page.getByTestId("pricing-hero-cta")).toHaveText("S’inscrire");
      await expect(page.getByTestId("pricing-final-cta")).toHaveText("S’inscrire");
      await expect(page.getByText(/minutes offertes/)).toHaveCount(0);
      // The French page keeps its locale on every link and is fully translated.
      await expect(page.getByRole("heading", { level: 1 })).toHaveText("Choisissez vos crédits. Choisissez votre conseiller. Gardez le contrôle.");
      await expect(page.getByTestId("faq-refund-link")).toHaveAttribute("href", "/fr/refund-policy");
      await expect(page.getByTestId("pricing-hero-cta")).toHaveAttribute("href", "/fr/signup");
    });
  });
});
