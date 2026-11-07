// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";

vi.mock("next/link", () => ({ default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => <a href={href} {...rest}>{children}</a> }));
vi.mock("@/components/reviews/review-form", () => ({ ReviewForm: () => null, ReviewThanks: () => null }));
vi.mock("@/components/waitlist/waitlist-button", () => ({ WaitlistButton: () => null }));

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
