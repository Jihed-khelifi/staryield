/**
 * Growth: every calculator result exports a 9:16 image (1080 × 1920 PNG) with
 * "staryield.net" on it. The artwork comes from R2 with CORS (the canvas is
 * never tainted, or it could not be exported), the download is a real PNG,
 * and the link that goes with it never carries the birth data (fragment).
 * Run with E2E_BASE_URL=http://localhost:3001 (R2's CORS allows that origin).
 */
import { readFile } from "node:fs/promises";
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { quietReportPrompt } from "./helpers";

test.beforeEach(({ page }) => quietReportPrompt(page));

/** Width and height from a PNG's IHDR chunk (bytes 16–23, big-endian). */
async function pngSize(path: string) {
  const bytes = await readFile(path);
  expect(bytes.subarray(0, 8).toString("hex")).toBe("89504e470d0a1a0a");
  expect(bytes.subarray(12, 16).toString("ascii")).toBe("IHDR");
  return { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20), bytes: bytes.length };
}

async function downloadShareImage(page: Page, kind: string) {
  await page.getByTestId("share-result-button").first().click();
  const dialog = page.getByTestId("share-dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog.getByTestId("share-preview")).toBeVisible({ timeout: 20_000 });
  const download = page.waitForEvent("download");
  await dialog.getByTestId("share-download").click();
  const file = await download;
  expect(file.suggestedFilename()).toBe(`staryield-${kind}.png`);
  await expect(dialog.getByText("Image saved. Post it anywhere you like.")).toBeVisible();
  return pngSize((await file.path())!);
}

test.describe("share a calculator result as a 9:16 image", () => {
  test("tarot birth card: the PNG is 1080 × 1920, the artwork is loaded with CORS and the link has no birth data", async ({ page, context, baseURL }) => {
    const art: { url: string; origin?: string; allow?: string }[] = [];
    page.on("response", (r) => {
      if (r.url().includes("assets.staryield.net/assets/tarot/") && r.request().headers()["origin"])
        art.push({ url: r.url(), origin: r.request().headers()["origin"], allow: r.headers()["access-control-allow-origin"] });
    });
    const events: { name: string; properties?: Record<string, unknown> }[] = [];
    page.on("request", (r) => {
      if (r.url().endsWith("/api/v1/analytics/events") && r.method() === "POST") events.push(...((r.postDataJSON()?.events ?? []) as typeof events));
    });
    await page.goto("/en/calculators/tarot-birth-card#v=1&d=1987-06-15");
    await expect(page.getByTestId("tarot-headline")).toHaveText("Your tarot birth card is Wheel Of Fortune");
    // A share button next to the date and again by the call to action.
    await expect(page.getByTestId("share-result-button")).toHaveCount(2);

    const png = await downloadShareImage(page, "tarot-birth-card");
    expect([png.width, png.height]).toEqual([1080, 1920]);
    expect(png.bytes).toBeGreaterThan(50_000); // a real picture, not a blank canvas
    const preview = page.getByTestId("share-preview");
    await expect(preview).toHaveAttribute("alt", /My tarot birth card is Wheel Of Fortune/);
    expect(art.map((a) => a.url.replace(/\?.*$/, ""))).toEqual(
      expect.arrayContaining(["https://assets.staryield.net/assets/tarot/wheel-of-fortune.png", "https://assets.staryield.net/assets/tarot/the-magician.png"]),
    );
    for (const a of art) expect(a.allow).toBe(a.origin);
    await expectNoA11yViolations(page, ['[data-testid="share-dialog"]']);

    // Copy link: the calculator's page, without the fragment that holds the birth date.
    await context.grantPermissions(["clipboard-read", "clipboard-write"], { origin: baseURL });
    await page.getByTestId("share-copy").click();
    await expect(page.getByText("Link copied.")).toBeVisible();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(`${baseURL}/en/calculators/tarot-birth-card`);

    // Escape closes the dialog and focus returns to the button that opened it.
    await page.keyboard.press("Escape");
    await expect(page.getByTestId("share-dialog")).toHaveCount(0);
    await expect(page.getByTestId("share-result-button").first()).toBeFocused();
    await expect
      .poll(() => events.filter((e) => e.name === "share_opened" || e.name === "share_completed").map((e) => [e.name, e.properties?.method ?? ""]), { timeout: 15_000 })
      .toEqual(expect.arrayContaining([["share_opened", ""], ["share_completed", "download"], ["share_completed", "copy_link"]]));
  });

  test("path of life: the number image is 1080 × 1920 too", async ({ page }) => {
    await page.goto("/en/calculators/path-of-life#v=1&d=1990-11-29");
    await expect(page.getByTestId("number-lifePath")).toHaveText("5");
    const png = await downloadShareImage(page, "path-of-life");
    expect([png.width, png.height]).toEqual([1080, 1920]);
  });

  test("natal chart and sun sign export as well", async ({ page }) => {
    await page.goto("/en/calculators/natal-chart#v=1&d=1990-03-14&t=08:45&p=gn:2988507");
    await expect(page.getByTestId("big-three")).toBeVisible();
    const natal = await downloadShareImage(page, "natal-chart");
    expect([natal.width, natal.height]).toEqual([1080, 1920]);
    await expect(page.getByTestId("share-preview")).toHaveAttribute("alt", /Sun in Pisces/);

    await page.goto("/en/calculators/astrology#v=1&d=1990-05-02");
    await expect(page.getByTestId("sun-sign")).toHaveText("Taurus");
    const sun = await downloadShareImage(page, "astrology");
    expect([sun.width, sun.height]).toEqual([1080, 1920]);
    await expect(page.getByTestId("share-preview")).toHaveAttribute("alt", /I'm a Taurus/);
  });

  test("the sample result offers no share (its date is not the visitor's)", async ({ page }) => {
    await page.goto("/en/calculators/tarot-birth-card/results");
    await expect(page.getByTestId("tarot-headline")).toBeVisible();
    await expect(page.getByTestId("share-result-button")).toHaveCount(0);
  });
});
