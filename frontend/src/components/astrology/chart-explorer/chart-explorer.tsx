"use client";

import * as React from "react";
import { ChevronDown } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { HOUSE_SYSTEMS, type HouseSystem, type NatalChart } from "@/lib/api/astrology";
import { errorMessage, type ApiError } from "@/lib/api/errors";
import { NatalWheel } from "@/components/astrology/NatalWheel";
import { ChartBalance } from "@/components/astrology/chart-explorer/chart-balance";
import { ChartTables } from "@/components/astrology/chart-explorer/chart-tables";
import { PlanetRow, rowBodies } from "@/components/astrology/chart-explorer/planet-rows";
import { Accordion } from "@/components/ui/accordion";
import { Button } from "@/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";

const HOUSE_SYSTEM_LABELS: Record<HouseSystem, string> = {
  placidus: "Placidus",
  whole_sign: "Whole Sign",
  equal: "Equal",
};

export type ChartExplorerProps = {
  /** The chart to explore; null while the first one is being calculated (skeleton). */
  chart: NatalChart | null;
  /** The selected house system; undefined shows the chart's own. */
  houseSystem?: HouseSystem;
  onHouseSystemChange: (system: HouseSystem) => void;
  /** A new chart is being calculated: the previous one stays on screen (aria-busy, "Updating…"). */
  updating?: boolean;
  /** The last update failed: an inline message with "Try again" under the select. */
  updateError?: ApiError | null;
  onRetry?: () => void;
  /** Shown when the chart has no birth time (each page says where to add it). */
  timeNote: React.ReactNode;
  /** Shown when a birth time was saved without a birthplace. */
  placeNote: React.ReactNode;
};

/**
 * The natal chart, explored: the wheel with every point as an accordion row
 * beside it (Figma 172:598 desktop, 867:16922 mobile), notes and warnings,
 * the house system switch, the element and modality balance and the chart as
 * tables. One implementation for the signed-in chart (`/profile/chart`) and
 * the public natal chart calculator: each passes its own chart, house system
 * handling and the notes about missing birth data.
 */
export function ChartExplorer({ chart, houseSystem, onHouseSystemChange, updating = false, updateError = null, onRetry, timeNote, placeNote }: ChartExplorerProps) {
  const { text, locale } = useI18n();
  const [open, setOpen] = React.useState<string>("");
  const selectLabelId = React.useId();

  if (!chart) {
    return (
      <div className="flex flex-col items-center gap-6 py-6" aria-busy="true" data-testid="chart-explorer-loading">
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
    );
  }

  const rows = rowBodies(chart);
  const split = Math.ceil(rows.length / 2);
  const birthDate = new Intl.DateTimeFormat(locale, { day: "2-digit", month: "2-digit", year: "numeric", timeZone: "UTC" }).format(new Date(`${chart.birth_date}T12:00:00Z`));
  const warnings = chart.warnings ?? [];
  const selected = houseSystem ?? (HOUSE_SYSTEMS as readonly string[]).find((s) => s === chart.house_system) ?? "placidus";

  return (
    <div className="flex flex-col gap-6" aria-busy={updating} data-testid="chart-explorer">
      <Accordion type="single" collapsible value={open} onValueChange={setOpen}>
        {/* One list of rows: two flex columns beside the wheel on desktop; on
            phones the columns dissolve (display: contents) into a two-column
            grid under the wheel (867:16922), and an open row spans both. */}
        <div className="chart-rows grid grid-cols-2 items-start gap-x-3 md:grid-cols-[minmax(0,1fr)_minmax(0,1.6fr)_minmax(0,1fr)] md:gap-x-6 xl:grid-cols-[minmax(0,1fr)_minmax(0,560px)_minmax(0,1fr)]">
          <div className="col-span-2 mx-auto mb-4 w-full max-w-[350px] md:col-span-1 md:col-start-2 md:row-start-1 md:mb-0 md:max-w-[560px] md:self-center">
            <NatalWheel chart={chart} highlight={open || null} />
            <p className="mt-3 text-center text-[10px] font-bold tracking-[1.5px] text-[#5c5646] uppercase md:hidden">{birthDate}</p>
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

      <div className="flex flex-col gap-2 text-center text-xs text-[var(--muted-ink)]">
        {!chart.time_known ? <p>{timeNote}</p> : null}
        {warnings.includes("birth_place_missing") ? <p>{placeNote}</p> : null}
        {warnings.includes("pluto_out_of_range") ? <p>{text("Pluto's position is not available for this date.")}</p> : null}
        {chart.fallback_reason ? <p>{text("Placidus is undefined this close to the poles, so Porphyry houses were used.")}</p> : null}
        <p>{text(chart.disclaimer)}</p>
        {chart.attribution ? <p lang="en">{chart.attribution}</p> : null}
      </div>

      <div className="flex flex-col items-center gap-2">
        <div className="flex flex-wrap items-center justify-center gap-3 text-sm">
          <span id={selectLabelId}>{text("House system")}</span>
          <Select value={selected} onValueChange={(value) => onHouseSystemChange(value as HouseSystem)} disabled={!chart.time_known}>
            <SelectTrigger className="h-9 w-44" aria-labelledby={selectLabelId} data-testid="house-system-select">
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
          {updating ? (
            <span role="status" className="text-xs">
              {text("Updating…")}
            </span>
          ) : null}
        </div>
        {updateError ? (
          <div role="alert" className="flex flex-wrap items-center justify-center gap-3 rounded-lg border border-danger/50 bg-danger/5 px-3 py-2 text-sm" data-testid="chart-update-error">
            <span>{text("We could not update your houses. {value0}", { value0: text(errorMessage(updateError)) })}</span>
            {onRetry ? (
              <Button type="button" variant="outline" size="sm" className="border-gold bg-cream" onClick={onRetry} data-testid="chart-update-retry">
                {text("Try again")}
              </Button>
            ) : null}
          </div>
        ) : null}
      </div>

      <ChartBalance chart={chart} />

      <Collapsible className="rounded-lg border border-gold/50 bg-cream-light/40">
        <CollapsibleTrigger className="group flex w-full items-center justify-between gap-3 px-4 py-3 text-left text-sm font-semibold" data-testid="chart-details-toggle">
          {text("Chart details as tables")}
          <ChevronDown aria-hidden className="size-4 transition-transform group-data-[state=open]:rotate-180 motion-reduce:transition-none" />
        </CollapsibleTrigger>
        <CollapsibleContent className="px-4 pb-5">
          <ChartTables chart={chart} />
        </CollapsibleContent>
      </Collapsible>
    </div>
  );
}
