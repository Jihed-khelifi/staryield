// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, within } from "@testing-library/react";
import type * as React from "react";

import { renderWithI18n } from "@/test/render";

vi.mock("next/link", () => ({ default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => <a href={href} {...rest}>{children}</a> }));
vi.mock("next/navigation", () => ({ usePathname: () => "/en/faq" }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: "anonymous", user: null }) }));

import Faq from "@/components/content/faq";
import { ACCOUNT_FAQ, FAQ_PAGE_ENTRIES, LIVE_READINGS_FAQ, SUPPORT_FAQ } from "@/content/faq";

afterEach(() => cleanup());

// revisions 5.1: memberships are switched off, so the FAQ does not mention them (docs/platform-extensions.md §25.1).
describe("FAQ page", () => {
  it("has no membership, subscription, Premium or app-store question", () => {
    const { container } = renderWithI18n(<Faq />);
    const text = container.textContent ?? "";
    expect(text).not.toMatch(/subscription|Premium|membership|App Store|Play Store|iPhone|Android|SpinWheel|3-days trial|Web subscription/i);
  });

  it("keeps the website, advisor, astrologer, billing and live-reading categories", () => {
    const { container } = renderWithI18n(<Faq />);
    const text = container.textContent ?? "";
    for (const heading of ["STARYIELD Website Functional", "Chatting with Advisors", "STARYIELD Astrologers", "STARYIELD Billing", "Live readings"]) expect(text).toContain(heading);
  });

  it("offers help for the account, credits and readings, and no audit or assurance wording", () => {
    const { container } = renderWithI18n(<Faq />);
    const text = container.textContent ?? "";
    expect(text).toContain("If you need help with your account, credits or readings");
    expect(text).not.toMatch(/audit|assurance|verification|financial infrastructure/i);
  });

  // Conversion follow-ups, slice E (docs/platform-extensions.md §27.1): every question has an answer.
  it("answers every question: no item is left with the support-team placeholder", () => {
    const { container } = renderWithI18n(<Faq />);
    expect(container.textContent).not.toContain("For help with this question");
    const items = [...container.querySelectorAll<HTMLDetailsElement>("details.faq-item")];
    expect(items).toHaveLength(FAQ_PAGE_ENTRIES.length);
    for (const item of items) {
      const answer = item.querySelector("summary + p");
      expect(answer?.textContent?.trim().length ?? 0, item.querySelector("summary")?.textContent ?? "").toBeGreaterThan(80);
    }
  });

  it("shows the answer of each of the 22 support questions under its question", () => {
    const { container } = renderWithI18n(<Faq />);
    const items = [...container.querySelectorAll("details.faq-item")];
    for (const entry of SUPPORT_FAQ) {
      const item = items.find((candidate) => candidate.querySelector("summary")?.textContent?.includes(entry.q));
      expect(item, entry.q).toBeTruthy();
      const plain = entry.a.replace(/\[([^\]]+)\]\([^)\s]+\)/g, "$1");
      expect(item!.querySelector("summary + p")?.textContent, entry.q).toBe(plain);
    }
  });

  it("adds the group on free minutes, invites and your account, after the live readings", () => {
    const { container } = renderWithI18n(<Faq />);
    const group = container.querySelector<HTMLElement>("#account-and-offers");
    expect(group).toBeTruthy();
    expect(within(group!).getByRole("heading", { name: "Free minutes, invites and your account" })).toBeTruthy();
    for (const entry of ACCOUNT_FAQ) expect(group!.textContent).toContain(entry.q);
    const live = container.querySelector("#live-readings")!;
    for (const entry of LIVE_READINGS_FAQ) expect(live.textContent).toContain(entry.q);
    expect(live.compareDocumentPosition(group!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("links the answers to the pages they mention", () => {
    const { container } = renderWithI18n(<Faq />);
    const reset = [...container.querySelectorAll("details.faq-item")].find((item) => item.querySelector("summary")?.textContent?.includes("reset staryield Website account"));
    expect(reset?.querySelector("p a")?.getAttribute("href")).toBe("/en/login");
    expect(reset?.textContent).toContain("reset link");
    const refund = container.querySelector('details.faq-item a[href="/en/refund-policy"]');
    expect(refund).toBeTruthy();
  });
});
