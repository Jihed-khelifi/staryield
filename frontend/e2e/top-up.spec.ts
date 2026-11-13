/**
 * Credit top-up (revisions 5.0, docs/platform-extensions.md §24.7): one
 * dialog opened in place from the header and the chatroom, the Welcome Pack
 * offer, the pack radio group, the explicit acknowledgement, the sandbox
 * payment and the live balance. Specs that need "never bought credits" use a
 * brand-new account; the live-chat spec shares the usual advisor with the
 * other chat journeys, so it runs serially.
 */
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { advisor, buyStandardPack, expectNoHorizontalScroll, newClient, setAvailability, signedInPage, topUpCatalog, uniqueSandboxCard, walletBalance } from "./helpers";

const THEO = advisor("theo");
const dialog = (page: Page) => page.getByTestId("top-up");
const balancePill = (page: Page) => page.locator("a.balance-pill").filter({ visible: true }).first();
const pack = (page: Page, name: RegExp | string) => page.getByRole("radio", { name });
const money = (cents: number) => (cents % 100 === 0 ? `$${cents / 100}` : `$${(cents / 100).toFixed(2)}`);

/** Fill the sandbox card fields (only a token ever reaches the API). */
async function fillSandboxCard(page: Page) {
  // A card no earlier run used: the Welcome Pack is one per payment method (§24.2).
  const card = uniqueSandboxCard();
  await page.getByLabel("Card number").fill(card.number);
  await page.getByLabel("Expiry date").fill(card.expiry);
  await page.getByLabel("CVV").fill("123");
}

/** Tick the required policy agreement (§25.6) on the pay step. */
async function agreeToPolicies(page: Page) {
  await page.getByTestId("top-up-consent").check();
  await expect(page.getByTestId("top-up-consent")).toBeChecked();
}

test.describe("credit top-up", { tag: "@responsive" }, () => {
  test("opens from the header in place, a pack is chosen, checkout is confirmed and the balance updates", async ({ page }) => {
    await newClient(page, "topup");
    await page.goto("/en/profile");
    const before = await walletBalance(page);
    const { packs } = await topUpCatalog(page);
    const starter = packs.find((p) => p.code === "starter_315")!;
    expect(starter, "the canonical Starter pack is in the catalog").toBeTruthy();

    const here = page.url();
    await balancePill(page).click();
    await expect(dialog(page)).toBeVisible();
    // In place: no navigation, no new page.
    expect(page.url()).toBe(here);
    await expect(page.getByRole("dialog", { name: "Choose your credit pack" })).toBeVisible();
    await expect(page.getByRole("radiogroup", { name: "Credit packs" })).toBeVisible();

    // Choosing updates the dock together and never charges.
    await pack(page, new RegExp(starter.name)).click();
    await expect(pack(page, new RegExp(starter.name))).toHaveAttribute("aria-checked", "true");
    await expect(page.getByTestId("top-up-dock-pack")).toHaveText(`${starter.name} · ${starter.credits.toLocaleString("en-US")} credits`);
    await expect(page.getByTestId("top-up-total")).toHaveText(`Total: ${money(starter.price_cents)} USD`);
    await expect(page.getByTestId("top-up-continue")).toHaveText(`Continue to Checkout — ${money(starter.price_cents)}`);
    await expect(page.getByTestId("top-up-ack")).toContainText(`purchase ${starter.credits.toLocaleString("en-US")} credits for ${money(starter.price_cents)} USD as a one-time payment`);
    expect(await walletBalance(page)).toBe(before);
    await expectNoHorizontalScroll(page);

    await page.getByTestId("top-up-continue").click();
    await expect(page.getByTestId("checkout-pay")).toHaveText(`Pay ${money(starter.price_cents)} USD`);
    // Going back keeps the choice.
    await page.getByTestId("top-up-back").click();
    await expect(pack(page, new RegExp(starter.name))).toHaveAttribute("aria-checked", "true");
    await page.getByTestId("top-up-continue").click();

    await fillSandboxCard(page);
    await agreeToPolicies(page);
    await page.getByTestId("checkout-pay").click();
    await expect(page.getByText(/credits added to your balance\./)).toBeVisible();
    await expect(dialog(page)).toHaveCount(0);
    await expect.poll(() => walletBalance(page)).toBe(before + starter.credits);
    // The header balance follows (response + realtime), without a reload.
    await expect(balancePill(page)).toContainText((before + starter.credits).toLocaleString("en-US"));
    expect(page.url()).toBe(here);
  });

  test("the pay step shows the policy checkbox, card marks, the statement descriptor and the support line; paying needs the box ticked (§25.6)", async ({ page }) => {
    await newClient(page, "consent");
    await page.goto("/en/credits");
    await expect(dialog(page)).toBeVisible();
    await page.getByTestId("top-up-continue").click();
    await fillSandboxCard(page);
    const before = await walletBalance(page);

    // Required and unchecked; the three links open in a new tab.
    const box = page.getByTestId("top-up-consent");
    await expect(box).not.toBeChecked();
    for (const [name, href] of [["Terms of Use", "/en/terms"], ["Privacy Policy", "/en/privacy"], ["Refund & Cancellation Policy", "/en/refund-policy"]] as const) {
      const link = dialog(page).getByRole("link", { name: new RegExp(`^${name}`) });
      await expect(link).toHaveAttribute("href", href);
      await expect(link).toHaveAttribute("target", "_blank");
    }

    // Card marks, descriptor and support line are on screen next to the button.
    await expect(page.getByTestId("accepted-cards").getByAltText("Visa")).toBeVisible();
    await expect(page.getByTestId("accepted-cards").getByAltText("Mastercard")).toBeVisible();
    await expect(page.getByTestId("top-up-descriptor")).toHaveText(/^This charge appears on your card statement as .+\.$/);
    await expect(page.getByTestId("support-line")).toContainText("support@staryield.net");
    await expect(page.getByTestId("support-line")).toContainText("Support is available 24/7. We reply within 1–2 days.");
    await expect(page.getByTestId("support-email")).toHaveAttribute("href", "mailto:support@staryield.net");
    await expectNoHorizontalScroll(page);

    // Without the box: a message, focus on the box, nothing charged.
    await page.getByTestId("checkout-pay").click();
    await expect(dialog(page).getByText("Accept the Terms of Use, Privacy Policy and Refund & Cancellation Policy to continue.")).toBeVisible();
    await expect(box).toBeFocused();
    expect(await walletBalance(page)).toBe(before);
    await expectNoA11yViolations(page, ["[data-testid=top-up]"]);

    // The box is at least 44 px to hit.
    const target = await box.evaluate((el) => {
      const after = getComputedStyle(el, "::after");
      const rect = el.getBoundingClientRect();
      const inset = Math.abs(parseFloat(after.top));
      return { w: rect.width + 2 * inset, h: rect.height + 2 * inset };
    });
    expect(target.w).toBeGreaterThanOrEqual(44);
    expect(target.h).toBeGreaterThanOrEqual(44);

    await agreeToPolicies(page);
    await page.getByTestId("checkout-pay").click();
    await expect(page.getByText(/credits added to your balance\./)).toBeVisible();
    await expect.poll(() => walletBalance(page)).toBeGreaterThan(before);
  });

  test("the pay step fits a phone without a horizontal scroll", async ({ page }) => {
    await newClient(page, "phone");
    await page.setViewportSize({ width: 390, height: 780 });
    await page.goto("/en/credits");
    await page.getByTestId("top-up-continue").click();
    await expect(page.getByTestId("top-up-consent")).toBeVisible();
    await expect(page.getByTestId("checkout-pay")).toBeVisible();
    await expectNoHorizontalScroll(page);
  });

  test("the Welcome Pack is preselected for a new client and is gone once bought", async ({ page }) => {
    await newClient(page, "welcome");
    expect((await topUpCatalog(page)).welcomeEligible).toBe(true);
    await page.goto("/en/credits");
    await expect(dialog(page)).toBeVisible();
    const welcome = pack(page, /Welcome Pack/);
    await expect(welcome).toHaveAttribute("aria-checked", "true");
    await expect(welcome).toContainText("Your one-time new-client offer.");
    await expect(page.getByTestId("top-up-dock-pack")).toContainText("Welcome Pack");
    await expectNoHorizontalScroll(page);

    await page.getByTestId("top-up-continue").click();
    await fillSandboxCard(page);
    await agreeToPolicies(page);
    await page.getByTestId("checkout-pay").click();
    await expect(page.getByText(/credits added to your balance\./)).toBeVisible();
    // The deep link closes to the profile.
    await page.waitForURL(/\/en\/profile/);

    expect((await topUpCatalog(page)).welcomeEligible).toBe(false);
    await page.goto("/en/credits");
    await expect(dialog(page)).toBeVisible();
    // Absent, never disabled; the first standard pack is the initial selection.
    await expect(pack(page, /Welcome Pack/)).toHaveCount(0);
    await expect(page.getByRole("radio").first()).toHaveAttribute("aria-checked", "true");
    await expect(page.getByTestId("top-up-dock-pack")).not.toContainText("Welcome Pack");
  });

  test("'Not now' and Escape close the dialog without charging", async ({ page }) => {
    await newClient(page, "close");
    await page.goto("/en/profile");
    const before = await walletBalance(page);
    await balancePill(page).click();
    await expect(dialog(page)).toBeVisible();
    await page.getByTestId("top-up-dismiss").click();
    await expect(dialog(page)).toHaveCount(0);
    await balancePill(page).click();
    await expect(dialog(page)).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(dialog(page)).toHaveCount(0);
    expect(await walletBalance(page)).toBe(before);
  });
});

test.describe("credit top-up on desktop", () => {
  test("is a keyboard-operable radio group in a focus-trapping dialog that returns focus", async ({ page }) => {
    await newClient(page, "keys");
    await buyStandardPack(page, (packs) => packs[0]); // a returning client: the offer is gone, the packs are all standard
    await page.goto("/en/profile");
    await balancePill(page).focus();
    await page.keyboard.press("Enter");
    await expect(dialog(page)).toBeVisible();

    const radios = page.getByRole("radio");
    const first = radios.first();
    await expect(first).toHaveAttribute("aria-checked", "true");
    await first.focus();
    await page.keyboard.press("ArrowRight");
    await expect(radios.nth(1)).toHaveAttribute("aria-checked", "true");
    await expect(radios.nth(1)).toBeFocused();
    await page.keyboard.press("ArrowLeft");
    await expect(first).toHaveAttribute("aria-checked", "true");
    // Space picks the focused card too.
    await radios.nth(2).focus();
    await page.keyboard.press("Space");
    await expect(radios.nth(2)).toHaveAttribute("aria-checked", "true");

    // Focus cannot leave the dialog.
    for (let i = 0; i < 12; i++) {
      await page.keyboard.press("Tab");
      expect(await dialog(page).evaluate((el) => el.contains(document.activeElement))).toBe(true);
    }
    await expect(page.getByTestId("top-up-close")).toHaveAccessibleName("Close");
    await expectNoA11yViolations(page, ["[data-testid=top-up]"]);

    await page.keyboard.press("Escape");
    await expect(dialog(page)).toHaveCount(0);
    await expect(balancePill(page)).toBeFocused();
  });

  test("the pack grid follows the breakpoints: 3 columns, 2 on tablets, 1 on phones, a sheet below 768 px", async ({ page }) => {
    await newClient(page, "grid");
    await buyStandardPack(page, (packs) => packs[0]);
    await page.goto("/en/credits");
    await expect(dialog(page)).toBeVisible();
    const columns = () =>
      page.locator(".top-up-pack-grid").evaluate((el) => getComputedStyle(el).gridTemplateColumns.split(" ").length);
    for (const [width, expected] of [[1440, 3], [1024, 2], [820, 2], [390, 1]] as const) {
      await page.setViewportSize({ width, height: 900 });
      expect(await columns(), `${width}px`).toBe(expected);
      const box = (await dialog(page).boundingBox())!;
      if (width >= 768) {
        expect(box.width).toBeLessThanOrEqual(1100);
        expect(box.width).toBeGreaterThanOrEqual(Math.min(960, width - 48) - 1);
      } else {
        // A full-screen sheet.
        expect(box.width).toBeGreaterThanOrEqual(width - 1);
        expect(box.height).toBeGreaterThanOrEqual(900 - 1);
      }
      await expectNoHorizontalScroll(page);
    }
  });
});

test.describe.serial("top-up during a live chat", () => {
  test("opens in place from the chatroom, with the advisor's rate, without leaving or pausing the chat", async ({ browser }) => {
    const { context: psychicCtx, page: psychic } = await signedInPage(browser, THEO.email, THEO.password);
    const clientCtx = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "en-US", timezoneId: "Europe/Paris" });
    const client = await clientCtx.newPage();
    try {
      await newClient(client, "livechat");
      await buyStandardPack(client);
      await psychic.goto("/en/profile");
      await setAvailability(psychic, "online");
      await psychic.goto("/en/chatroom");

      await client.goto(`/en/chatroom?reader=${THEO.slug}`);
      await client.getByRole("button", { name: /Start live chat/ }).click();
      await psychic.getByRole("button", { name: "Accept", exact: true }).click();
      await expect(client.getByText("Live chat in progress")).toBeVisible();
      await expect(psychic.getByText("Live chat in progress")).toBeVisible();

      const here = client.url();
      const pagesBefore = clientCtx.pages().length;
      const pill = client.getByRole("link", { name: /Balance: .* credits/ }).filter({ visible: true }).first();
      await pill.click();

      // The dialog opens over the chat: same URL, no new tab, estimate at the advisor's rate.
      await expect(dialog(client)).toBeVisible();
      expect(client.url()).toBe(here);
      expect(clientCtx.pages()).toHaveLength(pagesBefore);
      await expect(client.getByTestId("top-up-advisor-estimate")).toHaveText(/^At Theo's rate of \d+ credits\/minute, this pack provides approximately [\d.]+ minutes\.$/);
      // The reading goes on for both sides: nothing paused or ended.
      await expect(psychic.getByText("Live chat in progress")).toBeVisible();

      await client.getByTestId("top-up-dismiss").click();
      await expect(dialog(client)).toHaveCount(0);
      expect(client.url()).toBe(here);
      await expect(client.getByText("Live chat in progress")).toBeVisible();
      await expect(psychic.getByText("Live chat in progress")).toBeVisible();

      await client.getByRole("button", { name: /End the chat with/ }).click({ timeout: 5000 }).catch(() => undefined);
    } finally {
      await psychic.goto("/en/profile").catch(() => undefined);
      await setAvailability(psychic, "offline").catch(() => undefined);
      await clientCtx.close();
      await psychicCtx.close();
    }
  });
});

test.describe("credit top-up deep link", () => {
  test("/credits closes to ?next=", async ({ page }) => {
    await newClient(page, "next");
    await page.goto(`/en/credits?next=${encodeURIComponent("/en/psychics")}`);
    await expect(dialog(page)).toBeVisible();
    await page.getByTestId("top-up-dismiss").click();
    await page.waitForURL(/\/en\/psychics$/);
  });
});
