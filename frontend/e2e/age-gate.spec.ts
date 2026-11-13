/**
 * Revisions 5.0: the 18+ age gate (docs §24.6). There is no age pop-up: the age
 * comes from the date of birth given at sign-up and the account is gated
 * accordingly.
 *
 *  - sign up, skip the birth step by opening the chat room → the birth step
 *    returns, then the date takes the person to the chat room;
 *  - an under-age date → the calm "Account restricted" screen, for good (no
 *    retry with another date, every gated page leads back to it);
 *  - signing in without a date lands on the birth step;
 *  - public pages stay open to everyone and never show a pop-up.
 *
 * Every journey signs up a fresh account, so the shared seed accounts are not
 * touched. Runs on desktop and on the mobile project (@responsive).
 */
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { expectNoHorizontalScroll } from "./helpers";

const PASSWORD = "E2e!Secure#2026x";
const unique = (tag: string) => `e2e+${tag}-${Date.now()}-${Math.random().toString(36).slice(2, 7)}@staryield.test`;
const submit = (page: Page) => page.locator("form button[type=submit]");

/** Sign up with e-mail and password and stop at the birth step. */
async function signUpToBirthStep(page: Page, tag: string): Promise<string> {
  const email = unique(tag);
  await page.goto("/en/signup");
  await page.locator("input[name=email]").fill(email);
  await page.getByRole("checkbox").check();
  await submit(page).click();
  await page.waitForURL(/\/en\/signup\/password$/);
  await page.locator("input[name=password]").fill(PASSWORD);
  await page.locator("input[name=confirm]").fill(PASSWORD);
  await submit(page).click();
  await page.waitForURL(/\/en\/signup\/birth$/);
  await expect(page.getByTestId("birth-info-form")).toBeVisible();
  return email;
}

/** Enter a date of birth (no time) and submit the birth step. */
async function enterBirthDate(page: Page, date: { day: string; month: string; year: string }) {
  const form = page.getByTestId("birth-info-form");
  // The dev server may not have hydrated the form yet: retry until the control reacts.
  const unknownTime = form.getByRole("checkbox", { name: "I don't know my time" });
  await expect(async () => {
    if ((await unknownTime.getAttribute("aria-checked")) !== "true") await unknownTime.click();
    expect(await unknownTime.getAttribute("aria-checked")).toBe("true");
  }).toPass({ timeout: 20_000 });
  await form.getByRole("textbox", { name: /Day$/ }).fill(date.day);
  await form.getByRole("textbox", { name: "Month", exact: true }).fill(date.month);
  await form.getByRole("textbox", { name: "Year", exact: true }).fill(date.year);
  await submit(page).click();
}

const chatroom = (page: Page) => page.locator("main.chat-workspace, [data-testid=chat-header]").filter({ visible: true }).first();

test.describe("18+ age gate", { tag: "@responsive" }, () => {
  test("skipping the birth step is not possible: the chat room sends the person back, then on to the chat room", async ({ page }) => {
    await signUpToBirthStep(page, "agegate");
    // The birth step says who Staryield is for; there is no pop-up anywhere.
    await expect(page.getByTestId("birth-age-notice")).toHaveText(
      "Staryield is for adults 18 and over. Your date of birth confirms your eligibility and powers your chart.",
    );
    await expect(page.getByRole("dialog")).toHaveCount(0);

    // Skip it: the gated page refuses and returns to the step, remembering where the person was heading.
    await page.goto("/en/chatroom");
    await page.waitForURL((url) => url.pathname === "/en/signup/birth" && url.searchParams.get("next") === "/en/chatroom");
    await expect(page.getByTestId("birth-info-form")).toBeVisible();
    await expect(page.getByTestId("age-restricted")).toHaveCount(0);

    // An adult date confirms the account and leads straight to the chat room.
    const patch = page.waitForResponse((r) => r.url().endsWith("/api/v1/me/profile") && r.request().method() === "PATCH");
    await enterBirthDate(page, { day: "14", month: "03", year: "1990" });
    expect((await patch).status()).toBe(200);
    await page.waitForURL(/\/en\/chatroom$/);
    await expect(chatroom(page)).toBeVisible();

    // The gate stays open on later visits and for the other gated pages.
    await page.goto("/en/credits");
    await expect(page).toHaveURL(/\/en\/credits$/);
    await page.goto("/en/chatroom");
    await expect(page).toHaveURL(/\/en\/chatroom$/);
    await expectNoHorizontalScroll(page);
  });

  test("an under-age date restricts the account: the restricted screen, no retry with another date", async ({ page }) => {
    await signUpToBirthStep(page, "agekid");
    const year = String(new Date().getFullYear() - 12);
    const refused = page.waitForResponse((r) => r.url().endsWith("/api/v1/me/profile") && r.request().method() === "PATCH");
    await enterBirthDate(page, { day: "02", month: "01", year });
    expect((await refused).status()).toBe(422);

    // The calm full-page screen, with the three ways forward.
    await page.waitForURL(/\/en\/age-restricted$/);
    const screen = page.getByTestId("age-restricted");
    await expect(screen).toBeVisible();
    await expect(screen.getByRole("heading", { name: "Account restricted" })).toBeVisible();
    await expect(screen.getByText("Staryield is for adults 18 and over.", { exact: false }).first()).toBeVisible();
    await expect(screen.getByTestId("age-restricted-support")).toHaveAttribute("href", "/en/contact");
    await expect(screen.getByTestId("age-restricted-close")).toHaveAttribute("href", "/en/profile/settings#account-actions");
    await expect(screen.getByTestId("age-restricted-privacy")).toHaveAttribute("href", "/en/privacy");
    await expect(screen.locator("input")).toHaveCount(0);
    await expectNoA11yViolations(page, ['[data-testid="age-restricted"]']);
    await expectNoHorizontalScroll(page);

    // No retry: the birth step and every gated page lead back to the screen.
    await page.goto("/en/signup/birth");
    await page.waitForURL(/\/en\/age-restricted$/);
    await page.goto("/en/chatroom");
    await page.waitForURL(/\/en\/age-restricted$/);
    await page.goto("/en/credits");
    await page.waitForURL(/\/en\/age-restricted$/);
    await expect(page.getByTestId("age-restricted")).toBeVisible();

    // A restricted account cannot change the date through the API either.
    const retry = await page.evaluate(async (apiUrl) => {
      const csrf = (await (await fetch(`${apiUrl}/api/v1/auth/csrf`, { credentials: "include" })).json()) as { csrf_token: string };
      const res = await fetch(`${apiUrl}/api/v1/me/profile`, {
        method: "PATCH",
        credentials: "include",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf.csrf_token },
        body: JSON.stringify({ birth_date: "1990-03-14" }),
      });
      return { status: res.status, code: ((await res.json()) as { error: { code: string } }).error.code };
    }, process.env.E2E_API_URL ?? "http://localhost:8080");
    expect(retry).toEqual({ status: 403, code: "age_restricted" });
  });

  test("signing in without a date of birth lands on the birth step, then on where the person was going", async ({ page, browser }) => {
    const email = await signUpToBirthStep(page, "agelogin");
    // A new browser: only the account knows it has no date of birth.
    const fresh = await browser.newContext({ viewport: page.viewportSize() ?? undefined, locale: "en-US" });
    const login = await fresh.newPage();
    await login.goto(`/en/login?next=${encodeURIComponent("/en/credits")}`);
    await login.locator("input[name=email]").fill(email);
    await login.locator("input[name=password]").fill(PASSWORD);
    await submit(login).click();
    await login.waitForURL((url) => url.pathname === "/en/signup/birth" && url.searchParams.get("next") === "/en/credits");
    await enterBirthDate(login, { day: "05", month: "06", year: "1985" });
    await login.waitForURL(/\/en\/credits$/);
    await fresh.close();
  });

  test("public pages stay open to everyone, say who they are for, and never show an age pop-up", async ({ page }) => {
    for (const path of ["/en", "/en/about", "/en/faq", "/en/calculators/natal-chart", "/en/terms"]) {
      await page.goto(path);
      await expect(page).toHaveURL(new RegExp(`${path}$`));
      await expect(page.getByTestId("footer-age-line")).toHaveText("18+ only · For entertainment purposes only.");
      await expect(page.getByRole("dialog")).toHaveCount(0);
      await expect(page.getByRole("alertdialog")).toHaveCount(0);
    }
  });
});
