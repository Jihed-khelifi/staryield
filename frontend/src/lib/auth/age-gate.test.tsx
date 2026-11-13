// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, renderHook, screen } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import { notifyAgeBlocked, reportAgeBlock } from "@/lib/api/age-events";
import { AgeGateEffects, AgeGuard, useAgeGuard, useRequireAdult } from "@/lib/auth/age-gate";

// revisions 5.0: the age guard (docs §24.6): one rule for gated pages, gated
// actions and gated API answers; public pages never use it.

const mocks = vi.hoisted(() => ({
  router: { replace: vi.fn(), push: vi.fn() },
  pathname: { value: "/en/chatroom" },
  session: { current: {} as Record<string, unknown> },
  reload: vi.fn(async () => null),
}));

vi.mock("next/navigation", () => ({ useRouter: () => mocks.router, usePathname: () => mocks.pathname.value }));
vi.mock("@/i18n/i18n-provider", () => ({ useI18n: () => ({ text: (s: string) => s, locale: "en" }) }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => mocks.session.current }));

function signedIn(age: string | undefined, role = "client") {
  mocks.session.current = { status: "authenticated", user: { id: "u1", role, age_status: age }, reload: mocks.reload };
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.pathname.value = "/en/chatroom";
  window.history.replaceState(null, "", "/en/chatroom?reader=theo");
  signedIn("adult");
});
afterEach(cleanup);

const birthHref = `/en/signup/birth?next=${encodeURIComponent("/en/chatroom?reader=theo")}`;

describe("AgeGuard (a gated page)", () => {
  it("shows the page to an adult", () => {
    render(<AgeGuard><p>chat</p></AgeGuard>);
    expect(screen.getByText("chat")).toBeTruthy();
    expect(mocks.router.replace).not.toHaveBeenCalled();
  });

  it("shows nothing and sends an account without a birth date to the birth step, returning here afterwards", () => {
    signedIn("birth_date_required");
    render(<AgeGuard><p>chat</p></AgeGuard>);
    expect(screen.queryByText("chat")).toBeNull();
    expect(mocks.router.replace).toHaveBeenCalledWith(birthHref);
  });

  it("shows nothing and sends a restricted account to the restricted screen", () => {
    signedIn("restricted");
    render(<AgeGuard><p>chat</p></AgeGuard>);
    expect(screen.queryByText("chat")).toBeNull();
    expect(mocks.router.replace).toHaveBeenCalledWith("/en/age-restricted");
  });

  it("holds the page back while the session loads (no gated call is made yet)", () => {
    mocks.session.current = { status: "loading", user: null };
    render(<AgeGuard><p>chat</p></AgeGuard>);
    expect(screen.queryByText("chat")).toBeNull();
    expect(mocks.router.replace).not.toHaveBeenCalled();
  });

  it("leaves anonymous visitors to the page's own sign-in redirect", () => {
    mocks.session.current = { status: "anonymous", user: null };
    render(<AgeGuard><p>chat</p></AgeGuard>);
    expect(screen.getByText("chat")).toBeTruthy();
    expect(mocks.router.replace).not.toHaveBeenCalled();
  });

  it("treats a user from an API without the gate as an adult", () => {
    signedIn(undefined);
    render(<AgeGuard><p>chat</p></AgeGuard>);
    expect(screen.getByText("chat")).toBeTruthy();
  });

  it("reports the state", () => {
    signedIn("restricted");
    const { result } = renderHook(() => useAgeGuard());
    expect(result.current).toEqual({ state: "restricted", allowed: false });
  });
});

describe("useRequireAdult (a gated action)", () => {
  it("lets an adult, an advisor and an anonymous visitor through without any redirect", () => {
    const seen = vi.fn();
    const off = import("@/lib/api/age-events").then((m) => m.onAgeBlocked(seen));
    for (const setup of [() => signedIn("adult"), () => signedIn("adult", "psychic"), () => (mocks.session.current = { status: "anonymous", user: null })]) {
      setup();
      const { result } = renderHook(() => useRequireAdult());
      expect(result.current()).toBe(true);
    }
    expect(seen).not.toHaveBeenCalled();
    void off.then((fn) => fn());
  });

  it.each([
    ["birth_date_required", "birth_date_required"],
    ["restricted", "restricted"],
  ])("stops the action of a %s account and reports the block", async (age, block) => {
    const { onAgeBlocked } = await import("@/lib/api/age-events");
    const seen = vi.fn();
    const off = onAgeBlocked(seen);
    signedIn(age);
    const { result } = renderHook(() => useRequireAdult());
    expect(result.current()).toBe(false);
    expect(seen).toHaveBeenCalledWith(block);
    off();
  });
});

describe("AgeGateEffects (a gated API answer)", () => {
  // Each test runs more than 1.5 s apart from the debounce of the last one.
  beforeEach(() => vi.useFakeTimers({ toFake: ["Date"] }));
  afterEach(() => vi.useRealTimers());
  const next = () => vi.setSystemTime(Date.now() + 5000);

  it("turns a 403 birth_date_required into the birth step, here as `next`, and re-reads the account", () => {
    render(<AgeGateEffects />);
    act(() => notifyAgeBlocked(new ApiError({ code: "birth_date_required", message: "x", status: 403 })));
    expect(mocks.router.replace).toHaveBeenCalledWith(birthHref);
    expect(mocks.reload).toHaveBeenCalled();
  });

  it("turns a 403 age_restricted into the restricted screen", () => {
    render(<AgeGateEffects />);
    next();
    act(() => notifyAgeBlocked(new ApiError({ code: "age_restricted", message: "x", status: 403 })));
    expect(mocks.router.replace).toHaveBeenCalledWith("/en/age-restricted");
  });

  it("does not redirect from the birth step to itself, nor from the restricted screen to itself", () => {
    next();
    mocks.pathname.value = "/en/signup/birth";
    const { rerender } = render(<AgeGateEffects />);
    act(() => reportAgeBlock("birth_date_required"));
    expect(mocks.router.replace).not.toHaveBeenCalled();
    mocks.pathname.value = "/en/age-restricted";
    rerender(<AgeGateEffects />);
    act(() => reportAgeBlock("restricted"));
    expect(mocks.router.replace).not.toHaveBeenCalled();
    // The birth step still sends a restricted account to the screen.
    mocks.pathname.value = "/en/signup/birth";
    rerender(<AgeGateEffects />);
    act(() => reportAgeBlock("restricted"));
    expect(mocks.router.replace).toHaveBeenCalledWith("/en/age-restricted");
  });

  it("redirects once when several calls are refused together", () => {
    render(<AgeGateEffects />);
    next();
    act(() => {
      for (let i = 0; i < 4; i++) notifyAgeBlocked(new ApiError({ code: "birth_date_required", message: "x", status: 403 }));
    });
    expect(mocks.router.replace).toHaveBeenCalledTimes(1);
  });

  it("ignores every other error", () => {
    render(<AgeGateEffects />);
    next();
    act(() => notifyAgeBlocked(new ApiError({ code: "forbidden", message: "x", status: 403 })));
    expect(mocks.router.replace).not.toHaveBeenCalled();
  });
});
