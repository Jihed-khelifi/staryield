import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";

import { API_URL, CLIENT, accountTrigger, signIn } from "./helpers";

test.describe("short links and campaign attribution", () => {
  test("/go/:code hands valid codes to the API click tracker and sends malformed ones home", async ({ request, baseURL }) => {
    const valid = await request.get("/go/e2eLink42", { maxRedirects: 0 });
    expect(valid.status()).toBe(302);
    expect(valid.headers()["location"]).toBe(`${API_URL}/l/e2eLink42`);

    const invalid = await request.get("/go/x!", { maxRedirects: 0 });
    expect(invalid.status()).toBe(302);
    expect(invalid.headers()["location"]).toBe(`${baseURL}/`);
  });

  test("unknown short codes end on the home page", async ({ page }) => {
    await page.goto("/go/zz-unknown-e2e");
    await expect(page).toHaveURL(/\/(en|fr|es)?\/?$/);
  });

  test("UTM landing keeps the first touch, strips sy_click and is sent with sign-up", async ({ page }) => {
    const click = randomUUID();
    await page.goto(`/en/psychics?utm_source=e2e&utm_medium=test&utm_campaign=e2e-spring&utm_content=hero&sy_click=${click}`);

    // sy_click (and only it) is removed from the address bar.
    await expect.poll(() => new URL(page.url()).searchParams.has("sy_click")).toBe(false);
    expect(new URL(page.url()).searchParams.get("utm_campaign")).toBe("e2e-spring");

    const stored = await page.evaluate(() => JSON.parse(localStorage.getItem("staryield.attribution") ?? "null"));
    expect(stored).toMatchObject({ click_id: click, utm_source: "e2e", utm_medium: "test", utm_campaign: "e2e-spring", utm_content: "hero", landing_path: "/en/psychics" });
    expect(Date.parse(stored.first_seen_at)).not.toBeNaN();

    // A later landing does not replace the first touch.
    await page.goto("/en?utm_source=other&utm_campaign=other");
    expect(await page.evaluate(() => JSON.parse(localStorage.getItem("staryield.attribution")!).utm_campaign)).toBe("e2e-spring");

    // Sign up: the register request carries the attribution.
    const email = `e2e+${Date.now()}@staryield.test`;
    await page.goto("/en/signup");
    await page.locator("input[name=email]").fill(email);
    await page.getByRole("checkbox").check();
    await page.locator("form button[type=submit]").click();
    await page.waitForURL(/\/en\/signup\/password$/);
    await page.locator("input[name=password]").fill("E2e!Secure#2026x");
    await page.locator("input[name=confirm]").fill("E2e!Secure#2026x");
    const register = page.waitForRequest((r) => r.url().endsWith("/api/v1/auth/register") && r.method() === "POST");
    await page.locator("form button[type=submit]").click();
    const body = (await register).postDataJSON();
    expect(body.email).toBe(email);
    expect(body.attribution).toMatchObject({ click_id: click, utm_source: "e2e", utm_campaign: "e2e-spring", landing_path: "/en/psychics" });
    await page.waitForURL(/\/en\/signup\/birth$/);
  });
});

test.describe("sign in and sign out", () => {
  test("desktop: the avatar menu signs out to the locale home", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await accountTrigger(page).click();
    const menu = page.getByRole("menu");
    await expect(menu.getByRole("menuitem", { name: "Profile" })).toBeVisible();
    await expect(menu.getByRole("menuitem", { name: "Settings" })).toBeVisible();
    await expect(menu.getByRole("menuitem", { name: "Analytics" })).toHaveCount(0); // advisors only
    await menu.getByRole("menuitem", { name: "Sign out" }).click();
    await page.waitForURL(/\/en$/);
    await expect(page.getByRole("link", { name: "Log In" }).first()).toBeVisible();
    // The session is really gone.
    await page.goto("/en/profile");
    await page.waitForURL(/\/en\/login\?next=/);
  });

  test("settings and account sidebar offer sign out", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto("/en/profile/settings");
    await expect(page.locator(".account-sidebar").getByRole("button", { name: "Sign out" })).toBeVisible();
    await page.getByTestId("settings-sign-out").click();
    await page.waitForURL(/\/en$/);
  });

  test("phones: the menu sheet signs out", async ({ browser }) => {
    const context = await browser.newContext({ viewport: { width: 375, height: 812 }, isMobile: true, hasTouch: true });
    const page = await context.newPage();
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto("/en/chatroom");
    await accountTrigger(page).click();
    const sheet = page.getByRole("dialog");
    await sheet.getByRole("button", { name: "Sign out" }).click();
    await page.waitForURL(/\/en$/);
    await context.close();
  });

  test("French locale: logout and menu copy are translated", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password, "fr");
    await page.goto("/fr/profile");
    await accountTrigger(page).click();
    await expect(page.getByRole("menuitem", { name: "Se déconnecter" })).toBeVisible();
    await page.keyboard.press("Escape");
  });
});
