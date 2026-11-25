/**
 * Growth: the tarot birth card calculator, end to end. It is computed in the
 * browser from the birth date alone (no API call with birth data), restores
 * from `#v=1&d=YYYY-MM-DD`, has no e-mail report, and is reachable from the
 * site's navigation in every language.
 */
import { expect, test } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { API_URL, expectNoHorizontalScroll } from "./helpers";

async function settle(page: import("@playwright/test").Page) {
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => undefined)),
    ),
  );
}

test.describe("tarot birth card calculator", { tag: "@responsive" }, () => {
  test("date in, card out: artwork, sums, constellation, CTA; refresh restores it; no birth data leaves the browser", async ({ page }) => {
    const calls: string[] = [];
    page.on("request", (r) => {
      // The card is computed in the browser: no astrology request, and the date never travels.
      if (r.url().startsWith(API_URL) && r.url().includes("/astrology/")) calls.push(`${r.method()} ${r.url()}`);
      if (r.url().includes("1990-11-29") || (r.postData() ?? "").includes("1990-11-29")) calls.push(`LEAK ${r.url()}`);
    });
    await page.goto("/en/calculators/tarot-birth-card");
    await expect(page).toHaveTitle(/Tarot Birth Card Calculator/);
    const form = page.getByTestId("calculator-form");
    await expect(form.getByText("Your birth date is used only in your browser to find your card. It is never sent to us.")).toBeVisible();
    await expect(form.locator('input[name="birth.time"]')).toHaveCount(0);

    // Validation first.
    await page.getByTestId("calculator-submit").click();
    await expect(form.getByText("Enter your date of birth.")).toBeVisible();

    await form.locator('input[name="birth.day"]').fill("29");
    await form.locator('input[name="birth.month"]').fill("11");
    await form.locator('input[name="birth.year"]').fill("1990");
    await page.getByTestId("calculator-submit").click();

    const result = page.getByTestId("calculator-result");
    await expect(result.getByTestId("tarot-headline")).toHaveText("Your tarot birth card is The Hierophant");
    expect(page.url()).toMatch(/\/en\/calculators\/tarot-birth-card#v=1&d=1990-11-29$/);
    await expect(result.getByTestId("tarot-steps")).toHaveText("11 + 29 + 1990 = 2030 → 2+0+3+0 = 5");
    await expect(result.getByTestId("birth-card-numeral")).toHaveText("V");
    const art = result.getByRole("img", { name: "The Hierophant" });
    await expect(art).toHaveAttribute("src", "https://assets.staryield.net/assets/tarot/the-hierophant.png");
    await expect.poll(() => art.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth)).toBe(210);
    await expect(result.getByTestId("birth-card-meaning")).toContainText("Tradition, teaching, belonging");
    await expect(result.getByTestId("tarot-constellation").locator("[data-chain]")).toHaveText([/^14\/5/]);
    await expect(result.getByTestId("advisor-cta").getByRole("link", { name: "Talk to an advisor" })).toBeVisible();
    // No e-mail report for this calculator.
    await expect(page.getByTestId("email-report-button")).toHaveCount(0);
    await expect(page.getByRole("dialog")).toHaveCount(0);
    // The long-form article follows, with its FAQ.
    await expect(page.locator("[data-calculator-article=tarot-birth-card]")).toBeVisible();
    const faq = page.getByTestId("article-faq").getByRole("button", { name: "Do I need my birth time to find my tarot birth card?" });
    await faq.scrollIntoViewIfNeeded();
    await faq.click();
    await expect(page.getByText(/the card is worked out on your device/)).toBeVisible();
    await settle(page);
    await expectNoA11yViolations(page, ['[data-testid="calculator-result"]', "[data-calculator-article]"]);
    await expectNoHorizontalScroll(page);

    // A shared link with two cards: personality to soul.
    await page.goto("/en/calculators/tarot-birth-card#v=1&d=1987-06-15");
    await expect(page.getByTestId("birth-card")).toHaveCount(2);
    await expect(page.getByTestId("birth-card-numeral")).toHaveText(["X", "I"]);
    await expect(page.getByTestId("tarot-chain")).toHaveText("10 Wheel Of Fortune → 1 The Magician");
    await page.reload();
    await expect(page.getByTestId("tarot-headline")).toHaveText("Your tarot birth card is Wheel Of Fortune");
    expect(calls).toEqual([]);
  });

  test("is in the site navigation and translated", async ({ page, isMobile }) => {
    await page.goto("/fr/calculators/tarot-birth-card#v=1&d=1940-07-10");
    await expect(page.getByTestId("tarot-headline")).toHaveText("Votre carte de naissance du tarot est Le Mat");
    await expect(page.getByTestId("birth-card-numeral")).toHaveText(["0", "IV"]);
    await expect(page.locator("[data-calculator-article=tarot-birth-card]")).toContainText("Les neuf constellations du tarot");
    await expect(page.locator("footer").getByRole("link", { name: "Carte de naissance du tarot" })).toHaveAttribute("href", "/fr/calculators/tarot-birth-card");
    if (!isMobile) {
      await page.goto("/es");
      await page.locator("header").getByRole("link", { name: "Arcano de nacimiento" }).click();
      await expect(page).toHaveURL(/\/es\/calculators\/tarot-birth-card$/);
      await expect(page.getByTestId("calculator-form")).toBeVisible();
      await expect(page).toHaveTitle(/Calculadora de carta de nacimiento del tarot/);
    }
  });

  test("the header fits every language at 1024 px", async ({ page, isMobile }) => {
    test.skip(isMobile, "desktop navigation");
    await page.setViewportSize({ width: 1024, height: 800 });
    for (const locale of ["en", "fr", "es"]) {
      await page.goto(`/${locale}/calculators/tarot-birth-card`);
      await expect(page.locator("header nav a[href$='/calculators/tarot-birth-card']")).toBeVisible();
      await expectNoHorizontalScroll(page);
    }
  });
});
