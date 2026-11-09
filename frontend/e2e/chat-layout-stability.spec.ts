/**
 * Composer and feed stay still while typing (§22.5), on a 390x844 touch phone,
 * plus the advisor's header with the client's date of birth on a 360 px phone
 * (§22.6): typing a multi-line message never makes the feed jump, "Draft
 * saved" and the counter never change the composer's height, and the typing
 * indicator never changes the feed's scroll height.
 *
 * Needs the §22 backend (and the shared seed accounts).
 */
import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test";

import { CLIENT, advisor, ensureCredits, expectNoHorizontalScroll, setAvailability, signIn } from "./helpers";

const THEO = advisor("theo");
const RUN = Date.now().toString(36);
const PHONE = { width: 390, height: 844 };
const SMALL_PHONE = { width: 360, height: 740 };

async function phone(browser: Browser, email: string, password: string, viewport = PHONE) {
  const context = await browser.newContext({ viewport, isMobile: true, hasTouch: true, deviceScaleFactor: 3, locale: "en-US", timezoneId: "Europe/Paris" });
  const page = await context.newPage();
  await signIn(page, email, password);
  return { context, page };
}

const composer = (page: Page) => page.locator("form.composer");
const box = (page: Page) => page.getByRole("textbox", { name: "Message" });

async function send(page: Page, body: string) {
  await box(page).fill(body);
  await page.getByRole("button", { name: "Send message" }).click();
  await expect(box(page)).toHaveValue("");
}

/** What the screen reports when a phone keyboard 300 px tall is open (same trick as chat-mobile.spec). */
async function openKeyboard(page: Page) {
  await page.evaluate(() => {
    const vv = window.visualViewport!;
    Object.defineProperty(vv, "height", { configurable: true, value: window.innerHeight - 300 });
    vv.dispatchEvent(new Event("resize"));
  });
  await expect.poll(() => page.evaluate(() => document.documentElement.dataset.keyboard)).toBe("open");
}

type Frame = { scrollTop: number; scrollHeight: number; composerHeight: number; lastTop: number; lastBottomGap: number };

/** Sample the feed and the composer on every animation frame until `stop()` is called. */
async function startRecording(page: Page) {
  await page.evaluate(() => {
    const w = window as unknown as { __frames: Frame[]; __recording: boolean };
    w.__frames = [];
    w.__recording = true;
    const feed = document.querySelector("[data-testid=chat-feed]") as HTMLElement;
    const form = document.querySelector("form.composer") as HTMLElement;
    const tick = () => {
      if (!w.__recording) return;
      const last = Array.from(feed.querySelectorAll("[data-testid=chat-message]")).at(-1) as HTMLElement;
      const lastRect = last.getBoundingClientRect();
      w.__frames.push({
        scrollTop: feed.scrollTop,
        scrollHeight: feed.scrollHeight,
        composerHeight: form.getBoundingClientRect().height,
        lastTop: lastRect.top,
        lastBottomGap: feed.getBoundingClientRect().bottom - lastRect.bottom,
      });
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });
}
async function stopRecording(page: Page): Promise<Frame[]> {
  return page.evaluate(() => {
    const w = window as unknown as { __frames: Frame[]; __recording: boolean };
    w.__recording = false;
    return w.__frames;
  });
}

test.describe.serial("composer and feed stay still while typing", { tag: "@mobile-only" }, () => {
  let clientCtx: BrowserContext;
  let psychicCtx: BrowserContext;
  let client: Page;
  let psychic: Page;

  test.beforeAll(async ({ browser }) => {
    ({ context: psychicCtx, page: psychic } = await phone(browser, THEO.email, THEO.password, SMALL_PHONE));
    ({ context: clientCtx, page: client } = await phone(browser, CLIENT.email, CLIENT.password));
    await ensureCredits(client);
    // The advisor goes online from the profile page and opens the chatroom.
    await psychic.goto("/en/profile");
    await setAvailability(psychic, "online");
    await psychic.goto("/en/chatroom");
    await client.goto(`/en/chatroom?reader=${THEO.slug}`);
    const start = client.getByRole("button", { name: /Start live chat/ }).first();
    await expect(start).toBeEnabled();
    await start.click();
    await psychic.getByRole("button", { name: "Accept", exact: true }).click();
    await expect(client.getByText("Live chat in progress")).toBeVisible();
    await expect(psychic.getByText("Live chat in progress")).toBeVisible();
    // Enough history for the feed to scroll.
    for (let i = 1; i <= 14; i++) await send(psychic, `history ${i} ${RUN} — a line long enough to wrap onto a second line on a phone screen`);
    await expect(client.getByTestId("chat-message").filter({ hasText: `history 14 ${RUN}` })).toBeVisible();
  });

  test.afterAll(async () => {
    await client
      ?.getByRole("button", { name: /End the chat with/ })
      .click({ timeout: 3_000 })
      .catch(() => undefined);
    await psychic?.goto("/en/profile").catch(() => undefined);
    await setAvailability(psychic, "offline").catch(() => undefined);
    await clientCtx?.close();
    await psychicCtx?.close();
  });

  test("typing a multi-line message does not move the feed", async () => {
    const feed = client.getByTestId("chat-feed");
    // Pinned to the newest message before we start.
    await expect.poll(() => feed.evaluate((el) => el.scrollHeight - el.scrollTop - el.clientHeight)).toBeLessThan(4);
    await box(client).tap();
    await startRecording(client);
    const heights: string[] = [];
    const text = "First line of a longer note\nSecond line with a few more words in it\nThird line, still typing quietly\nFourth";
    for (const ch of text) {
      if (ch === "\n") await client.keyboard.press("Enter"); // touch: Enter inserts a newline
      else await client.keyboard.type(ch);
      heights.push(await box(client).evaluate((el) => (el as HTMLTextAreaElement).style.height));
    }
    await client.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
    const frames = await stopRecording(client);
    expect(frames.length).toBeGreaterThan(10);

    // The field only ever grows while typing (never collapsed and re-grown), and never writes "auto".
    expect(heights.every((h) => h !== "auto" && h !== "")).toBe(true);
    const px = heights.map((h) => parseFloat(h));
    for (let i = 1; i < px.length; i++) expect(px[i], `height after key ${i}`).toBeGreaterThanOrEqual(px[i - 1]);

    // A frame is "settled" when the composer did not change in it nor in the frame before (the pin follows a resize one step later).
    const settled = (i: number) => i >= 2 && frames[i].composerHeight === frames[i - 1].composerHeight && frames[i - 1].composerHeight === frames[i - 2].composerHeight;
    // Settled frames must show an unchanged feed: nothing jumps.
    for (let i = 2; i < frames.length; i++) {
      if (!settled(i)) continue;
      expect(frames[i].scrollTop, `scrollTop in frame ${i}`).toBe(frames[i - 1].scrollTop);
      expect(frames[i].lastTop, `last message top in frame ${i}`).toBe(frames[i - 1].lastTop);
      expect(frames[i].scrollHeight, `scrollHeight in frame ${i}`).toBe(frames[i - 1].scrollHeight);
      // Wherever the composer ended up, the newest message stays pinned right above it.
      expect(Math.abs(frames[i].lastBottomGap - frames[0].lastBottomGap), `pin gap in frame ${i}`).toBeLessThanOrEqual(1);
    }
    await box(client).fill("");
  });

  test("Draft saved and the counter never change the composer's height", async () => {
    await box(client).fill("");
    await expect(composer(client)).toBeVisible();
    // Fill past six lines (the field stops growing) just below the counter threshold.
    const body = "x\n".repeat(10) + "a".repeat(779); // 799 characters
    await box(client).fill(body);
    const before = (await composer(client).boundingBox())!.height;
    await expect(client.getByTestId("draft-saved")).toBeVisible({ timeout: 5_000 });
    expect((await composer(client).boundingBox())!.height).toBe(before);
    await box(client).pressSequentially("a"); // 800: the counter shows
    await expect(client.getByText("800/1000")).toBeVisible();
    expect((await composer(client).boundingBox())!.height).toBe(before);
    // The hint goes away again: still the same.
    await expect(client.getByTestId("draft-saved")).toHaveCount(0, { timeout: 6_000 });
    expect((await composer(client).boundingBox())!.height).toBe(before);

    // And with the keyboard open (the note is hidden there, the hint is not).
    await openKeyboard(client);
    const withKeyboard = (await composer(client).boundingBox())!.height;
    await box(client).pressSequentially("b");
    await expect(client.getByTestId("draft-saved")).toBeVisible({ timeout: 5_000 });
    expect((await composer(client).boundingBox())!.height).toBe(withKeyboard);
    await box(client).fill("");
    await client.evaluate(() => {
      const vv = window.visualViewport!;
      Object.defineProperty(vv, "height", { configurable: true, value: window.innerHeight });
      vv.dispatchEvent(new Event("resize"));
    });
    await expect.poll(() => client.evaluate(() => document.documentElement.dataset.keyboard)).toBeUndefined();
  });

  test("the keyboard state does not flap between 80 and 120 px of covered area", async () => {
    const setCovered = (px: number) =>
      client.evaluate((covered) => {
        const vv = window.visualViewport!;
        Object.defineProperty(vv, "height", { configurable: true, value: window.innerHeight - covered });
        vv.dispatchEvent(new Event("resize"));
        return new Promise<string | undefined>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve(document.documentElement.dataset.keyboard))));
      }, px);
    expect(await setCovered(0)).toBeUndefined();
    expect(await setCovered(300)).toBe("open");
    expect(await setCovered(110)).toBe("open");
    expect(await setCovered(90)).toBe("open");
    expect(await setCovered(70)).toBeUndefined();
    expect(await setCovered(110)).toBeUndefined();
    expect(await setCovered(130)).toBe("open");
    expect(await setCovered(0)).toBeUndefined();
  });

  test("the typing indicator never changes the feed's scroll height", async () => {
    const feed = client.getByTestId("chat-feed");
    await expect(client.getByTestId("typing-slot")).toBeAttached();
    // The previous test toggled the keyboard state: let the header and tools row finish their
    // transition (they change the feed's height) before measuring.
    const settled = async () => {
      const a = await feed.evaluate((el) => el.clientHeight);
      await client.waitForTimeout(300);
      return a === (await feed.evaluate((el) => el.clientHeight));
    };
    await expect.poll(settled).toBe(true);
    await expect.poll(() => feed.evaluate((el) => el.scrollHeight - el.scrollTop - el.clientHeight)).toBeLessThan(4);
    const idle = await feed.evaluate((el) => ({ scrollHeight: el.scrollHeight, scrollTop: el.scrollTop }));
    await box(psychic).pressSequentially("typing from the advisor");
    await expect(client.getByRole("status", { name: /is typing/ })).toBeVisible();
    const typing = await feed.evaluate((el) => ({ scrollHeight: el.scrollHeight, scrollTop: el.scrollTop }));
    // The scroll height never changes; the position may round by one device pixel
    // at a fractional bottom (3x screens), which is not a visible movement.
    expect(typing.scrollHeight).toBe(idle.scrollHeight);
    expect(Math.abs(typing.scrollTop - idle.scrollTop)).toBeLessThanOrEqual(1);
    await box(psychic).fill("");
    await expect(client.getByRole("status", { name: /is typing/ })).toHaveCount(0, { timeout: 8_000 });
    expect(await feed.evaluate((el) => el.scrollHeight)).toBe(idle.scrollHeight);
  });

  test("the advisor's header shows the client's date of birth and fits a 360 px phone", async () => {
    // Make the profile deterministic: a date of birth with the birth city NOT shared.
    await psychic.route("**/api/v1/chat/conversations/*/client", async (route) => {
      const res = await route.fetch();
      const json = (await res.json()) as Record<string, unknown>;
      await route.fulfill({ response: res, json: { ...json, date_of_birth: "1990-03-14", age: 36, birth_details_shared: false, birth_details: undefined } });
    });
    await psychic.reload();
    const dob = psychic.getByTestId("chat-header-dob");
    await expect(dob).toBeVisible();
    await expect(dob).toContainText("March 14, 1990");
    // It sits inside the header, under the name, and nothing overflows.
    const header = (await psychic.getByTestId("chat-header").boundingBox())!;
    const box_ = (await dob.boundingBox())!;
    expect(box_.y).toBeGreaterThanOrEqual(header.y);
    expect(box_.y + box_.height).toBeLessThanOrEqual(header.y + header.height + 1);
    expect(box_.x + box_.width).toBeLessThanOrEqual(SMALL_PHONE.width);
    await expectNoHorizontalScroll(psychic);
    // The client panel shows "{date} · {age} years old" (details sheet on phones).
    await psychic.getByRole("button", { name: /View .*'s details/ }).click();
    // The panel is mounted twice on phones (hidden xl column + this sheet): read the visible one.
    const panel = psychic.getByTestId("client-panel").filter({ visible: true });
    await expect(panel.getByTestId("client-birth-date")).toContainText("Mar 14, 1990 · 36 years old");
    await psychic.keyboard.press("Escape");
    await psychic.unroute("**/api/v1/chat/conversations/*/client");
  });
});
