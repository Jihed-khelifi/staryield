/* eslint-disable @next/next/no-img-element */
import type { Locale } from "@/i18n/config";
import type {
  LegalDocumentId,
  LegalHeadingBlock,
} from "@/content/legal/types";
import { getLegalDocument } from "@/content/legal";
import {
  Block,
  divider,
  formatDate,
  headingNumber,
  paperTexture,
} from "@/components/content/legal/legal-blocks";
// revisions 5.0: reading-card layout for policies (Refund Policy).
import { PolicyDocument } from "@/components/content/legal/policy-document";

/**
 * Server-rendered legal document: semantic headings with stable anchors, a
 * table of contents generated from the structure, dates in the hero and at
 * the end. Ships no client JavaScript.
 */
export function LegalDocument({
  id,
  locale,
}: {
  id: LegalDocumentId;
  locale: Locale;
}) {
  const doc = getLegalDocument(id, locale);
  // revisions 5.0: documents with a `policy` section use the reading-card layout.
  if (doc.policy) return <PolicyDocument id={id} doc={doc} locale={locale} />;
  const { meta, labels } = doc;
  const effective = formatDate(meta.effective, locale);
  const updated = formatDate(meta.lastUpdated, locale);
  const dates = (
    <p className="text-[14px] italic text-[#5a4f43]">
      <span>{labels.effective}</span>
      <span aria-hidden> </span>
      <time dateTime={meta.effective}>{effective}</time>
      <span aria-hidden> · </span>
      <span>{labels.lastUpdated}</span>
      <span aria-hidden> </span>
      <time dateTime={meta.lastUpdated}>{updated}</time>
    </p>
  );

  return (
    <div
      className="flex w-full flex-col items-center bg-cream print:bg-white"
      data-legal-document={id}
    >
      <header className="flex w-full flex-col items-center gap-4 px-4 pt-14 pb-8 text-center md:px-10 md:pt-20">
        <p className="font-serif text-[13px] font-semibold uppercase tracking-[3px] text-amber md:text-[14px]">
          {meta.badge}
        </p>
        <h1
          id="legal-title"
          className="font-display text-[34px] leading-[1.15] text-[#1a1612] md:text-[48px]"
        >
          {meta.title}
        </h1>
        <img
          alt=""
          aria-hidden
          className="h-0.5 w-30 max-w-none"
          src={divider}
        />
        {dates}
      </header>
      <div className="w-full px-3 pb-20 sm:px-6 md:px-10">
        <article
          aria-labelledby="legal-title"
          className="relative mx-auto w-full max-w-243.5 rounded-[12px] border border-gold px-5 py-10 font-serif text-[15px] leading-[1.7] text-[#2d241e] shadow-[0_12px_16px_rgba(0,0,0,0.04)] sm:px-10 md:px-16 md:py-18 print:border-0 print:px-0 print:shadow-none"
        >
          <div
            aria-hidden
            className="pointer-events-none absolute inset-0 rounded-[12px] print:hidden"
          >
            <div className="absolute inset-0 rounded-[12px] bg-cream" />
            <div
              className="absolute inset-0 rounded-[12px] bg-top-left mix-blend-overlay"
              style={{ backgroundImage: `url("${paperTexture}")` }}
            />
          </div>
          <div className="relative mx-auto max-w-[78ch] wrap-anywhere">
            {doc.intro.length > 0 && (
              <div className="mb-8 rounded-md border border-[rgb(176_145_79/0.45)] bg-[#f8f4ea]/70 px-5 py-4 md:px-7 md:py-5">
                {doc.intro.map((block, index) => (
                  <Block key={index} block={block} locale={locale} />
                ))}
              </div>
            )}
            <nav
              id="toc"
              aria-labelledby="legal-toc-title"
              className="mb-10 rounded-md border border-[rgb(176_145_79/0.45)] bg-[#f8f4ea] px-5 py-5 md:px-8 md:py-7"
            >
              <h2
                id="legal-toc-title"
                className="mb-3 font-display text-[18px] leading-snug text-ink"
              >
                {labels.toc}
              </h2>
              <ol className="space-y-2">
                {doc.sections.map((section) => {
                  const subsections = section.blocks.filter(
                    (block): block is LegalHeadingBlock => block.type === "h3",
                  );
                  return (
                    <li key={section.id}>
                      <a
                        href={`#${section.id}`}
                        className="font-semibold text-ink underline-offset-2 hover:underline focus-visible:rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#c29a3d]"
                      >
                        {`${section.n}. ${section.title}`}
                      </a>
                      {subsections.length > 0 && (
                        <ol className="mt-1 space-y-1 ps-5 text-[14px]">
                          {subsections.map((sub) => (
                            <li key={sub.id}>
                              <a
                                href={`#${sub.id}`}
                                className="text-[#3d332a] underline-offset-2 hover:underline focus-visible:rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#c29a3d]"
                              >
                                {`${headingNumber(sub.n)}${sub.title}`}
                              </a>
                            </li>
                          ))}
                        </ol>
                      )}
                    </li>
                  );
                })}
              </ol>
            </nav>
            {doc.sections.map((section) => (
              <section
                key={section.id}
                aria-labelledby={section.id}
                className="mt-10 first-of-type:mt-0"
              >
                <h2
                  id={section.id}
                  className="mb-3 scroll-mt-28 font-display text-[18px] leading-snug text-[#1a1612] md:text-[20px]"
                >
                  {`${section.n}. ${section.title}`}
                </h2>
                {section.blocks.map((block, index) => (
                  <Block key={index} block={block} locale={locale} />
                ))}
              </section>
            ))}
            <footer className="mt-12 flex flex-col gap-3 border-t border-[rgb(176_145_79/0.45)] pt-6">
              {dates}
              <a
                href="#toc"
                className="self-start text-[14px] font-semibold text-[#6b4f1d] underline decoration-[#c29a3d] underline-offset-2 print:hidden"
              >
                {labels.backToTop}
              </a>
            </footer>
          </div>
        </article>
      </div>
    </div>
  );
}
