// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup } from "@testing-library/react";
import type * as React from "react";

import { renderWithI18n } from "@/test/render";

vi.mock("next/link", () => ({ default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => <a href={href} {...rest}>{children}</a> }));
vi.mock("next/navigation", () => ({ usePathname: () => "/en/faq" }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: "anonymous", user: null }) }));

import Faq from "@/components/content/faq";

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
});
