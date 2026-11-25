/**
 * growth: lifecycle and onboarding. The client's "Get started" checklist on
 * the Account Overview, against the real API: a fresh client sees the steps
 * that are open with the right progress, a step completed elsewhere is checked
 * off, hiding it comes with an undo, the choice is kept across visits and
 * every open tab follows at once (onboarding.updated over the WebSocket).
 * Advisors never see it.
 *
 * The steps listed depend on the deployment: the e-mail step needs e-mail
 * verification and the invitation step the referral program, so the spec
 * reads the expected steps from GET /me/onboarding instead of assuming four.
 */
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { API_URL, advisor, expectNoHorizontalScroll, newClient, signedInPage } from "./helpers";

type Checklist = { steps: { key: string; done: boolean }[]; completed: number; total: number; dismissed: boolean };

async function csrf(page: Page): Promise<string> {
  return ((await (await page.request.get(`${API_URL}/api/v1/auth/csrf`)).json()) as { csrf_token: string }).csrf_token;
}

async function checklistFromApi(page: Page): Promise<Checklist> {
  const res = await page.request.get(`${API_URL}/api/v1/me/onboarding`);
  expect(res.status(), await res.text()).toBe(200);
  return (await res.json()) as Checklist;
}

function card(page: Page) {
  return page.getByTestId("onboarding-checklist");
}

test.describe("Get started checklist", () => {
  test("a fresh client sees the open steps with their progress and where each is done", async ({ page }) => {
    await newClient(page, "onboarding");
    await page.goto("/en/profile");
    const api = await checklistFromApi(page);
    expect(api.dismissed).toBe(false);
    expect(api.steps.map((s) => s.key)).toEqual(expect.arrayContaining(["birth_details", "first_reading"]));

    await expect(card(page)).toHaveAttribute("data-state", "ready");
    await expect(card(page).getByRole("heading", { name: "Get started" })).toBeVisible();
    const progress = `${api.completed} of ${api.total} done`;
    await expect(page.getByTestId("onboarding-progress-text")).toHaveText(progress);
    await expect(page.getByRole("progressbar")).toHaveAttribute("aria-valuetext", progress);
    await expect(page.getByTestId("onboarding-step")).toHaveCount(api.total);
    // Registration asks for the date of birth only: the place is still missing.
    await expect(page.getByTestId("onboarding-cta-birth_details")).toHaveAttribute("href", "/en/profile/settings#account");
    await expect(page.getByTestId("onboarding-cta-first_reading")).toHaveAttribute("href", "/en/psychics");
    await expectNoA11yViolations(page, ["[data-testid=onboarding-checklist]"]);

    // Adding the birth place completes the step (the account changed elsewhere: a new visit shows it).
    const token = await csrf(page);
    const saved = await page.request.patch(`${API_URL}/api/v1/me/profile`, {
      headers: { "X-CSRF-Token": token, "Content-Type": "application/json" },
      data: { birth_place_id: "gn:2988507", birth_time: "07:45" },
    });
    expect(saved.status(), await saved.text()).toBe(200);
    await page.reload();
    await expect(page.getByTestId("onboarding-progress-text")).toHaveText(`${api.completed + 1} of ${api.total} done`);
    const birth = page.locator("[data-testid=onboarding-step][data-step=birth_details]");
    await expect(birth).toHaveAttribute("data-done", "true");
    await expect(birth.getByRole("link")).toHaveCount(0);
    await expect(birth).toContainText("Birth details added");

    // The step's call to action goes where the step is done.
    await page.getByTestId("onboarding-cta-first_reading").click();
    await page.waitForURL(/\/en\/psychics/);
  });

  test("hide with undo, kept across visits and followed by every open tab", async ({ page, context }) => {
    await newClient(page, "onboarding-hide");
    await page.goto("/en/profile");
    await expect(card(page)).toHaveAttribute("data-state", "ready");
    const other = await context.newPage();
    await other.goto("/en/profile");
    await expect(card(other)).toHaveAttribute("data-state", "ready");

    await page.getByRole("button", { name: "Hide the checklist" }).click();
    await expect(card(page)).toHaveCount(0);
    await expect(page.getByText("Checklist hidden.")).toBeVisible();
    // The other tab follows without a reload.
    await expect(card(other)).toHaveCount(0);
    expect((await checklistFromApi(page)).dismissed).toBe(true);

    await page.getByRole("button", { name: "Undo" }).click();
    await expect(card(page)).toHaveAttribute("data-state", "ready");
    await expect(page.getByText("Your checklist is back.")).toBeVisible();
    await expect(card(other)).toHaveAttribute("data-state", "ready");

    // Hidden for good: a later visit does not show it again.
    await page.getByRole("button", { name: "Hide the checklist" }).click();
    await expect(card(page)).toHaveCount(0);
    await page.reload();
    await expect(page.getByTestId("client-overview")).toBeVisible();
    await expect(card(page)).toHaveCount(0);
    await other.close();
  });

  test("fits a phone screen @responsive", async ({ page }) => {
    await newClient(page, "onboarding-phone");
    await page.goto("/en/profile");
    await expect(card(page)).toHaveAttribute("data-state", "ready");
    await expect(page.getByTestId("onboarding-cta-first_reading")).toBeVisible();
    await expectNoHorizontalScroll(page);
  });

  test("advisors never see it", async ({ browser }) => {
    const { context, page } = await signedInPage(browser, advisor("theo").email, advisor("theo").password);
    await page.goto("/en/profile");
    await expect(page.locator("main")).toBeVisible();
    await expect(card(page)).toHaveCount(0);
    const res = await page.request.get(`${API_URL}/api/v1/me/onboarding`);
    expect(res.status()).toBe(403);
    await context.close();
  });
});
