// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";

const h = vi.hoisted(() => ({
  handler: null as null | ((type: string, data: unknown) => void),
  sessions: vi.fn(),
  lookup: vi.fn(),
  announce: vi.fn(),
  showNotification: vi.fn(),
}));

vi.mock("next/link", () => ({ default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => <a href={href} {...rest}>{children}</a> }));
vi.mock("next/navigation", () => ({ usePathname: () => "/en/profile" }));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ status: "authenticated", user: { id: "adv", role: "psychic" } }),
  useRealtimeEvents: (_types: readonly string[], fn: (type: string, data: unknown) => void) => {
    h.handler = fn;
  },
  useRealtimeEvent: () => undefined,
}));
vi.mock("@/lib/api/endpoints", () => ({ chatApi: { sessions: (...a: unknown[]) => h.sessions(...a) }, aliasApi: { lookup: (...a: unknown[]) => h.lookup(...a) } }));
vi.mock("@/lib/chat/ringtone", () => ({ preloadRingtone: vi.fn(), setPendingRequests: vi.fn(), unlockRingtone: vi.fn(), useRingtoneState: () => ({ status: "ringing", pending: 0 }) }));
vi.mock("@/lib/chat/request-notifications", () => ({ showRequestNotification: (o: unknown) => h.showNotification(o), closeRequestNotification: vi.fn() }));
vi.mock("@/lib/push", () => ({ pushPermission: () => "granted" }));
vi.mock("@/components/feedback/live-region", () => ({ announce: (...a: unknown[]) => h.announce(...a) }));

import { IncomingRequestRinger } from "@/components/chat/incoming-request-ringer";
import { ClientAliasProvider } from "@/lib/chat/client-alias-provider";

const dictionary = { ...en, copy } as Dictionary;
const iso = (sec: number) => new Date(Date.now() + sec * 1000).toISOString();
const request = { id: "s1", status: "requested", client: { id: "c1", name: "Paul" }, requested_at: iso(-2), expires_at: iso(60), server_time: iso(0) };
const ui = () => (
  <I18nProvider locale="en" dictionary={dictionary}>
    <ClientAliasProvider>
      <IncomingRequestRinger />
    </ClientAliasProvider>
  </I18nProvider>
);
const setVisibility = (state: "visible" | "hidden") => Object.defineProperty(document, "visibilityState", { configurable: true, get: () => state });

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  h.handler = null;
  h.sessions.mockReset().mockResolvedValue({ items: [request] });
  h.lookup.mockReset().mockResolvedValue([{ client_id: "c1", alias: "Moon Paul", updated_at: "x" }]);
  h.announce.mockReset();
  h.showNotification.mockReset().mockResolvedValue(true);
  setVisibility("hidden");
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  setVisibility("visible");
});

describe("incoming request ringer with nicknames (§23.4)", () => {
  it("the banner, the announcement and the system notification use the nickname, not the name on the account", async () => {
    setVisibility("visible");
    render(ui());
    expect(await screen.findByText("Moon Paul would like to start a live chat.")).toBeTruthy();
    expect(screen.queryByText("Paul would like to start a live chat.")).toBeNull();
    await waitFor(() => expect(h.announce).toHaveBeenCalledWith("Moon Paul would like to start a live chat.", "assertive"));
    expect(h.announce).not.toHaveBeenCalledWith("Paul would like to start a live chat.", "assertive");
  });

  it("the notification text waits for the nickname", async () => {
    render(ui());
    await screen.findByTestId("incoming-request-banner");
    await act(async () => void (await vi.advanceTimersByTimeAsync(100)));
    expect(h.lookup).toHaveBeenCalledTimes(1);
    expect(h.showNotification).toHaveBeenCalledTimes(1);
    expect(h.showNotification.mock.calls[0][0]).toMatchObject({ body: "Moon Paul would like to start a live chat." });
  });

  it("if the nickname cannot be found in time, the real name is used (and shown once)", async () => {
    h.lookup.mockReturnValue(new Promise(() => undefined));
    setVisibility("visible");
    render(ui());
    await screen.findByTestId("incoming-request-banner");
    await act(async () => void (await vi.advanceTimersByTimeAsync(100)));
    expect(h.announce).not.toHaveBeenCalled();
    await act(async () => void (await vi.advanceTimersByTimeAsync(1600)));
    expect(h.announce).toHaveBeenCalledWith("Paul would like to start a live chat.", "assertive");
    expect(h.announce).toHaveBeenCalledTimes(1);
  });
});
