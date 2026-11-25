/**
 * E-mail verification by secure link, announced live to every open tab.
 *
 * Needs the API's log mail driver (every e-mail body is printed in its log):
 * E2E_API_LOG (or E2E_MAIL_LOG, the name account-settings.spec.ts uses for the
 * same file) is the path of that log. Every journey registers its own account.
 */
import { readFileSync } from "node:fs";
import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { API_URL, expectNoHorizontalScroll } from "./helpers";

const PASSWORD = "E2e!Secure#2026x";
const LOG = process.env.E2E_API_LOG || process.env.E2E_MAIL_LOG;
const unique = () => `e2e+verify-${Date.now()}-${Math.random().toString(36).slice(2, 7)}@staryield.test`;

/** The verification links e-mailed to one address, oldest first (the log driver prints one line per e-mail, with its recipient). */
function linksFor(email: string): string[] {
  if (!LOG) return [];
  return readFileSync(LOG, "utf8")
    .split("\n")
    .filter((line) => line.includes(`"to": "${email}"`))
    .flatMap((line) => line.match(/\/en\/verify-email#token=[A-Za-z0-9_-]+/g) ?? []);
}

/** Register through the API: the context's cookie jar now holds the new session. */
async function register(context: BrowserContext) {
  const email = unique();
  const csrf = (await (await context.request.get(`${API_URL}/api/v1/auth/csrf`)).json()).csrf_token as string;
  const res = await context.request.post(`${API_URL}/api/v1/auth/register`, {
    headers: { "X-CSRF-Token": csrf, "Content-Type": "application/json" },
    data: { email, password: PASSWORD, accept_terms: true, display_name: "Verify Tester", birth_date: "1990-03-14", locale: "en" },
  });
  expect(res.status()).toBe(201);
  return email;
}

/** The newest link sent to the address once there are more than `before` (the outbox worker delivers within seconds). */
async function newLink(email: string, before = 0) {
  await expect.poll(() => linksFor(email).length, { timeout: 30_000, message: `no new verification e-mail to ${email} in the API log` }).toBeGreaterThan(before);
  return linksFor(email).at(-1)!;
}

/** The unread count of the visible notification bell (0 when it shows no badge). */
async function unread(page: Page) {
  const badge = page.getByTestId("notification-badge").filter({ visible: true });
  return (await badge.count()) ? Number(await badge.first().textContent()) : 0;
}

async function twoTabs(browser: Browser) {
  const context = await browser.newContext({ locale: "en-US", timezoneId: "Europe/Paris" });
  const email = await register(context);
  const profile = await context.newPage();
  await profile.goto("/en/profile");
  const settings = await context.newPage();
  await settings.goto("/en/profile/settings");
  return { context, profile, settings, email, link: await newLink(email) };
}

test.describe("e-mail verification", { tag: "@responsive" }, () => {
  test.skip(!LOG, "needs the API log (E2E_API_LOG or E2E_MAIL_LOG) to read the links of the log mail driver");

  test("the link verifies the address on another device and every open tab updates live", async ({ browser }) => {
    const { context, profile, settings, link } = await twoTabs(browser);

    // Both tabs show the unverified state, with the resend control.
    const notice = profile.getByTestId("email-verification-notice");
    await expect(notice).toBeVisible();
    await expect(notice.getByText("Confirm your email address")).toBeVisible();
    // The welcome link is waiting (GET /me says so): the notice says it was sent.
    await expect(notice.getByTestId("email-verification-notice-body")).toContainText(/^We sent a link to e2e\+verify-/);
    await expect(settings.getByTestId("email-unverified")).toBeVisible();
    await expectNoA11yViolations(profile, ['[data-testid="email-verification-notice"]']);
    const unreadBefore = await unread(profile);

    // Another device (no session): opening the link verifies without a click, and strips the fragment.
    const device = await browser.newContext({ locale: "en-US", timezoneId: "Europe/Paris" });
    const other = await device.newPage();
    await other.goto(link);
    await expect(other.getByRole("heading", { name: "Your email is verified" })).toBeVisible();
    expect(new URL(other.url()).hash).toBe("");
    await expect(other.getByTestId("verify-email-advisors")).toBeVisible();
    await expect(other.getByTestId("verify-email-profile")).toBeVisible();
    expect(await other.locator('meta[name="referrer"]').getAttribute("content")).toBe("no-referrer");
    await expectNoA11yViolations(other, ['[data-testid="verify-email"]']);

    // Both original tabs update without a reload: toast, no more notice, a Verified badge.
    for (const tab of [profile, settings]) await expect(tab.getByText("Your email is verified. Thank you for securing your account.")).toBeVisible();
    // The in-app notification arrives live too, without a second toast.
    await expect.poll(() => unread(profile), { timeout: 15_000 }).toBe(unreadBefore + 1);
    for (const tab of [profile, settings]) {
      await expect(tab.getByText("Your email is verified. Thank you for securing your account.")).toHaveCount(1);
      await expect(tab.getByText("Your email address is confirmed. Thank you for securing your account.")).toHaveCount(0);
    }
    await expect(notice).toHaveCount(0);
    await expect(settings.getByTestId("email-verified-badge")).toBeVisible();
    await expect(settings.getByTestId("email-unverified")).toHaveCount(0);

    // The link works once: reopening it explains and does not verify anything again.
    await other.goto(link);
    await expect(other.getByRole("heading", { name: "This link has expired" })).toBeVisible();
    await expect(other.getByTestId("verify-email-login")).toBeVisible();

    // A reload keeps the verified state (it comes from the account, not from the event).
    await profile.reload();
    await expect(profile.getByTestId("client-overview")).toBeVisible();
    await expect(profile.getByTestId("email-verification-notice")).toHaveCount(0);
    await device.close();
    await context.close();
  });

  test("an invalid link says so and a signed-out visitor is sent to log in", async ({ browser }) => {
    const context = await browser.newContext({ locale: "en-US", timezoneId: "Europe/Paris" });
    const page = await context.newPage();
    await page.goto("/en/verify-email#token=not-a-real-token");
    await expect(page.getByRole("heading", { name: "This link has expired" })).toBeVisible();
    expect(new URL(page.url()).hash).toBe("");
    const login = page.getByTestId("verify-email-login");
    await expect(login).toHaveText("Log in to get a new link");
    await expect(login).toHaveAttribute("href", /\/en\/login\?next=%2Fen%2Fprofile$/);
    await expectNoA11yViolations(page, ['[data-testid="verify-email"]']);
    await expectNoHorizontalScroll(page);
    await page.goto("/en/verify-email");
    await expect(page.getByRole("heading", { name: "This link is incomplete" })).toBeVisible();
    await context.close();
  });

  test("resending: the cooldown after the welcome e-mail is visible, then a new link goes to the masked address and replaces the old one", async ({ browser }) => {
    // The welcome link was sent at sign-up, so the server's 60 s cooldown is waited out once.
    test.slow();
    const { context, settings, profile, email, link: welcome } = await twoTabs(browser);
    const control = settings.getByTestId("email-unverified");
    const button = control.getByTestId("resend-verification-button");

    // Right after the welcome e-mail the button already counts down (from the account's waiting link): no click into the cooldown.
    await expect(button).toBeDisabled();
    await expect(button).toHaveText(/Resend in \d+s/);
    await expect(profile.getByTestId("email-verification-notice").getByTestId("resend-verification-button")).toHaveText(/Resend in \d+s/);
    await expectNoA11yViolations(settings, ['[data-testid="email-unverified"]']);

    // Once the countdown ends, a new link goes out to the masked address.
    await expect(button).toBeEnabled({ timeout: 75_000 });
    const before = linksFor(email).length;
    await button.click();
    await expect(control.getByText(/We sent a new link to .\*\*\*@staryield\.test/)).toBeVisible();
    await expect(button).toBeDisabled();
    await expect(button).toHaveText(/Resend in \d+s/);
    const fresh = await newLink(email, before);
    expect(fresh).not.toBe(welcome);

    // The welcome link stopped working; the signed-in browser opens the new one in a tab and the other tabs follow.
    const verify = await context.newPage();
    await verify.goto(welcome);
    await expect(verify.getByRole("heading", { name: "This link has expired" })).toBeVisible();
    await expect(verify.getByTestId("resend-verification")).toBeVisible();
    await verify.goto(fresh);
    await expect(verify.getByRole("heading", { name: "Your email is verified" })).toBeVisible();
    await expect(settings.getByTestId("email-verified-badge")).toBeVisible();
    await expect(profile.getByTestId("email-verification-notice")).toHaveCount(0);
    await context.close();
  });

  test("the phone layout keeps the notice readable", async ({ browser }) => {
    const context = await browser.newContext({ locale: "en-US", timezoneId: "Europe/Paris", viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });
    const email = await register(context);
    const page: Page = await context.newPage();
    await page.goto("/en/profile");
    await expect(page.getByTestId("email-verification-notice")).toBeVisible();
    await expectNoHorizontalScroll(page);
    await newLink(email);
    await context.close();
  });
});
