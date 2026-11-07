/**
 * Tablets (768–1023px): the signed-in header and the account pages fit the
 * screen. At exactly 768px the header used to be 7px (clients) to 11px
 * (advisors) too wide and the Today stats row 186px too wide.
 */
import { expect, test } from "@playwright/test";

import { CLIENT, advisor, expectNoHorizontalScroll, signIn } from "./helpers";

const WIDTHS = [768, 820, 1023];

test.describe("tablet layout", () => {
  test("client pages fit 768–1023px", async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await signIn(page, CLIENT.email, CLIENT.password);
    for (const path of ["/en/profile", "/en/credits", "/en/chatroom", "/en/support/requests"]) {
      await page.goto(path);
      await expect(page.locator("header.site-header")).toBeVisible();
      for (const width of WIDTHS) {
        await page.setViewportSize({ width, height: 1024 });
        await expectNoHorizontalScroll(page);
      }
      await page.setViewportSize({ width: 768, height: 1024 });
    }
    // The Today stats stay readable: all three tiles are visible.
    await page.goto("/en/profile");
    await expect(page.getByTestId("overview-stats").locator("> div")).toHaveCount(3);
  });

  test("advisor pages fit 768–1023px, availability shown as its dot", async ({ page }) => {
    const ramone = advisor("ramone");
    await page.setViewportSize({ width: 768, height: 1024 });
    await signIn(page, ramone.email, ramone.password);
    for (const path of ["/en/profile", "/en/profile/analytics", "/en/chatroom"]) {
      await page.goto(path);
      await expect(page.locator("header.site-header")).toBeVisible();
      for (const width of WIDTHS) {
        await page.setViewportSize({ width, height: 1024 });
        await expectNoHorizontalScroll(page);
      }
      await page.setViewportSize({ width: 768, height: 1024 });
    }
    // The compact trigger keeps its accessible name.
    await expect(page.locator("header.site-header").getByTestId("availability-menu")).toHaveAccessibleName(/^Availability: /);
  });
});
