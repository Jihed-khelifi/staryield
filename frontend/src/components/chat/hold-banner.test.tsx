// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen } from "@testing-library/react";

import type { ChatHold } from "@/lib/api/holds";
import { axeViolations, renderWithI18n } from "@/test/render";
import { ChatHoldProvider, type ChatHoldValue } from "@/components/chat/chat-hold-context";
import { HoldBanner } from "@/components/chat/hold-banner";

const NOW = Date.parse("2026-11-19T10:00:00Z");
beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

const hold = (over: Partial<ChatHold> = {}): ChatHold => ({
  id: "h1",
  session_id: "s",
  conversation_id: "c",
  reason: "trial_ended",
  status: "open",
  started_at: "2026-11-19T10:00:00Z",
  expires_at: "2026-11-19T10:01:00Z",
  price_credits: 40,
  held_messages: 0,
  ...over,
});
const value = (over: Partial<ChatHoldValue> = {}): ChatHoldValue => ({
  role: "client",
  hold: hold(),
  counterpartName: "Theo",
  clockOffsetMs: 0,
  hidden: null,
  openLocked: vi.fn(),
  openTopUp: vi.fn(),
  busy: false,
  continueOffered: false,
  continuing: false,
  continueReading: vi.fn(),
  ...over,
});
const banner = (v: ChatHoldValue) =>
  renderWithI18n(
    <ChatHoldProvider value={v}>
      <HoldBanner />
    </ChatHoldProvider>,
  );

describe("client banner", () => {
  it("counts down to the server's expires_at, not from a local 60", () => {
    // 17 seconds into the hold: 43 s are left, whatever the hold length was.
    vi.setSystemTime(NOW + 17_000);
    banner(value());
    expect(screen.getByTestId("hold-banner-timer").textContent).toBe("0:43");
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(screen.getByTestId("hold-banner-timer").textContent).toBe("0:42");
    act(() => {
      vi.advanceTimersByTime(60_000);
    });
    expect(screen.getByTestId("hold-banner-timer").textContent).toBe("0:00");
  });

  it("follows the server clock when the device clock is off", () => {
    // The device is two minutes behind the server: without the offset the hold would look 3 minutes long.
    vi.setSystemTime(NOW - 120_000);
    banner(value({ clockOffsetMs: 120_000 }));
    expect(screen.getByTestId("hold-banner-timer").textContent).toBe("1:00");
  });

  it("re-syncs when the hold is extended (session.hold_extended)", () => {
    const view = banner(value());
    expect(screen.getByTestId("hold-banner-timer").textContent).toBe("1:00");
    view.rerender(
      <ChatHoldProvider value={value({ hold: hold({ expires_at: "2026-11-19T10:01:15Z" }) })}>
        <HoldBanner />
      </ChatHoldProvider>,
    );
    expect(screen.getByTestId("hold-banner-timer").textContent).toBe("1:15");
  });

  it("says why, what continuing costs and offers Top up", () => {
    const v = value();
    banner(v);
    expect(screen.getByText(/Your free minutes are over\./)).toBeTruthy();
    expect(screen.getByTestId("hold-banner-note").textContent).toBe("Top up now to keep chatting with Theo and read their replies. Continuing costs 40 credits per minute.");
    fireEvent.click(screen.getByTestId("hold-banner-topup"));
    expect(v.openTopUp).toHaveBeenCalledTimes(1);
  });

  it("uses the credits copy when the credits ran out", () => {
    banner(value({ hold: hold({ reason: "credits_exhausted" }) }));
    expect(screen.getByText(/Your credits ran out\./)).toBeTruthy();
    expect(screen.queryByText(/free minutes are over/)).toBeNull();
  });

  it("offers 'Continue reading' only when the wallet covers the price and the hold did not convert", () => {
    const v = value();
    const view = banner(v);
    expect(screen.queryByTestId("hold-banner-continue")).toBeNull();
    view.rerender(
      <ChatHoldProvider value={{ ...v, continueOffered: true }}>
        <HoldBanner />
      </ChatHoldProvider>,
    );
    const continueButton = screen.getByTestId("hold-banner-continue");
    expect(continueButton.textContent).toBe("Continue reading (40 credits)");
    fireEvent.click(continueButton);
    expect(v.continueReading).toHaveBeenCalledTimes(1);
  });

  it("disables 'Continue reading' while it runs", () => {
    banner(value({ continueOffered: true, continuing: true }));
    expect((screen.getByTestId("hold-banner-continue") as HTMLButtonElement).disabled).toBe(true);
  });

  it("is not a live region (the countdown would be read every second)", () => {
    banner(value());
    expect(screen.getByTestId("hold-banner-timer").getAttribute("aria-live")).toBe("off");
    expect(screen.getByTestId("hold-banner").closest("[aria-live]")).toBeNull();
  });

  it("shows nothing without an open hold", () => {
    banner(value({ hold: hold({ status: "expired" }) }));
    expect(screen.queryByTestId("hold-banner")).toBeNull();
    cleanup();
    banner(value({ hold: null }));
    expect(screen.queryByTestId("hold-banner")).toBeNull();
  });

  it("has no serious accessibility violations", async () => {
    // axe waits on real timers.
    vi.useRealTimers();
    const { container } = banner(value({ continueOffered: true }));
    expect(await axeViolations(container)).toEqual([]);
  });
});

describe("advisor banner", () => {
  it("tells the advisor their replies are hidden, with the countdown", () => {
    banner(value({ role: "psychic", counterpartName: "Alex" }));
    const el = screen.getByTestId("hold-banner");
    expect(el.textContent).toContain("Alex's free minutes are over. Your replies are hidden from them until they top up. — 1:00");
    expect(screen.queryByTestId("hold-banner-topup")).toBeNull();
    expect(screen.queryByTestId("hold-banner-continue")).toBeNull();
  });

  it("names the other reason when the credits ran out", () => {
    banner(value({ role: "psychic", counterpartName: "Alex", hold: hold({ reason: "credits_exhausted" }) }));
    expect(screen.getByTestId("hold-banner").textContent).toContain("Alex's credits ran out. Your replies are hidden from them until they top up.");
  });
});
