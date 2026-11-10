import type { Metadata } from "next";
import { notFound } from "next/navigation";
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
import { CreditCheckout } from "@/components/account/credit-checkout";
// account settings: one-time e-mail link pages (tokens are read client-side, then stripped).
import { EmailLinkPage, type EmailLinkAction } from "@/components/account/email-link-page";
import { LinkTicketPage, MyTicketPage, SupportRequestsPage } from "@/components/support/support-pages";
import { isLocale } from "@/i18n/config";
import advisors from "@/data/advisors.json";
import { getDictionary } from "@/i18n/dictionaries";
import { createTranslateCopy } from "@/i18n/translate-copy";
import { localePath, locales } from "@/i18n/config";

const content = {
  about: About,
  reviews: Reviews,
  standards: Standards,
  faq: Faq,
  disclaimer: Disclaimer,
};
// Legal documents are server-rendered from src/content/legal and need the locale.
const legal = { terms: Terms, privacy: Privacy };
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
  const titles: Record<string, string> = {
    about: "About",
    reviews: "Reviews",
    standards: "Standards",
    faq: "FAQ",
    disclaimer: "Disclaimer",
    terms: "Terms of Use",
    privacy: "Privacy Policy",
    contact: "Contact",
    psychics: "Psychics",
    profile: "Profile",
    "profile/chart": "Natal Chart",
    "profile/settings": "Settings",
    "profile/analytics": "Analytics",
    credits: "Credits",
    signup: "Sign up",
    "signup/password": "Create password",
    "signup/birth": "Date of birth",
    "signup/offer": "Sign up",
    login: "Log in",
    "forgot-password": "Forgot password",
    "reset-password": "Choose a new password",
    "calculators/natal-chart": "Natal Chart",
    "calculators/path-of-life": "Life Path",
    "calculators/astrology": "Astrology",
    "auth/google": "Log in",
    "confirm-email": "Confirm your email",
    "cancel-email-change": "Email change",
    "keep-account": "Keep your account",
    "support/requests": "Support requests",
  };
  const advisor =
    screen[0] === "psychics"
      ? advisors.find((item) => item.slug === screen[1])
      : undefined;
  const title =
    advisor?.name ?? text(titles[route.replace(/\/results$/, "")] ?? "Home");
  // One-time links: never indexed, and the URL (with its token) is never
  // sent as a referrer.
  if (route in emailLinks || route === "auth/google")
    return { title, referrer: "no-referrer", robots: { index: false, follow: false } };
  // Support desk (requester side): signed-in pages and the private link are never indexed;
  // the private link also never leaks its URL as a referrer.
  const support = supportRoute(screen);
  if (support) {
    const title = text(support.kind === "list" ? "Support requests" : "Support request");
    return support.kind === "link"
      ? { title, referrer: "no-referrer", robots: { index: false, follow: false } }
      : { title, robots: { index: false, follow: false } };
  }
  // The directory is for signed-in customers (visitors are redirected home).
  if (route === "psychics") return { title, robots: { index: false } };
  return {
    title:
      screen.at(-1) === "results" ? `${title} — ${text("Results")}` : title,
    alternates: {
      languages: Object.fromEntries(
        locales.map((locale) => [locale, localePath(`/${route}`, locale)]),
      ),
    },
  };
}
export default async function ScreenPage({ params }: RouteProps) {
  const { lang, screen } = await params;
  if (!isLocale(lang)) notFound();
  const route = screen.join("/");
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
  if (route in auth) return <AuthFlow key={route} step={auth[route]} />;
  if (route in accounts) return <Account key={route} view={accounts[route]} />;
  if (route in emailLinks) return <EmailLinkPage key={route} action={emailLinks[route]} />;
  // Google sign-in: where the API sends the browser back (result in the query, read client-side).
  if (route === "auth/google") return <GoogleReturn key={route} />;
  if (route === "contact") return <ContactForm />;
  const support = supportRoute(screen);
  if (support?.kind === "list") return <SupportRequestsPage />;
  if (support?.kind === "mine") return <MyTicketPage key={route} reference={support.reference} />;
  if (support?.kind === "link") return <LinkTicketPage key={route} reference={support.reference} />;
  if (route === "psychics") return <PsychicsBrowse />;
  // Advisors come from the API; any well-formed slug is resolved client-side
  // (unknown slugs render the not-found state).
  if (
    screen[0] === "psychics" &&
    screen.length === 2 &&
    /^[a-z0-9]+(-[a-z0-9]+)*$/.test(screen[1]) &&
    screen[1].length <= 64
  )
    return <PsychicProfile key={route} slug={screen[1]} />;
  if (route === "credits")
    return (
      <>
        <Account view="overview" />
        <CreditCheckout />
      </>
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
