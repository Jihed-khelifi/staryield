"use client";
/* eslint-disable @next/next/no-img-element */
import Link from "next/link";
import { useI18n } from "@/i18n/i18n-provider";
import { usePsychicsPath } from "@/lib/psychics-link";
import { SunLogo } from "@/components/brand/sun-logo";
import { visibleSocialLinks } from "@/config/social";
import { TrustpilotReviewCollector } from "@/components/reviews/trustpilot-review-collector";
import { AcceptedCards } from "@/components/billing/accepted-cards"; // revisions 5.1
import { COMPANY, supportMailto } from "@/config/company"; // revisions 5.1
const groups = [
  {
    name: "Platform",
    links: [
      ["Browse Psychics", "/psychics"],
      ["Natal Charts", "/calculators/natal-chart"],
      ["Path of life", "/calculators/path-of-life"],
      ["Astrology", "/calculators/astrology"],
      ["Sign Up | Sign In", "/signup"],
    ],
  },
  {
    name: "Support",
    links: [
      ["FAQ", "/faq"],
      ["Terms of Use", "/terms"],
      ["Privacy Policy", "/privacy"],
      ["Contact Us", "/contact"],
      ["Refund policy", "/refund-policy"], // revisions 5.0
      ["Full Disclaimer", "/disclaimer"],
    ],
  },
  {
    name: "Staryield",
    links: [
      ["About Us", "/about"],
      ["Staryield Reviews", "/reviews"],
      ["Advisor Integrity & Interaction Standards", "/standards"],
      ["Pricing", "/pricing"], // revisions 5.0
    ],
  },
];
export function SiteFooter() {
  const { text, locale } = useI18n();
  const psychics = usePsychicsPath();
  return (
    <footer className="site-footer paper-page">
      <div className="footer-columns">
        <div>
          <Link href={`/${locale}`} className="brand">
            <SunLogo />
            <span>{text("Staryield")}</span>
          </Link>
          <p className="mt-4 max-w-80 text-sm font-light">
            {text(
              "Astrology and psychic readings for entertainment purposes only. ",
            )}
          </p>
          {/* revisions 5.1: company and customer-service details on every page with a footer (docs/platform-extensions.md §25.4). */}
          <address className="mt-4 max-w-80 text-sm font-light not-italic [overflow-wrap:anywhere]" data-testid="footer-company">
            <p className="font-medium" data-testid="footer-company-name">{COMPANY.legalName}</p>
            <p data-testid="footer-company-address">{COMPANY.addressLine}</p>
            <p>
              <a className="underline hover:no-underline" href={supportMailto()} data-testid="footer-support-email">
                {COMPANY.supportEmail}
              </a>
            </p>
            <p data-testid="footer-support-hours">{text("Support is available 24/7. We reply within 1–2 days.")}</p>
          </address>
          <AcceptedCards label={text("We accept")} className="mt-4" />
        </div>
        {groups.map((group) => (
          <nav key={group.name} aria-label={text(group.name)}>
            <h2 className="mb-4 text-lg">{text(group.name.toUpperCase())}</h2>
            <ul className="space-y-3 text-sm font-light">
              {group.links.map(([label, path]) => {
                const href = path === "/psychics" ? psychics : path;
                return (
                <li key={path}>
                  <Link className="hover:underline" href={`/${locale}${href}`}>
                    {text(label)}
                  </Link>
                </li>
                );
              })}
            </ul>
          </nav>
        ))}
      </div>
      <TrustpilotReviewCollector placement="footer" className="mx-auto mt-10 max-w-[360px]" />
      <div className="mt-8 flex flex-wrap items-center justify-between gap-6 border-t border-ink/60 pt-4 text-xs">
        <div className="flex flex-col gap-1">
          <p>
            {text(
              "© 2026 Staryield Media LLC. All rights reserved. For entertainment purposes only. ",
            )}
          </p>
          {/* revisions 5.0: adults only, entertainment (docs/platform-extensions.md §24.6). */}
          <p data-testid="footer-age-line">{text("18+ only · For entertainment purposes only.")}</p>
        </div>
        <nav aria-label={text("Social platforms")} data-testid="social-links">
          <ul className="flex items-end gap-1.25">
            {visibleSocialLinks().map((link) => (
              <li key={link.id}>
                <a
                  href={link.href}
                  target="_blank"
                  rel="noopener noreferrer"
                  aria-label={text("Staryield on {value0} (opens in a new tab)", { value0: link.label })}
                  className="block rounded-sm p-0.5 hover:opacity-70"
                >
                  <img src={link.icon} alt="" width={link.size} height={link.size} />
                </a>
              </li>
            ))}
          </ul>
        </nav>
      </div>
    </footer>
  );
}
