// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { ApiError } from "@/lib/api/errors";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const m = vi.hoisted(() => ({
  forgotPassword: vi.fn(),
  exportsApi: { request: vi.fn(), list: vi.fn(), get: vi.fn(), downloadToken: vi.fn(), download: vi.fn() },
  emailChangeApi: { request: vi.fn() },
  accountApi: { deactivate: vi.fn(), close: vi.fn() },
  signOut: vi.fn(),
}));
vi.mock("@/lib/api/account", () => ({ exportsApi: m.exportsApi, emailChangeApi: m.emailChangeApi, accountApi: m.accountApi }));
vi.mock("@/lib/api/endpoints", () => ({ authApi: { forgotPassword: m.forgotPassword } }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ user: { id: "u1", email: "jane@example.com", role: "client" }, signOut: m.signOut, status: "authenticated" }),
}));

import { PasswordNotSetNotice, CREATE_PASSWORD_RESEND_SECONDS } from "@/components/account/settings/password-not-set";
import { EmailChangeDialog } from "@/components/account/settings/email-change-dialog";
import { ExportDialog } from "@/components/account/settings/export-dialog";
import { AccountActionsCard } from "@/components/account/settings/account-actions";

const notSet = () => new ApiError({ code: "password_not_set", message: "no password", status: 409 });
const wrap = (ui: React.ReactElement) => renderWithI18n(<ToastProvider>{ui}</ToastProvider>);

beforeAll(stubResizeObserver);
beforeEach(() => {
  m.forgotPassword.mockResolvedValue({ status: "ok" });
  m.exportsApi.list.mockResolvedValue({ items: [] });
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.clearAllMocks();
});

describe("PasswordNotSetNotice", () => {
  it("explains the situation and sends the create-password link with the account e-mail", async () => {
    const u = userEvent.setup();
    wrap(<PasswordNotSetNotice />);
    const notice = screen.getByTestId("password-not-set");
    expect(notice.textContent).toContain("This needs your account password, and you haven't created one yet.");
    expect(await axeViolations()).toEqual([]);
    await u.click(within(notice).getByRole("button", { name: "Create a password" }));
    await waitFor(() => expect(m.forgotPassword).toHaveBeenCalledWith("jane@example.com", "en"));
    expect((await within(notice).findByRole("status")).textContent).toContain("We've e-mailed you a link to create your password.");
  });

  it("throttles resending until the wait is over", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const u = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    wrap(<PasswordNotSetNotice />);
    await u.click(screen.getByTestId("create-password"));
    await screen.findByTestId("create-password-sent");
    const button = screen.getByTestId("create-password");
    expect(button.hasAttribute("disabled")).toBe(true);
    expect(button.textContent).toContain(`Send again in ${CREATE_PASSWORD_RESEND_SECONDS}s`);
    await act(async () => {
      await vi.advanceTimersByTimeAsync((CREATE_PASSWORD_RESEND_SECONDS + 1) * 1000);
    });
    expect(screen.getByTestId("create-password").hasAttribute("disabled")).toBe(false);
    expect(screen.getByTestId("create-password").textContent).toContain("Send the link again");
    await u.click(screen.getByTestId("create-password"));
    await waitFor(() => expect(m.forgotPassword).toHaveBeenCalledTimes(2));
  });
});

describe("password_not_set in the dialogs that ask for the password", () => {
  it("e-mail change", async () => {
    const u = userEvent.setup();
    m.emailChangeApi.request.mockRejectedValue(notSet());
    wrap(<EmailChangeDialog open onOpenChange={() => {}} currentEmail="jane@example.com" onRequested={vi.fn()} />);
    await u.type(screen.getByLabelText("New email address"), "new@example.com");
    await u.type(screen.getByLabelText("Current password"), "x");
    await u.click(screen.getByRole("button", { name: "Send confirmation link" }));
    const notice = await screen.findByTestId("password-not-set");
    await u.click(within(notice).getByTestId("create-password"));
    await waitFor(() => expect(m.forgotPassword).toHaveBeenCalledWith("jane@example.com", "en"));
  });

  it("deactivate", async () => {
    const u = userEvent.setup();
    m.accountApi.deactivate.mockRejectedValue(notSet());
    wrap(<AccountActionsCard />);
    await u.click(screen.getByTestId("open-deactivate"));
    await u.type(screen.getByLabelText("Confirm your password"), "x");
    await u.click(screen.getByTestId("deactivate-confirm"));
    expect(await screen.findByTestId("password-not-set")).toBeTruthy();
    expect(m.signOut).not.toHaveBeenCalled();
  });

  it("close", async () => {
    const u = userEvent.setup();
    m.accountApi.close.mockRejectedValue(notSet());
    wrap(<AccountActionsCard />);
    await u.click(screen.getByTestId("open-close"));
    const dialog = await screen.findByTestId("close-dialog");
    await u.type(within(dialog).getByLabelText("Confirm your password"), "x");
    await u.type(within(dialog).getByTestId("close-confirmation"), "DELETE");
    await u.click(within(dialog).getByTestId("close-confirm"));
    expect(await within(dialog).findByTestId("password-not-set")).toBeTruthy();
    expect(m.signOut).not.toHaveBeenCalled();
  });

  it("export request", async () => {
    const u = userEvent.setup();
    m.exportsApi.request.mockRejectedValue(notSet());
    wrap(<ExportDialog open onOpenChange={() => {}} />);
    await u.type(await screen.findByLabelText("Confirm your password"), "x");
    await u.click(screen.getByTestId("export-request"));
    const notice = await screen.findByTestId("password-not-set");
    expect(within(notice).getByTestId("create-password")).toBeTruthy();
    expect(screen.queryByText("This needs your account password, and you haven't created one yet.", { selector: "[data-slot=form-root-error] *" })).toBeNull();
  });

  it("export download", async () => {
    const u = userEvent.setup();
    m.exportsApi.list.mockResolvedValue({
      items: [{ id: "e1", status: "ready", requested_at: "2026-11-05T10:00:00Z", expires_at: "2026-11-12T10:00:00Z", download_count: 0, bytes: 1024 }],
    });
    m.exportsApi.downloadToken.mockRejectedValue(notSet());
    wrap(<ExportDialog open onOpenChange={() => {}} />);
    await u.type(await screen.findByLabelText("Confirm your password"), "x");
    await u.click(await screen.findByTestId("export-download"));
    expect(await screen.findByTestId("password-not-set")).toBeTruthy();
  });
});
