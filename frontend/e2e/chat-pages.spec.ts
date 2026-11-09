/**
 * Chat surfaces that page or wait: the inbox (cursor pages, infinite
 * scroll, error and empty states) and the waiting list on an advisor's
 * profile and in the chatroom when the advisor cannot chat.
 */
import { expect, test, type Page, type Route } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { CLIENT, advisor, expectNoHorizontalScroll, signIn } from "./helpers";

const SOLOMON = advisor("solomon");

/** A synthetic inbox row (only the first one names a real advisor). */
function row(i: number) {
  const at = new Date(Date.UTC(2026, 10, 5, 12, 0, 0) - i * 60_000).toISOString();
  const slug = i === 0 ? "ramone" : `advisor-${i}`;
  return {
    id: `00000000-0000-4000-8000-${String(i).padStart(12, "0")}`,
    counterpart: { id: `p-${i}`, role: "psychic", name: i === 0 ? "Ramone" : `Advisor ${i}`, slug, status: i % 3 === 0 ? "online" : "offline" },
    last_message: { id: `m-${i}`, conversation_id: "x", session_id: "s", kind: "text", body: `Preview ${i}`, created_at: at, seq: i + 1 },
    last_message_at: at,
    last_activity_at: at,
    last_seq: i + 1,
    read_seq: i + 1,
    peer_read_seq: 0,
    unread: i === 2 ? 4 : 0,
    created_at: at,
  };
}

let inboxHealed: () => void = () => undefined;

/** Serve the inbox from 25 rows in pages of 20 (cursor "page-2"). */
async function mockInbox(page: Page, opts: { failFirst?: boolean; empty?: boolean } = {}) {
  const calls: string[] = [];
  // The first load keeps failing (reconnect refreshes included) until the retry button is pressed.
  let failures = opts.failFirst ? 1000 : 0;
  inboxHealed = () => (failures = 0);
  await page.route(/\/api\/v1\/chat\/conversations(\?.*)?$/, async (route: Route) => {
    const url = new URL(route.request().url());
    calls.push(url.search);
    if (failures > 0) {
      failures--;
      await route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "service_unavailable", message: "Down for a moment." } }) });
      return;
    }
    const cursor = url.searchParams.get("cursor");
    const body = opts.empty
      ? { items: [], has_more: false }
      : cursor === "page-2"
        ? { items: Array.from({ length: 5 }, (_, i) => row(20 + i)), has_more: false }
        : { items: Array.from({ length: 20 }, (_, i) => row(i)), next_cursor: "page-2", has_more: true };
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
  });
  return calls;
}

/** The inbox: the left column on desktop, the switcher sheet on phones. */
async function openInbox(page: Page) {
  const menu = page.getByRole("button", { name: "Open conversations" });
  // Below lg the inbox lives in the switcher sheet (the menu button shows once a conversation is open).
  const phone = (page.viewportSize()?.width ?? 1440) < 1024;
  if (phone) {
    await expect(menu).toBeVisible();
    await menu.click();
    return page.getByTestId("chat-switcher").getByTestId("inbox");
  }
  return page.locator("aside").getByTestId("inbox");
}

test.describe("inbox pages @responsive", () => {
  test.beforeEach(async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
  });

  test("loads conversations a page at a time as the list scrolls", async ({ page }) => {
    const calls = await mockInbox(page);
    await page.goto("/en/chatroom");
    const inbox = await openInbox(page);
    const rows = inbox.getByTestId("inbox-row");
    // The phone switcher lists the other conversations (the open one is the "current advisor").
    // Wait for the first page to render before counting it.
    await expect.poll(() => rows.count()).toBeGreaterThanOrEqual(19);
    const firstPage = await rows.count();
    expect(calls[0]).toContain("limit=20");
    await expect(inbox.getByTestId("inbox-load-more")).toBeVisible();
    await expect(inbox.getByLabel("4 unread messages")).toBeVisible();

    // Scrolling to the end fetches the next page with the cursor.
    const next = page.waitForRequest((r) => r.url().includes("cursor=page-2"));
    await inbox.evaluate((el) => el.scrollTo({ top: el.scrollHeight }));
    await next;
    await expect(rows).toHaveCount(firstPage + 5);
    await expect(inbox.getByTestId("inbox-load-more")).toHaveCount(0);
    await expectNoHorizontalScroll(page);
    await expectNoA11yViolations(page, ["[data-testid=inbox]"]);
  });

  test("shows an error with a retry, then the list", async ({ page }) => {
    await mockInbox(page, { failFirst: true });
    await page.goto("/en/chatroom");
    // The conversation column explains and retries (phones have no inbox column).
    const main = page.getByRole("main", { name: "Chatroom" }).locator("section");
    await expect(main.getByText("Your conversations could not be loaded.")).toBeVisible();
    await expectNoA11yViolations(page, ["main section"]);
    inboxHealed();
    await main.getByRole("button", { name: "Try again" }).click();
    // The newest conversation opens, and the full list is one tap away.
    await expect(page.getByTestId("chat-header")).toContainText("Ramone");
    const inbox = await openInbox(page);
    await expect(inbox.getByTestId("inbox-row").first()).toBeVisible();
  });

  test("says so when there are no conversations yet", async ({ page }) => {
    await mockInbox(page, { empty: true });
    await page.goto("/en/chatroom");
    await expect(page.getByText("Choose an advisor to start a conversation.")).toBeVisible();
    const empty = page.locator("aside").getByTestId("inbox-empty");
    if (await page.locator("aside").first().isVisible()) await expect(empty).toBeVisible();
  });
});

test.describe("waiting list @responsive", () => {
  test("a visitor is asked to sign in first", async ({ page }) => {
    await page.goto(`/en/psychics/${SOLOMON.slug}`);
    const join = page.getByTestId("profile-ctas").getByRole("button", { name: "Join Waiting List" });
    await expect(join).toBeVisible();
    await join.click();
    await page.waitForURL(/\/en\/login\?next=/);
    expect(new URL(page.url()).searchParams.get("next")).toBe(`/en/psychics/${SOLOMON.slug}`);
  });

  test("a client joins and leaves from the profile; the state survives a reload", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    // Start clean whatever an earlier run left.
    await page.goto(`/en/psychics/${SOLOMON.slug}`);
    const ctas = page.getByTestId("profile-ctas");
    await expect(ctas.getByRole("link", { name: /Start live chat/ })).toBeVisible();
    const button = ctas.getByTestId("waitlist-button");
    await expect(button).toBeEnabled();
    if ((await button.getAttribute("data-joined")) === "true") {
      await button.click();
      await expect(button).not.toHaveAttribute("data-joined", "true");
    }
    await expect(button).toHaveText("Join Waiting List");
    await expect(button).toHaveAttribute("data-variant", "glass");
    // The button is disabled (dimmed) while a join/leave request is in flight: check the settled state.
    await expect(button).toBeEnabled();
    await expectNoA11yViolations(page, ["[data-testid=profile-ctas]"]);

    const joined = page.waitForResponse((r) => r.url().endsWith(`/psychics/${SOLOMON.slug}/waitlist`) && r.request().method() === "POST" && r.ok());
    await button.click();
    await joined;
    await expect(button).toHaveAttribute("data-joined", "true");
    await expect(button).toContainText("On the waiting list");

    // Joined state comes from GET /me/waitlist after a reload.
    await page.reload();
    await expect(page.getByTestId("profile-ctas").getByTestId("waitlist-button")).toHaveAttribute("data-joined", "true");
    await expectNoHorizontalScroll(page);

    // The chatroom offers the same action instead of a dead end.
    await page.goto(`/en/chatroom?reader=${SOLOMON.slug}`);
    const card = page.getByTestId("chat-waitlist-card");
    await expect(card).toContainText("Solomon is offline right now.");
    const inRoom = card.getByTestId("waitlist-button");
    await expect(inRoom).toHaveAttribute("data-joined", "true");
    await expectNoA11yViolations(page, ["[data-testid=chat-waitlist-card]"]);
    const left = page.waitForResponse((r) => r.url().endsWith(`/psychics/${SOLOMON.slug}/waitlist`) && r.request().method() === "DELETE");
    await inRoom.click();
    expect((await left).status()).toBe(204);
    await expect(inRoom).toHaveText("Join Waiting List");
  });

  test("an advisor who turns out to be online switches the profile to 'Start chat'", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    // The profile was loaded while the advisor looked busy; the server knows better (409).
    await page.route(/\/api\/v1\/psychics\/solomon$/, async (route) => {
      const res = await route.fetch();
      const body = (await res.json()) as Record<string, unknown>;
      await route.fulfill({ response: res, body: JSON.stringify({ ...body, status: "busy" }) });
    });
    await page.route(/\/api\/v1\/psychics\/solomon\/waitlist$/, (route) =>
      route.request().method() === "POST"
        ? route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ error: { code: "advisor_available", message: "Solomon is online now." } }) })
        : route.continue(),
    );
    await page.goto(`/en/psychics/${SOLOMON.slug}`);
    const ctas = page.getByTestId("profile-ctas");
    await ctas.getByRole("button", { name: "Join Waiting List" }).click();
    await expect(page.getByText("Solomon is online now. Start your chat.")).toBeVisible();
    await expect(ctas.getByTestId("waitlist-button")).toHaveCount(0);
    await expect(ctas.getByRole("link", { name: /Start live chat/ })).toBeVisible();
  });
});
