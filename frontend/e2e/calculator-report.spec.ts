/**
 * The calculators' e-mail report (revisions 6.0, §26.4), against the real API:
 * the "Save your reading" dialog opens by itself once a result has been on
 * screen (or 60 % was scrolled), never on the sample route; the button reopens
 * it; the e-mail is queued; the marketing box starts ticked only where the API
 * says so; errors have a way out; the unsubscribe page. Desktop and phone
 * (@responsive).
 *
 * The stack's mailer is not readable from a browser, so the unsubscribe
 * success path uses a stubbed answer and the real API answers the invalid
 * link; the whole e-mail → link → unsubscribe flow is covered by the API's e2e suite
 * (backend/tests/e2e/calculator_report_test.go).
 */
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { expectNoHorizontalScroll } from "./helpers";

const PATH_OF_LIFE = "/en/calculators/path-of-life#v=1&d=1990-11-29";
const REPORT_API = "**/api/v1/calculators/report-email";
const CONFIG_API = "**/api/v1/calculators/report-email/config";

let counter = 0;
const address = () => `e2e-report-${Date.now()}-${++counter}@example.com`;
const dialog = (page: Page) => page.getByTestId("report-email-dialog");

/**
 * The page remembers the dialog was shown n days ago (null: never). Seeded once per tab: an init script
 * runs again on every reload, and would wipe what the page itself remembered since (a dismissal).
 */
async function shownDaysAgo(page: Page, kind: string, days: number | null) {
  await page.addInitScript(
    ([k, d]) => {
      const seeded = `e2e.seeded.${k}`;
      if (window.sessionStorage.getItem(seeded)) return;
      window.sessionStorage.setItem(seeded, "1");
      if (d === null) window.localStorage.removeItem(`staryield.report-email.${k}`);
      else window.localStorage.setItem(`staryield.report-email.${k}`, String(Date.now() - Number(d) * 86_400_000));
    },
    [kind, days] as const,
  );
}

async function scrollToShare(page: Page, share: number) {
  await page.evaluate((s) => {
    const max = document.documentElement.scrollHeight - window.innerHeight;
    window.scrollTo(0, Math.ceil(max * s));
  }, share);
}

test.describe("report e-mail dialog", { tag: "@responsive" }, () => {
  test("opens by itself after 6 seconds, sends the report, and the button reopens it", async ({ page }) => {
    await shownDaysAgo(page, "path-of-life", null);
    await page.goto(PATH_OF_LIFE);
    await expect(page.getByTestId("calculator-result")).toBeVisible();
    await expect(page.getByTestId("email-report-button")).toBeVisible();
    await expect(dialog(page)).toHaveCount(0);

    // Opens by itself within a few seconds of the result being on screen.
    await expect(dialog(page)).toBeVisible({ timeout: 12_000 });
    await expect(dialog(page).getByRole("heading", { name: "Save your reading" })).toBeVisible();
    await expect(page.getByTestId("report-email-marketing")).toHaveAttribute("aria-checked", /true|false/);
    await expectNoA11yViolations(page, ['[data-testid="report-email-dialog"]']);
    await expectNoHorizontalScroll(page);

    // Validation first, then a real request: 202 and the confirmation with the address.
    await page.getByTestId("report-email-submit").click();
    await expect(dialog(page).getByText("Enter your email address.")).toBeVisible();
    const to = address();
    await page.getByTestId("report-email-input").fill(to);
    const queued = page.waitForResponse((r) => r.url().endsWith("/api/v1/calculators/report-email") && r.request().method() === "POST");
    await page.getByTestId("report-email-submit").click();
    const response = await queued;
    expect(response.status()).toBe(202);
    expect(response.request().postDataJSON()).toMatchObject({
      kind: "path-of-life",
      input: { birth_date: "1990-11-29" },
      email: to,
      locale: "en",
      website: "",
    });
    await expect(page.getByTestId("report-email-success")).toContainText(`Your report is on its way to ${to}.`);
    await page.getByTestId("report-email-close").click();
    await expect(dialog(page)).toHaveCount(0);

    // The result stays on the page and the button reopens the dialog at any time.
    await expect(page.getByTestId("life-path-title")).toBeVisible();
    await page.getByTestId("email-report-button").click();
    await expect(page.getByTestId("report-email-form")).toBeVisible();
    await page.getByTestId("report-email-dismiss").click();
    await expect(dialog(page)).toHaveCount(0);
  });

  test("opens at once when 60 % of the page was scrolled, once per calculator per browser", async ({ page }) => {
    await shownDaysAgo(page, "astrology", null);
    await page.goto("/en/calculators/astrology#v=1&d=1990-05-02");
    await expect(page.getByTestId("sun-sign")).toBeVisible();
    await expect(dialog(page)).toHaveCount(0);
    await scrollToShare(page, 0.3);
    await expect(dialog(page)).toHaveCount(0);
    await scrollToShare(page, 0.7);
    await expect(dialog(page)).toBeVisible({ timeout: 2_000 });
    await page.getByTestId("report-email-dismiss").click();
    await expect(dialog(page)).toHaveCount(0);

    // Dismissed: a reload does not open it again, however far the visitor scrolls ...
    await page.reload();
    await expect(page.getByTestId("sun-sign")).toBeVisible();
    await scrollToShare(page, 0.9);
    await page.waitForTimeout(1_500);
    await expect(dialog(page)).toHaveCount(0);
    // ... the button still does.
    await page.getByTestId("email-report-button").click();
    await expect(dialog(page)).toBeVisible();
  });

  test("comes back 30 days after it was shown", async ({ page }) => {
    await shownDaysAgo(page, "path-of-life", 31);
    await page.goto(PATH_OF_LIFE);
    await scrollToShare(page, 0.8);
    await expect(dialog(page)).toBeVisible({ timeout: 12_000 });
  });

  test("never opens by itself on the sample route, which has no button either", async ({ page }) => {
    await shownDaysAgo(page, "natal-chart", null);
    await page.goto("/en/calculators/natal-chart/results");
    await expect(page.getByTestId("calculator-result")).toBeVisible();
    await scrollToShare(page, 0.9);
    await page.waitForTimeout(7_500);
    await expect(dialog(page)).toHaveCount(0);
    await expect(page.getByTestId("email-report-button")).toHaveCount(0);
  });

  test("the marketing box starts ticked only where the API says so, and is never required", async ({ page }) => {
    await shownDaysAgo(page, "path-of-life", 1);
    // Going to the same URL again only changes the hash (no reload, the open dialog stays): leave the page first.
    const visit = async () => {
      await page.goto("about:blank");
      await page.goto(PATH_OF_LIFE);
      await page.getByTestId("email-report-button").click();
    };
    for (const prechecked of [true, false]) {
      await page.route(CONFIG_API, (route) => route.fulfill({ json: { enabled: true, marketing_prechecked: prechecked } }));
      await visit();
      await expect(page.getByTestId("report-email-marketing")).toHaveAttribute("aria-checked", String(prechecked));
      await page.unroute(CONFIG_API);
    }
    // A config that cannot be read: unticked, and the dialog still works.
    await page.route(CONFIG_API, (route) => route.abort());
    await visit();
    await expect(page.getByTestId("report-email-marketing")).toHaveAttribute("aria-checked", "false");
    await page.unroute(CONFIG_API);
  });

  test("sends what the visitor chose: a ticked box, a pre-ticked box left alone", async ({ page }) => {
    await shownDaysAgo(page, "path-of-life", 1);
    await page.route(CONFIG_API, (route) => route.fulfill({ json: { enabled: true, marketing_prechecked: true } }));
    let body: Record<string, unknown> = {};
    await page.route(REPORT_API, async (route) => {
      body = route.request().postDataJSON();
      await route.fulfill({ status: 202, json: { status: "queued" } });
    });
    await page.goto(PATH_OF_LIFE);
    await page.getByTestId("email-report-button").click();
    await page.getByTestId("report-email-input").fill(address());
    await page.getByTestId("report-email-submit").click();
    await expect(page.getByTestId("report-email-success")).toBeVisible();
    expect(body).toMatchObject({ marketing_opt_in: true, marketing_prechecked: true });
  });

  test("the report is hidden when the API switched it off", async ({ page }) => {
    await shownDaysAgo(page, "path-of-life", null);
    await page.route(CONFIG_API, (route) => route.fulfill({ json: { enabled: false, marketing_prechecked: false } }));
    await page.goto(PATH_OF_LIFE);
    await expect(page.getByTestId("calculator-result")).toBeVisible();
    await scrollToShare(page, 0.9);
    await page.waitForTimeout(7_500);
    await expect(dialog(page)).toHaveCount(0);
    await expect(page.getByTestId("email-report-button")).toHaveCount(0);
  });

  test("errors have a way out: limit, network failure with retry, switched off", async ({ page }) => {
    await shownDaysAgo(page, "path-of-life", 1);
    await page.goto(PATH_OF_LIFE);
    await page.getByTestId("email-report-button").click();
    await page.getByTestId("report-email-input").fill(address());

    await page.route(REPORT_API, (route) => route.fulfill({ status: 429, headers: { "Retry-After": "3600" }, json: { error: { code: "rate_limited", message: "Too many requests." } } }));
    await page.getByTestId("report-email-submit").click();
    await expect(page.getByTestId("report-email-error")).toContainText("Too many requests. Please try again later.");
    await expect(page.getByTestId("report-email-retry")).toHaveCount(0);

    await page.unroute(REPORT_API);
    await page.route(REPORT_API, (route) => route.abort("connectionfailed"));
    await page.getByTestId("report-email-submit").click();
    await expect(page.getByTestId("report-email-error")).toContainText(/couldn't reach Staryield|offline/);
    await page.unroute(REPORT_API);
    await page.route(REPORT_API, (route) => route.fulfill({ status: 202, json: { status: "queued" } }));
    await page.getByTestId("report-email-retry").click();
    await expect(page.getByTestId("report-email-success")).toBeVisible();

    await page.getByTestId("report-email-close").click();
    await page.unroute(REPORT_API);
    await page.route(REPORT_API, (route) => route.fulfill({ status: 503, json: { error: { code: "feature_unavailable", message: "off" } } }));
    await page.getByTestId("email-report-button").click();
    await page.getByTestId("report-email-input").fill(address());
    await page.getByTestId("report-email-submit").click();
    await expect(page.getByTestId("report-email-error")).toContainText("The e-mail report isn't available right now.");
  });

  test("is translated: French", async ({ page }) => {
    await shownDaysAgo(page, "path-of-life", 1);
    await page.goto("/fr/calculators/path-of-life#v=1&d=1990-11-29");
    await page.getByTestId("email-report-button").click();
    await expect(dialog(page).getByRole("heading", { name: "Enregistrez votre lecture" })).toBeVisible();
    await expect(page.getByTestId("report-email-submit")).toContainText("Envoyer mon rapport");
    const body = page.waitForRequest((r) => r.url().endsWith("/api/v1/calculators/report-email") && r.method() === "POST");
    await page.getByTestId("report-email-input").fill(address());
    await page.getByTestId("report-email-submit").click();
    expect((await body).postDataJSON()).toMatchObject({ locale: "fr" });
  });
});

test.describe("unsubscribe page", { tag: "@responsive" }, () => {
  test("confirms, strips the token, and says it is done", async ({ page }) => {
    await page.route("**/api/v1/marketing/unsubscribe", (route) => route.fulfill({ json: { status: "unsubscribed" } }));
    await page.goto("/en/unsubscribe?token=AZhs8Vq0cGJ5xW2mQnT1uA.1W7bN3kqP0dRfX9yLzC4vUe8HsTjMoIaYgDwKcEBhFQ");
    await expect(page.getByRole("heading", { name: "Unsubscribe from Staryield e-mails?" })).toBeVisible();
    await expect.poll(() => new URL(page.url()).search).toBe("");
    await expectNoA11yViolations(page, ['[data-testid="unsubscribe"]']);
    await expectNoHorizontalScroll(page);
    const request = page.waitForRequest((r) => r.url().endsWith("/api/v1/marketing/unsubscribe"));
    await page.getByTestId("unsubscribe-action").click();
    expect((await request).postDataJSON()).toEqual({ token: "AZhs8Vq0cGJ5xW2mQnT1uA.1W7bN3kqP0dRfX9yLzC4vUe8HsTjMoIaYgDwKcEBhFQ" });
    await expect(page.getByRole("heading", { name: "You are unsubscribed" })).toBeVisible();
    await expect(page.getByTestId("unsubscribe-next")).toHaveAttribute("href", "/en");
  });

  test("the real API refuses an unknown token; a missing token and a network failure are explained", async ({ page }) => {
    await page.goto("/en/unsubscribe?token=AAAAAAAAAAAAAAAAAAAAAA.BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB");
    await page.getByTestId("unsubscribe-action").click();
    await expect(page.getByRole("heading", { name: "This link is not valid" })).toBeVisible();
    await expect(page.getByTestId("unsubscribe-action")).toHaveCount(0);

    await page.goto("/en/unsubscribe");
    await expect(page.getByRole("heading", { name: "This link is incomplete" })).toBeVisible();

    await page.route("**/api/v1/marketing/unsubscribe", (route) => route.abort("connectionfailed"));
    await page.goto("/en/unsubscribe?token=AZhs8Vq0cGJ5xW2mQnT1uA.1W7bN3kqP0dRfX9yLzC4vUe8HsTjMoIaYgDwKcEBhFQ");
    await page.getByTestId("unsubscribe-action").click();
    await expect(page.getByRole("heading", { name: "Something went wrong" })).toBeVisible();
    await page.unroute("**/api/v1/marketing/unsubscribe");
    await page.route("**/api/v1/marketing/unsubscribe", (route) => route.fulfill({ json: { status: "unsubscribed" } }));
    await page.getByTestId("unsubscribe-action").click();
    await expect(page.getByRole("heading", { name: "You are unsubscribed" })).toBeVisible();
  });

  test("is translated: Spanish", async ({ page }) => {
    await page.goto("/es/unsubscribe?token=AZhs8Vq0cGJ5xW2mQnT1uA.1W7bN3kqP0dRfX9yLzC4vUe8HsTjMoIaYgDwKcEBhFQ");
    await expect(page.getByRole("heading", { name: "¿Darte de baja de los correos de Staryield?" })).toBeVisible();
  });
});
