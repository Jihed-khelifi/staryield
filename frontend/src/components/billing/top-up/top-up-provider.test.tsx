// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import type { TopUpAdvisor } from "@/lib/billing/top-up";

const m = vi.hoisted(() => ({ push: vi.fn(), status: { current: "authenticated" } }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: m.push }) }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: m.status.current }) }));
vi.mock("@/components/billing/top-up/credit-top-up", () => ({
  CreditTopUp: ({ context, onClose }: { context?: TopUpAdvisor; onClose: () => void }) => (
    <div role="dialog" aria-label="Top up" data-testid="top-up-stub">
      <span data-testid="stub-context">{JSON.stringify(context ?? null)}</span>
      <button type="button" onClick={onClose}>
        close
      </button>
    </div>
  ),
}));

import { TopUpAdvisorScope, TopUpLink, TopUpProvider, useTopUp } from "@/components/billing/top-up/top-up-provider";

const dictionary = { ...en, copy } as Dictionary;
const wrap = (node: React.ReactNode) => <I18nProvider locale="en" dictionary={dictionary}>{node}</I18nProvider>;

function Opener({ context }: { context?: TopUpAdvisor }) {
  const { open, isOpen } = useTopUp();
  return (
    <button type="button" onClick={() => open(context)}>
      {isOpen ? "open" : "closed"}
    </button>
  );
}

beforeEach(() => {
  m.status.current = "authenticated";
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("TopUpProvider", () => {
  it("opens the dialog in place, without navigating, and closes it", async () => {
    const u = userEvent.setup();
    render(wrap(<TopUpProvider><Opener /></TopUpProvider>));
    expect(screen.queryByTestId("top-up-stub")).toBeNull();
    await u.click(screen.getByRole("button", { name: "closed" }));
    expect(screen.getByTestId("top-up-stub")).toBeTruthy();
    expect(screen.getByRole("button", { name: "open" })).toBeTruthy();
    expect(m.push).not.toHaveBeenCalled();
    await u.click(screen.getByRole("button", { name: "close" }));
    expect(screen.queryByTestId("top-up-stub")).toBeNull();
  });

  it("passes the advisor and the rate the client pays to the dialog", async () => {
    const u = userEvent.setup();
    render(wrap(<TopUpProvider><Opener context={{ advisorName: "Solomon", ratePerMinute: 35 }} /></TopUpProvider>));
    await u.click(screen.getByRole("button"));
    expect(JSON.parse(screen.getByTestId("stub-context").textContent ?? "")).toEqual({ advisorName: "Solomon", ratePerMinute: 35 });
  });

  it("sends a signed-out visitor to the login instead of opening", async () => {
    const u = userEvent.setup();
    m.status.current = "anonymous";
    render(wrap(<TopUpProvider><Opener /></TopUpProvider>));
    await u.click(screen.getByRole("button"));
    expect(screen.queryByTestId("top-up-stub")).toBeNull();
    expect(m.push).toHaveBeenCalledWith(expect.stringMatching(/^\/en\/login\?next=/));
  });

  it("closes when the session ends", async () => {
    const u = userEvent.setup();
    const { rerender } = render(wrap(<TopUpProvider><Opener /></TopUpProvider>));
    await u.click(screen.getByRole("button"));
    expect(screen.getByTestId("top-up-stub")).toBeTruthy();
    m.status.current = "anonymous";
    rerender(wrap(<TopUpProvider><Opener /></TopUpProvider>));
    expect(screen.queryByTestId("top-up-stub")).toBeNull();
  });

  it("gives the focus back to what opened it when it closes", async () => {
    const u = userEvent.setup();
    render(wrap(<TopUpProvider><Opener /></TopUpProvider>));
    const opener = screen.getByRole("button", { name: "closed" });
    await u.click(opener);
    await u.click(screen.getByRole("button", { name: "close" }));
    expect(screen.queryByTestId("top-up-stub")).toBeNull();
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("button", { name: "closed" })));
  });

  it("useTopUp needs the provider", () => {
    const error = vi.spyOn(console, "error").mockImplementation(() => undefined);
    expect(() => render(wrap(<Opener />))).toThrow("useTopUp must be used inside <TopUpProvider>");
    error.mockRestore();
  });
});

describe("TopUpLink", () => {
  it("keeps the href and opens the dialog in place on a plain click", () => {
    render(wrap(<TopUpProvider><TopUpLink href="/en/credits?next=%2Fen%2Fchatroom" advisorName="Theo" ratePerMinute={40}>Top up</TopUpLink></TopUpProvider>));
    const link = screen.getByRole("link", { name: "Top up" });
    expect(link.getAttribute("href")).toBe("/en/credits?next=%2Fen%2Fchatroom");
    // fireEvent returns false when the default (navigation) was prevented.
    expect(fireEvent.click(link)).toBe(false);
    expect(JSON.parse(screen.getByTestId("stub-context").textContent ?? "")).toEqual({ advisorName: "Theo", ratePerMinute: 40 });
  });

  it("lets modified clicks and new-tab links follow the href", () => {
    render(wrap(<TopUpProvider><TopUpLink>Top up</TopUpLink><TopUpLink target="_blank">Elsewhere</TopUpLink></TopUpProvider>));
    expect(fireEvent.click(screen.getByRole("link", { name: "Top up" }), { ctrlKey: true })).toBe(true);
    expect(fireEvent.click(screen.getByRole("link", { name: "Top up" }), { metaKey: true })).toBe(true);
    expect(fireEvent.click(screen.getByRole("link", { name: "Top up" }), { button: 1 })).toBe(true);
    expect(fireEvent.click(screen.getByRole("link", { name: "Elsewhere" }))).toBe(true);
    expect(screen.queryByTestId("top-up-stub")).toBeNull();
  });

  it("defaults to /credits and is a plain link without a provider", () => {
    render(wrap(<TopUpLink>Top up</TopUpLink>));
    const link = screen.getByRole("link", { name: "Top up" });
    expect(link.getAttribute("href")).toBe("/en/credits");
    expect(fireEvent.click(link)).toBe(true);
  });

  it("runs the caller's onClick first (e.g. closing a drawer)", () => {
    const onClick = vi.fn();
    render(wrap(<TopUpProvider><TopUpLink onClick={onClick}>Top up</TopUpLink></TopUpProvider>));
    fireEvent.click(screen.getByRole("link", { name: "Top up" }));
    expect(onClick).toHaveBeenCalled();
    expect(screen.getByTestId("top-up-stub")).toBeTruthy();
  });
});

// revisions 5.0: the conversation on screen sets the advisor for every top-up opened meanwhile (§24.7).
describe("TopUpAdvisorScope", () => {
  it("gives a top-up opened without context the advisor of the conversation on screen", () => {
    render(
      wrap(
        <TopUpProvider>
          <TopUpAdvisorScope advisor={{ advisorName: "Orion", ratePerMinute: 45 }} />
          <TopUpLink>Balance</TopUpLink>
        </TopUpProvider>,
      ),
    );
    fireEvent.click(screen.getByRole("link", { name: "Balance" }));
    expect(JSON.parse(screen.getByTestId("stub-context").textContent ?? "")).toEqual({ advisorName: "Orion", ratePerMinute: 45 });
  });

  it("lets an explicit context win over the scope", async () => {
    const u = userEvent.setup();
    render(
      wrap(
        <TopUpProvider>
          <TopUpAdvisorScope advisor={{ advisorName: "Orion", ratePerMinute: 45 }} />
          <Opener context={{ advisorName: "Theo", ratePerMinute: 30 }} />
        </TopUpProvider>,
      ),
    );
    await u.click(screen.getByRole("button"));
    expect(JSON.parse(screen.getByTestId("stub-context").textContent ?? "")).toEqual({ advisorName: "Theo", ratePerMinute: 30 });
  });

  it("forgets the advisor when the conversation leaves the screen, and ignores incomplete advisors", async () => {
    const u = userEvent.setup();
    const tree = (scope: React.ReactNode) => wrap(<TopUpProvider>{scope}<Opener /></TopUpProvider>);
    const { rerender } = render(tree(<TopUpAdvisorScope advisor={{ advisorName: "Orion", ratePerMinute: 45 }} />));
    rerender(tree(null));
    await u.click(screen.getByRole("button"));
    expect(screen.getByTestId("stub-context").textContent).toBe("null");
    await u.click(screen.getByRole("button", { name: "close" }));
    rerender(tree(<TopUpAdvisorScope advisor={{ advisorName: "Orion", ratePerMinute: 0 }} />));
    await u.click(screen.getByRole("button"));
    expect(screen.getByTestId("stub-context").textContent).toBe("null");
  });
});
