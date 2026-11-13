/**
 * Revisions 5.0 message actions (§24.9) in a real live chat: each delivered
 * message has one quiet three-dot button (top-left on the viewer's own
 * messages, top-right on received ones) that opens a small menu (Reply,
 * Bookmark, Like, Copy text); the old side buttons are gone; a saved message
 * shows a small bookmark; long-press on a phone opens the same menu.
 *
 * Runs on desktop and on the 390x844 touch phone (@responsive): the client
 * uses the project's viewport, the advisor is always on desktop.
 */
import { expect, test, type BrowserContext, type CDPSession, type Locator, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { CLIENT, advisor, ensureCredits, expectNoHorizontalScroll, setAvailability, signIn, signedInPage } from "./helpers";

const THEO = advisor("theo");
const RUN = Date.now().toString(36);
const PHONE = { width: 390, height: 844 };

const feedMessages = (page: Page) => page.getByTestId("chat-feed").getByTestId("chat-message");
const rowWith = (page: Page, text: string) => feedMessages(page).filter({ has: page.locator(".message-text", { hasText: text }) }).last();
const bubbleOf = (row: Locator) => row.locator(".message-bubble");
const triggerOf = (row: Locator) => row.getByTestId("message-menu-trigger");
const menuOf = (page: Page) => page.getByRole("menu", { name: "Message actions" });

async function send(page: Page, body: string) {
  const box = page.getByRole("textbox", { name: "Message" });
  await box.fill(body);
  // Enter adds a line on touch keyboards: the send button works everywhere.
  await page.getByRole("button", { name: "Send message" }).click();
  await expect(box).toHaveValue("");
}

/** Open the menu with the three-dot button (a tap on touch screens). */
async function openMenu(page: Page, row: Locator) {
  await row.scrollIntoViewIfNeeded();
  await triggerOf(row).click();
  await expect(menuOf(page)).toBeVisible();
  return menuOf(page);
}

/** Touch input through the browser's own input pipeline (Chromium). */
async function longPress(cdp: CDPSession, x: number, y: number) {
  await cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x, y, id: 1 }] });
  await new Promise((resolve) => setTimeout(resolve, 650));
  await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
}

test.describe.serial("three-dot message menu (revisions 5.0)", { tag: "@responsive" }, () => {
  let phone = false;
  let psychicCtx: BrowserContext;
  let clientCtx: BrowserContext;
  let psychic: Page;
  let client: Page;

  test.beforeAll(async ({ browser }, testInfo) => {
    phone = testInfo.project.name === "mobile";
    ({ context: psychicCtx, page: psychic } = await signedInPage(browser, THEO.email, THEO.password));
    clientCtx = await browser.newContext(
      phone
        ? { viewport: PHONE, isMobile: true, hasTouch: true, deviceScaleFactor: 3, locale: "en-US", timezoneId: "Europe/Paris" }
        : { viewport: { width: 1440, height: 900 }, locale: "en-US", timezoneId: "Europe/Paris" },
    );
    client = await clientCtx.newPage();
    await signIn(client, CLIENT.email, CLIENT.password);
    await ensureCredits(client);

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
  });

  test.afterAll(async () => {
    await client?.getByRole("button", { name: /End the chat with/ }).click({ timeout: 5_000 }).catch(() => undefined);
    await psychic?.goto("/en/profile").catch(() => undefined);
    await setAvailability(psychic, "offline").catch(() => undefined);
    await clientCtx?.close().catch(() => undefined);
    await psychicCtx?.close().catch(() => undefined);
  });

  test("a delivered message has one three-dot button, top-left on sent and top-right on received messages", async () => {
    const sent = `Sent by the client ${RUN}`;
    const received = `Received by the client ${RUN}`;
    await send(client, sent);
    await send(psychic, received);
    const mine = rowWith(client, sent);
    const theirs = rowWith(client, received);
    await expect(mine).toBeVisible();
    await expect(theirs).toBeVisible();

    for (const [row, own] of [[mine, true], [theirs, false]] as const) {
      await row.scrollIntoViewIfNeeded();
      const trigger = triggerOf(row);
      await expect(trigger).toHaveCount(1);
      await expect(trigger).toBeVisible();
      await expect(trigger).toHaveAccessibleName("Message actions");
      await expect(trigger).toHaveAttribute("aria-haspopup", "menu");
      await expect(trigger).toHaveAttribute("aria-expanded", "false");
      if (own) await expect(trigger).toHaveAttribute("data-mine", "true");
      else await expect(trigger).not.toHaveAttribute("data-mine");
      // Icon only, and none of the old side buttons.
      await expect(trigger).toHaveText("");
      await expect(row.getByTestId("message-like")).toHaveCount(0);
      await expect(row.getByTestId("message-reply")).toHaveCount(0);
      await expect(row.getByRole("button", { name: "Bookmark this message" })).toHaveCount(0);

      // At the corner of the bubble, on the side that faces the middle of the screen.
      const b = (await bubbleOf(row).boundingBox())!;
      const t = (await trigger.boundingBox())!;
      const cx = t.x + t.width / 2;
      const cy = t.y + t.height / 2;
      expect(Math.abs(cy - b.y)).toBeLessThanOrEqual(12);
      expect(Math.abs(cx - (own ? b.x : b.x + b.width))).toBeLessThanOrEqual(12);
      // Mirrors the bubble: the sent bubble is on the right, the received one on the left.
      expect(own ? cx : b.x + b.width - cx).toBeLessThan(own ? b.x + b.width / 2 : b.width / 2);
    }

    // The advisor sees the same on their side (their own message is "sent").
    const advisorOwn = rowWith(psychic, received);
    await advisorOwn.scrollIntoViewIfNeeded();
    await expect(triggerOf(advisorOwn)).toHaveAttribute("data-mine", "true");
    await expect(triggerOf(rowWith(psychic, sent))).not.toHaveAttribute("data-mine");

    // System notices have no actions.
    await expect(client.getByTestId("chat-system-message").first().getByRole("button")).toHaveCount(0);
  });

  test("it is quiet at rest and full on hover (desktop), subtle but visible on touch, with a 44px target on touch", async () => {
    const row = rowWith(client, `Sent by the client ${RUN}`);
    const trigger = triggerOf(row);
    await row.scrollIntoViewIfNeeded();
    const opacity = () => trigger.evaluate((el) => Number(getComputedStyle(el).opacity));
    if (phone) {
      const rest = await opacity();
      expect(rest).toBeGreaterThan(0.3);
      expect(rest).toBeLessThan(1);
      // The invisible extended hit box (WCAG 2.5.8).
      const hit = await trigger.evaluate((el) => {
        const style = getComputedStyle(el, "::before");
        return { width: parseFloat(style.width), height: parseFloat(style.height) };
      });
      expect(hit.width).toBeGreaterThanOrEqual(44);
      expect(hit.height).toBeGreaterThanOrEqual(44);
    } else {
      await client.mouse.move(2, 2);
      await expect.poll(opacity).toBeLessThan(0.6);
      await bubbleOf(row).hover();
      await expect.poll(opacity).toBe(1);
    }
    await expectNoHorizontalScroll(client);
  });

  test("the three-dot menu lists Reply, Bookmark, Like and Copy text; Reply sets the reply target", async () => {
    const text = `Reply to me ${RUN}`;
    await send(psychic, text);
    const row = rowWith(client, text);
    await expect(row).toBeVisible();
    const menu = await openMenu(client, row);
    await expect(triggerOf(row)).toHaveAttribute("aria-expanded", "true");
    await expect(menu.getByRole("menuitem")).toHaveText(["Reply", "Bookmark", "Like", "Copy text"]);
    await expectNoA11yViolations(client, ["[role=menu]"]);
    await menu.getByRole("menuitem", { name: "Reply" }).click();
    await expect(menu).toBeHidden();
    const bar = client.getByTestId("composer-reply");
    await expect(bar).toContainText(/Replying to /);
    await expect(bar).toContainText(text);
    await expect(client.getByRole("textbox", { name: "Message" })).toBeFocused();
    await client.getByTestId("composer-reply-cancel").click();
    await expect(bar).toHaveCount(0);
  });

  test("Bookmark saves the message: a small bookmark in its meta line, listed in the saved panel; Remove bookmark undoes it", async () => {
    const text = `Keep me ${RUN}`;
    await send(psychic, text);
    const row = rowWith(client, text);
    await expect(row).toBeVisible();
    await expect(row.getByTestId("message-saved")).toHaveCount(0);

    const saved = client.waitForResponse((r) => /\/api\/v1\/chat\/saved\/[^/]+$/.test(r.url()) && r.request().method() === "PUT" && r.ok());
    await (await openMenu(client, row)).getByRole("menuitem", { name: "Bookmark", exact: true }).click();
    await saved;
    const mark = row.getByTestId("message-saved");
    await expect(mark).toBeVisible();
    await expect(mark).toHaveText("Bookmarked");
    // A mark, not a button.
    await expect(mark.getByRole("button")).toHaveCount(0);
    await expect(mark.locator("svg")).toHaveAttribute("aria-hidden", "true");

    // The saved panel lists it (a side panel on desktop, the Bookmarks sheet on phones).
    if (phone) await client.getByTestId("chat-session-tools").getByRole("button", { name: "Bookmarks" }).click();
    const panel = client.getByTestId("reader-panel").filter({ visible: true });
    await expect(panel.getByTestId("saved-message").filter({ hasText: text })).toBeVisible();
    if (phone) await client.keyboard.press("Escape");

    // The label flips, and removing it clears the mark and the list.
    const menu = await openMenu(client, row);
    await expect(menu.getByRole("menuitem")).toHaveText(["Reply", "Remove bookmark", "Like", "Copy text"]);
    await menu.getByRole("menuitem", { name: "Remove bookmark" }).click();
    await expect(row.getByTestId("message-saved")).toHaveCount(0);
    if (phone) await client.getByTestId("chat-session-tools").getByRole("button", { name: "Bookmarks" }).click();
    await expect(client.getByTestId("reader-panel").filter({ visible: true }).getByTestId("saved-message").filter({ hasText: text })).toHaveCount(0);
    if (phone) await client.keyboard.press("Escape");
  });

  test("a liked message keeps its badge, and the button never covers it or the quote", async () => {
    const text = `Like and quote ${RUN}`;
    await send(psychic, text);
    const original = rowWith(client, text);
    await expect(original).toBeVisible();
    await (await openMenu(client, original)).getByRole("menuitem", { name: "Like" }).click();
    const badge = original.getByTestId("message-like-badge");
    await expect(badge).toBeVisible();
    const t = (await triggerOf(original).boundingBox())!;
    const b = (await badge.boundingBox())!;
    const overlap = t.x < b.x + b.width && t.x + t.width > b.x && t.y < b.y + b.height && t.y + t.height > b.y;
    expect(overlap).toBe(false);

    // A reply: the quote block sits inside the bubble, the button outside its content.
    await (await openMenu(client, original)).getByRole("menuitem", { name: "Reply" }).click();
    await send(client, `Answering ${RUN}`);
    const answer = rowWith(client, `Answering ${RUN}`);
    await expect(answer.getByTestId("message-quote")).toBeVisible();
    const trigger = (await triggerOf(answer).boundingBox())!;
    const quote = (await answer.getByTestId("message-quote").boundingBox())!;
    expect(trigger.x < quote.x + quote.width && trigger.x + trigger.width > quote.x && trigger.y < quote.y + quote.height && trigger.y + trigger.height > quote.y).toBe(false);
    // The quote still jumps to the original.
    await answer.getByTestId("message-quote").click();
    await expect(original).toBeInViewport();
  });

  test("keyboard: Enter opens the menu, the arrows move, Escape closes it and returns the focus", async () => {
    test.skip(phone, "keyboard behaviour is checked on desktop");
    const row = rowWith(client, `Keep me ${RUN}`);
    await row.scrollIntoViewIfNeeded();
    const trigger = triggerOf(row);
    await trigger.focus();
    // Focusing the button shows it fully.
    await expect.poll(() => trigger.evaluate((el) => Number(getComputedStyle(el).opacity))).toBe(1);
    await client.keyboard.press("Enter");
    const menu = menuOf(client);
    await expect(menu).toBeVisible();
    await expect(menu.getByRole("menuitem", { name: "Reply" })).toBeFocused();
    await client.keyboard.press("ArrowDown");
    await expect(menu.getByRole("menuitem", { name: "Bookmark" })).toBeFocused();
    await client.keyboard.press("ArrowUp");
    await client.keyboard.press("ArrowUp");
    await expect(menu.getByRole("menuitem", { name: "Copy text" })).toBeFocused();
    await client.keyboard.press("Escape");
    await expect(menu).toBeHidden();
    await expect(trigger).toBeFocused();
    // Space opens it too, and a click outside closes it.
    await client.keyboard.press("Space");
    await expect(menu).toBeVisible();
    await client.getByRole("textbox", { name: "Message" }).click();
    await expect(menu).toBeHidden();
  });

  test("long-press on a phone opens the same menu at the finger", async () => {
    test.skip(!phone, "long-press is a touch gesture");
    const text = `Hold me ${RUN}`;
    await send(psychic, text);
    const row = rowWith(client, text);
    await expect(row).toBeVisible();
    await row.scrollIntoViewIfNeeded();
    const b = (await row.locator(".message-text").boundingBox())!;
    const cdp = await clientCtx.newCDPSession(client);
    await longPress(cdp, b.x + Math.min(b.width / 2, 60), b.y + b.height / 2);
    const menu = menuOf(client);
    await expect(menu).toBeVisible();
    await expect(menu.getByRole("menuitem")).toHaveText(["Reply", "Bookmark", "Like", "Copy text"]);
    // It opens at the finger, not under the button, and stays on the screen.
    await expect(menu).toHaveAttribute("data-anchor", "point");
    const box = (await menu.boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(PHONE.width);
    await menu.getByRole("menuitem", { name: "Bookmark", exact: true }).click();
    await expect(row.getByTestId("message-saved")).toBeVisible();
    // Tapping the button opens the same menu, anchored to it.
    await (await openMenu(client, row)).getByRole("menuitem", { name: "Remove bookmark" }).click();
    await expect(row.getByTestId("message-saved")).toHaveCount(0);
    await expectNoHorizontalScroll(client);
  });
});
