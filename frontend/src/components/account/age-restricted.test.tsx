// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen } from "@testing-library/react";
import type * as React from "react";

import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";
import { AgeRestricted } from "@/components/account/age-restricted";

// revisions 5.0: the calm "Account restricted" screen (docs §24.6).
const router = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));
const session = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));

vi.mock("next/navigation", () => ({ useRouter: () => router }));
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
vi.mock("@/components/forms/radiant-panel", () => ({
  RadiantPanel: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

beforeAll(stubResizeObserver);
beforeEach(() => {
  vi.clearAllMocks();
  session.current = { status: "authenticated", user: { id: "u1", role: "client", age_status: "restricted" } };
});
afterEach(cleanup);

describe("AgeRestricted", () => {
  it("explains the restriction calmly and offers support, closing the account and the privacy policy", async () => {
    renderWithI18n(<AgeRestricted />);
    expect(screen.getByRole("heading", { name: "Account restricted" })).toBeTruthy();
    expect(screen.getByText(/Staryield is for adults 18 and over/)).toBeTruthy();
    expect(screen.getByText(/If you entered the wrong date of birth, contact our support team/)).toBeTruthy();
    expect(screen.getByTestId("age-restricted-support").getAttribute("href")).toBe("/en/contact");
    expect(screen.getByTestId("age-restricted-close").getAttribute("href")).toBe("/en/profile/settings#account-actions");
    expect(screen.getByTestId("age-restricted-privacy").getAttribute("href")).toBe("/en/privacy");
    expect(router.replace).not.toHaveBeenCalled();
    expect(await axeViolations()).toEqual([]);
  });

  it("offers no way to enter another date", () => {
    renderWithI18n(<AgeRestricted />);
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByRole("link", { name: /date of birth/i })).toBeNull();
  });

  it("renders nothing while the session loads", () => {
    session.current = { status: "loading", user: null };
    renderWithI18n(<AgeRestricted />);
    expect(screen.queryByTestId("age-restricted")).toBeNull();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it.each([
    ["an adult", { status: "authenticated", user: { id: "u1", age_status: "adult" } }, "/en/profile"],
    ["an account without a birth date", { status: "authenticated", user: { id: "u1", age_status: "birth_date_required" } }, "/en/signup/birth"],
    ["a visitor", { status: "anonymous", user: null }, `/en/login?next=${encodeURIComponent("/en/age-restricted")}`],
  ])("never contradicts the account: sends %s on", (_label, state, target) => {
    session.current = state;
    renderWithI18n(<AgeRestricted />);
    expect(screen.queryByTestId("age-restricted")).toBeNull();
    expect(router.replace).toHaveBeenCalledWith(target);
  });
});
