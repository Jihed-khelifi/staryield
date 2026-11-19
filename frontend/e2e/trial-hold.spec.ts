/**
 * Free-trial hold (revisions 6.0, docs/platform-extensions.md §26.1), end to
 * end with two browsers: a brand-new client whose free minutes run out in a
 * live reading, and an advisor.
 *
 *   1. The reading is held: the top-up dialog opens by itself with the hold
 *      notice and its countdown, the advisor sees the banner, writes, and her
 *      messages are marked "Hidden from client". The client sees blurred
 *      bubbles (never the text, not even on the realtime socket), a sticky
 *      banner after closing the dialog, and can still write. Paying with the
 *      sandbox card continues the reading: the texts appear, the banner and the
 *      advisor's markers go.
 *   2. A second reading is left to expire: the reading ends, the advisor's
 *      messages read "Not revealed", the client's summary offers "Top up to
 *      read {n} hidden messages", a short wallet opens the top-up and then the
 *      confirm dialog by itself, and revealing shows the texts.
 *
 * Needs the merged §26.1 backend and a stack where the sign-up offer is small:
 * a new client has `free_minutes` free minutes, so the hold starts after that
 * many minutes of reading. Set the sign-up offer to 1 minute (admin console,
 * "Sign-up offer") for a fast run; the hold length is the seeded 60 s (the
 * admin minimum is 30 s). The specs size their waits from the new client's
 * wallet and skip when the offer is longer than 5 minutes.
 *
 * Runs on the desktop project and the phone project (@responsive).
 */
import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { API_URL, advisor, expectNoHorizontalScroll, newClient, setAvailability, signedInPage, uniqueSandboxCard, walletBalance } from "./helpers";

const SOLOMON = advisor("solomon");
const RUN = Date.now().toString(36);
/** The hold is the seeded 60 s; leave room for the payment grace and slow machines. */
const HOLD_MS = 60_000;
const MAX_FREE_MINUTES = 5;

const box = (page: Page) => page.getByRole("textbox", { name: "Message" });

async function send(page: Page, body: string) {
  await expect(box(page)).toBeEnabled();
  await box(page).fill(body);
  // Enter adds a line on touch keyboards: the send button works everywhere.
  await page.getByRole("button", { name: "Send message" }).click();
  await expect(box(page)).toHaveValue("");
}

/** A new client in a context that has the project's viewport (desktop or phone). */
async function newClientContext(browser: Browser): Promise<{ context: BrowserContext; page: Page }> {
  const use = test.info().project.use;
  const context = await browser.newContext({
    viewport: use.viewport ?? { width: 1440, height: 900 },
    isMobile: use.isMobile,
    hasTouch: use.hasTouch,
    deviceScaleFactor: use.deviceScaleFactor,
    userAgent: use.userAgent,
    locale: "en-US",
    timezoneId: "Europe/Paris",
  });
  const page = await context.newPage();
  await newClient(page, "hold");
  return { context, page };
}

async function freeMinutes(page: Page): Promise<number> {
  const wallet = (await (await page.context().request.get(`${API_URL}/api/v1/wallet`)).json()) as { free_minutes?: number };
  return wallet.free_minutes ?? 0;
}

/** Start a live reading from the client and accept it as the advisor. */
async function startReading(client: Page, psychic: Page) {
  await client.goto(`/en/chatroom?reader=${SOLOMON.slug}`);
  const start = client.getByRole("button", { name: /Start live chat/ }).first();
  await expect(start).toBeEnabled();
  await start.click();
  const accept = psychic.getByRole("button", { name: "Accept", exact: true });
  await expect(accept).toBeVisible();
  await accept.click();
  await expect(client.getByTestId("chat-header-status")).toContainText("Live chat in progress");
  await expect(psychic.getByTestId("chat-header-status")).toContainText("Live chat in progress");
}

/** Pick the preselected pack, fill the sandbox card and pay in the open top-up dialog. */
async function payInDialog(page: Page) {
  const dialog = page.getByTestId("top-up");
  await expect(dialog).toBeVisible();
  await page.getByTestId("top-up-continue").click();
  const card = uniqueSandboxCard();
  await page.getByLabel("Card number").fill(card.number);
  await page.getByLabel("Expiry date").fill(card.expiry);
  await page.getByLabel("CVV").fill("123");
  await page.getByTestId("top-up-consent").check();
  await page.getByTestId("checkout-pay").click();
  await expect(dialog).toHaveCount(0);
}

/** Seconds on a "m:ss" timer. */
async function seconds(timer: ReturnType<Page["getByTestId"]>): Promise<number> {
  const [m, s] = ((await timer.textContent()) ?? "0:00").split(":").map(Number);
  return m * 60 + s;
}

test.describe.serial("free-trial hold", { tag: "@responsive" }, () => {
  let psychicCtx: BrowserContext;
  let psychic: Page;

  test.beforeAll(async ({ browser }) => {
    ({ context: psychicCtx, page: psychic } = await signedInPage(browser, SOLOMON.email, SOLOMON.password));
    await psychic.goto("/en/profile");
    await setAvailability(psychic, "online");
    await psychic.goto("/en/chatroom");
  });

  test.afterAll(async () => {
    await psychic?.goto("/en/profile").catch(() => undefined);
    await setAvailability(psychic, "offline").catch(() => undefined);
    await psychicCtx?.close();
  });

  test("the trial runs out: hold, locked replies, a top-up in time continues the reading", async ({ browser }) => {
    const { context, page: client } = await newClientContext(browser);
    try {
      const free = await freeMinutes(client);
      test.skip(free > MAX_FREE_MINUTES, `the sign-up offer is ${free} minutes: set it to ${MAX_FREE_MINUTES} or fewer for this spec`);
      test.setTimeout((free + 1) * 60_000 + 4 * HOLD_MS);
      expect(await walletBalance(client)).toBe(0);
      const TEXT = `hold-secret-${RUN}-a`;
      const TEXT2 = `hold-secret-${RUN}-b`;

      // What the client's realtime socket carries: the hidden text must never be on it.
      const frames: string[] = [];
      client.on("websocket", (ws) => ws.on("framereceived", (f) => frames.push(String(f.payload))));

      await startReading(client, psychic);
      const composerBottom = async () => (await box(client).boundingBox())!.y + (await box(client).boundingBox())!.height;
      const bottomBefore = await composerBottom();

      // --- the hold starts: the dialog opens by itself, once, with the hold notice -----------------------------
      const dialog = client.getByTestId("top-up");
      await expect(dialog).toBeVisible({ timeout: free * 60_000 + HOLD_MS });
      const notice = client.getByTestId("top-up-hold");
      await expect(notice).toContainText("Your free minutes are over.");
      await expect(client.getByTestId("top-up-hold-price")).toContainText("credits per minute");
      const left = await seconds(client.getByTestId("top-up-hold-timer"));
      expect(left).toBeGreaterThan(0);
      expect(left).toBeLessThanOrEqual(HOLD_MS / 1000);
      await expectNoA11yViolations(client, ['[data-testid="top-up"]']);
      await expect(client.getByTestId("chat-header-status")).toContainText("Reading on hold");

      // --- the advisor is told and writes; her messages are marked --------------------------------------------
      await expect(psychic.getByTestId("hold-banner")).toContainText("free minutes are over");
      await expect(psychic.getByTestId("hold-banner")).toContainText("Your replies are hidden from them until they top up");
      await send(psychic, TEXT);
      await send(psychic, TEXT2);
      await expect(psychic.getByTestId("message-held")).toHaveCount(2);
      await expect(psychic.getByTestId("message-held").first()).toContainText("Hidden from client");

      // --- the client: closed dialog, sticky banner, blurred bubbles, never the text --------------------------
      await client.getByTestId("top-up-close").click();
      await expect(dialog).toHaveCount(0);
      const banner = client.getByTestId("hold-banner");
      await expect(banner).toBeVisible();
      await expect(banner.getByTestId("hold-banner-timer")).toHaveText(/^\d:\d\d$/);
      await expect(client.getByTestId("locked-message")).toHaveCount(2);
      await expect(client.getByRole("button", { name: /^Hidden message from .+\. Top up to read it\.$/ })).toHaveCount(2);
      await expect(client.getByText(TEXT)).toHaveCount(0);
      expect(await client.content()).not.toContain(TEXT);
      expect(frames.filter((f) => f.includes(TEXT) || f.includes(TEXT2))).toEqual([]);
      // The composer did not move (§22.5) and the phone does not scroll sideways.
      expect(Math.abs((await composerBottom()) - bottomBefore)).toBeLessThanOrEqual(1);
      await expectNoHorizontalScroll(client);
      await expectNoA11yViolations(client, ['[data-testid="hold-banner"]']);

      // The client can still write; a locked message can be answered but not liked, saved or copied.
      await send(client, `still-here-${RUN}`);
      await client.getByTestId("locked-message").first().hover();
      await client.getByTestId("message-menu-trigger").first().click({ force: true });
      await expect(client.getByTestId("message-menu").getByRole("menuitem")).toHaveText(["Reply"]);
      await client.keyboard.press("Escape");

      // Clicking a locked bubble brings the dialog back.
      await client.getByTestId("locked-message").first().click();
      await expect(dialog).toBeVisible();
      await expect(client.getByTestId("top-up-hold")).toBeVisible();

      // --- a top-up in time continues the reading and reveals the texts ---------------------------------------
      await payInDialog(client);
      await expect(client.getByText(/Your reading with .+ continues\./).first()).toBeVisible();
      await expect(client.getByText(TEXT)).toBeVisible();
      await expect(client.getByText(TEXT2)).toBeVisible();
      await expect(client.getByTestId("locked-message")).toHaveCount(0);
      await expect(client.getByTestId("hold-banner")).toHaveCount(0);
      await expect(client.getByTestId("chat-header-status")).toContainText("Live chat in progress");
      await expect(psychic.getByTestId("hold-banner")).toHaveCount(0);
      await expect(psychic.getByTestId("message-held")).toHaveCount(0);
      await expect(psychic.getByTestId("chat-header-status")).toContainText("Live chat in progress");

      await client.getByRole("button", { name: /End the chat with/ }).click();
      await expect(client.getByTestId("chat-summary")).toBeVisible();
    } finally {
      await context.close();
    }
  });

  test("the hold expires: the reading ends, then one minute reveals the replies", async ({ browser }) => {
    const { context, page: client } = await newClientContext(browser);
    try {
      const free = await freeMinutes(client);
      test.skip(free > MAX_FREE_MINUTES, `the sign-up offer is ${free} minutes: set it to ${MAX_FREE_MINUTES} or fewer for this spec`);
      test.setTimeout((free + 1) * 60_000 + 4 * HOLD_MS);
      const TEXT = `hold-expired-${RUN}-a`;
      const TEXT2 = `hold-expired-${RUN}-b`;

      await startReading(client, psychic);
      await expect(client.getByTestId("top-up")).toBeVisible({ timeout: free * 60_000 + HOLD_MS });
      await send(psychic, TEXT);
      await send(psychic, TEXT2);
      await expect(psychic.getByTestId("message-held")).toHaveCount(2);
      await client.getByTestId("top-up-close").click();
      await expect(client.getByTestId("locked-message")).toHaveCount(2);

      // --- nobody pays: the reading ends with "hold_expired" and nothing is charged -----------------------------
      await expect(client.getByTestId("hold-banner")).toHaveCount(0, { timeout: HOLD_MS + 45_000 });
      const summary = client.getByTestId("chat-summary");
      await expect(summary).toBeVisible();
      await expect(summary).toContainText("The chat ended because no top-up arrived in time. No credits were used.");
      await expect(client.getByTestId("locked-message")).toHaveCount(2);
      await expect(client.getByTestId("hidden-messages-prompt")).toContainText("Top up to read 2 hidden messages");
      expect(await walletBalance(client)).toBe(0);
      await expect(psychic.getByTestId("message-held")).toHaveCount(2);
      await expect(psychic.getByTestId("message-held").first()).toContainText("Not revealed");
      await expect(psychic.getByTestId("hold-banner")).toHaveCount(0);
      await expectNoHorizontalScroll(client);

      // --- reveal: a short wallet opens the top-up, then the confirm dialog comes by itself ----------------------
      await client.getByTestId("hidden-messages-reveal").click();
      await expect(client.getByTestId("top-up")).toBeVisible();
      await expect(client.getByTestId("top-up-hold")).toHaveCount(0); // the hold is over: no countdown
      await payInDialog(client);
      const confirm = client.getByTestId("hold-reveal-dialog");
      await expect(confirm).toBeVisible();
      await expect(client.getByTestId("hold-reveal-question")).toHaveText(/^Reveal 2 hidden messages from .+ for \d+ credits\?$/);
      const price = Number(/for (\d+) credits/.exec((await client.getByTestId("hold-reveal-question").textContent()) ?? "")![1]);
      const before = await walletBalance(client);
      await expectNoA11yViolations(client, ['[data-testid="hold-reveal-dialog"]']);
      await client.getByTestId("hold-reveal-confirm").click();

      await expect(client.getByText(TEXT)).toBeVisible();
      await expect(client.getByText(TEXT2)).toBeVisible();
      await expect(client.getByTestId("locked-message")).toHaveCount(0);
      await expect(client.getByTestId("hidden-messages-prompt")).toHaveCount(0);
      await expect.poll(() => walletBalance(client)).toBe(before - price);
      // The advisor's markers go once the replies were revealed.
      await expect(psychic.getByTestId("message-held")).toHaveCount(0);
    } finally {
      await context.close();
    }
  });
});
