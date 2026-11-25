import type { Metadata } from "next";
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
import advisors from "@/data/advisors.json";
import { getDictionary } from "@/i18n/dictionaries";
import { createTranslateCopy, type TranslateCopy } from "@/i18n/translate-copy";
import type { Locale } from "@/i18n/config";
// growth (seo): page metadata, link previews and structured data.
import { articleKindForCalculator, getCalculatorArticle, type CalculatorRouteKind } from "@/content/calculators";
import { SEO_COMMON } from "@/content/seo/names";
import { JsonLd } from "@/lib/seo/json-ld";
import { breadcrumbJsonLd, faqJsonLd, type JsonLdObject } from "@/lib/seo/jsonld";
import { ADVISOR_DESCRIPTION, AUTH_ROUTES, DEFAULT_IMAGE_ALT, PAGE_SEO } from "@/lib/seo/pages";
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
/** An advisor profile slug (`/psychics/<slug>`), as the page accepts it. */
const advisorSlug = /^(?=.{1,64}$)[a-z0-9]+(-[a-z0-9]+)*$/;
/** "maria-luz" → "Maria Luz". */
function nameFromSlug(slug: string): string {
  return slug
    .split("-")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
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
  // growth (seo): advisors come from the API; a profile missing from the static list is named after its slug.
  const advisor =
    screen[0] === "psychics" && screen.length === 2 && advisorSlug.test(screen[1])
      ? (advisors.find((item) => item.slug === screen[1]) ?? { slug: screen[1], name: nameFromSlug(screen[1]) })
      : undefined;
  const base = route.replace(/\/results$/, "");
  const title = advisor?.name ?? text(PAGE_SEO[base]?.title ?? titles[base] ?? "Home");
  // revisions 6.0: the invoice and the payment return page are private.
  if (invoiceRoute(screen)) return { title: text("Invoice"), robots: NOINDEX, referrer: "no-referrer" };
  if (route === "credits/return") return { title, robots: NOINDEX, referrer: "no-referrer" };
  // One-time links: never indexed, and the URL (with its token) is never
  // sent as a referrer.
  if (route in emailLinks || route === "auth/google" || route === "age-restricted" || route === "unsubscribe")
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
  // growth (seo): the account, the sign-in steps and the design's sample results are not indexed.
  if (route in accounts || route === "credits" || AUTH_ROUTES.has(route)) return { title, robots: NOINDEX_FOLLOW };
  if (screen[0] === "calculators" && screen.at(-1) === "results")
    return { title: `${title} — ${text("Results")}`, robots: NOINDEX_FOLLOW };
  // growth (seo): indexable pages get a keyword-aware title and description, a canonical URL,
  // absolute hreflang alternates (with x-default) and a link preview.
  const seo = PAGE_SEO[route];
  if (seo)
    return pageMetadata({ locale: lang, route: `/${route}`, title, description: text(seo.description), imageAlt: text(DEFAULT_IMAGE_ALT) });
  if (advisor)
    return pageMetadata({
      locale: lang,
      route: `/${route}`,
      title,
      description: text(ADVISOR_DESCRIPTION, { value0: advisor.name }),
      imageAlt: text(DEFAULT_IMAGE_ALT),
    });
  return { title, alternates: pageAlternates(`/${route}`, lang) };
}

/**
 * growth (seo): BreadcrumbList for the indexable pages of this route, and
 * FAQPage for a calculator's article (its questions and answers are on the page).
 */
function screenJsonLd(route: string, lang: Locale, text: TranslateCopy): JsonLdObject[] {
  const seo = PAGE_SEO[route];
  if (!seo) return [];
  const data = [
    breadcrumbJsonLd(lang, [
      { name: SEO_COMMON[lang].home, route: "" },
      { name: text(seo.title), route: `/${route}` },
    ]),
  ];
  const kind = route.startsWith("calculators/") ? route.slice("calculators/".length) : "";
  if (kind in articleKindForCalculator) {
    const faq = getCalculatorArticle(kind as CalculatorRouteKind, lang)
      .sections.flatMap((section) => section.blocks)
      .flatMap((block) => (block.type === "faq" ? block.items : []));
    if (faq.length) data.push(faqJsonLd(faq));
  }
  return data;
}

export default async function ScreenPage({ params }: RouteProps) {
  const { lang, screen } = await params;
  if (!isLocale(lang)) notFound();
  const route = screen.join("/");
  const page = screenPage(lang, screen, route);
  const data = PAGE_SEO[route] ? screenJsonLd(route, lang, createTranslateCopy((await getDictionary(lang)).copy)) : [];
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
  if (route in content) {
    const Content = content[route as keyof typeof content];
    return (
      <PublicFrame>
        <Content />
      </PublicFrame>
    );
  }
  if (route in legal) {
    const Legal = legal[route as keyof typeof legal];
    return (
      <PublicFrame>
        <Legal locale={lang} />
      </PublicFrame>
    );
  }
  // revisions 5.0: server-rendered from GET /pricing.
  if (route === "pricing") return <PricingScreen locale={lang} />;
  if (route in auth) return <AuthFlow key={route} step={auth[route]} />;
  if (route in accounts) return <Account key={route} view={accounts[route]} />;
  if (route in emailLinks) return <EmailLinkPage key={route} action={emailLinks[route]} />;
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
  if (screen[0] === "psychics" && screen.length === 2 && advisorSlug.test(screen[1]))
    return <PsychicProfile key={route} slug={screen[1]} />;
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
