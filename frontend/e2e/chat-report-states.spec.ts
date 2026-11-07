/**
 * "Report an issue" (§16.4) with intercepted API answers, for the states
 * that cannot be produced on demand (rate limit, outage, duplicate, field
 * errors), and the Support requests views for conversation reports.
 * Only the report endpoints are mocked: the chatroom itself is the real one.
 */
import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { createConversation, reportTrigger } from "./chat-report-helpers";
import { CLIENT, expectNoHorizontalScroll, signIn } from "./helpers";

const REF = "SY-RPT123";
const err = (status: number, code: string, extra: Record<string, unknown> = {}, headers: Record<string, string> = {}) => ({
  status,
  headers: { "access-control-expose-headers": "Retry-After, X-Request-ID", ...headers },
  json: { error: { code, message: code, ...extra } },
});
const reportRow = {
  reference: REF,
  subject: "Chat report · The advisor stopped responding · Theo",
  topic: "advisor",
  status: "open",
  awaiting: "support",
  created_at: "2026-11-01T10:00:00Z",
  last_message_at: "2026-11-02T09:00:00Z",
  kind: "chat_report",
  report: { reason: "advisor_unresponsive", conversation_id: "c1", advisor: { name: "Theo", slug: "theo" }, session_started_at: "2026-11-01T09:30:00Z" },
};
const contactRow = { ...reportRow, reference: "SY-CNT123", subject: "Where is my invoice?", topic: "billing", kind: "contact", report: undefined };

test.describe("conversation reports in Support requests (intercepted API)", { tag: "@responsive" }, () => {
  test("the list and the detail tell a report apart from a contact request", async ({ page }) => {
    await page.route("**/api/v1/me/support/tickets?**", (route) => route.fulfill({ json: { items: [reportRow, contactRow] } }));
    await page.route(`**/api/v1/me/support/tickets/${REF}`, (route) =>
      route.fulfill({
        json: { ...reportRow, can_reply: true, messages: [{ id: "m1", author: { type: "customer", name: "Jane" }, body: "Stopped answering", created_at: "2026-11-01T10:00:00Z" }] },
      }),
    );
    await signIn(page, CLIENT.email, CLIENT.password);
    await page.goto("/en/support/requests");
    const rows = page.getByTestId("request-row");
    await expect(rows).toHaveCount(2);
    await expect(rows.nth(0).getByTestId("report-badge")).toHaveText("Conversation report");
    await expect(rows.nth(0).getByTestId("request-title")).toHaveText("The advisor stopped responding");
    await expect(rows.nth(0)).toContainText("With Theo");
    await expect(rows.nth(1).getByTestId("report-badge")).toHaveCount(0);
    await expect(rows.nth(1).getByTestId("request-title")).toHaveText("Where is my invoice?");
    await expectNoHorizontalScroll(page);
    await expectNoA11yViolations(page, ["main"]);

    await rows.nth(0).getByRole("link").click();
    await expect(page.getByTestId("report-context")).toContainText("Report about your conversation with Theo");
    await expect(page.getByTestId("report-open-conversation")).toHaveAttribute("href", "/en/chatroom?reader=theo");
    await expectNoHorizontalScroll(page);
    await expectNoA11yViolations(page, ['[data-testid="ticket-thread"]']);
  });
});

async function openDialog(page: Page, submit = true) {
  // A dialog that is still closing would swallow the click on the trigger.
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await (await reportTrigger(page)).click();
  const dialog = page.getByRole("dialog", { name: "Report an issue" });
  await expect(dialog.getByTestId("report-form")).toBeVisible();
  if (submit) {
    await dialog.getByTestId("report-reason-billing").click();
    await dialog.getByTestId("report-submit").click();
  }
  return dialog;
}

test.describe("report dialog error states (intercepted report API)", { tag: "@responsive" }, () => {
  test("rate limit, outage with retry, duplicate and field errors", async ({ browser, viewport }) => {
    const setup = await createConversation(browser, viewport ?? undefined);
    const { client } = setup;
    try {
      await client.route("**/api/v1/me/support/chat-reports?**", (route) => route.fulfill({ json: { open: null } }));
      const post = (handler: Parameters<Page["route"]>[1]) => client.route("**/api/v1/me/support/chat-reports", (route) => (route.request().method() === "POST" ? handler(route, route.request()) : route.fallback()));

      // 429 with Retry-After
      await post((route) => route.fulfill(err(429, "rate_limited", {}, { "Retry-After": "90" })));
      await client.goto(client.url());
      let dialog = await openDialog(client);
      await expect(dialog.getByTestId("report-error")).toContainText("Try again in about 2 minutes.");
      await dialog.getByRole("button", { name: "Back to chat" }).click();
      await expect(dialog).toHaveCount(0);
      await client.unroute("**/api/v1/me/support/chat-reports");
      await client.route("**/api/v1/me/support/chat-reports?**", (route) => route.fulfill({ json: { open: null } }));

      // Network failure, then Retry succeeds
      let calls = 0;
      await post((route) => {
        calls++;
        if (calls === 1) return route.abort("failed");
        return route.fulfill({ status: 201, json: { ...reportRow, can_reply: true, messages: [] } });
      });
      dialog = await openDialog(client);
      const notice = dialog.getByTestId("report-error");
      await expect(notice).toContainText("We could not send your report");
      await expectNoA11yViolations(client, ['[role="dialog"]']);
      await notice.getByTestId("report-retry").click();
      await expect(dialog.getByTestId("report-success").getByTestId("report-reference")).toHaveText(REF);
      expect(calls).toBe(2);
      await dialog.getByTestId("report-back").click();
      await client.unroute("**/api/v1/me/support/chat-reports");
      await client.route("**/api/v1/me/support/chat-reports?**", (route) => route.fulfill({ json: { open: null } }));

      // 409 report_exists switches to the existing report
      await post((route) => route.fulfill(err(409, "report_exists", { details: { reference: "SY-DUP999" } })));
      dialog = await openDialog(client);
      await expect(dialog.getByTestId("report-exists").getByTestId("report-reference")).toHaveText("SY-DUP999");
      await expect(dialog.getByTestId("report-view")).toHaveAttribute("href", "/en/support/requests/SY-DUP999");
      await dialog.getByTestId("report-back").click();
      await client.unroute("**/api/v1/me/support/chat-reports");
      await client.route("**/api/v1/me/support/chat-reports?**", (route) => route.fulfill({ json: { open: null } }));

      // 422 puts the message on the field
      await post((route) => route.fulfill(err(422, "validation_failed", { details: { fields: { details: "Please shorten your details." } } })));
      dialog = await openDialog(client);
      await expect(dialog.getByText("Please shorten your details.")).toBeVisible();
      await expectNoHorizontalScroll(client);
    } finally {
      await setup.close();
    }
  });
});
