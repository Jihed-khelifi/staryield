// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import { WithSiteConfig } from "@/test/site-config";

vi.mock("next/link", () => ({ default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => <a href={href} {...rest}>{children}</a> }));
vi.mock("@/components/reviews/review-form", () => ({ ReviewForm: () => null, ReviewThanks: () => null }));
// The standard book button is tested on its own: here, what the waiting card hands it.
vi.mock("@/components/booking/book-button", () => ({
  BookButton: ({ advisor, placement }: { advisor: Record<string, unknown>; placement: string }) => (
    <span data-testid="book-button-stub" data-placement={placement} data-advisor={JSON.stringify(advisor)} />
  ),
}));
vi.mock("@/lib/booking/waitlist-provider", () => ({ WaitlistScope: ({ children }: { children: React.ReactNode }) => <>{children}</> }));
vi.mock("@/components/reviews/trustpilot-review-collector", () => ({
  TrustpilotReviewCollector: ({ placement }: { placement: string }) => <div data-testid="trustpilot-widget" data-placement={placement} />,
}));

import { SessionPanel } from "@/components/chat/session-panel";
import { ApiError } from "@/lib/api/errors";

const dictionary = { ...en, copy } as Dictionary;
const counterpart = { id: "p1", role: "psychic", name: "Theo", slug: "theo", status: "online" } as never;
const wallet = { balance: 50, free_minutes: 0 } as never;
const base = {
  role: "client" as const,
  counterpart,
  session: null,
  summary: null,
  wallet,
  rate: 4,
  secondsLeft: 0,
  busy: false,
  error: null,
  returnPath: "/en/chatroom?reader=theo",
  onStart: vi.fn(),
  onCancel: vi.fn(),
  onAccept: vi.fn(),
  onDecline: vi.fn(),
  onDismissSummary: vi.fn(),
};
const wrap = (p: Partial<React.ComponentProps<typeof SessionPanel>>) => (
  <I18nProvider locale="en" dictionary={dictionary}>
    <SessionPanel {...base} {...p} />
  </I18nProvider>
);

afterEach(() => cleanup());

describe("SessionPanel waiting card", () => {
  it("offers the standard book button when the advisor cannot chat", () => {
    render(wrap({ counterpart: { id: "p1", role: "psychic", name: "Theo", slug: "theo", status: "busy" } }));
    expect(screen.getByTestId("chat-waitlist-card").textContent).toContain("Theo is in another reading right now.");
    const stub = screen.getByTestId("book-button-stub");
    expect(stub.getAttribute("data-placement")).toBe("chatroom");
    expect(JSON.parse(stub.getAttribute("data-advisor")!)).toEqual({ slug: "theo", name: "Theo", status: "busy", rate_per_minute: 4 });
  });

  it("shows the start card instead while the advisor is online", () => {
    render(wrap({}));
    expect(screen.queryByTestId("book-button-stub")).toBeNull();
  });
});

describe("SessionPanel free mode", () => {
  it("normally shows the rate and the balance", () => {
    render(wrap({}));
    expect(screen.getByTestId("start-note").textContent).toContain("4 credits per started minute");
  });

  it("free mode asks for no credits in the start card", () => {
    render(wrap({ free: true }));
    const note = screen.getByTestId("start-note").textContent!;
    expect(note).toBe("Chat is free right now. Staryield covers the cost.");
    expect(screen.getByRole("button", { name: /Start live chat/ })).toBeTruthy();
  });

  it("no top-up prompt for an insufficient-credits error while free", () => {
    const error = new ApiError({ status: 402, code: "insufficient_credits", message: "no credits" } as never);
    render(wrap({ free: true, error }));
    expect(screen.queryByText("Top up credits")).toBeNull();
    cleanup();
    render(wrap({ error }));
    expect(screen.getByText("Top up credits")).toBeTruthy();
  });

  const summary = (sponsored: boolean) =>
    ({
      session: { id: "s1", status: "ended", billed_minutes: 4, credits_charged: sponsored ? 0 : 16, psychic_earnings: 12, sponsored },
      duration_seconds: 240,
    }) as never;

  it("the client summary reads Free (sponsored) · N minutes", () => {
    render(wrap({ summary: summary(true) }));
    expect(screen.getByTestId("summary-sponsored").textContent).toBe("Free (sponsored) · 4 minutes");
  });

  it("advisors still see their earnings for a sponsored session", () => {
    render(wrap({ role: "psychic", summary: summary(true) }));
    expect(screen.queryByTestId("summary-sponsored")).toBeNull();
    expect(screen.getByText("Your earnings")).toBeTruthy();
    expect(screen.getByText("12")).toBeTruthy();
  });
});

describe("SessionPanel member discount (§25.1)", () => {
  const requested = { id: "s1", status: "requested" } as never;
  const rateView = { rate: 32, listRate: 40, discountPct: 20, discounted: true };
  const panel = (memberships?: boolean) => (
    <I18nProvider locale="en" dictionary={dictionary}>
      <WithSiteConfig memberships={memberships}>
        <SessionPanel {...base} session={requested} secondsLeft={30} rateView={rateView} />
      </WithSiteConfig>
    </I18nProvider>
  );

  it("shows the member price under a waiting request only when memberships are on", () => {
    render(panel(true));
    expect(screen.getByTestId("request-rate").textContent).toContain("32 credits/min");
    expect(screen.getByTestId("list-rate").textContent).toBe("40");
  });

  it.each([undefined, false])("shows no member price while memberships are off (features %s)", (flag) => {
    render(panel(flag));
    expect(screen.queryByTestId("request-rate")).toBeNull();
    expect(screen.queryByTestId("list-rate")).toBeNull();
    expect(screen.queryByText(/member/i)).toBeNull();
  });
});

describe("SessionPanel ringtone hint", () => {
  const request = { id: "s1", status: "requested" } as never;

  it("advisor request card offers the ringtone when blocked, and the tap unlocks", async () => {
    const ringtone = await import("@/lib/chat/ringtone");
    const unlock = vi.spyOn(ringtone, "unlockRingtone").mockResolvedValue();
    ringtone.configureRingtone({ AudioContext: null, fetch: null, locks: null, Audio: null });
    ringtone.resetRingtone();
    // no sound available + nothing blocked: no hint
    render(wrap({ role: "psychic", session: request }));
    expect(screen.queryByRole("button", { name: "Tap to turn on the ringtone" })).toBeNull();
    cleanup();

    class Audio {
      src = "";
      preload = "";
      loop = false;
      muted = false;
      currentTime = 0;
      play = () => Promise.reject(new Error("blocked"));
      pause = () => undefined;
    }
    ringtone.configureRingtone({ AudioContext: null, fetch: null, locks: null, Audio });
    ringtone.resetRingtone();
    await ringtone.preloadRingtone();
    ringtone.startRinging();
    await new Promise((r) => setTimeout(r, 0));
    render(wrap({ role: "psychic", session: request }));
    const tap = screen.getByRole("button", { name: "Tap to turn on the ringtone" });
    tap.click();
    expect(unlock).toHaveBeenCalled();
    ringtone.stopRinging();
    ringtone.configureRingtone(null);
    ringtone.resetRingtone();
  });

  it("clients never see the hint", () => {
    render(wrap({ role: "client", session: request }));
    expect(screen.queryByRole("button", { name: "Tap to turn on the ringtone" })).toBeNull();
  });
});

describe("SessionPanel Trustpilot invitation", () => {
  const ended = (extra: Record<string, unknown>) =>
    ({
      session: { id: "s1", status: "ended", billed_minutes: 4, credits_charged: 16, psychic_earnings: 12, sponsored: false },
      duration_seconds: 240,
      ...extra,
    }) as never;

  it.each([
    ["can still rate", { can_review: true }],
    ["cannot rate", { can_review: false }],
    ["already rated", { review: { id: "r1", rating: 1, comment: "", created_at: "2026-11-09T10:00:00Z" } }],
    ["rated five stars", { can_review: false, review: { id: "r2", rating: 5, comment: "", created_at: "2026-11-09T10:00:00Z" } }],
  ])("shows the widget to every client who ended a live chat (%s)", (_name, extra) => {
    render(wrap({ summary: ended(extra) }));
    const widget = screen.getByTestId("trustpilot-widget");
    expect(widget.getAttribute("data-placement")).toBe("chat-summary");
    expect(screen.getByTestId("chat-summary").contains(widget)).toBe(true);
  });

  it("never shows it to advisors", () => {
    render(wrap({ role: "psychic", summary: ended({ can_review: false }) }));
    expect(screen.getByTestId("chat-summary")).toBeTruthy();
    expect(screen.queryByTestId("trustpilot-widget")).toBeNull();
  });
});

// --- revisions 6.0 (§26.1): free-trial hold ---------------------------------------------------------------------------
import { ChatHoldProvider, type ChatHoldValue } from "@/components/chat/chat-hold-context";
import type { ChatHold } from "@/lib/api/holds";
import type { ChatSession, SessionSummary } from "@/lib/api/types";
import { fireEvent } from "@testing-library/react";

const holdOf = (over: Partial<ChatHold> = {}): ChatHold => ({
  id: "h1",
  session_id: "s1",
  conversation_id: "c1",
  reason: "trial_ended",
  status: "expired",
  started_at: "2026-11-19T10:00:00Z",
  expires_at: "2026-11-19T10:01:00Z",
  price_credits: 40,
  held_messages: 3,
  ...over,
});
const holdValue = (over: Partial<ChatHoldValue> = {}): ChatHoldValue => ({
  role: "client",
  hold: holdOf(),
  counterpartName: "Theo",
  clockOffsetMs: 0,
  hidden: { holdId: "h1", count: 3 },
  openLocked: vi.fn(),
  openTopUp: vi.fn(),
  busy: false,
  continueOffered: false,
  continuing: false,
  continueReading: vi.fn(),
  ...over,
});
const endedSession = { id: "s1", conversation_id: "c1", status: "ended", end_reason: "hold_expired", billed_minutes: 5, free_minutes_used: 5, credits_charged: 0, psychic_earnings: 0 } as unknown as ChatSession;
const summary = { session: endedSession, duration_seconds: 300, messages: 8, can_review: false } as SessionSummary;
const withHold = (v: ChatHoldValue, p: Partial<React.ComponentProps<typeof SessionPanel>>) => (
  <I18nProvider locale="en" dictionary={dictionary}>
    <ChatHoldProvider value={v}>
      <SessionPanel {...base} {...p} />
    </ChatHoldProvider>
  </I18nProvider>
);

describe("SessionPanel with a hold (§26.1)", () => {
  it("renders nothing for a held reading: the banner above the composer owns it", () => {
    const held = { ...endedSession, status: "held", end_reason: undefined } as unknown as ChatSession;
    const { container } = render(withHold(holdValue({ hold: holdOf({ status: "open" }) }), { session: held }));
    expect(container.textContent).toBe("");
  });

  it("the ended summary says why and adds 'Top up to read {n} hidden messages'", () => {
    const v = holdValue();
    render(withHold(v, { session: endedSession, summary }));
    expect(screen.getByTestId("chat-summary").textContent).toContain("The chat ended because no top-up arrived in time. No credits were used.");
    const button = screen.getByTestId("hidden-messages-reveal");
    expect(button.textContent).toBe("Top up to read 3 hidden messages");
    fireEvent.click(button);
    expect(v.openLocked).toHaveBeenCalledWith("h1");
  });

  it("the card after the summary was closed keeps the prompt", () => {
    render(withHold(holdValue(), { session: endedSession, summary: null }));
    expect(screen.getByTestId("hidden-messages-reveal")).toBeTruthy();
    expect(screen.getByText(/Start a private live chat with Theo/)).toBeTruthy();
  });

  it("no prompt when nothing is hidden", () => {
    render(withHold(holdValue({ hidden: null }), { session: endedSession, summary }));
    expect(screen.queryByTestId("hidden-messages-prompt")).toBeNull();
    expect(screen.getByTestId("chat-summary")).toBeTruthy();
  });

  it("advisors get the summary without the prompt", () => {
    render(withHold(holdValue({ role: "psychic" }), { role: "psychic", session: endedSession, summary }));
    expect(screen.queryByTestId("hidden-messages-prompt")).toBeNull();
  });
});
