import { expect, test, type Page } from "@playwright/test";

import { API_URL, expectNoHorizontalScroll, newClient } from "./helpers";

// growth: referrals. The inviter copies their link, a new visitor lands with
// ?ref=, sees the invitation, signs up (the code travels with the sign-up),
// and the inviter's counters show the friend after a reload. Names never
// appear on the inviter's side.

const PASSWORD = "E2e!Secure#2026x";

async function inviteLink(page: Page): Promise<string> {
  const field = page.getByTestId("referral-link").first();
  await expect(field).toHaveValue(/\?ref=[A-Z2-9]{8}$/);
  return field.inputValue();
}

test.describe("invite friends", () => {
  test("the inviter copies the link, a friend signs up with it and is counted", async ({ page, browser, baseURL }) => {
    await page.context().grantPermissions(["clipboard-read", "clipboard-write"], { origin: baseURL });
    await newClient(page, "inviter");

    // Overview card: the reward, the link and an empty progress.
    await page.goto("/en/profile");
    const card = page.getByTestId("referral-card");
    await expect(card.getByRole("heading", { name: "Invite friends, get free minutes" })).toBeVisible();
    await expect(card.getByTestId("referral-progress")).toHaveText("No friend has joined with your link yet.");
    const link = await inviteLink(page);
    const code = new URL(link).searchParams.get("ref")!;
    expect(new URL(link).pathname).toBe("/en");

    // Copy: the clipboard holds the link and the button says so.
    await card.getByTestId("referral-copy").click();
    await expect(card.getByTestId("referral-copy")).toHaveText("Copied");
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(link);

    // The invite page explains the rules before the launch.
    await card.getByRole("link", { name: "How it works" }).click();
    await page.waitForURL(/\/en\/profile\/invite$/);
    await expect(page.getByTestId("referral-program-notice")).toHaveAttribute("data-state", "beta");
    await expect(page.getByTestId("stat-joined")).toContainText("0");
    await page.getByRole("button", { name: "What does my friend get?" }).click();
    await expect(page.getByText(/There is no extra reward for being invited\./)).toBeVisible();

    // A new visitor opens the link: the code is kept, ?ref= leaves the address bar.
    const visitor = await browser.newContext({ locale: "en-US", timezoneId: "Europe/Paris" });
    const friend = await visitor.newPage();
    await friend.goto(link.replace(new URL(link).origin, baseURL!));
    await expect.poll(() => new URL(friend.url()).searchParams.has("ref")).toBe(false);
    const welcome = friend.getByTestId("referral-welcome");
    await expect(welcome.getByRole("heading", { name: "A friend invited you to Staryield" })).toBeVisible();
    await expect(welcome).toContainText("like every new client");
    expect(await friend.evaluate(() => JSON.parse(localStorage.getItem("staryield.referral") ?? "null")?.code)).toBe(code);

    // A later link does not replace the first one.
    await friend.goto("/en?ref=ZZZZ2345");
    expect(await friend.evaluate(() => JSON.parse(localStorage.getItem("staryield.referral") ?? "null")?.code)).toBe(code);

    // Sign-up: the banner stays on the steps and the code travels with the attribution.
    await welcome.getByRole("link", { name: "Create my account" }).click();
    await friend.waitForURL(/\/en\/signup$/);
    await expect(friend.getByTestId("referral-welcome")).toBeVisible();
    await friend.locator("input[name=email]").fill(`e2e+friend-${Date.now()}@staryield.test`);
    await friend.getByRole("checkbox").check();
    await friend.locator("form button[type=submit]").click();
    await friend.waitForURL(/\/en\/signup\/password$/);
    await expect(friend.getByTestId("referral-welcome")).toBeVisible();
    await friend.locator("input[name=password]").fill(PASSWORD);
    await friend.locator("input[name=confirm]").fill(PASSWORD);
    const register = friend.waitForRequest((r) => r.url() === `${API_URL}/api/v1/auth/register` && r.method() === "POST");
    await friend.locator("form button[type=submit]").click();
    expect((await register).postDataJSON().attribution).toMatchObject({ referral_code: code });
    await friend.waitForURL(/\/en\/signup\/birth$/);
    // The invite was used: this browser does not carry it to another account.
    expect(await friend.evaluate(() => localStorage.getItem("staryield.referral"))).toBeNull();
    await visitor.close();

    // The inviter sees one friend joined, and nobody's name.
    await page.reload();
    await expect(page.getByTestId("stat-joined")).toContainText("1");
    await expect(page.getByTestId("stat-qualified")).toContainText("0");
    await page.goto("/en/profile");
    await expect(page.getByTestId("referral-progress")).toHaveText("1 friend joined · 0 completed a reading");
  });

  test("the invite page reads well on a phone and in French @responsive", async ({ page }) => {
    await newClient(page, "invite-phone");
    await page.goto("/fr/profile/invite");
    await expect(page.getByRole("heading", { name: "Invitez vos amis, gagnez des minutes gratuites" })).toBeVisible();
    await expect(page.getByTestId("referral-copy")).toHaveText(/Copier le lien/);
    await expect(page.getByTestId("referral-faq")).toBeVisible();
    await expectNoHorizontalScroll(page);
  });

  test("advisors have no invite page", async ({ page }) => {
    await page.goto("/en/login");
    await page.locator("input[name=email]").fill("theo@advisors.staryield.net");
    await page.locator("input[name=password]").fill("Psych1c!Secure#2026");
    await Promise.all([page.waitForURL((url) => !url.pathname.endsWith("/login")), page.locator("form button[type=submit]").click()]);
    await page.goto("/en/profile/invite");
    await page.waitForURL(/\/en\/profile$/);
    await expect(page.getByTestId("invite-page")).toHaveCount(0);
  });
});
