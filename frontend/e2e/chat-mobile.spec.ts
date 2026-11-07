/**
 * Revisions 3.0 chatroom: phone layout (no bottom bar, keyboard-ready
 * composer), header chat shortcut with the unread badge, drafts, the image
 * viewer and large screens. Needs the merged backend (§18 unread count and
 * realtime `chat.unread`).
 */
import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { API_URL, CLIENT, advisor, expectNoHorizontalScroll, pngFixture, setAvailability, signIn, signedInPage } from "./helpers";

/** The signed-in user's unread total, straight from the API (cookies of the page's context). */
async function unreadTotal(page: Page): Promise<number> {
  const res = await page.request.get(`${API_URL}/api/v1/chat/unread-count`, { headers: { Origin: new URL(page.url()).origin } });
  expect(res.ok()).toBeTruthy();
  return ((await res.json()) as { count: number }).count;
}
/** Emulate the tab going to the background or coming back (Page Visibility API). */
async function setVisibility(page: Page, state: "visible" | "hidden") {
  await page.evaluate((s) => {
    Object.defineProperty(document, "visibilityState", { configurable: true, get: () => s });
    Object.defineProperty(document, "hidden", { configurable: true, get: () => s === "hidden" });
    document.dispatchEvent(new Event("visibilitychange"));
  }, state);
}
const badgeText = (n: number) => (n > 99 ? "99+" : String(n));
const unreadLabel = (n: number) => (n === 0 ? "Chat" : n === 1 ? "Chat, 1 unread message" : `Chat, ${badgeText(n)} unread messages`);


const THEO = advisor("theo");
const PHONE = { width: 390, height: 844 };

/** A signed-in phone context (touch, mobile UA metrics). */
async function phonePage(browser: Browser, email: string, password: string) {
  const context = await browser.newContext({ viewport: PHONE, isMobile: true, hasTouch: true, deviceScaleFactor: 3, locale: "en-US", timezoneId: "Europe/Paris" });
  const page = await context.newPage();
  await signIn(page, email, password);
  return { context, page };
}

const shortcut = (page: Page) => page.getByTestId("chat-shortcut").filter({ visible: true });
const badge = (page: Page) => page.getByTestId("chat-unread-badge").filter({ visible: true });

test.describe("header chat shortcut visibility", { tag: "@responsive" }, () => {
  for (const path of ["/en", "/en/psychics", "/en/login", "/en/calculators/natal-chart"]) {
    test(`absent for anonymous visitors on ${path}`, async ({ page }) => {
      await page.goto(path);
      // Phones hide the header on the sign-in pages by design: it only has to be rendered.
      await expect(page.locator("header.site-header")).toBeAttached();
      await expect(page.getByTestId("chat-shortcut")).toHaveCount(0);
    });
  }

  test("signed-in clients see it on public and app pages, linking to the chatroom", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    for (const path of ["/en", "/en/psychics", "/en/profile", "/en/credits"]) {
      await page.goto(path);
      await expect(shortcut(page)).toBeVisible();
      await expect(shortcut(page)).toHaveAttribute("href", "/en/chatroom");
      // /en/credits opens the checkout modal, which (correctly) hides the header from assistive tech.
      await expect(shortcut(page)).toHaveAttribute("aria-label", /^Chat/);
      await expectNoHorizontalScroll(page);
    }
  });

  test("signed-in advisors see it too", async ({ page }) => {
    const ramone = advisor("ramone");
    await signIn(page, ramone.email, ramone.password);
    for (const path of ["/en", "/en/profile"]) {
      await page.goto(path);
      await expect(shortcut(page)).toBeVisible();
    }
  });

  test("signing out removes it", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto("/en/profile");
    await expect(shortcut(page)).toBeVisible();
    await page.context().clearCookies();
    await page.goto("/en/psychics");
    await expect(page.getByTestId("chat-shortcut")).toHaveCount(0);
  });
});

test.describe("phone chatroom layout", { tag: "@mobile-only" }, () => {
  test("no bottom navigation in the chatroom; its links live in the menu and the switcher", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    // The bar is still there on other app pages.
    await page.goto("/en/profile");
    await expect(page.locator(".mobile-navigation")).toBeVisible();

    await page.goto(`/en/chatroom?reader=${THEO.slug}`);
    await expect(page.getByRole("textbox", { name: "Message" })).toBeVisible();
    await expect(page.locator(".mobile-navigation")).toHaveCount(0);
    await expect(page.getByRole("navigation", { name: "Main navigation" })).toHaveCount(0);

    // No site header on phones in the chatroom (the chat header is the top bar, §20.2).
    await expect(page.locator("header.site-header")).toBeHidden();
    await expect(page.getByTestId("mobile-menu-trigger")).toBeHidden();

    // Switcher: compact quick links.
    await page.getByRole("button", { name: "Open conversations" }).click();
    const quick = page.getByTestId("chat-quick-links");
    await expect(quick.getByRole("link")).toHaveText(["Today", "Psychics", "Chat", "Profile"]);
    await quick.getByRole("link", { name: "Psychics" }).click();
    await page.waitForURL(/\/en\/psychics/);
  });

  test("the composer has room: 16px input, above the fold, nothing scrolls sideways", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto(`/en/chatroom?reader=${THEO.slug}`);
    const box = page.getByRole("textbox", { name: "Message" });
    await expect(box).toBeVisible();
    const fontSize = await box.evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
    expect(fontSize).toBeGreaterThanOrEqual(16);
    const bottom = await box.evaluate((el) => el.getBoundingClientRect().bottom);
    expect(bottom).toBeLessThanOrEqual(PHONE.height);
    // The workspace owns the screen: the document itself never scrolls.
    const scrolls = await page.evaluate(() => document.documentElement.scrollHeight > window.innerHeight + 1);
    expect(scrolls).toBe(false);
    await expect(page.getByTestId("chat-feed")).toHaveCSS("overscroll-behavior-y", "contain");
    await expect(page.locator("meta[name=viewport]")).toHaveAttribute("content", /interactive-widget=resizes-content/);
    await expectNoHorizontalScroll(page);
  });

  test("the frame follows the visible area when the keyboard shrinks it", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto(`/en/chatroom?reader=${THEO.slug}`);
    await expect(page.getByRole("textbox", { name: "Message" })).toBeVisible();
    // Simulate what iOS Safari reports: a visual viewport 300px shorter.
    await page.evaluate(() => {
      const vv = window.visualViewport!;
      Object.defineProperty(vv, "height", { configurable: true, value: window.innerHeight - 300 });
      vv.dispatchEvent(new Event("resize"));
    });
    await expect.poll(() => page.evaluate(() => document.documentElement.dataset.keyboard)).toBe("open");
    const frame = await page.locator(".chat-frame").evaluate((el) => el.getBoundingClientRect().height);
    expect(frame).toBeLessThanOrEqual(844 - 299);
    const bottom = await page.getByRole("textbox", { name: "Message" }).evaluate((el) => el.getBoundingClientRect().bottom);
    expect(bottom).toBeLessThanOrEqual(844 - 300);
  });
});

const stateOf = (page: Page) => ({
  header: page.getByTestId("chat-header"),
  tools: page.getByTestId("chat-session-tools"),
});
const ACCOUNT_ROUTES = { notifications: "**/api/v1/me/notifications/unread-count", chat: "**/api/v1/chat/unread-count" };

test.describe("phone chatroom top bar and drawer (revisions 3.1)", { tag: "@mobile-only" }, () => {
  test("client: the chat header is the top bar, tools row has credits, report and bookmarks", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto(`/en/chatroom?reader=${THEO.slug}`);
    await expect(page.getByRole("textbox", { name: "Message" })).toBeVisible();
    const { header, tools } = stateOf(page);
    await expect(page.locator("header.site-header")).toBeHidden();
    // The chat header starts at the very top of the screen and the whole top chrome stays slim.
    const h = (await header.boundingBox())!;
    expect(h.y).toBeLessThanOrEqual(1);
    expect(h.height).toBeLessThanOrEqual(70);
    const t = (await tools.boundingBox())!;
    expect(t.y + t.height).toBeLessThanOrEqual(120);
    // The details button is gone (the avatar opens details); report moved to the tools row.
    await expect(header.getByRole("button", { name: "Open reader details" })).toBeHidden();
    await expect(header.getByTestId("report-issue-trigger")).toBeHidden();
    const report = tools.getByTestId("report-issue-trigger");
    await expect(report).toBeVisible();
    await expect(report).toHaveAttribute("data-placement", "tools");
    await expect(tools.getByRole("button", { name: "Bookmarks" })).toBeVisible();
    await header.getByRole("button", { name: /View .*'s details/ }).click();
    await expect(page.locator(".chat-details-sheet")).toBeVisible();
    await page.keyboard.press("Escape");
    await expectNoA11yViolations(page);
  });

  for (const width of [320, 360, 390]) {
    test(`nothing scrolls sideways at ${width}px (page and drawer)`, async ({ page }) => {
      await page.setViewportSize({ width, height: 700 });
      await signIn(page, CLIENT.email, CLIENT.password);
      await page.goto(`/en/chatroom?reader=${THEO.slug}`);
      await expect(page.getByRole("textbox", { name: "Message" })).toBeVisible();
      await expectNoHorizontalScroll(page);
      await page.getByRole("button", { name: /^Open conversations/ }).click();
      await expect(page.getByTestId("switcher-account")).toBeVisible();
      await expectNoHorizontalScroll(page);
    });
  }

  test("client drawer: account block with balance link and notifications above the drawer", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto(`/en/chatroom?reader=${THEO.slug}`);
    await page.getByRole("button", { name: /^Open conversations/ }).click();
    const account = page.getByTestId("switcher-account");
    await expect(account).toBeVisible();
    const pill = account.getByTestId("drawer-balance-pill");
    await expect(pill).toHaveAttribute("href", `/en/credits?next=${encodeURIComponent(`/en/chatroom?reader=${THEO.slug}`)}`);
    await expect(account.getByTestId("switcher-current")).toHaveCount(0);
    await expect(page.getByTestId("switcher-current")).toBeVisible();
    await expect(page.getByTestId("chat-quick-links")).toBeVisible();
    await expect(page.getByRole("link", { name: "Leave Chatroom" })).toBeVisible();
    await expectNoA11yViolations(page, ["[data-testid=chat-switcher]"]);

    // The bell opens its own sheet on top of the drawer.
    await account.getByRole("button", { name: /^Notifications/ }).click();
    const sheet = page.getByRole("dialog", { name: "Notifications" });
    await expect(sheet).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(sheet).toHaveCount(0);
    await expect(page.getByTestId("chat-switcher")).toBeVisible();
  });

  test("the menu button shows a dot for unread notifications or other conversations", async ({ page }) => {
    let notifications = 0;
    await page.route(ACCOUNT_ROUTES.notifications, (route) => route.fulfill({ json: { count: notifications } }));
    await page.route(ACCOUNT_ROUTES.chat, (route) => route.fulfill({ json: { count: 0, conversations: 0 } }));
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto(`/en/chatroom?reader=${THEO.slug}`);
    await expect(page.getByRole("textbox", { name: "Message" })).toBeVisible();
    await expect(page.getByTestId("menu-activity-dot")).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Open conversations", exact: true })).toBeVisible();
    notifications = 3;
    await page.reload();
    const menu = page.getByRole("button", { name: "Open conversations, new activity" });
    await expect(menu).toBeVisible();
    await expect(menu.getByTestId("menu-activity-dot")).toHaveAttribute("aria-hidden", "true");
    await expectNoA11yViolations(page, ["[data-testid=chat-header]"]);
  });

  test("keyboard open: the tools row, notice and composer note collapse and the header compacts", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto(`/en/chatroom?reader=${THEO.slug}`);
    const box = page.getByRole("textbox", { name: "Message" });
    await expect(box).toBeVisible();
    const { header, tools } = stateOf(page);
    const closed = (await header.boundingBox())!.height;
    const feedClosed = (await page.getByTestId("chat-feed").boundingBox())!.height;
    await expect(tools).toBeVisible();
    await page.evaluate(() => {
      document.documentElement.dataset.keyboard = "open";
    });
    await expect(tools).toBeHidden();
    await expect.poll(async () => (await header.boundingBox())!.height).toBeLessThan(closed - 8);
    expect((await header.boundingBox())!.height).toBeLessThanOrEqual(54);
    await expect.poll(async () => (await page.getByTestId("chat-feed").boundingBox())!.height).toBeGreaterThan(feedClosed + 40);
    await expect(page.locator(".chat-workspace .composer-note")).toBeHidden();
    await expect(header.getByRole("button", { name: /^Open conversations/ })).toBeVisible();
    await page.evaluate(() => {
      delete document.documentElement.dataset.keyboard;
    });
    await expect(tools).toBeVisible();
  });

  test("the feed stays on the latest message when its height changes", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto(`/en/chatroom?reader=${THEO.slug}`);
    await expect(page.getByRole("textbox", { name: "Message" })).toBeVisible();
    const feed = page.getByTestId("chat-feed");
    await feed.evaluate((el) => {
      el.scrollTop = el.scrollHeight;
    });
    await page.evaluate(() => {
      document.documentElement.dataset.keyboard = "open";
    });
    await page.waitForTimeout(400);
    const gap = await feed.evaluate((el) => el.scrollHeight - el.scrollTop - el.clientHeight);
    expect(gap).toBeLessThanOrEqual(2);
  });

  test("advisor: availability in the tools row, wallet, availability and bell in the drawer", async ({ browser }) => {
    const { context, page } = await phonePage(browser, advisor("ramone").email, advisor("ramone").password);
    try {
      await page.goto("/en/chatroom");
      // With a conversation: the chat header; without one: a slim bar that still opens the drawer.
      await expect(page.locator("[data-testid=chat-header], [data-testid=chat-topbar]").filter({ visible: true }).first()).toBeVisible();
      await expect(page.locator("header.site-header")).toBeHidden();
      if ((await page.getByTestId("chat-header").count()) > 0) {
        await expect(page.getByTestId("chat-session-tools").getByTestId("availability-menu")).toBeVisible();
      }
      await page.getByRole("button", { name: /^Open conversations/ }).click();
      const account = page.getByTestId("switcher-account");
      await expect(account.getByTestId("wallet-pill")).toBeVisible();
      await expect(account.getByTestId("availability-menu")).toBeVisible();
      await expect(account.getByRole("button", { name: /^Notifications/ })).toBeVisible();
      await expect(account.getByTestId("drawer-balance-pill")).toHaveCount(0);
      await expectNoHorizontalScroll(page);
      await expectNoA11yViolations(page, ["[data-testid=chat-switcher]"]);
    } finally {
      await context.close();
    }
  });
});

test.describe("tablet and desktop keep the site header", { tag: "@responsive" }, () => {
  test("the report button sits in the chat header on tablet and up", async ({ page, isMobile }) => {
    test.skip(Boolean(isMobile), "phones have the tools row");
    for (const size of [{ width: 768, height: 1024 }, { width: 1024, height: 800 }, { width: 1440, height: 900 }]) {
      await page.setViewportSize(size);
      if (size.width === 768) {
        await signIn(page, CLIENT.email, CLIENT.password);
        await page.goto(`/en/chatroom?reader=${THEO.slug}`);
      }
      await expect(page.locator("header.site-header")).toBeVisible();
      await expect(page.getByTestId("chat-header").getByTestId("report-issue-trigger")).toHaveAttribute("data-placement", "header");
      await expect(page.getByTestId("switcher-account")).toHaveCount(0);
      await expectNoHorizontalScroll(page);
    }
  });
});

test.describe("large screens", () => {
  test("1920x1080: the chatroom uses the width and the columns scale up", async ({ page }) => {
    await page.setViewportSize({ width: 1920, height: 1080 });
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto(`/en/chatroom?reader=${THEO.slug}`);
    const workspace = page.locator(".chat-workspace");
    await expect(workspace).toBeVisible();
    const box = async (sel: string) => page.locator(sel).filter({ visible: true }).first().evaluate((el) => el.getBoundingClientRect().width);
    const total = await workspace.evaluate((el) => el.getBoundingClientRect().width);
    expect(total).toBeGreaterThan(1700); // no longer capped at 1440
    expect(await box(".chat-inbox-column")).toBeGreaterThanOrEqual(340);
    expect(await box(".chat-detail-column")).toBeGreaterThanOrEqual(360);
    expect(await box(".chat-column")).toBeGreaterThan(900);
    await expectNoHorizontalScroll(page);
  });

  test("2560x1440 stops at the 2560px cap and 1440x900 is unchanged", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.setViewportSize({ width: 2560, height: 1440 });
    await page.goto(`/en/chatroom?reader=${THEO.slug}`);
    await expect(page.locator(".chat-workspace")).toBeVisible();
    expect(await page.locator(".dashboard-page").evaluate((el) => el.getBoundingClientRect().width)).toBeLessThanOrEqual(2560);
    await page.setViewportSize({ width: 1440, height: 900 });
    await expect(page.locator(".chat-inbox-column")).toBeVisible();
    expect(await page.locator(".chat-inbox-column").evaluate((el) => el.getBoundingClientRect().width)).toBeLessThanOrEqual(262);
    expect(await page.locator(".dashboard-page").evaluate((el) => el.getBoundingClientRect().width)).toBeLessThanOrEqual(1440);
  });
});

test.describe.serial("live chat on a phone: drafts, unread badge, image viewer", () => {
  let clientCtx: BrowserContext;
  let psychicCtx: BrowserContext;
  let client: Page;
  let psychic: Page;
  const DRAFT = `unsent thought ${Date.now()}`;
  // Unique per run: the seed conversations keep their history between runs.
  const RUN = Date.now().toString(36);

  test.beforeAll(async ({ browser }) => {
    ({ context: psychicCtx, page: psychic } = await signedInPage(browser, THEO.email, THEO.password));
    ({ context: clientCtx, page: client } = await phonePage(browser, CLIENT.email, CLIENT.password));
  });

  test.afterAll(async () => {
    await client?.goto("/en/chatroom?reader=theo").catch(() => undefined);
    await client?.getByRole("button", { name: /End the chat with/ }).click({ timeout: 3000 }).catch(() => undefined);
    await psychic?.goto("/en/profile").catch(() => undefined);
    await setAvailability(psychic, "offline").catch(() => undefined);
    await clientCtx?.close();
    await psychicCtx?.close();
  });

  test("start a live chat", async () => {
    await psychic.goto("/en/profile");
    await setAvailability(psychic, "online");
    await psychic.goto("/en/chatroom");
    await client.goto(`/en/chatroom?reader=${THEO.slug}`);
    const start = client.getByRole("button", { name: /Start live chat/ });
    await expect(start).toBeEnabled();
    await start.click();
    await psychic.getByRole("button", { name: "Accept", exact: true }).click();
    await expect(client.getByRole("textbox", { name: "Message" })).toBeEnabled();
  });

  test("an unsent draft survives a reload and is cleared by sending", async () => {
    const box = client.getByRole("textbox", { name: "Message" });
    await box.fill(DRAFT);
    await expect(client.getByTestId("draft-saved")).toHaveText("Draft saved");
    await client.reload();
    const again = client.getByRole("textbox", { name: "Message" });
    await expect(again).toHaveValue(DRAFT);
    await again.press("Control+Enter");
    await expect(client.getByTestId("chat-message").filter({ hasText: DRAFT }).first()).toBeVisible();
    await expect(again).toHaveValue("");
    await client.reload();
    await expect(client.getByRole("textbox", { name: "Message" })).toHaveValue("");
  });

  // A live chat pauses as soon as one side leaves the chatroom (BR-05), so the
  // badge matters when the chat stays open in a background tab (or on another
  // device): messages arrive unseen, every other page's header counts them, and
  // bringing the chat back to the front reads them and clears every badge live.
  test("the unread badge counts messages the client has not seen and clears when they open the chat", async () => {
    await setVisibility(client, "hidden");
    const other = await clientCtx.newPage();
    await other.goto("/en/psychics");
    await expect(shortcut(other)).toBeVisible();
    // Other specs share the seed accounts: counts are relative to what is already unread.
    const base = await unreadTotal(other);
    await expect(shortcut(other)).toHaveAttribute("aria-label", unreadLabel(base));

    const box = psychic.getByRole("textbox", { name: "Message" });
    await box.fill(`first unread ${RUN}`);
    await box.press("Enter");
    await expect(badge(other)).toHaveText(badgeText(base + 1));
    await expect(shortcut(other)).toHaveAccessibleName(unreadLabel(base + 1));

    await box.fill(`second unread ${RUN}`);
    await box.press("Enter");
    await expect(badge(other)).toHaveText(badgeText(base + 2));
    await expect(shortcut(other)).toHaveAccessibleName(unreadLabel(base + 2));

    // Opening the chat (the tab comes back to the front) reads the messages.
    await setVisibility(client, "visible");
    await expect(client.getByTestId("chat-message").filter({ hasText: `second unread ${RUN}` })).toBeVisible();
    await expect(shortcut(other)).toHaveAttribute("aria-label", unreadLabel(base));
    await other.close();
  });

  test("and the other way: the advisor's badge follows the client's messages", async () => {
    await setVisibility(psychic, "hidden");
    const other = await psychicCtx.newPage();
    await other.goto("/en/profile");
    await expect(shortcut(other)).toBeVisible();
    const base = await unreadTotal(other);
    const box = client.getByRole("textbox", { name: "Message" });
    await box.fill(`hello from the phone ${RUN}`);
    await box.press("Control+Enter");
    await expect(badge(other)).toHaveText(badgeText(base + 1));
    await setVisibility(psychic, "visible");
    await expect(psychic.getByTestId("chat-message").filter({ hasText: `hello from the phone ${RUN}` })).toBeVisible();
    await expect(shortcut(other)).toHaveAttribute("aria-label", unreadLabel(base));
    await other.close();
  });

  test("an image opens in the full-screen viewer and closes with Escape and the button", async () => {
    await client.getByTestId("attach-input").setInputFiles({ name: "spread.png", mimeType: "image/png", buffer: pngFixture(120, 80) });
    const upload = client.waitForResponse((r) => /\/chat\/sessions\/[^/]+\/attachments$/.test(r.url()) && r.status() === 201);
    const box = client.getByRole("textbox", { name: "Message" });
    await box.fill(`viewer test ${RUN}`);
    await box.press("Control+Enter");
    await upload;

    const frame = client.getByTestId("chat-message").filter({ hasText: `viewer test ${RUN}` }).last().locator(".chat-image-frame");
    await expect(frame).toBeEnabled();
    // The frame reserves its aspect ratio (no layout shift) and fits the phone.
    const { w, h } = await frame.evaluate((el) => ({ w: el.getBoundingClientRect().width, h: el.getBoundingClientRect().height }));
    expect(w / h).toBeCloseTo(120 / 80, 1);
    expect(h).toBeLessThanOrEqual(262);

    await frame.tap();
    const viewer = client.getByTestId("image-viewer");
    await expect(viewer).toBeVisible();
    await expect(viewer.getByRole("img", { name: `viewer test ${RUN}` })).toBeVisible();
    // Zoom: double tap toggles.
    const stage = viewer.getByTestId("lightbox-stage");
    await stage.dblclick();
    await expect(stage).toHaveAttribute("data-zoom", "2.50");
    await viewer.getByRole("button", { name: "Reset zoom" }).click();
    await expect(stage).toHaveAttribute("data-zoom", "1.00");

    await client.keyboard.press("Escape");
    await expect(viewer).toHaveCount(0);
    await expect(frame).toBeFocused();

    await frame.tap();
    await client.getByRole("button", { name: "Close image" }).tap();
    await expect(client.getByTestId("image-viewer")).toHaveCount(0);
    await expectNoHorizontalScroll(client);
  });

  test("long words and URLs wrap inside the bubble", async () => {
    const long = `${"x".repeat(160)} https://staryield.net/en/${"segment".repeat(14)}`;
    const box = client.getByRole("textbox", { name: "Message" });
    await box.fill(long);
    await box.press("Control+Enter");
    const bubble = client.getByTestId("chat-message").filter({ hasText: "xxxxxxxxxx" }).last().locator(".message-bubble");
    await expect(bubble).toBeVisible();
    const fits = await bubble.evaluate((el) => el.scrollWidth <= el.clientWidth + 1 && el.getBoundingClientRect().right <= window.innerWidth);
    expect(fits).toBe(true);
    await expectNoHorizontalScroll(client);
  });
});
