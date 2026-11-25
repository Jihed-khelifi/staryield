// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { ApiError } from "@/lib/api/errors";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const m = vi.hoisted(() => ({
  api: { verify: vi.fn(), resend: vi.fn() },
  session: {
    status: "authenticated",
    user: { id: "u1", email: "moon@example.com", email_verified: false } as Record<string, unknown> | null,
    reload: vi.fn(),
    updateUser: vi.fn(),
  },
  hint: { value: false },
}));
vi.mock("@/lib/api/email-verification", () => ({ emailVerificationApi: m.api }));
vi.mock("@/lib/api/client", () => ({ readAuthHint: () => m.hint.value }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => m.session }));
vi.mock("@/lib/psychics-link", () => ({ usePsychicsHref: () => "/en/psychics" }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/components/forms/auth/auth-card", () => ({
  AuthShell: ({ children }: { children: React.ReactNode }) => <main>{children}</main>,
  AuthLegal: () => null,
}));

import { EmailVerificationNotice } from "@/components/account/email-verification-notice";
import { EmailVerificationStatus } from "@/components/account/email-verification-status";
import { ResendVerification } from "@/components/account/resend-verification";
import { VerifyEmailPage } from "@/components/account/verify-email-page";

const err = (code: string, status: number, retryAfter?: number) => new ApiError({ code, message: code, status, retryAfter });
const sent = (seconds = 60) => ({ status: "sent", email: "m***@example.com", resend_available_at: new Date(Date.now() + seconds * 1000).toISOString() });
/** The account's waiting link (GET /me `email_verification`), sent `ago` seconds ago with a 60 s cooldown. */
const pendingLink = (ago = 15) => ({
  sent_at: new Date(Date.now() - ago * 1000).toISOString(),
  resend_available_at: new Date(Date.now() + (60 - ago) * 1000).toISOString(),
});

beforeAll(stubResizeObserver);
beforeEach(() => {
  m.session.status = "authenticated";
  m.session.user = { id: "u1", email: "moon@example.com", email_verified: false };
  m.hint.value = false;
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.clearAllMocks();
  window.history.replaceState(null, "", "/en");
});

describe("ResendVerification", () => {
  it("sends the link, names the masked address and counts down until another can be requested", async () => {
    const u = userEvent.setup();
    m.api.resend.mockResolvedValue(sent(60));
    renderWithI18n(<ResendVerification />);
    // No link is waiting (an account from before verification existed): the action is a first send.
    await u.click(screen.getByRole("button", { name: "Send me a link" }));
    expect(await screen.findByText(/We sent a new link to m\*\*\*@example.com/)).toBeTruthy();
    const button = screen.getByTestId("resend-verification-button") as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    expect(button.textContent).toMatch(/^Resend in (59|60)s$/);
    expect(screen.getByTestId("resend-verification").getAttribute("data-phase")).toBe("sent");
    expect(await axeViolations()).toEqual([]);
  });

  it("re-enables the button when the countdown ends", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    m.api.resend.mockResolvedValue(sent(3));
    renderWithI18n(<ResendVerification />);
    await act(async () => screen.getByTestId("resend-verification-button").click());
    await waitFor(() => expect((screen.getByTestId("resend-verification-button") as HTMLButtonElement).disabled).toBe(true));
    await act(async () => void vi.advanceTimersByTime(4000));
    await waitFor(() => expect((screen.getByTestId("resend-verification-button") as HTMLButtonElement).disabled).toBe(false));
    expect(screen.getByTestId("resend-verification-button").textContent).toBe("Resend link");
  });

  it("is busy while sending and sends once for a double click", async () => {
    const u = userEvent.setup();
    let resolve!: (v: unknown) => void;
    m.api.resend.mockReturnValue(new Promise((r) => (resolve = r)));
    renderWithI18n(<ResendVerification />);
    const button = screen.getByTestId("resend-verification-button") as HTMLButtonElement;
    await u.click(button);
    expect(button.getAttribute("aria-busy")).toBe("true");
    expect(button.disabled).toBe(true);
    expect(button.textContent).toBe("Sending…");
    await u.click(button);
    expect(m.api.resend).toHaveBeenCalledTimes(1);
    await act(async () => resolve(sent()));
  });

  it("shows the cooldown reported by the server (429 with Retry-After)", async () => {
    const u = userEvent.setup();
    m.api.resend.mockRejectedValue(err("rate_limited", 429, 30));
    renderWithI18n(<ResendVerification />);
    await u.click(screen.getByTestId("resend-verification-button"));
    expect(await screen.findByText(/A link was just sent/)).toBeTruthy();
    expect(screen.getByTestId("resend-verification-button").textContent).toMatch(/^Resend in (29|30)s$/);
  });

  it("explains an exhausted hourly budget in minutes", async () => {
    const u = userEvent.setup();
    m.api.resend.mockRejectedValue(err("rate_limited", 429, 1800));
    renderWithI18n(<ResendVerification />);
    await u.click(screen.getByTestId("resend-verification-button"));
    expect(await screen.findByText("You asked for several links. Please try again in 30 minutes.")).toBeTruthy();
  });

  it("treats 409 already_verified as success: refreshes the session and drops the button", async () => {
    const u = userEvent.setup();
    m.api.resend.mockRejectedValue(err("already_verified", 409));
    renderWithI18n(<ResendVerification />);
    await u.click(screen.getByTestId("resend-verification-button"));
    expect(await screen.findByText("Your email address is already verified.")).toBeTruthy();
    expect(screen.queryByTestId("resend-verification-button")).toBeNull();
    expect(m.session.reload).toHaveBeenCalledTimes(1);
  });

  it("starts counting down from the link the account already has, without a click into the cooldown", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    m.session.user = { id: "u1", email: "moon@example.com", email_verified: false, email_verification: pendingLink(15) };
    renderWithI18n(<ResendVerification />);
    const button = screen.getByTestId("resend-verification-button") as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    expect(button.textContent).toMatch(/^Resend in (44|45)s$/);
    // The notice already says a link was sent: the live region stays quiet.
    expect(screen.getByTestId("resend-verification-status").textContent).toBe("");
    await act(async () => void vi.advanceTimersByTime(46_000));
    await waitFor(() => expect(button.disabled).toBe(false));
    expect(button.textContent).toBe("Resend link");
    expect(m.api.resend).not.toHaveBeenCalled();
  });

  it("ignores a waiting link whose cooldown is over", () => {
    m.session.user = { id: "u1", email: "moon@example.com", email_verified: false, email_verification: pendingLink(600) };
    renderWithI18n(<ResendVerification />);
    const button = screen.getByTestId("resend-verification-button") as HTMLButtonElement;
    expect(button.disabled).toBe(false);
    expect(button.textContent).toBe("Resend link");
  });

  it("writes the sent link into the session, so every surface of the tab says a link is waiting", async () => {
    const u = userEvent.setup();
    const answer = sent(60);
    m.api.resend.mockResolvedValue(answer);
    renderWithI18n(<ResendVerification />);
    await u.click(screen.getByTestId("resend-verification-button"));
    await screen.findByText(/We sent a new link/);
    expect(m.session.updateUser).toHaveBeenCalledTimes(1);
    const update = m.session.updateUser.mock.calls[0][0] as (u: Record<string, unknown>) => Record<string, unknown>;
    const next = update({ id: "u1", email: "moon@example.com", email_verified: false, email_verification: null });
    expect(next.email_verification).toEqual({ sent_at: expect.any(String), resend_available_at: answer.resend_available_at });
  });

  it("explains the daily limit in hours", async () => {
    const u = userEvent.setup();
    m.api.resend.mockRejectedValue(err("rate_limited", 429, 20 * 3600 + 30));
    renderWithI18n(<ResendVerification />);
    await u.click(screen.getByTestId("resend-verification-button"));
    expect(await screen.findByText("You asked for several links today. Please try again in 21 hours.")).toBeTruthy();
    expect((screen.getByTestId("resend-verification-button") as HTMLButtonElement).disabled).toBe(true);
  });

  it("tells an address that refuses e-mail to check or change it, without a pointless retry", async () => {
    const u = userEvent.setup();
    m.api.resend.mockRejectedValue(err("email_undeliverable", 409));
    renderWithI18n(<ResendVerification />);
    await u.click(screen.getByTestId("resend-verification-button"));
    expect(await screen.findByText(/We can't deliver email to this address/)).toBeTruthy();
    expect(screen.queryByTestId("resend-verification-button")).toBeNull();
    const change = screen.getByTestId("resend-verification-change-email");
    expect(change.textContent).toBe("Change your email address");
    expect(change.getAttribute("href")).toBe("/en/profile/settings#account");
    expect(screen.getByTestId("resend-verification").getAttribute("data-phase")).toBe("undeliverable");
    expect(await axeViolations()).toEqual([]);
  });

  it("shows a retryable error", async () => {
    const u = userEvent.setup();
    m.api.resend.mockRejectedValueOnce(err("internal_error", 500)).mockResolvedValueOnce(sent());
    renderWithI18n(<ResendVerification />);
    await u.click(screen.getByTestId("resend-verification-button"));
    expect(await screen.findByText("Something went wrong on our side. Please try again.")).toBeTruthy();
    const retry = screen.getByRole("button", { name: "Try again" });
    await u.click(retry);
    expect(await screen.findByText(/We sent a new link/)).toBeTruthy();
    expect(m.api.resend).toHaveBeenCalledTimes(2);
  });
});

describe("EmailVerificationNotice", () => {
  it("says a link was sent when one is waiting, and offers another after the cooldown", async () => {
    m.session.user = { id: "u1", email: "moon@example.com", email_verified: false, email_verification: pendingLink(600) };
    renderWithI18n(<EmailVerificationNotice />);
    expect(screen.getByRole("heading", { name: "Confirm your email address" })).toBeTruthy();
    expect(screen.getByTestId("email-verification-notice-body").textContent).toBe(
      "We sent a link to moon@example.com. Confirming it secures your account and lets us send you reading receipts and account notices.",
    );
    expect(screen.getByRole("button", { name: "Resend link" })).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
  });

  it("claims no link for an account that never received one (created before verification existed, or by the team)", async () => {
    m.session.user = { id: "u1", email: "moon@example.com", email_verified: false, email_verification: null };
    renderWithI18n(<EmailVerificationNotice />);
    const body = screen.getByTestId("email-verification-notice-body").textContent ?? "";
    expect(body).toBe("Confirming moon@example.com secures your account and lets us send you reading receipts and account notices. We'll email you a link.");
    expect(body).not.toMatch(/We sent/);
    expect(screen.getByRole("button", { name: "Send me a link" })).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
  });

  it("is not shown to verified users, signed-out visitors or while the session loads", () => {
    for (const [status, user] of [
      ["authenticated", { id: "u1", email: "a@b.c", email_verified: true }],
      ["authenticated", { id: "u1", email: "a@b.c" }],
      ["anonymous", null],
      ["loading", null],
    ] as const) {
      m.session.status = status;
      m.session.user = user;
      const { container, unmount } = renderWithI18n(<EmailVerificationNotice />);
      expect(container.querySelector('[data-testid="email-verification-notice"]'), `${status} ${JSON.stringify(user)}`).toBeNull();
      unmount();
    }
  });

  it("disappears when the session learns the address is verified (another tab)", () => {
    const view = renderWithI18n(<EmailVerificationNotice />);
    expect(screen.getByTestId("email-verification-notice")).toBeTruthy();
    m.session.user = { id: "u1", email: "moon@example.com", email_verified: true };
    view.rerender(<EmailVerificationNotice />);
    expect(screen.queryByTestId("email-verification-notice")).toBeNull();
  });
});

describe("EmailVerificationStatus (settings row)", () => {
  it("shows a Verified badge from the session", () => {
    m.session.user = { id: "u1", email: "a@b.c", email_verified: true };
    renderWithI18n(<EmailVerificationStatus verified={false} />);
    expect(screen.getByTestId("email-verified-badge").textContent).toBe("Verified");
  });

  it("shows Not verified with the send control", () => {
    renderWithI18n(<EmailVerificationStatus verified={false} />);
    expect(screen.getByText("Not verified")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Send me a link" })).toBeTruthy();
  });

  it("waits for the session user before offering a link (the countdown starts from the account's link)", () => {
    m.session.user = null;
    renderWithI18n(<EmailVerificationStatus verified={false} />);
    expect(screen.getByText("Not verified")).toBeTruthy();
    expect(screen.queryByTestId("resend-verification")).toBeNull();
  });

  it("falls back to the settings document until the session user is known", () => {
    m.session.user = null;
    renderWithI18n(<EmailVerificationStatus verified />);
    expect(screen.getByTestId("email-verified-badge")).toBeTruthy();
  });
});

describe("VerifyEmailPage", () => {
  const phase = () => screen.getByTestId("verify-email").getAttribute("data-phase");

  it("posts the token from the fragment at once, without a click, and strips the address bar", async () => {
    window.history.replaceState(null, "", "/en/verify-email#token=secret-token");
    let resolve!: (v: unknown) => void;
    m.api.verify.mockReturnValue(new Promise((r) => (resolve = r)));
    renderWithI18n(<VerifyEmailPage />);
    await waitFor(() => expect(m.api.verify).toHaveBeenCalledWith("secret-token"));
    expect(window.location.hash).toBe("");
    expect(phase()).toBe("verifying");
    expect(screen.getByTestId("verify-email").getAttribute("aria-busy")).toBe("true");
    await act(async () => resolve({ status: "verified", email_verified: true }));
    expect(await screen.findByRole("heading", { name: "Your email is verified" })).toBeTruthy();
    expect(phase()).toBe("done-verified");
    expect(screen.getByTestId("verify-email-advisors").getAttribute("href")).toBe("/en/psychics");
    expect(screen.getByTestId("verify-email-profile").getAttribute("href")).toBe("/en/profile");
    expect(m.api.verify).toHaveBeenCalledTimes(1);
    expect(await axeViolations()).toEqual([]);
  });

  it("refreshes the session when this browser holds one", async () => {
    window.history.replaceState(null, "", "/en/verify-email#token=t");
    m.hint.value = true;
    m.api.verify.mockResolvedValue({ status: "verified", email_verified: true });
    renderWithI18n(<VerifyEmailPage />);
    await screen.findByText("Your email is verified");
    expect(m.session.reload).toHaveBeenCalledTimes(1);
  });

  it("does not touch the session of a browser without one (another device)", async () => {
    window.history.replaceState(null, "", "/en/verify-email#token=t");
    m.session.status = "anonymous";
    m.session.user = null;
    m.api.verify.mockResolvedValue({ status: "verified", email_verified: true });
    renderWithI18n(<VerifyEmailPage />);
    await screen.findByText("Your email is verified");
    expect(m.session.reload).not.toHaveBeenCalled();
  });

  it("says when the address was already verified", async () => {
    window.history.replaceState(null, "", "/en/verify-email#token=t");
    m.api.verify.mockResolvedValue({ status: "already_verified", email_verified: true });
    renderWithI18n(<VerifyEmailPage />);
    expect(await screen.findByRole("heading", { name: "Your email is already verified" })).toBeTruthy();
    expect(phase()).toBe("done-already_verified");
  });

  it("explains an expired link and offers a new one to a signed-in user", async () => {
    const u = userEvent.setup();
    window.history.replaceState(null, "", "/en/verify-email#token=old");
    m.api.verify.mockRejectedValue(err("invalid_token", 400));
    m.api.resend.mockResolvedValue(sent());
    renderWithI18n(<VerifyEmailPage />);
    expect(await screen.findByRole("heading", { name: "This link has expired" })).toBeTruthy();
    expect(phase()).toBe("invalid");
    await u.click(screen.getByRole("button", { name: "Send a new link" }));
    expect(await screen.findByText(/We sent a new link to m\*\*\*@example.com/)).toBeTruthy();
    expect(m.api.verify).toHaveBeenCalledTimes(1);
  });

  it("sends a signed-out visitor to log in, returning to the profile", async () => {
    window.history.replaceState(null, "", "/en/verify-email#token=old");
    m.session.status = "anonymous";
    m.session.user = null;
    m.api.verify.mockRejectedValue(err("invalid_token", 400));
    renderWithI18n(<VerifyEmailPage />);
    const link = await screen.findByTestId("verify-email-login");
    expect(link.textContent).toBe("Log in to get a new link");
    expect(link.getAttribute("href")).toBe(`/en/login?next=${encodeURIComponent("/en/profile")}`);
  });

  it("recognises an already verified account behind a used link", async () => {
    window.history.replaceState(null, "", "/en/verify-email#token=used");
    m.session.user = { id: "u1", email: "a@b.c", email_verified: true };
    m.api.verify.mockRejectedValue(err("invalid_token", 400));
    renderWithI18n(<VerifyEmailPage />);
    expect(await screen.findByRole("heading", { name: "Your email is already verified" })).toBeTruthy();
  });

  it("retries a network or server failure with the same token", async () => {
    const u = userEvent.setup();
    window.history.replaceState(null, "", "/en/verify-email#token=keep");
    m.api.verify.mockRejectedValueOnce(err("network_error", 0)).mockResolvedValueOnce({ status: "verified", email_verified: true });
    renderWithI18n(<VerifyEmailPage />);
    expect(await screen.findByRole("heading", { name: "We couldn't confirm your email" })).toBeTruthy();
    expect(phase()).toBe("error");
    await u.click(screen.getByTestId("verify-email-retry"));
    expect(await screen.findByRole("heading", { name: "Your email is verified" })).toBeTruthy();
    expect(m.api.verify).toHaveBeenNthCalledWith(2, "keep");
  });

  it("takes the token of another link opened in the same tab (fragment-only navigation)", async () => {
    window.history.replaceState(null, "", "/en/verify-email#token=first");
    m.api.verify.mockResolvedValueOnce({ status: "verified", email_verified: true }).mockRejectedValueOnce(err("invalid_token", 400));
    renderWithI18n(<VerifyEmailPage />);
    expect(await screen.findByRole("heading", { name: "Your email is verified" })).toBeTruthy();
    await act(async () => {
      window.history.replaceState(null, "", "/en/verify-email#token=first");
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });
    expect(await screen.findByRole("heading", { name: "This link has expired" })).toBeTruthy();
    expect(m.api.verify).toHaveBeenNthCalledWith(2, "first");
    expect(window.location.hash).toBe("");
    // A fragment without a token (or the strip itself) changes nothing.
    await act(async () => window.dispatchEvent(new HashChangeEvent("hashchange")));
    expect(m.api.verify).toHaveBeenCalledTimes(2);
  });

  it("explains an incomplete link (no token) without calling the API", async () => {
    renderWithI18n(<VerifyEmailPage />);
    expect(await screen.findByRole("heading", { name: "This link is incomplete" })).toBeTruthy();
    expect(m.api.verify).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Send a new link" })).toBeTruthy();
  });

  it("sends a signed-out visitor with an incomplete link to log in, and waits for the session before offering anything", async () => {
    m.session.status = "loading";
    m.session.user = null;
    const view = renderWithI18n(<VerifyEmailPage />);
    expect(await screen.findByRole("heading", { name: "This link is incomplete" })).toBeTruthy();
    expect(screen.queryByTestId("verify-email-next")).toBeNull();
    expect(screen.queryByTestId("resend-verification")).toBeNull();
    m.session.status = "anonymous";
    view.rerender(<VerifyEmailPage />);
    expect(screen.getByTestId("verify-email-next").textContent).toBe("Log in to get a new link");
  });
});
