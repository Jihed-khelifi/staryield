// @vitest-environment jsdom
import type * as React from "react";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { ApiError } from "@/lib/api/errors";
import type { CustomerTicket, CustomerTicketRow } from "@/lib/api/types";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const api = vi.hoisted(() => ({ createChatReport: vi.fn(), openChatReport: vi.fn() }));
const analytics = vi.hoisted(() => ({ track: vi.fn() }));

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/api/endpoints", () => ({ supportApi: api }));
vi.mock("@/lib/analytics", () => analytics);

import { ReportIssueButton, type ReportIssueTarget } from "@/components/chat/report-issue";
import { TooltipProvider } from "@/components/ui/tooltip";

const target: ReportIssueTarget = { conversationId: "conv-1", sessionId: "sess-1", advisorName: "Theo", seq: 42 };
const apiError = (code: string, status: number, extra: Partial<{ retryAfter: number; details: Record<string, unknown> }> = {}) => new ApiError({ code, message: code, status, ...extra });
const row = { reference: "SY-OLD123" } as CustomerTicketRow;
const ticket = { reference: "SY-NEW456" } as CustomerTicket;

function setup(placement: "header" | "tools" = "header") {
  const u = userEvent.setup();
  renderWithI18n(
    <TooltipProvider>
      <ReportIssueButton target={target} placement={placement} />
    </TooltipProvider>,
  );
  return u;
}
async function openDialog(u: ReturnType<typeof userEvent.setup>) {
  await u.click(screen.getByTestId("report-issue-trigger"));
  return screen.findByRole("dialog");
}
async function fillAndSubmit(u: ReturnType<typeof userEvent.setup>, reason = "billing", details?: string) {
  await u.click(await screen.findByTestId(`report-reason-${reason}`));
  if (details) await u.type(screen.getByRole("textbox"), details);
  await u.click(screen.getByTestId("report-submit"));
}

beforeAll(stubResizeObserver);
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("ReportIssueButton", () => {
  it("renders a labelled trigger with its placement", () => {
    setup("tools");
    const trigger = screen.getByRole("button", { name: "Report an issue" });
    expect(trigger.dataset.testid).toBe("report-issue-trigger");
    expect(trigger.dataset.placement).toBe("tools");
    expect(api.openChatReport).not.toHaveBeenCalled();
  });

  it("checks for an open report on open, with a skeleton meanwhile, then shows the form", async () => {
    let resolve!: (v: { open: null }) => void;
    api.openChatReport.mockReturnValue(new Promise((r) => (resolve = r)));
    const u = setup();
    const dialog = await openDialog(u);
    expect(within(dialog).getByTestId("report-checking")).toBeTruthy();
    expect(within(dialog).getByRole("heading", { name: "Report an issue" })).toBeTruthy();
    expect(within(dialog).getByText("About your conversation with Theo")).toBeTruthy();
    resolve({ open: null });
    expect(await within(dialog).findByTestId("report-form")).toBeTruthy();
    expect(api.openChatReport.mock.calls[0][0]).toBe("conv-1");
    expect(within(dialog).getByTestId("report-privacy").textContent).toContain("Theo is not told about your report.");
    expect(await axeViolations()).toEqual([]);
  });

  it("shows the already-reported state with a link to the report, and no form", async () => {
    api.openChatReport.mockResolvedValue({ open: row });
    const u = setup();
    const dialog = await openDialog(u);
    const state = await within(dialog).findByTestId("report-exists");
    expect(state.textContent).toContain("You already reported this conversation");
    expect(within(state).getByTestId("report-reference").textContent).toBe("SY-OLD123");
    expect(within(state).getByRole("link", { name: "View report" }).getAttribute("href")).toBe("/en/support/requests/SY-OLD123");
    expect(within(dialog).queryByTestId("report-form")).toBeNull();
    expect(await axeViolations()).toEqual([]);
  });

  it("falls back to the form when the check fails", async () => {
    api.openChatReport.mockRejectedValue(apiError("network_error", 0));
    const u = setup();
    const dialog = await openDialog(u);
    expect(await within(dialog).findByTestId("report-form")).toBeTruthy();
  });

  it("validates: reason required, 'Something else' needs details, live counter", async () => {
    api.openChatReport.mockResolvedValue({ open: null });
    const u = setup();
    const dialog = await openDialog(u);
    await within(dialog).findByTestId("report-form");
    await u.click(screen.getByTestId("report-submit"));
    expect(await within(dialog).findByText("Choose what went wrong.")).toBeTruthy();
    expect(api.createChatReport).not.toHaveBeenCalled();

    const radios = within(dialog).getAllByRole("radio");
    expect(radios).toHaveLength(5);
    expect(within(dialog).getByRole("radiogroup", { name: "What went wrong?" })).toBeTruthy();
    expect(within(dialog).getByText("A technical problem (messages, images, connection)")).toBeTruthy();

    await u.click(screen.getByTestId("report-reason-other"));
    expect(within(dialog).getByText("Tell us what happened")).toBeTruthy();
    await u.click(screen.getByTestId("report-submit"));
    expect(await within(dialog).findByText("Tell us what happened (at least 10 characters).")).toBeTruthy();
    await u.type(screen.getByRole("textbox"), "hello");
    expect(within(dialog).getByText("5/2000")).toBeTruthy();
    expect(api.createChatReport).not.toHaveBeenCalled();
  });

  it("submits once with the loading state, sends the contract payload and shows the success state", async () => {
    api.openChatReport.mockResolvedValue({ open: null });
    let resolve!: (t: CustomerTicket) => void;
    api.createChatReport.mockReturnValue(new Promise((r) => (resolve = r)));
    const u = setup();
    const dialog = await openDialog(u);
    await fillAndSubmit(u, "other", "  The advisor was rude to me  ");
    const submit = within(dialog).getByTestId("report-submit") as HTMLButtonElement;
    await waitFor(() => expect(submit.disabled).toBe(true));
    expect(submit.textContent).toBe("Sending…");
    await u.click(submit);
    expect(api.createChatReport).toHaveBeenCalledTimes(1);
    expect(api.createChatReport).toHaveBeenCalledWith({ conversation_id: "conv-1", session_id: "sess-1", reason: "other", details: "The advisor was rude to me", seq: 42 });
    resolve(ticket);
    const success = await within(dialog).findByTestId("report-success");
    expect(success.textContent).toContain("Thanks — your report was sent.");
    expect(success.textContent).toContain("We'll reply by e-mail and in Support requests.");
    expect(within(success).getByTestId("report-reference").textContent).toBe("SY-NEW456");
    expect(within(success).getByRole("link", { name: "View report" }).getAttribute("href")).toBe("/en/support/requests/SY-NEW456");
    // Only the reason is tracked, never the free text.
    expect(analytics.track).toHaveBeenCalledWith("chat_report_submitted", { reason: "other" });
    expect(JSON.stringify(analytics.track.mock.calls)).not.toContain("rude");
    await waitFor(() => expect(document.activeElement).toBe(success));
    expect(await axeViolations()).toEqual([]);
  });

  it("omits empty details and unknown session/seq", async () => {
    api.openChatReport.mockResolvedValue({ open: null });
    api.createChatReport.mockResolvedValue(ticket);
    const u = userEvent.setup();
    renderWithI18n(
      <TooltipProvider>
        <ReportIssueButton target={{ conversationId: "c", advisorName: "Theo" }} placement="header" />
      </TooltipProvider>,
    );
    await openDialog(u);
    await fillAndSubmit(u, "technical");
    await screen.findByTestId("report-success");
    expect(api.createChatReport).toHaveBeenCalledWith({ conversation_id: "c", reason: "technical" });
  });

  it("closes with Back to chat and returns focus to the trigger", async () => {
    api.openChatReport.mockResolvedValue({ open: null });
    api.createChatReport.mockResolvedValue(ticket);
    const u = setup();
    await openDialog(u);
    await fillAndSubmit(u);
    await u.click(await screen.findByTestId("report-back"));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(document.activeElement).toBe(screen.getByTestId("report-issue-trigger"));
  });

  it("closes with Escape and returns focus to the trigger", async () => {
    api.openChatReport.mockResolvedValue({ open: null });
    const u = setup();
    await openDialog(u);
    await screen.findByTestId("report-form");
    await u.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(document.activeElement).toBe(screen.getByTestId("report-issue-trigger"));
  });

  it("switches to the already-reported state on 409 report_exists", async () => {
    api.openChatReport.mockResolvedValue({ open: null });
    api.createChatReport.mockRejectedValue(apiError("report_exists", 409, { details: { reference: "SY-DUP999" } }));
    const u = setup();
    const dialog = await openDialog(u);
    await fillAndSubmit(u);
    const state = await within(dialog).findByTestId("report-exists");
    expect(within(state).getByTestId("report-reference").textContent).toBe("SY-DUP999");
    expect(analytics.track).not.toHaveBeenCalled();
  });

  it("maps 422 field messages onto the fields", async () => {
    api.openChatReport.mockResolvedValue({ open: null });
    api.createChatReport.mockRejectedValue(apiError("validation_failed", 422, { details: { fields: { details: "Too long for us." } } }));
    const u = setup();
    const dialog = await openDialog(u);
    await fillAndSubmit(u);
    expect(await within(dialog).findByText("Too long for us.")).toBeTruthy();
    expect(within(dialog).queryByTestId("report-error")).toBeNull();
  });

  it("shows a generic message for a 422 on session_id", async () => {
    api.openChatReport.mockResolvedValue({ open: null });
    api.createChatReport.mockRejectedValue(apiError("validation_failed", 422, { details: { fields: { session_id: "bad" } } }));
    const u = setup();
    const dialog = await openDialog(u);
    await fillAndSubmit(u);
    expect((await within(dialog).findByTestId("report-root-error")).textContent).toContain("We could not attach your report to this conversation.");
  });

  it("explains a rate limit with the wait time and a way back", async () => {
    api.openChatReport.mockResolvedValue({ open: null });
    api.createChatReport.mockRejectedValue(apiError("rate_limited", 429, { retryAfter: 1200 }));
    const u = setup();
    const dialog = await openDialog(u);
    await fillAndSubmit(u);
    const notice = await within(dialog).findByTestId("report-error");
    expect(notice.dataset.kind).toBe("rate_limited");
    expect(notice.textContent).toContain("Try again in about 20 minutes.");
    expect(within(notice).getByRole("button", { name: "Back to chat" })).toBeTruthy();
  });

  it("retries after a network failure and keeps the entered values", async () => {
    api.openChatReport.mockResolvedValue({ open: null });
    api.createChatReport.mockRejectedValueOnce(apiError("network_error", 0)).mockResolvedValueOnce(ticket);
    const u = setup();
    const dialog = await openDialog(u);
    await fillAndSubmit(u, "billing", "I was charged twice");
    const notice = await within(dialog).findByTestId("report-error");
    expect(notice.textContent).toContain("We could not send your report. Check your connection and try again.");
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("I was charged twice");
    await u.click(within(notice).getByTestId("report-retry"));
    expect(await within(dialog).findByTestId("report-success")).toBeTruthy();
    expect(api.createChatReport).toHaveBeenCalledTimes(2);
  });

  it.each([
    ["unauthenticated", 401, "sign_in", "Please sign in to continue."],
    ["forbidden", 403, "forbidden", "Only clients can report a conversation."],
    ["not_found", 404, "not_found", "This conversation is no longer available."],
  ])("handles %s with its own message and action", async (code, status, kind, message) => {
    api.openChatReport.mockResolvedValue({ open: null });
    api.createChatReport.mockRejectedValue(apiError(code, status));
    const u = setup();
    const dialog = await openDialog(u);
    await fillAndSubmit(u);
    const notice = await within(dialog).findByTestId("report-error");
    expect(notice.dataset.kind).toBe(kind);
    expect(notice.textContent).toContain(message);
    if (kind === "sign_in") expect(within(notice).getByRole("link", { name: /sign in/i }).getAttribute("href")).toMatch(/^\/en\/login\?next=/);
    else expect(within(notice).getByRole("button", { name: "Back to chat" })).toBeTruthy();
  });
});
