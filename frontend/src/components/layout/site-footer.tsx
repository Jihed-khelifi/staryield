"use client";
/* eslint-disable @next/next/no-img-element */
import Link from "next/link";
import { useI18n } from "@/i18n/i18n-provider";
import { SunLogo } from "@/components/brand/sun-logo";
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
              {group.links.map(([label, href]) => (
                <li key={href}>
                  <Link className="hover:underline" href={`/${locale}${href}`}>
                    {text(label)}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
        ))}
      </div>
      <div className="mt-12 flex flex-wrap items-center justify-between gap-6 border-t border-ink/60 pt-4 text-xs">
        <p>
          {text(
            "© 2026 STARYIELD. All rights reserved. For entertainment and alignment audits only. ",
          )}
        </p>
        <div
          className="flex items-center gap-4"
          aria-label={text("Social platforms")}
        >
          <span
            aria-label={text("LinkedIn")}
            className="text-base font-semibold"
          >
            in
          </span>
          {[
            ["Instagram", "a5ec5.svg"],
            ["TikTok", "0487b.svg"],
            ["YouTube", "07613.svg"],
            ["X", "5d5a7.svg"],
          ].map(([name, file]) => (
            <img
              key={name}
              alt={text(name)}
              width={20}
              height={20}
              src={`https://assets.staryield.net/assets/figma/${file}`}
            />
          ))}
        </div>
      </div>
    </footer>
  );
}
