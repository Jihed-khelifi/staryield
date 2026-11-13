// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import fr from "@/i18n/dictionaries/fr.json";
import copyEn from "@/i18n/dictionaries/copy-en.json";
import copyFr from "@/i18n/dictionaries/copy-fr.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import type { PricingSummary } from "@/lib/api/pricing";
import { canonicalPricing } from "@/test/pricing-fixtures";
import { axeViolations } from "@/test/render";

const h = vi.hoisted(() => ({ session: { status: "anonymous", user: null } as { status: string; user: unknown }, summary: vi.fn() }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => h.session }));
vi.mock("next/link", () => ({ default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => <a href={href} {...rest}>{children}</a> }));
vi.mock("@/lib/api/pricing", async (importOriginal) => ({ ...(await importOriginal<typeof import("@/lib/api/pricing")>()), pricingApi: { summary: h.summary, catalog: vi.fn() } }));

import { PricingPage } from "@/components/pricing/pricing-page";

const dictionaries = { en: { ...en, copy: copyEn } as Dictionary, fr: { ...fr, copy: copyFr } as Dictionary };
function ui(initial: PricingSummary | null, locale: "en" | "fr" = "en") {
  return (
    <I18nProvider locale={locale} dictionary={dictionaries[locale]}>
      <PricingPage initial={initial} />
    </I18nProvider>
  );
}

beforeEach(() => {
  h.session = { status: "anonymous", user: null };
  h.summary.mockReset();
});
afterEach(() => cleanup());

const cards = () => screen.getAllByTestId("pack-card");

describe("PricingPage with the catalog", () => {
  it("renders the six packs in the API's canonical order", () => {
    render(ui(canonicalPricing()));
    expect(cards().map((card) => card.getAttribute("data-pack"))).toEqual(["welcome_600", "quick_150", "starter_315", "popular_675", "deep_1350", "premium_2400"]);
    expect(cards().map((card) => within(card).getByRole("heading", { level: 3 }).textContent)).toEqual(["Welcome Pack", "Quick Insight", "Starter", "Most Popular", "Deep Dive", "Premium"]);
  });

  it("formats price, credits and the approximate minutes from the live rate range", () => {
    render(ui(canonicalPricing()));
    const [welcome, , starter, , , premium] = cards();
    expect(welcome.textContent).toContain("$10");
    expect(welcome.textContent).toContain("USD · one time");
    expect(welcome.textContent).toContain("600 credits");
    expect(welcome.textContent).toContain("≈ 20 min. at 30 credits/min. · ≈ 10 min. at 60 credits/min.");
    expect(starter.textContent).toContain("≈ 10.5 min. at 30 credits/min. · ≈ 5.25 min. at 60 credits/min.");
    expect(premium.textContent).toContain("2,400 credits");
    expect(premium.textContent).toContain("≈ 80 min. at 30 credits/min. · ≈ 40 min. at 60 credits/min.");
  });

  it("computes the eyebrow from the badges, else the label, else nothing", () => {
    render(ui(canonicalPricing()));
    const text = cards().map((card) => card.querySelector("p")?.textContent ?? "");
    expect(text[0]).toBe("NEW CLIENT OFFER");
    expect(text[1]).toBe("One focused question");
    expect(text[2]).toBe("SAVE 4%");
    expect(text[3]).toBe("MOST POPULAR · SAVE 11%");
    expect(text[4]).toBe("SAVE 16%");
    expect(text[5]).toBe("BEST VALUE · SAVE 21%");
    const bare = canonicalPricing();
    bare.packs[1] = { ...bare.packs[1], label: "", badges: [] };
    cleanup();
    render(ui(bare));
    expect(within(cards()[1]).queryByText("One focused question")).toBeNull();
  });

  it("gives the Welcome Pack its own call to action and eligibility fine print; the others the trial line", () => {
    render(ui(canonicalPricing()));
    const [welcome, quick] = cards();
    expect(within(welcome).getByRole("link", { name: /Start With the Welcome Offer/ }).getAttribute("href")).toBe("/en/signup");
    expect(welcome.textContent).toContain("Available once per eligible new customer after sign-up. Limit one per customer and payment method.");
    expect(within(quick).getByRole("link", { name: /Get started/i }).getAttribute("href")).toBe("/en/signup");
    expect(quick.textContent).toContain("Start with 4 free minutes. No card required.");
  });

  it("uses the live trial minutes and the live rate range in the hero, the steps and the FAQ", () => {
    render(ui(canonicalPricing({ free_trial: { enabled: true, minutes: 7 }, rate_range: { min: 20, max: 50 } })));
    expect(screen.getByRole("link", { name: "Start 7 Free Minutes" }).getAttribute("href")).toBe("/en/signup");
    expect(within(screen.getByRole("list", { name: "Our commitments" })).getAllByRole("listitem")[0].textContent).toBe("✓ 7 minutes free");
    expect(screen.getByText(/Advisors charge between 20 and 50 credits per minute/)).toBeTruthy();
    expect(screen.getByText("Try 7 minutes free")).toBeTruthy();
    expect(screen.getByText(/Advisors charge 20–50 credits per minute/)).toBeTruthy();
    expect(screen.getByText("No. Your first 7 minutes require no payment method.")).toBeTruthy();
    expect(screen.getByRole("heading", { level: 2, name: "Start with 7 minutes free" })).toBeTruthy();
  });

  it("states the footnote with the savings basis from the reference pack", () => {
    render(ui(canonicalPricing()));
    const note = screen.getByTestId("pricing-footnote").textContent ?? "";
    expect(note).toContain("Minute estimates are approximate and depend on the advisor’s displayed rate and applicable billing increments. Credits are added after successful payment. Pack purchases are one-time payments.");
    expect(note).toContain("Savings compared with the Quick Insight pack rate.");
  });

  it("shows the Visa and Mastercard marks after the footnote (revisions 5.1)", () => {
    render(ui(canonicalPricing()));
    const marks = screen.getByTestId("accepted-cards");
    expect(within(marks).getByText("We accept")).toBeTruthy();
    expect(within(marks).getAllByRole("img").map((img) => img.getAttribute("alt"))).toEqual(["Visa", "Mastercard"]);
    expect(screen.getByTestId("pricing-footnote").compareDocumentPosition(marks) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("omits the savings basis when no pack shows a saving", () => {
    const summary = canonicalPricing();
    summary.packs = summary.packs.map((pack) => ({ ...pack, badges: pack.badges.filter((badge) => badge.type !== "save") }));
    render(ui(summary));
    expect(screen.getByTestId("pricing-footnote").textContent).not.toContain("Savings compared");
  });

  it("shows the five commitments and the FAQ with the refund answer linking to the Refund Policy", () => {
    render(ui(canonicalPricing()));
    const chips = screen.getByRole("list", { name: "Our commitments" });
    expect(within(chips).getAllByRole("listitem").map((li) => li.textContent?.replace("✓ ", ""))).toEqual(["4 minutes free", "One-time payments", "No subscriptions", "No automatic top-ups", "Advisor rates shown upfront"]);
    expect(screen.getByRole("heading", { level: 2, name: "Before your first session" })).toBeTruthy();
    expect(screen.getByTestId("faq-refund-link").getAttribute("href")).toBe("/en/refund-policy");
    expect(screen.getByText("What is your refund policy?").nextElementSibling?.textContent).toBe("Review our Refund Policy.");
  });

  it("renders the final call to action", () => {
    render(ui(canonicalPricing()));
    expect(screen.getByTestId("pricing-final-cta").textContent).toBe("Start My Free Reading");
    expect(screen.getByTestId("pricing-final-cta").getAttribute("href")).toBe("/en/signup");
    expect(screen.getByText("No card required. No automatic charges.")).toBeTruthy();
  });

  it("is translated (French wording and number formats)", () => {
    render(ui(canonicalPricing(), "fr"));
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe("Choisissez vos crédits. Choisissez votre conseiller. Gardez le contrôle.");
    expect(cards()[2].textContent).toContain("≈ 10,5 min à 30 crédits/min · ≈ 5,25 min à 60 crédits/min");
    expect(cards()[3].textContent).toContain("LE PLUS POPULAIRE · ÉCONOMISEZ 11 %");
    expect(screen.getByTestId("faq-refund-link").getAttribute("href")).toBe("/fr/refund-policy");
    expect(screen.getByText("Quelle est votre politique de remboursement ?")).toBeTruthy();
  });

  it("has no serious accessibility violations", async () => {
    const { container } = render(ui(canonicalPricing()));
    expect(await axeViolations(container)).toEqual([]);
  });
});

describe("PricingPage with the trial switched off", () => {
  const off = () => canonicalPricing({ free_trial: { enabled: false, minutes: 0 } });

  it("drops the trial chip and uses plain sign-up copy", () => {
    render(ui(off()));
    expect(screen.queryByText(/minutes free/)).toBeNull();
    expect(screen.queryByText("No card required for your free trial.")).toBeNull();
    expect(screen.getByTestId("pricing-hero-cta").textContent).toBe("Sign up");
    expect(screen.getByTestId("pricing-final-cta").textContent).toBe("Sign up");
    expect(screen.getByRole("heading", { level: 2, name: "Start your first reading" })).toBeTruthy();
    expect(cards()[1].textContent).toContain("Create a free account.");
    expect(cards()[1].textContent).not.toContain("free minutes");
    expect(screen.getByText("Do I need a card to sign up?")).toBeTruthy();
    expect(screen.getByText("Create a free account")).toBeTruthy();
    const chips = within(screen.getByRole("list", { name: "Our commitments" })).getAllByRole("listitem");
    expect(chips).toHaveLength(4);
  });

  it("keeps the Welcome Pack offer and the one-time purchase promise", () => {
    render(ui(off()));
    expect(cards()[0].textContent).toContain("Start With the Welcome Offer");
    expect(screen.getByText("No. Each credit pack is a one-time purchase.")).toBeTruthy();
  });
});

describe("PricingPage for a signed-in visitor", () => {
  it("sends pack buttons to the top-up page and trial buttons to the advisor directory", () => {
    h.session = { status: "authenticated", user: { id: "u1", role: "client" } };
    render(ui(canonicalPricing()));
    for (const card of cards()) expect(within(card).getByTestId("pack-cta").getAttribute("href")).toBe("/en/credits");
    expect(screen.getByTestId("pricing-hero-cta").getAttribute("href")).toBe("/en/psychics");
    expect(screen.getByTestId("pricing-final-cta").getAttribute("href")).toBe("/en/psychics");
    expect(within(cards()[1]).queryByText(/free minutes/)).toBeNull();
  });

  it("renders the signed-out destinations until the session is known (no hydration flash)", () => {
    h.session = { status: "loading", user: null };
    render(ui(canonicalPricing()));
    expect(screen.getByTestId("pricing-hero-cta").getAttribute("href")).toBe("/en/signup");
    expect(within(cards()[0]).getByTestId("pack-cta").getAttribute("href")).toBe("/en/signup");
  });
});

describe("PricingPage when the catalog is unavailable", () => {
  it("still renders the hero, the FAQ and the final call to action, with a notice and Retry instead of the grid", () => {
    render(ui(null));
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy();
    expect(screen.getByRole("heading", { level: 2, name: "Before your first session" })).toBeTruthy();
    expect(screen.getByTestId("pricing-final-cta")).toBeTruthy();
    expect(screen.getByTestId("pricing-unavailable").textContent).toContain("Our prices can’t be shown right now.");
    expect(screen.queryByTestId("pack-grid")).toBeNull();
    expect(screen.queryByText(/\$\d/)).toBeNull();
    expect(screen.queryByTestId("pricing-footnote")).toBeNull();
    // Without a catalog the copy stays neutral: no invented minutes or rates.
    expect(screen.queryByText(/between \d+ and \d+ credits/)).toBeNull();
    expect(screen.queryByText(/\d+–\d+ credits per minute/)).toBeNull();
    expect(screen.getByTestId("pricing-hero-cta").textContent).toBe("Sign up");
  });

  it("Retry refetches /pricing, shows a skeleton while loading, then the packs", async () => {
    let resolve!: (value: unknown) => void;
    h.summary.mockReturnValue(new Promise((r) => (resolve = r)));
    render(ui(null));
    fireEvent.click(screen.getByTestId("pricing-retry"));
    expect(h.summary).toHaveBeenCalledTimes(1);
    expect(await screen.findByTestId("pricing-skeleton")).toBeTruthy();
    expect(screen.getByTestId("pricing-retry").getAttribute("aria-disabled")).toBe("true");
    resolve(canonicalPricing());
    await waitFor(() => expect(cards()).toHaveLength(6));
    expect(screen.queryByTestId("pricing-unavailable")).toBeNull();
    expect(screen.getByTestId("pricing-hero-cta").textContent).toBe("Start 4 Free Minutes");
  });

  it("stays on the notice when the retry fails or returns an unusable body", async () => {
    h.summary.mockRejectedValueOnce(new Error("down"));
    render(ui(null));
    fireEvent.click(screen.getByTestId("pricing-retry"));
    await waitFor(() => expect(screen.queryByTestId("pricing-skeleton")).toBeNull());
    expect(screen.getByTestId("pricing-unavailable")).toBeTruthy();
    h.summary.mockResolvedValueOnce({ packs: [] });
    fireEvent.click(screen.getByTestId("pricing-retry"));
    await waitFor(() => expect(h.summary).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.queryByTestId("pricing-skeleton")).toBeNull());
    expect(screen.getByTestId("pricing-unavailable")).toBeTruthy();
    expect(screen.queryByTestId("pack-grid")).toBeNull();
  });

  it("has no serious accessibility violations", async () => {
    const { container } = render(ui(null));
    expect(await axeViolations(container)).toEqual([]);
  });
});
