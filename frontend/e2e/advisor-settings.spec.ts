/**
 * Advisor Settings (revisions 2.0, Figma 1219:1284): contact number, About,
 * Modalities, the profile photo sent for review and the Customs offering.
 * Desktop and phone. Every test restores Solomon's settings afterwards, so
 * the other journeys see the seed data.
 */
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { API_URL, advisor, expectImageLoaded, expectNoHorizontalScroll, pngFixture, signIn } from "./helpers";

const SOLOMON = advisor("solomon");

type Settings = { version: number; phone: string; about: string[]; modalities: string[] };

/** Calls the API with the page's session (CSRF token from the API). */
async function apiCall<T>(page: Page, method: "GET" | "PATCH" | "PUT" | "DELETE", path: string, data?: unknown): Promise<{ status: number; body: T }> {
  const headers: Record<string, string> = { Origin: new URL(page.url()).origin };
  if (method !== "GET") {
    const csrf = (await (await page.request.get(`${API_URL}/api/v1/auth/csrf`, { headers })).json()) as { csrf_token: string };
    headers["X-CSRF-Token"] = csrf.csrf_token;
  }
  const res = await page.request.fetch(`${API_URL}${path}`, { method, headers, data });
  const text = await res.text();
  return { status: res.status(), body: (text ? JSON.parse(text) : undefined) as T };
}

/** The visible "Save Changes": the header button on desktop, the save bar on phones. */
function saveButton(page: Page, mobile: boolean) {
  return page.getByTestId(mobile ? "settings-save-bar" : "settings-save");
}

async function openSettings(page: Page) {
  await signIn(page, SOLOMON.email, SOLOMON.password);
  await page.goto("/en/profile/settings");
  const settings = page.getByTestId("psychic-settings");
  await expect(settings.getByRole("heading", { level: 1, name: "Profile Settings" })).toBeVisible();
  await expect(page.getByTestId("advisor-profile-form")).toBeVisible();
  await expect(page.getByTestId("customs-form")).toBeVisible();
  await expect(page.getByTestId("profile-photo")).toBeVisible();
  return settings;
}

test.describe("advisor settings @responsive", () => {
  test("edits contact, About and Modalities with validation, persistence and the unsaved-changes guard", async ({ page }, info) => {
    const mobile = info.project.name === "mobile";
    await openSettings(page);
    const original = (await apiCall<Settings>(page, "GET", "/api/v1/psychic/settings")).body;
    try {
      const phone = page.getByLabel("Phone number");
      await expect(phone).toHaveValue(original.phone);
      await expect(saveButton(page, mobile)).toHaveCount(mobile ? 0 : 1);

      // Invalid phone: nothing is sent, the field explains why and gets focus.
      await phone.fill("12345");
      await saveButton(page, mobile).click();
      await expect(page.getByText(/international format, e\.g\. \+14155552671/).first()).toBeVisible();
      await expect(phone).toBeFocused();

      await phone.fill("+1 415 555 2671");
      const about = page.getByLabel("About section");
      await about.fill("E2E: reads palms and auras.\n\nTwenty years of practice.");
      const modality = page.getByRole("textbox", { name: "Modalities" });
      await modality.fill("E2E Aura");
      await modality.press("Enter");
      await expect(page.getByTestId("advisor-profile-form").getByText("E2E Aura", { exact: true })).toBeVisible();

      // Leaving with unsaved changes asks first.
      page.once("dialog", (dialog) => void dialog.dismiss());
      await page.getByRole("link", { name: /Analytics|Back to Dashboard/ }).filter({ visible: true }).first().click();
      await expect(page).toHaveURL(/\/en\/profile\/settings$/);

      const [response] = await Promise.all([
        page.waitForResponse((r) => r.url().endsWith("/api/v1/psychic/settings") && r.request().method() === "PATCH", { timeout: 20_000 }),
        saveButton(page, mobile).click({ timeout: 20_000 }),
      ]);
      expect(response.status()).toBe(200);
      const body = response.request().postDataJSON() as Record<string, unknown>;
      expect(body).toMatchObject({ phone: "+14155552671", expected_version: original.version });
      expect(body.modalities).toEqual([...original.modalities, "E2E Aura"]);
      await expect(page.getByText("Your changes were saved.")).toBeVisible();

      // Saved on the server and shown on the public profile.
      await page.reload();
      await expect(page.getByLabel("Phone number")).toHaveValue("+14155552671");
      await expect(page.getByLabel("About section")).toHaveValue("E2E: reads palms and auras.\n\nTwenty years of practice.");
      await expectNoA11yViolations(page, ["[data-testid=psychic-settings]"]);
      await expectNoHorizontalScroll(page);

      await page.goto(`/en/psychics/${SOLOMON.slug}`);
      await expect(page.getByText("E2E: reads palms and auras.")).toBeVisible();
      await expect(page.getByText("E2E Aura").first()).toBeVisible();
      await expect(page.getByText("+14155552671")).toHaveCount(0);
    } finally {
      await apiCall(page, "PATCH", "/api/v1/psychic/settings", { phone: original.phone, about: original.about, modalities: original.modalities });
    }
  });

  test("creates the Customs offering, lists it on the profile and removes it", async ({ page }, info) => {
    const mobile = info.project.name === "mobile";
    await openSettings(page);
    try {
      const card = page.getByTestId("customs-card");
      await expect(card.getByTestId("customs-empty")).toBeVisible();
      await card.getByLabel("Custom name").fill("E2E Soulmate sketch");
      await card.getByLabel("Custom rate").fill("5");
      await card.getByLabel("Average delivery time").fill("8");
      await saveButton(page, mobile).click();
      await expect(card.getByText("Use a price between 10 and 50,000 credits.")).toBeVisible();

      await card.getByLabel("Custom rate").fill("2000");
      const saved = page.waitForResponse((r) => r.url().endsWith("/api/v1/psychic/service") && r.request().method() === "PUT");
      await saveButton(page, mobile).click();
      expect((await saved).status()).toBe(201);
      await expect(page.getByText("Your custom offering is now on your profile.")).toBeVisible();
      await expect(card.getByRole("button", { name: "Remove the custom offering" })).toBeVisible();
      await expectNoA11yViolations(page, ["[data-testid=customs-card]"]);

      await page.goto(`/en/psychics/${SOLOMON.slug}`);
      const offering = page.locator("article", { hasText: "E2E Soulmate sketch" });
      await expect(offering).toBeVisible();
      await expect(offering.getByTestId("service-delivery")).toHaveText("Custom request · delivered in about 8 days");
      await expect(offering).toContainText("2,000");

      await page.goto("/en/profile/settings");
      await page.getByTestId("customs-card").getByRole("button", { name: "Remove the custom offering" }).click();
      const dialog = page.getByRole("alertdialog");
      await expect(dialog).toContainText("Remove “E2E Soulmate sketch”?");
      await dialog.getByRole("button", { name: "Remove offering" }).click();
      await expect(page.getByTestId("customs-card").getByTestId("customs-empty")).toBeVisible();
      await page.goto(`/en/psychics/${SOLOMON.slug}`);
      await expect(page.locator("article", { hasText: "E2E Soulmate sketch" })).toHaveCount(0);
    } finally {
      await apiCall(page, "DELETE", "/api/v1/psychic/service").catch(() => undefined);
    }
  });

  test("crops a photo, sends it for review privately and withdraws it", async ({ page }) => {
    await openSettings(page);
    const before = (await apiCall<{ image_url: string }>(page, "GET", "/api/v1/psychic/photo")).body.image_url;
    try {
      const section = page.getByTestId("profile-photo");
      await section.getByTestId("photo-input").setInputFiles({ name: "portrait.png", mimeType: "image/png", buffer: pngFixture(600, 800) });
      const dialog = page.getByRole("dialog", { name: "Crop your photo" });
      await expect(dialog).toBeVisible();
      await expect(dialog.getByText(/upper half of the image/)).toBeVisible();
      const viewport = dialog.getByTestId("photo-crop-viewport");
      await viewport.focus();
      await viewport.press("ArrowDown");
      await dialog.getByLabel("Zoom").press("End"); // zoom in as far as the minimum size allows
      await expectNoA11yViolations(page, ["[data-testid=photo-crop-dialog]"]);

      const uploaded = page.waitForResponse((r) => r.url().endsWith("/api/v1/psychic/photo") && r.request().method() === "POST");
      await dialog.getByTestId("photo-crop-confirm").click();
      expect((await uploaded).status()).toBe(201);
      await expect(dialog).toBeHidden();
      await expect(section.getByText("Pending review")).toBeVisible();
      await expectImageLoaded(page, "[data-testid=profile-photo] [data-testid=photo-image]");
      await expect(section.getByTestId("photo-image")).toHaveAttribute("src", /^blob:/);

      // Cropped to a square in the browser; private until approved.
      const state = (await apiCall<{ image_url: string; submission: { width: number; height: number; status: string } }>(page, "GET", "/api/v1/psychic/photo")).body;
      expect(state.submission.status).toBe("pending");
      expect(state.submission.width).toBe(state.submission.height);
      expect(state.image_url).toBe(before);
      const profile = (await apiCall<{ image_url: string }>(page, "GET", `/api/v1/psychics/${SOLOMON.slug}`)).body;
      expect(profile.image_url).toBe(before);

      await section.getByRole("button", { name: "Withdraw" }).click();
      await expect(page.getByText("Photo withdrawn.")).toBeVisible();
      await expect(section.getByText("Pending review")).toHaveCount(0);
      await expectNoHorizontalScroll(page);
    } finally {
      const current = (await apiCall<{ submission: { status: string } | null }>(page, "GET", "/api/v1/psychic/photo")).body;
      if (current.submission?.status === "pending") await apiCall(page, "DELETE", "/api/v1/psychic/photo");
    }
  });

  test("refuses files that are not photos before cropping", async ({ page }) => {
    await openSettings(page);
    const input = page.getByTestId("profile-photo").getByTestId("photo-input");
    await input.setInputFiles({ name: "notes.txt", mimeType: "text/plain", buffer: Buffer.from("hello") });
    await expect(page.getByText("Use a PNG, JPEG or WebP image.")).toBeVisible();
    await input.setInputFiles({ name: "tiny.png", mimeType: "image/png", buffer: pngFixture(120, 120) });
    const dialog = page.getByRole("dialog", { name: "Crop your photo" });
    await expect(dialog.getByText("Use a photo at least 256 pixels wide and 256 pixels high.")).toBeVisible();
    await expect(dialog.getByTestId("photo-crop-confirm")).toBeDisabled();
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(dialog).toBeHidden();
  });
});
