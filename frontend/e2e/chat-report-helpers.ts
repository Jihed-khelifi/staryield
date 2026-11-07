/**
 * Shared setup for the "Report an issue" specs (§16.4): a client with a
 * conversation with an advisor, so the chatroom shows the report trigger.
 */
import { expect, type Browser, type BrowserContext, type Page } from "@playwright/test";

import { CLIENT, advisor, setAvailability, signedInPage } from "./helpers";

export const THEO = advisor("theo");
export const PHONE = { width: 390, height: 844 };

/**
 * The advisor goes online, the client requests a chat with them and cancels
 * the request: the conversation (and its session) now exist and the client's
 * chatroom for that advisor shows "Report an issue".
 */
export async function createConversation(browser: Browser, clientViewport = { width: 1440, height: 900 }) {
  const { context: psychicCtx, page: psychic } = await signedInPage(browser, THEO.email, THEO.password);
  await psychic.goto("/en/profile");
  await setAvailability(psychic, "online");
  const { context: clientCtx, page: client } = await signedInPage(browser, CLIENT.email, CLIENT.password, clientViewport);
  await client.goto(`/en/chatroom?reader=${THEO.slug}`);
  const start = client.getByRole("button", { name: /Start live chat/ });
  // A conversation from an earlier run is enough: only request a chat when it can be started.
  if (await start.isEnabled().catch(() => false)) {
    await start.click();
    await expect(client.getByText(/Waiting for .* to accept/)).toBeVisible();
    await client.getByRole("button", { name: "Cancel request" }).click();
    await expect(client.getByText(/Waiting for .* to accept/)).toHaveCount(0);
  }
  return {
    client,
    psychic,
    async close() {
      await setAvailability(psychic, "offline").catch(() => undefined);
      await clientCtx.close();
      await psychicCtx.close();
    },
  };
}

/** The visible "Report an issue" trigger of the chatroom, with the placement that fits the viewport. */
export async function reportTrigger(page: Page) {
  const trigger = page.getByTestId("report-issue-trigger").filter({ visible: true });
  await expect(trigger).toBeVisible();
  const width = page.viewportSize()?.width ?? 1440;
  await expect(trigger).toHaveAttribute("data-placement", width < 768 ? "tools" : "header");
  return trigger;
}

export type { BrowserContext };
