// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { ApiError } from "@/lib/api/errors";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";
import { browser } from "@/lib/auth/google";
import { GoogleButton } from "@/components/forms/auth/google-button";
import { LoginForm } from "@/components/forms/auth/login-form";
import { SignupEmailForm } from "@/components/forms/auth/signup-email-form";

const router = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));
const site = vi.hoisted(() => ({ config: null as unknown }));
const api = vi.hoisted(() => ({ googleStart: vi.fn(), login: vi.fn() }));
const toast = vi.hoisted(() => ({ show: vi.fn(), showError: vi.fn() }));
const setUser = vi.fn();

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
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: "anonymous", user: null, setUser }) }));
vi.mock("@/lib/site-config/site-config-provider", () => ({ useSiteConfig: () => ({ config: site.config, status: "ready", refresh: vi.fn() }) }));
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => toast }));
vi.mock("@/components/layout/public-frame", () => ({ PublicFrame: ({ children }: { children: React.ReactNode }) => <main>{children}</main> }));
vi.mock("@/components/forms/radiant-panel", () => ({ RadiantPanel: ({ children }: { children: React.ReactNode }) => <div>{children}</div> }));
vi.mock("@/lib/api/account", () => ({ accountApi: { reactivate: vi.fn() } }));
vi.mock("@/lib/api/endpoints", () => ({ authApi: { googleStart: api.googleStart, login: api.login } }));

const configured = (google: boolean | undefined) => ({ auth: google === undefined ? undefined : { google } });
const GOOGLE_URL = "https://accounts.google.com/o/oauth2/v2/auth?state=abc";
let assign: ReturnType<typeof vi.spyOn>;

beforeAll(stubResizeObserver);
beforeEach(() => {
  site.config = configured(true);
  sessionStorage.clear();
  window.history.replaceState(null, "", "/en/login");
  assign = vi.spyOn(browser, "assign").mockImplementation(() => {});
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.restoreAllMocks();
});

describe("GoogleButton", () => {
  it.each([
    ["no site config yet", null],
    ["an older API without `auth`", configured(undefined)],
    ["Google not configured", configured(false)],
  ])("renders nothing with %s", (_label, config) => {
    site.config = config;
    renderWithI18n(<GoogleButton intent="signin" onError={vi.fn()} />);
    expect(screen.queryByTestId("google-button")).toBeNull();
  });

  it("is a 40px branded button with the four-colour G and the right label per intent", async () => {
    const { rerender } = renderWithI18n(<GoogleButton intent="signin" onError={vi.fn()} />);
    const button = screen.getByRole("button", { name: "Continue with Google" });
    expect(button.className).toContain("h-10");
    expect(button.className).toContain("w-full");
    expect(button.className).toContain("border-[#747775]");
    expect(button.className).toContain("bg-[#FFFFFF]");
    expect(button.className).toContain("text-[#1F1F1F]");
    const g = screen.getByTestId("google-g");
    expect(g.getAttribute("width")).toBe("18");
    expect(g.getAttribute("aria-hidden")).toBe("true");
    expect([...g.querySelectorAll("path")].map((p) => p.getAttribute("fill"))).toEqual(["#EA4335", "#4285F4", "#FBBC05", "#34A853"]);
    expect(await axeViolations()).toEqual([]);
    rerender(<GoogleButton intent="signup" onError={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Sign up with Google" })).toBeTruthy();
  });

  it("starts the flow and navigates to Google", async () => {
    const u = userEvent.setup();
    api.googleStart.mockResolvedValue({ authorization_url: GOOGLE_URL });
    renderWithI18n(<GoogleButton intent="signin" next={() => "/chatroom"} onError={vi.fn()} />);
    await u.click(screen.getByRole("button", { name: "Continue with Google" }));
    await waitFor(() => expect(assign).toHaveBeenCalledWith(GOOGLE_URL));
    expect(api.googleStart).toHaveBeenCalledWith({ intent: "signin", next: "/chatroom", locale: "en", attribution: undefined });
  });

  it("shows a busy, disabled state while the start call runs", async () => {
    const u = userEvent.setup();
    let resolve!: (v: { authorization_url: string }) => void;
    api.googleStart.mockReturnValue(new Promise((r) => (resolve = r)));
    renderWithI18n(<GoogleButton intent="signin" onError={vi.fn()} />);
    await u.click(screen.getByRole("button", { name: "Continue with Google" }));
    const busy = await screen.findByRole("button", { name: "Opening Google…" });
    expect(busy.hasAttribute("disabled")).toBe(true);
    expect(busy.getAttribute("aria-busy")).toBe("true");
    expect(screen.queryByTestId("google-g")).toBeNull();
    await u.click(busy);
    expect(api.googleStart).toHaveBeenCalledTimes(1);
    resolve({ authorization_url: GOOGLE_URL });
    await waitFor(() => expect(assign).toHaveBeenCalled());
  });

  it("reports an API error and becomes usable again", async () => {
    const u = userEvent.setup();
    const onError = vi.fn();
    api.googleStart.mockRejectedValue(new ApiError({ code: "google_unavailable", message: "x", status: 503 }));
    renderWithI18n(<GoogleButton intent="signin" onError={onError} />);
    await u.click(screen.getByRole("button", { name: "Continue with Google" }));
    await waitFor(() => expect(onError).toHaveBeenCalledTimes(1));
    expect(onError.mock.calls[0][0].code).toBe("google_unavailable");
    expect(screen.getByRole("button", { name: "Continue with Google" }).hasAttribute("disabled")).toBe(false);
    expect(assign).not.toHaveBeenCalled();
  });

  it.each(["http://evil.example/auth", "javascript:alert(1)", "//evil.example/auth"])("refuses to navigate to %s", async (url) => {
    const u = userEvent.setup();
    const onError = vi.fn();
    api.googleStart.mockResolvedValue({ authorization_url: url });
    renderWithI18n(<GoogleButton intent="signin" onError={onError} />);
    await u.click(screen.getByRole("button", { name: "Continue with Google" }));
    await waitFor(() => expect(onError).toHaveBeenCalled());
    expect(assign).not.toHaveBeenCalled();
  });

  it("stops when beforeStart says no", async () => {
    const u = userEvent.setup();
    renderWithI18n(<GoogleButton intent="signup" beforeStart={() => false} onError={vi.fn()} />);
    await u.click(screen.getByRole("button", { name: "Sign up with Google" }));
    expect(api.googleStart).not.toHaveBeenCalled();
  });
});

describe("SignupEmailForm with Google", () => {
  it("shows the divider and button only when Google is configured", () => {
    site.config = configured(false);
    const { unmount } = renderWithI18n(<SignupEmailForm />);
    expect(screen.queryByTestId("google-button")).toBeNull();
    expect(screen.queryByTestId("auth-divider")).toBeNull();
    unmount();
    site.config = configured(true);
    renderWithI18n(<SignupEmailForm />);
    expect(screen.getByTestId("auth-divider").textContent).toContain("or");
    expect(screen.getByRole("button", { name: "Sign up with Google" })).toBeTruthy();
  });

  it("needs the terms box, with the same message as the e-mail path", async () => {
    const u = userEvent.setup();
    renderWithI18n(<SignupEmailForm />);
    await u.click(screen.getByRole("button", { name: "Sign up with Google" }));
    expect(await screen.findByText("Please accept the Terms of Service and Privacy Policy.")).toBeTruthy();
    expect(api.googleStart).not.toHaveBeenCalled();
  });

  it("starts the sign-up with accept_terms once the box is ticked", async () => {
    const u = userEvent.setup();
    api.googleStart.mockResolvedValue({ authorization_url: GOOGLE_URL });
    renderWithI18n(<SignupEmailForm />);
    await u.click(screen.getByRole("checkbox"));
    await u.click(screen.getByRole("button", { name: "Sign up with Google" }));
    await waitFor(() => expect(assign).toHaveBeenCalledWith(GOOGLE_URL));
    expect(api.googleStart).toHaveBeenCalledWith({ intent: "signup", accept_terms: true, next: undefined, locale: "en", attribution: undefined });
    expect(await axeViolations()).toEqual([]);
  });

  it("shows the code's copy through the form's root error", async () => {
    const u = userEvent.setup();
    api.googleStart.mockRejectedValue(new ApiError({ code: "google_unavailable", message: "x", status: 503 }));
    renderWithI18n(<SignupEmailForm />);
    await u.click(screen.getByRole("checkbox"));
    await u.click(screen.getByRole("button", { name: "Sign up with Google" }));
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Google sign-in isn't available right now.");
  });
});

describe("LoginForm with Google", () => {
  it("offers Google when configured and sends the sign-in intent with the current ?next", async () => {
    const u = userEvent.setup();
    window.history.replaceState(null, "", "/en/login?next=%2Fen%2Fchatroom");
    api.googleStart.mockResolvedValue({ authorization_url: GOOGLE_URL });
    renderWithI18n(<LoginForm />);
    expect(screen.getByTestId("auth-divider")).toBeTruthy();
    await u.click(screen.getByRole("button", { name: "Continue with Google" }));
    await waitFor(() => expect(assign).toHaveBeenCalledWith(GOOGLE_URL));
    expect(api.googleStart).toHaveBeenCalledWith({ intent: "signin", next: "/chatroom", locale: "en", attribution: undefined });
    expect(await axeViolations()).toEqual([]);
  });

  it("has no Google button when it is not configured", () => {
    site.config = configured(false);
    renderWithI18n(<LoginForm />);
    expect(screen.queryByTestId("google-button")).toBeNull();
  });

  it("shows the link notice with the e-mail prefilled, then confirms the connection after the password sign-in", async () => {
    const u = userEvent.setup();
    window.history.replaceState(null, "", "/en/login?notice=google_link");
    sessionStorage.setItem("staryield.signup.email", "jane@example.com");
    api.login.mockResolvedValue({ user: { id: "u1", role: "client", email: "jane@example.com" }, linked: "google" });
    renderWithI18n(<LoginForm />);
    expect((await screen.findByTestId("login-notice")).textContent).toContain(
      "You already have a Staryield account with this e-mail. Sign in with your password once to connect Google.",
    );
    expect((screen.getByRole("textbox", { name: "Email Address" }) as HTMLInputElement).value).toBe("jane@example.com");
    await u.type(screen.getByLabelText("Password"), "Secret!1Aa");
    await u.click(screen.getByRole("button", { name: /^Continu(e|ing…)$/ }));
    await waitFor(() => expect(router.push).toHaveBeenCalledWith("/en/profile"));
    expect(toast.show).toHaveBeenCalledWith({ tone: "success", message: "Google is now connected." });
  });

  it("shows no toast for an ordinary password sign-in", async () => {
    const u = userEvent.setup();
    api.login.mockResolvedValue({ user: { id: "u1", role: "client", email: "jane@example.com" } });
    renderWithI18n(<LoginForm />);
    await u.type(screen.getByRole("textbox", { name: "Email Address" }), "jane@example.com");
    await u.type(screen.getByLabelText("Password"), "Secret!1Aa");
    await u.click(screen.getByRole("button", { name: /^Continu(e|ing…)$/ }));
    await waitFor(() => expect(router.push).toHaveBeenCalled());
    expect(toast.show).not.toHaveBeenCalled();
  });
});
