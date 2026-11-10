// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { ApiError } from "@/lib/api/errors";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { browser } from "@/lib/auth/google";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const m = vi.hoisted(() => ({
  methods: { get: vi.fn(), disconnectGoogle: vi.fn() },
  auth: { forgotPassword: vi.fn(), changePassword: vi.fn(), googleStart: vi.fn() },
  devicesApi: { list: vi.fn(), revoke: vi.fn(), revokeOthers: vi.fn() },
}));
vi.mock("@/lib/api/account", () => ({ devicesApi: m.devicesApi }));
vi.mock("@/lib/api/endpoints", () => ({ signInMethodsApi: m.methods, authApi: m.auth }));
vi.mock("@/lib/attribution", () => ({ attributionForSignup: () => undefined }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ user: { id: "u1", email: "jane@example.com", role: "client" } }) }));

import { SecurityCard } from "@/components/account/settings/security-card";

const both = { password: true, google: { email: "jane@gmail.com", linked_at: "2026-11-01T10:00:00Z" }, google_available: true };
const GOOGLE_URL = "https://accounts.google.com/o/oauth2/v2/auth?state=link";
const apiError = (code: string, status = 409, extra: Partial<ConstructorParameters<typeof ApiError>[0]> = {}) =>
  new ApiError({ code, message: code, status, ...extra });

const view = () =>
  renderWithI18n(
    <ToastProvider>
      <SecurityCard email="jane@example.com" />
    </ToastProvider>,
  );

beforeAll(stubResizeObserver);
beforeEach(() => {
  m.methods.get.mockResolvedValue(both);
  m.devicesApi.list.mockResolvedValue({ items: [] });
  m.auth.forgotPassword.mockResolvedValue({ status: "ok" });
  vi.spyOn(browser, "assign").mockImplementation(() => {});
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.restoreAllMocks();
});

describe("Sign-in methods block", () => {
  it("shows a loading state, then the password and the connected Google account", async () => {
    let resolve!: (v: unknown) => void;
    m.methods.get.mockReturnValue(new Promise((r) => (resolve = r)));
    view();
    const block = screen.getByTestId("sign-in-methods");
    expect(within(block).getByRole("status").textContent).toBe("Loading…");
    expect(block.querySelector("[aria-busy=true]")).toBeTruthy();
    resolve(both);
    expect(await within(block).findByTestId("google-email")).toBeTruthy();
    expect(screen.getByTestId("google-email").textContent).toBe("Connected as jane@gmail.com");
    expect(screen.getByTestId("method-password").getAttribute("data-state")).toBe("set");
    expect(screen.getByRole("button", { name: "Disconnect Google" })).toBeTruthy();
    // The usual change-password form stays.
    expect(screen.getByLabelText("Current Password")).toBeTruthy();
    expect(screen.queryByTestId("create-password")).toBeNull();
    expect(await axeViolations()).toEqual([]);
  });

  it("offers Connect Google when it is available, starting the link flow back to the settings", async () => {
    const u = userEvent.setup();
    m.methods.get.mockResolvedValue({ password: true, google_available: true });
    m.auth.googleStart.mockResolvedValue({ authorization_url: GOOGLE_URL });
    view();
    await u.click(await screen.findByTestId("connect-google"));
    await waitFor(() => expect(browser.assign).toHaveBeenCalledWith(GOOGLE_URL));
    expect(m.auth.googleStart).toHaveBeenCalledWith({ intent: "link", next: "/profile/settings", locale: "en", attribution: undefined });
  });

  it("shows why connecting failed", async () => {
    const u = userEvent.setup();
    m.methods.get.mockResolvedValue({ password: true, google_available: true });
    m.auth.googleStart.mockRejectedValue(apiError("google_unavailable", 503));
    view();
    await u.click(await screen.findByTestId("connect-google"));
    expect((await screen.findByTestId("connect-google-error")).textContent).toContain("Google sign-in isn't available right now.");
    expect(screen.getByTestId("connect-google").hasAttribute("disabled")).toBe(false);
  });

  it("has no Google row when Google is neither connected nor available", async () => {
    m.methods.get.mockResolvedValue({ password: true, google_available: false });
    view();
    await screen.findByTestId("method-password");
    expect(screen.queryByTestId("method-google")).toBeNull();
  });

  it("shows an error with a retry", async () => {
    const u = userEvent.setup();
    m.methods.get.mockRejectedValueOnce(apiError("network_error", 0)).mockResolvedValueOnce(both);
    view();
    const block = screen.getByTestId("sign-in-methods");
    await u.click(await within(block).findByRole("button", { name: "Try again" }));
    expect(await within(block).findByTestId("google-email")).toBeTruthy();
    expect(m.methods.get).toHaveBeenCalledTimes(2);
  });
});

describe("accounts without a password", () => {
  beforeEach(() => m.methods.get.mockResolvedValue({ password: false, google: both.google, google_available: true }));

  it("replaces the change-password form with the create-password state and e-mails a link", async () => {
    const u = userEvent.setup();
    view();
    const create = await screen.findByTestId("create-password");
    expect(screen.getByTestId("method-password").getAttribute("data-state")).toBe("not-set");
    expect(screen.queryByLabelText("Current Password")).toBeNull();
    expect(screen.getByTestId("password-create-state").textContent).toContain("You haven't created a password yet");
    expect(await axeViolations()).toEqual([]);

    await u.click(create);
    await waitFor(() => expect(m.auth.forgotPassword).toHaveBeenCalledWith("jane@example.com", "en"));
    expect((await screen.findByTestId("create-password-sent")).textContent).toContain("We've e-mailed you a link to create your password.");
    // Throttled: no second request right away.
    const again = screen.getByTestId("create-password");
    expect(again.textContent).toMatch(/Send again in \d+s/);
    expect(again.hasAttribute("disabled")).toBe(true);
    await u.click(again);
    expect(m.auth.forgotPassword).toHaveBeenCalledTimes(1);
  });

  it("shows the API's error when the link could not be requested", async () => {
    const u = userEvent.setup();
    m.auth.forgotPassword.mockRejectedValue(apiError("rate_limited", 429, { retryAfter: 30 }));
    view();
    await u.click(await screen.findByTestId("create-password"));
    const alert = await screen.findByText("Too many requests. Please wait a moment.");
    expect(alert).toBeTruthy();
    expect(screen.getByTestId("create-password").textContent).toMatch(/Send again in 30s/);
  });
});

describe("a stale list on the change-password form", () => {
  it("switches to the create-password state when the API says there is no password", async () => {
    const u = userEvent.setup();
    m.methods.get.mockResolvedValueOnce({ password: true, google_available: false }).mockResolvedValue({ password: false, google_available: false });
    m.auth.changePassword.mockRejectedValue(apiError("password_not_set"));
    view();
    await u.type(await screen.findByLabelText("Current Password"), "Old!pass1");
    await u.type(screen.getByLabelText("New Password"), "Newer!pass12");
    await u.type(screen.getByLabelText("Confirm New Password"), "Newer!pass12");
    await u.click(screen.getByTestId("update-password"));
    expect(await screen.findByTestId("password-create-state")).toBeTruthy();
    expect(screen.queryByLabelText("Current Password")).toBeNull();
  });
});

describe("disconnecting Google", () => {
  async function openDialog() {
    const u = userEvent.setup();
    view();
    await u.click(await screen.findByTestId("disconnect-google"));
    const dialog = await screen.findByRole("dialog");
    return { u, dialog };
  }

  it("asks for the password, disconnects, confirms with a toast and refreshes the list", async () => {
    const { u, dialog } = await openDialog();
    expect(within(dialog).getByText(/You won't be able to sign in with jane@gmail.com through Google any more/)).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
    await u.click(within(dialog).getByTestId("disconnect-google-confirm"));
    expect(await within(dialog).findByText("Enter your password.")).toBeTruthy();
    expect(m.methods.disconnectGoogle).not.toHaveBeenCalled();

    m.methods.disconnectGoogle.mockResolvedValue(undefined);
    m.methods.get.mockResolvedValue({ password: true, google_available: true });
    await u.type(within(dialog).getByLabelText("Confirm your password"), "Secret!1");
    await u.click(within(dialog).getByTestId("disconnect-google-confirm"));
    await waitFor(() => expect(m.methods.disconnectGoogle).toHaveBeenCalledWith("Secret!1"));
    expect(await screen.findByText("Google has been disconnected.")).toBeTruthy();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(await screen.findByTestId("connect-google")).toBeTruthy();
    expect(m.methods.get).toHaveBeenCalledTimes(2);
  });

  it("maps a wrong password onto the field", async () => {
    const { u, dialog } = await openDialog();
    m.methods.disconnectGoogle.mockRejectedValue(apiError("invalid_credentials", 401));
    await u.type(within(dialog).getByLabelText("Confirm your password"), "wrong");
    await u.click(within(dialog).getByTestId("disconnect-google-confirm"));
    expect(await within(dialog).findByText("The password is incorrect.")).toBeTruthy();
    expect(screen.getByRole("dialog")).toBeTruthy();
  });

  it("explains last_sign_in_method with the create-password action", async () => {
    const { u, dialog } = await openDialog();
    m.methods.disconnectGoogle.mockRejectedValue(apiError("last_sign_in_method"));
    await u.type(within(dialog).getByLabelText("Confirm your password"), "x");
    await u.click(within(dialog).getByTestId("disconnect-google-confirm"));
    const note = await within(dialog).findByTestId("last-sign-in-method");
    expect(note.textContent).toContain("Create a password before you disconnect Google, so you can still sign in.");
    await u.click(within(note).getByTestId("create-password"));
    await waitFor(() => expect(m.auth.forgotPassword).toHaveBeenCalledWith("jane@example.com", "en"));
    expect(await within(note).findByTestId("create-password-sent")).toBeTruthy();
  });

  it("shows the rate-limit copy", async () => {
    const { u, dialog } = await openDialog();
    m.methods.disconnectGoogle.mockRejectedValue(apiError("rate_limited", 429));
    await u.type(within(dialog).getByLabelText("Confirm your password"), "x");
    await u.click(within(dialog).getByTestId("disconnect-google-confirm"));
    expect(await within(dialog).findByText("Too many requests. Please wait a moment.")).toBeTruthy();
  });

  it("just refreshes when Google was already disconnected elsewhere", async () => {
    const { u, dialog } = await openDialog();
    m.methods.disconnectGoogle.mockRejectedValue(apiError("not_found", 404));
    m.methods.get.mockResolvedValue({ password: true, google_available: true });
    await u.type(within(dialog).getByLabelText("Confirm your password"), "x");
    await u.click(within(dialog).getByTestId("disconnect-google-confirm"));
    expect(await screen.findByTestId("connect-google")).toBeTruthy();
  });
});
