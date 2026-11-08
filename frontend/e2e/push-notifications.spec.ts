/**
 * Turning push on (docs §21.3): Brave's blocked push service explained with
 * a copyable address, a working subscription stored by the API, the advisor
 * prompt, and the web app manifest. The browser's push service is faked in an
 * init script (no real FCM in CI); the API call is real.
 */
import { createECDH, randomBytes } from "node:crypto";
import { expect, test, type Browser, type Page } from "@playwright/test";

import { API_URL, advisor, expectNoHorizontalScroll, signIn } from "./helpers";

const PASSWORD = "E2e!Secure#2026x";
const unique = () => `e2e+push-${Date.now()}-${Math.random().toString(36).slice(2, 7)}@staryield.test`;
const b64url = (buf: Buffer) => buf.toString("base64url");

/** A fresh client account, registered through the API and signed in through the form. */
async function signedInClient(page: Page) {
  const email = unique();
  const csrf = (await (await page.request.get(`${API_URL}/api/v1/auth/csrf`)).json()).csrf_token as string;
  const res = await page.request.post(`${API_URL}/api/v1/auth/register`, {
    headers: { "X-CSRF-Token": csrf, "Content-Type": "application/json" },
    data: { email, password: PASSWORD, accept_terms: true, display_name: "Push Tester", birth_date: "1990-03-14" },
  });
  expect(res.status()).toBe(201);
  await page.request.post(`${API_URL}/api/v1/auth/logout`, { headers: { "X-CSRF-Token": csrf } });
  await signIn(page, email, PASSWORD);
}

async function context(browser: Browser, opts: { granted: boolean; viewport?: { width: number; height: number } }) {
  const ctx = await browser.newContext({ viewport: opts.viewport ?? { width: 1280, height: 900 }, locale: "en-US" });
  const origin = new URL(process.env.E2E_BASE_URL ?? "http://localhost:3000").origin;
  if (opts.granted) await ctx.grantPermissions(["notifications"], { origin });
  // This headless Chromium reports "denied" even after grantPermissions, so the state is pinned:
  // "granted" for sign-up, "default" for the advisor prompt.
  const state = opts.granted ? "granted" : "default";
  await ctx.addInitScript((value) => Object.defineProperty(Notification, "permission", { configurable: true, get: () => value }), state);
  return ctx;
}

/** Brave: defines navigator.brave and a push service that rejects subscribe(). */
const BRAVE_ABORT = () => {
  Object.defineProperty(navigator, "brave", { configurable: true, value: { isBrave: () => Promise.resolve(true) } });
  PushManager.prototype.subscribe = () => Promise.reject(new DOMException("Registration failed - push service error", "AbortError"));
};

/** A push service that answers with a valid subscription (real P-256 key, 16-byte auth). */
const FAKE_SERVICE = (sub: { endpoint: string; p256dh: string; auth: string }) => {
  PushManager.prototype.subscribe = () =>
    Promise.resolve({
      endpoint: sub.endpoint,
      options: {},
      toJSON: () => ({ endpoint: sub.endpoint, expirationTime: null, keys: { p256dh: sub.p256dh, auth: sub.auth } }),
      unsubscribe: () => Promise.resolve(true),
    } as unknown as PushSubscription);
};

test.describe("push sign-up", () => {
  test("Brave with Google push services off gets the fix instructions, a Copy button and Try again", async ({ browser }) => {
    const ctx = await context(browser, { granted: true });
    const page = await ctx.newPage();
    await page.addInitScript(BRAVE_ABORT);
    await signedInClient(page);
    await page.goto("/en/profile/settings");
    const toggle = page.getByTestId("notify-push");
    await expect(toggle).toBeEnabled();
    await toggle.click();
    const hint = page.getByTestId("push-hint");
    await expect(hint).toContainText("Use Google services for push messaging", { timeout: 15_000 });
    await expect(page.getByTestId("push-brave-address")).toHaveText("brave://settings/privacy");
    await expect(page.getByTestId("push-brave-copy")).toBeVisible();
    await expect(page.getByTestId("push-retry")).toBeVisible();
    await expect(toggle).not.toBeChecked();
    await ctx.close();
  });

  test("Copy puts brave://settings/privacy on the clipboard", async ({ browser }) => {
    const ctx = await context(browser, { granted: true });
    await ctx.grantPermissions(["notifications", "clipboard-read", "clipboard-write"], { origin: new URL(process.env.E2E_BASE_URL ?? "http://localhost:3000").origin });
    const page = await ctx.newPage();
    await page.addInitScript(BRAVE_ABORT);
    await signedInClient(page);
    await page.goto("/en/profile/settings");
    await page.getByTestId("notify-push").click();
    await page.getByTestId("push-brave-copy").click({ timeout: 15_000 });
    await expect(page.getByTestId("push-brave-copy")).toContainText("Copied");
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe("brave://settings/privacy");
    await ctx.close();
  });

  test("a working push service subscribes: the API stores it and the switch turns on", async ({ browser }) => {
    const ecdh = createECDH("prime256v1");
    ecdh.generateKeys();
    const sub = { endpoint: `https://fcm.googleapis.com/fcm/send/${b64url(randomBytes(24))}`, p256dh: b64url(ecdh.getPublicKey()), auth: b64url(randomBytes(16)) };
    const ctx = await context(browser, { granted: true });
    const page = await ctx.newPage();
    await page.addInitScript(FAKE_SERVICE, sub);
    await signedInClient(page);
    await page.goto("/en/profile/settings");
    const toggle = page.getByTestId("notify-push");
    await expect(toggle).toBeEnabled();
    const stored = page.waitForResponse((res) => res.url().includes("/api/v1/me/push-subscriptions") && res.request().method() === "POST");
    await toggle.click();
    const res = await stored;
    expect(res.status()).toBeLessThan(300);
    expect(res.request().postDataJSON()).toMatchObject({ endpoint: sub.endpoint, keys: { p256dh: sub.p256dh, auth: sub.auth } });
    await expect(toggle).toBeChecked();
    await ctx.close();
  });

  test("the web app manifest is served and linked", async ({ page }) => {
    await page.goto("/en");
    const href = await page.locator("link[rel=manifest]").getAttribute("href");
    expect(href).toBe("/manifest.webmanifest");
    const manifest = await (await page.request.get(href!)).json();
    expect(manifest).toMatchObject({ name: "Staryield", display: "standalone", start_url: "/", scope: "/" });
    expect(manifest.icons.length).toBeGreaterThan(0);
    await expect(page.locator("meta[name=mobile-web-app-capable], meta[name=apple-mobile-web-app-capable]").first()).toHaveCount(1);
  });
});

test.describe("advisor prompt", () => {
  test("theo sees \"Never miss a chat request\" on the overview straight away", async ({ browser }) => {
    const ctx = await context(browser, { granted: false });
    const page = await ctx.newPage();
    const theo = advisor("theo");
    await signIn(page, theo.email, theo.password);
    await page.goto("/en/profile");
    await expect(page.getByText("Never miss a chat request")).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText("Turn on notifications to hear about new chat requests, even when this page is closed.")).toBeVisible();
    await ctx.close();
  });

  test("theo sees the prompt on the overview and Not now dismisses it @responsive", async ({ browser }) => {
    const ctx = await context(browser, { granted: false, viewport: { width: 390, height: 844 } });
    const page = await ctx.newPage();
    const theo = advisor("theo");
    await signIn(page, theo.email, theo.password);
    await page.goto("/en/profile");
    const prompt = page.getByTestId("chat-push-prompt");
    await expect(prompt).toBeVisible({ timeout: 15_000 });
    await expectNoHorizontalScroll(page);
    await prompt.getByRole("button", { name: "Not now" }).click();
    await expect(prompt).toHaveCount(0);
    await page.reload();
    await expect(page.getByTestId("psychic-overview")).toBeVisible();
    await expect(page.getByTestId("chat-push-prompt")).toHaveCount(0);
    await ctx.close();
  });

  test("a client does not see the advisor copy", async ({ browser }) => {
    const ctx = await context(browser, { granted: false });
    const page = await ctx.newPage();
    await signedInClient(page);
    await page.goto("/en/chatroom");
    await expect(page.getByText("Never miss a chat request")).toHaveCount(0);
    await ctx.close();
  });
});
