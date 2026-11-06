/**
 * Advisors' notes about a client (§9.5, Figma 1172:1191 / 1174:2449), with
 * the seed accounts: Ramone writes a private and a shared note during a live
 * chat; Theo, in his own chat with the same client, sees only the shared one
 * under "Notes from other psychics"; the client never sees any notes UI.
 */
import { expect, test, type BrowserContext, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { CLIENT, advisor, setAvailability, signedInPage } from "./helpers";

const RAMONE = advisor("ramone");
const THEO = advisor("theo");
const RUN = Date.now();
const PRIVATE_NOTE = `Private: prefers evening readings ${RUN}`;
const SHARED_NOTE = `Shared: asking about a career change ${RUN}`;

/** The client starts a chat with `slug` and the advisor accepts it. */
async function startChat(client: Page, psychic: Page, slug: string) {
  await client.goto(`/en/chatroom?reader=${slug}`);
  const start = client.getByRole("button", { name: /Start live chat/ });
  await expect(start).toBeEnabled();
  await start.click();
  const accept = psychic.getByRole("button", { name: "Accept", exact: true });
  await expect(accept).toBeVisible();
  await accept.click();
  await expect(client.getByText("Live chat in progress")).toBeVisible();
  await expect(psychic.getByText("Live chat in progress")).toBeVisible();
}

async function endChat(client: Page) {
  await client.getByRole("button", { name: /End the chat with/ }).click();
  await expect(client.getByTestId("chat-summary")).toBeVisible();
}

async function goOnlineInChatroom(psychic: Page) {
  await psychic.goto("/en/profile");
  await setAvailability(psychic, "online");
  await psychic.goto("/en/chatroom");
}

test.describe.serial("advisor notes about a client", () => {
  let clientCtx: BrowserContext;
  let ramoneCtx: BrowserContext;
  let theoCtx: BrowserContext;
  let client: Page;
  let ramone: Page;
  let theo: Page;

  test.beforeAll(async ({ browser }) => {
    ({ context: ramoneCtx, page: ramone } = await signedInPage(browser, RAMONE.email, RAMONE.password));
    ({ context: theoCtx, page: theo } = await signedInPage(browser, THEO.email, THEO.password));
    ({ context: clientCtx, page: client } = await signedInPage(browser, CLIENT.email, CLIENT.password));
  });

  test.afterAll(async () => {
    for (const page of [ramone, theo]) {
      await page?.goto("/en/profile").catch(() => undefined);
      if (page) await setAvailability(page, "offline").catch(() => undefined);
    }
    await clientCtx?.close();
    await ramoneCtx?.close();
    await theoCtx?.close();
  });

  test("Ramone keeps a private note and shares another during the chat", async () => {
    await goOnlineInChatroom(ramone);
    await startChat(client, ramone, RAMONE.slug);

    const panel = ramone.getByTestId("client-panel").filter({ visible: true });
    const notes = panel.getByTestId("client-notes");
    const composer = notes.getByPlaceholder("Write notes about this client…");
    await expect(composer).toBeVisible();
    const share = notes.getByRole("checkbox", { name: "Share with other advisors" });
    await expect(share).not.toBeChecked();

    // Private (the default).
    let created = ramone.waitForResponse((r) => /\/chat\/conversations\/[^/]+\/notes$/.test(r.url()) && r.request().method() === "POST");
    await composer.fill(PRIVATE_NOTE);
    await notes.getByRole("button", { name: "Save note" }).click();
    expect((await created).status()).toBe(201);
    const privateCard = notes.getByTestId("note-card").filter({ hasText: PRIVATE_NOTE });
    await expect(privateCard).toBeVisible();
    await expect(privateCard.getByTestId("note-shared-badge")).toHaveCount(0);

    // Shared, saved with Ctrl+Enter.
    created = ramone.waitForResponse((r) => /\/chat\/conversations\/[^/]+\/notes$/.test(r.url()) && r.request().method() === "POST");
    await share.check();
    await composer.fill(SHARED_NOTE);
    await composer.press("Control+Enter");
    const body = (await (await created).json()) as { shared: boolean; body: string };
    expect(body).toMatchObject({ shared: true, body: SHARED_NOTE });
    const sharedCard = notes.getByTestId("note-card").filter({ hasText: SHARED_NOTE });
    await expect(sharedCard.getByTestId("note-shared-badge")).toHaveText("Shared");
    await expect(share).not.toBeChecked();

    // Newest first.
    await expect(notes.getByTestId("note-card").first()).toContainText(SHARED_NOTE);
    await expectNoA11yViolations(ramone, ["[data-testid=client-panel]"]);
  });

  test("the client never sees notes", async () => {
    await expect(client.getByTestId("client-notes")).toHaveCount(0);
    await expect(client.getByPlaceholder("Write notes about this client…")).toHaveCount(0);
    await expect(client.getByText(PRIVATE_NOTE)).toHaveCount(0);
    await expect(client.getByText(SHARED_NOTE)).toHaveCount(0);
    await endChat(client);
    await ramone.goto("/en/profile");
    await setAvailability(ramone, "offline");
  });

  test("Theo sees only the shared note under Notes from other psychics", async () => {
    await goOnlineInChatroom(theo);
    await startChat(client, theo, THEO.slug);

    const panel = theo.getByTestId("client-panel").filter({ visible: true });
    const notes = panel.getByTestId("client-notes");
    // Ramone's notes are not Theo's own notes.
    await expect(notes.getByPlaceholder("Write notes about this client…")).toBeVisible();
    await expect(notes.getByText(SHARED_NOTE)).toHaveCount(0);

    await notes.getByRole("button", { name: "+ Check other advisor's notes on this user" }).click();
    const dialog = theo.getByRole("dialog", { name: "Notes from other psychics" });
    const shared = dialog.getByTestId("other-note").filter({ hasText: SHARED_NOTE });
    await expect(shared).toBeVisible();
    await expect(shared).toContainText("Ramone");
    await expect(dialog.getByText(PRIVATE_NOTE)).toHaveCount(0);
    await expectNoA11yViolations(theo, ["[data-testid=other-notes-dialog]"]);
    await theo.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);

    // Below xl the panel is a sheet with Overview / Saved / Notes tabs.
    await theo.setViewportSize({ width: 390, height: 844 });
    await theo.getByRole("button", { name: "Open reader details" }).click();
    const sheet = theo.getByRole("dialog");
    await expect(sheet.getByRole("tab")).toHaveText(["Overview", "Saved", "Notes"]);
    await sheet.getByRole("tab", { name: "Notes" }).click();
    await expect(sheet.getByPlaceholder("Write notes about this client…")).toBeVisible();
    await expect(sheet.getByRole("button", { name: "+ Check other advisor's notes on this user" })).toBeVisible();
    await theo.keyboard.press("Escape");
    await theo.setViewportSize({ width: 1440, height: 900 });

    await endChat(client);
  });
});
