// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { FakeEventSource, installFakeEventSource } from "@/test/event-source";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const m = vi.hoisted(() => ({ replace: vi.fn(), setWallet: vi.fn(), status: { current: "authenticated" } }));
vi.mock("@/lib/api/client", () => ({ api: vi.fn(), refreshSession: vi.fn().mockResolvedValue(false) }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: m.replace, push: vi.fn() }), usePathname: () => "/en" }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: m.status.current, setWallet: m.setWallet }) }));
vi.mock("@/components/layout/public-frame", () => ({ PublicFrame: ({ children }: { children: React.ReactNode }) => <main>{children}</main> }));

import { CreditsReturn } from "@/components/billing/top-up/credits-return";

const purchase = (over: Record<string, unknown> = {}) => ({
  id: "pu_12345678", pack_id: "p", credits: 600, price_cents: 1000, currency: "USD", status: "pending", provider: "stripe", created_at: "2026-11-19T10:00:00Z", ...over,
});
const send = (p: unknown) => act(() => FakeEventSource.last.emit({ type: "purchase.updated", data: p }));

function at(search: string) {
  window.history.replaceState(null, "", `/en/credits/return${search}`);
  return renderWithI18n(<CreditsReturn />);
}

beforeAll(stubResizeObserver);
beforeEach(() => {
  m.status.current = "authenticated";
  installFakeEventSource();
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
  window.history.replaceState(null, "", "/");
});

describe("CreditsReturn", () => {
  it("waits on the purchase stream with a processing state", async () => {
    at("?purchase=pu_12345678");
    expect((await screen.findByTestId("return-waiting")).textContent).toContain("Confirming your payment…");
    expect(FakeEventSource.last.url).toContain("/credits/purchases/pu_12345678/stream");
  });

  it("shows the success, updates the wallet and links back to the account and the advisors", async () => {
    at("?purchase=pu_12345678");
    await screen.findByTestId("return-waiting");
    send(purchase({ status: "succeeded", wallet: { balance: 720, held: 0, free_minutes: 0 } }));
    expect((await screen.findByTestId("return-success")).textContent).toContain("600 credits added to your balance.");
    expect(m.setWallet).toHaveBeenCalledWith({ balance: 720, held: 0, free_minutes: 0 });
    expect(screen.getByRole("link", { name: "Back to my account" }).getAttribute("href")).toBe("/en/profile");
    expect(screen.getByRole("link", { name: "Choose an Advisor" }).getAttribute("href")).toBe("/en/psychics");
    expect(FakeEventSource.last.closed).toBe(true);
    expect(await axeViolations()).toEqual([]);
  });

  it("shows the failure with the bank-confirmation message and a way to try again", async () => {
    at("?purchase=pu_12345678");
    await screen.findByTestId("return-waiting");
    send(purchase({ status: "failed", failure_reason: "authentication_failed" }));
    const failed = await screen.findByTestId("return-failed");
    expect(failed.textContent).toContain("Your bank couldn't confirm this payment. Try again or use another card.");
    expect(screen.getByRole("link", { name: "Try again" }).getAttribute("href")).toBe("/en/credits");
    expect(m.setWallet).not.toHaveBeenCalled();
  });

  it("shows a refunded purchase", async () => {
    at("?purchase=pu_12345678");
    await screen.findByTestId("return-waiting");
    send(purchase({ status: "refunded" }));
    expect((await screen.findByTestId("return-refunded")).textContent).toContain("This payment was refunded");
  });

  it("tells a payment still processing after the timeout, with links on", async () => {
    at("?purchase=pu_12345678");
    await screen.findByTestId("return-waiting");
    act(() => FakeEventSource.last.emit({ type: "purchase.timeout" }));
    const processing = await screen.findByTestId("return-processing");
    expect(processing.textContent).toContain("Your payment is processing. Credits appear as soon as it is confirmed.");
    expect(screen.getByRole("link", { name: "Back to my account" })).toBeTruthy();
  });

  it("offers a retry when the stream cannot be read, then listens again", async () => {
    const u = userEvent.setup();
    at("?purchase=pu_12345678");
    await screen.findByTestId("return-waiting");
    act(() => FakeEventSource.last.fail(true));
    expect(await screen.findByTestId("return-error")).toBeTruthy();
    await u.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByTestId("return-waiting")).toBeTruthy();
    expect(FakeEventSource.instances.length).toBeGreaterThan(1);
  });

  it.each(["", "?purchase=", "?purchase=../../x", "?purchase=<script>"])("explains a missing or malformed purchase (%s) without opening a stream", async (search) => {
    at(search);
    expect((await screen.findByTestId("return-missing")).textContent).toContain("We could not find this payment");
    expect(screen.getByRole("link", { name: "Open billing" }).getAttribute("href")).toBe("/en/profile/settings#billing");
    expect(FakeEventSource.instances).toHaveLength(0);
  });

  it("sends a signed-out visitor to the login without opening a stream", () => {
    m.status.current = "anonymous";
    at("?purchase=pu_12345678");
    expect(m.replace).toHaveBeenCalledWith(expect.stringContaining("/en/login"));
    expect(FakeEventSource.instances).toHaveLength(0);
  });

  it("closes the stream when the page is left", async () => {
    at("?purchase=pu_12345678");
    await screen.findByTestId("return-waiting");
    const es = FakeEventSource.last;
    cleanup();
    expect(es.closed).toBe(true);
  });
});
