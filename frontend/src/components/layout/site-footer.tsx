"use client";
/* eslint-disable @next/next/no-img-element */
import Link from "next/link";
import { useI18n } from "@/i18n/i18n-provider";
import { usePsychicsPath } from "@/lib/psychics-link";
import { SunLogo } from "@/components/brand/sun-logo";
import { visibleSocialLinks } from "@/config/social";
import { TrustpilotReviewCollector } from "@/components/reviews/trustpilot-review-collector";
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
      ["Full Disclaimer", "/disclaimer"],
    ],
  },
  {
    name: "Staryield",
    links: [
      ["About Us", "/about"],
      ["Staryield Reviews", "/reviews"],
      ["Advisor Professional Integrity & Interaction Standards", "/standards"],
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
              "Professional spiritual guidance built with analytical clarity and strict privacy standards. ",
            )}
          </p>
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
        <p>
          {text(
            "© 2026 Staryield Media LLC. All rights reserved. For entertainment and alignment audits only. ",
          )}
        </p>
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
