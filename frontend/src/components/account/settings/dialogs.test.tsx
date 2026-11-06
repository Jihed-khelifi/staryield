// @vitest-environment jsdom
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { ApiError } from "@/lib/api/errors";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const m = vi.hoisted(() => ({
  exportsApi: { request: vi.fn(), list: vi.fn(), get: vi.fn(), downloadToken: vi.fn(), download: vi.fn() },
  consentsApi: { get: vi.fn(), update: vi.fn() },
  emailChangeApi: { request: vi.fn(), confirm: vi.fn(), cancel: vi.fn() },
  accountApi: { deactivate: vi.fn(), close: vi.fn(), reactivate: vi.fn(), cancelClosure: vi.fn() },
  signOut: vi.fn(),
  reload: vi.fn(),
}));
vi.mock("@/lib/api/account", () => ({ ...m }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ signOut: m.signOut, reload: m.reload, status: "authenticated" }) }));
vi.mock("@/components/forms/auth/auth-card", () => ({
  AuthShell: ({ children }: { children: React.ReactNode }) => <main>{children}</main>,
  AuthLegal: () => null,
}));

import { ConsentDialog } from "@/components/account/settings/consent-dialog";
import { EmailChangeDialog } from "@/components/account/settings/email-change-dialog";
import { ExportDialog } from "@/components/account/settings/export-dialog";
import { AccountActionsCard } from "@/components/account/settings/account-actions";
import { EmailLinkPage } from "@/components/account/email-link-page";

beforeAll(stubResizeObserver);
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  window.history.replaceState(null, "", "/en");
});

const wrap = (ui: React.ReactElement) => renderWithI18n(<ToastProvider>{ui}</ToastProvider>);
const exp = (over = {}) => ({ id: "e1", status: "queued", requested_at: "2026-11-05T10:00:00Z", download_count: 0, ...over });

describe("ExportDialog", () => {
  it("requests an export with the password and shows its status", async () => {
    const u = userEvent.setup();
    m.exportsApi.list.mockResolvedValue({ items: [] });
    m.exportsApi.request.mockResolvedValue(exp());
    wrap(<ExportDialog open onOpenChange={() => {}} />);
    expect(await screen.findByText("You haven't requested an export yet.")).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
    await u.click(screen.getByTestId("export-request"));
    expect(await screen.findByText("Enter your password.")).toBeTruthy();
    await u.type(screen.getByLabelText("Confirm your password"), "Secret!1");
    await u.click(screen.getByTestId("export-request"));
    await waitFor(() => expect(m.exportsApi.request).toHaveBeenCalledWith("Secret!1"));
    expect((await screen.findAllByText("Queued")).length).toBeGreaterThan(0);
  });

  it("downloads a ready export through a single-use token", async () => {
    const u = userEvent.setup();
    URL.createObjectURL = vi.fn(() => "blob:x");
    URL.revokeObjectURL = vi.fn();
    m.exportsApi.list.mockResolvedValue({ items: [exp({ status: "ready", bytes: 2048, expires_at: "2026-11-12T10:00:00Z" })] });
    m.exportsApi.downloadToken.mockResolvedValue({ token: "tok", expires_at: "x", url: "/x" });
    m.exportsApi.download.mockResolvedValue(new Blob(["zip"]));
    wrap(<ExportDialog open onOpenChange={() => {}} />);
    await u.type(await screen.findByLabelText("Confirm your password"), "Secret!1");
    await u.click(screen.getByTestId("export-download"));
    await waitFor(() => expect(m.exportsApi.download).toHaveBeenCalledWith("e1", "tok"));
    expect(m.exportsApi.downloadToken).toHaveBeenCalledWith("e1", "Secret!1");
  });

  it("maps a wrong password onto the field", async () => {
    const u = userEvent.setup();
    m.exportsApi.list.mockResolvedValue({ items: [] });
    m.exportsApi.request.mockRejectedValue(new ApiError({ code: "validation_failed", message: "x", status: 422, details: { fields: { password: "The password is incorrect." } } }));
    wrap(<ExportDialog open onOpenChange={() => {}} />);
    await u.type(await screen.findByLabelText("Confirm your password"), "bad");
    await u.click(screen.getByTestId("export-request"));
    expect(await screen.findByText("The password is incorrect.")).toBeTruthy();
  });
});

describe("ConsentDialog", () => {
  it("loads, toggles and saves the choices", async () => {
    const u = userEvent.setup();
    const items = (a: boolean) => [
      { category: "essential", granted: true, required: true, explicit: true },
      { category: "analytics", granted: a, required: false, explicit: false },
      { category: "marketing_attribution", granted: false, required: false, explicit: false },
      { category: "personalization", granted: true, required: false, explicit: false },
    ];
    m.consentsApi.get.mockResolvedValue({ policy_version: "2026-11", items: items(false), outdated: false });
    m.consentsApi.update.mockResolvedValue({ policy_version: "2026-11", items: items(true), outdated: false });
    wrap(<ConsentDialog open onOpenChange={() => {}} />);
    await u.click(await screen.findByTestId("consent-analytics"));
    await u.click(screen.getByTestId("consent-save"));
    await waitFor(() => expect(m.consentsApi.update).toHaveBeenCalledWith({ analytics: true, marketing_attribution: false, personalization: true }));
  });
});

describe("EmailChangeDialog", () => {
  it("rejects the current address, then confirms the request", async () => {
    const u = userEvent.setup();
    m.emailChangeApi.request.mockResolvedValue({ status: "pending_confirmation" });
    const onRequested = vi.fn();
    wrap(<EmailChangeDialog open onOpenChange={() => {}} currentEmail="jane@example.com" onRequested={onRequested} />);
    await u.type(screen.getByLabelText("New email address"), "jane@example.com");
    await u.type(screen.getByLabelText("Current password"), "Secret!1");
    await u.click(screen.getByRole("button", { name: "Send confirmation link" }));
    expect(await screen.findByText("This is already your email address.")).toBeTruthy();
    expect(m.emailChangeApi.request).not.toHaveBeenCalled();
    await u.clear(screen.getByLabelText("New email address"));
    await u.type(screen.getByLabelText("New email address"), "new@example.com");
    await u.click(screen.getByRole("button", { name: "Send confirmation link" }));
    await waitFor(() => expect(m.emailChangeApi.request).toHaveBeenCalledWith("new@example.com", "Secret!1"));
    expect(await screen.findByText(/We sent a confirmation link to new@example.com/)).toBeTruthy();
    expect(onRequested).toHaveBeenCalled();
  });
});

describe("AccountActionsCard", () => {
  it("deactivates with the password and signs out to the login page", async () => {
    const u = userEvent.setup();
    m.accountApi.deactivate.mockResolvedValue(undefined);
    wrap(<AccountActionsCard />);
    await u.click(screen.getByTestId("open-deactivate"));
    await u.type(screen.getByLabelText("Confirm your password"), "Secret!1");
    await u.click(screen.getByTestId("deactivate-confirm"));
    await waitFor(() => expect(m.accountApi.deactivate).toHaveBeenCalledWith("Secret!1"));
    expect(m.signOut).toHaveBeenCalledWith({ to: "/en/login?notice=deactivated" });
  });

  it("needs the word DELETE to close, and shows advisor blockers with actions", async () => {
    const u = userEvent.setup();
    m.accountApi.close.mockRejectedValue(new ApiError({ code: "closure_blocked", message: "x", status: 409, details: { reasons: ["earnings_balance", "open_requests"] } }));
    wrap(<AccountActionsCard />);
    await u.click(screen.getByTestId("open-close"));
    await u.type(screen.getByLabelText("Confirm your password"), "Secret!1");
    await u.type(screen.getByTestId("close-confirmation"), "delete");
    await u.click(screen.getByTestId("close-confirm"));
    expect(await screen.findAllByText("Type DELETE to confirm.")).toBeTruthy();
    expect(m.accountApi.close).not.toHaveBeenCalled();
    await u.clear(screen.getByTestId("close-confirmation"));
    await u.type(screen.getByTestId("close-confirmation"), "DELETE");
    await u.click(screen.getByTestId("close-confirm"));
    const blocked = await screen.findByTestId("closure-blocked");
    expect(within(blocked).getByRole("link", { name: "Request a payout" })).toBeTruthy();
    expect(within(blocked).getByRole("link", { name: "View requests" })).toBeTruthy();
    expect(m.signOut).not.toHaveBeenCalled();
  });
});

describe("EmailLinkPage", () => {
  it("strips the token from the URL, then confirms on click", async () => {
    const u = userEvent.setup();
    window.history.replaceState(null, "", "/en/confirm-email?token=abc123");
    m.emailChangeApi.confirm.mockResolvedValue({ status: "confirmed" });
    wrap(<EmailLinkPage action="confirm-email" />);
    await waitFor(() => expect(window.location.search).toBe(""));
    expect(m.emailChangeApi.confirm).not.toHaveBeenCalled();
    expect(await axeViolations()).toEqual([]);
    await u.click(await screen.findByTestId("email-link-action"));
    await waitFor(() => expect(m.emailChangeApi.confirm).toHaveBeenCalledWith("abc123"));
    expect(await screen.findByText("Your email address is updated")).toBeTruthy();
  });

  it("explains an expired link and a missing token", async () => {
    const u = userEvent.setup();
    window.history.replaceState(null, "", "/en/keep-account?token=old");
    m.accountApi.cancelClosure.mockRejectedValue(new ApiError({ code: "invalid_token", message: "x", status: 400 }));
    wrap(<EmailLinkPage action="keep-account" />);
    await u.click(await screen.findByTestId("email-link-action"));
    expect(await screen.findByText("This link has expired")).toBeTruthy();
    cleanup();
    window.history.replaceState(null, "", "/en/cancel-email-change");
    wrap(<EmailLinkPage action="cancel-email-change" />);
    expect(await screen.findByText("This link is incomplete")).toBeTruthy();
  });
});
