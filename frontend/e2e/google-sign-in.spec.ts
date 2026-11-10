/**
 * Sign in / sign up with Google (docs/platform-extensions.md §23.1), against
 * the real API and a mock OpenID issuer (./mock-google.ts) instead of Google.
 *
 * The API under test must be configured to use the mock issuer:
 *
 *   STARYIELD_GOOGLE_ISSUER=http://127.0.0.1:19090        (E2E_GOOGLE_MOCK_PORT, default 19090)
 *   STARYIELD_GOOGLE_CLIENT_ID=e2e-client
 *   STARYIELD_GOOGLE_CLIENT_SECRET=e2e-secret
 *   STARYIELD_GOOGLE_REDIRECT_URL={E2E_API_URL}/api/v1/auth/google/callback
 *
 * The whole file is skipped (with a message) when `GET {E2E_API_URL}/api/v1/site/config`
 * does not report `auth.google: true`. The mock issuer is started in `beforeAll`.
 * Every journey uses fresh Google identities and e-mail addresses.
 */
import { expect, test, type Browser, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { API_URL, signIn } from "./helpers";
import { MockGoogle } from "./mock-google";

const PASSWORD = "E2e!Secure#2026x";
const PORT = Number(process.env.E2E_GOOGLE_MOCK_PORT ?? 19090);
const mock = new MockGoogle({ port: PORT, clientId: "e2e-client", clientSecret: "e2e-secret" });
const rand = () => Math.random().toString(36).slice(2, 9);
const unique = (tag: string) => `e2e+${tag}-${Date.now()}-${rand()}@staryield.test`;
const identity = (email: string) => ({ sub: `e2e-sub-${Date.now()}-${rand()}`, email, email_verified: true });

let configured = false;

test.beforeAll(async () => {
  try {
    const res = await fetch(`${API_URL}/api/v1/site/config`);
    const body = (await res.json()) as { auth?: { google?: boolean } };
    configured = body.auth?.google === true;
  } catch {
    configured = false;
  }
  await mock.start();
});

test.afterAll(async () => {
  await mock.stop();
});

test.beforeEach(() => {
  test.skip(
    !configured,
    `Google sign-in is not enabled on ${API_URL} (site config has no auth.google). Start the API with STARYIELD_GOOGLE_ISSUER=${mock.issuer}, STARYIELD_GOOGLE_CLIENT_ID=e2e-client, STARYIELD_GOOGLE_CLIENT_SECRET=e2e-secret and STARYIELD_GOOGLE_REDIRECT_URL=${API_URL}/api/v1/auth/google/callback.`,
  );
});

/** The signed-in account according to the API (cookies of this page's context). */
async function me(page: Page): Promise<{ email: string; role: string } | null> {
  const res = await page.request.get(`${API_URL}/api/v1/me`);
  return res.ok() ? ((await res.json()) as { email: string; role: string }) : null;
}

/** A password account registered through the API (signed out afterwards). */
async function registerPasswordAccount(page: Page, email: string) {
  const csrf = ((await (await page.request.get(`${API_URL}/api/v1/auth/csrf`)).json()) as { csrf_token: string }).csrf_token;
  const res = await page.request.post(`${API_URL}/api/v1/auth/register`, {
    headers: { "X-CSRF-Token": csrf, "Content-Type": "application/json" },
    data: { email, password: PASSWORD, accept_terms: true, display_name: "Google Tester", birth_date: "1990-03-14" },
  });
  expect(res.status()).toBe(201);
  // Registering rotates the CSRF token: sign out with the new one.
  const session = (await res.json()) as { csrf_token: string };
  const out = await page.request.post(`${API_URL}/api/v1/auth/logout`, { headers: { "X-CSRF-Token": session.csrf_token } });
  expect(out.status()).toBe(204);
}

/** Sign up through "Sign up with Google" in a throw-away context, so the account exists afterwards. */
async function createGoogleAccount(browser: Browser, who: ReturnType<typeof identity>) {
  const context = await browser.newContext({ locale: "en-US" });
  const page = await context.newPage();
  mock.use(who);
  await page.goto("/en/signup");
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: "Sign up with Google" }).click();
  await page.waitForURL(/\/en\/signup\/birth$/);
  expect((await me(page))?.email).toBe(who.email);
  await context.close();
}

const googleButton = (page: Page, name: "Continue with Google" | "Sign up with Google") => page.getByRole("button", { name });

test.describe("Google sign-in", () => {
  test("sign-up from /signup needs the terms, then lands on the birth step signed in", async ({ page }) => {
    const who = identity(unique("gsignup"));
    mock.use(who);
    await page.goto("/en/signup");
    const button = googleButton(page, "Sign up with Google");
    await expect(button).toBeVisible();
    await expect(page.getByTestId("auth-divider")).toBeVisible();
    await expectNoA11yViolations(page, ["main"]);

    // Same validation message as the e-mail path; nothing is sent to Google.
    await button.click();
    await expect(page.getByText("Please accept the Terms of Service and Privacy Policy.")).toBeVisible();
    expect(page.url()).toContain("/en/signup");

    await page.getByRole("checkbox").check();
    await button.click();
    await page.waitForURL(/\/en\/signup\/birth$/);
    expect(page.url()).not.toContain("result=");
    expect(await me(page)).toMatchObject({ email: who.email, role: "client" });

    // What the API asked Google for.
    expect(mock.lastAuthorize).toMatchObject({
      response_type: "code",
      scope: "openid email",
      code_challenge_method: "S256",
      prompt: "select_account",
      client_id: "e2e-client",
    });
    expect(mock.lastAuthorize?.state).toBeTruthy();
    expect(mock.lastAuthorize?.nonce).toBeTruthy();
  });

  test("sign-in from /login for that account", async ({ browser }) => {
    const who = identity(unique("gsignin"));
    await createGoogleAccount(browser, who);

    const context = await browser.newContext({ locale: "en-US" });
    const page = await context.newPage();
    mock.use(who);
    await page.goto("/en/login");
    await googleButton(page, "Continue with Google").click();
    await page.waitForURL(/\/en\/profile$/);
    expect(await me(page)).toMatchObject({ email: who.email });
    await context.close();
  });

  test("sign-in from /login returns to ?next", async ({ browser }) => {
    const who = identity(unique("gnext"));
    await createGoogleAccount(browser, who);

    const context = await browser.newContext({ locale: "en-US" });
    const page = await context.newPage();
    mock.use(who);
    await page.goto("/en/login?next=%2Fen%2Fprofile%2Fsettings");
    await googleButton(page, "Continue with Google").click();
    await page.waitForURL(/\/en\/profile\/settings/);
    await context.close();
  });

  test("a brand-new identity from /login goes through the consent screen to an account", async ({ page }) => {
    const who = identity(unique("gconsent"));
    mock.use(who);
    await page.goto("/en/login");
    await googleButton(page, "Continue with Google").click();

    await expect(page.getByRole("heading", { name: "Create your Staryield account" })).toBeVisible();
    await expect(page.getByTestId("google-consent-email")).toContainText(who.email);
    expect(page.url()).not.toContain("result=");
    await expectNoA11yViolations(page, ["main"]);

    await page.getByRole("button", { name: "Create account" }).click();
    await expect(page.getByText("Please accept the Terms of Service and Privacy Policy.")).toBeVisible();
    await page.getByRole("checkbox").check();
    await page.getByRole("button", { name: "Create account" }).click();
    await page.waitForURL(/\/en\/signup\/birth$/);
    expect(await me(page)).toMatchObject({ email: who.email, role: "client" });
  });

  test("an e-mail that already has a password account: notice, password sign-in connects Google, Google works afterwards", async ({ page, browser }) => {
    const email = unique("glink");
    await registerPasswordAccount(page, email);
    const who = identity(email);
    mock.use(who);

    await page.goto("/en/login");
    await googleButton(page, "Continue with Google").click();
    await page.waitForURL(/\/en\/login/);
    await expect(page.getByTestId("login-notice")).toContainText(
      "You already have a Staryield account with this e-mail. Sign in with your password once to connect Google.",
    );
    await expect(page.locator("input[name=email]")).toHaveValue(email);
    expect(await me(page)).toBeNull();

    await page.locator("input[name=password]").fill(PASSWORD);
    await page.locator("form button[type=submit]").click();
    await expect(page.getByText("Google is now connected.")).toBeVisible();
    await page.waitForURL(/\/en\/profile$/);

    const context = await browser.newContext({ locale: "en-US" });
    const again = await context.newPage();
    mock.use(who);
    await again.goto("/en/login");
    await googleButton(again, "Continue with Google").click();
    await again.waitForURL(/\/en\/profile$/);
    expect(await me(again)).toMatchObject({ email });
    await context.close();
  });

  test("cancelling at Google shows the error and Try again restarts the sign-in", async ({ page }) => {
    const who = identity(unique("gcancel"));
    mock.use(who);
    mock.denyNext();
    await page.goto("/en/login");
    await googleButton(page, "Continue with Google").click();

    await expect(page.getByTestId("google-return")).toHaveAttribute("data-code", "google_cancelled");
    await expect(page.getByText("Google sign-in was cancelled. Nothing was changed.")).toBeVisible();
    await expect(page.getByRole("link", { name: "Back to sign in" })).toBeVisible();
    expect(page.url()).not.toContain("result=");
    await expectNoA11yViolations(page, ["main"]);

    await page.getByTestId("google-retry").click();
    // A new identity: the retry comes back at the consent screen.
    await expect(page.getByRole("heading", { name: "Create your Staryield account" })).toBeVisible();
  });

  test("settings: connect and disconnect Google", async ({ page }) => {
    const email = unique("gsettings");
    await registerPasswordAccount(page, email);
    await signIn(page, email, PASSWORD);

    await page.goto("/en/profile/settings");
    const methods = page.getByTestId("sign-in-methods");
    await expect(methods).toBeVisible();
    await expect(methods.getByTestId("method-password")).toContainText("Set");
    await expect(methods.getByTestId("connect-google")).toBeVisible();

    const googleEmail = unique("gsettings-google");
    mock.use(identity(googleEmail));
    await methods.getByTestId("connect-google").click();
    await page.waitForURL(/\/en\/profile\/settings/);
    expect(page.url()).not.toContain("result=");
    await expect(page.getByText("Google is now connected.").first()).toBeVisible();
    await expect(page.getByTestId("google-email")).toHaveText(`Connected as ${googleEmail}`);

    await page.getByTestId("disconnect-google").click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Confirm your password").fill("Wrong!Pass1x");
    await dialog.getByTestId("disconnect-google-confirm").click();
    await expect(dialog.getByText("The password is incorrect.")).toBeVisible();

    await dialog.getByLabel("Confirm your password").fill(PASSWORD);
    await dialog.getByTestId("disconnect-google-confirm").click();
    await expect(page.getByText("Google has been disconnected.").first()).toBeVisible();
    await expect(page.getByTestId("connect-google")).toBeVisible();
    await expect(page.getByTestId("google-email")).toHaveCount(0);
  });
});
