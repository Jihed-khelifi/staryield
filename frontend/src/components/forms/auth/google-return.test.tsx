// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { ApiError } from "@/lib/api/errors";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";
import { browser, rememberGoogleIntent } from "@/lib/auth/google";
import { GoogleReturn } from "@/components/forms/auth/google-return";

const router = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));
const api = vi.hoisted(() => ({
  adoptSession: vi.fn(),
  googlePending: vi.fn(),
  googleComplete: vi.fn(),
  googleStart: vi.fn(),
  me: vi.fn(),
  reactivate: vi.fn(),
}));
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
const attribution = vi.hoisted(() => ({ value: undefined as Record<string, string> | undefined }));
vi.mock("@/lib/attribution", () => ({ attributionForSignup: () => attribution.value }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: "anonymous", user: null, setUser }) }));
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => toast }));
vi.mock("@/components/layout/public-frame", () => ({ PublicFrame: ({ children }: { children: React.ReactNode }) => <main>{children}</main> }));
vi.mock("@/components/forms/radiant-panel", () => ({ RadiantPanel: ({ children }: { children: React.ReactNode }) => <div>{children}</div> }));
vi.mock("@/lib/api/account", () => ({ accountApi: { reactivate: api.reactivate } }));
vi.mock("@/lib/api/endpoints", () => ({
  authApi: {
    adoptSession: api.adoptSession,
    googlePending: api.googlePending,
    googleComplete: api.googleComplete,
    googleStart: api.googleStart,
  },
  meApi: { get: api.me },
}));

const client = { id: "u1", email: "jane@gmail.com", role: "client", profile: {} };
const advisor = { id: "u2", email: "theo@advisors.test", role: "psychic", profile: {} };
const GOOGLE_URL = "https://accounts.google.com/o/oauth2/v2/auth?state=again";
let assign: ReturnType<typeof vi.spyOn>;

function visit(query: string) {
  window.history.replaceState(null, "", `/en/auth/google${query}`);
  return renderWithI18n(<GoogleReturn />);
}
const apiError = (code: string, status = 400) => new ApiError({ code, message: code, status });

beforeAll(stubResizeObserver);
beforeEach(() => {
  sessionStorage.clear();
  api.me.mockResolvedValue(client);
  assign = vi.spyOn(browser, "assign").mockImplementation(() => {});
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.restoreAllMocks();
});

describe("GoogleReturn: URL handling", () => {
  it("strips result and next from the address bar", async () => {
    visit("?result=signed_in&next=%2Fchatroom");
    await waitFor(() => expect(router.replace).toHaveBeenCalled());
    expect(window.location.search).toBe("");
    expect(window.location.pathname).toBe("/en/auth/google");
  });

  it("shows a busy status while the session loads", async () => {
    let resolve!: (v: unknown) => void;
    api.me.mockReturnValue(new Promise((r) => (resolve = r)));
    visit("?result=signed_in");
    const card = await screen.findByTestId("google-return");
    await waitFor(() => expect(card.getAttribute("data-phase")).toBe("signing_in"));
    expect(card.getAttribute("aria-busy")).toBe("true");
    expect(screen.getByRole("heading", { name: "Signing you in…" })).toBeTruthy();
    resolve(client);
    await waitFor(() => expect(router.replace).toHaveBeenCalled());
  });
});

describe("GoogleReturn: signed in / up / linked", () => {
  it("signed_in adopts the session, loads /me and goes to the default landing page", async () => {
    visit("?result=signed_in");
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/profile"));
    expect(api.adoptSession).toHaveBeenCalled();
    expect(setUser).toHaveBeenCalledWith(client);
  });

  it("signed_in follows a safe next (locale-less or prefixed) and ignores an unsafe one", async () => {
    visit("?result=signed_in&next=%2Fpsychics%2Ftheo");
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/psychics/theo"));
    cleanup();
    router.replace.mockClear();
    visit("?result=signed_in&next=%2Fen%2Fchatroom");
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/chatroom"));
    cleanup();
    router.replace.mockClear();
    visit("?result=signed_in&next=https%3A%2F%2Fevil.example");
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/profile"));
  });

  // revisions 5.0: 18+ age gate (docs §24.6).
  it("signed_in without a birth date goes to the birth step and then where it was going", async () => {
    api.me.mockResolvedValue({ ...client, age_status: "birth_date_required" });
    visit("?result=signed_in&next=%2Fen%2Fchatroom");
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith(`/en/signup/birth?next=${encodeURIComponent("/en/chatroom")}`));
  });

  it("signed_in of a restricted account goes to the restricted screen", async () => {
    api.me.mockResolvedValue({ ...client, age_status: "restricted" });
    visit("?result=signed_in");
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/age-restricted"));
  });

  it("signed_in of an adult is unchanged", async () => {
    api.me.mockResolvedValue({ ...client, age_status: "adult" });
    visit("?result=signed_in");
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/profile"));
  });

  it("sends advisors to the chat room", async () => {
    api.me.mockResolvedValue(advisor);
    visit("?result=signed_in");
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/chatroom"));
  });

  it("signed_up continues at the birth step", async () => {
    visit("?result=signed_up&next=%2Fprofile");
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/signup/birth"));
    expect(setUser).toHaveBeenCalledWith(client);
  });

  it("linked goes to the settings with a success toast", async () => {
    visit("?result=linked");
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/profile/settings#security"));
    expect(toast.show).toHaveBeenCalledWith({ tone: "success", message: "Google is now connected." });
  });

  it("linked returns to the page it started from", async () => {
    visit("?result=linked&next=%2Fprofile%2Fsettings");
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/profile/settings"));
  });

  it("retries loading the session after a network failure", async () => {
    const u = userEvent.setup();
    api.me.mockRejectedValueOnce(apiError("network_error", 0)).mockResolvedValueOnce(client);
    visit("?result=signed_in");
    const retry = await screen.findByRole("button", { name: "Try again" });
    expect(router.replace).not.toHaveBeenCalled();
    expect(await axeViolations()).toEqual([]);
    await u.click(retry);
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/profile"));
    expect(api.adoptSession).toHaveBeenCalledTimes(2);
  });

  it("treats a missing session as an expired attempt", async () => {
    api.me.mockRejectedValue(apiError("unauthenticated", 401));
    visit("?result=signed_in");
    const card = await screen.findByTestId("google-return");
    await waitFor(() => expect(card.getAttribute("data-code")).toBe("google_state_invalid"));
    expect(router.replace).not.toHaveBeenCalled();
  });
});

describe("GoogleReturn: signup_consent", () => {
  beforeEach(() => api.googlePending.mockResolvedValue({ kind: "signup_consent", email: "jane@gmail.com" }));

  it("asks for the terms, then creates the account and continues at the birth step", async () => {
    const u = userEvent.setup();
    api.googleComplete.mockResolvedValue({ user: client });
    visit("?result=signup_consent");
    expect(await screen.findByRole("heading", { name: "Create your Staryield account" })).toBeTruthy();
    expect(screen.getByTestId("google-consent-email").textContent).toContain("jane@gmail.com");
    const hrefs = screen.getAllByRole("link").map((a) => a.getAttribute("href"));
    expect(hrefs).toContain("/en/terms");
    expect(hrefs).toContain("/en/privacy");
    expect(await axeViolations()).toEqual([]);

    await u.click(screen.getByRole("button", { name: "Create account" }));
    expect(await screen.findByText("Please accept the Terms of Service and Privacy Policy.")).toBeTruthy();
    expect(api.googleComplete).not.toHaveBeenCalled();

    await u.click(screen.getByRole("checkbox"));
    await u.click(screen.getByRole("button", { name: "Create account" }));
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/signup/birth"));
    expect(api.googleComplete).toHaveBeenCalledWith({ accept_terms: true, locale: "en", attribution: undefined });
    expect(setUser).toHaveBeenCalledWith(client);
  });

  it("sends the first-touch attribution, and retries without it when the API rejects it", async () => {
    const u = userEvent.setup();
    attribution.value = { utm_source: "news" };
    api.googleComplete
      .mockRejectedValueOnce(new ApiError({ code: "validation_failed", message: "x", status: 422, details: { fields: { "attribution.utm_source": "bad" } } }))
      .mockResolvedValueOnce({ user: client });
    visit("?result=signup_consent");
    await u.click(await screen.findByRole("checkbox"));
    await u.click(screen.getByRole("button", { name: "Create account" }));
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/signup/birth"));
    expect(api.googleComplete).toHaveBeenNthCalledWith(1, { accept_terms: true, locale: "en", attribution: { utm_source: "news" } });
    expect(api.googleComplete).toHaveBeenNthCalledWith(2, { accept_terms: true, locale: "en", attribution: undefined });
    attribution.value = undefined;
  });

  it("explains an expired attempt when nothing is pending any more", async () => {
    const u = userEvent.setup();
    api.googleComplete.mockRejectedValue(apiError("google_state_invalid"));
    visit("?result=signup_consent");
    await u.click(await screen.findByRole("checkbox"));
    await u.click(screen.getByRole("button", { name: "Create account" }));
    const card = await screen.findByTestId("google-return");
    expect(card.getAttribute("data-code")).toBe("google_state_invalid");
    expect(card.textContent).toContain("That sign-in attempt expired. Please start again.");
  });

  it("points to Log In when the address was registered meanwhile", async () => {
    const u = userEvent.setup();
    api.googleComplete.mockRejectedValue(apiError("email_taken", 409));
    visit("?result=signup_consent");
    await u.click(await screen.findByRole("checkbox"));
    await u.click(screen.getByRole("button", { name: "Create account" }));
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("An account with this email already exists.");
    expect(within(alert).getByRole("link", { name: "Log In" }).getAttribute("href")).toBe("/en/login");
  });
});

describe("GoogleReturn: link_password", () => {
  it("hands over to /login with the notice and the e-mail prefilled", async () => {
    api.googlePending.mockResolvedValue({ kind: "link_password", email: "jane@gmail.com" });
    visit("?result=link_password");
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/login?notice=google_link"));
    expect(sessionStorage.getItem("staryield.signup.email")).toBe("jane@gmail.com");
  });

  it("keeps the destination", async () => {
    api.googlePending.mockResolvedValue({ kind: "link_password", email: "jane@gmail.com" });
    visit("?result=link_password&next=%2Fpsychics");
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/login?notice=google_link&next=%2Fen%2Fpsychics"));
  });
});

describe("GoogleReturn: reactivation", () => {
  it("offers to reactivate with the existing offer component and signs in", async () => {
    const u = userEvent.setup();
    api.googlePending.mockResolvedValue({ kind: "reactivation", code: "account_closure_pending", reactivation_token: "tok", deletion_scheduled_at: "2026-12-01T00:00:00Z" });
    api.reactivate.mockResolvedValue({ user: client });
    visit("?result=reactivation");
    const offer = await screen.findByTestId("reactivation-offer");
    expect(offer.getAttribute("data-kind")).toBe("account_closure_pending");
    expect(screen.getByRole("heading", { name: "Welcome back" })).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
    await u.click(screen.getByTestId("reactivate"));
    await waitFor(() => expect(api.reactivate).toHaveBeenCalledWith("tok"));
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/en/profile"));
    expect(setUser).toHaveBeenCalledWith(client);
  });

  it("offers the same for a disabled account", async () => {
    api.googlePending.mockResolvedValue({ kind: "reactivation", code: "account_deactivated", reactivation_token: "tok" });
    visit("?result=reactivation");
    expect((await screen.findByTestId("reactivation-offer")).getAttribute("data-kind")).toBe("account_deactivated");
    expect(screen.getByRole("link", { name: "Back to sign in" }).getAttribute("href")).toBe("/en/login");
  });
});

describe("GoogleReturn: pending lookups", () => {
  it("treats a 404 as an expired attempt", async () => {
    api.googlePending.mockRejectedValue(apiError("not_found", 404));
    visit("?result=signup_consent");
    const card = await screen.findByTestId("google-return");
    await waitFor(() => expect(card.getAttribute("data-code")).toBe("google_state_invalid"));
    expect(screen.getByRole("button", { name: "Try again" })).toBeTruthy();
  });

  it("offers a retry for other failures", async () => {
    const u = userEvent.setup();
    api.googlePending.mockRejectedValueOnce(apiError("network_error", 0)).mockResolvedValueOnce({ kind: "signup_consent", email: "jane@gmail.com" });
    visit("?result=signup_consent");
    await u.click(await screen.findByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("heading", { name: "Create your Staryield account" })).toBeTruthy();
    expect(api.googlePending).toHaveBeenCalledTimes(2);
  });
});

describe("GoogleReturn: errors", () => {
  const cases: Array<[string, string, { retry: boolean; support?: boolean }]> = [
    ["google_cancelled", "Google sign-in was cancelled. Nothing was changed.", { retry: true }],
    ["google_state_invalid", "That sign-in attempt expired. Please start again.", { retry: true }],
    ["google_unavailable", "Google sign-in isn't available right now.", { retry: true }],
    ["google_email_unverified", "Google hasn't verified this email address yet.", { retry: false }],
    ["google_account_in_use", "That Google account is already connected to another Staryield account.", { retry: false }],
    ["google_not_allowed", "Google sign-in isn't available for this account.", { retry: false }],
    ["account_disabled", "This account has been deactivated. Please contact support.", { retry: false, support: true }],
  ];

  it.each(cases)("%s shows its copy and the right actions", async (code, copy, expected) => {
    visit(`?result=${code}`);
    const card = await screen.findByTestId("google-return");
    await waitFor(() => expect(card.getAttribute("data-code")).toBe(code));
    expect(card.textContent).toContain(copy);
    expect(Boolean(screen.queryByRole("button", { name: "Try again" }))).toBe(expected.retry);
    expect(Boolean(screen.queryByRole("link", { name: "Contact support" }))).toBe(Boolean(expected.support));
    expect(screen.getByRole("link", { name: "Back to sign in" }).getAttribute("href")).toBe("/en/login");
    expect(await axeViolations()).toEqual([]);
  });

  it("Try again restarts the same sign-up", async () => {
    const u = userEvent.setup();
    rememberGoogleIntent("signup", "/psychics");
    api.googleStart.mockResolvedValue({ authorization_url: GOOGLE_URL });
    visit("?result=google_cancelled");
    await u.click(await screen.findByRole("button", { name: "Try again" }));
    await waitFor(() => expect(assign).toHaveBeenCalledWith(GOOGLE_URL));
    expect(api.googleStart).toHaveBeenCalledWith({ intent: "signup", accept_terms: true, next: "/psychics", locale: "en", attribution: undefined });
  });

  it("Try again signs in when the intent is unknown", async () => {
    const u = userEvent.setup();
    api.googleStart.mockResolvedValue({ authorization_url: GOOGLE_URL });
    visit("?result=google_unavailable");
    await u.click(await screen.findByRole("button", { name: "Try again" }));
    await waitFor(() => expect(api.googleStart).toHaveBeenCalledWith(expect.objectContaining({ intent: "signin" })));
  });

  it("shows why a retry failed and stays on the panel", async () => {
    const u = userEvent.setup();
    api.googleStart.mockRejectedValue(apiError("google_unavailable", 503));
    visit("?result=google_unavailable");
    await u.click(await screen.findByRole("button", { name: "Try again" }));
    const alerts = await screen.findAllByRole("alert");
    expect(alerts.some((a) => a.textContent?.includes("Google sign-in isn't available right now."))).toBe(true);
    expect(assign).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Try again" }).hasAttribute("disabled")).toBe(false);
  });

  it("goes back to the settings when a connect attempt failed", async () => {
    rememberGoogleIntent("link", "/profile/settings");
    visit("?result=google_account_in_use");
    const back = await screen.findByRole("link", { name: "Back to settings" });
    expect(back.getAttribute("href")).toBe("/en/profile/settings#security");
  });

  it("Contact support links to the contact page", async () => {
    visit("?result=account_disabled");
    expect((await screen.findByRole("link", { name: "Contact support" })).getAttribute("href")).toBe("/en/contact");
  });

  it.each(["?result=nonsense", "?next=%2Fprofile", ""])("shows a generic error with a way back for %s", async (query) => {
    visit(query);
    const card = await screen.findByTestId("google-return");
    await waitFor(() => expect(card.getAttribute("data-code")).toBe("unknown"));
    expect(card.textContent).toContain("We couldn't complete your Google sign-in. Please sign in again.");
    expect(screen.queryByRole("button", { name: "Try again" })).toBeNull();
    expect(screen.getByRole("link", { name: "Back to sign in" }).getAttribute("href")).toBe("/en/login");
    expect(api.adoptSession).not.toHaveBeenCalled();
  });
});
