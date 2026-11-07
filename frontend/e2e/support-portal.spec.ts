/**
 * Revisions 3.0 support desk, requester side (§16.2-16.3): the contact form
 * lands in the signed-in list and thread, replies, and the anonymous private
 * link (token in the URL fragment, sent only as a header).
 *
 * The signed-in journey needs the merged backend. The anonymous link page is
 * also exercised with intercepted API answers for states that need staff
 * action (staff replies, closed, rate limited).
 */
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { CLIENT, expectNoHorizontalScroll, signIn } from "./helpers";

const TOKEN = "dGVzdC10b2tlbi1mb3ItZTJlLXN1cHBvcnQtbGluay0xMjM";
const REF = "SY-E2E123";

type Msg = { id: string; author: { type: "customer" | "staff"; name: string }; body: string; created_at: string };
function ticket(over: Record<string, unknown> = {}, messages?: Msg[]) {
  return {
    reference: REF,
    subject: "Where is my invoice?",
    topic: "billing",
    status: "pending",
    awaiting: "you",
    created_at: "2026-11-01T10:00:00Z",
    last_message_at: "2026-11-02T09:00:00Z",
    can_reply: true,
    messages: messages ?? [
      { id: "m1", author: { type: "customer", name: "Jane" }, body: "Line one\nLine two " + "x".repeat(200), created_at: "2026-11-01T10:00:00Z" },
      { id: "m2", author: { type: "staff", name: "Staryield Support · Anna" }, body: "We sent it again.", created_at: "2026-11-02T09:00:00Z" },
    ],
    ...over,
  };
}
const err = (status: number, code: string, headers: Record<string, string> = {}) => ({
  status,
  // Cross-origin (site → API): browsers only expose Retry-After when the API
  // lists it, as the real API does (router.go ExposeHeaders).
  headers: { "access-control-expose-headers": "Retry-After, X-Request-ID", ...headers },
  json: { error: { code, message: code } },
});

async function openLink(page: Page, path = `/en/support/${REF}#t=${TOKEN}`) {
  await page.goto(path);
}

test.describe("anonymous private link (intercepted API)", { tag: "@responsive" }, () => {
  test("the token leaves the address bar, goes only in the X-Support-Token header and the page is not indexable", async ({ page }) => {
    const seen: { url: string; token?: string }[] = [];
    await page.route(`**/api/v1/support/tickets/${REF}`, (route) => {
      seen.push({ url: route.request().url(), token: route.request().headers()["x-support-token"] });
      return route.fulfill({ json: ticket() });
    });
    await openLink(page);
    await expect(page.getByTestId("ticket-thread")).toBeVisible();
    await expect.poll(() => page.url()).not.toContain(TOKEN);
    expect(new URL(page.url()).hash).toBe("");
    // React Strict Mode (development) runs the loading effect twice: every
    // request must carry the token in the header and never in the URL.
    expect(seen.length).toBeGreaterThan(0);
    for (const request of seen) {
      expect(request.token).toBe(TOKEN);
      expect(request.url).not.toContain(TOKEN);
    }
    expect(await page.locator('meta[name="referrer"]').getAttribute("content")).toBe("no-referrer");
    expect(await page.locator('meta[name="robots"]').getAttribute("content")).toMatch(/noindex/);

    // A reload keeps working for this tab (sessionStorage), still without the token in the URL.
    await page.reload();
    await expect(page.getByTestId("ticket-thread")).toBeVisible();
    expect(seen.at(-1)?.token).toBe(TOKEN);
    await expectNoHorizontalScroll(page);
  });

  test("renders the status chip, names, dates and long bodies without overflow", async ({ page }) => {
    await page.route(`**/api/v1/support/tickets/${REF}`, (route) => route.fulfill({ json: ticket() }));
    await openLink(page);
    await expect(page.getByTestId("ticket-status")).toHaveText("Awaiting your reply");
    const messages = page.getByTestId("ticket-message");
    await expect(messages).toHaveCount(2);
    await expect(messages.nth(0)).toContainText("Jane");
    await expect(messages.nth(1)).toContainText("Staryield Support · Anna");
    await expect(messages.nth(0).locator("time")).toHaveAttribute("datetime", "2026-11-01T10:00:00Z");
    await expectNoHorizontalScroll(page);
    await expectNoA11yViolations(page, ['[data-testid="ticket-thread"]']);
  });

  test("a refused or missing token shows the expired-link state with a CTA to the contact form", async ({ page }) => {
    await page.route(`**/api/v1/support/tickets/${REF}`, (route) => route.fulfill(err(404, "not_found")));
    await openLink(page);
    const box = page.getByTestId("support-link-invalid");
    await expect(box).toContainText("This link has expired or is not valid");
    await box.getByRole("link", { name: "Send a new request" }).click();
    await expect(page).toHaveURL(/\/en\/contact$/);
    await openLink(page, `/en/support/${REF}`);
    await expect(page.getByTestId("support-link-invalid")).toBeVisible();
  });

  test("an invalid token against the real API ends in the same state", async ({ page }) => {
    await openLink(page, `/en/support/SY-ZZZZ99#t=${TOKEN}`);
    await expect(page.getByTestId("support-link-invalid")).toBeVisible();
    expect(page.url()).not.toContain(TOKEN);
  });

  test("replying: validation, success refreshes the thread, then closed and rate-limited answers", async ({ page }) => {
    let replies = 0;
    await page.route(`**/api/v1/support/tickets/${REF}/messages`, async (route) => {
      replies++;
      if (replies === 1)
        return route.fulfill({
          json: ticket({ status: "open", awaiting: "support" }, [
            ...ticket().messages,
            { id: "m3", author: { type: "customer", name: "Jane" }, body: "Thanks!", created_at: "2026-11-03T09:00:00Z" },
          ]),
        });
      if (replies === 2) return route.fulfill(err(429, "rate_limited", { "Retry-After": "90" }));
      return route.fulfill(err(409, "invalid_state"));
    });
    await page.route(`**/api/v1/support/tickets/${REF}`, (route) => route.fulfill({ json: ticket() }));
    await openLink(page);
    const box = page.getByRole("textbox", { name: "Your reply" });
    await page.getByRole("button", { name: "Send reply" }).click();
    await expect(page.getByText("This field is required.")).toBeVisible();
    await box.fill("x".repeat(5001));
    await page.getByRole("button", { name: "Send reply" }).click();
    await expect(page.getByText("Use at most 5000 characters.")).toBeVisible();
    await expect(page.locator('[data-slot="field-counter"]')).toHaveText("5001/5000");
    expect(replies).toBe(0);

    await box.fill("Thanks!");
    await page.getByRole("button", { name: "Send reply" }).click();
    await expect(page.getByTestId("ticket-message")).toHaveCount(3);
    await expect(page.getByTestId("ticket-status")).toHaveText("Awaiting support");
    await expect(page.getByText("Reply sent. We will get back to you by email.").first()).toBeVisible();
    await expect(box).toHaveValue("");

    await box.fill("Again");
    await page.getByRole("button", { name: "Send reply" }).click();
    await expect(page.getByRole("alert").filter({ hasText: "too fast" })).toContainText("about 2 minutes");
    await expect(box).toHaveValue("Again");

    await page.getByRole("button", { name: "Send reply" }).click();
    const closed = page.getByTestId("ticket-closed");
    await expect(closed).toContainText("This request is closed");
    await expect(closed.getByRole("link", { name: "Send a new request" })).toHaveAttribute("href", "/en/contact");
    await expect(page.getByTestId("ticket-status")).toHaveText("Closed");
  });

  test("a closed request shows the notice instead of the composer; failures can be retried", async ({ page }) => {
    // Fail until "Try again" (the API client itself retries GETs, and Strict
    // Mode loads twice in development), then answer with a closed request.
    let failing = true;
    await page.route(`**/api/v1/support/tickets/${REF}`, (route) =>
      failing ? route.fulfill(err(500, "internal_error")) : route.fulfill({ json: ticket({ status: "closed", awaiting: "none", can_reply: false }) }),
    );
    await openLink(page);
    const retry = page.getByRole("alert").getByRole("button", { name: "Try again" });
    await expect(retry).toBeVisible();
    failing = false;
    await retry.click();
    await expect(page.getByTestId("ticket-closed")).toBeVisible();
    await expect(page.getByRole("textbox", { name: "Your reply" })).toHaveCount(0);
    await expect(page.getByTestId("ticket-status")).toHaveText("Closed");
  });
});

test.describe("signed-in support requests", () => {
  test("contact form → list → thread → reply, and the account menu entry", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto("/en/contact");
    const subject = `E2E support ${Date.now()}`;
    await page.getByRole("textbox", { name: "Subject" }).fill(subject);
    await page.getByRole("textbox", { name: /Describe your issue/ }).fill("Please help me understand my last invoice.");
    await page.getByRole("checkbox").check();
    await page.locator("form button[type=submit]").click();
    const success = page.getByTestId("contact-success");
    await expect(success).toBeVisible();
    const reference = (await success.getByTestId("contact-reference").textContent())!.trim();
    await expect(success.getByTestId("contact-follow")).toBeVisible();
    await success.getByRole("link", { name: "your support requests" }).click();
    await expect(page).toHaveURL(new RegExp(`/en/support/requests/${reference}$`));

    const thread = page.getByTestId("ticket-thread");
    await expect(thread).toContainText(subject);
    await expect(page.getByTestId("ticket-status")).toHaveText("Awaiting support");
    await expect(page.getByTestId("ticket-message")).toHaveCount(1);
    await page.getByRole("textbox", { name: "Your reply" }).fill("One more detail: it was in October.");
    await page.getByRole("button", { name: "Send reply" }).click();
    await expect(page.getByTestId("ticket-message")).toHaveCount(2);
    await expect(page.getByText("Reply sent. We will get back to you by email.").first()).toBeVisible();

    // The list shows it, and the account menu links to the list.
    await page.goto("/en/support/requests");
    await expect(page.getByTestId("request-row").filter({ hasText: reference })).toBeVisible();
    await expectNoA11yViolations(page, ["main"]);
    await page.locator("[data-testid=user-menu-trigger], [data-testid=mobile-menu-trigger]").filter({ visible: true }).first().click();
    await expect(page.getByRole("menuitem", { name: "Support requests" }).or(page.getByRole("link", { name: "Support requests" })).first()).toBeVisible();
  });

  test("the list paginates", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    const row = (n: number) => ({ reference: `SY-PAGE${String(n).padStart(2, "0")}`, subject: `Subject ${n}`, topic: "general", status: "open", awaiting: "support", created_at: "2026-11-01T10:00:00Z", last_message_at: "2026-11-02T10:00:00Z" });
    await page.route("**/api/v1/me/support/tickets*", (route) => {
      const cursor = new URL(route.request().url()).searchParams.get("cursor");
      return route.fulfill({ json: cursor ? { items: [row(21)] } : { items: Array.from({ length: 20 }, (_, i) => row(i + 1)), next_cursor: "c2" } });
    });
    await page.goto("/en/support/requests");
    await expect(page.getByTestId("request-row")).toHaveCount(20);
    await page.getByRole("button", { name: "Next", exact: true }).click();
    await expect(page.getByTestId("request-row")).toHaveCount(1);
    await page.getByRole("button", { name: "Previous", exact: true }).click();
    await expect(page.getByTestId("request-row")).toHaveCount(20);
  });

  test("another user's request is not found, with a way back", async ({ page }) => {
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto("/en/support/requests/SY-NOTMINE");
    await expect(page.getByTestId("support-not-found")).toBeVisible();
    await page.getByRole("link", { name: "Back to your requests" }).click();
    await expect(page).toHaveURL(/\/en\/support\/requests$/);
  });
});

test.describe("anonymous visitors", () => {
  test("the signed-in pages redirect to the login page and come back", async ({ page }) => {
    await page.goto("/en/support/requests");
    await page.waitForURL(/\/en\/login\?next=/);
    expect(new URL(page.url()).searchParams.get("next")).toBe("/en/support/requests");
    await page.goto(`/en/support/requests/${REF}`);
    await page.waitForURL(/\/en\/login\?next=/);
    expect(new URL(page.url()).searchParams.get("next")).toBe(`/en/support/requests/${REF}`);
  });

  test("the contact form promises a private link", async ({ page }) => {
    await page.goto("/en/contact");
    await page.getByRole("textbox", { name: "Subject" }).fill("Anonymous question");
    await page.getByRole("textbox", { name: "Email" }).fill(`e2e+support-${Date.now()}@staryield.test`);
    await page.getByRole("textbox", { name: /Describe your issue/ }).fill("I have a question about my reading.");
    await page.getByRole("checkbox").check();
    await page.locator("form button[type=submit]").click();
    await expect(page.getByTestId("contact-private-link")).toHaveText("We e-mailed you a private link to follow your request");
    await expect(page.getByTestId("contact-follow")).toHaveCount(0);
  });
});
