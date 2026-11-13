/** The canonical catalog of docs/platform-extensions.md §24.1 as `GET /pricing` returns it (tests and the e2e mock). */
import type { PricingPack, PricingSummary } from "@/lib/api/pricing";

const pack = (p: Partial<PricingPack> & Pick<PricingPack, "id" | "code" | "name" | "credits" | "price_cents">, sort: number): PricingPack => ({
  kind: "standard",
  tagline: "",
  label: "",
  currency: "USD",
  featured: false,
  new_clients_only: false,
  sort_order: sort,
  badges: [],
  ...p,
});

export const canonicalPricing = (over: Partial<PricingSummary> = {}): PricingSummary => ({
  currency: "USD",
  free_trial: { enabled: true, minutes: 4 },
  rate_range: { min: 30, max: 60 },
  reference: { pack_id: "p-quick", credits: 150, price_cents: 2000 },
  packs: [
    pack({ id: "p-welcome", code: "welcome_600", kind: "intro", name: "Welcome Pack", credits: 600, price_cents: 1000, new_clients_only: true, tagline: "Extra time to explore your first paid reading.", badges: [{ type: "new_client_offer" }] }, 1),
    pack({ id: "p-quick", code: "quick_150", name: "Quick Insight", credits: 150, price_cents: 2000, label: "One focused question", tagline: "For one focused question." }, 2),
    pack({ id: "p-starter", code: "starter_315", name: "Starter", credits: 315, price_cents: 4000, tagline: "A little more time for a fuller conversation.", badges: [{ type: "save", percent: 4 }] }, 3),
    pack({ id: "p-popular", code: "popular_675", name: "Most Popular", credits: 675, price_cents: 8000, featured: true, tagline: "The best balance of price and reading time.", badges: [{ type: "most_popular" }, { type: "save", percent: 11 }] }, 4),
    pack({ id: "p-deep", code: "deep_1350", name: "Deep Dive", credits: 1350, price_cents: 15000, tagline: "For multiple questions or a longer reading.", badges: [{ type: "save", percent: 16 }] }, 5),
    pack({ id: "p-premium", code: "premium_2400", name: "Premium", credits: 2400, price_cents: 25000, tagline: "Our best ongoing rate for regular readings.", badges: [{ type: "best_value" }, { type: "save", percent: 21 }] }, 6),
  ],
  generated_at: "2026-11-12T00:00:00Z",
  ...over,
});
