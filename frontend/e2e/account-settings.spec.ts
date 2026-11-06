/**
 * Account Settings (revisions 2.0): settings round trip, app-wide theme,
 * devices, data export, deactivate/close with the login offers, membership
 * with a sandbox card and push fallbacks. Every journey registers its own
 * account, so the shared seed accounts are never changed.
 */
import { readFileSync } from "node:fs";
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { API_URL, expectNoHorizontalScroll } from "./helpers";

const PASSWORD = "E2e!Secure#2026x";
const unique = () => `e2e+set-${Date.now()}-${Math.random().toString(36).slice(2, 7)}@staryield.test`;

/** Register an account through the API (shares the page's cookie jar) and sign in with the form. */
async function newAccount(page: Page, name = "Settings Tester") {
  const email = unique();
  const csrf = (await (await page.request.get(`${API_URL}/api/v1/auth/csrf`)).json()).csrf_token as string;
  const res = await page.request.post(`${API_URL}/api/v1/auth/register`, {
    headers: { "X-CSRF-Token": csrf, "Content-Type": "application/json" },
    data: { email, password: PASSWORD, accept_terms: true, display_name: name, birth_date: "1990-03-14" },
  });
  expect(res.status()).toBe(201);
  await page.request.post(`${API_URL}/api/v1/auth/logout`, { headers: { "X-CSRF-Token": csrf } });
  return { email };
}

async function login(page: Page, email: string, password = PASSWORD) {
  await page.goto("/en/login");
  await page.locator("input[name=email]").fill(email);
  await page.locator("input[name=password]").fill(password);
  await page.locator("form button[type=submit]").click();
}

async function signedIn(page: Page) {
  const { email } = await newAccount(page);
  await login(page, email);
  await page.waitForURL(/\/en\/profile$/);
  return email;
}

const save = (page: Page) => page.getByTestId("settings-save");

/** Axe once entrance animations have finished. */
async function accessible(page: Page, selector: string) {
  await page.evaluate(() => Promise.all(document.getAnimations().filter((a) => a.effect?.getTiming().iterations !== Infinity).map((a) => a.finished.catch(() => undefined))));
  await expectNoA11yViolations(page, [selector]);
}

async function openSettings(page: Page) {
  await page.goto("/en/profile/settings");
  await expect(page.getByTestId("account-settings")).toBeVisible();
}

/** The mobile layout moves "Save Changes" into a bar that appears once something changed. */
async function clickSave(page: Page) {
  const patch = page.waitForResponse((r) => r.url().endsWith("/api/v1/me/settings") && r.request().method() === "PATCH");
  await save(page).click();
  return patch;
}

test.describe("account settings", { tag: "@responsive" }, () => {
  test("round trip: one PATCH with the changed fields, persisted across reloads", async ({ page }) => {
    await signedIn(page);
    await openSettings(page);
    await accessible(page, '[data-testid="account-settings"]');
    await expectNoHorizontalScroll(page);

    await page.getByLabel("Full Name").fill("Renamed Tester");
    await page.getByTestId("notify-marketing").click();
    await page.getByTestId("notify-events").click();
    await expect(page.locator("[data-slot=save-status]").first()).toContainText("Unsaved changes");
    const res = await clickSave(page);
    expect(res.status()).toBe(200);
    const body = res.request().postDataJSON();
    expect(body.expected_version).toBeGreaterThan(0);
    expect(body.account).toEqual({ display_name: "Renamed Tester" });
    expect(body.notifications).toEqual({ marketing: true, events: true });
    expect(body.profile).toBeUndefined();
    await expect(page.locator("[data-slot=save-status]").first()).toContainText("All changes saved");

    await page.reload();
    await expect(page.getByLabel("Full Name")).toHaveValue("Renamed Tester");
    await expect(page.getByTestId("notify-marketing")).toHaveAttribute("aria-checked", "true");
  });

  test("birth data: a time needs a birthplace; unknown time clears it", async ({ page }) => {
    await signedIn(page);
    await openSettings(page);
    // A new account has a date and no time, so "I don't know my time" starts ticked.
    await expect(page.getByLabel("I don't know my time")).toBeChecked();
    await page.getByLabel("I don't know my time").click();
    await page.getByLabel("Time of birth").fill("08:45");
    await save(page).click();
    await expect(page.getByText("Choose where you were born so we can place your rising sign.")).toBeVisible();
    await page.getByRole("combobox", { name: "Location" }).fill("Paris");
    await page.getByRole("option", { name: /^Paris, (.+, )?France$/ }).first().click();
    const res = await clickSave(page);
    expect(res.status()).toBe(200);
    expect(res.request().postDataJSON().profile).toMatchObject({ birth_time: "08:45", birth_place_id: expect.stringMatching(/^gn:/) });
    await page.getByLabel("I don't know my time").click();
    const again = await clickSave(page);
    expect(again.request().postDataJSON().profile).toEqual({ birth_time: "" });
  });

  test("a stale version shows the conflict choice", async ({ page, browser }) => {
    const email = await signedIn(page);
    await openSettings(page);
    // A second device saves first.
    const other = await browser.newContext();
    const second = await other.newPage();
    await login(second, email);
    await second.waitForURL(/\/en\/profile$/);
    await openSettings(second);
    await second.getByTestId("notify-events").click();
    await clickSave(second);
    await other.close();

    await page.getByTestId("notify-chart").click();
    await save(page).click();
    const dialog = page.getByTestId("conflict-dialog");
    await expect(dialog).toBeVisible();
    await dialog.getByTestId("conflict-reload").click();
    await expect(dialog).toBeHidden();
    await expect(page.getByTestId("notify-events")).toHaveAttribute("aria-checked", "true");
  });

  test("theme: Light Cream is applied app-wide, rendered by the server from sy_theme", async ({ page }) => {
    await signedIn(page);
    await openSettings(page);
    await expect(page.locator("html")).toHaveAttribute("data-theme", "warm-beige");
    await page.getByTestId("theme-select").click();
    await page.getByRole("option", { name: "Light Cream" }).click();
    await clickSave(page);
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light-cream");
    const cookies = await page.context().cookies();
    expect(cookies.find((c) => c.name === "sy_theme")?.value).toBe("light-cream");

    // No flash: the server-rendered HTML already carries the theme.
    const html = await (await page.request.get("/en/psychics", { headers: { cookie: "sy_theme=light-cream" } })).text();
    expect(html).toMatch(/<html[^>]*data-theme="light-cream"/);
    await page.goto("/en/psychics");
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light-cream");
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light-cream");
  });

  test("devices: revoke another device, then sign out of all others", async ({ page, browser }) => {
    const email = await signedIn(page);
    const contexts = [];
    for (let i = 0; i < 2; i++) {
      const ctx = await browser.newContext();
      const p = await ctx.newPage();
      await login(p, email);
      await p.waitForURL(/\/en\/profile$/);
      contexts.push({ ctx, p });
    }
    await openSettings(page);
    const rows = page.getByTestId("device-row");
    await expect(rows.filter({ hasText: "(Current)" })).toHaveCount(1);
    await expect(rows.first()).toBeVisible();
    const before = await rows.count();
    expect(before).toBeGreaterThanOrEqual(3);

    await rows.filter({ hasNotText: "(Current)" }).first().getByRole("button", { name: /^Sign out/ }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Sign out", exact: true }).click();
    await expect(rows).toHaveCount(before - 1);

    await page.getByTestId("sign-out-others").click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Sign out", exact: true }).click();
    await expect(rows).toHaveCount(1);
    for (const { ctx, p } of contexts) {
      const me = await p.request.get(`${API_URL}/api/v1/me`);
      expect([401, 403]).toContain(me.status());
      await ctx.close();
    }
  });

  test("export: request with the password, wait for ready, download the ZIP", async ({ page }) => {
    await signedIn(page);
    await openSettings(page);
    await page.getByTestId("open-export").click();
    const dialog = page.getByTestId("export-dialog");
    await accessible(page, '[data-testid="export-dialog"]');
    await dialog.getByLabel("Confirm your password").fill("wrong-password");
    await dialog.getByTestId("export-request").click();
    await expect(dialog.getByText("The password is incorrect.")).toBeVisible();
    await dialog.getByLabel("Confirm your password").fill(PASSWORD);
    await dialog.getByTestId("export-request").click();
    await expect(dialog.getByTestId("export-row")).toHaveCount(1);
    // The worker polls every minute; the dialog polls the status until it is ready.
    await expect(dialog.getByTestId("export-download")).toBeVisible({ timeout: 100_000 });
    await dialog.getByLabel("Confirm your password").fill(PASSWORD);
    const download = page.waitForEvent("download");
    await dialog.getByTestId("export-download").click();
    expect((await download).suggestedFilename()).toMatch(/^staryield-export-\d{4}-\d{2}-\d{2}\.zip$/);
  });

  test("deactivate, sign in, reactivate", async ({ page }) => {
    const email = await signedIn(page);
    await openSettings(page);
    await page.getByTestId("open-deactivate").click();
    await page.getByLabel("Confirm your password").fill("wrong-password");
    await page.getByTestId("deactivate-confirm").click();
    await expect(page.getByText("The password is incorrect.")).toBeVisible();
    await page.getByLabel("Confirm your password").fill(PASSWORD);
    await page.getByTestId("deactivate-confirm").click();
    await page.waitForURL(/\/en\/login\?notice=deactivated/);
    await expect(page.getByTestId("login-notice")).toBeVisible();

    await login(page, email);
    const offer = page.getByTestId("reactivation-offer");
    await expect(offer).toBeVisible();
    await accessible(page, '[data-testid="reactivation-offer"]');
    await offer.getByRole("button", { name: "Reactivate my account" }).click();
    await page.waitForURL(/\/en\/profile$/);
  });

  test("close permanently, then keep the account from the login offer", async ({ page }) => {
    const email = await signedIn(page);
    await openSettings(page);
    await page.getByTestId("open-close").click();
    await page.getByLabel("Confirm your password").fill(PASSWORD);
    await page.getByTestId("close-confirmation").fill("delete");
    await page.getByTestId("close-confirm").click();
    await expect(page.getByText("Type DELETE to confirm.").first()).toBeVisible();
    await page.getByTestId("close-confirmation").fill("DELETE");
    await page.getByTestId("close-confirm").click();
    await page.waitForURL(/\/en\/login\?notice=closure_scheduled/);

    await login(page, email);
    const offer = page.getByTestId("reactivation-offer");
    await expect(offer).toHaveAttribute("data-kind", "account_closure_pending");
    await offer.getByRole("button", { name: "Keep my account" }).click();
    await page.waitForURL(/\/en\/profile$/);
  });

  test("membership: subscribe with a sandbox card, cancel, resume", async ({ page }) => {
    await signedIn(page);
    await openSettings(page);
    await accessible(page, '[data-testid="settings-billing"]');
    await page.getByTestId("membership-subscribe").click();
    const dialog = page.getByTestId("subscribe-dialog");
    await dialog.getByLabel("Cardholder name").fill("Settings Tester");
    await dialog.getByLabel("Card number").fill("4242424242424242");
    await dialog.getByLabel("Expiry date").fill("1239");
    await dialog.getByLabel("CVV").fill("123");
    // The card number never leaves the browser: only the sandbox token is posted.
    const posted: string[] = [];
    page.on("request", (r) => r.method() !== "GET" && posted.push(r.postData() ?? ""));
    await dialog.getByTestId("subscribe-consent").click();
    await dialog.getByTestId("subscribe-confirm").click();
    await expect(dialog).toBeHidden();
    expect(posted.join("\n")).toContain("sbx_a_visa_4242_12_2039");
    expect(posted.join("\n")).not.toContain("4242424242424242");
    await expect(page.getByTestId("membership-status")).toContainText("Renews on");
    await expect(page.getByTestId("payment-method")).toContainText("Visa ending in 4242");
    await expect(page.getByTestId("transaction").first()).toContainText("$9.99");

    await page.getByTestId("membership-cancel").click();
    await page.getByTestId("membership-cancel-confirm").click();
    await expect(page.getByTestId("membership-status")).toContainText("Ends on");
    await page.getByTestId("membership-resume").click();
    await expect(page.getByTestId("membership-status")).toContainText("Renews on");

    // A declined sandbox card (…0002) is explained, never charged.
    await page.getByTestId("add-card").click();
    const add = page.getByTestId("add-card-dialog");
    await add.getByLabel("Cardholder name").fill("Settings Tester");
    await add.getByLabel("Card number").fill("4000000000000002");
    await add.getByLabel("Expiry date").fill("1239");
    await add.getByLabel("CVV").fill("123");
    await add.getByTestId("add-card-save").click();
    await expect(add).toBeHidden();
    await expect(page.getByTestId("payment-method")).toHaveCount(2);
  });

  test("push: unsupported and denied browsers get a clear explanation", async ({ page }) => {
    await signedIn(page);
    await page.addInitScript(() => {
      // @ts-expect-error simulate a browser without Push
      delete window.PushManager;
    });
    await openSettings(page);
    await expect(page.getByTestId("push-hint")).toContainText("doesn't support push");
    await expect(page.getByTestId("notify-push")).toBeDisabled();
  });

  test("push: a blocked permission is explained and the switch stays off", async ({ page }) => {
    await signedIn(page);
    await page.addInitScript(() => {
      Object.defineProperty(Notification, "permission", { get: () => "denied" });
    });
    await openSettings(page);
    await expect(page.getByTestId("push-hint")).toContainText("blocked");
    await expect(page.getByTestId("notify-push")).toBeDisabled();
    await expect(page.getByTestId("notify-push")).toHaveAttribute("aria-checked", "false");
  });
});

test.describe("e-mail link pages", { tag: "@responsive" }, () => {
  test("a missing token and an invalid token explain what to do, and the URL is cleaned", async ({ page }) => {
    await page.goto("/en/cancel-email-change");
    await expect(page.getByText("This link is incomplete")).toBeVisible();
    await page.goto("/en/confirm-email?token=not-a-real-token");
    await expect.poll(() => new URL(page.url()).search).toBe("");
    await accessible(page, '[data-testid="email-link"]');
    await page.getByTestId("email-link-action").click();
    await expect(page.getByText("This link has expired")).toBeVisible();
    expect(await page.locator('meta[name="referrer"]').getAttribute("content")).toBe("no-referrer");
  });

  test("e-mail change end to end through the mailed link", async ({ page }) => {
    const log = process.env.E2E_MAIL_LOG;
    test.skip(!log, "needs the API log (E2E_MAIL_LOG) to read the log mail driver's links");
    const email = await signedIn(page);
    const next = unique();
    // Links already in the log belong to earlier runs (or the other project): wait for a new one.
    const links = () => readFileSync(log!, "utf8").match(/\/en\/confirm-email\?token=[A-Za-z0-9_-]+/g) ?? [];
    const before = links().length;
    await openSettings(page);
    await page.getByTestId("change-email").click();
    const dialog = page.getByTestId("email-change-dialog");
    await dialog.getByLabel("New email address").fill(next);
    await dialog.getByLabel("Current password").fill(PASSWORD);
    await dialog.getByRole("button", { name: "Send confirmation link" }).click();
    await expect(dialog.getByText(/We sent a confirmation link/)).toBeVisible();
    await dialog.getByRole("button", { name: "Done" }).click();
    await expect(page.getByTestId("pending-email")).toBeVisible();

    await expect.poll(() => links().length, { timeout: 30_000 }).toBeGreaterThan(before);
    const link = links().at(-1)!;
    await page.goto(link);
    await page.getByTestId("email-link-action").click();
    await expect(page.getByText("Your email address is updated")).toBeVisible();
    await login(page, next);
    await page.waitForURL(/\/en\/profile$/);
    expect(email).not.toBe(next);
  });
});
