/**
 * The full live-chat journey with two browsers: a client and an advisor.
 * Runs against the real API and realtime server; steps are serial and share
 * the two sessions.
 */
import { expect, test, type BrowserContext, type Page } from "@playwright/test";

import { CLIENT, advisor, expectImageLoaded, expectNoHorizontalScroll, pngFixture, signedInPage } from "./helpers";

const THEO = advisor("theo");
const LINK = "https://staryield.net/en/psychics";
const COMMENT = `Wonderful reading ${Date.now()}`;

test.describe.serial("live chat between a client and an advisor", () => {
  let clientCtx: BrowserContext;
  let psychicCtx: BrowserContext;
  let client: Page;
  let psychic: Page;

  test.beforeAll(async ({ browser }) => {
    ({ context: psychicCtx, page: psychic } = await signedInPage(browser, THEO.email, THEO.password));
    ({ context: clientCtx, page: client } = await signedInPage(browser, CLIENT.email, CLIENT.password));
  });

  test.afterAll(async () => {
    // Leave the advisor offline for the next run.
    await psychic.goto("/en/profile").catch(() => undefined);
    await psychic.getByText("● Offline", { exact: true }).click().catch(() => undefined);
    await clientCtx?.close();
    await psychicCtx?.close();
  });

  test("the advisor goes online and opens the chatroom", async () => {
    await psychic.goto("/en/profile");
    const availability = psychic.waitForResponse((r) => r.url().endsWith("/api/v1/psychic/availability") && r.ok());
    await psychic.getByText("● Online", { exact: true }).click();
    await availability;
    await psychic.getByRole("link", { name: "Chatroom", exact: true }).first().click();
    await psychic.waitForURL(/\/en\/chatroom/);
  });

  test("the client requests a chat and the advisor accepts", async () => {
    await client.goto(`/en/chatroom?reader=${THEO.slug}`);
    const start = client.getByRole("button", { name: /Start live chat/ });
    await expect(start).toBeEnabled();
    await start.click();
    await expect(client.getByText(/Waiting for .* to accept/)).toBeVisible();

    const accept = psychic.getByRole("button", { name: "Accept", exact: true });
    await expect(accept).toBeVisible();
    await accept.click();

    await expect(client.getByText("Live chat in progress")).toBeVisible();
    await expect(psychic.getByText("Live chat in progress")).toBeVisible();
  });

  test("the advisor sees the client panel and presence", async () => {
    const panel = psychic.getByTestId("client-panel").filter({ visible: true });
    await expect(panel).toBeVisible();
    await expect(panel.getByTestId("client-presence")).toHaveText("Online");
    await expect(panel.getByTestId("client-sessions")).toBeVisible();
    // Never sensitive data.
    await expect(panel).not.toContainText(CLIENT.email);
    await expect(panel).not.toContainText(/balance/i);
    // Presence dot in the inbox row.
    await expect(psychic.locator("aside .presence-dot[data-online]").first()).toBeVisible();
  });

  test("a multiline message with a link and emoji keeps its formatting", async () => {
    const box = client.getByRole("textbox", { name: "Message" });
    await box.click();
    await box.pressSequentially("Hello Theo 🔮");
    await box.press("Shift+Enter");
    await box.pressSequentially(`   see ${LINK}.`);
    await expect(box).toHaveValue(`Hello Theo 🔮\n   see ${LINK}.`);
    await box.press("Enter");
    await expect(box).toHaveValue("");

    for (const page of [client, psychic]) {
      const message = page.getByTestId("chat-message").filter({ hasText: "Hello Theo" }).last();
      await expect(message).toBeVisible();
      const text = message.locator(".message-text");
      await expect(text).toHaveCSS("white-space", "pre-wrap");
      expect(await text.textContent()).toBe(`Hello Theo 🔮\n   see ${LINK}.`);
      const link = message.getByRole("link", { name: LINK });
      await expect(link).toHaveAttribute("href", LINK);
      await expect(link).toHaveAttribute("target", "_blank");
      await expect(link).toHaveAttribute("rel", "noopener noreferrer nofollow ugc");
    }

    // Emoji-only messages are shown larger.
    await box.fill("🔮✨");
    await box.press("Enter");
    await expect(psychic.locator(".message-text[data-emoji-only]").filter({ hasText: "🔮✨" }).last()).toBeVisible();
  });

  test("the emoji picker inserts at the caret", async () => {
    const box = psychic.getByRole("textbox", { name: "Message" });
    await box.fill("Thank you");
    await box.evaluate((el: HTMLTextAreaElement) => el.setSelectionRange(5, 5));
    await psychic.getByTestId("emoji-trigger").click();
    const picker = psychic.getByRole("dialog", { name: "Emoji" });
    await picker.getByRole("tab", { name: "Hearts" }).click();
    await picker.getByRole("button", { name: "💖" }).click();
    await psychic.keyboard.press("Escape");
    await expect(box).toHaveValue("Thank💖 you");
    await box.fill("");
  });

  test("typing is shown to the other side", async () => {
    await psychic.getByRole("textbox", { name: "Message" }).pressSequentially("I see the cards");
    await expect(client.getByRole("status", { name: /is typing/ })).toBeVisible();
    await psychic.getByRole("textbox", { name: "Message" }).press("Enter");
    await expect(client.getByTestId("chat-message").filter({ hasText: "I see the cards" })).toBeVisible();
  });

  test("an image attachment uploads and displays for both sides", async () => {
    await client.getByTestId("attach-input").setInputFiles({ name: "spread.png", mimeType: "image/png", buffer: pngFixture() });
    await expect(client.getByTestId("composer-attachment")).toBeVisible();
    const upload = client.waitForResponse((r) => /\/chat\/sessions\/[^/]+\/attachments$/.test(r.url()) && r.status() === 201);
    const box = client.getByRole("textbox", { name: "Message" });
    await box.fill("My spread\nthis morning");
    await box.press("Enter");
    const message = (await (await upload).json()).message as { id: string; kind: string; attachments: { id: string }[] };
    expect(message.kind).toBe("image");
    expect(message.attachments).toHaveLength(1);

    for (const page of [client, psychic]) {
      const bubble = page.getByTestId("chat-message").filter({ hasText: "My spread" }).last();
      await expect(bubble.locator("[data-kind=image]")).toBeVisible();
      await expectImageLoaded(page, `[data-testid=chat-message]:has-text("My spread") .chat-image-frame img >> nth=-1`);
    }

    // Lightbox.
    await psychic.getByTestId("chat-message").filter({ hasText: "My spread" }).last().locator(".chat-image-frame").click();
    const lightbox = psychic.getByRole("dialog");
    await expect(lightbox.getByRole("img")).toBeVisible();
    await psychic.keyboard.press("Escape");
    await expect(lightbox).toHaveCount(0);
  });

  test("images are private to the two participants", async ({ request }) => {
    // Shown from blob URLs fetched with the session cookies (the sender reuses its own file).
    expect(await client.locator(".chat-image-frame img").last().getAttribute("src")).toMatch(/^blob:/);
    expect(await psychic.locator(".chat-image-frame img").last().getAttribute("src")).toMatch(/^blob:/);
    const ids = await psychic.evaluate(() => performance.getEntriesByType("resource").map((e) => e.name).filter((n) => n.includes("/chat/attachments/")));
    expect(ids.length).toBeGreaterThan(0);
    const anonymous = await request.get(ids[0]);
    expect([401, 403, 404]).toContain(anonymous.status());
  });

  test("the client ends the chat and rates it", async () => {
    await client.getByRole("button", { name: /End the chat with/ }).click();
    const summary = client.getByTestId("chat-summary");
    await expect(summary).toBeVisible();
    const form = summary.getByTestId("review-form");
    await expect(form).toBeVisible();
    // Submitting without stars is rejected client-side.
    await form.getByRole("button", { name: "Submit review" }).click();
    await expect(form.getByRole("alert")).toHaveText("Choose a rating from 1 to 5 stars.");
    await form.locator(".star-option").nth(4).click();
    await expect(form.getByRole("radio", { name: "5 out of 5 stars" })).toBeChecked();
    await form.getByRole("textbox").fill(COMMENT);
    const review = client.waitForResponse((r) => r.url().endsWith("/review") && r.request().method() === "POST");
    await form.getByRole("button", { name: "Submit review" }).click();
    expect((await review).status()).toBe(201);
    await expect(summary.getByTestId("review-thanks")).toContainText(COMMENT);
  });

  test("the advisor is notified and sees the review", async () => {
    const badge = psychic.getByTestId("notification-badge");
    await expect(badge).toBeVisible();
    await psychic.getByRole("button", { name: /Notifications/ }).click();
    await expect(psychic.locator(".notification-item").filter({ hasText: "New review" }).first()).toBeVisible();
    await psychic.keyboard.press("Escape");

    await psychic.goto("/en/profile/analytics");
    await expect(psychic.getByTestId("psychic-analytics")).toBeVisible();
    await expect(psychic.getByTestId("analytics-reviews")).toContainText(COMMENT);
  });

  test("the analytics page renders charts", async () => {
    await expect(psychic.getByTestId("kpi-sessions")).toBeVisible();
    await expect(psychic.getByTestId("score-card").locator("canvas")).toBeVisible();
    const earnings = psychic.getByTestId("earnings-card");
    await expect(earnings.locator("canvas").or(earnings.getByText("No earnings in this period yet."))).toBeVisible();
    await expect(psychic.getByTestId("hours-card").locator(".heatmap-cell:not(.heatmap-swatch)")).toHaveCount(7 * 24);
    // Range selector refetches with another interval.
    const refetch = psychic.waitForRequest((r) => r.url().includes("/api/v1/psychic/analytics") && r.url().includes("interval=month"));
    await psychic.getByText("12 months", { exact: true }).click();
    const url = new URL((await refetch).url());
    expect(url.searchParams.get("tz")).toBe("Europe/Paris");
  });

  test("withdrawal requests are validated", async () => {
    const form = psychic.getByTestId("withdrawal-form");
    await form.getByLabel("Amount (credits)").fill("1");
    await form.getByRole("button", { name: "Request withdrawal" }).click();
    await expect(form.getByText(/The minimum withdrawal is [\d,]+ credits\./)).toBeVisible();
    await expect(form.getByText("Enter where we should send the money.")).toBeVisible();

    await form.getByText("PayPal", { exact: true }).click();
    await form.getByLabel("PayPal email").fill("not-an-email");
    await form.getByRole("button", { name: "Request withdrawal" }).click();
    await expect(form.getByText("Enter the email address of your PayPal account.")).toBeVisible();
  });

  test("notifications can all be marked as read", async () => {
    await psychic.getByRole("button", { name: /Notifications/ }).click();
    const markAll = psychic.getByRole("button", { name: "Mark all as read" });
    if (await markAll.isEnabled()) {
      const read = psychic.waitForResponse((r) => r.url().endsWith("/api/v1/me/notifications/read") && r.ok());
      await markAll.click();
      await read;
    }
    await expect(psychic.getByTestId("notification-badge")).toHaveCount(0);
    await expect(psychic.locator(".notification-item[data-unread]")).toHaveCount(0);
    await psychic.keyboard.press("Escape");
  });

  test("chatroom and analytics fit a phone screen", async () => {
    await client.setViewportSize({ width: 390, height: 844 });
    await client.goto(`/en/chatroom?reader=${THEO.slug}`);
    await expect(client.getByRole("textbox", { name: "Message" })).toBeVisible();
    await expectNoHorizontalScroll(client);
    await client.setViewportSize({ width: 320, height: 640 });
    await expectNoHorizontalScroll(client);

    await psychic.setViewportSize({ width: 390, height: 844 });
    await psychic.goto("/en/profile/analytics");
    await expect(psychic.getByTestId("score-card")).toBeVisible();
    await expectNoHorizontalScroll(psychic);
    await psychic.setViewportSize({ width: 768, height: 1024 });
    await expectNoHorizontalScroll(psychic);
    await psychic.setViewportSize({ width: 1440, height: 900 });
  });

  test("the public profile lists the review", async () => {
    await client.setViewportSize({ width: 1280, height: 900 });
    await client.goto(`/en/psychics/${THEO.slug}`);
    const reviews = client.getByTestId("psychic-reviews");
    await expect(reviews).toBeVisible();
    await expect(reviews.locator(".review-item").first()).toBeVisible();
  });
});
