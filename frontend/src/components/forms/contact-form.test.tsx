// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { ApiError } from "@/lib/api/errors";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";
import { ContactForm } from "@/components/forms/contact-form";

const session = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));
const createTicket = vi.hoisted(() => vi.fn());

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => session.current }));
vi.mock("@/components/layout/public-frame", () => ({
  PublicFrame: ({ children }: { children: React.ReactNode }) => <main>{children}</main>,
}));
vi.mock("@/lib/api/endpoints", () => ({ supportApi: { createTicket } }));

beforeAll(stubResizeObserver);
beforeEach(() => {
  session.current = { status: "anonymous", user: null };
  window.history.replaceState(null, "", "/en/contact");
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

const submit = () => screen.getByRole("button", { name: "Submit" });

async function fill(u: ReturnType<typeof userEvent.setup>, { email = "jane@example.com" } = {}) {
  await u.type(screen.getByRole("textbox", { name: "Subject" }), "Export my data");
  if (email) await u.type(screen.getByRole("textbox", { name: "Email" }), email);
  await u.type(screen.getByRole("textbox", { name: /Describe your issue/ }), "Please send me a copy of my data.");
  await u.click(screen.getByRole("checkbox"));
}

describe("ContactForm", () => {
  it("validates every field before sending", async () => {
    const u = userEvent.setup();
    renderWithI18n(<ContactForm />);
    await u.click(submit());
    expect(await screen.findByText("Enter your email address.")).toBeTruthy();
    expect(screen.getAllByText("This field is required.")).toHaveLength(2);
    expect(screen.getByText("Please confirm to continue.")).toBeTruthy();
    expect(screen.getByRole("textbox", { name: "Subject" }).getAttribute("aria-invalid")).toBe("true");
    expect(createTicket).not.toHaveBeenCalled();
    expect(await axeViolations()).toEqual([]);

    await u.type(screen.getByRole("textbox", { name: "Subject" }), "ab");
    expect(await screen.findByText("Use at least 3 characters.")).toBeTruthy();
  });

  it("sends the ticket and shows its reference", async () => {
    const u = userEvent.setup();
    createTicket.mockResolvedValue({ reference: "SY-7K3M9Q", topic: "general", created_at: "2026-11-05T10:00:00Z" });
    renderWithI18n(<ContactForm />);
    await fill(u);
    await u.click(submit());
    const success = await screen.findByTestId("contact-success");
    expect(createTicket).toHaveBeenCalledWith({
      topic: "general",
      subject: "Export my data",
      email: "jane@example.com",
      message: "Please send me a copy of my data.",
      consent: true,
    });
    expect(within(success).getByTestId("contact-reference").textContent).toBe("SY-7K3M9Q");
    expect(success.textContent).toContain("We will reply to jane@example.com.");
    expect(within(success).getByRole("link", { name: "Browse the FAQ" }).getAttribute("href")).toBe("/en/faq");
    expect(await axeViolations()).toEqual([]);

    await u.click(screen.getByRole("button", { name: "Send another request" }));
    expect((screen.getByRole("textbox", { name: "Subject" }) as HTMLInputElement).value).toBe("");
  });

  it("preselects the topic from ?topic= and prefills the account email", async () => {
    const u = userEvent.setup();
    window.history.replaceState(null, "", "/en/contact?topic=privacy");
    session.current = { status: "authenticated", user: { id: "u1", email: "member@example.com", role: "client" } };
    createTicket.mockResolvedValue({ reference: "SY-AB2CD3", topic: "privacy", created_at: "2026-11-05T10:00:00Z" });
    renderWithI18n(<ContactForm />);
    await waitFor(() => expect(screen.getByRole("combobox", { name: "Topic" }).textContent).toContain("Privacy & my data"));
    expect((screen.getByRole("textbox", { name: "Email" }) as HTMLInputElement).value).toBe("member@example.com");
    await fill(u, { email: "" });
    await u.click(submit());
    await waitFor(() => expect(createTicket).toHaveBeenCalledWith(expect.objectContaining({ topic: "privacy", email: "member@example.com" })));
  });

  it("ignores unknown topics", () => {
    window.history.replaceState(null, "", "/en/contact?topic=%3Cscript%3E");
    renderWithI18n(<ContactForm />);
    expect(screen.getByRole("combobox", { name: "Topic" }).textContent).toContain("General question");
  });

  it("puts server field errors on their fields and other failures above the form", async () => {
    const u = userEvent.setup();
    createTicket.mockRejectedValueOnce(
      new ApiError({
        code: "validation_failed",
        message: "bad",
        status: 422,
        details: { fields: { subject: "Write a subject between 3 and 120 characters." } },
      })
    );
    renderWithI18n(<ContactForm />);
    await fill(u);
    await u.click(submit());
    expect(await screen.findByText("Write a subject between 3 and 120 characters.")).toBeTruthy();
    expect(screen.getByRole("textbox", { name: "Subject" }).getAttribute("aria-invalid")).toBe("true");

    createTicket.mockRejectedValueOnce(new ApiError({ code: "rate_limited", message: "slow down", status: 429, retryAfter: 60 }));
    await u.type(screen.getByRole("textbox", { name: "Subject" }), " please");
    await u.click(submit());
    expect((await screen.findByRole("alert")).textContent).toContain("Too many requests. Please wait a moment.");
  });
});
