/* eslint-disable @next/next/no-img-element */
import { createElement, type ReactNode } from "react";
import type { Locale } from "@/i18n/config";
import type {
  ArticleBlock,
  ArticleFormulaId,
  ArticleImage,
  ArticleKind,
  ArticleWeight,
} from "@/content/calculators/types";
import {
  getCalculatorArticle,
  type CalculatorRouteKind,
} from "@/content/calculators";
import { InlineText } from "@/components/content/legal/inline-text";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";

const TIMELINE_STAR =
  "https://assets.staryield.net/assets/articles/timeline-star-73dd09ac.png";
const TIMELINE_CONNECTOR =
  "https://assets.staryield.net/assets/articles/timeline-connector-b79b343a.svg";

const weightClass: Record<ArticleWeight, string> = {
  light: "font-light",
  regular: "font-normal",
  medium: "font-medium",
  semibold: "font-semibold",
};

// React's JSX types do not include MathML elements, so formulas are built
// with createElement. Function names stay untranslated by design.
const mi = (text: string) => createElement("mi", null, text);
const mo = (text: string) => createElement("mo", null, text);
const mrow = (...children: ReactNode[]) =>
  createElement("mrow", null, ...children);
const mfrac = (numerator: ReactNode, denominator: ReactNode) =>
  createElement("mfrac", null, numerator, denominator);
const call = (fn: string, arg: string) => [mi(fn), mo("("), mi(arg), mo(")")];

export const FORMULAS: Record<ArticleFormulaId, ReactNode> = {
  ascendant: createElement(
    "math",
    { display: "block", className: "text-[1.05em]" },
    mrow(
      ...call("tan", "Asc"),
      mo("="),
      mfrac(
        mrow(mo("−"), ...call("cos", "RAMC")),
        mrow(
          ...call("sin", "RAMC"),
          mo("·"),
          ...call("cos", "ε"),
          mo("+"),
          ...call("tan", "φ"),
          mo("·"),
          ...call("sin", "ε"),
        ),
      ),
    ),
  ),
};

function ArticleFigure({ image }: { image: ArticleImage }) {
  const { frame } = image;
  return (
    <img
      src={image.src}
      alt={image.alt}
      width={image.width}
      height={image.height}
      loading="lazy"
      decoding="async"
      className={`block h-auto w-full border object-cover ${image.position === "bottom" ? "object-bottom" : "object-center"} border-gold`}
      style={{
        aspectRatio: `${frame.width} / ${frame.height}`,
        maxWidth: frame.width,
        borderRadius: frame.radius,
      }}
    />
  );
}

function Block({
  block,
  locale,
  prefix,
}: {
  block: ArticleBlock;
  locale: Locale;
  prefix: string;
}) {
  switch (block.type) {
    case "p":
      return (
        <p className={weightClass[block.weight ?? "regular"]}>
          <InlineText text={block.text} locale={locale} />
        </p>
      );
    case "h2":
      return (
        <h2
          id={`${prefix}-${block.id}`}
          className={`scroll-mt-28 ${
            block.size === "lg"
              ? "text-[24px] font-semibold sm:text-[28px] lg:text-[32px]"
              : "font-medium"
          } ${block.align === "center" ? "text-center" : ""}`}
        >
          {block.text}
        </h2>
      );
    case "h3":
      return (
        <h3 id={`${prefix}-${block.id}`} className="scroll-mt-28 font-bold">
          {block.text}
        </h3>
      );
    case "ul":
    case "ol": {
      const List = block.type;
      return (
        <List
          className={`space-y-1 ps-7 lg:ps-9 ${block.type === "ul" ? "list-disc" : "list-decimal"}`}
        >
          {block.items.map((item, index) => (
            <li key={index}>
              <InlineText text={item} locale={locale} />
            </li>
          ))}
        </List>
      );
    }
    case "split":
      return (
        <div className="flex flex-col items-center gap-8 lg:flex-row lg:gap-12">
          <div className="flex min-w-0 flex-1 flex-col gap-7">
            {block.blocks.map((inner, index) => (
              <Block key={index} block={inner} locale={locale} prefix={prefix} />
            ))}
          </div>
          <div className="w-full shrink-0" style={{ maxWidth: block.image.frame.width }}>
            <ArticleFigure image={block.image} />
          </div>
        </div>
      );
    case "image":
      return <ArticleFigure image={block.image} />;
    case "timeline":
      return (
        <figure
          aria-label={block.label}
          className="rounded-md border border-[rgb(176_145_79/0.3)] bg-[#f8f4ea]/60 px-5 py-8 sm:px-9 sm:py-10"
        >
          <ol className="grid grid-cols-2 gap-x-4 gap-y-8 sm:flex sm:items-start sm:gap-0">
            {block.items.map((item, index) => (
              <li key={index} className="contents">
                {index > 0 && (
                  <img
                    src={TIMELINE_CONNECTOR}
                    alt=""
                    aria-hidden
                    width={204}
                    height={10}
                    className="mt-2.75 hidden h-[9.64px] min-w-6 flex-1 sm:block"
                  />
                )}
                <div className="flex flex-col items-center gap-4 text-center sm:min-w-28">
                  <img
                    src={TIMELINE_STAR}
                    alt=""
                    aria-hidden
                    width={23}
                    height={23}
                    className="size-[22.96px]"
                  />
                  <div className="flex flex-col gap-1">
                    <span className="text-[18px] leading-[1.27] text-[#29271f] lg:text-[24px]">
                      {item.title}
                    </span>
                    {item.period && (
                      <span className="text-[14px] leading-normal text-[#6d6753] lg:text-[15.5px]">
                        {item.period}
                      </span>
                    )}
                  </div>
                </div>
              </li>
            ))}
          </ol>
        </figure>
      );
    case "twoColumnList":
      return (
        <ul className="gap-x-8 lg:columns-2">
          {block.items.map((item, index) => (
            <li key={index} className="mb-9 break-inside-avoid">
              <p>
                <strong className="font-semibold">{item.title}</strong>
                <span aria-hidden> </span>
                <span>{item.subtitle}</span>
              </p>
              {item.body.map((line, lineIndex) => (
                <p key={lineIndex} className="font-light">
                  <InlineText text={line} locale={locale} />
                </p>
              ))}
            </li>
          ))}
        </ul>
      );
    case "faq":
      // FAQ accordions (revisions 2.0): questions are buttons inside headings
      // and the answer opens in place.
      return (
        <Accordion type="multiple" className="flex flex-col" data-testid="article-faq">
          {block.items.map((item, index) => (
            <AccordionItem key={index} value={`faq-${index}`} className="border-b border-[rgb(176_145_79/0.3)] last:border-b-0">
              <AccordionTrigger className="py-4 text-left text-[17px] leading-normal font-medium tracking-[0.02em] sm:text-[19px] lg:text-[24px]">
                {item.q}
              </AccordionTrigger>
              <AccordionContent className="pb-5">
                <p className="text-[17px] leading-normal font-light tracking-[0.02em] text-black sm:text-[19px] lg:text-[24px]">
                  <InlineText text={item.a} locale={locale} />
                </p>
              </AccordionContent>
            </AccordionItem>
          ))}
        </Accordion>
      );
    case "formula":
      return (
        <div
          role="img"
          aria-label={block.label}
          className="overflow-x-auto py-4 text-center font-serif"
        >
          <div aria-hidden>{FORMULAS[block.formula]}</div>
        </div>
      );
  }
}

/**
 * Long-form explainer under a calculator's results (Figma 482:2010,
 * 1279:1749 and 482:2275). Server component: render it from the route and
 * pass it to the client calculator as a slot so the copy is never bundled for
 * the client, e.g. `<Calculator article={<CalculatorArticle kind="astrology" locale={lang} />} />`.
 */
export function CalculatorArticle({
  kind,
  locale,
}: {
  kind: ArticleKind | CalculatorRouteKind;
  locale: Locale;
}) {
  const article = getCalculatorArticle(kind, locale);
  return (
    <section
      aria-label={article.label}
      data-calculator-article={article.kind}
      className="w-full bg-[linear-gradient(148deg,#e0d5c3_11%,#f2e9db_102%)] px-5 py-14 font-serif text-[17px] leading-normal tracking-[0.02em] text-black sm:px-10 sm:text-[19px] lg:px-37 lg:py-23 lg:text-[24px]"
    >
      <div className="mx-auto flex w-full max-w-286 flex-col gap-14 lg:gap-21.25">
        {article.sections.map((section) => (
          <div
            key={section.id}
            data-article-section={section.id}
            className="flex flex-col gap-7"
          >
            {section.blocks.map((block, index) => (
              <Block key={index} block={block} locale={locale} prefix={article.kind} />
            ))}
          </div>
        ))}
      </div>
    </section>
  );
}
