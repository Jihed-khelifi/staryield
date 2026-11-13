/**
 * The chatroom never scrolls the page (§24.9): the frame fills exactly the
 * viewport and only the feed, the inbox and the panel scroll.
 *
 * Root cause of the bug this guards: the advisor's inbox rows carry an
 * off-screen ("sr-only", absolutely positioned) availability label. With no
 * positioned ancestor it was laid out at its static position deep inside the
 * scrolling inbox, escaped the inbox's clip and stretched the whole document
 * (about 90 px per screenful of rows), leaving empty paper below the columns.
 */
import { expect, test, type Page } from "@playwright/test";

import { CLIENT, advisor, signIn } from "./helpers";

const VIEWPORTS = [
  { width: 1920, height: 996 },
  { width: 1440, height: 900 },
  { width: 1366, height: 768 },
  { width: 1280, height: 720 },
  { width: 2560, height: 1440 },
  { width: 1024, height: 768 },
  { width: 768, height: 1024 },
  { width: 390, height: 844 },
];

/** An advisor with many conversations: the real inbox, repeated under other ids. */
async function manyConversations(page: Page, count = 24) {
  await page.route(/\/api\/v1\/chat\/conversations(\?.*)?$/, async (route) => {
    const res = await route.fetch();
    const body = (await res.json()) as { items: { id: string; counterpart: { id: string } }[] };
    const first = body.items[0];
    if (!first) return route.fulfill({ response: res, json: body });
    const suffix = (i: number) => String(i).padStart(2, "0");
    const items = Array.from({ length: count }, (_, i) => ({
      ...first,
      id: `${first.id.slice(0, -2)}${suffix(i)}`,
      counterpart: { ...first.counterpart, id: `${first.counterpart.id.slice(0, -2)}${suffix(i)}` },
    }));
    return route.fulfill({ response: res, json: { ...body, items } });
  });
}

async function expectNoPageScroll(page: Page, label: string) {
  // After a viewport change the phone frame's height follows the visualViewport resize event
  // (`--vvh`, useVisualViewport), which can land a frame or two after setViewportSize returns:
  // wait for the layout to settle before measuring.
  await expect
    .poll(() => page.evaluate(() => Math.round(document.querySelector(".dashboard-page")!.getBoundingClientRect().bottom) - window.innerHeight), {
      message: `${label}: the frame settles to the viewport height`,
    })
    .toBe(0);
  const m = await page.evaluate(() => {
    const root = document.scrollingElement as HTMLElement;
    window.scrollTo(0, 5000);
    const frame = document.querySelector(".dashboard-page")!.getBoundingClientRect();
    return { scrollHeight: root.scrollHeight, innerHeight: window.innerHeight, scrolledTo: window.scrollY, frameBottom: frame.bottom, frameTop: frame.top };
  });
  expect(m.scrollHeight, `${label}: document taller than the viewport`).toBeLessThanOrEqual(m.innerHeight);
  expect(m.scrolledTo, `${label}: page scrolled`).toBe(0);
  // The frame fills exactly the viewport.
  expect(m.frameTop).toBe(0);
  expect(Math.round(m.frameBottom)).toBe(m.innerHeight);
}

test.describe("the chatroom page never scrolls", () => {
  test("advisor with a long inbox, at every viewport", async ({ browser }) => {
    const a = advisor("theo");
    const context = await browser.newContext({ viewport: VIEWPORTS[0], locale: "en-US", timezoneId: "Europe/Paris" });
    const page = await context.newPage();
    await signIn(page, a.email, a.password);
    await manyConversations(page);
    await page.goto("/en/chatroom");
    await expect(page.getByTestId("inbox-row").first()).toBeVisible();
    for (const viewport of VIEWPORTS) {
      await page.setViewportSize(viewport);
      await expect(page.locator("main.chat-workspace")).toBeVisible();
      await expectNoPageScroll(page, `advisor ${viewport.width}x${viewport.height}`);
    }
    await context.close();
  });

  test("the inbox is what scrolls, not the page", async ({ browser }) => {
    const a = advisor("theo");
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "en-US", timezoneId: "Europe/Paris" });
    const page = await context.newPage();
    await signIn(page, a.email, a.password);
    await manyConversations(page);
    await page.goto("/en/chatroom");
    await expect(page.getByTestId("inbox-row")).toHaveCount(24);
    const inbox = page.getByTestId("inbox");
    const { scrollable } = await inbox.evaluate((el) => ({ scrollable: el.scrollHeight > el.clientHeight + 100 }));
    expect(scrollable).toBe(true);
    await inbox.evaluate((el) => el.scrollTo(0, el.scrollHeight));
    await expect.poll(() => inbox.evaluate((el) => el.scrollTop)).toBeGreaterThan(0);
    expect(await page.evaluate(() => window.scrollY)).toBe(0);
    await expectNoPageScroll(page, "advisor, inbox scrolled to the end");
    await context.close();
  });

  test("client at every viewport", async ({ browser }) => {
    const context = await browser.newContext({ viewport: VIEWPORTS[0], locale: "en-US", timezoneId: "Europe/Paris" });
    const page = await context.newPage();
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto("/en/chatroom?reader=theo");
    for (const viewport of VIEWPORTS) {
      await page.setViewportSize(viewport);
      await expect(page.locator("main.chat-workspace")).toBeVisible();
      await expectNoPageScroll(page, `client ${viewport.width}x${viewport.height}`);
    }
    await context.close();
  });
});
