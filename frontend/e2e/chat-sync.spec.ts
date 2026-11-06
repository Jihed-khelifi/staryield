/**
 * Chat delivery guarantees in a real live session (client + advisor):
 * strict seq order, a single REST gap fill for missed events, reconnects
 * that backfill what the socket never delivered, the sessionStorage outbox
 * (offline, reload, flush), "Jump to latest", read receipts and the
 * advisor's live earnings. The client's realtime socket runs through a
 * Playwright proxy so the test can drop, cut, block and inject frames.
 */
import { expect, test, type Browser, type BrowserContext, type Page, type WebSocketRoute } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { CLIENT, advisor, expectNoHorizontalScroll, setAvailability, signIn } from "./helpers";

const RAMONE = advisor("ramone");
const RUN = Date.now().toString(36);

/** Realtime socket proxy: forwards both ways unless told otherwise. */
class SocketProxy {
  /** Drop this many `message.created` frames from the server. */
  dropNext = 0;
  /** Drop every `message.created` frame from the server. */
  dropAll = false;
  /** Accept new sockets in the page but never connect them (no hello). */
  block = false;
  private sockets: WebSocketRoute[] = [];

  async attach(page: Page) {
    await page.routeWebSocket(/\/api\/v1\/realtime\/ws/, (ws) => {
      this.sockets.push(ws);
      if (this.block) return; // looks open to the page, but the server never answers
      const server = ws.connectToServer();
      server.onMessage((raw) => {
        const type = (() => {
          try {
            return (JSON.parse(String(raw)) as { type?: string }).type;
          } catch {
            return undefined;
          }
        })();
        if (type === "message.created" && (this.dropAll || this.dropNext > 0)) {
          if (this.dropNext > 0) this.dropNext--;
          return;
        }
        ws.send(raw);
      });
    });
  }

  /** Cut every socket (the page sees an abnormal close and reconnects). */
  async cut() {
    const open = this.sockets.splice(0);
    await Promise.all(open.map((ws) => ws.close({ code: 4000, reason: "test cut" }).catch(() => undefined)));
  }

  /** Push a frame to the page as if the server sent it. */
  inject(frame: object) {
    this.sockets.at(-1)?.send(JSON.stringify(frame));
  }
}

async function signedIn(browser: Browser, email: string, password: string, proxy?: SocketProxy) {
  const context: BrowserContext = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "en-US", timezoneId: "Europe/Paris" });
  const page = await context.newPage();
  if (proxy) await proxy.attach(page);
  await signIn(page, email, password);
  return { context, page };
}

const feedMessages = (page: Page) => page.getByTestId("chat-feed").getByTestId("chat-message");
/** Every entry of the list, system notices included (they own seqs too). */
const feedEntries = (page: Page) => page.getByTestId("chat-feed").locator("[data-seq]");

/** Seqs of the confirmed messages on screen, in DOM order. */
async function seqsOnScreen(page: Page): Promise<number[]> {
  return feedEntries(page).evaluateAll((els) => els.map((el) => Number(el.getAttribute("data-seq") || 0)).filter((n) => n > 0));
}

/** Strictly increasing and gap-free: the feed is ordered by seq with nothing missing or doubled. */
function expectContiguous(seqs: number[]) {
  expect(seqs.length).toBeGreaterThan(0);
  for (let i = 1; i < seqs.length; i++) expect(seqs[i], `seq after ${seqs[i - 1]}`).toBe(seqs[i - 1] + 1);
}

async function send(page: Page, body: string) {
  const box = page.getByRole("textbox", { name: "Message" });
  await box.fill(body);
  await box.press("Enter");
  await expect(box).toHaveValue("");
}

test.describe.serial("chat sync: ordering, gaps, reconnects and the outbox", () => {
  let clientCtx: BrowserContext;
  let psychicCtx: BrowserContext;
  let client: Page;
  let psychic: Page;
  let sessionId = "";
  const clientProxy = new SocketProxy();
  const psychicProxy = new SocketProxy();

  test.beforeAll(async ({ browser }) => {
    ({ context: psychicCtx, page: psychic } = await signedIn(browser, RAMONE.email, RAMONE.password, psychicProxy));
    ({ context: clientCtx, page: client } = await signedIn(browser, CLIENT.email, CLIENT.password, clientProxy));
  });

  test.afterAll(async () => {
    clientProxy.block = false;
    clientProxy.dropAll = false;
    await clientCtx?.setOffline(false).catch(() => undefined);
    // End the chat and leave the advisor offline for the next run.
    await client
      ?.getByRole("button", { name: /End the chat with/ })
      .click({ timeout: 5_000 })
      .catch(() => undefined);
    await psychic?.goto("/en/profile").catch(() => undefined);
    await setAvailability(psychic, "offline").catch(() => undefined);
    await clientCtx?.close();
    await psychicCtx?.close();
  });

  test("a live chat starts", async () => {
    await psychic.goto("/en/profile");
    await setAvailability(psychic, "online");
    await psychic.goto("/en/chatroom");
    await client.goto(`/en/chatroom?reader=${RAMONE.slug}`);
    const start = client.getByRole("button", { name: /Start live chat/ });
    await expect(start).toBeEnabled();
    await start.click();
    const accept = psychic.getByRole("button", { name: "Accept", exact: true });
    await expect(accept).toBeVisible();
    const accepted = psychic.waitForResponse((r) => /\/chat\/sessions\/[^/]+\/accept$/.test(r.url()) && r.ok());
    await accept.click();
    sessionId = ((await (await accepted).json()) as { id: string }).id;
    await expect(client.getByText("Live chat in progress")).toBeVisible();
    await expect(psychic.getByText("Live chat in progress")).toBeVisible();
    await send(client, `hello-${RUN}`);
    await expect(feedMessages(psychic).filter({ hasText: `hello-${RUN}` })).toHaveCount(1);
  });

  test("a missed event is filled once over REST, in seq order", async () => {
    clientProxy.dropNext = 1; // the client never hears about gap-1
    const fill = client.waitForRequest((r) => /\/messages\/since\?after_seq=\d+/.test(r.url()));
    await send(psychic, `gap-1-${RUN}`);
    await expect(feedMessages(psychic).filter({ hasText: `gap-1-${RUN}` })).toHaveCount(1);
    await send(psychic, `gap-2-${RUN}`);
    await fill;
    const one = feedMessages(client).filter({ hasText: `gap-1-${RUN}` });
    const two = feedMessages(client).filter({ hasText: `gap-2-${RUN}` });
    await expect(one).toHaveCount(1);
    await expect(two).toHaveCount(1);
    const [a, b] = [Number(await one.getAttribute("data-seq")), Number(await two.getAttribute("data-seq"))];
    expect(b).toBe(a + 1);
    expectContiguous(await seqsOnScreen(client));
  });

  test("after a reconnect, messages the socket never delivered are backfilled without duplicates", async () => {
    clientProxy.dropAll = true; // the socket stays up, but these never arrive
    for (const n of [1, 2, 3]) await send(psychic, `away-${n}-${RUN}`);
    await expect(feedMessages(psychic).filter({ hasText: `away-3-${RUN}` })).toHaveCount(1);
    await client.waitForTimeout(500);
    await expect(feedMessages(client).filter({ hasText: `away-1-${RUN}` })).toHaveCount(0);

    const fill = client.waitForRequest((r) => /\/messages\/since\?after_seq=\d+/.test(r.url()), { timeout: 30_000 });
    await clientProxy.cut(); // cut the socket; replayed events keep being dropped
    await fill;
    for (const n of [1, 2, 3]) await expect(feedMessages(client).filter({ hasText: `away-${n}-${RUN}` })).toHaveCount(1);
    clientProxy.dropAll = false;
    const seqs = await seqsOnScreen(client);
    expect(new Set(seqs).size).toBe(seqs.length);
    expectContiguous(seqs);
    // The session resumed: both sides can talk again.
    await expect(client.getByText("Live chat in progress")).toBeVisible({ timeout: 30_000 });
    await send(client, `back-${RUN}`);
    await expect(feedMessages(psychic).filter({ hasText: `back-${RUN}` })).toHaveCount(1);
  });

  test("messages typed offline wait in the outbox, survive a reload and are delivered in order", async () => {
    clientProxy.block = true;
    await clientProxy.cut();
    await clientCtx.setOffline(true);
    const banner = client.getByTestId("connection-banner");
    await expect(banner).toContainText("You are offline. Messages will be sent when your connection returns.");
    await send(client, `offline-1-${RUN}`);
    await send(client, `offline-2-${RUN}`);
    const first = feedMessages(client).filter({ hasText: `offline-1-${RUN}` });
    await expect(first.getByTestId("message-status")).toHaveText("Waiting for connection…");
    await expect(banner).toContainText("2 waiting to send.");
    const stored = await client.evaluate(() => {
      const key = Object.keys(sessionStorage).find((k) => k.startsWith("staryield.chat.outbox:"));
      return key ? (JSON.parse(sessionStorage.getItem(key) ?? "[]") as { body: string }[]).map((e) => e.body) : [];
    });
    expect(stored).toEqual([`offline-1-${RUN}`, `offline-2-${RUN}`]);
    await expectNoA11yViolations(client, ["[data-testid=chat-feed]", "[data-testid=connection-banner]"]);

    // Back online but the realtime server is still unreachable: reload.
    await clientCtx.setOffline(false);
    await client.reload();
    await expect(feedMessages(client).filter({ hasText: `offline-1-${RUN}` }).getByTestId("message-status")).toBeVisible();
    await expect(feedMessages(client).filter({ hasText: `offline-2-${RUN}` })).toHaveCount(1);
    await expect(feedMessages(psychic).filter({ hasText: `offline-1-${RUN}` })).toHaveCount(0);

    // The connection returns: the outbox flushes in order, exactly once.
    clientProxy.block = false;
    await clientProxy.cut();
    for (const page of [client, psychic]) {
      await expect(feedMessages(page).filter({ hasText: `offline-1-${RUN}` })).toHaveCount(1, { timeout: 45_000 });
      await expect(feedMessages(page).filter({ hasText: `offline-2-${RUN}` })).toHaveCount(1, { timeout: 45_000 });
    }
    await expect(feedMessages(client).getByTestId("message-status")).toHaveCount(0);
    const order = await feedMessages(psychic).evaluateAll((els, run) => els.map((el) => el.textContent ?? "").filter((t) => t.includes(`offline-`) && t.includes(run)), RUN);
    expect(order[0]).toContain("offline-1");
    expect(order[1]).toContain("offline-2");
    expectContiguous(await seqsOnScreen(client));
    expect(await client.evaluate(() => Object.keys(sessionStorage).filter((k) => k.startsWith("staryield.chat.outbox:")).length)).toBe(0);
  });

  test("'Jump to latest' appears when new messages arrive while reading history", async () => {
    for (let i = 1; i <= 12; i++) await send(psychic, `filler ${i} ${RUN}\nline two\nline three`);
    await expect(feedMessages(client).filter({ hasText: `filler 12 ${RUN}` })).toHaveCount(1);
    const feed = client.getByTestId("chat-feed");
    await feed.evaluate((el) => el.scrollTo({ top: 0 }));
    await expect(feed).not.toHaveAttribute("data-at-bottom", "true");
    await send(psychic, `latest-${RUN}`);
    const pill = client.getByTestId("jump-to-latest");
    await expect(pill).toBeVisible();
    await expect(pill).toContainText("Jump to latest");
    await expect(pill).toContainText("1 new");
    // Screen readers hear the incoming message (never the client's own).
    await expect(client.getByTestId("chat-live-log")).toContainText(`latest-${RUN}`);
    await expect(client.getByTestId("chat-live-log")).not.toContainText(`back-${RUN}`);
    await pill.click();
    await expect(pill).toBeHidden();
    await expect(feed).toHaveAttribute("data-at-bottom", "true");
    await expect(feedMessages(client).filter({ hasText: `latest-${RUN}` })).toBeInViewport();
  });

  test("read receipts: the advisor sees 'Seen' once the client has read up to the message", async () => {
    await send(psychic, `please read ${RUN}`);
    const mine = feedMessages(psychic).filter({ hasText: `please read ${RUN}` });
    await expect(mine.getByTestId("message-seen")).toBeVisible();
    // Only the newest read message carries the label.
    await expect(feedMessages(psychic).getByTestId("message-seen")).toHaveCount(1);
  });

  test("the advisor's header shows live session earnings in money next to the timer", async () => {
    const earned = psychic.getByTestId("session-earnings");
    await expect(earned).toBeVisible();
    await expect(earned).toHaveText(/\(\$\d+\.\d{2}\)/);
    await expect(psychic.getByRole("timer")).toHaveText(/^\d{2}:\d{2}:\d{2}$/);
    // A per-minute billing event updates it live (Intl currency formatting).
    psychicProxy.inject({
      id: "",
      type: "session.billed",
      ts: new Date().toISOString(),
      data: { session_id: sessionId, minute: 9, free: false, charged: 40, next_billing_at: new Date(Date.now() + 60_000).toISOString(), active_seconds: 500, psychic_earnings: 2017, psychic_earnings_cents: 4033, currency: "USD" },
    });
    await expect(earned).toContainText("($40.33)");
    await expect(earned).toContainText("Session earnings: $40.33");
    // Clients never see the advisor's earnings.
    await expect(client.getByTestId("session-earnings")).toHaveCount(0);
    // The "End chat" button's accessible name predates this work (label-content-name-mismatch); the rest of the header is checked.
    await expectNoA11yViolations(psychic, ["[data-testid=chat-header-status]", "[data-testid=session-earnings]", "[role=timer]"]);
  });

  test("on a phone: session tools, the advisor switcher and the side panel as sheets (Figma 867:16787, 1261:1537)", async () => {
    await client.setViewportSize({ width: 390, height: 844 });
    const tools = client.getByTestId("chat-session-tools");
    await expect(tools).toBeVisible();
    await expect(tools.getByRole("link", { name: /Balance: .* credits/ })).toBeVisible();
    await expectNoHorizontalScroll(client);

    await client.getByRole("button", { name: "Open conversations" }).click();
    const switcher = client.getByTestId("chat-switcher");
    await expect(switcher).toBeVisible();
    await expect(switcher.getByTestId("switcher-current")).toContainText("Ramone");
    await expect(switcher.getByRole("note")).toContainText("Confirming ends your paid session with Ramone.");
    await expect(switcher.getByRole("link", { name: "Leave Chatroom" })).toHaveAttribute("href", "/en/psychics");
    // Contrast is measured once the sheet has finished fading in.
    await expect.poll(() => switcher.evaluate((el) => el.getAnimations().length)).toBe(0);
    await expectNoA11yViolations(client, ["[data-testid=chat-switcher]"]);
    await client.keyboard.press("Escape");
    await expect(switcher).toBeHidden();

    await tools.getByRole("button", { name: "Bookmarks" }).click();
    await expect(client.getByTestId("reader-panel").filter({ visible: true })).toBeVisible();
    await client.keyboard.press("Escape");
    await client.setViewportSize({ width: 1440, height: 900 });
  });
});
