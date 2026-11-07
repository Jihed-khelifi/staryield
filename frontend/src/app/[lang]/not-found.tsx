"use client";

import Link from "next/link";

import { useI18n } from "@/i18n/i18n-provider";
import { PublicFrame } from "@/components/layout/public-frame";
import { usePsychicsPath } from "@/lib/psychics-link";
import { Button } from "@/components/ui/button";

/**
 * 404 for unknown URLs under a locale (`notFound()` from any page). Renders
 * inside the locale layout, so it is translated and keeps the site chrome;
 * it offers the main ways back into the site.
 */
export default function NotFound() {
  const { text, locale } = useI18n();
  const psychics = usePsychicsPath();
  const links: [string, string][] = [
    ["Psychics", psychics],
    ["Natal Chart", "/calculators/natal-chart"],
    ["Path of Life", "/calculators/path-of-life"],
    ["Astrology", "/calculators/astrology"],
    ["Contact Us", "/contact"],
  ];
  return (
    <PublicFrame>
      <title>{`${text("Page not found")} | ${text("Staryield")}`}</title>
      <section
        data-testid="not-found"
        aria-labelledby="not-found-title"
        className="paper-page flex min-h-[60vh] flex-col items-center justify-center gap-5 px-6 py-20 text-center"
      >
        <p aria-hidden className="font-display text-7xl text-gold">
          404
        </p>
        <h1 id="not-found-title" className="font-display text-3xl md:text-4xl">
          {text("Page not found")}
        </h1>
        <p className="max-w-md font-light">{text("We could not find what you were looking for.")}</p>
        <Button asChild variant="cta" size="xl">
          <Link href={`/${locale}`}>{text("Back to home")}</Link>
        </Button>
        <nav aria-label={text("Helpful links")} className="mt-2">
          <ul className="flex flex-wrap items-center justify-center gap-x-6 gap-y-2 text-sm">
            {links.map(([label, href]) => (
              <li key={href}>
                <Link className="underline underline-offset-4 hover:text-ink/70" href={`/${locale}${href}`}>
                  {text(label)}
                </Link>
              </li>
            ))}
          </ul>
        </nav>
      </section>
    </PublicFrame>
  );
}
