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

import { PushSync } from "@/components/notifications/push-sync";

beforeEach(() => {
  m.syncPush.mockReset().mockResolvedValue("idle");
  m.settingsApi.get.mockReset().mockResolvedValue({ notifications: { push: true } });
  m.pushApi.config.mockReset().mockResolvedValue({ enabled: true, public_key: "KEY" });
});
afterEach(cleanup);

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
});
