/**
 * The standard "book" button: one state machine and copy on the home cards,
 * the directory, the profile and the chat room, plus the sticky booking bar
 * on phones. Visitors sign up (or sign in) and land where they were heading:
 * the chat with an online advisor, or back on the profile, where the join
 * they asked for is finished.
 *
 * Advisors are driven through the header's availability menu (e2e/helpers.ts)
 * and kept signed in, so they stay connected (online). Desktop project only:
 * the phone checks open their own phone context.
 */
import { expect, test, type Browser, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { advisor, API_URL, CLIENT, enterBirthDate, expectNoHorizontalScroll, PSYCHIC_PASSWORD, setAvailability, signedInPage, signIn } from "./helpers";

const PASSWORD = "E2e!Secure#2026x";
const submit = (page: Page) => page.locator("form button[type=submit]");

/** The advisor's own session, kept open (see advisor-cards.spec.ts). */
async function advisorSession(browser: Browser, slug: "ramone" | "theo" | "solomon") {
  const session = await signedInPage(browser, advisor(slug).email, PSYCHIC_PASSWORD);
  await session.page.goto("/en/profile");
  return session;
}

/** What the sign-up offer says right now, from the same public configuration the page reads. */
async function offerLine(page: Page): Promise<string> {
  const res = await page.request.get(`${API_URL}/api/v1/site/config`);
  const config = (await res.json()) as { signup_offer: { enabled: boolean; free_minutes: number }; free_mode: { active: boolean } };
  if (config.free_mode.active) return "Free for a limited time";
  if (!config.signup_offer.enabled) return "30 credits/min";
  return config.signup_offer.free_minutes === 1 ? "1 free minute for new members" : `${config.signup_offer.free_minutes} free minutes for new members`;
}

test.describe.configure({ mode: "serial" });

test.describe("visitors", () => {
  test("an online advisor offers 'Chat now' with the new-member offer, and sign-up ends in the chat", async ({ browser, page }) => {
    const ramone = await advisorSession(browser, "ramone");
    try {
      await setAvailability(ramone.page, "online");
      await page.goto("/en/psychics/ramone");
      const hero = page.getByTestId("profile-ctas").getByTestId("book-button");
      await expect(hero).toContainText("Chat now");
      await expect(hero).toHaveAttribute("data-variant", "cta");
      // The line is the real configuration (offer or free mode), or the advisor's own rate when none applies.
      const expected = await offerLine(page);
      if (expected.includes("credits/min")) await expect(hero).toContainText("credits/min");
      else await expect(hero).toContainText(expected);
      await hero.click();
      await page.waitForURL(/\/en\/signup\?next=/);
      expect(new URL(page.url()).searchParams.get("next")).toBe("/en/chatroom?reader=ramone");

      // Every sign-up step keeps it, and the last one opens the chat with Ramone.
      await page.locator("input[name=email]").fill(`e2e+booking-${Date.now()}@staryield.test`);
      await page.getByRole("checkbox").check();
      await submit(page).click();
      await page.waitForURL(/\/en\/signup\/password\?next=/);
      expect(new URL(page.url()).searchParams.get("next")).toBe("/en/chatroom?reader=ramone");
      await page.locator("input[name=password]").fill(PASSWORD);
      await page.locator("input[name=confirm]").fill(PASSWORD);
      await submit(page).click();
      await page.waitForURL(/\/en\/signup\/birth\?next=/);
      expect(new URL(page.url()).searchParams.get("next")).toBe("/en/chatroom?reader=ramone");
      await enterBirthDate(page, { day: "14", month: "03", year: "1990" });
      await page.waitForURL((url) => url.pathname === "/en/chatroom" && url.searchParams.get("reader") === "ramone");
      await expect(page.locator("main.chat-workspace, [data-testid=chat-header]").filter({ visible: true }).first()).toBeVisible();
    } finally {
      await setAvailability(ramone.page, "offline").catch(() => undefined);
      await ramone.context.close();
    }
  });

  test("an advisor who is not online offers 'Notify me when online'; signing in finishes the join", async ({ browser, page }) => {
    const solomon = await advisorSession(browser, "solomon");
    // The seed client starts off Solomon's list, whatever an earlier run left.
    const own = await signedInPage(browser, CLIENT.email, CLIENT.password);
    try {
      await setAvailability(solomon.page, "offline");
      await own.page.goto("/en/psychics/solomon");
      const ownButton = own.page.getByTestId("profile-ctas").getByTestId("book-button");
      await expect(ownButton).toHaveAttribute("data-state", "ready");
      if ((await ownButton.getAttribute("data-action")) === "leave_waitlist") {
        await ownButton.click();
        await expect(ownButton).toHaveAttribute("data-action", "join_waitlist");
      }

      await page.goto("/en/psychics/solomon");
      const hero = page.getByTestId("profile-ctas").getByTestId("book-button");
      await expect(hero).toContainText("Notify me when online");
      await expect(hero).toHaveAttribute("data-variant", "glass");
      await hero.click();
      await page.waitForURL(/\/en\/signup\?next=/);
      expect(new URL(page.url()).searchParams.get("next")).toBe("/en/psychics/solomon");

      // An existing client takes the sign-in link, which keeps the way back.
      await page.getByRole("link", { name: "Already have an account? Log in" }).click();
      await page.waitForURL(/\/en\/login\?next=/);
      expect(new URL(page.url()).searchParams.get("next")).toBe("/en/psychics/solomon");
      await page.locator("input[name=email]").fill(CLIENT.email);
      await page.locator("input[name=password]").fill(CLIENT.password);
      const joined = page.waitForResponse((r) => r.url().endsWith("/psychics/solomon/waitlist") && r.request().method() === "POST" && r.ok());
      await submit(page).click();
      await page.waitForURL(/\/en\/psychics\/solomon$/);
      // Back on the profile, the click made as a visitor is finished: one join, announced.
      await joined;
      const back = page.getByTestId("profile-ctas").getByTestId("book-button");
      await expect(back).toHaveAttribute("data-action", "leave_waitlist");
      await expect(page.getByText("You're on Solomon's waiting list.")).toBeVisible();

      // Only once: a reload does not join again (the state comes from the list).
      let posts = 0;
      page.on("request", (r) => {
        if (r.method() === "POST" && r.url().endsWith("/psychics/solomon/waitlist")) posts++;
      });
      await page.reload();
      await expect(page.getByTestId("profile-ctas").getByTestId("book-button")).toHaveAttribute("data-action", "leave_waitlist");
      expect(posts).toBe(0);
      await page.getByTestId("profile-ctas").getByTestId("book-button").click();
      await expect(page.getByTestId("profile-ctas").getByTestId("book-button")).toHaveAttribute("data-action", "join_waitlist");
    } finally {
      await own.context.close();
      await solomon.context.close();
    }
  });
});

test.describe("signed-in client", () => {
  test("joins and leaves from a home card, and the profile shows the same state with one waiting-list read", async ({ browser, page }) => {
    const solomon = await advisorSession(browser, "solomon");
    try {
      await setAvailability(solomon.page, "offline");
      await signIn(page, CLIENT.email, CLIENT.password);
      await page.goto("/en");
      const card = page.getByTestId("featured-advisor").filter({ has: page.getByRole("heading", { name: "Solomon" }) });
      const button = card.getByTestId("book-button");
      await expect(button).toBeEnabled();
      // Start clean whatever an earlier run left.
      if ((await button.getAttribute("data-action")) === "leave_waitlist") {
        await button.click();
        await expect(button).toHaveAttribute("data-action", "join_waitlist");
      }
      await expect(button).toContainText("Notify me when online");

      const joined = page.waitForResponse((r) => r.url().endsWith("/psychics/solomon/waitlist") && r.request().method() === "POST" && r.ok());
      await button.click();
      await joined;
      await expect(button).toHaveAttribute("data-action", "leave_waitlist");
      await expect(button).toContainText("On the waiting list");
      await expect(page.getByText("You're on Solomon's waiting list.")).toBeVisible();

      // The profile shows the same state, from a single read of the lists (hero and phone bar share it).
      let reads = 0;
      page.on("request", (r) => {
        if (r.method() === "GET" && /\/api\/v1\/me\/waitlist/.test(r.url())) reads++;
      });
      await page.goto("/en/psychics/solomon");
      const hero = page.getByTestId("profile-ctas").getByTestId("book-button");
      await expect(hero).toHaveAttribute("data-action", "leave_waitlist");
      expect(reads).toBe(1);

      // Leaving from the profile shows on the home card too.
      const left = page.waitForResponse((r) => r.url().endsWith("/psychics/solomon/waitlist") && r.request().method() === "DELETE");
      await hero.click();
      expect((await left).status()).toBe(204);
      await expect(hero).toHaveAttribute("data-action", "join_waitlist");
      await page.goto("/en");
      await expect(card.getByTestId("book-button")).toHaveAttribute("data-action", "join_waitlist");
    } finally {
      await solomon.context.close();
    }
  });

  test("the directory cards carry the same button", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto("/en/psychics");
    const cards = page.locator("article.advisor-card");
    await expect(cards.first()).toBeVisible();
    const count = await cards.count();
    await expect(cards.getByTestId("book-button")).toHaveCount(count);
    await expectNoHorizontalScroll(page);
  });
});

test.describe("phones", () => {
  test("the profile shows a sticky booking bar above the tab bar, without horizontal scroll or a covered footer", async ({ browser }) => {
    const ramone = await advisorSession(browser, "ramone");
    const context = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, locale: "en-US" });
    const page = await context.newPage();
    try {
      await setAvailability(ramone.page, "online");
      await page.goto("/en/psychics/ramone");
      const bar = page.getByTestId("booking-bar");
      await expect(bar).toBeVisible();
      await expect(bar).toContainText("Ramone");
      await expect(bar.getByTestId("book-button")).toContainText("Chat now");
      await expectNoHorizontalScroll(page);
      await expectNoA11yViolations(page, ["[data-testid=booking-bar]"]);

      // Bottom of the page: the bar sits above the tab bar and over no content.
      await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
      const geometry = await page.evaluate(() => {
        const barBox = document.querySelector('[data-testid="booking-bar"]')!.getBoundingClientRect();
        const nav = document.querySelector(".mobile-navigation")!.getBoundingClientRect();
        const main = document.querySelector("main.psychic-profile")!;
        const last = main.lastElementChild!.getBoundingClientRect();
        return { barTop: barBox.top, barBottom: barBox.bottom, barLeft: barBox.left, barRight: barBox.right, navTop: nav.top, lastBottom: last.bottom, width: window.innerWidth };
      });
      expect(geometry.barBottom).toBeLessThanOrEqual(geometry.navTop);
      expect(geometry.barLeft).toBeGreaterThanOrEqual(0);
      expect(geometry.barRight).toBeLessThanOrEqual(geometry.width);
      expect(geometry.lastBottom).toBeLessThanOrEqual(geometry.barTop + 1);
    } finally {
      await context.close();
      await setAvailability(ramone.page, "offline").catch(() => undefined);
      await ramone.context.close();
    }
  });

  test("the bar is hidden from the tablet width up", async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 800 });
    await page.goto("/en/psychics/ramone");
    await expect(page.getByTestId("profile-ctas")).toBeVisible();
    await expect(page.getByTestId("booking-bar")).toBeHidden();
  });
});
