"use client";
/* eslint-disable @next/next/no-img-element */
import { InlineText } from "@/components/content/legal/inline-text";
import type { FaqEntry } from "@/content/faq/live-readings";
import { useI18n } from "@/i18n/i18n-provider";

/**
 * A FAQ category whose questions are plain data (`content/faq/*`): the same two-column block as the Figma ones
 * (heading and description on the left, the expandable questions on the right). The page's FAQPage structured data
 * reads the same entries (`lib/seo/screen-json-ld.ts`).
 */
export function FaqAnswerBlock({
  id,
  blockName,
  itemName,
  heading,
  description,
  icon,
  entries,
}: {
  id: string;
  /** Figma name of the block, e.g. "Section / FAQ Block / 08". */
  blockName: string;
  /** Prefix of the Figma name of each item, e.g. "Instance / Live readings". */
  itemName: string;
  heading: string;
  description: string;
  icon: string;
  entries: readonly FaqEntry[];
}) {
  const { text, locale } = useI18n();
  return (
    <section className="content-stretch flex gap-[40px] items-start relative shrink-0 w-full figma-section" data-name={blockName} id={id}>
      <section className="content-stretch flex flex-col items-start relative shrink-0 w-[455px] figma-section" data-name="Section / Left Column">
        <div className="content-stretch flex gap-[12px] items-center pb-[16px] relative shrink-0" data-name="Header / Category">
          <div className="relative shrink-0 size-[34.07px]" data-name="Instance / Star">
            <img alt="" className="absolute block inset-0 max-w-none size-full" height="34.07" src={icon} width="34.07" />
          </div>
          <h2 className="[word-break:break-word] font-display leading-[normal] not-italic relative shrink-0 text-[20px] text-black tracking-[0.6px]">
            {text(heading)}
          </h2>
        </div>
        <p className="[word-break:break-word] font-serif font-normal leading-[22px] min-w-full relative shrink-0 text-[14px] text-black w-[min-content]">
          {text(description)}
        </p>
      </section>
      <section
        className="[word-break:break-word] content-stretch flex flex-col gap-[16px] items-start relative shrink-0 text-black text-left w-[705px] figma-section"
        data-name="Section / Right Column"
      >
        {entries.map((item, index) => (
          <details
            key={item.q}
            className="faq-item content-stretch flex flex-col gap-[8px] items-start overflow-clip pb-[32px] relative shrink-0 w-[495px]"
            data-name={`${itemName} / ${String(index + 1).padStart(2, "0")}`}
          >
            <summary>
              <p className="font-serif font-normal leading-[28px] relative shrink-0 text-[22px]">↑</p>
              <div className="content-stretch flex flex-col gap-[2px] items-start leading-[normal] relative shrink-0 text-[9px] w-full" data-name="Frame">
                <p className="font-display not-italic relative shrink-0 tracking-[1.5px] uppercase whitespace-normal">{text(item.q)}</p>
              </div>
            </summary>
            <p className="mt-4 font-serif text-sm leading-[22px] tracking-normal normal-case whitespace-normal">
              <InlineText text={text(item.a)} locale={locale} />
            </p>
          </details>
        ))}
      </section>
    </section>
  );
}
