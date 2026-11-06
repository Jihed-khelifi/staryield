// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { ApiError } from "@/lib/api/errors";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";
import { BirthInfoForm } from "@/components/forms/auth/birth-info-form";
import { ForgotPasswordForm } from "@/components/forms/auth/forgot-password-form";
import { LoginForm } from "@/components/forms/auth/login-form";
import { ResetPasswordForm } from "@/components/forms/auth/reset-password-form";
import { SignupEmailForm } from "@/components/forms/auth/signup-email-form";
import { SignupPasswordForm } from "@/components/forms/auth/signup-password-form";

const router = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));
const session = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));
const account = vi.hoisted(() => ({ reactivate: vi.fn() }));
const api = vi.hoisted(() => ({
  register: vi.fn(),
  login: vi.fn(),
  forgotPassword: vi.fn(),
  resetPassword: vi.fn(),
  updateMe: vi.fn(),
  places: vi.fn(),
}));

vi.mock("next/navigation", () => ({ useRouter: () => router }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/analytics", () => ({ track: vi.fn() }));
vi.mock("@/lib/attribution", () => ({ attributionForSignup: () => undefined }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => session.current }));
vi.mock("@/components/layout/public-frame", () => ({
  PublicFrame: ({ children }: { children: React.ReactNode }) => <main>{children}</main>,
}));
vi.mock("@/components/forms/radiant-panel", () => ({
  RadiantPanel: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));
vi.mock("@/lib/api/account", () => ({ accountApi: { reactivate: account.reactivate } }));
vi.mock("@/lib/api/endpoints", () => ({
  authApi: { register: api.register, login: api.login, forgotPassword: api.forgotPassword, resetPassword: api.resetPassword },
  meApi: { update: api.updateMe },
  astrologyApi: { places: api.places },
}));

const user = { id: "u1", email: "jane@example.com", role: "client", profile: {} };
const setUser = vi.fn();

beforeAll(stubResizeObserver);
beforeEach(() => {
  session.current = { status: "anonymous", user: null, setUser };
  sessionStorage.clear();
  window.history.replaceState(null, "", "/en/signup");
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

const submitButton = () => screen.getByRole("button", { name: /^Continu/ });

describe("SignupEmailForm", () => {
  it("requires an email and the terms, then continues to the password step", async () => {
    const u = userEvent.setup();
    renderWithI18n(<SignupEmailForm />);
    await u.click(submitButton());
    expect(await screen.findByText("Enter your email address.")).toBeTruthy();
    expect(screen.getByText("Please accept the Terms of Service and Privacy Policy.")).toBeTruthy();
    expect(screen.getAllByRole("checkbox")).toHaveLength(1);
    expect(await axeViolations()).toEqual([]);

    await u.type(screen.getByRole("textbox", { name: "Email Address" }), " jane@example.com ");
    await u.click(screen.getByRole("checkbox"));
    await u.click(submitButton());
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/en/signup/password"));
    expect(sessionStorage.getItem("staryield.signup.email")).toBe("jane@example.com");
  });
});

describe("SignupPasswordForm", () => {
  beforeEach(() => sessionStorage.setItem("staryield.signup.email", "jane@example.com"));

  it("ticks the password rules live", async () => {
    const u = userEvent.setup();
    renderWithI18n(<SignupPasswordForm />);
    const list = screen.getByRole("list", { name: "Password requirements" });
    const met = () => within(list).getAllByRole("listitem").filter((li) => li.hasAttribute("data-met")).length;
    expect(met()).toBe(0);
    const password = screen.getByLabelText("New password");
    await u.type(password, "abcdefgh");
    expect(met()).toBe(1);
    await u.type(password, "A1!");
    expect(met()).toBe(4);
    expect(within(list).getAllByText("Requirement met")).toHaveLength(4);
    expect(password.getAttribute("aria-describedby")).toContain(list.id);
  });

  it("creates the account and moves on to the birth step", async () => {
    const u = userEvent.setup();
    api.register.mockResolvedValue({ user });
    renderWithI18n(<SignupPasswordForm />);
    await u.type(screen.getByLabelText("New password"), "Longenough1!");
    await u.type(screen.getByLabelText("Confirm password"), "Longenough1!");
    await u.click(submitButton());
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/en/signup/birth"));
    expect(api.register).toHaveBeenCalledWith({ email: "jane@example.com", password: "Longenough1!", accept_terms: true, locale: "en", attribution: undefined });
    expect(setUser).toHaveBeenCalledWith(user);
    expect(sessionStorage.getItem("staryield.signup.email")).toBeNull();
  });

  it("maps email_taken to a form error with a Log In link", async () => {
    const u = userEvent.setup();
    api.register.mockRejectedValue(new ApiError({ code: "email_taken", message: "taken", status: 409 }));
    renderWithI18n(<SignupPasswordForm />);
    await u.type(screen.getByLabelText("New password"), "Longenough1!");
    await u.type(screen.getByLabelText("Confirm password"), "Longenough1!");
    await u.click(submitButton());
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("An account with this email already exists.");
    expect(within(alert).getByRole("link", { name: "Log In" }).getAttribute("href")).toBe("/en/login");
  });

  it("sends an invalid email back to step 1", async () => {
    const u = userEvent.setup();
    api.register.mockRejectedValue(
      new ApiError({ code: "validation_failed", message: "bad", status: 422, details: { fields: { email: "Enter a valid email address." } } })
    );
    renderWithI18n(<SignupPasswordForm />);
    await u.type(screen.getByLabelText("New password"), "Longenough1!");
    await u.type(screen.getByLabelText("Confirm password"), "Longenough1!");
    await u.click(submitButton());
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Enter a valid email address.");
    expect(within(alert).getByRole("link", { name: "Change email" }).getAttribute("href")).toBe("/en/signup");
  });

  it("goes back to step 1 without a saved email", () => {
    sessionStorage.clear();
    renderWithI18n(<SignupPasswordForm />);
    expect(router.replace).toHaveBeenCalledWith("/en/signup");
  });
});

describe("BirthInfoForm", () => {
  beforeEach(() => {
    session.current = { status: "authenticated", user, setUser };
    api.places.mockResolvedValue([
      { id: "2988507", name: "Paris", admin1: "Île-de-France", country: "France", time_zone: "Europe/Paris" },
      { id: "4717560", name: "Paris", admin1: "Texas", country: "United States", time_zone: "America/Chicago" },
      null,
    ]);
  });

  async function fillDate(u: ReturnType<typeof userEvent.setup>, time = "08:45") {
    await u.type(screen.getByRole("textbox", { name: /Day/ }), "14");
    await u.type(screen.getByRole("textbox", { name: "Month" }), "03");
    await u.type(screen.getByRole("textbox", { name: "Year" }), "1990");
    if (time) await u.type(screen.getByRole("textbox", { name: "Time" }), time);
  }

  it("asks for a birthplace when a time is given, then saves the profile", async () => {
    const u = userEvent.setup();
    api.updateMe.mockResolvedValue({ ...user, profile: { birth_date: "1990-03-14" } });
    renderWithI18n(<BirthInfoForm />);
    expect(screen.getByText("Your birth details stay private and are only used for your charts.")).toBeTruthy();
    await fillDate(u);
    await u.click(submitButton());
    expect(await screen.findByText("Choose where you were born so we can place your rising sign.")).toBeTruthy();
    expect(api.updateMe).not.toHaveBeenCalled();

    await u.type(screen.getByRole("combobox"), "Par");
    await u.click(await screen.findByRole("option", { name: "Paris, Île-de-France, France" }));
    expect(api.places.mock.calls[0][0]).toBe("Par");
    await u.click(submitButton());
    await waitFor(() =>
      expect(api.updateMe).toHaveBeenCalledWith({ birth_date: "1990-03-14", birth_time: "08:45", birth_place_id: "2988507" })
    );
    expect(router.push).toHaveBeenCalledWith("/en/signup/offer");
    expect(await axeViolations()).toEqual([]);
  });

  it("“I don't know my time” hides the time and makes the place optional", async () => {
    const u = userEvent.setup();
    api.updateMe.mockResolvedValue(user);
    renderWithI18n(<BirthInfoForm />);
    await fillDate(u, "");
    await u.click(submitButton());
    expect(await screen.findByText("Enter your birth time, or tick “I don't know my time”.")).toBeTruthy();

    await u.click(screen.getByRole("checkbox", { name: "I don't know my time" }));
    expect(screen.queryByRole("textbox", { name: "Time" })).toBeNull();
    expect(screen.getByText("No problem: we use noon, so your rising sign and houses stay hidden.")).toBeTruthy();
    await u.click(submitButton());
    await waitFor(() => expect(api.updateMe).toHaveBeenCalledWith({ birth_date: "1990-03-14", birth_time: "", birth_place_id: "" }));
  });

  it("rejects minors and shows server field errors on the right field", async () => {
    const u = userEvent.setup();
    renderWithI18n(<BirthInfoForm />);
    await u.type(screen.getByRole("textbox", { name: /Day/ }), "01");
    await u.type(screen.getByRole("textbox", { name: "Month" }), "01");
    await u.type(screen.getByRole("textbox", { name: "Year" }), String(new Date().getFullYear() - 10));
    await u.click(screen.getByRole("checkbox", { name: "I don't know my time" }));
    await u.click(submitButton());
    expect(await screen.findByText("You must be at least 18 years old.")).toBeTruthy();

    await u.clear(screen.getByRole("textbox", { name: "Year" }));
    await u.type(screen.getByRole("textbox", { name: "Year" }), "1990");
    api.updateMe.mockRejectedValue(
      new ApiError({ code: "validation_failed", message: "bad", status: 422, details: { fields: { birth_place_id: "Choose a place from the list." } } })
    );
    await u.click(submitButton());
    expect(await screen.findByText("Choose a place from the list.")).toBeTruthy();
    expect(screen.getByRole("combobox").getAttribute("aria-invalid")).toBe("true");
  });

  it("sends signed-out visitors to log in", () => {
    session.current = { status: "anonymous", user: null, setUser };
    renderWithI18n(<BirthInfoForm />);
    expect(router.replace).toHaveBeenCalledWith(`/en/login?next=${encodeURIComponent("/en/signup/birth")}`);
  });
});

describe("LoginForm", () => {
  async function signIn(u: ReturnType<typeof userEvent.setup>) {
    await u.type(screen.getByRole("textbox", { name: "Email Address" }), "jane@example.com");
    await u.type(screen.getByLabelText("Password"), "Secret-pass1");
    await u.click(submitButton());
  }

  it("keeps wrong credentials a form-level error and clears the password", async () => {
    const u = userEvent.setup();
    api.login.mockRejectedValue(new ApiError({ code: "invalid_credentials", message: "nope", status: 401 }));
    renderWithI18n(<LoginForm />);
    await signIn(u);
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("The email or password is incorrect.");
    expect((screen.getByLabelText("Password") as HTMLInputElement).value).toBe("");
    expect(screen.getByRole("textbox", { name: "Email Address" }).getAttribute("aria-invalid")).toBe("false");
    expect(await axeViolations()).toEqual([]);
  });

  it("counts down a lock-out and blocks the form meanwhile", async () => {
    const u = userEvent.setup();
    api.login.mockRejectedValue(new ApiError({ code: "account_locked", message: "locked", status: 429, retryAfter: 2 }));
    renderWithI18n(<LoginForm />);
    await signIn(u);
    expect((await screen.findByRole("alert")).textContent).toContain("Too many sign-in attempts. Please try again later.");
    expect(screen.getByRole("timer").textContent).toBe("Try again in 2s");
    expect(submitButton().hasAttribute("disabled")).toBe(true);
    await waitFor(() => expect(screen.getByRole("timer").textContent).toBe("Try again in 1s"), { timeout: 2500 });
    await waitFor(() => expect(submitButton().hasAttribute("disabled")).toBe(false), { timeout: 2500 });
  });

  it("signs in and follows a safe ?next", async () => {
    const u = userEvent.setup();
    window.history.replaceState(null, "", "/en/login?next=%2Fen%2Fcredits");
    api.login.mockResolvedValue({ user });
    renderWithI18n(<LoginForm />);
    await signIn(u);
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/en/credits"));
    expect(api.login).toHaveBeenCalledWith("jane@example.com", "Secret-pass1");
  });
});

describe("ForgotPasswordForm", () => {
  it("validates the email and answers the same way for every address", async () => {
    const u = userEvent.setup();
    api.forgotPassword.mockResolvedValue(undefined);
    renderWithI18n(<ForgotPasswordForm />);
    await u.type(screen.getByRole("textbox", { name: "Email Address" }), "jane@");
    await u.click(submitButton());
    expect(await screen.findByText("Enter a valid email address.")).toBeTruthy();
    await u.type(screen.getByRole("textbox", { name: "Email Address" }), "example.com");
    await u.click(submitButton());
    expect(await screen.findByText("If an account exists for this email, a reset link is on its way.")).toBeTruthy();
    expect(api.forgotPassword).toHaveBeenCalledWith("jane@example.com", "en");
    expect(screen.getByRole("button", { name: "Send again" })).toBeTruthy();
  });

  it("shows rate limiting as a form error", async () => {
    const u = userEvent.setup();
    api.forgotPassword.mockRejectedValue(new ApiError({ code: "rate_limited", message: "slow down", status: 429 }));
    renderWithI18n(<ForgotPasswordForm />);
    await u.type(screen.getByRole("textbox", { name: "Email Address" }), "jane@example.com");
    await u.click(submitButton());
    expect((await screen.findByRole("alert")).textContent).toContain("Too many requests. Please wait a moment.");
  });
});

describe("ResetPasswordForm", () => {
  it("explains a link without a token and offers a new one", () => {
    window.history.replaceState(null, "", "/en/reset-password");
    renderWithI18n(<ResetPasswordForm />);
    const alert = screen.getByRole("alert");
    expect(alert.textContent).toContain("This link is invalid or has expired.");
    expect(within(alert).getByRole("link", { name: "Request a new link" }).getAttribute("href")).toBe("/en/forgot-password");
    expect(submitButton().hasAttribute("disabled")).toBe(true);
  });

  it("maps invalid_token to “Request a new link”", async () => {
    const u = userEvent.setup();
    window.history.replaceState(null, "", "/en/reset-password?token=expired-token");
    api.resetPassword.mockRejectedValue(new ApiError({ code: "invalid_token", message: "expired", status: 400 }));
    renderWithI18n(<ResetPasswordForm />);
    await u.type(screen.getByLabelText("New password"), "Longenough1!");
    await u.type(screen.getByLabelText("Confirm password"), "Longenough1!");
    await u.click(submitButton());
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("This link is invalid or has expired.");
    expect(within(alert).getByRole("link", { name: "Request a new link" })).toBeTruthy();
    expect(api.resetPassword).toHaveBeenCalledWith("expired-token", "Longenough1!");
  });

  it("confirms the new password and links to sign in", async () => {
    const u = userEvent.setup();
    window.history.replaceState(null, "", "/en/reset-password?token=good-token");
    api.resetPassword.mockResolvedValue(undefined);
    renderWithI18n(<ResetPasswordForm />);
    await u.type(screen.getByLabelText("New password"), "Longenough1!");
    await u.type(screen.getByLabelText("Confirm password"), "Longenough1?");
    await u.click(submitButton());
    expect(await screen.findByText("The passwords do not match.")).toBeTruthy();
    await u.clear(screen.getByLabelText("Confirm password"));
    await u.type(screen.getByLabelText("Confirm password"), "Longenough1!");
    await u.click(submitButton());
    expect(await screen.findByText("Your password has been updated. You can now sign in.")).toBeTruthy();
  });
});

describe("LoginForm reactivation", () => {
  async function signInTo(error: ApiError) {
    const u = userEvent.setup();
    api.login.mockRejectedValue(error);
    renderWithI18n(<LoginForm />);
    await u.type(screen.getByLabelText("Email Address"), "jane@example.com");
    await u.type(screen.getByLabelText("Password"), "Secret!1pass");
    await u.click(screen.getByRole("button", { name: /^Continu/ }));
    return u;
  }

  it("offers to reactivate a disabled account and signs in with the token", async () => {
    const u = await signInTo(new ApiError({ code: "account_deactivated", message: "x", status: 403, details: { reactivation_token: "tok1" } }));
    expect(await screen.findByTestId("reactivation-offer")).toBeTruthy();
    account.reactivate.mockResolvedValue({ user: { ...user, role: "client" } });
    await u.click(screen.getByRole("button", { name: "Reactivate my account" }));
    await waitFor(() => expect(account.reactivate).toHaveBeenCalledWith("tok1"));
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/en/profile"));
  });

  it("offers Keep my account for a scheduled closure, with the deletion date", async () => {
    await signInTo(new ApiError({ code: "account_closure_pending", message: "x", status: 403, details: { reactivation_token: "tok2", deletion_scheduled_at: "2026-12-05T00:00:00Z" } }));
    const offer = await screen.findByTestId("reactivation-offer");
    expect(offer.textContent).toContain("December");
    expect(within(offer).getByRole("button", { name: "Keep my account" })).toBeTruthy();
  });

  it("explains an expired offer", async () => {
    const u = await signInTo(new ApiError({ code: "account_deactivated", message: "x", status: 403, details: { reactivation_token: "old" } }));
    account.reactivate.mockRejectedValue(new ApiError({ code: "invalid_token", message: "x", status: 400 }));
    await u.click(await screen.findByRole("button", { name: "Reactivate my account" }));
    expect(await screen.findByText("This offer has expired. Sign in again to continue.")).toBeTruthy();
  });
});
