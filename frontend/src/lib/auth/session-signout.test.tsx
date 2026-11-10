// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render } from "@testing-library/react";

// Sign-out removes this browser's push subscription (docs §23.6): the endpoint
// goes to POST /auth/logout, the browser subscription is removed, the "last
// synced" markers are cleared, and nothing here can stop the session from ending.

const mocks = vi.hoisted(() => ({
  me: vi.fn(),
  logout: vi.fn(),
  logoutAll: vi.fn(),
  endSession: vi.fn(),
  start: vi.fn(),
  stop: vi.fn(),
  prepare: vi.fn(),
  clearMarkers: vi.fn(),
  realPrepare: { current: false },
}));

vi.mock("next/navigation", () => ({ usePathname: () => "/en" }));
vi.mock("@/i18n/i18n-provider", () => ({ useI18n: () => ({ text: (s: string) => s, locale: "en" }) }));
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => ({ show: vi.fn() }) }));
vi.mock("@/lib/analytics", () => ({ track: vi.fn() }));
vi.mock("@/lib/chat/outbox", () => ({ clearOutbox: vi.fn() }));
vi.mock("@/lib/chat/drafts", () => ({ clearDrafts: vi.fn() }));
vi.mock("@/lib/chat/attachments", () => ({ clearAttachmentCache: vi.fn() }));
vi.mock("@/lib/api/account", () => ({ pushApi: { subscribe: vi.fn(), unsubscribe: vi.fn(), config: vi.fn() } }));
vi.mock("@/lib/api/endpoints", () => ({
  meApi: { get: mocks.me },
  authApi: { logout: mocks.logout, logoutAll: mocks.logoutAll },
}));
vi.mock("@/lib/api/client", () => ({
  endSession: mocks.endSession,
  setAuthHint: vi.fn(),
  readAuthHint: () => true,
  onSessionExpired: () => () => undefined,
  onSignedOutElsewhere: () => () => undefined,
}));
vi.mock("@/lib/realtime/client", () => ({
  RealtimeClient: class {
    start = mocks.start;
    stop = mocks.stop;
    onState() {
      return () => undefined;
    }
    on() {
      return () => undefined;
    }
  },
}));
vi.mock("@/lib/push", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/push")>();
  return {
    ...actual,
    prepareSignOutPush: (...args: unknown[]) => (mocks.realPrepare.current ? actual.prepareSignOutPush(...(args as [])) : mocks.prepare(...args)),
    clearPushSyncMarkers: mocks.clearMarkers,
  };
});

import { SessionProvider, useSession } from "@/lib/auth/session-provider";

const USER = { id: "u1", role: "client", email: "a@b.c", display_name: "A", wallet: { balance: 1 } };
let signOut!: ReturnType<typeof useSession>["signOut"];
function Probe() {
  const session = useSession();
  React.useEffect(() => {
    signOut = session.signOut;
  });
  return null;
}
const flush = () => act(async () => void (await Promise.resolve()));
const assign = vi.fn();

async function mount() {
  render(
    <SessionProvider>
      <Probe />
    </SessionProvider>,
  );
  await flush();
  await flush();
}
const handle = (endpoint: string | null) => ({ endpoint, unsubscribe: vi.fn().mockResolvedValue(undefined) });

beforeEach(() => {
  Object.defineProperty(window, "location", { configurable: true, value: { ...window.location, assign } });
  assign.mockReset();
  mocks.realPrepare.current = false;
  for (const fn of [mocks.me, mocks.logout, mocks.logoutAll, mocks.endSession, mocks.start, mocks.stop, mocks.prepare, mocks.clearMarkers]) fn.mockReset();
  mocks.me.mockResolvedValue(USER);
  mocks.logout.mockResolvedValue(undefined);
  mocks.logoutAll.mockResolvedValue(undefined);
  mocks.prepare.mockResolvedValue(handle(null));
  vi.spyOn(console, "warn").mockImplementation(() => undefined);
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  Reflect.deleteProperty(navigator, "serviceWorker");
});

describe("signOut and push", () => {
  it("sends this browser's endpoint with the logout, then removes the browser subscription and the markers", async () => {
    const push = handle("https://push.example/abc");
    const order: string[] = [];
    mocks.logout.mockImplementation(async () => void order.push("logout"));
    push.unsubscribe.mockImplementation(async () => void order.push("unsubscribe"));
    mocks.prepare.mockResolvedValue(push);
    await mount();
    await act(async () => signOut());
    expect(mocks.logout).toHaveBeenCalledWith({ push_endpoint: "https://push.example/abc" });
    expect(order).toEqual(["logout", "unsubscribe"]);
    expect(mocks.clearMarkers).toHaveBeenCalledWith("u1");
    expect(mocks.endSession).toHaveBeenCalled();
    expect(assign).toHaveBeenCalledWith("/en");
  });

  it("sends no body when push is unsupported, denied or not subscribed", async () => {
    await mount();
    await act(async () => signOut());
    expect(mocks.logout).toHaveBeenCalledWith(undefined);
    expect(assign).toHaveBeenCalledWith("/en");
  });

  it("sign out everywhere also unsubscribes locally (the API drops every subscription itself)", async () => {
    const push = handle("https://push.example/abc");
    mocks.prepare.mockResolvedValue(push);
    await mount();
    await act(async () => signOut({ everywhere: true }));
    expect(mocks.logoutAll).toHaveBeenCalledTimes(1);
    expect(mocks.logout).not.toHaveBeenCalled();
    expect(push.unsubscribe).toHaveBeenCalledTimes(1);
    expect(mocks.clearMarkers).toHaveBeenCalledWith("u1");
    expect(assign).toHaveBeenCalledWith("/en");
  });

  it("still signs out when the logout request fails, and still removes the subscription", async () => {
    const push = handle("https://push.example/abc");
    mocks.prepare.mockResolvedValue(push);
    mocks.logout.mockRejectedValue(new Error("network"));
    await mount();
    await act(async () => signOut({ to: "/en/login" }));
    expect(push.unsubscribe).toHaveBeenCalledTimes(1);
    expect(mocks.endSession).toHaveBeenCalled();
    expect(assign).toHaveBeenCalledWith("/en/login");
  });

  it("still signs out when the logout request fails and push was never subscribed", async () => {
    mocks.logoutAll.mockRejectedValue(new Error("network"));
    await mount();
    await act(async () => signOut({ everywhere: true }));
    expect(mocks.clearMarkers).toHaveBeenCalledWith("u1");
    expect(assign).toHaveBeenCalledWith("/en");
  });

  it("does not hang for longer than the 3 s cap when the service worker never answers (real push module)", async () => {
    mocks.realPrepare.current = true;
    vi.stubGlobal("PushManager", class {});
    vi.stubGlobal("Notification", Object.assign(vi.fn(), { permission: "granted" }));
    Object.defineProperty(window, "isSecureContext", { configurable: true, value: true });
    Object.defineProperty(navigator, "serviceWorker", {
      configurable: true,
      value: { getRegistration: () => new Promise(() => undefined), addEventListener: () => undefined, removeEventListener: () => undefined },
    });
    await mount();
    vi.useFakeTimers();
    let signedOut = false;
    const pending = signOut().then(() => {
      signedOut = true;
    });
    await vi.advanceTimersByTimeAsync(2900);
    expect(assign).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(300);
    await pending;
    expect(signedOut).toBe(true);
    expect(mocks.logout).toHaveBeenCalledWith(undefined);
    expect(assign).toHaveBeenCalledWith("/en");
    vi.unstubAllGlobals();
  });

  it("signs out only once when called twice", async () => {
    await mount();
    await act(async () => {
      await Promise.all([signOut(), signOut()]);
    });
    expect(mocks.logout).toHaveBeenCalledTimes(1);
  });
});
