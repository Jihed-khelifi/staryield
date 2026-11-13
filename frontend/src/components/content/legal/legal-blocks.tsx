import type { Locale } from "@/i18n/config";
import type { LegalBlock } from "@/content/legal/types";
import { Button } from "@/components/ui/button";
import { InlineText, resolveHref } from "@/components/content/legal/inline-text";

export const assetPathPrefix = "https://assets.staryield.net/assets/figma";
export const paperTexture = `${assetPathPrefix}/fb9ee.jpg`;
export const divider = `${assetPathPrefix}/27235.svg`;

export function formatDate(iso: string, locale: Locale) {
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "long",
    timeZone: "UTC",
  }).format(new Date(`${iso}T00:00:00Z`));
}

export const headingNumber = (n?: string) => (n ? `${n} ` : "");

export function Block({ block, locale }: { block: LegalBlock; locale: Locale }) {
  switch (block.type) {
    case "p":
      return (
        <p className="my-3">
          <InlineText text={block.text} locale={locale} />
        </p>
      );
    case "ul":
    case "ol": {
      const List = block.type;
      return (
        <List
          className={`my-3 space-y-2 ps-6 ${block.type === "ul" ? "list-disc" : "list-decimal"} marker:text-amber`}
        >
          {block.items.map((item, index) => (
            <li key={index}>
              <InlineText text={item} locale={locale} />
            </li>
          ))}
        </List>
      );
    }
    case "h3":
      return (
        <h3
          id={block.id}
          className="mt-8 mb-2 scroll-mt-28 font-serif text-[17px] font-bold leading-snug text-ink"
        >
          {headingNumber(block.n)}
          {block.title}
        </h3>
      );
    case "h4":
      return (
        <h4
          id={block.id}
          className="mt-6 mb-2 scroll-mt-28 font-serif text-[16px] font-semibold leading-snug text-ink"
        >
          {headingNumber(block.n)}
          {block.title}
        </h4>
      );
    // revisions 5.0: reading-card policy blocks.
    case "callout":
      return (
        <p className="my-4 rounded-md bg-[rgb(224_213_195/0.7)] p-6 font-semibold leading-snug text-ink">
          <InlineText text={block.text} locale={locale} />
        </p>
      );
    case "button": {
      const href = resolveHref(block.href, locale);
      if (!href) return null;
      return (
        <p className="my-4">
          <Button asChild variant="cta" size="xl" className="h-12 rounded-md px-6 text-base">
            <a href={href}>{block.label}</a>
          </Button>
        </p>
      );
    }
    case "table":
      return (
        <div
          className="my-4 overflow-x-auto rounded-md border border-[rgb(176_145_79/0.45)] bg-[#f8f4ea] focus-visible:outline-2 focus-visible:outline-[#c29a3d]"
          role="region"
          aria-label={block.head.join(" · ")}
          tabIndex={0}
        >
          <table className="w-full min-w-140 border-collapse text-left text-[14px] leading-relaxed">
            <thead>
              <tr>
                {block.head.map((cell, index) => (
                  <th
                    key={index}
                    scope="col"
                    className="border-b border-[rgb(176_145_79/0.45)] px-3 py-2 align-bottom font-semibold text-ink"
                  >
                    <InlineText text={cell} locale={locale} />
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {block.rows.map((row, rowIndex) => (
                <tr key={rowIndex} className="even:bg-[rgb(224_213_195/0.35)]">
                  {row.map((cell, cellIndex) =>
                    cellIndex === 0 ? (
                      <th
                        key={cellIndex}
                        scope="row"
                        className="border-t border-[rgb(176_145_79/0.25)] px-3 py-2 align-top font-semibold text-ink"
                      >
                        <InlineText text={cell} locale={locale} />
                      </th>
                    ) : (
                      <td
                        key={cellIndex}
                        className="border-t border-[rgb(176_145_79/0.25)] px-3 py-2 align-top"
                      >
                        <InlineText text={cell} locale={locale} />
                      </td>
                    ),
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      );
  }
}

