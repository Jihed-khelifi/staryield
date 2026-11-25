import type { Metadata } from "next";
import type { ComponentType } from "react";
import { notFound, permanentRedirect } from "next/navigation";
import About from "@/components/content/about";
import Reviews from "@/components/content/reviews";
import Standards from "@/components/content/standards";
import Faq from "@/components/content/faq";
import Disclaimer from "@/components/content/disclaimer";
import Terms from "@/components/content/terms";
import Privacy from "@/components/content/privacy";
import { PublicFrame } from "@/components/layout/public-frame";
import { AuthFlow, type AuthStep } from "@/components/forms/auth-flow";
import { GoogleReturn } from "@/components/forms/auth/google-return";
import { ContactForm } from "@/components/forms/contact-form";
import { PsychicsBrowse, PsychicProfile } from "@/components/psychics/psychics";
import {
  Calculator,
  type CalculatorKind,
} from "@/components/calculators/calculator";
import { CalculatorArticle } from "@/components/content/article/calculator-article";
import { Account, type AccountView } from "@/components/account/account";
import { CreditsRoute } from "@/components/billing/top-up/credits-route"; // revisions 5.0: top-up replaces the old checkout
// revisions 6.0 (§26.5, §26.6): the payment redirect return page and the invoice page.
import { CreditsReturn } from "@/components/billing/top-up/credits-return";
import { InvoicePage } from "@/components/billing/invoice-page";
// revisions 5.0: 18+ age gate (§24.6).
import { AgeRestricted } from "@/components/account/age-restricted";
import { AgeGuard } from "@/lib/auth/age-gate";
// account settings: one-time e-mail link pages (tokens are read client-side, then stripped).
import { EmailLinkPage, type EmailLinkAction } from "@/components/account/email-link-page";
import { LinkTicketPage, MyTicketPage, SupportRequestsPage } from "@/components/support/support-pages";
import { UnsubscribePage } from "@/components/marketing/unsubscribe-page"; // revisions 6.0 (§26.4)
// revisions 5.0: public pricing page and the Refund Policy (docs/platform-extensions.md §24.8).
import RefundPolicy from "@/components/content/refund-policy";
import { PricingScreen } from "@/components/pricing/pricing-screen";
import { isLocale } from "@/i18n/config";
import { isRefundAlias } from "@/lib/legal/refund-aliases"; // revisions 5.1
import { getDictionary } from "@/i18n/dictionaries";
import { createTranslateCopy, type TranslateCopy } from "@/i18n/translate-copy";
import type { Locale } from "@/i18n/config";
// growth (seo): page metadata, link previews and structured data.
import { ADVISOR_SLUG, fetchAdvisor } from "@/lib/psychics/server";
import { JsonLd } from "@/lib/seo/json-ld";
import { screenJsonLd } from "@/lib/seo/screen-json-ld";
import { ADVISOR_DESCRIPTION, AUTH_ROUTES, DEFAULT_IMAGE_ALT, pageSeo } from "@/lib/seo/pages";
import { NOINDEX, NOINDEX_FOLLOW, pageAlternates, pageMetadata } from "@/lib/seo/site";

const content = {
  about: About,
  reviews: Reviews,
  standards: Standards,
  faq: Faq,
  disclaimer: Disclaimer,
};
// Legal documents are server-rendered from src/content/legal and need the locale.
const legal = { terms: Terms, privacy: Privacy, "refund-policy": RefundPolicy }; // revisions 5.0: refund-policy
const auth: Record<string, AuthStep> = {
  signup: "email",
  "signup/password": "password",
  "signup/birth": "birth",
  "signup/offer": "offer",
  login: "login",
  "forgot-password": "reset",
  "reset-password": "newPassword",
};
const accounts: Record<string, AccountView> = {
  profile: "overview",
  "profile/chart": "chart",
  "profile/settings": "settings",
  "profile/analytics": "analytics",
};
const emailLinks: Record<string, EmailLinkAction> = {
  "confirm-email": "confirm-email",
  "cancel-email-change": "cancel-email-change",
  "keep-account": "keep-account",
};
const calculatorKinds: CalculatorKind[] = [
  "natal-chart",
  "path-of-life",
  "astrology",
];
type RouteProps = { params: Promise<{ lang: string; screen: string[] }> };

const supportReference = /^SY-[A-Za-z0-9]{4,16}$/;
/** The advisor slug of a profile route (`/psychics/<slug>`), else null. */
function advisorRoute(screen: string[]): string | null {
  return screen.length === 2 && screen[0] === "psychics" && ADVISOR_SLUG.test(screen[1]) ? screen[1] : null;
}
/**
 * `map[key]` only when the map itself defines `key`: a route such as
 * "constructor" or "toString" must never resolve to Object.prototype.
 */
function own<T>(map: Record<string, T>, key: string): T | undefined {
  return Object.hasOwn(map, key) ? map[key] : undefined;
}
/** revisions 6.0 (§26.6): `/profile/invoices/<id>` (ids are UUIDs). */
const invoiceId = /^[A-Za-z0-9-]{8,64}$/;
function invoiceRoute(screen: string[]): string | null {
  return screen.length === 3 && screen[0] === "profile" && screen[1] === "invoices" && invoiceId.test(screen[2]) ? screen[2] : null;
}

/** The support desk routes: list, signed-in thread and the anonymous private link. */
function supportRoute(screen: string[]): { kind: "list" } | { kind: "mine" | "link"; reference: string } | null {
  if (screen[0] !== "support") return null;
  if (screen.length === 2 && screen[1] === "requests") return { kind: "list" };
  if (screen.length === 3 && screen[1] === "requests" && supportReference.test(screen[2])) return { kind: "mine", reference: screen[2] };
  if (screen.length === 2 && supportReference.test(screen[1])) return { kind: "link", reference: screen[1] };
  return null;
}

export async function generateMetadata({
  params,
}: RouteProps): Promise<Metadata> {
  const { lang, screen } = await params;
  if (!isLocale(lang)) return {};
  const text = createTranslateCopy((await getDictionary(lang)).copy);
  const route = screen.join("/");
  if (screen.length === 1 && isRefundAlias(screen[0])) return { robots: NOINDEX }; // revisions 5.1: redirect only
  // Pages that are not indexed keep a plain title (the indexable ones are described by PAGE_SEO).
  const titles: Record<string, string> = {
    psychics: "Psychics",
    profile: "Profile",
    "profile/chart": "Natal Chart",
    "profile/settings": "Settings",
    "profile/analytics": "Analytics",
    credits: "Credits",
    "credits/return": "Payment", // revisions 6.0
    signup: "Sign up",
    "signup/password": "Create password",
    "signup/birth": "Date of birth",
    "signup/offer": "Sign up",
    login: "Log in",
    "forgot-password": "Forgot password",
    "reset-password": "Choose a new password",
    "auth/google": "Log in",
    "confirm-email": "Confirm your email",
    "cancel-email-change": "Email change",
    "keep-account": "Keep your account",
    unsubscribe: "Unsubscribe", // revisions 6.0 (§26.4)
    "support/requests": "Support requests",
    "age-restricted": "Account restricted", // revisions 5.0
  };
  const base = route.replace(/\/results$/, "");
  const title = text(pageSeo(base)?.title ?? own(titles, base) ?? "Home");
  // revisions 6.0: the invoice and the payment return page are private.
  if (invoiceRoute(screen)) return { title: text("Invoice"), robots: NOINDEX, referrer: "no-referrer" };
  if (route === "credits/return") return { title, robots: NOINDEX, referrer: "no-referrer" };
  // One-time links: never indexed, and the URL (with its token) is never
  // sent as a referrer.
  if (Object.hasOwn(emailLinks, route) || route === "auth/google" || route === "age-restricted" || route === "unsubscribe")
    return { title, referrer: "no-referrer", robots: NOINDEX };
  // Support desk (requester side): signed-in pages and the private link are never indexed;
  // the private link also never leaks its URL as a referrer.
  const support = supportRoute(screen);
  if (support) {
    const title = text(support.kind === "list" ? "Support requests" : "Support request");
    return support.kind === "link"
      ? { title, referrer: "no-referrer", robots: NOINDEX }
      : { title, robots: NOINDEX };
  }
  // The directory is for signed-in customers (visitors are redirected home).
  if (route === "psychics") return { title, robots: { index: false } };
  const slug = advisorRoute(screen);
  if (slug) return advisorMetadata(lang, route, slug, text);
  // growth (seo): the account, the sign-in steps and the design's sample results are not indexed.
  if (Object.hasOwn(accounts, route) || route === "credits" || AUTH_ROUTES.has(route)) return { title, robots: NOINDEX_FOLLOW };
  if (screen[0] === "calculators" && screen.at(-1) === "results")
    return { title: `${title} — ${text("Results")}`, robots: NOINDEX_FOLLOW };
  // growth (seo): indexable pages get a keyword-aware title and description, a canonical URL,
  // absolute hreflang alternates (with x-default) and a link preview.
  const seo = pageSeo(route);
  if (seo)
    return pageMetadata({ locale: lang, route: `/${route}`, title, description: text(seo.description), imageAlt: text(DEFAULT_IMAGE_ALT) });
  return { title, alternates: pageAlternates(`/${route}`, lang) };
}

/**
 * growth (seo): an advisor profile is described only with what the API
 * confirms (never words taken from the URL): the advisor's own name, and
 * their photo as the link preview. A slug the API does not know is a 404;
 * when the API cannot answer, the page stays out of the index until it can.
 */
async function advisorMetadata(lang: Locale, route: string, slug: string, text: TranslateCopy): Promise<Metadata> {
  const lookup = await fetchAdvisor(slug);
  if (lookup?.found === false) notFound();
  if (!lookup) return { title: text("Advisor profile"), robots: NOINDEX_FOLLOW };
  const { advisor } = lookup;
  const photo = /^https:\/\//.test(advisor.image_url) ? { url: advisor.image_url } : undefined;
  return pageMetadata({
    locale: lang,
    route: `/${route}`,
    title: advisor.name,
    description: text(ADVISOR_DESCRIPTION, { value0: advisor.name }),
    image: photo,
    imageAlt: photo ? advisor.name : text(DEFAULT_IMAGE_ALT),
  });
}

export default async function ScreenPage({ params }: RouteProps) {
  const { lang, screen } = await params;
  if (!isLocale(lang)) notFound();
  const route = screen.join("/");
  // growth (seo): a profile the API does not know is a real 404, decided before anything streams.
  const slug = advisorRoute(screen);
  if (slug && (await fetchAdvisor(slug))?.found === false) notFound();
  const page = screenPage(lang, screen, route);
  const data = pageSeo(route) ? screenJsonLd(route, lang, createTranslateCopy((await getDictionary(lang)).copy)) : [];
  return data.length ? (
    <>
      <JsonLd data={data} />
      {page}
    </>
  ) : (
    page
  );
}

function screenPage(lang: Locale, screen: string[], route: string) {
  // revisions 5.1: fallback for the refund aliases (the proxy normally redirects them first), §25.6.
  if (screen.length === 1 && isRefundAlias(screen[0])) permanentRedirect(`/${lang}/refund-policy`);
  const Content = own<ComponentType>(content, route);
  if (Content) {
    return (
      <PublicFrame>
        <Content />
      </PublicFrame>
    );
  }
  const Legal = own<ComponentType<{ locale: Locale }>>(legal, route);
  if (Legal) {
    return (
      <PublicFrame>
        <Legal locale={lang} />
      </PublicFrame>
    );
  }
  // revisions 5.0: server-rendered from GET /pricing.
  if (route === "pricing") return <PricingScreen locale={lang} />;
  const step = own(auth, route);
  if (step) return <AuthFlow key={route} step={step} />;
  const view = own(accounts, route);
  if (view) return <Account key={route} view={view} />;
  const action = own(emailLinks, route);
  if (action) return <EmailLinkPage key={route} action={action} />;
  // Google sign-in: where the API sends the browser back (result in the query, read client-side).
  if (route === "auth/google") return <GoogleReturn key={route} />;
  // revisions 6.0 (§26.4): the unsubscribe link of the report e-mail (token read client-side, then stripped).
  if (route === "unsubscribe") return <UnsubscribePage key={route} />;
  // revisions 5.0: the screen of an account restricted to adults (§24.6).
  if (route === "age-restricted") return <AgeRestricted key={route} />;
  if (route === "contact") return <ContactForm />;
  const support = supportRoute(screen);
  if (support?.kind === "list") return <SupportRequestsPage />;
  if (support?.kind === "mine") return <MyTicketPage key={route} reference={support.reference} />;
  if (support?.kind === "link") return <LinkTicketPage key={route} reference={support.reference} />;
  if (route === "psychics") return <PsychicsBrowse />;
  // Advisors come from the API; any well-formed slug is resolved client-side
  // (unknown slugs render the not-found state).
  const advisor = advisorRoute(screen);
  if (advisor) return <PsychicProfile key={route} slug={advisor} />;
  const invoice = invoiceRoute(screen);
  if (invoice) return <InvoicePage key={invoice} id={invoice} />;
  if (route === "credits/return") return <CreditsReturn key={route} />;
  if (route === "credits")
    return (
      // revisions 5.0: top-ups are for adults (§24.6): no date of birth sends the person to the birth step.
      <AgeGuard>
        <CreditsRoute />
      </AgeGuard>
    );
  if (
    screen[0] === "calculators" &&
    calculatorKinds.includes(screen[1] as CalculatorKind) &&
    (screen.length === 2 || (screen.length === 3 && screen[2] === "results"))
  )
    return (
      <Calculator
        key={route}
        kind={screen[1] as CalculatorKind}
        showResults={screen[2] === "results"}
        article={
          <CalculatorArticle
            kind={screen[1] as CalculatorKind}
            locale={lang}
          />
        }
      />
    );
  notFound();
}
