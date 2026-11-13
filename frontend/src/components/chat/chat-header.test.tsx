// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, within } from "@testing-library/react";

import type { ChatSession, Participant } from "@/lib/api/types";
import { axeViolations, renderWithI18n } from "@/test/render";
import { WithSiteConfig } from "@/test/site-config";
import { ChatHeader, ChatroomTopBar } from "@/components/chat/chat-header";
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
  const discounted = () => sessionRate(session({ rate_per_minute: 32, list_rate_per_minute: 40, discount_pct: 20 }));
  const withMemberships = (ui: React.ReactElement, memberships?: boolean) => renderWithI18n(<WithSiteConfig memberships={memberships}>{ui}</WithSiteConfig>);

  it("strikes the list price through and says it plainly to screen readers", async () => {
    withMemberships(<RatePrice rate={discounted()} />, true);
    const price = screen.getByTestId("member-rate");
    const list = within(price).getByTestId("list-rate");
    expect(list.tagName).toBe("S");
    expect(list.textContent).toBe("40");
    expect(list.closest("[aria-hidden='true']")).not.toBeNull();
    expect(within(price).getByText("Member price: 32 credits per minute instead of 40 (20% off).")).toBeTruthy();
    expect(price.textContent).toContain("32 credits/min");
    expect(await axeViolations()).toEqual([]);
  });

  it.each([undefined, false])("advertises no member price while memberships are off (features %s): a plain rate", (flag) => {
    withMemberships(<RatePrice rate={discounted()} />, flag);
    expect(screen.queryByTestId("member-rate")).toBeNull();
    expect(screen.queryByTestId("list-rate")).toBeNull();
    expect(screen.queryByText(/Member price/)).toBeNull();
    expect(screen.getByText("32 credits/min")).toBeTruthy();
  });

  it("shows a plain rate without a discount", () => {
    renderWithI18n(<RatePrice rate={sessionRate(session({ rate_per_minute: 40, list_rate_per_minute: 40 }))} />);
    expect(screen.queryByTestId("member-rate")).toBeNull();
    expect(screen.getByText("40 credits/min")).toBeTruthy();
  });

  it("appears in the client's header next to the status only when memberships are on", () => {
    const s = session({ rate_per_minute: 32, list_rate_per_minute: 40, discount_pct: 20 });
    withMemberships(<ChatHeader counterpart={advisor} viewerRole="client" session={s} elapsedSeconds={521} onOpenInbox={noop} onOpenDetails={noop} onEnd={noop} ending={false} rate={sessionRate(s)} />, true);
    expect(within(screen.getByTestId("chat-header-status")).getByTestId("member-rate")).toBeTruthy();
    cleanup();
    header({ viewerRole: "client", counterpart: advisor, session: s, rate: sessionRate(s) });
    expect(screen.queryByTestId("member-rate")).toBeNull();
    expect(within(screen.getByTestId("chat-header-status")).getByText("32 credits/min")).toBeTruthy();
  });
});

describe("phone top bar (§20.2)", () => {
  it("dots the menu button when there is activity elsewhere and says so", async () => {
    header({ viewerRole: "client", counterpart: advisor, activity: true });
    const menu = screen.getByRole("button", { name: "Open conversations, new activity" });
    const dot = within(menu).getByTestId("menu-activity-dot");
    expect(dot.getAttribute("aria-hidden")).toBe("true");
    expect(await axeViolations()).toEqual([]);
  });

  it("has a plain menu label and no dot without activity", () => {
    header({ viewerRole: "client", counterpart: advisor });
    expect(screen.getByRole("button", { name: "Open conversations" })).toBeTruthy();
    expect(screen.queryByTestId("menu-activity-dot")).toBeNull();
  });

  it("hides the details button below 768px (the avatar opens details) and keeps End chat", () => {
    header({ viewerRole: "client", counterpart: advisor });
    expect(screen.getByRole("button", { name: "Open reader details" }).className).toContain("max-md:hidden");
    expect(screen.getByRole("button", { name: "End the chat with Theo" })).toBeTruthy();
  });

  it("places the report button in the header, hidden on phones, only with a target", () => {
    const { unmount } = header({ viewerRole: "client", counterpart: advisor });
    expect(screen.queryByTestId("report-issue-trigger")).toBeNull();
    unmount();
    header({ viewerRole: "client", counterpart: advisor, report: { conversationId: "conv", advisorName: "Theo" } });
    const trigger = screen.getByTestId("report-issue-trigger");
    expect(trigger.getAttribute("data-placement")).toBe("header");
    expect(trigger.className).toContain("max-md:hidden");
  });
});

describe("ChatroomTopBar (no conversation on screen)", () => {
  it("keeps the drawer reachable and dots activity", async () => {
    const onOpen = vi.fn();
    renderWithI18n(<ChatroomTopBar onOpenInbox={onOpen} activity />);
    screen.getByRole("button", { name: "Open conversations, new activity" }).click();
    expect(onOpen).toHaveBeenCalled();
    expect(screen.getByTestId("chat-topbar").className).toContain("md:hidden");
    expect(await axeViolations()).toEqual([]);
  });
});
