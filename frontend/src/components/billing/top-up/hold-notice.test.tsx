// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import type { TopUpAdvisor, TopUpHold } from "@/lib/billing/top-up";

const m = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: m.push }) }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: "authenticated" }) }));
vi.mock("@/components/billing/top-up/credit-top-up", async () => {
  const { HoldNotice } = await import("@/components/billing/top-up/hold-notice");
  return {
    CreditTopUp: ({ context, hold }: { context?: TopUpAdvisor; hold?: TopUpHold }) => (
      <div role="dialog" aria-label="Top up" data-testid="top-up-stub">
        {hold ? <HoldNotice hold={hold} advisorName={context?.advisorName} /> : <span data-testid="no-hold" />}
      </div>
    ),
  };
});

import { HoldNotice } from "@/components/billing/top-up/hold-notice";
import { TopUpAdvisorScope, TopUpHoldScope, TopUpProvider, useTopUp } from "@/components/billing/top-up/top-up-provider";

const dictionary = { ...en, copy } as Dictionary;
const wrap = (node: React.ReactNode) => (
  <I18nProvider locale="en" dictionary={dictionary}>
    {node}
  </I18nProvider>
);
const NOW = Date.parse("2026-11-19T10:00:00Z");
const hold = (over: Partial<TopUpHold> = {}): TopUpHold => ({ reason: "trial_ended", expiresAt: "2026-11-19T10:00:42Z", priceCredits: 40, clockOffsetMs: 0, ...over });

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("HoldNotice (the dialog's hold banner)", () => {
  it("counts down to expires_at and states the reason and the price", () => {
    render(wrap(<HoldNotice hold={hold()} advisorName="Theo" />));
    expect(screen.getByText(/Your free minutes are over\./)).toBeTruthy();
    expect(screen.getByTestId("top-up-hold-timer").textContent).toBe("0:42");
    expect(screen.getByTestId("top-up-hold-price").textContent).toBe("Top up now to keep chatting with Theo and read their replies. Continuing costs 40 credits per minute.");
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(screen.getByTestId("top-up-hold-timer").textContent).toBe("0:40");
  });

  it("uses the credits copy and a neutral name without an advisor", () => {
    render(wrap(<HoldNotice hold={hold({ reason: "credits_exhausted" })} />));
    expect(screen.getByText(/Your credits ran out\./)).toBeTruthy();
    expect(screen.getByTestId("top-up-hold-price").textContent).toContain("chatting with your advisor");
  });

  it("is not a live region", () => {
    render(wrap(<HoldNotice hold={hold()} advisorName="Theo" />));
    expect(screen.getByTestId("top-up-hold-timer").getAttribute("aria-live")).toBe("off");
  });
});

function Opener({ context }: { context?: TopUpAdvisor }) {
  const { open } = useTopUp();
  return (
    <button type="button" onClick={() => open(context)}>
      open
    </button>
  );
}

describe("TopUpHoldScope", () => {
  it("the dialog shows the live hold of the mounted scope and drops it when the hold ends", () => {
    const view = render(
      wrap(
        <TopUpProvider>
          <TopUpAdvisorScope advisor={{ advisorName: "Theo", ratePerMinute: 40 }} />
          <TopUpHoldScope hold={hold()} />
          <Opener />
        </TopUpProvider>,
      ),
    );
    act(() => screen.getByText("open").click());
    expect(screen.getByTestId("top-up-hold")).toBeTruthy();
    // The hold converted: the notice goes away without closing the dialog.
    view.rerender(
      wrap(
        <TopUpProvider>
          <TopUpAdvisorScope advisor={{ advisorName: "Theo", ratePerMinute: 40 }} />
          <TopUpHoldScope hold={undefined} />
          <Opener />
        </TopUpProvider>,
      ),
    );
    expect(screen.queryByTestId("top-up-hold")).toBeNull();
    expect(screen.getByTestId("top-up-stub")).toBeTruthy();
  });

  it("follows an extension of the deadline", () => {
    const view = render(
      wrap(
        <TopUpProvider>
          <TopUpHoldScope hold={hold()} />
          <Opener context={{ advisorName: "Theo", ratePerMinute: 40 }} />
        </TopUpProvider>,
      ),
    );
    act(() => screen.getByText("open").click());
    expect(screen.getByTestId("top-up-hold-timer").textContent).toBe("0:42");
    view.rerender(
      wrap(
        <TopUpProvider>
          <TopUpHoldScope hold={hold({ expiresAt: "2026-11-19T10:00:57Z" })} />
          <Opener context={{ advisorName: "Theo", ratePerMinute: 40 }} />
        </TopUpProvider>,
      ),
    );
    expect(screen.getByTestId("top-up-hold-timer").textContent).toBe("0:57");
  });

  it("without a mounted scope the dialog shows the hold it was opened with", () => {
    render(
      wrap(
        <TopUpProvider>
          <Opener context={{ advisorName: "Theo", ratePerMinute: 40, hold: hold() }} />
        </TopUpProvider>,
      ),
    );
    act(() => screen.getByText("open").click());
    expect(screen.getByTestId("top-up-hold")).toBeTruthy();
  });

  it("no hold, no notice", () => {
    render(
      wrap(
        <TopUpProvider>
          <TopUpHoldScope hold={undefined} />
          <Opener context={{ advisorName: "Theo", ratePerMinute: 40 }} />
        </TopUpProvider>,
      ),
    );
    act(() => screen.getByText("open").click());
    expect(screen.getByTestId("no-hold")).toBeTruthy();
  });
});
