/**
 * Revisions 4.0 chatroom (§23.2-§23.5) in a real live session between a
 * client and an advisor: replies with quotes and the jump to the original,
 * likes (button, live on the other side, survive a reload and a cut socket),
 * touch gestures on a phone, the advisor's private client nicknames, and the
 * fixed order of the placements in the client panel.
 *
 * Needs the API with the §23 endpoints (reply_to_id, message likes,
 * /chat/conversations/:id/alias, /chat/aliases).
 */
import { expect, test, type Browser, type BrowserContext, type CDPSession, type Locator, type Page, type WebSocketRoute } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { API_URL, CLIENT, advisor, ensureCredits, expectNoHorizontalScroll, setAvailability, signIn, signedInPage } from "./helpers";

const THEO = advisor("theo");
const RUN = Date.now().toString(36);
const PHONE = { width: 390, height: 844 };

/** The client's realtime socket runs through a proxy so the test can cut it (see chat-sync.spec.ts). */
class SocketProxy {
  block = false;
  private sockets: WebSocketRoute[] = [];
  async attach(page: Page) {
    await page.routeWebSocket(/\/api\/v1\/realtime\/ws/, (ws) => {
      this.sockets.push(ws);
      if (this.block) return;
      const server = ws.connectToServer();
      server.onMessage((raw) => ws.send(raw));
    });
  }
  async cut() {
    const open = this.sockets.splice(0);
    await Promise.all(open.map((ws) => ws.close({ code: 4000, reason: "test cut" }).catch(() => undefined)));
  }
}

const feedMessages = (page: Page) => page.getByTestId("chat-feed").getByTestId("chat-message");
// By the row's own text: a reply's quote repeats the original, so the whole row text would also match the reply.
const rowWith = (page: Page, text: string) => feedMessages(page).filter({ has: page.locator(".message-text", { hasText: text }) }).last();
const badge = (row: Locator) => row.getByTestId("message-like-badge");

async function send(page: Page, body: string) {
  const box = page.getByRole("textbox", { name: "Message" });
  await box.fill(body);
  await box.press("Enter");
  await expect(box).toHaveValue("");
}

/** Removes the advisor's nickname for the client (idempotent): a failed run must not leak it into the next one. */
async function clearAlias(page: Page, conversationId: string) {
  const req = page.context().request;
  const { csrf_token } = (await (await req.get(`${API_URL}/api/v1/auth/csrf`)).json()) as { csrf_token: string };
  const res = await req.delete(`${API_URL}/api/v1/chat/conversations/${conversationId}/alias`, { headers: { "X-CSRF-Token": csrf_token } });
  expect(res.status(), `clear alias: ${await res.text()}`).toBe(204);
}

/** Reply from the hover actions of a message. */
async function startReply(page: Page, row: Locator) {
  await row.hover();
  await row.getByTestId("message-reply").click();
}

/** Messages through the API (as the signed-in user of the page): faster than typing them. */
async function apiSend(page: Page, conversationId: string, body: string, replyToId?: string): Promise<{ id: string; seq: number }> {
  const req = page.context().request;
  const { csrf_token } = (await (await req.get(`${API_URL}/api/v1/auth/csrf`)).json()) as { csrf_token: string };
  const res = await req.post(`${API_URL}/api/v1/chat/conversations/${conversationId}/messages`, {
    headers: { "X-CSRF-Token": csrf_token },
    data: { body, client_msg_id: `e2e-${RUN}-${Math.random().toString(36).slice(2, 10)}`, ...(replyToId ? { reply_to_id: replyToId } : {}) },
  });
  expect(res.ok(), `send: ${res.status()} ${await res.text()}`).toBe(true);
  return ((await res.json()) as { message: { id: string; seq: number } }).message;
}

/** Touch input through the browser's own input pipeline (Chromium). */
class Finger {
  constructor(private readonly cdp: CDPSession) {}
  private point(x: number, y: number) {
    return [{ x, y, id: 1 }];
  }
  async down(x: number, y: number) {
    await this.cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: this.point(x, y) });
  }
  async move(x: number, y: number) {
    await this.cdp.send("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: this.point(x, y) });
  }
  async up() {
    await this.cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
  }
  async tap(x: number, y: number) {
    await this.down(x, y);
    await this.up();
  }
  async swipeRight(x: number, y: number, distance: number) {
    await this.down(x, y);
    const steps = 8;
    for (let i = 1; i <= steps; i++) await this.move(x + (distance * i) / steps, y + (i % 2 ? 1 : 0));
    await this.up();
  }
}

async function center(locator: Locator) {
  const box = await locator.boundingBox();
  if (!box) throw new Error("not visible");
  return { x: box.x + Math.min(box.width / 2, 60), y: box.y + box.height / 2 };
}

/** The client panel's name for the advisor, however the header shows it. */
const headerName = (page: Page) => page.getByTestId("chat-header-name");

test.describe.serial("replies, likes, nicknames and placements (revisions 4.0)", () => {
  let clientCtx: BrowserContext;
  let psychicCtx: BrowserContext;
  let client: Page;
  let psychic: Page;
  let conversationId = "";
  let realName = "";
  const clientProxy = new SocketProxy();
  const aliasRequestsByClient: string[] = [];

  test.beforeAll(async ({ browser }) => {
    ({ context: psychicCtx, page: psychic } = await signedInPage(browser, THEO.email, THEO.password));
    clientCtx = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "en-US", timezoneId: "Europe/Paris" });
    client = await clientCtx.newPage();
    await clientProxy.attach(client);
    // A client never asks for nicknames.
    client.on("request", (r) => {
      if (/\/chat\/aliases|\/alias$/.test(r.url())) aliasRequestsByClient.push(r.url());
    });
    await signIn(client, CLIENT.email, CLIENT.password);
    await ensureCredits(client);
  });

  test.afterAll(async () => {
    clientProxy.block = false;
    if (conversationId) await clearAlias(psychic, conversationId).catch(() => undefined);
    await client?.getByRole("button", { name: /End the chat with/ }).click({ timeout: 5_000 }).catch(() => undefined);
    await psychic?.setViewportSize({ width: 1440, height: 900 }).catch(() => undefined);
    await psychic?.unroute("**/api/v1/chat/conversations/*/client").catch(() => undefined);
    await psychic?.goto("/en/profile").catch(() => undefined);
    await setAvailability(psychic, "offline").catch(() => undefined);
    await clientCtx?.close().catch(() => undefined);
    await psychicCtx?.close().catch(() => undefined);
  });

  test("a live chat starts", async () => {
    await psychic.goto("/en/profile");
    await setAvailability(psychic, "online");
    await psychic.goto("/en/chatroom");
    await client.goto(`/en/chatroom?reader=${THEO.slug}`);
    const start = client.getByRole("button", { name: /Start live chat/ });
    await expect(start).toBeEnabled();
    await start.click();
    const accept = psychic.getByRole("button", { name: "Accept", exact: true });
    await expect(accept).toBeVisible();
    await accept.click();
    await expect(client.getByText("Live chat in progress")).toBeVisible();
    await expect(psychic.getByText("Live chat in progress")).toBeVisible();
    const thread = (await (await client.context().request.get(`${API_URL}/api/v1/chat/psychics/${THEO.slug}`)).json()) as { conversation: { id: string } };
    conversationId = thread.conversation.id;
    expect(conversationId).toBeTruthy();
    // A nickname left behind by an earlier run would be read as the "real" name below.
    await clearAlias(psychic, conversationId);
    await psychic.reload();
    await expect(headerName(psychic)).toBeVisible();
    realName = ((await headerName(psychic).textContent()) ?? "").trim();
    expect(realName).not.toBe("");
  });

  test("a reply shows the quote on both sides, and Esc cancels a reply in progress", async () => {
    const question = `Which card called to you? ${RUN}`;
    await send(psychic, question);
    const original = rowWith(client, question);
    await expect(original).toBeVisible();

    // Desktop: reply and like appear beside the bookmark on hover.
    await startReply(client, original);
    const bar = client.getByTestId("composer-reply");
    await expect(bar).toContainText(/Replying to /);
    await expect(bar).toContainText(question);
    await expect(client.getByRole("textbox", { name: "Message" })).toBeFocused();
    await client.keyboard.press("Escape");
    await expect(bar).toHaveCount(0);

    await startReply(client, original);
    await send(client, `The Moon, I keep seeing it ${RUN}`);
    await expect(bar).toHaveCount(0); // cleared by sending

    for (const page of [client, psychic]) {
      const answer = rowWith(page, `The Moon, I keep seeing it ${RUN}`);
      await expect(answer).toBeVisible();
      const quote = answer.getByTestId("message-quote");
      await expect(quote).toContainText(question);
      // The quote names who wrote the original: "You" for the advisor, the advisor's name for the client.
      const author = quote.locator(".message-quote-author");
      if (page === psychic) await expect(author).toHaveText("You");
      else await expect(author).not.toHaveText(/^(You)?$/);
    }
    // The quote is a button: keyboard-focusable.
    await rowWith(psychic, `The Moon, I keep seeing it ${RUN}`).getByTestId("message-quote").focus();
    await expect(rowWith(psychic, `The Moon, I keep seeing it ${RUN}`).getByTestId("message-quote")).toBeFocused();
    await expectNoA11yViolations(client, ["[data-testid=chat-feed]", "[data-testid=composer-reply]"]);
  });

  test("pressing a quote scrolls to the original and highlights it", async () => {
    const original = rowWith(psychic, `Which card called to you? ${RUN}`);
    // Push the original out of view.
    for (let i = 1; i <= 12; i++) await send(psychic, `filler ${i} ${RUN}\nline two\nline three`);
    await expect(rowWith(client, `filler 12 ${RUN}`)).toBeVisible();
    await expect(original).not.toBeInViewport();
    await rowWith(psychic, `The Moon, I keep seeing it ${RUN}`).getByTestId("message-quote").click();
    await expect(original).toBeInViewport();
    await expect(original).toHaveAttribute("data-flash", "true");
    await expect(original).not.toHaveAttribute("data-flash", "true", { timeout: 5_000 });
  });

  test("a reply to a message older than the loaded page loads older pages (bounded) and finds it", async () => {
    const target = await apiSend(psychic, conversationId, `Original, long ago ${RUN}`);
    for (let i = 1; i <= 55; i++) await apiSend(psychic, conversationId, `bulk ${i} ${RUN}`);
    await apiSend(client, conversationId, `Answering the old one ${RUN}`, target.id);
    await client.reload();
    const reply = rowWith(client, `Answering the old one ${RUN}`);
    await expect(reply).toBeVisible();
    // Only the newest page is loaded: the original is not on screen.
    await expect(feedMessages(client).locator(".message-text", { hasText: `Original, long ago ${RUN}` })).toHaveCount(0);
    await reply.getByTestId("message-quote").click();
    await expect(rowWith(client, `Original, long ago ${RUN}`)).toBeInViewport({ timeout: 20_000 });
    await expect(rowWith(client, `Original, long ago ${RUN}`)).toHaveAttribute("data-flash", "true");
  });

  test("a reply to a message that is gone is refused with an explanation (no phantom reply)", async () => {
    const req = client.context().request;
    const { csrf_token } = (await (await req.get(`${API_URL}/api/v1/auth/csrf`)).json()) as { csrf_token: string };
    const res = await req.post(`${API_URL}/api/v1/chat/conversations/${conversationId}/messages`, {
      headers: { "X-CSRF-Token": csrf_token },
      data: { body: "to nowhere", client_msg_id: `e2e-bad-${RUN}`, reply_to_id: "00000000-0000-4000-8000-000000000000" },
    });
    expect(res.status()).toBe(422);
    expect(((await res.json()) as { error: { code: string } }).error.code).toBe("reply_target_invalid");
  });

  test("a like from the button shows live on the other side, unlike removes it, and it survives a reload", async () => {
    const text = `Like me ${RUN}`;
    await send(client, text);
    const mine = rowWith(psychic, text);
    await expect(mine).toBeVisible();
    await mine.hover();
    const like = mine.getByTestId("message-like");
    await expect(like).toHaveAttribute("aria-pressed", "false");
    await like.click();
    await expect(like).toHaveAttribute("aria-pressed", "true");
    // The advisor's own copy says "You", the client's says who.
    await expect(badge(mine)).toHaveAccessibleName("You liked this");
    await expect(badge(rowWith(client, text))).toHaveAccessibleName(/ liked this$/);
    await expect(badge(rowWith(client, text))).not.toHaveAccessibleName(/^You/);

    // Both like it: "You and …".
    await rowWith(client, text).hover();
    await rowWith(client, text).getByTestId("message-like").click();
    await expect(badge(rowWith(client, text))).toHaveAccessibleName(/^You and .* liked this$/);
    await expect(badge(mine)).toHaveAccessibleName(/^You and .* liked this$/);

    // Unlike (advisor): the client's heart is the only one left.
    await mine.hover();
    await like.click();
    await expect(like).toHaveAttribute("aria-pressed", "false");
    await expect(badge(rowWith(client, text))).toHaveAccessibleName("You liked this");

    // Survives a reload on both sides.
    await psychic.reload();
    await client.reload();
    await expect(badge(rowWith(psychic, text))).toHaveAccessibleName(/ liked this$/);
    await expect(badge(rowWith(client, text))).toHaveAccessibleName("You liked this");
    await expect(rowWith(client, text).getByTestId("message-like")).toHaveAttribute("aria-pressed", "true");
    // System notices have no actions.
    await expect(psychic.getByTestId("chat-system-message").first().getByRole("button")).toHaveCount(0);
  });

  test("double-click toggles a like on desktop", async () => {
    const text = `Double click ${RUN}`;
    await send(psychic, text);
    const row = rowWith(client, text);
    await expect(row).toBeVisible();
    await row.locator(".message-text").dblclick();
    await expect(badge(row)).toBeVisible();
    await expect(badge(rowWith(psychic, text))).toBeVisible();
    await row.locator(".message-text").dblclick();
    await expect(badge(row)).toHaveCount(0);
    await expect(badge(rowWith(psychic, text))).toHaveCount(0);
  });

  test("a like made while the other side's socket is cut appears after it reconnects", async () => {
    const text = `Liked while away ${RUN}`;
    await send(client, text);
    await expect(rowWith(psychic, text)).toBeVisible();
    clientProxy.block = true;
    await clientProxy.cut();
    await expect(client.getByTestId("connection-banner")).toBeVisible();
    await rowWith(psychic, text).hover();
    await rowWith(psychic, text).getByTestId("message-like").click();
    await expect(badge(rowWith(psychic, text))).toBeVisible();
    await expect(badge(rowWith(client, text))).toHaveCount(0);
    clientProxy.block = false;
    await clientProxy.cut();
    await expect(badge(rowWith(client, text))).toBeVisible({ timeout: 45_000 });
    await expect(client.getByText("Live chat in progress")).toBeVisible({ timeout: 30_000 });
  });

  test("placements follow the fixed order, with Unknown slots when there is no birth time (§23.5)", async () => {
    const pattern = "**/api/v1/chat/conversations/*/client";
    await psychic.route(pattern, async (route) => {
      const res = await route.fetch();
      const json = (await res.json()) as Record<string, unknown>;
      const place = (body: string, sign: string) => ({ body, sign, retrograde: false });
      await route.fulfill({
        response: res,
        json: {
          ...json,
          date_of_birth: "1990-03-14",
          astro: {
            sun_sign: "Pisces",
            time_known: false,
            placements: [place("Pluto", "Capricorn"), place("Mars", "Leo"), place("Midheaven", "Virgo"), place("Sun", "Pisces"), place("Mercury", "Aquarius"), place("Chiron", "Leo"), place("Venus", "Aries")],
          },
        },
      });
    });
    await psychic.reload();
    const section = psychic.getByTestId("client-placements").filter({ visible: true });
    await expect(section).toBeVisible();
    // The preview is the first row.
    await expect(section.getByRole("listitem")).toHaveCount(3);
    await section.getByRole("button", { name: "+ expand" }).click();
    const titles = await section.getByRole("listitem").evaluateAll((els) => els.map((el) => el.getAttribute("title")));
    expect(titles).toEqual(["Sun", "Moon", "Ascendant", "Mercury", "Venus", "Mars", "Pluto"]);
    await expect(section.getByTestId("placement-moon")).toContainText("Unknown");
    await expect(section.getByTestId("placement-ascendant")).toContainText("Unknown");
    await expect(section.getByText("Moon sign unknown")).toBeAttached();
    await expect(section).not.toContainText("Midheaven");
    await expect(section.getByTestId("placement-ascendant")).toHaveCount(1);
    await expectNoA11yViolations(psychic, ["[data-testid=client-placements]"]);
    await psychic.unroute(pattern);
    await psychic.reload();
  });

  test("the advisor gives the client a nickname in the header: header, list and panel show it, only the advisor sees it", async () => {
    const nick = `Moon ${RUN}`;
    await expect(headerName(psychic)).toHaveText(realName);
    await psychic.getByTestId("edit-nickname").click();
    const input = psychic.getByRole("textbox", { name: /^Nickname for / });
    await expect(input).toBeFocused();
    await expect(input).toHaveAttribute("placeholder", realName);
    await expect(input).toHaveAttribute("maxlength", "40");
    await expectNoA11yViolations(psychic, ["[data-testid=nickname-form]"]);
    // Esc cancels without a request.
    await input.fill("Discarded");
    await input.press("Escape");
    await expect(psychic.getByTestId("nickname-form")).toHaveCount(0);
    await expect(headerName(psychic)).toHaveText(realName);

    await psychic.getByTestId("edit-nickname").click();
    const saved = psychic.waitForResponse((r) => /\/chat\/conversations\/[^/]+\/alias$/.test(r.url()) && r.request().method() === "PUT");
    await psychic.getByRole("textbox", { name: /^Nickname for / }).fill(nick);
    await psychic.getByRole("textbox", { name: /^Nickname for / }).press("Enter");
    expect((await saved).ok()).toBe(true);
    await expect(psychic.getByTestId("nickname-form")).toHaveCount(0);
    await expect(headerName(psychic)).toHaveText(nick);
    await expect(headerName(psychic)).toHaveAttribute("title", `Name on the account: ${realName}`);
    await expect(psychic.getByTestId("inbox").getByTestId("inbox-row").first()).toContainText(nick);
    const panel = psychic.getByTestId("client-panel").filter({ visible: true });
    await expect(panel.getByRole("heading", { name: nick })).toBeVisible();
    await expect(panel.getByTestId("client-account-name")).toHaveText(`Name on the account: ${realName}`);
    // Feed labels use it too.
    await expect(psychic.getByTestId("chat-live-log")).not.toContainText(realName);

    // Survives a reload.
    await psychic.reload();
    await expect(headerName(psychic)).toHaveText(nick);
    await expect(psychic.getByTestId("inbox").getByTestId("inbox-row").first()).toContainText(nick);

    // The client's UI never contains it and never asked for it.
    await client.reload();
    await expect(client.locator("body")).not.toContainText(nick);
    const html = await client.content();
    expect(html).not.toContain(nick);
    expect(aliasRequestsByClient).toEqual([]);
    await expect(client.getByTestId("edit-nickname")).toHaveCount(0);
  });

  test("on a phone the nickname editor fits the header without overflowing", async () => {
    await psychic.setViewportSize(PHONE);
    await psychic.goto("/en/chatroom");
    const header = psychic.getByTestId("chat-header");
    await expect(headerName(psychic)).toBeVisible();
    await psychic.getByTestId("edit-nickname").click();
    const input = psychic.getByRole("textbox", { name: /^Nickname for / });
    await expect(input).toBeVisible();
    await input.fill("A".repeat(40));
    for (const target of [input, psychic.getByTestId("nickname-save"), psychic.getByTestId("nickname-cancel")]) {
      const box = (await target.boundingBox())!;
      expect(box.x).toBeGreaterThanOrEqual(0);
      expect(box.x + box.width).toBeLessThanOrEqual(PHONE.width);
    }
    const headerBox = (await header.boundingBox())!;
    const inputBox = (await input.boundingBox())!;
    expect(inputBox.x + inputBox.width).toBeLessThanOrEqual(headerBox.x + headerBox.width);
    await expectNoHorizontalScroll(psychic);
    await input.press("Escape");
    await psychic.setViewportSize({ width: 1440, height: 900 });
    await psychic.goto("/en/chatroom");
  });

  test("clearing the nickname restores the real name everywhere", async () => {
    await psychic.getByTestId("edit-nickname").click();
    const input = psychic.getByRole("textbox", { name: /^Nickname for / });
    await expect(input).toHaveValue(`Moon ${RUN}`);
    const cleared = psychic.waitForResponse((r) => /\/chat\/conversations\/[^/]+\/alias$/.test(r.url()) && r.request().method() === "DELETE");
    await input.fill("   ");
    await input.press("Enter");
    expect((await cleared).status()).toBe(204);
    await expect(headerName(psychic)).toHaveText(realName);
    await expect(psychic.getByTestId("inbox").getByTestId("inbox-row").first()).toContainText(realName);
    await expect(psychic.getByTestId("client-account-name")).toHaveCount(0);
    await psychic.reload();
    await expect(headerName(psychic)).toHaveText(realName);
  });

  test("on a phone: double-tap likes, swipe right replies, long-press opens the menu", async ({ browser }) => {
    // The desktop client leaves; the phone takes its place in the same live chat.
    await clientCtx.close();
    const { context, page: phone, finger } = await phoneClient(browser);
    clientCtx = context;
    client = phone; // the rest of the run (and the clean-up) talks to the phone
    try {
      await expect(phone.getByText("Live chat in progress")).toBeVisible({ timeout: 30_000 });
      const text = `Phone gestures ${RUN}`;
      await send(psychic, text);
      const row = rowWith(phone, text);
      await expect(row).toBeVisible();
      const bubble = row.locator(".message-bubble");
      // Reply and like buttons are not shown on touch devices.
      await expect(row.getByTestId("message-like")).toBeHidden();

      // Double-tap likes (the heart pops), the advisor sees it live.
      const at = await center(row.locator(".message-text"));
      await finger.tap(at.x, at.y);
      await finger.tap(at.x, at.y);
      await expect(badge(row)).toBeVisible();
      await expect(badge(rowWith(psychic, text))).toBeVisible();
      // Double-tapping again takes it back.
      await phone.waitForTimeout(500);
      await finger.tap(at.x, at.y);
      await finger.tap(at.x, at.y);
      await expect(badge(row)).toHaveCount(0);

      // A short swipe springs back without replying; a long one starts a reply.
      const start = await center(bubble);
      await finger.swipeRight(start.x, start.y, 30);
      await expect(phone.getByTestId("composer-reply")).toHaveCount(0);
      await finger.swipeRight(start.x, start.y, 90);
      await expect(phone.getByTestId("composer-reply")).toContainText(text);
      await expect(bubble).not.toHaveAttribute("data-swiping", "true");
      await phone.getByTestId("composer-reply-cancel").click();

      // Vertical scrolling is not hijacked.
      const feed = phone.getByTestId("chat-feed");
      for (let i = 1; i <= 6; i++) await send(psychic, `scroll ${i} ${RUN}\nline two\nline three\nline four`);
      await expect(rowWith(phone, `scroll 6 ${RUN}`)).toBeVisible();
      const before = await feed.evaluate((el) => el.scrollTop);
      const box = (await feed.boundingBox())!;
      await finger.down(box.x + 120, box.y + box.height / 2);
      for (let i = 1; i <= 8; i++) await finger.move(box.x + 124, box.y + box.height / 2 + i * 20);
      await finger.up();
      await expect.poll(() => feed.evaluate((el) => el.scrollTop)).toBeLessThan(before);
      await expect(phone.getByTestId("composer-reply")).toHaveCount(0);

      // Long-press: the menu.
      await rowWith(phone, text).scrollIntoViewIfNeeded();
      const pressAt = await center(rowWith(phone, text).locator(".message-text"));
      await finger.down(pressAt.x, pressAt.y);
      await phone.waitForTimeout(650);
      await finger.up();
      const menu = phone.getByRole("menu", { name: "Message actions" });
      await expect(menu).toBeVisible();
      await expect(menu.getByRole("menuitem")).toHaveText(["Reply", "Like", "Save", "Copy text"]);
      await menu.getByRole("menuitem", { name: "Reply" }).click();
      await expect(phone.getByTestId("composer-reply")).toContainText(text);
      await expectNoHorizontalScroll(phone);
    } finally {
      await expectNoA11yViolations(phone, ["[data-testid=chat-feed]"]).catch(() => undefined);
    }
  });
});

/** A signed-in phone context for the client (touch input through CDP). */
async function phoneClient(browser: Browser) {
  const context = await browser.newContext({ viewport: PHONE, isMobile: true, hasTouch: true, deviceScaleFactor: 3, locale: "en-US", timezoneId: "Europe/Paris" });
  const page = await context.newPage();
  await signIn(page, CLIENT.email, CLIENT.password);
  await page.goto(`/en/chatroom?reader=${THEO.slug}`);
  const finger = new Finger(await context.newCDPSession(page));
  return { context, page, finger };
}
