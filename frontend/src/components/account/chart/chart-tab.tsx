"use client";

import * as React from "react";
import Link from "next/link";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { astroApi, type HouseSystem } from "@/lib/api/astrology";
import type { User } from "@/lib/api/types";
import { useAsync } from "@/lib/use-async";
import { ChartExplorer } from "@/components/astrology/chart-explorer/chart-explorer";
import { NumbersSummary } from "@/components/astrology/chart-explorer/numbers-summary";
import { EmptyState } from "@/components/feedback/empty-state";
import { ErrorState } from "@/components/feedback/error-state";
import { Button } from "@/components/ui/button";

/**
 * The client's chart tab (Figma 172:598 desktop, 867:16922 mobile): the
 * natal chart from `GET /me/chart` in the shared chart explorer (the same
 * one as the public natal chart calculator) and the client's numbers from
 * `GET /me/numerology`.
 */
export function ChartTab({ name, user }: { name: string; user: User }) {
  const { text, locale } = useI18n();
  const [houseSystem, setHouseSystem] = React.useState<HouseSystem | undefined>(undefined);
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

      <ChartExplorer
        chart={data}
        houseSystem={houseSystem}
        onHouseSystemChange={setHouseSystem}
        updating={chart.loading && Boolean(data)}
        updateError={data ? chart.error : null}
        onRetry={chart.reload}
        timeNote={text("Add your birth time and place in settings for your rising sign and houses.")}
        placeNote={text("Your birth time is saved without a birthplace, so it cannot be used yet. Add your birthplace in settings.")}
      />

      {data ? (
        <NumbersSummary
          numbers={numbers.data ? { lifePath: numbers.data.life_path, personalYear: numbers.data.personal_year, personalMonth: numbers.data.personal_month } : null}
          error={numbers.error}
          onRetry={numbers.reload}
        />
      ) : null}
    </section>
  );
}
