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
import { ContactForm } from "@/components/forms/contact-form";
import { PsychicsBrowse, PsychicProfile } from "@/components/psychics/psychics";
import {
  Calculator,
  type CalculatorKind,
} from "@/components/calculators/calculator";
import { Account, type AccountView } from "@/components/account/account";
import { CreditCheckout } from "@/components/account/credit-checkout";
import { isLocale } from "@/i18n/config";
import advisors from "@/data/advisors.json";

const content = {
  about: About,
  reviews: Reviews,
  standards: Standards,
  faq: Faq,
  disclaimer: Disclaimer,
  terms: Terms,
  privacy: Privacy,
};
const auth: Record<string, AuthStep> = {
  signup: "email",
  "signup/password": "password",
  "signup/birth": "birth",
  "signup/offer": "offer",
  login: "login",
  "forgot-password": "reset",
};
const accounts: Record<string, AccountView> = {
  profile: "overview",
  "profile/chart": "chart",
  "profile/settings": "settings",
};
const calculatorKinds: CalculatorKind[] = [
  "natal-chart",
  "path-of-life",
  "astrology",
];
type RouteProps = { params: Promise<{ lang: string; screen: string[] }> };

export async function generateMetadata({
  params,
}: RouteProps): Promise<Metadata> {
  const { screen } = await params;
  return {
    title: screen
      .at(-1)!
      .split("-")
      .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
      .join(" "),
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
  if (route in auth) return <AuthFlow key={route} step={auth[route]} />;
  if (route in accounts) return <Account key={route} view={accounts[route]} />;
  if (route === "contact") return <ContactForm />;
  if (route === "psychics") return <PsychicsBrowse />;
  if (
    screen[0] === "psychics" &&
    screen.length === 2 &&
    advisors.some((advisor) => advisor.slug === screen[1])
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
      />
    );
  notFound();
}
