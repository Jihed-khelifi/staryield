"use client";

import * as React from "react";
import Link from "next/link";
import { ChevronDown } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { astroApi, HOUSE_SYSTEMS, type AstroPosition, type HouseSystem, type NatalChart } from "@/lib/api/astrology";
import type { User } from "@/lib/api/types";
import { useAsync } from "@/lib/use-async";
import { ASPECT_NAMES, BODY_ORDER, PLANET_GLYPHS, degreeInSign, houseLabel } from "@/lib/astro/zodiac";
import { aspectLineStyle } from "@/lib/astro/wheel-geometry";
import { houseTheme, planetMeaning, signFlavor } from "@/components/account/chart-copy";
import { NatalWheel } from "@/components/astrology/NatalWheel";
import { EmptyState } from "@/components/feedback/empty-state";
import { ErrorState } from "@/components/feedback/error-state";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Button } from "@/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

const HOUSE_SYSTEM_LABELS: Record<HouseSystem, string> = {
  placidus: "Placidus",
  whole_sign: "Whole Sign",
  equal: "Equal",
};

/** Rows shown beside the wheel: the Ascendant and the ten planets (Figma 172:598). */
function rowBodies(chart: NatalChart): AstroPosition[] {
  const order = BODY_ORDER.filter((b) => b !== "Midheaven") as readonly string[];
  return chart.positions.filter((p) => order.includes(p.body)).sort((a, b) => order.indexOf(a.body) - order.indexOf(b.body));
}

/** "The Sun reflects… In Virgo… In the third house…" from chart-copy. */
function useInterpretation() {
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

function PlanetRow({ position, mobileAlign, desktopAlign }: { position: AstroPosition; mobileAlign: Align; desktopAlign: Align }) {
  const { text, locale } = useI18n();
  const interpret = useInterpretation();
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
        <Link href={`/${locale}/psychics`} className="mt-2 inline-block text-xs font-semibold underline underline-offset-4">
          {text("Get more insight from one of our advisors ")}
        </Link>
      </AccordionContent>
    </AccordionItem>
  );
}

function ChartTables({ chart }: { chart: NatalChart }) {
  const { text } = useI18n();
  const bodies = chart.positions.filter((p) => (BODY_ORDER as readonly string[]).includes(p.body));
  const legend = ["square", "trine", "sextile"].map((type) => ({ type, style: aspectLineStyle(type)! }));
  return (
    <div className="grid gap-8 text-sm lg:grid-cols-2">
      <table className="w-full border-collapse text-left">
        <caption className="mb-2 text-left font-semibold">{text("Positions")}</caption>
        <thead>
          <tr className="border-b border-gold/50 text-xs tracking-wide uppercase">
            <th scope="col" className="py-2 pr-3 font-semibold">{text("Point")}</th>
            <th scope="col" className="py-2 pr-3 font-semibold">{text("Sign and degree")}</th>
            <th scope="col" className="py-2 pr-3 font-semibold">{text("House")}</th>
            <th scope="col" className="py-2 font-semibold">{text("Motion")}</th>
          </tr>
        </thead>
        <tbody>
          {bodies.map((p) => (
            <tr key={p.body} className="border-b border-gold/20">
              <th scope="row" className="py-1.5 pr-3 font-normal">{text(p.body)}</th>
              <td className="py-1.5 pr-3">
                {text(p.sign)} {p.degree}°{p.minute}′{p.second}″
              </td>
              <td className="py-1.5 pr-3">{p.house ?? "—"}</td>
              <td className="py-1.5">{p.retrograde ? text("Retrograde") : text("Direct")}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="grid content-start gap-8">
        {chart.houses && chart.houses.length > 0 ? (
          <table className="w-full border-collapse text-left">
            <caption className="mb-2 text-left font-semibold">{text("House cusps")}</caption>
            <thead>
              <tr className="border-b border-gold/50 text-xs tracking-wide uppercase">
                <th scope="col" className="py-2 pr-3 font-semibold">{text("House")}</th>
                <th scope="col" className="py-2 font-semibold">{text("Cusp")}</th>
              </tr>
            </thead>
            <tbody>
              {chart.houses.map((h) => (
                <tr key={h.number} className="border-b border-gold/20">
                  <th scope="row" className="py-1.5 pr-3 font-normal">{text(houseLabel(h.number))}</th>
                  <td className="py-1.5">
                    {text(h.sign)} {degreeInSign(h.cusp)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : null}
        <table className="w-full border-collapse text-left">
          <caption className="mb-2 text-left font-semibold">{text("Aspects")}</caption>
          <thead>
            <tr className="border-b border-gold/50 text-xs tracking-wide uppercase">
              <th scope="col" className="py-2 pr-3 font-semibold">{text("Aspect")}</th>
              <th scope="col" className="py-2 pr-3 font-semibold">{text("Orb")}</th>
              <th scope="col" className="py-2 font-semibold">{text("Phase")}</th>
            </tr>
          </thead>
          <tbody>
            {chart.aspects.length === 0 ? (
              <tr>
                <td colSpan={3} className="py-2">
                  {text("No major aspects.")}
                </td>
              </tr>
            ) : (
              chart.aspects.map((a) => (
                <tr key={`${a.a}-${a.b}-${a.type}`} className="border-b border-gold/20">
                  <th scope="row" className="py-1.5 pr-3 font-normal">
                    {text("{value0} {value1} {value2}", { value0: text(a.a), value1: text(ASPECT_NAMES[a.type] ?? a.type).toLowerCase(), value2: text(a.b) })}
                  </th>
                  <td className="py-1.5 pr-3">{a.orb.toFixed(1)}°</td>
                  <td className="py-1.5">{a.applying ? text("Applying") : text("Separating")}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
        <ul className="flex flex-wrap gap-x-6 gap-y-2 text-xs" aria-label={text("Aspect lines")}>
          {legend.map(({ type, style }) => (
            <li key={type} className="flex items-center gap-2">
              <svg width="36" height="6" aria-hidden="true">
                <line x1="0" y1="3" x2="36" y2="3" stroke={style.color} strokeWidth={1.4} strokeDasharray={style.dash} opacity={style.opacity} />
              </svg>
              {text(style.label)}
            </li>
          ))}
          <li>{text("Conjunctions are listed but not drawn.")}</li>
        </ul>
      </div>
    </div>
  );
}

/**
 * The client's chart tab (Figma 172:598 desktop, 867:16922 mobile): the
 * natal wheel drawn from `GET /me/chart`, with every point as an accordion
 * row beside it (rows replace the old popups), a data-table alternative,
 * the house system switch and the client's numbers.
 */
export function ChartTab({ name, user }: { name: string; user: User }) {
  const { text, locale } = useI18n();
  const [houseSystem, setHouseSystem] = React.useState<HouseSystem | undefined>(undefined);
  const [open, setOpen] = React.useState<string>("");
  const p = user.profile;
  const chart = useAsync((signal) => astroApi.myChart(houseSystem, signal), [user.id, p.birth_date, p.birth_time, p.birth_place_id, houseSystem]);
  const numbers = useAsync((signal) => astroApi.myNumerology(signal), [user.id, p.birth_date]);
  const data = chart.data;
  const tracked = React.useRef(false);

  React.useEffect(() => {
    if (data && !tracked.current) {
      tracked.current = true;
      track("chart_viewed", { has_location: data.has_location, time_known: data.time_known });
    }
  }, [data]);

  if (chart.error && chart.error.code === "validation_failed" && chart.error.fields.birth_date) {
    return (
      <EmptyState
        className="my-8"
        title={text("Add your birth date in settings to see your chart.")}
        description={text("Add your birth time and place too for your rising sign and houses.")}
        action={
          <Button asChild variant="cta" size="lg" className="px-4">
            <Link href={`/${locale}/profile/settings`}>{text("Open settings")}</Link>
          </Button>
        }
      />
    );
  }
  if (chart.error && !data) return <ErrorState error={chart.error} onRetry={chart.reload} className="my-8" title={text("Your chart could not be loaded")} />;

  const rows = data ? rowBodies(data) : [];
  const split = Math.ceil(rows.length / 2);
  const birthDate = data ? new Intl.DateTimeFormat(locale, { day: "2-digit", month: "2-digit", year: "numeric", timeZone: "UTC" }).format(new Date(`${data.birth_date}T12:00:00Z`)) : "";
  const warnings = data?.warnings ?? [];

  return (
    <section aria-labelledby="chart-heading" className="flex flex-col gap-6" data-testid="chart-tab" aria-busy={chart.loading}>
      <div className="flex flex-col items-center gap-2 text-center md:sr-only">
        <p className="text-[10px] font-bold tracking-[1.5px] text-[#74571a] uppercase">{name}</p>
        <h2 id="chart-heading" className="text-[30px] leading-[1.08] font-medium text-[#17150f]">
          {data
            ? data.rising_sign
              ? `${text("{value0} sun", { value0: text(data.sun_sign) })} · ${text("{value0} rising", { value0: text(data.rising_sign) })}`
              : text("{value0} sun", { value0: text(data.sun_sign) })
            : text("Your natal chart")}
        </h2>
        <p className="max-w-[340px] text-[13px] leading-normal text-[var(--muted-ink)]">
          {text("Your natal chart is a map of the sky at the precise moment you arrived. ")}
        </p>
      </div>

      {!data ? (
        <div className="flex flex-col items-center gap-6 py-6" aria-busy="true">
          <p role="status" className="sr-only">
            {text("Calculating your chart…")}
          </p>
          <Skeleton className="aspect-square w-full max-w-[420px] rounded-full" />
          <div className="grid w-full grid-cols-2 gap-4 md:max-w-[640px]">
            {Array.from({ length: 6 }, (_, i) => (
              <Skeleton key={i} className="h-16" />
            ))}
          </div>
        </div>
      ) : (
        <Accordion type="single" collapsible value={open} onValueChange={setOpen}>
          {/* One list of rows: two flex columns beside the wheel on desktop; on
              phones the columns dissolve (display: contents) into a two-column
              grid under the wheel (867:16922), and an open row spans both. */}
          <div className="chart-rows grid grid-cols-2 items-start gap-x-3 md:grid-cols-[minmax(0,1fr)_minmax(0,1.6fr)_minmax(0,1fr)] md:gap-x-6 xl:grid-cols-[minmax(0,1fr)_minmax(0,560px)_minmax(0,1fr)]">
            <div className="col-span-2 mx-auto mb-4 w-full max-w-[350px] md:col-span-1 md:col-start-2 md:row-start-1 md:mb-0 md:max-w-[560px] md:self-center">
              <NatalWheel chart={data} highlight={open || null} />
              <p className="mt-3 text-center text-[10px] font-bold tracking-[1.5px] text-[#6d6753] uppercase md:hidden">{birthDate}</p>
            </div>
            <div className="contents md:col-start-1 md:row-start-1 md:flex md:flex-col lg:px-6" data-testid="planet-rows-left">
              {rows.slice(0, split).map((position, i) => (
                <PlanetRow key={position.body} position={position} mobileAlign={i % 2 === 0 ? "start" : "end"} desktopAlign="start" />
              ))}
            </div>
            <div className="contents md:col-start-3 md:row-start-1 md:flex md:flex-col md:self-center lg:px-6" data-testid="planet-rows-right">
              {rows.slice(split).map((position, i) => (
                <PlanetRow key={position.body} position={position} mobileAlign={(split + i) % 2 === 0 ? "start" : "end"} desktopAlign="end" />
              ))}
            </div>
          </div>
        </Accordion>
      )}

      {data ? (
        <>
          <div className="flex flex-col gap-2 text-center text-xs text-[var(--muted-ink)]">
            {!data.time_known ? <p>{text("Add your birth time and place in settings for your rising sign and houses.")}</p> : null}
            {warnings.includes("birth_place_missing") ? (
              <p>{text("Your birth time is saved without a birthplace, so it cannot be used yet. Add your birthplace in settings.")}</p>
            ) : null}
            {warnings.includes("pluto_out_of_range") ? <p>{text("Pluto's position is not available for this date.")}</p> : null}
            {data.fallback_reason ? <p>{text("Placidus is undefined this close to the poles, so Porphyry houses were used.")}</p> : null}
            <p>{text(data.disclaimer)}</p>
            {data.attribution ? <p lang="en">{data.attribution}</p> : null}
          </div>

          <div className="flex flex-wrap items-center justify-center gap-3 text-sm">
            <span id="house-system-label">{text("House system")}</span>
            <Select
              value={houseSystem ?? (HOUSE_SYSTEMS as readonly string[]).find((s) => s === data.house_system) ?? "placidus"}
              onValueChange={(value) => setHouseSystem(value as HouseSystem)}
              disabled={!data.time_known}
            >
              <SelectTrigger className="h-9 w-44" aria-labelledby="house-system-label" data-testid="house-system-select">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {HOUSE_SYSTEMS.map((system) => (
                  <SelectItem key={system} value={system}>
                    {text(HOUSE_SYSTEM_LABELS[system])}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {chart.loading ? (
              <span role="status" className="text-xs">
                {text("Updating…")}
              </span>
            ) : null}
          </div>

          <Collapsible className="rounded-lg border border-gold/50 bg-cream-light/40">
            <CollapsibleTrigger className="group flex w-full items-center justify-between gap-3 px-4 py-3 text-left text-sm font-semibold" data-testid="chart-details-toggle">
              {text("Chart details as tables")}
              <ChevronDown aria-hidden className="size-4 transition-transform group-data-[state=open]:rotate-180 motion-reduce:transition-none" />
            </CollapsibleTrigger>
            <CollapsibleContent className="px-4 pb-5">
              <ChartTables chart={data} />
            </CollapsibleContent>
          </Collapsible>

          <section aria-labelledby="numbers-heading" className="rounded-lg border border-gold/50 px-4 py-4">
            <h3 id="numbers-heading" className="text-sm font-semibold">
              {text("Your numbers")}
            </h3>
            {numbers.error ? (
              <ErrorState error={numbers.error} onRetry={numbers.reload} compact />
            ) : numbers.data ? (
              <dl className="mt-3 grid grid-cols-3 gap-3 text-center" data-testid="my-numbers">
                {(
                  [
                    ["Life Path", numbers.data.life_path],
                    ["Personal year", numbers.data.personal_year],
                    ["Personal month", numbers.data.personal_month],
                  ] as const
                ).map(([label, value]) => (
                  <div key={label} className="flex flex-col gap-1">
                    <dt className="text-[10px] font-semibold tracking-wide uppercase">{text(label)}</dt>
                    <dd className="font-display text-2xl text-[#8a6a1f]">{value}</dd>
                  </div>
                ))}
              </dl>
            ) : (
              <Skeleton className="mt-3 h-12 w-full" />
            )}
          </section>
        </>
      ) : null}
    </section>
  );
}
