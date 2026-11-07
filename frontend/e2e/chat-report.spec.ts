/**
 * Revisions 3.1 "Report an issue" (§16.4) against the real API: a client with
 * a conversation reports it from the chatroom, sees the report in Support
 * requests with its own badge and title, and cannot file a second one while
 * the first is open. Needs the merged backend and the chatroom placement.
 */
import { expect, test } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { createConversation, reportTrigger } from "./chat-report-helpers";
import { expectNoHorizontalScroll } from "./helpers";

test.describe("report an issue with a conversation", { tag: "@responsive" }, () => {
  test("validation, submit, the report in Support requests, and the duplicate guard", async ({ browser, viewport }) => {
    const setup = await createConversation(browser, viewport ?? undefined);
    const { client } = setup;
    try {
      const trigger = await reportTrigger(client);
      await expect(trigger).toHaveAccessibleName("Report an issue");
      const box = await trigger.boundingBox();
      // Small to look at, comfortable to touch.
      expect(box!.width).toBeLessThanOrEqual(40);
      await trigger.click();
      const dialog = client.getByRole("dialog", { name: "Report an issue" });
      await expect(dialog).toBeVisible();
      await expect(dialog).toContainText("About your conversation with");

      const form = dialog.getByTestId("report-form");
      const alreadyReported = dialog.getByTestId("report-exists");
      await expect(form.or(alreadyReported)).toBeVisible();

      let reference: string;
      if (await form.isVisible()) {
        await expectNoA11yViolations(client, ['[role="dialog"]']);
        await dialog.getByTestId("report-submit").click();
        await expect(dialog.getByText("Choose what went wrong.")).toBeVisible();

        await dialog.getByTestId("report-reason-other").click();
        await dialog.getByTestId("report-submit").click();
        await expect(dialog.getByText("Tell us what happened (at least 10 characters).")).toBeVisible();
        const details = dialog.getByRole("textbox");
        await details.fill("The advisor stopped answering after my first question.");
        await expect(dialog.locator('[data-slot="field-counter"]')).toHaveText(/^\d+\/2000$/);
        await expect(dialog.getByTestId("report-privacy")).toContainText("is not told about your report");
        await expectNoHorizontalScroll(client);

        await dialog.getByTestId("report-reason-advisor_unresponsive").click();
        await dialog.getByTestId("report-submit").click();
        const success = dialog.getByTestId("report-success");
        await expect(success).toContainText("Thanks — your report was sent.");
        reference = (await success.getByTestId("report-reference").textContent()) ?? "";
        expect(reference).toMatch(/^SY-[A-Z0-9]{4,16}$/);
        await expectNoA11yViolations(client, ['[role="dialog"]']);

        // The live chat stays where it was; Back to chat closes and returns focus.
        const url = client.url();
        await success.getByTestId("report-back").click();
        await expect(dialog).toHaveCount(0);
        await expect(trigger).toBeFocused();
        expect(client.url()).toBe(url);
      } else {
        // A report from an earlier run is still open: the duplicate guard is what is checked here.
        reference = (await alreadyReported.getByTestId("report-reference").textContent()) ?? "";
        await client.keyboard.press("Escape");
        await expect(dialog).toHaveCount(0);
      }

      // Reopening shows the report instead of the form.
      await trigger.click();
      const again = client.getByRole("dialog", { name: "Report an issue" }).getByTestId("report-exists");
      await expect(again).toContainText("You already reported this conversation");
      await expect(again.getByTestId("report-reference")).toHaveText(reference);
      await expect(client.getByTestId("report-form")).toHaveCount(0);
      await expectNoA11yViolations(client, ['[role="dialog"]']);
      await again.getByTestId("report-view").click();

      // Support requests: badge, localized reason as the title, advisor meta line.
      await expect(client).toHaveURL(new RegExp(`/en/support/requests/${reference}$`));
      const thread = client.getByTestId("ticket-thread");
      await expect(thread).toBeVisible();
      await expect(thread.getByTestId("report-badge")).toHaveText("Conversation report");
      await expect(thread.getByTestId("report-meta")).toContainText("With ");
      const card = client.getByTestId("report-context");
      await expect(card).toContainText("Report about your conversation with");
      await expect(card.getByTestId("report-context-reason")).not.toBeEmpty();
      await expect(card.getByTestId("report-open-conversation")).toHaveAttribute("href", /\/en\/chatroom\?reader=[a-z0-9-]+$/);
      await expectNoHorizontalScroll(client);
      await expectNoA11yViolations(client, ['[data-testid="ticket-thread"]']);

      await client.goto("/en/support/requests");
      const row = client.getByTestId("request-row").filter({ hasText: reference });
      await expect(row).toBeVisible();
      await expect(row.getByTestId("report-badge")).toHaveText("Conversation report");
      await expect(row.getByTestId("request-title")).not.toContainText("Chat report ·");
      await expectNoHorizontalScroll(client);
      await expectNoA11yViolations(client, ["main"]);

      await row.getByRole("link").click();
      await client.getByTestId("report-open-conversation").click();
      await expect(client).toHaveURL(/\/en\/chatroom\?reader=/);
    } finally {
      await setup.close();
    }
  });
});
