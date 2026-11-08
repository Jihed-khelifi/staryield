/**
 * Advisor incoming-request ringtone (docs §21.1) with two browsers.
 * The sound files are intercepted (the bucket's CORS does not allow the e2e
 * origin) and autoplay is allowed so the engine can start without a gesture.
 */
import { expect, test, type BrowserContext, type Page } from "@playwright/test";

import { CLIENT, advisor, setAvailability, signIn, signedInPage, wavFixture } from "./helpers";

const THEO = advisor("theo");

test.use({
  launchOptions: {
    args: ["--autoplay-policy=no-user-gesture-required"],
    ...(process.env.E2E_CHROMIUM ? { executablePath: process.env.E2E_CHROMIUM } : {}),
  },
});

test.describe.serial("advisor ringtone", () => {
  let clientCtx: BrowserContext;
  let psychicCtx: BrowserContext;
  let client: Page;
  let psychic: Page;

  test.beforeAll(async ({ browser }) => {
    const sounds = async (route: import("@playwright/test").Route) =>
      route.fulfill({ status: 200, contentType: "audio/wav", headers: { "access-control-allow-origin": "*" }, body: wavFixture() });
    psychicCtx = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "en-US" });
    await psychicCtx.route("https://assets.staryield.net/assets/sounds/**", sounds);
    psychic = await psychicCtx.newPage();
    await signIn(psychic, THEO.email, THEO.password);
    ({ context: clientCtx, page: client } = await signedInPage(browser, CLIENT.email, CLIENT.password));
  });

  test.afterAll(async () => {
    await psychic.goto("/en/profile").catch(() => undefined);
    await setAvailability(psychic, "offline").catch(() => undefined);
    await clientCtx?.close();
    await psychicCtx?.close();
  });

  async function request() {
    await client.goto(`/en/chatroom?reader=${THEO.slug}`);
    const start = client.getByRole("button", { name: /Start live chat/ });
    await expect(start).toBeEnabled();
    await start.click();
    await expect(client.getByText(/Waiting for .* to accept/)).toBeVisible();
  }

  test("a request rings the advisor on another page, through to the chatroom, until accepted", async () => {
    await psychic.goto("/en/profile");
    await setAvailability(psychic, "online");
    await request();

    const banner = psychic.getByTestId("incoming-request-banner");
    await expect(banner).toBeVisible();
    await expect(banner).toContainText("would like to start a live chat.");
    await expect(banner).toHaveAttribute("data-ringtone", "ringing");

    await banner.getByRole("link", { name: "Open chat" }).click();
    await psychic.waitForURL(/\/en\/chatroom/);
    await expect(psychic.getByTestId("incoming-request-banner")).toHaveCount(0);
    const accept = psychic.getByRole("button", { name: "Accept", exact: true });
    await expect(accept).toBeVisible();
    // Still ringing in the chatroom (client-side navigation keeps the audio): the on-screen request card is the UI.
    await expect(psychic.locator("html")).toHaveAttribute("data-ringtone", "ringing");
    await psychic.waitForTimeout(1500);
    await expect(psychic.locator("html")).toHaveAttribute("data-ringtone", "ringing");
    await expect(accept).toBeVisible();

    await accept.click();
    await expect(psychic.getByText("Live chat in progress")).toBeVisible();
    await expect(client.getByText("Live chat in progress")).toBeVisible();
    // Accepted and in the chat: the ringing stops.
    await expect(psychic.locator("html")).toHaveAttribute("data-ringtone", "idle");
  });

  test("a client cancelling stops the ringing", async () => {
    // The client ends the live chat from the first test.
    await client.getByRole("button", { name: /End the chat with/ }).click();
    await expect(client.getByTestId("chat-summary")).toBeVisible();
    await psychic.goto("/en/profile");
    await client.goto(`/en/chatroom?reader=${THEO.slug}`);
    await request();
    const banner = psychic.getByTestId("incoming-request-banner");
    await expect(banner).toHaveAttribute("data-ringtone", "ringing");
    await client.getByRole("button", { name: "Cancel request" }).click();
    await expect(banner).toHaveCount(0);
    await expect(psychic.locator("html")).toHaveAttribute("data-ringtone", "idle");
  });
});
