// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import { canonicalPricing } from "@/test/pricing-fixtures";

const h = vi.hoisted(() => ({ fetchPricing: vi.fn() }));
vi.mock("@/lib/pricing/server", () => ({ fetchPricing: h.fetchPricing }));
vi.mock("@/components/layout/public-frame", () => ({ PublicFrame: ({ children }: { children: React.ReactNode }) => <div data-testid="frame">{children}</div> }));
vi.mock("@/components/pricing/pricing-page", () => ({ PricingPage: ({ initial }: { initial: { packs: unknown[] } | null }) => <p data-testid="page">{initial ? `${initial.packs.length} packs` : "no catalog"}</p> }));

import { PricingScreen } from "@/components/pricing/pricing-screen";

afterEach(() => cleanup());

describe("PricingScreen (server entry of /pricing)", () => {
  it("fetches the catalog in the page's locale and renders the page inside the public frame", async () => {
    h.fetchPricing.mockResolvedValue(canonicalPricing());
    render(await PricingScreen({ locale: "fr" }));
    expect(h.fetchPricing).toHaveBeenCalledWith("fr");
    expect(screen.getByTestId("frame")).toBeTruthy();
    expect(screen.getByTestId("page").textContent).toBe("6 packs");
  });

  it("hands the page null when the API is unavailable (the page shows its Retry)", async () => {
    h.fetchPricing.mockResolvedValue(null);
    render(await PricingScreen({ locale: "en" }));
    expect(screen.getByTestId("page").textContent).toBe("no catalog");
  });
});
