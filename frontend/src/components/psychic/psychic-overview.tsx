"use client";

import * as React from "react";
import { Popover } from "radix-ui";
import { Info, Minus, Star, TrendingDown, TrendingUp } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { psychicsApi, reviewsApi } from "@/lib/api/endpoints";
import type { AppNotification, Review, StatsKPIs, StatsOverview } from "@/lib/api/types";
import { useRealtimeEvent } from "@/lib/auth/session-provider";
import { formatDate } from "@/lib/format";
import { RANGE_KEYS, viewerTimeZone, type RangeKey } from "@/lib/psychic-analytics";
import { useAsync } from "@/lib/use-async";
import { usePagedList } from "@/lib/use-paged-list";
import { cn } from "@/lib/utils";
import { rangeCopy } from "@/components/analytics/analytics-copy";
import { AdvisorRequests } from "@/components/account/orders";
import { EmptyState } from "@/components/feedback/empty-state";
import { ErrorState } from "@/components/feedback/error-state";
import { CursorPager } from "@/components/ui/pagination";
import { Skeleton, SkeletonList } from "@/components/ui/skeleton";
import { KPI_ORDER, formatDelta, formatKpi, kpiTrend, kpiValue, rangeDays, type KpiKey } from "@/components/psychic/format";

/** Reviews per page in "Recent reviews". */
export const REVIEWS_PAGE_SIZE = 5;

/** Card labels (Figma 1174:2294) and the definitions shown in their tips. */
export const KPI_COPY: Record<KpiKey, { label: string; definition: string }> = {
  free_to_paid: {
    label: "Free to paid",
    definition: "Clients whose first session with you in this period was free, and who then paid for a session.",
  },
  paid_to_retained: {
    label: "Paid to retained",
    definition: "Clients whose first paid session with you was in this period, and who came back for another paid session.",
  },
  paid_minutes: {
    label: "Paid minutes",
    definition: "Billed minutes in sessions that started in this period, not counting free minutes.",
  },
  retained_repeat_days: {
    label: "Retained repeat rate",
    definition: "The typical (median) number of days between paid sessions with returning clients.",
  },
  active_hours: {
    label: "Active hours",
    definition: "Hours you were online or in a live chat during this period.",
  },
};

/** A small "i" button that shows a definition on click, tap or keyboard. */
function DefinitionTip({ label, definition }: { label: string; definition: string }) {
  const { text } = useI18n();
  return (
    <Popover.Root>
      <Popover.Trigger asChild>
        <button type="button" className="kpi-tip" aria-label={text("What does {value0} mean?", { value0: label })}>
          <Info aria-hidden className="size-3.5" />
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content side="top" align="start" sideOffset={6} collisionPadding={16} className="kpi-tip-content" aria-label={label}>
          {definition}
          <Popover.Arrow className="fill-ink" />
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}

function TrendLine({ kpi, current, previous }: { kpi: KpiKey; current: StatsKPIs; previous: StatsKPIs }) {
  const { text, locale } = useI18n();
  const trend = kpiTrend(current, previous, kpi);
  if (kpiValue(current, kpi) === null) return null;
  if (!trend) {
    return <p className="kpi-trend">{text("No earlier data to compare")}</p>;
  }
  const delta = formatDelta(kpi, trend.delta, locale, (value) => text("{value0} pts", { value0: value }));
  const Icon = trend.direction === "up" ? TrendingUp : trend.direction === "down" ? TrendingDown : Minus;
  return (
    <p className="kpi-trend" data-direction={trend.direction}>
      <Icon aria-hidden className="size-3.5 shrink-0" />
      {trend.direction === "up"
        ? text("Up {value0} on the previous period", { value0: delta })
        : trend.direction === "down"
          ? text("Down {value0} on the previous period", { value0: delta })
          : text("Same as the previous period")}
    </p>
  );
}

function KpiCard({ kpi, stats }: { kpi: KpiKey; stats: StatsOverview }) {
  const { text, locale } = useI18n();
  const copy = KPI_COPY[kpi];
  const value = kpiValue(stats.kpis, kpi);
  const since = kpi === "active_hours" ? stats.kpis.active_hours.tracked_since : undefined;
  const late = since && new Date(since).getTime() > new Date(stats.range.from).getTime();
  return (
    <div className="kpi-card" data-testid={`kpi-${kpi}`}>
      <dt className="kpi-label">
        <span>{text(copy.label)}</span>
        <DefinitionTip label={text(copy.label)} definition={text(copy.definition)} />
      </dt>
      <dd className="kpi-value" data-empty={value === null || undefined}>
        {value === null ? text("No data yet") : formatKpi(kpi, value, locale)}
      </dd>
      <dd>
        <TrendLine kpi={kpi} current={stats.kpis} previous={stats.previous} />
        {late ? <p className="kpi-trend">{text("Tracked since {value0}", { value0: formatDate(since, locale) })}</p> : null}
      </dd>
    </div>
  );
}

/** The five "Advisor stats overview" cards for the chosen period. */
function StatsCards({ range }: { range: RangeKey }) {
  const { text } = useI18n();
  const tz = React.useMemo(() => viewerTimeZone(), []);
  const stats = useAsync((signal) => psychicsApi.stats({ ...rangeDays(range), tz }, signal), [range, tz]);
  const { reload } = stats;
  useRealtimeEvent("session.settled", () => reload());

  if (stats.error && !stats.data) {
    return <ErrorState error={stats.error} onRetry={stats.reload} title={text("Your stats could not be loaded.")} />;
  }
  if (!stats.data) {
    return (
      <div className="kpi-grid" role="status" aria-label={text("Loading your stats…")} data-testid="kpi-loading">
        {KPI_ORDER.map((key) => (
          <div key={key} className="kpi-card" aria-hidden>
            <Skeleton className="h-3.5 w-2/3" />
            <Skeleton className="mt-5 h-8 w-1/2" />
          </div>
        ))}
      </div>
    );
  }
  return (
    <dl className={cn("kpi-grid", stats.loading && "opacity-70")} aria-busy={stats.loading || undefined} data-testid="kpi-cards">
      {KPI_ORDER.map((key) => (
        <KpiCard key={key} kpi={key} stats={stats.data!} />
      ))}
    </dl>
  );
}

function ReviewRow({ review }: { review: Review }) {
  const { text, locale } = useI18n();
  const name = review.author?.name || text("A client");
  return (
    <li className="overview-review" data-testid="overview-review">
      <div className="flex items-start justify-between gap-3">
        <p className="text-sm font-semibold">{name}</p>
        <p className="flex shrink-0 items-center gap-3 text-xs">
          <time dateTime={review.created_at} className="text-ink/70">
            {formatDate(review.created_at, locale)}
          </time>
          <span className="overview-review-rating">
            <span aria-hidden>{review.rating}</span>
            <Star aria-hidden className={cn("size-4", review.rating >= 4 && "fill-amber")} />
            <span className="sr-only">{text("{count} out of 5 stars", { count: review.rating })}</span>
          </span>
        </p>
      </div>
      {review.comment ? <p className="mt-2 text-sm whitespace-pre-wrap">{review.comment}</p> : null}
    </li>
  );
}

/** "Recent reviews", five per page, refreshed when a new review arrives. */
function RecentReviews() {
  const { text } = useI18n();
  const headingId = React.useId();
  const list = usePagedList<Review>((cursor, signal) => reviewsApi.mine(cursor, signal, REVIEWS_PAGE_SIZE), []);
  const { reset, reload, error } = list;
  useRealtimeEvent<{ notification: AppNotification }>("notification.created", (d) => {
    if (d?.notification?.kind !== "review.received") return;
    // reset() alone would not refetch a first page that failed.
    if (error) reload();
    else reset();
  });

  return (
    <section className="overview-card" aria-labelledby={headingId} data-testid="overview-reviews">
      <h2 id={headingId} className="overview-card-title">
        {text("Recent reviews")}
      </h2>
      {list.error ? (
        <ErrorState error={list.error} onRetry={list.reload} compact />
      ) : list.loading ? (
        <div role="status" aria-label={text("Loading reviews…")}>
          <SkeletonList rows={3} rowClassName="h-16 bg-cream" />
        </div>
      ) : list.items.length === 0 ? (
        <EmptyState
          icon={<Star />}
          title={text("No reviews yet")}
          description={text("Reviews from your clients appear here after their chats.")}
        />
      ) : (
        <ul className="grid gap-2">
          {list.items.map((review) => (
            <ReviewRow key={review.id} review={review} />
          ))}
        </ul>
      )}
      <CursorPager
        page={list.page}
        hasPrev={list.hasPrev}
        hasNext={list.hasNext}
        onPrev={list.prev}
        onNext={list.next}
        loading={list.loading}
        label={text("Review pages")}
      />
    </section>
  );
}

function RangePicker({ value, onChange }: { value: RangeKey; onChange: (next: RangeKey) => void }) {
  const { text } = useI18n();
  const name = React.useId();
  return (
    <fieldset className="range-picker" data-testid="overview-range">
      <legend className="sr-only">{text("Period")}</legend>
      {RANGE_KEYS.map((key) => (
        <label key={key} className={cn("status-pill range-pill", value === key && "online")}>
          <input type="radio" name={name} className="sr-only" checked={value === key} onChange={() => onChange(key)} />
          {text(rangeCopy[key])}
        </label>
      ))}
    </fieldset>
  );
}

/**
 * Advisor home (Figma 1174:2294): stats for a period with trends, recent
 * reviews and custom requests.
 */
export function PsychicOverview() {
  const { text } = useI18n();
  const [range, setRange] = React.useState<RangeKey>("30d");
  return (
    <div className="advisor-overview" data-testid="psychic-overview">
      <div className="advisor-overview-head">
        <div>
          <h1 className="advisor-title">{text("Advisor stats overview")}</h1>
          <p className="mt-2 text-sm text-ink/75">{text("Performance snapshot for your recent sessions.")}</p>
        </div>
        <RangePicker value={range} onChange={setRange} />
      </div>
      <StatsCards range={range} />
      <RecentReviews />
      <AdvisorRequests />
    </div>
  );
}
