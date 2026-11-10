// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, waitFor } from "@testing-library/react";

const m = vi.hoisted(() => ({
  session: { current: { status: "authenticated", user: { id: "u1", role: "client" } } as { status: string; user: { id: string; role: string } | null } },
  settingsApi: { get: vi.fn() },
  pushApi: { config: vi.fn() },
  syncPush: vi.fn(),
}));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => m.session.current }));
vi.mock("@/lib/api/account", () => ({ settingsApi: m.settingsApi, pushApi: m.pushApi }));
vi.mock("@/lib/push", () => ({ syncPush: m.syncPush }));

import { PushSync, SUBSCRIPTION_CHANGED_MESSAGE } from "@/components/notifications/push-sync";

beforeEach(() => {
  m.syncPush.mockReset().mockResolvedValue("idle");
  m.settingsApi.get.mockReset().mockResolvedValue({ notifications: { push: true } });
  m.pushApi.config.mockReset().mockResolvedValue({ enabled: true, public_key: "KEY" });
});
afterEach(() => {
  cleanup();
  Reflect.deleteProperty(navigator, "serviceWorker");
});

/** A service worker container that records its message listeners. */
function fakeServiceWorker() {
  const listeners = new Set<(e: MessageEvent) => void>();
  Object.defineProperty(navigator, "serviceWorker", {
    configurable: true,
    value: {
      addEventListener: (_type: string, fn: (e: MessageEvent) => void) => listeners.add(fn),
      removeEventListener: (_type: string, fn: (e: MessageEvent) => void) => listeners.delete(fn),
    },
  });
  return { listeners, post: (data: unknown) => listeners.forEach((fn) => fn({ data } as MessageEvent)) };
}

describe("PushSync", () => {
  it("syncs once for a signed-in client, with lazy switch and key readers", async () => {
    const view = render(<PushSync />);
    await waitFor(() => expect(m.syncPush).toHaveBeenCalledTimes(1));
    view.rerender(<PushSync />);
    expect(m.syncPush).toHaveBeenCalledTimes(1);
    const opts = m.syncPush.mock.calls[0][0];
    expect(opts.userId).toBe("u1");
    expect(m.settingsApi.get).not.toHaveBeenCalled();
    expect(await opts.pushSwitchOn()).toBe(true);
    expect(await opts.publicKey()).toBe("KEY");
  });
  it("also runs for advisors", async () => {
    m.session.current = { status: "authenticated", user: { id: "p1", role: "psychic" } };
    render(<PushSync />);
    await waitFor(() => expect(m.syncPush).toHaveBeenCalledWith(expect.objectContaining({ userId: "p1" })));
  });
  it("returns no key when push is disabled on the server", async () => {
    m.session.current = { status: "authenticated", user: { id: "u1", role: "client" } };
    m.pushApi.config.mockResolvedValue({ enabled: false });
    render(<PushSync />);
    await waitFor(() => expect(m.syncPush).toHaveBeenCalled());
    expect(await m.syncPush.mock.calls[0][0].publicKey()).toBeNull();
  });
  it("does nothing for visitors", () => {
    m.session.current = { status: "anonymous", user: null };
    render(<PushSync />);
    expect(m.syncPush).not.toHaveBeenCalled();
  });
  it("swallows sync failures", async () => {
    m.session.current = { status: "authenticated", user: { id: "u1", role: "client" } };
    m.syncPush.mockRejectedValue(new Error("x"));
    render(<PushSync />);
    await waitFor(() => expect(m.syncPush).toHaveBeenCalled());
  });
  it("re-syncs at once, bypassing the throttles, when the service worker reports a changed subscription", async () => {
    m.session.current = { status: "authenticated", user: { id: "u1", role: "client" } };
    const sw = fakeServiceWorker();
    render(<PushSync />);
    await waitFor(() => expect(m.syncPush).toHaveBeenCalledTimes(1));
    expect(m.syncPush.mock.calls[0][0].force).toBe(false);
    expect(sw.listeners.size).toBe(1);
    sw.post({ type: SUBSCRIPTION_CHANGED_MESSAGE });
    await waitFor(() => expect(m.syncPush).toHaveBeenCalledTimes(2));
    expect(m.syncPush.mock.calls[1][0]).toMatchObject({ userId: "u1", force: true });
  });

  it("ignores other messages, and stops listening when it unmounts or the user signs out", async () => {
    m.session.current = { status: "authenticated", user: { id: "u1", role: "client" } };
    const sw = fakeServiceWorker();
    const view = render(<PushSync />);
    await waitFor(() => expect(m.syncPush).toHaveBeenCalledTimes(1));
    sw.post({ type: "notification.opened", id: "n1" });
    sw.post(null);
    expect(m.syncPush).toHaveBeenCalledTimes(1);
    view.unmount();
    expect(sw.listeners.size).toBe(0);
    m.session.current = { status: "anonymous", user: null };
    const anon = fakeServiceWorker();
    render(<PushSync />);
    expect(anon.listeners.size).toBe(0);
  });

  it("works in browsers without a service worker container", async () => {
    m.session.current = { status: "authenticated", user: { id: "u1", role: "client" } };
    render(<PushSync />);
    await waitFor(() => expect(m.syncPush).toHaveBeenCalledTimes(1));
  });
});
