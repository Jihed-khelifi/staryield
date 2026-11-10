/**
 * Notification hardening (docs/platform-extensions.md §23.6): read state is
 * shared between tabs, an opened push notification is marked read from the
 * `sy_n` URL parameter, and signing out removes this browser's push
 * subscription (the endpoint travels with POST /auth/logout).
 *
 * Needs the API of the same revision (realtime `notification.read`,
 * `POST /auth/logout {push_endpoint}`). Notifications are produced by buying a
 * credit pack through the sandbox payment provider (`purchase.succeeded`).
 * The browser's push service is faked in an init script (no real FCM in CI).
 */
import { createECDH, randomBytes } from "node:crypto";
import { expect, test, type Browser, type Page } from "@playwright/test";

import { API_URL, ensureCredits, signIn } from "./helpers";

const PASSWORD = "E2e!Secure#2026x";
const unique = () => `e2e+notif-${Date.now()}-${Math.random().toString(36).slice(2, 7)}@staryield.test`;
const b64url = (buf: Buffer) => buf.toString("base64url");
const ORIGIN = new URL(process.env.E2E_BASE_URL ?? "http://localhost:3000").origin;

/** A fresh client: registered through the API, signed in through the form. */
async function signedInClient(page: Page) {
  const email = unique();
  const csrf = (await (await page.request.get(`${API_URL}/api/v1/auth/csrf`)).json()).csrf_token as string;
  const res = await page.request.post(`${API_URL}/api/v1/auth/register`, {
    headers: { "X-CSRF-Token": csrf, "Content-Type": "application/json" },
    data: { email, password: PASSWORD, accept_terms: true, display_name: "Notif Tester", birth_date: "1990-03-14" },
  });
  expect(res.status()).toBe(201);
  await page.request.post(`${API_URL}/api/v1/auth/logout`, { headers: { "X-CSRF-Token": csrf } });
  await signIn(page, email, PASSWORD);
}

async function newContext(browser: Browser, opts: { push?: boolean } = {}) {
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 }, locale: "en-US" });
  if (opts.push) {
    await ctx.grantPermissions(["notifications"], { origin: ORIGIN });
    // This headless Chromium reports "denied" even after grantPermissions: pin it.
    await ctx.addInitScript(() => Object.defineProperty(Notification, "permission", { configurable: true, get: () => "granted" }));
  }
  return ctx;
}

type Listed = { items: { id: string; read_at?: string }[] };
const listNotifications = async (page: Page) => (await (await page.context().request.get(`${API_URL}/api/v1/me/notifications?limit=50`)).json()) as Listed;
const bellButton = (page: Page) => page.getByRole("button", { name: /^Notifications/ }).filter({ visible: true }).first();
const badge = (page: Page) => page.getByTestId("notification-badge").filter({ visible: true });

/** A push service that answers with a valid subscription and remembers it (so getSubscription() sees it). */
const STATEFUL_SERVICE = (sub: { endpoint: string; p256dh: string; auth: string }) => {
  let current: PushSubscription | null = null;
  const make = () =>
    ({
      endpoint: sub.endpoint,
      options: {},
      toJSON: () => ({ endpoint: sub.endpoint, expirationTime: null, keys: { p256dh: sub.p256dh, auth: sub.auth } }),
      unsubscribe: () => {
        current = null;
        void (window as unknown as { __pushUnsubscribed?: () => void }).__pushUnsubscribed?.();
        return Promise.resolve(true);
      },
    }) as unknown as PushSubscription;
  PushManager.prototype.subscribe = () => Promise.resolve((current = make()));
  PushManager.prototype.getSubscription = () => Promise.resolve(current);
};

test.describe("read state across tabs", () => {
  test("marking all read in one tab clears the badge in the other without a reload", async ({ browser }) => {
    const ctx = await newContext(browser);
    const a = await ctx.newPage();
    await signedInClient(a);
    await a.goto("/en/profile");
    const b = await ctx.newPage();
    await b.goto("/en/profile");
    await expect(bellButton(a)).toBeVisible();
    await expect(bellButton(b)).toBeVisible();

    await ensureCredits(a, 1); // purchase.succeeded -> in-app notification, delivered live to both tabs
    await expect(badge(a)).toBeVisible();
    await expect(badge(b)).toBeVisible();
    await b.evaluate(() => ((window as unknown as { __tab: string }).__tab = "original"));

    await bellButton(a).click();
    const read = a.waitForResponse((r) => r.url().endsWith("/api/v1/me/notifications/read") && r.request().method() === "POST" && r.ok());
    await a.getByRole("button", { name: "Mark all as read" }).click();
    await read;
    await expect(badge(a)).toHaveCount(0);
    await expect(badge(b)).toHaveCount(0);
    // Same document: nothing reloaded.
    expect(await b.evaluate(() => (window as unknown as { __tab: string }).__tab)).toBe("original");

    // The list in the other tab shows the row as read as well.
    await bellButton(b).click();
    await expect(b.locator(".notification-item").first()).toBeVisible();
    await expect(b.locator(".notification-item[data-unread]")).toHaveCount(0);
    await ctx.close();
  });

  test("a notification read in another tab updates the count of a tab that was in the background", async ({ browser }) => {
    const ctx = await newContext(browser);
    const a = await ctx.newPage();
    await signedInClient(a);
    await a.goto("/en/profile");
    await ensureCredits(a, 1);
    await expect(badge(a)).toBeVisible();
    const b = await ctx.newPage();
    await b.goto("/en/profile");
    await expect(badge(b)).toBeVisible();
    const { items } = await listNotifications(a);
    const csrf = (await (await a.context().request.get(`${API_URL}/api/v1/auth/csrf`)).json()).csrf_token as string;
    const res = await a.context().request.post(`${API_URL}/api/v1/me/notifications/read`, { headers: { "X-CSRF-Token": csrf }, data: { ids: items.map((n) => n.id) } });
    expect(res.ok()).toBe(true);
    await expect(badge(a)).toHaveCount(0);
    await expect(badge(b)).toHaveCount(0);
    await ctx.close();
  });
});

test.describe("opening a push notification (sy_n)", () => {
  test("marks that notification read and removes the parameter, keeping the rest of the URL", async ({ browser }) => {
    const ctx = await newContext(browser);
    const page = await ctx.newPage();
    await signedInClient(page);
    await ensureCredits(page, 1);
    // The purchase.succeeded notification is written asynchronously (about 250 ms after the purchase request returns).
    let unread: Listed["items"][number] | undefined;
    await expect
      .poll(async () => (unread = (await listNotifications(page)).items.find((n) => !n.read_at)), { message: "the purchase left an unread notification" })
      .toBeTruthy();

    const marked = page.waitForRequest((r) => r.url().endsWith("/api/v1/me/notifications/read") && r.method() === "POST");
    await page.goto(`/en/profile?sy_n=${unread!.id}`);
    expect((await marked).postDataJSON()).toEqual({ ids: [unread!.id] });
    await expect.poll(() => new URL(page.url()).searchParams.has("sy_n")).toBe(false);
    expect(new URL(page.url()).pathname).toBe("/en/profile");
    await expect(badge(page)).toHaveCount(0);
    await expect.poll(async () => (await listNotifications(page)).items.find((n) => n.id === unread!.id)?.read_at).toBeTruthy();

    // A reload does not mark anything again.
    let again = 0;
    page.on("request", (r) => r.url().endsWith("/api/v1/me/notifications/read") && r.method() === "POST" && again++);
    await page.reload();
    await expect(bellButton(page)).toBeVisible();
    expect(again).toBe(0);
    await ctx.close();
  });

  test("an unknown or malformed id is stripped without breaking the page", async ({ browser }) => {
    const ctx = await newContext(browser);
    const page = await ctx.newPage();
    await signedInClient(page);
    await page.goto("/en/profile?sy_n=%3Cscript%3E&x=1");
    await expect.poll(() => new URL(page.url()).search).toBe("?x=1");
    await expect(bellButton(page)).toBeVisible();
    await ctx.close();
  });
});

test.describe("signing out removes this browser's push subscription", () => {
  test("the logout request carries push_endpoint and the browser subscription is removed", async ({ browser }) => {
    const ecdh = createECDH("prime256v1");
    ecdh.generateKeys();
    const sub = { endpoint: `https://fcm.googleapis.com/fcm/send/${b64url(randomBytes(24))}`, p256dh: b64url(ecdh.getPublicKey()), auth: b64url(randomBytes(16)) };
    const ctx = await newContext(browser, { push: true });
    const page = await ctx.newPage();
    let unsubscribed = 0;
    await page.exposeFunction("__pushUnsubscribed", () => void unsubscribed++);
    await page.addInitScript(STATEFUL_SERVICE, sub);
    await signedInClient(page);
    await page.goto("/en/profile/settings");
    const toggle = page.getByTestId("notify-push");
    await expect(toggle).toBeEnabled();
    const stored = page.waitForResponse((r) => r.url().includes("/api/v1/me/push-subscriptions") && r.request().method() === "POST");
    await toggle.click();
    expect((await stored).status()).toBeLessThan(300);
    await expect(toggle).toBeChecked();

    const logout = page.waitForRequest((r) => r.url().endsWith("/api/v1/auth/logout") && r.method() === "POST");
    await page.getByTestId("settings-sign-out").click();
    expect((await logout).postDataJSON()).toEqual({ push_endpoint: sub.endpoint });
    await page.waitForURL((url) => url.pathname === "/en");
    expect(unsubscribed).toBe(1);
    await ctx.close();
  });

  test("without a subscription the logout request has no body and sign-out still works", async ({ browser }) => {
    const ctx = await newContext(browser);
    const page = await ctx.newPage();
    await signedInClient(page);
    await page.goto("/en/profile/settings");
    const logout = page.waitForRequest((r) => r.url().endsWith("/api/v1/auth/logout") && r.method() === "POST");
    await page.getByTestId("settings-sign-out").click();
    expect((await logout).postData() ?? "").toBe("");
    await page.waitForURL((url) => url.pathname === "/en");
    await ctx.close();
  });

  test("sign-out still succeeds when the logout request fails", async ({ browser }) => {
    const ctx = await newContext(browser);
    const page = await ctx.newPage();
    await signedInClient(page);
    await page.goto("/en/profile/settings");
    await page.route("**/api/v1/auth/logout", (route) => route.abort("failed"));
    await page.getByTestId("settings-sign-out").click();
    await page.waitForURL((url) => url.pathname === "/en");
    await ctx.close();
  });
});
