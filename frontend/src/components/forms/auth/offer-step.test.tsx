// @vitest-environment jsdom
import type * as React from "react";
import { cleanup, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { SiteConfig } from "@/lib/api/types";
import { renderWithI18n } from "@/test/render";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn(), replace: vi.fn() }) }));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: React.ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@/lib/analytics", () => ({ track: vi.fn() }));
vi.mock("@/components/layout/public-frame", () => ({ PublicFrame: ({ children }: { children: React.ReactNode }) => <main>{children}</main> }));
vi.mock("@/components/forms/radiant-panel", () => ({ RadiantPanel: ({ children }: { children: React.ReactNode }) => <div>{children}</div> }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: "authenticated" }) }));
vi.mock("@/lib/api/endpoints", () => ({ siteApi: { config: vi.fn(() => new Promise(() => {})) } }));

import { OfferStep } from "@/components/forms/auth/offer-step";
import { SiteConfigProvider } from "@/lib/site-config/site-config-provider";

const cfg = (enabled: boolean, minutes: number): SiteConfig => ({
  banners: { rotation_seconds: 6, items: [] },
  signup_offer: { enabled, free_minutes: enabled ? minutes : 0 },
  free_mode: { active: false },
  generated_at: "2026-11-07T00:00:00Z",
});
afterEach(cleanup);

describe("OfferStep free minutes", () => {
  it("shows the live number of free minutes", () => {
    renderWithI18n(<SiteConfigProvider initial={cfg(true, 8)}><OfferStep /></SiteConfigProvider>);
    expect(screen.getByTestId("offer-free-minutes").textContent).toBe("First 8 minutes are on us!");
  });
  it("hides the claim when the offer is off", () => {
    renderWithI18n(<SiteConfigProvider initial={cfg(false, 5)}><OfferStep /></SiteConfigProvider>);
    expect(screen.queryByTestId("offer-free-minutes")).toBeNull();
  });
});
