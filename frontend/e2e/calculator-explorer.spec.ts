/**
 * Growth: the full natal chart of /profile/chart in the public natal chart
 * calculator, for visitors (no account), against the real API: wheel, planet
 * rows, tables, element balance, numbers computed in the browser, and the
 * house system switch that recalculates in place and keeps `h=` in the
 * fragment. The sun sign result links to it with the same birth details.
 */
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { expectNoHorizontalScroll, quietReportPrompt } from "./helpers";

test.beforeEach(({ page }) => quietReportPrompt(page));

const natalResponse = (page: Page, house: string) =>
  page.waitForResponse((r) => r.url().endsWith("/api/v1/astrology/natal") && r.request().method() === "POST" && r.request().postDataJSON()?.house_system === house);

test.describe("natal chart calculator: the full chart for every visitor", { tag: "@responsive" }, () => {
  test("wheel, planets and tables; switching the house system updates the chart and the URL", async ({ page }) => {
    await page.goto("/en/calculators/natal-chart");
    const form = page.getByTestId("calculator-form");
    await form.locator('input[name="birth.day"]').fill("14");
    await form.locator('input[name="birth.month"]').fill("03");
    await form.locator('input[name="birth.year"]').fill("1990");
    await form.locator('input[name="birth.time"]').fill("08:45");
    await page.getByRole("combobox", { name: /Birthplace/ }).fill("Paris");
    await page.getByRole("option", { name: /^Paris, (.+, )?France$/ }).first().click();
    const first = natalResponse(page, "placidus");
    await page.getByTestId("calculator-submit").click();
    const response = await first;
    expect(response.request().postDataJSON()).toMatchObject({ include: ["tarot", "aspects"] });

    const result = page.getByTestId("calculator-result");
    await expect(result.getByTestId("big-three")).toHaveText(/^Sun in Pisces · Moon in \w+ · \w+ rising$/);
    await expect(result.getByTestId("natal-wheel")).toBeVisible();
    await expect(page.locator("[data-testid=natal-wheel] [data-layer=cusps] line")).toHaveCount(12);
    const sun = result.getByTestId("planet-row-Sun").filter({ visible: true });
    await sun.click();
    await expect(sun).toHaveAttribute("aria-expanded", "true");
    await expect(result.getByText(/The Sun reflects your core identity/)).toBeVisible();
    await expect(result.getByTestId("chart-balance")).toBeVisible();
    await expect(result.getByTestId("my-numbers")).toBeVisible();
    await expect(result.getByTestId("my-numbers").getByRole("definition").first()).toHaveText("9");
    await result.getByTestId("chart-details-toggle").click();
    await expect(result.getByRole("table", { name: "Positions" })).toBeVisible();
    await expect(result.getByRole("table", { name: "House cusps" })).toBeVisible();
    await expect(result.getByRole("table", { name: "Aspects" })).toBeVisible();
    // The spread and the advisor call to action follow the chart.
    await expect(result.getByTestId("spread-cards").locator("li")).toHaveCount(12);
    await expect(result.getByTestId("advisor-cta").getByRole("heading", { name: "Talk to an advisor about your chart" })).toBeVisible();

    // Switch the house system in the explorer: same request with the new system, the chart stays meanwhile.
    const firstCusp = await page.locator("[data-testid=natal-wheel] [data-layer=cusps] line").nth(1).getAttribute("x2");
    const whole = natalResponse(page, "whole_sign");
    await result.getByTestId("house-system-select").click();
    await page.getByRole("option", { name: "Whole Sign" }).click();
    expect((await whole).status()).toBe(200);
    await expect(page).toHaveURL(/#v=1&d=1990-03-14&t=08%3A45&p=gn%3A2988507&h=whole_sign$/);
    await expect(result.getByText("Houses: Whole Sign.")).toBeVisible();
    await expect(result.getByText("Updating…")).toHaveCount(0);
    expect(await page.locator("[data-testid=natal-wheel] [data-layer=cusps] line").nth(1).getAttribute("x2")).not.toBe(firstCusp);
    await expectNoA11yViolations(page, ['[data-testid="calculator-result"]']);
    await expectNoHorizontalScroll(page);

    // Refresh keeps the house system (it lives in the fragment).
    await page.reload();
    await expect(page.getByTestId("house-system-select")).toHaveText(/Whole Sign/);
    await expect(page.getByText("Houses: Whole Sign.")).toBeVisible();

    // Back to Placidus from the cache: no new request.
    let requests = 0;
    page.on("request", (r) => {
      if (r.url().endsWith("/api/v1/astrology/natal")) requests++;
    });
    await page.getByTestId("house-system-select").click();
    await page.getByRole("option", { name: "Placidus" }).click();
    await expect(page).toHaveURL(/p=gn%3A2988507$/);
    await expect(page.getByText("Houses: Placidus.")).toBeVisible();
    await page.getByTestId("house-system-select").click();
    await page.getByRole("option", { name: "Whole Sign" }).click();
    await expect(page.getByText("Houses: Whole Sign.")).toBeVisible();
    expect(requests).toBe(1); // only Placidus, which this page view had not loaded since the refresh
  });

  test("without a birth time the chart shows planets, the house switch is off and the page says how to add it", async ({ page }) => {
    await page.goto("/en/calculators/natal-chart#v=1&d=1985-11-05");
    const result = page.getByTestId("calculator-result");
    await expect(result.getByTestId("natal-wheel")).toBeVisible();
    await expect(result.getByTestId("house-system-select")).toBeDisabled();
    await expect(result.getByText("Add your birth time and place to see your rising sign and houses.")).toBeVisible();
    await result.getByTestId("add-birth-time").first().click();
    await expect(page.getByTestId("calculator-form")).toBeVisible();
  });

  test("the sun sign result opens the free full chart with the same birth details", async ({ page }) => {
    await page.goto("/en/calculators/astrology#v=1&d=1990-05-02");
    await expect(page.getByTestId("sun-sign")).toHaveText("Taurus");
    await expect(page.getByText("Sign in / up")).toHaveCount(0);
    await page.getByTestId("full-chart-link").click();
    await expect(page).toHaveURL(/\/en\/calculators\/natal-chart#v=1&d=1990-05-02$/);
    await expect(page.getByTestId("calculator-result").getByTestId("natal-wheel")).toBeVisible();
    await expect(page.getByTestId("big-three")).toContainText("Sun in Taurus");
  });

  test("a cusp day without a birth time: both Sun signs from the sun sign result to the full chart, never the noon guess alone", async ({ page }) => {
    await page.goto("/en/calculators/astrology#v=1&d=1990-02-19");
    await expect(page.getByTestId("sun-sign-uncertain")).toContainText("the Sun moved from Aquarius into Pisces that day");
    await page.getByTestId("full-chart-link").click();
    await expect(page).toHaveURL(/\/en\/calculators\/natal-chart#v=1&d=1990-02-19$/);
    await expect(page.getByTestId("big-three")).toHaveText(/^Sun in Aquarius or Pisces/);
    const note = page.getByTestId("sun-uncertain");
    await expect(note).toContainText("Your Sun sign needs your birth time and place.");
    await note.getByRole("button", { name: "Add birth time and place" }).click();
    await expect(page.getByTestId("calculator-form")).toBeVisible();
    await expect(page.getByTestId("calculator-form").locator('input[name="birth.year"]')).toHaveValue("1990");
  });
});
