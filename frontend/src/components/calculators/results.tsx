"use client";

import * as React from "react";
import Link from "next/link";
import { LockKeyhole, Mail } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import type { HouseSystem, NatalResult, SunSignResult } from "@/lib/api/astrology";
import type { ApiError } from "@/lib/api/errors";
import { usePsychicsHref } from "@/lib/psychics-link";
import { formatDateParts, romanNumeral } from "@/lib/calculators";
import { LIFE_PATH_COPY, NUMBER_CARDS } from "@/lib/astro/life-path-copy";
import { HOUSE_SYSTEM_NAMES } from "@/lib/astro/report-labels";
import { lifePathSteps } from "@/lib/astro/report-steps";
import { SIGN_FACTS, SIGN_INSIGHT, isSign } from "@/lib/astro/zodiac";
import { birthDateOf, type CalculatorFormValues, type CalculatorKind } from "@/lib/schemas/calculator";
import { natalHeadline } from "@/lib/share/card-spec";
import { BIRTH_DATA_NOTICE } from "@/components/calculators/birth-data-notice";
import { lifePathResult, type LifePathResult } from "@/components/calculators/calculate";
import { SpreadCardList, TarotSpread } from "@/components/calculators/tarot-spread";
import { ZodiacWheel } from "@/components/astrology/ZodiacWheel";
import { ChartExplorer } from "@/components/astrology/chart-explorer/chart-explorer";
import { NumbersSummary } from "@/components/astrology/chart-explorer/numbers-summary";
import { ShareResultButton } from "@/components/calculators/share-result";
import { cn } from "@/lib/utils";

const WALL_TIME_NOTES: Record<string, string> = {
  skipped: "That clock time did not exist there (the clocks went forward), so the time just before the change was used.",
  ambiguous: "That clock time happened twice there (the clocks went back), so the earlier one was used.",
  lmt: "Your birthplace kept local mean time then, so its own mean time was used.",
};

/** "Natal alignment calculated for:" with the date button that reopens the form (Figma 482:2040). */
export function ResultDate({
  values,
  details,
  onChange,
  sample = false,
  onEmail,
  onShare,
  title,
}: {
  values: CalculatorFormValues;
  /** The page's h1 (defaults to "Natal alignment calculated for:"). */
  title?: string;
  details?: React.ReactNode;
  onChange: () => void;
  sample?: boolean;
  /** "E-mail me this report" (revisions 6.0, §26.4); omitted when the report cannot be e-mailed (sample, switched off). */
  onEmail?: () => void;
  /** "Share your result" (omitted on the sample and while there is no result). */
  onShare?: () => void;
}) {
  const { text } = useI18n();
  return (
    <section className="flex flex-col items-center gap-6 border-b border-[rgb(186_175_157/0.4)] bg-cream px-5 py-10 text-center md:px-20 md:py-16">
      <h1 tabIndex={-1} id="calculator-result-title" className="font-display text-[24px] tracking-[1.28px] uppercase outline-none md:text-[32px]">
        {title ?? text("Natal alignment calculated for:")}
      </h1>
      <button type="button" className="calc-date-button" onClick={onChange} data-testid="change-birth-date">
        <span aria-hidden="true">{formatDateParts(birthDateOf(values.birth))}</span>
        <span className="sr-only">{text("Change birth date ({value0})", { value0: formatDateParts(birthDateOf(values.birth)) })}</span>
      </button>
      {details ? <div className="-mt-2 text-sm font-light">{details}</div> : null}
      {onEmail || onShare ? (
        <div className="flex flex-wrap items-center justify-center gap-3">
          {onEmail ? (
            <button type="button" className="calc-email-button" onClick={onEmail} data-testid="email-report-button">
              <Mail aria-hidden className="size-4" />
              {text("E-mail me this report")}
            </button>
          ) : null}
          {onShare ? <ShareResultButton onClick={onShare} /> : null}
        </div>
      ) : null}
      {sample ? (
        <p className="text-xs text-[var(--muted-ink)]">{text("Sample date from the design. Change the birth date to calculate your own result.")}</p>
      ) : null}
    </section>
  );
}

export function Lock({ children }: { children: React.ReactNode }) {
  return (
    <p className="flex items-center justify-center gap-1.5 text-center text-xs text-[var(--muted-ink)]">
      <LockKeyhole aria-hidden className="size-3 shrink-0" />
      {children}
    </p>
  );
}

export function StarHeading({ children, size = "md" }: { children: React.ReactNode; size?: "md" | "lg" }) {
  return (
    <h2
      className={cn(
        "font-display tracking-[0.88px] uppercase",
        size === "lg" ? "text-[30px] md:text-[44px]" : "text-[28px] tracking-[1.2px] md:text-[40px]",
      )}
    >
      <span aria-hidden="true">✦ </span>
      {children}
      <span aria-hidden="true"> ✦</span>
    </h2>
  );
}

export type NatalHouseState = {
  /** The house system asked for (the URL's `h=`); the chart on screen may still be the previous one. */
  houseSystem: HouseSystem;
  onChange: (system: HouseSystem) => void;
  /** A chart for the new house system is being calculated. */
  updating: boolean;
  error: ApiError | null;
  onRetry: () => void;
};

/**
 * Natal chart: the headline with the big three, the full chart explorer (the
 * same one as the signed-in chart), the numbers computed in the browser, the
 * data-driven Twelve Houses spread (Figma 482:2010) and the advisor call to
 * action.
 */
export function NatalResultView({
  result,
  houses,
  onEditBirth,
  onShare,
}: {
  result: NatalResult;
  houses: NatalHouseState;
  /** Back to the form to add a birth time and place. */
  onEditBirth: () => void;
  onShare?: () => void;
}) {
  const { text } = useI18n();
  const psychicsHref = usePsychicsHref();
  const spread = result.tarot;
  const chart = result.chart;
  const system = chart.house_system_used || spread?.house_system || "";
  // "Your numbers" come from the birth date in the browser: no extra request.
  const numbers = React.useMemo(() => {
    const { lifePath, cycles } = lifePathResult(chart.birth_date);
    return { lifePath: lifePath.number, personalYear: cycles.personalYear, personalMonth: cycles.personalMonth };
  }, [chart.birth_date]);
  const addBirthData = (
    <button type="button" className="ml-1 font-semibold underline underline-offset-4" onClick={onEditBirth} data-testid="add-birth-time">
      {text("Add birth time and place")}
    </button>
  );
  return (
    <>
      <section className="flex flex-col items-center bg-cream px-5 pt-14 pb-12 text-center md:px-12 md:pt-20 md:pb-16" aria-labelledby="chart-title" data-testid="natal-chart-section">
        <div id="chart-title">
          <StarHeading size="lg">{text("Your birth chart")}</StarHeading>
        </div>
        <p className="mt-4 max-w-3xl font-display text-[20px] leading-snug tracking-[0.6px] text-[#17150f] uppercase md:text-[26px]" data-testid="big-three">
          {natalHeadline(chart, text)}
        </p>
        {chart.moon_sign_uncertain ? (
          <p className="mt-2 max-w-2xl text-sm text-[var(--muted-ink)]" data-testid="moon-uncertain">
            {text("The Moon changed signs on your birth date, so your Moon sign needs your birth time and place.")}
          </p>
        ) : null}
        <p className="mt-3 max-w-2xl text-base leading-normal">{text("Your natal chart is a map of the sky at the precise moment you arrived. ")}</p>
        <div className="mt-8 w-full max-w-[1280px]">
          <ChartExplorer
            chart={chart}
            houseSystem={houses.houseSystem}
            onHouseSystemChange={houses.onChange}
            updating={houses.updating}
            updateError={houses.error}
            onRetry={houses.onRetry}
            timeNote={
              <>
                {text("Add your birth time and place to see your rising sign and houses.")}
                {addBirthData}
              </>
            }
            placeNote={
              <>
                {text("Your birth time was left out because no birthplace was given. Add your birthplace to use it.")}
                {addBirthData}
              </>
            }
          />
        </div>
        <NumbersSummary numbers={numbers} className="mt-8 w-full max-w-xl rounded-lg border border-gold/50 px-4 py-4" />
      </section>
      <section className="flex flex-col items-center bg-cream px-5 pt-4 pb-12 text-center md:px-20 md:pb-[68px]" aria-labelledby="spread-title">
        <div id="spread-title">
          <StarHeading size="lg">{text("Your spread")}</StarHeading>
        </div>
        <p className="mt-3 max-w-3xl text-base leading-normal">
          {text("A 12-card spread exploring the current landscape, themes, and guidance for your journey.")}
        </p>
        {spread?.uncertain ? (
          <p className="mt-4 max-w-2xl rounded-lg border border-gold/50 bg-cream-light/70 px-4 py-3 text-sm" data-testid="spread-uncertain">
            {text("Without an exact birth time, cards with a dashed frame may change. Add your time and place for a more detailed spread.")}
          </p>
        ) : null}
        {spread ? (
          <>
            <div className="mt-8 w-full max-w-[1000px]">
              <TarotSpread spread={spread} />
            </div>
            <div className="mt-12 w-full max-w-[1180px]">
              <h3 className="mb-6 text-left text-xl font-semibold">{text("Your cards and why they were drawn")}</h3>
              <SpreadCardList spread={spread} />
            </div>
          </>
        ) : null}
        <div className="mt-10 grid max-w-3xl gap-2 text-xs text-[var(--muted-ink)]">
          <p>
            {chart.time_known && system
              ? text("Houses: {value0}.", { value0: text(HOUSE_SYSTEM_NAMES[system] ?? system) })
              : text("Birth time unknown: houses are counted whole-sign from your Sun.")}
            {chart.fallback_reason ? ` ${text("Placidus is undefined this close to the poles, so Porphyry houses were used.")}` : ""}
          </p>
          {chart.wall_time_status && WALL_TIME_NOTES[chart.wall_time_status] ? <p>{text(WALL_TIME_NOTES[chart.wall_time_status])}</p> : null}
          <Lock>{text(BIRTH_DATA_NOTICE)}</Lock>
          {/* The explorer above already shows the chart's disclaimer and attribution. */}
          {result.disclaimer !== chart.disclaimer ? <p>{text(result.disclaimer)}</p> : null}
          {result.attribution && result.attribution !== chart.attribution ? <p lang="en">{result.attribution}</p> : null}
        </div>
      </section>
      <AdvisorCta
        title={text("Talk to an advisor about your chart")}
        body={text("A psychic advisor can walk you through your planets, your houses and this spread, for entertainment purposes only.")}
        href={psychicsHref}
        onShare={onShare}
      />
    </>
  );
}

/** The closing call to action of a result: talk to an advisor, and share the result. */
export function AdvisorCta({ title, body, href, onShare, children }: { title: string; body: string; href: string; onShare?: () => void; children?: React.ReactNode }) {
  const { text } = useI18n();
  return (
    <section className="calc-olive flex flex-col items-center gap-5 px-5 py-12 text-center md:px-20 md:py-16" style={{ "--calc-texture": 0.62 } as React.CSSProperties} data-testid="advisor-cta">
      <h2 className="max-w-[1000px] font-display text-[19px] leading-[1.5] tracking-[0.48px] text-black uppercase md:text-[24px]">{title}</h2>
      <p className="max-w-[800px] text-base leading-normal text-black">{body}</p>
      {children}
      <div className="flex flex-wrap items-center justify-center gap-4 pt-2">
        <Link href={href} className="calc-cta-link" data-testid="advisor-cta-link">
          {text("Talk to an advisor")}
        </Link>
        {onShare ? <ShareResultButton onClick={onShare} /> : null}
      </div>
    </section>
  );
}

/** Astrology: the sun sign on the zodiac wheel (Figma 482:2275). */
export function SunSignResultView({
  result,
  onAddTime,
  chartHref,
  onShare,
}: {
  result: SunSignResult;
  onAddTime: () => void;
  /** The free natal chart calculator carrying the same birth details in its fragment. */
  chartHref: string;
  onShare?: () => void;
}) {
  const { text } = useI18n();
  const sign = isSign(result.sun_sign) ? result.sun_sign : null;
  const alternatives = (result.alternatives ?? []).filter((s) => s !== result.sun_sign);
  const facts = sign ? SIGN_FACTS[sign] : null;
  return (
    <>
      <section className="flex flex-col items-center gap-8 bg-cream px-5 py-12 md:gap-12 md:p-20" aria-labelledby="zodiac-title">
        <div id="zodiac-title">
          <StarHeading>{text("Your zodiac")}</StarHeading>
        </div>
        {result.uncertain ? (
          <div className="max-w-2xl rounded-xl border border-gold/60 bg-cream-light/80 px-5 py-4 text-center text-sm" data-testid="sun-sign-uncertain">
            <p className="font-semibold">
              {text("You were born on a cusp: the Sun moved from {value0} into {value1} that day.", {
                value0: text(result.alternatives?.[0] ?? result.sun_sign),
                value1: text(result.alternatives?.[1] ?? result.sun_sign),
              })}
            </p>
            <p className="mt-1 font-light">
              {text("We show {value0}, the sign for your date at noon. Add your birth time and place to know for sure.", { value0: text(result.sun_sign) })}
            </p>
            <button type="button" className="mt-3 font-semibold underline underline-offset-4" onClick={onAddTime}>
              {text("Add birth time and place")}
            </button>
          </div>
        ) : null}
        <div className="w-full max-w-[800px]">
          <ZodiacWheel sign={result.sun_sign} alternatives={result.uncertain ? alternatives : []} />
        </div>
      </section>
      <section className="calc-olive px-5 py-12 text-white md:px-[120px] md:pb-14 md:pt-12" data-tone="dark" style={{ "--calc-texture": 0.2 } as React.CSSProperties}>
        <div className="mx-auto flex max-w-[1288px] flex-col gap-3 md:px-12">
          <h2 className="text-[22px] leading-snug font-semibold md:text-[28px]">
            {text("According to your alignment, the sun resides in the sign of")}{" "}
            <span className="font-display text-[34px] font-normal tracking-[0.56px] uppercase md:text-[48px]" data-testid="sun-sign">
              {text(result.sun_sign)}
            </span>
          </h2>
          <p className="text-[17px] leading-[1.6] md:text-lg">
            {sign ? text(SIGN_INSIGHT[sign]) : null}{" "}
            {facts
              ? text("{value0} is ruled by {value1} and belongs to the {value2} element ({value3}).", {
                  value0: text(result.sun_sign),
                  value1: text(facts.ruler),
                  value2: text(result.element || facts.element),
                  value3: text(result.modality),
                })
              : null}
          </p>
        </div>
      </section>
      <section className="calc-paper flex flex-col items-center gap-6 px-5 py-14 text-center md:px-[120px] md:py-20">
        <h2 className="max-w-[1000px] font-display text-[19px] leading-[1.5] tracking-[0.48px] text-ink uppercase md:text-[24px]">
          {text("Want to discover your planetary coordinates?")}
        </h2>
        <p className="max-w-[800px] text-base leading-normal text-[var(--muted-ink)]">
          {text("Your Moon sign, planets, houses and aspects come from the same birth details. See the full chart for free, with no account needed.")}
        </p>
        <div className="flex flex-wrap items-center justify-center gap-4 pt-3">
          <Link href={chartHref} className="calc-cta-link" data-testid="full-chart-link">
            {text("See your full birth chart — free")}
          </Link>
          {onShare ? <ShareResultButton onClick={onShare} /> : null}
        </div>
        <Lock>{text(BIRTH_DATA_NOTICE)}</Lock>
      </section>
    </>
  );
}

/** Path of Life: the three numbers, computed in the browser (Figma 482:2164). */
export function LifePathResultView({ result, onShare }: { result: LifePathResult; onShare?: () => void }) {
  const { text, locale } = useI18n();
  const psychicsHref = usePsychicsHref();
  const numbers: Record<(typeof NUMBER_CARDS)[number]["key"], number> = {
    lifePath: result.lifePath.number,
    personalYear: result.cycles.personalYear,
    personalMonth: result.cycles.personalMonth,
  };
  const copy = LIFE_PATH_COPY[result.lifePath.number] ?? LIFE_PATH_COPY[9];
  const steps = lifePathSteps(result.lifePath, text);
  return (
    <>
      <section className="flex flex-col items-center gap-10 bg-cream px-5 pt-14 pb-16 text-center md:gap-12 md:px-20 md:pt-20 md:pb-[100px]" aria-labelledby="numbers-title">
        <div id="numbers-title">
          <StarHeading>{text("Your numbers")}</StarHeading>
        </div>
        <ul className="grid w-full max-w-[1280px] gap-4 md:grid-cols-3 md:gap-6" data-testid="life-path-numbers">
          {NUMBER_CARDS.map((card) => (
            <li key={card.key} className="flex flex-col items-center gap-4 rounded-xl border border-warm-gray bg-cream p-6 md:min-h-[260px]" data-number={card.key}>
              <h3 className="font-display text-base tracking-[1.6px] uppercase">{text(card.label)}</h3>
              <p className="font-display text-[60px] leading-none text-[#8a6a1f] md:text-[72px]">
                <span aria-hidden="true">{romanNumeral(numbers[card.key])}</span>
                <span className="sr-only" data-testid={`number-${card.key}`}>
                  {numbers[card.key]}
                </span>
              </p>
              <p className="text-[15px] leading-[1.4] font-light">{text(card.body)}</p>
            </li>
          ))}
        </ul>
        <p className="text-xs text-[var(--muted-ink)]">
          {text("Personal cycles for {value0}, your local date.", {
            value0: new Intl.DateTimeFormat(locale, { dateStyle: "long", timeZone: "UTC" }).format(new Date(`${result.cycles.asOf}T12:00:00Z`)),
          })}
        </p>
      </section>
      <section className="calc-olive px-5 py-12 text-white md:px-[120px] md:py-20" data-tone="dark" style={{ "--calc-texture": 0.35 } as React.CSSProperties}>
        <div className="mx-auto flex max-w-[1200px] flex-col gap-5 md:px-12">
          <div className="flex flex-wrap items-end gap-x-7 gap-y-1">
            <h2 className="font-display text-[26px] tracking-[0.56px] uppercase md:text-[28px]" data-testid="life-path-title">
              {text("Life path {value0}", { value0: result.lifePath.number })}
            </h2>
            <p className="pb-1 text-sm font-light tracking-[0.28px]">{text(copy.title)}</p>
          </div>
          <p className="text-[17px] leading-[1.6] font-light md:text-lg">{text(copy.body)}</p>
          <p className="text-sm font-light" data-testid="life-path-steps">
            <span className="font-semibold">{text("How it adds up:")}</span> {steps.join(" · ")}
          </p>
        </div>
      </section>
      <section className="calc-paper flex flex-col items-center gap-4 px-5 py-14 text-center md:px-[120px] md:py-20">
        <h2 className="max-w-[1000px] font-display text-[19px] leading-[1.5] tracking-[0.48px] uppercase md:text-[24px]">
          {text("Get a deeper understanding of what a life path really means and the alignment that comes with it")}
        </h2>
        <p className="max-w-[800px] text-base leading-normal font-light">
          {text(
            "Chat with one of our psychic advisors to explore your chart and upcoming planetary transitions, for entertainment purposes only. ",
          )}
        </p>
        <div className="flex flex-wrap items-center justify-center gap-4 pt-3">
          <Link href={psychicsHref} className="calc-cta-link" data-testid="advisor-cta-link">
            {text("Click here!")}
          </Link>
          {onShare ? <ShareResultButton onClick={onShare} /> : null}
        </div>
        <Lock>{text(BIRTH_DATA_NOTICE)}</Lock>
      </section>
    </>
  );
}

export type { CalculatorKind };
