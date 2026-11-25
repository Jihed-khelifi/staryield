"use client";

import * as React from "react";
import Link from "next/link";

import { useI18n } from "@/i18n/i18n-provider";
import type { AstroPosition, NatalChart } from "@/lib/api/astrology";
import { BODY_ORDER, PLANET_GLYPHS, houseLabel } from "@/lib/astro/zodiac";
import { usePsychicsHref } from "@/lib/psychics-link";
import { houseTheme, planetMeaning, signFlavor } from "@/components/astrology/chart-explorer/chart-copy";
import { AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { cn } from "@/lib/utils";

/** Rows shown beside the wheel: the Ascendant and the ten planets (Figma 172:598). */
export function rowBodies(chart: NatalChart): AstroPosition[] {
  const order = BODY_ORDER.filter((b) => b !== "Midheaven") as readonly string[];
  return chart.positions.filter((p) => order.includes(p.body)).sort((a, b) => order.indexOf(a.body) - order.indexOf(b.body));
}

/** "The Sun reflects… In Virgo… In the third house…" from chart-copy. */
export function useInterpretation() {
  const { text } = useI18n();
  return React.useCallback(
    (p: AstroPosition) => {
      const parts = [text(planetMeaning[p.body] ?? ""), text(signFlavor[p.sign] ?? "")];
      if (p.house && houseTheme[p.house])
        parts.push(text("In the {value0}, it is often expressed through {value1}.", { value0: text(houseLabel(p.house)).toLowerCase(), value1: text(houseTheme[p.house]) }));
      if (p.retrograde) parts.push(text("It is retrograde in your chart: its themes tend to turn inward and ask for review."));
      return parts.filter(Boolean).join(" ");
    },
    [text],
  );
}

type Align = "start" | "end";

export function PlanetRow({ position, mobileAlign, desktopAlign }: { position: AstroPosition; mobileAlign: Align; desktopAlign: Align }) {
  const { text } = useI18n();
  const interpret = useInterpretation();
  const psychicsHref = usePsychicsHref();
  // Rows sit left or right of the wheel; on phones they alternate in two columns.
  const end = cn(mobileAlign === "end" && "max-md:items-end max-md:text-right", desktopAlign === "end" && "md:items-end md:text-right");
  return (
    <AccordionItem value={position.body} className="border-b-[0.75px] border-[#c8c0a8] max-md:border-b-0" data-body={position.body}>
      <AccordionTrigger
        className={cn(
          "items-start gap-3 py-0 pt-3 pb-3 hover:text-ink md:pt-7 md:pb-8 [&>svg]:mt-1 [&>svg]:size-3.5 [&>svg]:opacity-70",
          mobileAlign === "end" && "max-md:flex-row-reverse",
          desktopAlign === "end" && "md:flex-row-reverse",
        )}
        data-testid={`planet-row-${position.body}`}
      >
        <span className={cn("flex min-w-0 flex-col gap-1 md:gap-2", end)}>
          <span aria-hidden="true" className="chart-glyph text-[18px] leading-7 text-[#1a1a1a] md:text-[22px]">
            {PLANET_GLYPHS[position.body]}
          </span>
          <span className={cn("flex flex-col gap-px text-[9px] leading-normal font-normal md:gap-[2px]", end)}>
            <span className="font-display tracking-[1.5px] text-[#1a1a1a] uppercase">{text(position.body)}</span>
            <span className="tracking-[1px] text-[#555] uppercase">
              {text(position.sign)}, {position.degree}°{position.minute}′{position.second}″
              {position.retrograde ? <span className="ml-1 normal-case">{text("℞ retrograde")}</span> : null}
            </span>
            {position.house ? <span className="font-semibold tracking-[1.5px] text-[#555] uppercase">{text(houseLabel(position.house))}</span> : null}
          </span>
        </span>
      </AccordionTrigger>
      <AccordionContent
        className={cn(
          "pt-0 pb-5 text-[13px] leading-5 md:max-w-[335px]",
          desktopAlign === "end" && "md:ml-auto md:text-right",
        )}
      >
        <p>{interpret(position)}</p>
        <Link href={psychicsHref} className="mt-2 inline-block text-xs font-semibold underline underline-offset-4">
          {text("Get more insight from one of our advisors ")}
        </Link>
      </AccordionContent>
    </AccordionItem>
  );
}
