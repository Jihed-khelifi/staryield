/* eslint-disable @next/next/no-img-element */
import * as React from "react";
import Link from "next/link";
import { ChevronRight, Info } from "lucide-react";

import { localePath, type Locale } from "@/i18n/config";
import { getDictionary } from "@/i18n/dictionaries";
import { createTranslateCopy } from "@/i18n/translate-copy";
import type { HubGroup, HubItem, SeoBlock, SeoLink, SeoPage } from "@/content/seo/types";
import { breadcrumbJsonLd, faqJsonLd } from "@/lib/seo/jsonld";
import { JsonLd } from "@/lib/seo/json-ld";
import { InlineText } from "@/components/content/legal/inline-text";
import { PublicFrame } from "@/components/layout/public-frame";
import { AdvisorCta } from "@/components/seo/advisor-cta";
import { FAMILY_COPY, type SeoFamily } from "@/components/seo/copy";

/** The id of the calculator section; its heading is `${CALCULATOR_ID}-title`. */
export const CALCULATOR_ID = "seo-calculator";

const linkClass =
  "font-semibold text-[#6b4f1d] underline decoration-[#c29a3d] underline-offset-2 hover:text-ink focus-visible:rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#c29a3d]";

function Chips({ links, locale }: { links: SeoLink[]; locale: Locale }) {
  return (
    <ul className="flex flex-wrap gap-3">
      {links.map((link) => (
        <li key={link.href}>
          <Link
            href={localePath(link.href, locale)}
            className="inline-flex min-h-11 flex-col justify-center rounded-lg border border-warm-gray bg-cream-light/70 px-4 py-2 text-[15px] leading-snug font-semibold text-ink transition-colors hover:border-gold hover:bg-cream-light focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#c29a3d]"
          >
            <span>{link.label}</span>
            {link.note ? <span className="text-xs font-normal text-[var(--muted-ink)]">{link.note}</span> : null}
          </Link>
        </li>
      ))}
    </ul>
  );
}

function Block({ block, locale }: { block: SeoBlock; locale: Locale }) {
  switch (block.type) {
    case "p":
      return (
        <p>
          <InlineText text={block.text} locale={locale} />
        </p>
      );
    case "list": {
      const List = block.ordered ? "ol" : "ul";
      return (
        <List className={`space-y-2 ps-6 ${block.ordered ? "list-decimal" : "list-disc"} marker:text-[#8a6a1f]`}>
          {block.items.map((item, index) => (
            <li key={index} className="ps-1">
              <InlineText text={item} locale={locale} />
            </li>
          ))}
        </List>
      );
    }
    case "facts":
      return (
        <dl
          className={`grid gap-x-8 gap-y-4 rounded-xl border border-warm-gray bg-cream-light/60 p-5 md:p-6 ${block.columns === 3 ? "grid-cols-2 sm:grid-cols-3" : "sm:grid-cols-2"}`}
          data-testid="seo-facts"
        >
          {block.items.map((item) => (
            <div key={item.label} className="flex flex-col gap-0.5">
              <dt className="calc-label">{item.label}</dt>
              <dd className="text-[17px] font-semibold">{item.value}</dd>
            </div>
          ))}
        </dl>
      );
    case "links":
      return <Chips links={block.items} locale={locale} />;
    case "indicators":
      return (
        <div className="flex flex-col gap-4">
          <dl className="grid gap-3 rounded-xl border border-warm-gray bg-cream-light/60 p-5 md:p-6" data-testid="seo-indicators">
            {block.items.map((item) => (
              <div key={item.key} className="flex flex-wrap items-center justify-between gap-x-6 gap-y-1" data-indicator={item.key}>
                <dt className="font-display text-base tracking-[1.2px] uppercase">{item.label}</dt>
                <dd className="flex items-center gap-3">
                  <span aria-hidden className="flex gap-1.5">
                    {Array.from({ length: 5 }, (_, index) => (
                      <span
                        key={index}
                        className={`size-3.5 rounded-full border border-[#8a6a1f] ${index < item.value ? "bg-[#8a6a1f]" : "bg-transparent"}`}
                      />
                    ))}
                  </span>
                  <span className="text-sm font-semibold tabular-nums">{item.valueText}</span>
                </dd>
              </div>
            ))}
          </dl>
          <p className="text-sm leading-normal text-[var(--muted-ink)]">{block.note}</p>
        </div>
      );
  }
}

function HubTile({ item, locale }: { item: HubItem; locale: Locale }) {
  return (
    <li>
      <Link
        href={localePath(item.href, locale)}
        className="flex h-full flex-col items-center gap-3 rounded-xl border border-warm-gray bg-cream-light/60 p-4 text-center transition-colors hover:border-gold hover:bg-cream-light focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#c29a3d] md:p-5"
      >
        {item.image ? (
          <img src={item.image.src} alt="" width={item.image.width} height={item.image.height} loading="lazy" decoding="async" className="h-auto w-full max-w-[120px] rounded-md" />
        ) : item.numeral ? (
          <span aria-hidden className="font-display text-[44px] leading-none text-[#8a6a1f] md:text-[52px]">
            {item.numeral}
          </span>
        ) : null}
        <span className="font-display text-[15px] tracking-[0.8px] uppercase md:text-base">{item.title}</span>
        <span className="text-sm leading-snug text-[var(--muted-ink)]">{item.subtitle}</span>
      </Link>
    </li>
  );
}

function HubGroupCard({ group, locale }: { group: HubGroup; locale: Locale }) {
  return (
    <li className="flex flex-col gap-3 rounded-xl border border-warm-gray bg-cream-light/60 p-5">
      <h3 className="flex items-center gap-3 font-display text-lg tracking-[0.8px] uppercase">
        <img src={group.image.src} alt="" width={28} height={28} loading="lazy" decoding="async" className="size-7" />
        {group.title}
      </h3>
      <ul className="grid grid-cols-2 gap-x-4 gap-y-1.5 text-[15px]">
        {group.links.map((link) => (
          <li key={link.href} className="flex flex-col">
            <Link href={localePath(link.href, locale)} className={linkClass}>
              {link.short ? (
                <>
                  <span className="sr-only">{link.label}</span>
                  <span aria-hidden>{link.short}</span>
                </>
              ) : (
                link.label
              )}
            </Link>
            {link.note ? <span className="text-xs text-[var(--muted-ink)]">{link.note}</span> : null}
          </li>
        ))}
      </ul>
    </li>
  );
}

/**
 * A programmatic search page (Life Path number, tarot birth card, sign
 * pairing, or the hub of one of them). Server component: the whole text is
 * in the first HTML response, the breadcrumbs and FAQ are also given as
 * JSON-LD (built from the very same data), and the calculator is a client
 * slot — at the top of a hub, at the very end of a detail page.
 */
export async function SeoArticle({
  page,
  locale,
  family,
  calculator,
}: {
  page: SeoPage;
  locale: Locale;
  family: SeoFamily;
  calculator: React.ReactNode;
}) {
  const text = createTranslateCopy((await getDictionary(locale)).copy);
  const copy = FAMILY_COPY[family];
  const hub = Boolean(page.grid);
  const calculatorSection = (
    <section
      id={CALCULATOR_ID}
      aria-labelledby={`${CALCULATOR_ID}-title`}
      className="calc-paper scroll-mt-24 px-5 py-14 md:px-20 md:py-20"
      data-testid="seo-calculator"
    >
      <div className="mx-auto mb-8 flex max-w-[760px] flex-col items-center gap-3 text-center">
        <h2 id={`${CALCULATOR_ID}-title`} className="font-display text-[24px] leading-tight tracking-[0.88px] uppercase md:text-[32px]">
          <span aria-hidden="true">✦ </span>
          {text(copy.calculator.heading)}
          <span aria-hidden="true"> ✦</span>
        </h2>
        <p className="text-base leading-normal font-light">{text(copy.calculator.intro)}</p>
      </div>
      {calculator}
    </section>
  );

  return (
    <PublicFrame>
      <JsonLd data={[breadcrumbJsonLd(locale, page.breadcrumbs), faqJsonLd(page.faq)]} />
      <article className="bg-cream font-serif text-ink" data-testid="seo-page" data-route={page.route}>
        <nav aria-label={text("Breadcrumb")} className="border-b border-[rgb(186_175_157/0.4)] px-5 py-3 md:px-20" data-testid="seo-breadcrumbs">
          <ol className="mx-auto flex max-w-[1100px] flex-wrap items-center gap-1.5 text-sm">
            {page.breadcrumbs.map((crumb, index) => {
              const last = index === page.breadcrumbs.length - 1;
              return (
                <li key={crumb.route || "home"} className="flex items-center gap-1.5">
                  {index > 0 ? <ChevronRight aria-hidden className="size-3.5 text-[var(--muted-ink)]" /> : null}
                  {last ? (
                    <span aria-current="page" className="font-semibold">
                      {crumb.name}
                    </span>
                  ) : (
                    <Link href={localePath(crumb.route || "/", locale)} className="hover:underline">
                      {crumb.name}
                    </Link>
                  )}
                </li>
              );
            })}
          </ol>
        </nav>

        <header className="px-5 pt-10 pb-12 md:px-20 md:pt-14 md:pb-16">
          <div className="mx-auto flex max-w-[1100px] flex-col items-center gap-8 md:flex-row md:items-center md:gap-14">
            <div className="flex min-w-0 flex-1 flex-col gap-5">
              <p className="flex items-center gap-2 text-sm font-semibold tracking-[0.6px] text-[#6b4f1d] uppercase">
                <span aria-hidden className="size-2 rounded-[4px] bg-[var(--amber)]" />
                {page.eyebrow}
              </p>
              <h1 className="font-display text-[30px] leading-[1.15] [overflow-wrap:anywhere] sm:text-[38px] lg:text-[46px]" data-testid="seo-h1">
                {page.h1}
              </h1>
              <p className="text-[18px] leading-[1.65] md:text-[20px]" data-testid="seo-answer">
                <InlineText text={page.intro} locale={locale} />
              </p>
            </div>
            {page.artwork?.length ? (
              <div className="flex shrink-0 items-center gap-4">
                {page.artwork.map((art) => (
                  <img
                    key={art.src}
                    src={art.src}
                    alt={art.alt}
                    width={art.width}
                    height={art.height}
                    decoding="async"
                    className={art.height > art.width ? "h-auto w-[150px] rounded-lg shadow-[0_16px_20px_rgb(45_42_38/0.12)] md:w-[190px]" : "size-20 md:size-24"}
                  />
                ))}
              </div>
            ) : page.numeral ? (
              <p aria-hidden className="shrink-0 font-display text-[88px] leading-none text-[#8a6a1f] md:text-[120px]">
                {page.numeral}
              </p>
            ) : null}
          </div>
        </header>

        {hub ? calculatorSection : null}

        {page.grid ? (
          <section aria-labelledby="seo-grid-title" className="border-t border-[rgb(186_175_157/0.4)] px-5 py-12 md:px-20 md:py-16" data-testid="seo-grid">
            <div className="mx-auto flex max-w-[1100px] flex-col gap-6">
              <div className="flex flex-col gap-2">
                <h2 id="seo-grid-title" className="font-display text-[24px] tracking-[0.88px] uppercase md:text-[30px]">
                  {page.grid.heading}
                </h2>
                {page.grid.intro ? <p className="text-base">{page.grid.intro}</p> : null}
              </div>
              {page.grid.items ? (
                <ul className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
                  {page.grid.items.map((item) => (
                    <HubTile key={item.href} item={item} locale={locale} />
                  ))}
                </ul>
              ) : null}
              {page.grid.groups ? (
                <ul className="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
                  {page.grid.groups.map((group) => (
                    <HubGroupCard key={group.title} group={group} locale={locale} />
                  ))}
                </ul>
              ) : null}
            </div>
          </section>
        ) : null}

        {page.sections.map((section, index) => (
          <section
            key={section.id}
            id={section.id}
            aria-labelledby={`${section.id}-title`}
            className={`scroll-mt-24 border-t border-[rgb(186_175_157/0.4)] px-5 py-10 md:px-20 md:py-14 ${index % 2 === 1 ? "bg-cream-light/40" : ""}`}
            data-section={section.id}
          >
            <div className="mx-auto flex max-w-[860px] flex-col gap-5 text-[17px] leading-[1.7] md:text-[19px]">
              <h2 id={`${section.id}-title`} className="font-display text-[22px] leading-snug tracking-[0.6px] uppercase md:text-[28px]">
                {section.heading}
              </h2>
              {section.blocks.map((block, blockIndex) => (
                <Block key={blockIndex} block={block} locale={locale} />
              ))}
            </div>
          </section>
        ))}

        <section id="faq" aria-labelledby="faq-title" className="scroll-mt-24 border-t border-[rgb(186_175_157/0.4)] px-5 py-12 md:px-20 md:py-16" data-testid="seo-faq">
          <div className="mx-auto flex max-w-[860px] flex-col gap-6">
            <h2 id="faq-title" className="font-display text-[24px] tracking-[0.88px] uppercase md:text-[30px]">
              {text("Frequently asked questions")}
            </h2>
            <div className="flex flex-col">
              {page.faq.map((item) => (
                <div key={item.q} className="flex flex-col gap-2 border-b border-[rgb(176_145_79/0.3)] py-5 last:border-b-0">
                  <h3 className="text-[18px] leading-snug font-semibold md:text-[20px]">{item.q}</h3>
                  <p className="text-[17px] leading-[1.7] font-light md:text-[18px]">
                    <InlineText text={item.a} locale={locale} />
                  </p>
                </div>
              ))}
            </div>
          </div>
        </section>

        <nav aria-labelledby="seo-related-title" className="border-t border-[rgb(186_175_157/0.4)] px-5 py-10 md:px-20 md:py-14" data-testid="seo-related">
          <div className="mx-auto flex max-w-[860px] flex-col gap-5">
            <h2 id="seo-related-title" className="font-display text-[20px] tracking-[0.6px] uppercase md:text-[24px]">
              {page.related.heading}
            </h2>
            <Chips links={page.related.links} locale={locale} />
          </div>
        </nav>

        <div className="border-t border-[rgb(186_175_157/0.4)] px-5 py-6 md:px-20">
          <p className="mx-auto flex max-w-[860px] items-start gap-2 text-sm leading-normal text-[var(--muted-ink)]" data-testid="seo-entertainment-note">
            <Info aria-hidden className="mt-0.5 size-4 shrink-0" />
            {text(copy.note)}
          </p>
        </div>

        <AdvisorCta family={family} />

        {hub ? null : calculatorSection}
      </article>
    </PublicFrame>
  );
}
