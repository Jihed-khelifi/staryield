/**
 * Revisions 2.0 forms (react-hook-form + yup): sign-up through birth info,
 * sign-in errors and lock-out, forgot/reset password, the contact form with
 * its ticket reference, and the custom-request order sheet. Runs on desktop
 * and on the mobile project (@responsive); every journey uses fresh e-mails
 * so the shared seed accounts are never locked or changed.
 */
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { CLIENT, expectNoHorizontalScroll, signIn } from "./helpers";

const PASSWORD = "E2e!Secure#2026x";
const unique = (tag: string) => `e2e+${tag}-${Date.now()}-${Math.random().toString(36).slice(2, 7)}@staryield.test`;
const submit = (page: Page) => page.locator("form button[type=submit]");

/** axe after entrance animations (opacity fades) finish, so it measures final colours. */
async function expectAccessible(page: Page, selector: string) {
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((animation) => animation.effect?.getTiming().iterations !== Infinity)
        .map((animation) => animation.finished.catch(() => undefined))
    )
  );
  await expectNoA11yViolations(page, [selector]);
}

/** YYYY-MM-DD in the browser's time zone, `offset` days from today. */
const localDay = (page: Page, offset = 0) =>
  page.evaluate((days) => {
    const d = new Date();
    d.setDate(d.getDate() + days);
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
  }, offset);

test.describe("sign-up", { tag: "@responsive" }, () => {
  test("email → password → birth info with time and place → offer", async ({ page }) => {
    await page.goto("/en/signup");
    await submit(page).click();
    await expect(page.getByText("Enter your email address.")).toBeVisible();
    await expect(page.getByText("Please accept the Terms of Service and Privacy Policy.")).toBeVisible();
    await expect(page.locator("input[name=email]")).toHaveAttribute("aria-invalid", "true");
    await page.locator("input[name=email]").fill(unique("signup"));
    await page.getByRole("checkbox").check();
    await submit(page).click();
    await page.waitForURL(/\/en\/signup\/password$/);

    // Live password rules, then a mismatch.
    const rules = page.getByRole("list", { name: "Password requirements" });
    await page.locator("input[name=password]").fill("weakpass");
    await expect(rules.locator("li[data-met]")).toHaveCount(1);
    await page.locator("input[name=password]").fill(PASSWORD);
    await expect(rules.locator("li[data-met]")).toHaveCount(4);
    await page.locator("input[name=confirm]").fill(`${PASSWORD}?`);
    await submit(page).click();
    await expect(page.getByText("The passwords do not match.")).toBeVisible();
    await page.locator("input[name=confirm]").fill(PASSWORD);
    await submit(page).click();
    await page.waitForURL(/\/en\/signup\/birth$/);

    // Birth information: a time needs a birthplace.
    const form = page.getByTestId("birth-info-form");
    await expect(form.getByText("Your birth time and exact location stay private. Advisors you chat with see your date of birth, which readings rely on.")).toBeVisible();
    await form.getByRole("textbox", { name: /Day$/ }).fill("14");
    await form.getByRole("textbox", { name: "Month", exact: true }).fill("03");
    await form.getByRole("textbox", { name: "Year", exact: true }).fill("1990");
    await form.getByRole("textbox", { name: "Time", exact: true }).fill("08:45");
    await submit(page).click();
    await expect(form.getByText("Choose where you were born so we can place your rising sign.")).toBeVisible();
    await expectAccessible(page, '[data-testid="birth-info-form"]');

    await form.getByRole("combobox").fill("Paris");
    await page.getByRole("option", { name: /^Paris, (.+, )?France$/ }).first().click();
    const patch = page.waitForResponse((r) => r.url().endsWith("/api/v1/me/profile") && r.request().method() === "PATCH");
    await submit(page).click();
    const response = await patch;
    expect(response.status()).toBe(200);
    const body = response.request().postDataJSON();
    expect(body).toMatchObject({ birth_date: "1990-03-14", birth_time: "08:45" });
    expect(body.birth_place_id).toBeTruthy();
    await page.waitForURL(/\/en\/signup\/offer$/);
    await expect(page.getByTestId("offer-step")).toBeVisible();
    await expectNoHorizontalScroll(page);
  });

  test("an e-mail that already has an account offers to log in", async ({ page }) => {
    await page.goto("/en/signup");
    await page.locator("input[name=email]").fill(CLIENT.email);
    await page.getByRole("checkbox").check();
    await submit(page).click();
    await page.waitForURL(/\/en\/signup\/password$/);
    await page.locator("input[name=password]").fill(PASSWORD);
    await page.locator("input[name=confirm]").fill(PASSWORD);
    await submit(page).click();
    const alert = page.getByTestId("signup-password-form").getByRole("alert");
    await expect(alert).toContainText("An account with this email already exists.");
    await alert.getByRole("link", { name: "Log In" }).click();
    await page.waitForURL(/\/en\/login$/);
  });
});

test.describe("log in", { tag: "@responsive" }, () => {
  test("validation, wrong credentials and a counted-down lock-out", async ({ page }) => {
    await page.goto("/en/login");
    const form = page.getByTestId("login-form");
    await submit(page).click();
    await expect(form.getByText("Enter your email address.")).toBeVisible();
    await expect(form.getByText("Enter your password.")).toBeVisible();

    // A fresh address: lock-outs count failures per e-mail, known or not.
    await page.locator("input[name=email]").fill(unique("locked"));
    for (let attempt = 0; attempt < 5; attempt++) {
      await page.locator("input[name=password]").fill(`Wrong-pass-${attempt}`);
      const res = page.waitForResponse((r) => r.url().endsWith("/api/v1/auth/login"));
      await submit(page).click();
      expect((await res).status()).toBe(401);
      await expect(form.getByRole("alert")).toContainText("The email or password is incorrect.");
      await expect(page.locator("input[name=password]")).toHaveValue("");
    }
    await expect(page.locator("input[name=email]")).not.toHaveAttribute("aria-invalid", "true");

    await page.locator("input[name=password]").fill("Wrong-pass-5");
    const locked = page.waitForResponse((r) => r.url().endsWith("/api/v1/auth/login"));
    await submit(page).click();
    expect((await locked).status()).toBe(429);
    await expect(form.getByRole("alert")).toContainText("Too many sign-in attempts. Please try again later.");
    await expect(form.getByRole("timer")).toHaveText(/^Try again in \d+s$/);
    await expect(submit(page)).toBeDisabled();
    await expectAccessible(page, '[data-testid="login-form"]');
  });
});

test.describe("password reset", { tag: "@responsive" }, () => {
  test("forgot password answers the same way for any address", async ({ page }) => {
    await page.goto("/en/forgot-password");
    await page.locator("input[name=email]").fill("not-an-email");
    await submit(page).click();
    await expect(page.getByText("Enter a valid email address.")).toBeVisible();
    await page.locator("input[name=email]").fill(unique("forgot"));
    await submit(page).click();
    await expect(page.getByText("If an account exists for this email, a reset link is on its way.")).toBeVisible();
    await expect(submit(page)).toHaveText("Send again");
  });

  test("reset links without a token or with a bad one ask for a new link", async ({ page }) => {
    await page.goto("/en/reset-password");
    const form = page.getByTestId("reset-password-form");
    await expect(form.getByRole("alert")).toContainText("This link is invalid or has expired.");
    await expect(submit(page)).toBeDisabled();
    await form.getByRole("link", { name: "Request a new link" }).click();
    await page.waitForURL(/\/en\/forgot-password$/);

    await page.goto("/en/reset-password?token=e2e-not-a-real-token");
    await page.locator("input[name=password]").fill(PASSWORD);
    await page.locator("input[name=confirm]").fill(PASSWORD);
    await submit(page).click();
    await expect(form.getByRole("alert")).toContainText("This link is invalid or has expired.");
    await expect(form.getByRole("link", { name: "Request a new link" })).toHaveAttribute("href", "/en/forgot-password");
    await expectAccessible(page, '[data-testid="reset-password-form"]');
  });
});

test.describe("contact form", { tag: "@responsive" }, () => {
  test("validates, preselects ?topic= and shows the ticket reference", async ({ page }) => {
    await page.goto("/en/contact?topic=privacy");
    const form = page.getByTestId("contact-form");
    await expect(form.getByRole("combobox", { name: "Topic" })).toContainText("Privacy & my data");
    await form.locator("button[type=submit]").click();
    await expect(form.getByText("Enter your email address.")).toBeVisible();
    await expect(form.getByText("Please confirm to continue.")).toBeVisible();
    await expect(form.getByRole("textbox", { name: "Subject" })).toHaveAttribute("aria-invalid", "true");
    await expectAccessible(page, '[data-testid="contact-form"]');

    const email = unique("contact");
    await form.getByRole("textbox", { name: "Subject" }).fill("Export my data");
    await form.getByRole("textbox", { name: "Email" }).fill(email);
    await form.getByRole("textbox", { name: "Describe your issue" }).fill("Please send me a copy of my personal data.");
    await form.getByRole("checkbox").check();
    const created = page.waitForResponse((r) => r.url().endsWith("/api/v1/support/tickets") && r.request().method() === "POST");
    await form.locator("button[type=submit]").click();
    const response = await created;
    expect(response.status()).toBe(201);
    const { reference } = await response.json();
    expect(reference).toMatch(/^SY-[A-HJ-NP-Z2-9]{6}$/);
    expect(response.request().postDataJSON()).toMatchObject({ topic: "privacy", email, consent: true });

    const success = page.getByTestId("contact-success");
    await expect(success.getByTestId("contact-reference")).toHaveText(reference);
    await expect(success).toContainText(`We will reply to ${email}.`);
    await expectAccessible(page, '[data-testid="contact-success"]');
    await expectNoHorizontalScroll(page);
  });
});

test.describe("custom request order sheet", { tag: "@responsive" }, () => {
  test("framed as ordering a service; the time needs a date; past dates and long notes are refused", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto("/en/psychics/theo");
    await expect(page.getByRole("button", { name: /Book Session/ })).toHaveCount(0);
    await page.getByRole("button", { name: /Order service/ }).first().click();
    const form = page.getByRole("dialog").getByTestId("order-form");
    await expect(form.getByRole("button", { name: "Send request" })).toBeVisible();
    await expect(form.getByTestId("order-how-it-works")).toContainText("Your credits are held until it is delivered.");
    const date = form.getByLabel("Preferred date");
    const time = form.getByLabel("Preferred time");
    await expect(date).toHaveAttribute("min", await localDay(page));

    let ordered = false;
    page.on("request", (r) => {
      if (r.url().endsWith("/api/v1/custom-requests") && r.method() === "POST") ordered = true;
    });
    // The time stays disabled until a date is picked, with a helper that does not move the row.
    await expect(time).toBeDisabled();
    await expect(form.getByText("Pick a date first.")).toBeVisible();
    await date.fill(await localDay(page, -1));
    await form.getByRole("button", { name: "Send request" }).click();
    await expect(form.getByText("Choose today or a later date.")).toBeVisible();
    await date.fill(await localDay(page, 3));
    await expect(time).toBeEnabled();
    await expect(form.getByText("Pick a date first.")).toHaveCount(0);
    await time.fill("18:30");
    await form.getByRole("textbox", { name: "What would you like to explore?" }).fill("x".repeat(2001));
    await form.getByRole("button", { name: "Send request" }).click();
    await expect(form.getByText("Use at most 2000 characters.")).toBeVisible();
    await expect(form.getByText("2001/2000")).toBeVisible();
    expect(ordered).toBe(false);
    await expectAccessible(page, '[data-testid="order-form"]');
  });

  test("preferred date and time share one row on desktop and stack on phones, aligned at the top", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto("/en/psychics/theo");
    await page.getByRole("button", { name: /Order service/ }).first().click();
    const form = page.getByRole("dialog").getByTestId("order-form");
    const date = form.getByLabel("Preferred date");
    const time = form.getByLabel("Preferred time");
    const [dateBox, timeBox] = [await date.boundingBox(), await time.boundingBox()];
    expect(dateBox && timeBox).toBeTruthy();
    // Same size either way.
    expect(Math.abs(dateBox!.height - timeBox!.height)).toBeLessThanOrEqual(1);
    expect(Math.abs(dateBox!.width - timeBox!.width)).toBeLessThanOrEqual(1);
    const stacked = (page.viewportSize()?.width ?? 1440) < 640;
    if (stacked) {
      expect(timeBox!.y).toBeGreaterThan(dateBox!.y + dateBox!.height - 1);
    } else {
      // One row: same top edge, and the label lines match too.
      expect(Math.abs(dateBox!.y - timeBox!.y)).toBeLessThanOrEqual(1);
      expect(timeBox!.x).toBeGreaterThan(dateBox!.x + dateBox!.width - 1);
      const labels = [await form.locator("label", { hasText: "Preferred date" }).boundingBox(), await form.locator("label", { hasText: "Preferred time" }).boundingBox()];
      expect(Math.abs(labels[0]!.y - labels[1]!.y)).toBeLessThanOrEqual(1);
    }
    // Picking a date (which hides the helper) does not move the inputs.
    await date.fill(await localDay(page, 2));
    const after = await time.boundingBox();
    expect(Math.abs(after!.y - timeBox!.y)).toBeLessThanOrEqual(1);
    expect(Math.abs((await date.boundingBox())!.y - dateBox!.y)).toBeLessThanOrEqual(1);
  });
});
