// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, within } from "@testing-library/react";

import type { ChatSession, Participant } from "@/lib/api/types";
import { axeViolations, renderWithI18n } from "@/test/render";
import { ChatHeader } from "@/components/chat/chat-header";
import { RatePrice } from "@/components/chat/rate-price";
import { sessionEarnings, sessionRate } from "@/components/chat/chat-model";

afterEach(cleanup);

const client: Participant = { id: "c1", role: "client", name: "Paul", status: "online" };
const advisor: Participant = { id: "p1", role: "psychic", name: "Theo", slug: "theo", status: "busy" };
const session = (over: Partial<ChatSession> = {}) =>
  ({
    id: "s1",
    conversation_id: "conv",
    status: "active",
    client,
    psychic: advisor,
    rate_per_minute: 40,
    requested_at: "2026-11-05T10:00:00Z",
    active_seconds: 521,
    billed_minutes: 9,
    free_minutes_used: 3,
    reconnects: 0,
    server_time: "2026-11-05T10:09:00Z",
    ...over,
  }) as ChatSession;

const noop = vi.fn();
const header = (props: Partial<React.ComponentProps<typeof ChatHeader>>) =>
  renderWithI18n(
    <ChatHeader counterpart={client} viewerRole="psychic" session={session()} elapsedSeconds={521} onOpenInbox={noop} onOpenDetails={noop} onEnd={noop} ending={false} {...props} />,
  );

describe("advisor chat header (Figma 1172:1191)", () => {
  it("shows the live session earnings in money next to the timer", async () => {
    const s = session({ psychic_earnings: 2017, psychic_earnings_cents: 4033, currency: "USD" });
    header({ session: s, earnings: sessionEarnings(s), rate: sessionRate(s) });
    expect(screen.getByRole("timer").textContent).toBe("00:08:41");
    const earned = screen.getByTestId("session-earnings");
    expect(earned.textContent).toContain("($40.33)");
    expect(within(earned).getByText("Session earnings: $40.33")).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
  });

  it("falls back to credits when the money value is unknown", () => {
    const s = session({ psychic_earnings: 24 });
    header({ session: s, earnings: sessionEarnings(s) });
    expect(screen.getByTestId("session-earnings").textContent).toContain("(24 credits)");
  });

  it("formats the amount for the viewer's currency", () => {
    const s = session({ psychic_earnings: 10, psychic_earnings_cents: 1250, currency: "EUR" });
    header({ session: s, earnings: sessionEarnings(s) });
    expect(screen.getByTestId("session-earnings").textContent).toContain("€12.50");
  });

  it("never shows earnings to clients or before the chat starts", () => {
    const s = session({ psychic_earnings: 10, psychic_earnings_cents: 1250, currency: "USD" });
    const { unmount } = header({ viewerRole: "client", counterpart: advisor, session: s, earnings: sessionEarnings(s) });
    expect(screen.queryByTestId("session-earnings")).toBeNull();
    unmount();
    header({ session: session({ status: "requested", psychic_earnings: 0, psychic_earnings_cents: 0 }), earnings: { credits: 0, cents: 0, currency: "USD" } });
    expect(screen.queryByTestId("session-earnings")).toBeNull();
    expect(screen.queryByRole("timer")).toBeNull();
  });
});

describe("member price (client view of a discount)", () => {
  it("strikes the list price through and says it plainly to screen readers", async () => {
    const rate = sessionRate(session({ rate_per_minute: 32, list_rate_per_minute: 40, discount_pct: 20 }));
    renderWithI18n(<RatePrice rate={rate} />);
    const price = screen.getByTestId("member-rate");
    const list = within(price).getByTestId("list-rate");
    expect(list.tagName).toBe("S");
    expect(list.textContent).toBe("40");
    expect(list.closest("[aria-hidden='true']")).not.toBeNull();
    expect(within(price).getByText("Member price: 32 credits per minute instead of 40 (20% off).")).toBeTruthy();
    expect(price.textContent).toContain("32 credits/min");
    expect(await axeViolations()).toEqual([]);
  });

  it("shows a plain rate without a discount", () => {
    renderWithI18n(<RatePrice rate={sessionRate(session({ rate_per_minute: 40, list_rate_per_minute: 40 }))} />);
    expect(screen.queryByTestId("member-rate")).toBeNull();
    expect(screen.getByText("40 credits/min")).toBeTruthy();
  });

  it("appears in the client's header next to the status", () => {
    const s = session({ rate_per_minute: 32, list_rate_per_minute: 40, discount_pct: 20 });
    header({ viewerRole: "client", counterpart: advisor, session: s, rate: sessionRate(s) });
    expect(within(screen.getByTestId("chat-header-status")).getByTestId("member-rate")).toBeTruthy();
  });
});
