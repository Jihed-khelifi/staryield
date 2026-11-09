/**
 * Messages outside a live chat (§22.3), end to end with two browsers: after a
 * live chat ended both sides keep writing (text and an image), the composer
 * note shows the allowance, the daily cap and a refused send are explained
 * (mocked: setting the cap needs the admin console's step-up), and while a
 * live chat is paused the client can still send into it.
 *
 * Needs the §22 backend. Seed data is shared: the specs only rely on the
 * allowance being > 0 for the client, never on exact counts.
 */
import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test";

import { CLIENT, advisor, ensureCredits, expectImageLoaded, pngFixture, setAvailability, signIn, signedInPage } from "./helpers";

const SOLOMON = advisor("solomon");
const RUN = Date.now().toString(36);
const ALLOWANCE = "**/api/v1/chat/conversations/*/allowance";

const message = (page: Page, text: string) => page.getByTestId("chat-message").filter({ hasText: text }).last();
const offSessionLabel = (page: Page, text: string) => message(page, text).getByTestId("message-off-session");

async function send(page: Page, body: string) {
  const box = page.getByRole("textbox", { name: "Message" });
  await expect(box).toBeEnabled();
  await box.fill(body);
  await box.press("Enter");
  await expect(box).toHaveValue("");
}

/** Start a live chat from the client and accept it as the advisor. */
async function startLiveChat(client: Page, psychic: Page) {
  await client.goto(`/en/chatroom?reader=${SOLOMON.slug}`);
  const start = client.getByRole("button", { name: /Start live chat/ }).first();
  await expect(start).toBeEnabled();
  await start.click();
  const accept = psychic.getByRole("button", { name: "Accept", exact: true });
  await expect(accept).toBeVisible();
  await accept.click();
  await expect(client.getByText("Live chat in progress")).toBeVisible();
  await expect(psychic.getByText("Live chat in progress")).toBeVisible();
}

/** A client context whose realtime socket refuses `message.send` while `state.refuse` is set. */
async function refusingClient(browser: Browser, state: { refuse: boolean; remaining: number }) {
  const context: BrowserContext = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "en-US", timezoneId: "Europe/Paris" });
  const page = await context.newPage();
  await page.routeWebSocket(/\/api\/v1\/realtime\/ws/, (ws) => {
    const server = ws.connectToServer();
    server.onMessage((raw) => ws.send(raw));
    ws.onMessage((raw) => {
      let frame: { type?: string; ref?: string } = {};
      try {
        frame = JSON.parse(String(raw));
      } catch {
        // Forward what is not JSON untouched.
      }
      if (state.refuse && frame.type === "message.send") {
        ws.send(JSON.stringify({ type: "error", ref: frame.ref, error: { code: "off_session_limit_reached", message: "limit", details: { limit: 20, used: 20 } } }));
        return;
      }
      server.send(raw);
    });
  });
  await page.route(ALLOWANCE, (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      json: { live: false, enabled: true, eligible: true, unlimited: false, limit: 20, used: 20 - state.remaining, remaining: state.remaining, resets_at: new Date(Date.now() + 3_600_000).toISOString() },
    });
  });
  await signIn(page, CLIENT.email, CLIENT.password);
  return { context, page };
}

test.describe.serial("messages outside a live chat", () => {
  let clientCtx: BrowserContext;
  let psychicCtx: BrowserContext;
  let client: Page;
  let psychic: Page;

  test.beforeAll(async ({ browser }) => {
    ({ context: psychicCtx, page: psychic } = await signedInPage(browser, SOLOMON.email, SOLOMON.password));
    ({ context: clientCtx, page: client } = await signedInPage(browser, CLIENT.email, CLIENT.password));
    await ensureCredits(client);
  });

  test.afterAll(async () => {
    // End a chat left running and leave the advisor offline for the next run.
    await client
      ?.getByRole("button", { name: /End the chat with/ })
      .click({ timeout: 3_000 })
      .catch(() => undefined);
    await psychic?.goto("/en/profile").catch(() => undefined);
    await setAvailability(psychic, "offline").catch(() => undefined);
    await clientCtx?.close();
    await psychicCtx?.close();
  });

  test("a live chat runs and ends, which makes the conversation eligible", async () => {
    await psychic.goto("/en/profile");
    await setAvailability(psychic, "online");
    await psychic.goto("/en/chatroom");
    await startLiveChat(client, psychic);
    await send(client, `live-${RUN}`);
    await expect(message(psychic, `live-${RUN}`)).toBeVisible();
    // Messages inside a live chat carry no label.
    await expect(offSessionLabel(client, `live-${RUN}`)).toHaveCount(0);
    await send(psychic, `live-reply-${RUN}`);
    await expect(message(client, `live-reply-${RUN}`)).toBeVisible();

    await client.getByRole("button", { name: /End the chat with/ }).click();
    await expect(client.getByTestId("chat-summary")).toBeVisible();
    await expect(psychic.getByText("Live chat in progress")).toHaveCount(0);
  });

  test("both sides can keep writing, and the note shows what is left", async () => {
    const clientBox = client.getByRole("textbox", { name: "Message" });
    const psychicBox = psychic.getByRole("textbox", { name: "Message" });
    await expect(clientBox).toBeEnabled();
    await expect(psychicBox).toBeEnabled();
    await expect(client.getByTestId("composer-reason")).toHaveCount(0);
    await expect(client.getByTestId("composer-note")).toContainText(/Outside a live chat: free · (\d+ of \d+ left today|unlimited during free mode)/);
    await expect(psychic.getByTestId("composer-note")).toContainText(/Outside a live chat: not billed · (\d+ of \d+ left today|unlimited during free mode)/);

    await send(client, `after-${RUN}`);
    await expect(message(psychic, `after-${RUN}`)).toBeVisible();
    await expect(offSessionLabel(client, `after-${RUN}`)).toHaveText("Outside a live chat");
    await expect(offSessionLabel(psychic, `after-${RUN}`)).toHaveText("Outside a live chat");

    await send(psychic, `answer-${RUN}`);
    await expect(message(client, `answer-${RUN}`)).toBeVisible();
    await expect(offSessionLabel(client, `answer-${RUN}`)).toBeVisible();
  });

  test("an image can be sent outside a live chat too, through the conversation endpoint", async () => {
    await client.getByTestId("attach-input").setInputFiles({ name: "after.png", mimeType: "image/png", buffer: pngFixture() });
    await expect(client.getByTestId("composer-attachment")).toBeVisible();
    const upload = client.waitForResponse((r) => /\/chat\/conversations\/[^/]+\/attachments$/.test(r.url()) && r.request().method() === "POST");
    const box = client.getByRole("textbox", { name: "Message" });
    await box.fill(`photo-${RUN}`);
    await box.press("Enter");
    const response = await upload;
    expect(response.status()).toBe(201);
    const body = (await response.json()) as { message: { kind: string; off_session?: boolean }; allowance?: { live: boolean; remaining: number } };
    expect(body.message.kind).toBe("image");
    expect(body.message.off_session).toBe(true);
    expect(body.allowance?.live).toBe(false);
    for (const page of [client, psychic]) {
      await expect(message(page, `photo-${RUN}`).locator("[data-kind=image]")).toBeVisible();
      await expect(offSessionLabel(page, `photo-${RUN}`)).toBeVisible();
      await expectImageLoaded(page, `[data-testid=chat-message]:has-text("photo-${RUN}") .chat-image-frame img >> nth=-1`);
    }
  });

  test("the daily cap disables the composer with the reason and the start CTA (allowance mocked)", async () => {
    await client.route(ALLOWANCE, (route) =>
      route.request().method() === "GET"
        ? route.fulfill({ json: { live: false, enabled: true, eligible: true, unlimited: false, limit: 20, used: 20, remaining: 0, resets_at: new Date(Date.now() + 3_600_000).toISOString() } })
        : route.fallback(),
    );
    await client.goto(`/en/chatroom?reader=${SOLOMON.slug}`);
    const reason = client.getByTestId("composer-reason");
    await expect(reason).toContainText("You've used today's 20 messages outside a live chat. Start a live chat to keep talking.");
    await expect(client.getByRole("textbox", { name: "Message" })).toBeDisabled();
    await expect(reason.getByTestId("composer-start-chat")).toBeVisible();
    await expect(client.getByTestId("composer-note")).toContainText("Outside a live chat: free · 0 of 20 left today");

    // Dropping a file while disabled says why instead of silently ignoring it.
    await client.evaluate(() => {
      const target = document.querySelector("section.chat-column")!;
      const dt = new DataTransfer();
      dt.items.add(new File([new Uint8Array([1])], "x.png", { type: "image/png" }));
      target.dispatchEvent(new DragEvent("drop", { bubbles: true, cancelable: true, dataTransfer: dt }));
    });
    await expect(client.locator(".toast")).toContainText("You've used today's 20 messages outside a live chat. Start a live chat to keep talking.");
    await client.unroute(ALLOWANCE);
  });

  test("a send refused by the server stays visible with its reason and retry, and the composer follows the allowance", async ({ browser }) => {
    const state = { refuse: true, remaining: 3 };
    const { context, page } = await refusingClient(browser, state);
    try {
      await page.goto(`/en/chatroom?reader=${SOLOMON.slug}`);
      await expect(page.getByRole("textbox", { name: "Message" })).toBeEnabled();
      await send(page, `refused-${RUN}`);
      const failed = message(page, `refused-${RUN}`).getByTestId("message-failed");
      await expect(failed).toContainText("Not delivered: you've used today's messages outside a live chat.");
      await expect(failed.getByRole("button", { name: "Retry" })).toBeVisible();
      await expect(failed.getByRole("button", { name: "Remove" })).toBeVisible();
      // The refusal made the composer read the allowance again: nothing left.
      state.remaining = 0;
      await page.reload();
      await expect(page.getByTestId("composer-reason")).toContainText("Start a live chat to keep talking.");
      // Retrying while capped says why instead of failing again (the failed message was kept in the outbox).
      await expect(message(page, `refused-${RUN}`).getByTestId("message-failed")).toBeVisible();
      await message(page, `refused-${RUN}`).getByRole("button", { name: "Retry" }).click();
      await expect(page.locator(".toast")).toContainText("You've used today's 20 messages outside a live chat.");
      await message(page, `refused-${RUN}`).getByRole("button", { name: "Remove" }).click();
      await expect(message(page, `refused-${RUN}`)).toHaveCount(0);
    } finally {
      await context.close();
    }
  });

  test("while a live chat is paused the client can still send, into the live chat", async () => {
    await psychic.goto("/en/chatroom");
    await startLiveChat(client, psychic);
    // The advisor leaves the conversation on screen: the chat pauses (presence rule).
    await psychic.goto("/en/profile");
    await expect(client.getByText(/lost connection/).first()).toBeVisible();
    const box = client.getByRole("textbox", { name: "Message" });
    await expect(box).toBeEnabled();
    await send(client, `paused-${RUN}`);
    // It belongs to the paused live chat: no "outside a live chat" label.
    await expect(message(client, `paused-${RUN}`)).toBeVisible();
    await expect(offSessionLabel(client, `paused-${RUN}`)).toHaveCount(0);
    // The advisor comes back: the chat resumes and the message is there.
    await psychic.goto("/en/chatroom");
    await expect(psychic.getByText("Live chat in progress")).toBeVisible();
    await expect(message(psychic, `paused-${RUN}`)).toBeVisible();
    await expect(offSessionLabel(psychic, `paused-${RUN}`)).toHaveCount(0);
  });
});
