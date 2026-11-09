// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";

// Session provider rules of docs/platform-extensions.md §22.1: only a
// definitive 401 signs out; auth.revoked is deduplicated; a sign-out in
// another tab is followed.

const mocks = vi.hoisted(() => {
  const handlers = new Map<string, Set<(e: { type: string; data?: unknown }) => void>>();
  return {
    me: vi.fn(),
    toast: vi.fn(),
    handlers,
    signedOut: new Set<() => void>(),
    expired: new Set<() => void>(),
    hint: { value: true },
    endSession: vi.fn(),
    setAuthHint: vi.fn(),
    start: vi.fn(),
    stop: vi.fn(),
  };
});

vi.mock("next/navigation", () => ({ usePathname: () => "/en" }));
vi.mock("@/i18n/i18n-provider", () => ({ useI18n: () => ({ text: (s: string) => s, locale: "en" }) }));
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => ({ show: mocks.toast }) }));
vi.mock("@/lib/analytics", () => ({ track: vi.fn() }));
vi.mock("@/lib/chat/outbox", () => ({ clearOutbox: vi.fn() }));
vi.mock("@/lib/chat/drafts", () => ({ clearDrafts: vi.fn() }));
vi.mock("@/lib/chat/attachments", () => ({ clearAttachmentCache: vi.fn() }));
vi.mock("@/lib/api/endpoints", () => ({
  meApi: { get: mocks.me },
  authApi: { logout: vi.fn(async () => undefined), logoutAll: vi.fn(async () => undefined) },
}));
vi.mock("@/lib/api/client", () => ({
  endSession: mocks.endSession,
  setAuthHint: mocks.setAuthHint,
  readAuthHint: () => mocks.hint.value,
  onSessionExpired: (fn: () => void) => {
    mocks.expired.add(fn);
    return () => mocks.expired.delete(fn);
  },
  onSignedOutElsewhere: (fn: () => void) => {
    mocks.signedOut.add(fn);
    return () => mocks.signedOut.delete(fn);
  },
}));
vi.mock("@/lib/realtime/client", () => ({
  RealtimeClient: class {
    start = mocks.start;
    stop = mocks.stop;
    onState() {
      return () => undefined;
    }
    on(type: string, fn: (e: { type: string; data?: unknown }) => void) {
      if (!mocks.handlers.has(type)) mocks.handlers.set(type, new Set());
      mocks.handlers.get(type)!.add(fn);
      return () => mocks.handlers.get(type)!.delete(fn);
    }
  },
}));

import { SessionProvider, useSession } from "@/lib/auth/session-provider";

const USER = { id: "u1", role: "client", email: "a@b.c", display_name: "A", wallet: { balance: 1 } };
const err = (code: string, status: number) => new ApiError({ code, message: code, status });

function Probe() {
  const { status, user } = useSession();
  return (
    <p data-testid="state">
      {status}:{user?.id ?? "-"}
    </p>
  );
}
const state = () => screen.getByTestId("state").textContent;
const mount = () =>
  render(
    <SessionProvider>
      <Probe />
    </SessionProvider>,
  );
const emitRevoked = () => mocks.handlers.get("auth.revoked")?.forEach((fn) => fn({ type: "auth.revoked" }));
const flush = () => act(async () => void (await Promise.resolve()));

beforeEach(() => {
  vi.useFakeTimers();
  vi.spyOn(console, "warn").mockImplementation(() => undefined);
  mocks.handlers.clear();
  mocks.signedOut.clear();
  mocks.expired.clear();
  mocks.hint.value = true;
  for (const fn of [mocks.me, mocks.toast, mocks.endSession, mocks.setAuthHint, mocks.start, mocks.stop]) fn.mockReset();
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("initial load", () => {
  it("signs in when /me answers", async () => {
    mocks.me.mockResolvedValue(USER);
    mount();
    await flush();
    expect(state()).toBe("authenticated:u1");
    expect(mocks.setAuthHint).toHaveBeenCalledWith(true);
  });

  it("goes anonymous on a definitive 401", async () => {
    mocks.me.mockRejectedValue(err("session_expired", 401));
    mount();
    await flush();
    expect(state()).toBe("anonymous:-");
    expect(mocks.setAuthHint).toHaveBeenCalledWith(false);
  });

  it("keeps loading, keeps the hint and retries with backoff after a transient failure", async () => {
    mocks.me.mockRejectedValueOnce(err("service_unavailable", 503)).mockRejectedValueOnce(err("network_error", 0)).mockResolvedValue(USER);
    mount();
    await flush();
    expect(state()).toBe("loading:-");
    expect(mocks.setAuthHint).not.toHaveBeenCalledWith(false);
    await act(async () => void (await vi.advanceTimersByTimeAsync(2_100)));
    expect(mocks.me).toHaveBeenCalledTimes(2);
    expect(state()).toBe("loading:-");
    await act(async () => void (await vi.advanceTimersByTimeAsync(4_100)));
    expect(state()).toBe("authenticated:u1");
    expect(mocks.setAuthHint).not.toHaveBeenCalledWith(false);
    expect(mocks.endSession).not.toHaveBeenCalled();
  });

  it("does not retry forever for a visitor who never had a session", async () => {
    mocks.hint.value = false;
    mocks.me.mockRejectedValue(err("network_error", 0));
    mount();
    await flush();
    expect(state()).toBe("anonymous:-");
    await act(async () => void (await vi.advanceTimersByTimeAsync(60_000)));
    expect(mocks.me).toHaveBeenCalledTimes(1);
  });
});

describe("auth.revoked", () => {
  async function signedIn() {
    mocks.me.mockResolvedValue(USER);
    mount();
    await flush();
    mocks.me.mockClear();
  }

  it("deduplicates the frame and the 4001 close of one revocation (2 s)", async () => {
    await signedIn();
    emitRevoked();
    emitRevoked(); // the 4001 close right after the frame
    await flush();
    expect(mocks.me).toHaveBeenCalledTimes(1);
    expect(state()).toBe("authenticated:u1");
    await act(async () => void (await vi.advanceTimersByTimeAsync(2_100)));
    emitRevoked();
    await flush();
    expect(mocks.me).toHaveBeenCalledTimes(2);
  });

  it("does not count one revocation twice toward the 3-per-minute rule", async () => {
    await signedIn();
    for (let i = 0; i < 3; i++) {
      emitRevoked();
      emitRevoked();
      await flush();
      await act(async () => void (await vi.advanceTimersByTimeAsync(5_000)));
    }
    expect(state()).toBe("authenticated:u1");
    expect(mocks.endSession).not.toHaveBeenCalled();
  });

  it("signs out after more than 3 distinct revocations in a minute", async () => {
    await signedIn();
    for (let i = 0; i < 4; i++) {
      emitRevoked();
      await flush();
      await act(async () => void (await vi.advanceTimersByTimeAsync(3_000)));
    }
    expect(state()).toBe("anonymous:-");
    expect(mocks.endSession).toHaveBeenCalled();
  });

  it("keeps the session on a transient /me failure and retries with backoff", async () => {
    await signedIn();
    mocks.me.mockRejectedValueOnce(err("service_unavailable", 503)).mockResolvedValue(USER);
    emitRevoked();
    await flush();
    expect(state()).toBe("authenticated:u1");
    expect(mocks.endSession).not.toHaveBeenCalled();
    expect(mocks.toast).not.toHaveBeenCalled();
    await act(async () => void (await vi.advanceTimersByTimeAsync(2_100)));
    expect(mocks.me).toHaveBeenCalledTimes(2);
    expect(mocks.start).toHaveBeenCalled(); // reconnected after the re-read succeeded
    expect(state()).toBe("authenticated:u1");
  });

  it("signs out on a definitive 401", async () => {
    await signedIn();
    mocks.me.mockRejectedValue(err("unauthenticated", 401));
    emitRevoked();
    await flush();
    expect(state()).toBe("anonymous:-");
    expect(mocks.endSession).toHaveBeenCalled();
    expect(mocks.toast).not.toHaveBeenCalled(); // the failed refresh already announced the expiry
  });

  it("explains a deactivated account", async () => {
    await signedIn();
    mocks.me.mockRejectedValue(err("account_disabled", 403));
    emitRevoked();
    await flush();
    expect(state()).toBe("anonymous:-");
    expect(mocks.toast).toHaveBeenCalledWith(expect.objectContaining({ message: "You were signed out. Please sign in again." }));
  });
});

describe("other tabs and expiry", () => {
  it("clears local state when another tab signs out", async () => {
    mocks.me.mockResolvedValue(USER);
    mount();
    await flush();
    act(() => mocks.signedOut.forEach((fn) => fn()));
    expect(state()).toBe("anonymous:-");
  });

  it("announces an expiry once for a signed-in user", async () => {
    mocks.me.mockResolvedValue(USER);
    mount();
    await flush();
    act(() => mocks.expired.forEach((fn) => fn()));
    expect(state()).toBe("anonymous:-");
    expect(mocks.toast).toHaveBeenCalledWith(expect.objectContaining({ message: "Your session has expired. Please sign in again." }));
  });
});
