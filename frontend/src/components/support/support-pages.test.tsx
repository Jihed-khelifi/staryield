// @vitest-environment jsdom
import type * as React from "react";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { ApiError } from "@/lib/api/errors";
import type { CustomerTicket, CustomerTicketRow } from "@/lib/api/types";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const api = vi.hoisted(() => ({ myTickets: vi.fn(), myTicket: vi.fn(), replyMine: vi.fn(), linkTicket: vi.fn(), replyLink: vi.fn() }));
const session = vi.hoisted(() => ({ status: "authenticated" as string }));
const router = vi.hoisted(() => ({ replace: vi.fn(), push: vi.fn() }));
const toast = vi.hoisted(() => ({ show: vi.fn(), showError: vi.fn() }));

vi.mock("next/navigation", () => ({ useRouter: () => router, usePathname: () => "/en/support/requests" }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/api/endpoints", () => ({ supportApi: api }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: session.status, user: null }) }));
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => toast }));
vi.mock("@/components/layout/public-frame", () => ({ PublicFrame: ({ children }: { children: React.ReactNode }) => <main>{children}</main> }));

import { LinkTicketPage, MyTicketPage, SupportRequestsPage } from "@/components/support/support-pages";

const TOKEN = "Zm9vYmFyYmF6cXV4MTIzNDU2Nzg5MC1fQUJDREVGR0g";

function row(n: number, over: Partial<CustomerTicketRow> = {}): CustomerTicketRow {
  return { reference: `SY-AAAA${String(n).padStart(2, "0")}`, subject: `Subject ${n}`, topic: "billing", status: "open", awaiting: "support", created_at: "2026-11-01T10:00:00Z", last_message_at: "2026-11-02T10:00:00Z", ...over };
}
function ticket(over: Partial<CustomerTicket> = {}): CustomerTicket {
  return {
    ...row(1, { reference: "SY-7K2M9Q", subject: "Export my data", status: "pending", awaiting: "you" }),
    can_reply: true,
    messages: [
      { id: "m1", author: { type: "customer", name: "Jane" }, body: "Please send my data.\nThanks\nhttps://example.com/" + "a".repeat(120), created_at: "2026-11-01T10:00:00Z" },
      { id: "m2", author: { type: "staff", name: "Staryield Support · Anna" }, body: "Sure, <b>done</b>.", created_at: "2026-11-02T09:00:00Z" },
    ],
    ...over,
  };
}
const apiError = (code: string, status: number, extra: { retryAfter?: number } = {}) => new ApiError({ code, message: code, status, ...extra });

beforeAll(stubResizeObserver);
beforeEach(() => {
  session.status = "authenticated";
  sessionStorage.clear();
  window.history.replaceState(null, "", "/en/support/requests");
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("SupportRequestsPage", () => {
  it("lists requests with status chips and links, paginated with Next/Previous", async () => {
    api.myTickets
      .mockResolvedValueOnce({ items: [row(1), row(2, { status: "pending", awaiting: "you" })], next_cursor: "c2" })
      .mockResolvedValueOnce({ items: [row(3, { status: "closed", awaiting: "none" })] });
    const u = userEvent.setup();
    renderWithI18n(<SupportRequestsPage />);
    const rows = await screen.findAllByTestId("request-row");
    expect(rows).toHaveLength(2);
    expect(within(rows[0]).getByTestId("ticket-status").textContent).toBe("Awaiting support");
    expect(within(rows[1]).getByTestId("ticket-status").textContent).toBe("Awaiting your reply");
    expect(within(rows[0]).getByRole("link").getAttribute("href")).toBe("/en/support/requests/SY-AAAA01");
    expect(api.myTickets.mock.calls[0].slice(0, 2)).toEqual([undefined, 20]);
    expect(await axeViolations()).toEqual([]);

    await u.click(screen.getByRole("button", { name: /Next/ }));
    expect((await screen.findAllByTestId("request-row"))).toHaveLength(1);
    expect(api.myTickets.mock.calls[1].slice(0, 2)).toEqual(["c2", 20]);
    expect(screen.getByTestId("ticket-status").textContent).toBe("Closed");
    await u.click(screen.getByRole("button", { name: /Previous/ }));
    expect(await screen.findAllByTestId("request-row")).toHaveLength(2);
    expect(api.myTickets).toHaveBeenCalledTimes(2);
  });

  it("shows an empty state with a way to contact support", async () => {
    api.myTickets.mockResolvedValue({ items: [] });
    renderWithI18n(<SupportRequestsPage />);
    expect(await screen.findByText("You have no support requests yet")).toBeTruthy();
    expect(screen.getByRole("link", { name: "Contact support" }).getAttribute("href")).toBe("/en/contact");
  });

  it("explains a failed load and retries", async () => {
    api.myTickets.mockRejectedValueOnce(apiError("internal_error", 500)).mockResolvedValueOnce({ items: [row(1)] });
    const u = userEvent.setup();
    renderWithI18n(<SupportRequestsPage />);
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("We could not load your requests.");
    await u.click(within(alert).getByRole("button", { name: "Try again" }));
    expect(await screen.findAllByTestId("request-row")).toHaveLength(1);
  });

  it("redirects anonymous visitors to the login page with next, and loads nothing", async () => {
    session.status = "anonymous";
    renderWithI18n(<SupportRequestsPage />);
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/login?next=%2Fen%2Fsupport%2Frequests"));
    expect(api.myTickets).not.toHaveBeenCalled();
  });

  it("shows a skeleton while the session loads", () => {
    session.status = "loading";
    renderWithI18n(<SupportRequestsPage />);
    expect(screen.getByTestId("support-skeleton")).toBeTruthy();
    expect(router.replace).not.toHaveBeenCalled();
  });
});

describe("MyTicketPage (signed in thread)", () => {
  it("renders the thread: chip, names, dates, plain-text bodies with safe wrapping", async () => {
    api.myTicket.mockResolvedValue(ticket());
    renderWithI18n(<MyTicketPage reference="SY-7K2M9Q" />);
    expect(await screen.findByTestId("ticket-thread")).toBeTruthy();
    expect(screen.getByTestId("ticket-reference").textContent).toBe("SY-7K2M9Q");
    expect(screen.getByTestId("ticket-status").textContent).toBe("Awaiting your reply");
    const [mine, staff] = screen.getAllByTestId("ticket-message");
    expect(mine.dataset.author).toBe("customer");
    expect(within(mine).getByText("Jane")).toBeTruthy();
    expect(within(mine).getByText("You")).toBeTruthy();
    expect(staff.dataset.author).toBe("staff");
    expect(within(staff).getByText("Staryield Support · Anna")).toBeTruthy();
    expect(within(staff).getByText("Support team")).toBeTruthy();
    const body = within(mine).getByText(/Please send my data/);
    expect(body.className).toContain("whitespace-pre-wrap");
    expect(body.className).toContain("[overflow-wrap:anywhere]");
    // HTML in a body is text, never markup.
    expect(within(staff).getByText("Sure, <b>done</b>.")).toBeTruthy();
    expect(staff.querySelector("b")).toBeNull();
    expect(within(mine).getByText("Jane").closest("header")!.querySelector("time")!.getAttribute("datetime")).toBe("2026-11-01T10:00:00Z");
    expect(api.myTicket.mock.calls[0][0]).toBe("SY-7K2M9Q");
    expect(await axeViolations()).toEqual([]);
  });

  it("validates the reply (1–5000), counts characters and sends it, refreshing the thread", async () => {
    const u = userEvent.setup();
    const after = ticket({ status: "open", awaiting: "support", messages: [...ticket().messages, { id: "m3", author: { type: "customer", name: "Jane" }, body: "More details", created_at: "2026-11-03T09:00:00Z" }] });
    api.myTicket.mockResolvedValue(ticket());
    api.replyMine.mockResolvedValue(after);
    renderWithI18n(<MyTicketPage reference="SY-7K2M9Q" />);
    const box = await screen.findByRole("textbox", { name: "Your reply" });
    await u.click(screen.getByRole("button", { name: "Send reply" }));
    expect(await screen.findByText("This field is required.")).toBeTruthy();
    expect(box.getAttribute("aria-invalid")).toBe("true");
    expect(api.replyMine).not.toHaveBeenCalled();

    await u.type(box, "  More details  ");
    expect(document.querySelector('[data-slot="field-counter"]')!.textContent).toBe("16/5000");
    await u.click(screen.getByRole("button", { name: "Send reply" }));
    await waitFor(() => expect(api.replyMine).toHaveBeenCalledWith("SY-7K2M9Q", "More details"));
    expect(await screen.findAllByTestId("ticket-message")).toHaveLength(3);
    expect(screen.getByTestId("ticket-status").textContent).toBe("Awaiting support");
    expect(toast.show).toHaveBeenCalledWith(expect.objectContaining({ tone: "success" }));
    expect((screen.getByRole("textbox", { name: "Your reply" }) as HTMLTextAreaElement).value).toBe("");
  });

  it("refuses replies longer than 5000 characters", async () => {
    const u = userEvent.setup();
    api.myTicket.mockResolvedValue(ticket());
    renderWithI18n(<MyTicketPage reference="SY-7K2M9Q" />);
    const box = await screen.findByRole("textbox", { name: "Your reply" });
    await u.click(box);
    await u.paste("x".repeat(5001));
    await u.click(screen.getByRole("button", { name: "Send reply" }));
    expect(await screen.findByText("Use at most 5000 characters.")).toBeTruthy();
    expect(api.replyMine).not.toHaveBeenCalled();
  });

  it("shows the sending state and disables the button meanwhile", async () => {
    const u = userEvent.setup();
    api.myTicket.mockResolvedValue(ticket());
    let resolve!: (t: CustomerTicket) => void;
    api.replyMine.mockReturnValue(new Promise((r) => (resolve = r)));
    renderWithI18n(<MyTicketPage reference="SY-7K2M9Q" />);
    await u.type(await screen.findByRole("textbox", { name: "Your reply" }), "Hello");
    await u.click(screen.getByRole("button", { name: "Send reply" }));
    const busy = await screen.findByRole("button", { name: "Sending…" });
    expect((busy as HTMLButtonElement).disabled).toBe(true);
    resolve(ticket());
    await waitFor(() => expect(screen.getByRole("button", { name: "Send reply" })).toBeTruthy());
  });

  it("maps rate_limited to a retry hint", async () => {
    const u = userEvent.setup();
    api.myTicket.mockResolvedValue(ticket());
    api.replyMine.mockRejectedValue(apiError("rate_limited", 429, { retryAfter: 45 }));
    renderWithI18n(<MyTicketPage reference="SY-7K2M9Q" />);
    await u.type(await screen.findByRole("textbox", { name: "Your reply" }), "Hello");
    await u.click(screen.getByRole("button", { name: "Send reply" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Try again in 45 seconds.");
    // The text stays so nothing is lost.
    expect((screen.getByRole("textbox", { name: "Your reply" }) as HTMLTextAreaElement).value).toBe("Hello");
  });

  it("maps invalid_state to the closed notice with a CTA for a new request", async () => {
    const u = userEvent.setup();
    api.myTicket.mockResolvedValue(ticket());
    api.replyMine.mockRejectedValue(apiError("invalid_state", 409));
    renderWithI18n(<MyTicketPage reference="SY-7K2M9Q" />);
    await u.type(await screen.findByRole("textbox", { name: "Your reply" }), "Hello");
    await u.click(screen.getByRole("button", { name: "Send reply" }));
    const closed = await screen.findByTestId("ticket-closed");
    expect(closed.textContent).toContain("This request is closed");
    expect(within(closed).getByRole("link", { name: "Send a new request" }).getAttribute("href")).toBe("/en/contact");
    expect(screen.queryByRole("textbox", { name: "Your reply" })).toBeNull();
    expect(screen.getByTestId("ticket-status").textContent).toBe("Closed");
  });

  it("shows the closed notice instead of the composer for a closed request", async () => {
    api.myTicket.mockResolvedValue(ticket({ status: "closed", awaiting: "none", can_reply: false }));
    renderWithI18n(<MyTicketPage reference="SY-7K2M9Q" />);
    expect(await screen.findByTestId("ticket-closed")).toBeTruthy();
    expect(screen.queryByRole("textbox")).toBeNull();
  });

  it("offers to reopen a resolved request", async () => {
    api.myTicket.mockResolvedValue(ticket({ status: "resolved", awaiting: "none" }));
    renderWithI18n(<MyTicketPage reference="SY-7K2M9Q" />);
    expect(await screen.findByRole("textbox", { name: "Reply to reopen this request" })).toBeTruthy();
  });

  it("explains another user's request (404) with a way back", async () => {
    api.myTicket.mockRejectedValue(apiError("not_found", 404));
    renderWithI18n(<MyTicketPage reference="SY-7K2M9Q" />);
    expect(await screen.findByTestId("support-not-found")).toBeTruthy();
    expect(screen.getByRole("link", { name: "Back to your requests" }).getAttribute("href")).toBe("/en/support/requests");
  });

  it("retries a failed load", async () => {
    const u = userEvent.setup();
    api.myTicket.mockRejectedValueOnce(apiError("network_error", 0)).mockResolvedValueOnce(ticket());
    renderWithI18n(<MyTicketPage reference="SY-7K2M9Q" />);
    await u.click(await screen.findByRole("button", { name: "Try again" }));
    expect(await screen.findByTestId("ticket-thread")).toBeTruthy();
  });

  it("redirects anonymous visitors to log in and come back", async () => {
    session.status = "anonymous";
    renderWithI18n(<MyTicketPage reference="sy-7k2m9q" />);
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/login?next=%2Fen%2Fsupport%2Frequests%2FSY-7K2M9Q"));
    expect(api.myTicket).not.toHaveBeenCalled();
  });

  it("shows a skeleton while loading", () => {
    api.myTicket.mockReturnValue(new Promise(() => {}));
    renderWithI18n(<MyTicketPage reference="SY-7K2M9Q" />);
    expect(screen.getByTestId("support-skeleton")).toBeTruthy();
  });
});

describe("LinkTicketPage (anonymous private link)", () => {
  it("reads the token from #t=, strips it from the address bar and sends it only as a header argument", async () => {
    window.history.replaceState(null, "", `/en/support/SY-7K2M9Q#t=${TOKEN}`);
    api.linkTicket.mockResolvedValue(ticket());
    renderWithI18n(<LinkTicketPage reference="SY-7K2M9Q" />);
    expect(await screen.findByTestId("ticket-thread")).toBeTruthy();
    expect(api.linkTicket.mock.calls[0].slice(0, 2)).toEqual(["SY-7K2M9Q", TOKEN]);
    expect(window.location.hash).toBe("");
    expect(window.location.href).not.toContain(TOKEN);
    expect(sessionStorage.getItem("staryield.support.SY-7K2M9Q")).toBe(TOKEN);
    expect(api.myTicket).not.toHaveBeenCalled();
  });

  it("replies with the token and updates the thread", async () => {
    const u = userEvent.setup();
    window.history.replaceState(null, "", `/en/support/SY-7K2M9Q#t=${TOKEN}`);
    api.linkTicket.mockResolvedValue(ticket());
    api.replyLink.mockResolvedValue(ticket({ status: "open", awaiting: "support" }));
    renderWithI18n(<LinkTicketPage reference="SY-7K2M9Q" />);
    await u.type(await screen.findByRole("textbox", { name: "Your reply" }), "Thanks");
    await u.click(screen.getByRole("button", { name: "Send reply" }));
    await waitFor(() => expect(api.replyLink).toHaveBeenCalledWith("SY-7K2M9Q", TOKEN, "Thanks"));
    expect(api.replyMine).not.toHaveBeenCalled();
  });

  it("shows the expired-or-invalid state with a CTA to the contact form when the token is refused", async () => {
    window.history.replaceState(null, "", `/en/support/SY-7K2M9Q#t=${TOKEN}`);
    api.linkTicket.mockRejectedValue(apiError("not_found", 404));
    renderWithI18n(<LinkTicketPage reference="SY-7K2M9Q" />);
    const box = await screen.findByTestId("support-link-invalid");
    expect(box.textContent).toContain("This link has expired or is not valid");
    expect(within(box).getByRole("link", { name: "Send a new request" }).getAttribute("href")).toBe("/en/contact");
    expect(sessionStorage.getItem("staryield.support.SY-7K2M9Q")).toBeNull();
    expect(await axeViolations()).toEqual([]);
  });

  it("treats a missing or malformed fragment as an invalid link without calling the API", async () => {
    window.history.replaceState(null, "", "/en/support/SY-7K2M9Q#t=nope");
    renderWithI18n(<LinkTicketPage reference="SY-7K2M9Q" />);
    expect(await screen.findByTestId("support-link-invalid")).toBeTruthy();
    expect(api.linkTicket).not.toHaveBeenCalled();
    expect(window.location.hash).toBe("");
  });

  it("keeps working after a reload of the tab, from sessionStorage", async () => {
    sessionStorage.setItem("staryield.support.SY-7K2M9Q", TOKEN);
    window.history.replaceState(null, "", "/en/support/SY-7K2M9Q");
    api.linkTicket.mockResolvedValue(ticket());
    renderWithI18n(<LinkTicketPage reference="SY-7K2M9Q" />);
    expect(await screen.findByTestId("ticket-thread")).toBeTruthy();
    expect(api.linkTicket.mock.calls[0][1]).toBe(TOKEN);
  });

  it("maps rate_limited on load to a countdown retry", async () => {
    window.history.replaceState(null, "", `/en/support/SY-7K2M9Q#t=${TOKEN}`);
    api.linkTicket.mockRejectedValue(apiError("rate_limited", 429, { retryAfter: 30 }));
    renderWithI18n(<LinkTicketPage reference="SY-7K2M9Q" />);
    const alert = await screen.findByRole("alert");
    expect(within(alert).getByRole("button", { name: /Try again in \d+s/ })).toBeTruthy();
  });
});
