"use client";

import * as React from "react";
import Link from "next/link";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { usePsychicsHref } from "@/lib/psychics-link";
import { FAMILY_COPY, type SeoFamily } from "@/components/seo/copy";
import { AdvisorStrip } from "@/components/booking/advisor-strip";

/**
 * "Talk it through with an advisor" band on the search pages: up to three
 * advisors with the standard book button (growth: booking), then a link that
 * follows the site rule: the directory when signed in, the home page's
 * advisor section otherwise (`usePsychicsHref`).
 */
export function AdvisorCta({ family }: { family: SeoFamily }) {
  const { text } = useI18n();
  const psychicsHref = usePsychicsHref();
  const { heading, body } = FAMILY_COPY[family].cta;
  const placement = `seo_${family.replace("-", "_")}`;
  return (
    <section
      className="calc-olive flex flex-col items-center gap-5 px-5 py-12 text-center md:px-20 md:py-16"
      style={{ "--calc-texture": 0.62 } as React.CSSProperties}
      aria-labelledby="seo-advisor-cta"
      data-testid="seo-advisor-cta"
    >
      <h2 id="seo-advisor-cta" className="max-w-[1000px] font-display text-[19px] leading-[1.5] tracking-[0.48px] text-black uppercase md:text-[24px]">
        {text(heading)}
      </h2>
      <p className="max-w-[760px] text-base leading-normal text-black">{text(body)}</p>
      <AdvisorStrip heading="Choose an advisor" headingLevel={3} placement={placement} className="max-w-[640px] text-left" />
      <Link href={psychicsHref} className="calc-cta-link" onClick={() => track("chat_cta_clicked", { placement })}>
        {text("Meet our advisors")}
      </Link>
    </section>
  );
}
