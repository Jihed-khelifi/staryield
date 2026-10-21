"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { psychicsApi, reviewsApi } from "@/lib/api/endpoints";
import { errorMessage } from "@/lib/api/errors";
import type { AppNotification, Page, PsychicAnalytics, Review } from "@/lib/api/types";
import { useRealtimeEvent } from "@/lib/auth/session-provider";
import { formatRating } from "@/lib/format";
import {
  RANGE_KEYS,
  bestHours,
  creditsValue,
  earningsChartData,
  earningsChartKind,
  heatmapCells,
  hourLabel,
  rangeFor,
  ratio,
  viewerTimeZone,
  weekdayName,
  type HeatCell,
  type RangeKey,
} from "@/lib/psychic-analytics";
import { useAsync, type AsyncState } from "@/lib/use-async";
import { cn } from "@/lib/utils";
import { EarningsChart, ScoreGauge } from "@/components/analytics/charts";
import { componentCopy, gradeCopy, levelCopy, rangeCopy } from "@/components/analytics/analytics-copy";
import { Withdrawals } from "@/components/analytics/withdrawals";
import { ReviewItem } from "@/components/reviews/psychic-reviews";
import { StarDisplay } from "@/components/reviews/star-rating";

function Card({ title, children, className, action, testId }: { title: string; children: React.ReactNode; className?: string; action?: React.ReactNode; testId?: string }) {
  const id = React.useId();
  return (
    <section className={cn("analytics-card", className)} aria-labelledby={id} data-testid={testId}>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 id={id} className="text-xl font-semibold">
          {title}
        </h2>
        {action}
      </div>
      {children}
    </section>
  );
}

function Kpi({ label, value, detail, testId }: { label: string; value: string; detail?: string; testId?: string }) {
  return (
    <div className="stat-tile analytics-kpi" data-testid={testId}>
      <p className="text-[10px] uppercase">{label}</p>
      <p className="mt-2 text-2xl">{value}</p>
      {detail ? <span className="text-xs">{detail}</span> : null}
    </div>
  );
}

export function PsychicAnalyticsView() {
  const { text, t, locale } = useI18n();
  const [range, setRange] = React.useState<RangeKey>("30d");
  const tz = React.useMemo(() => viewerTimeZone(), []);
  const analytics = useAsync((signal) => psychicsApi.analytics({ ...rangeFor(range), tz }, signal), [range, tz]);
  const reviews = useAsync((signal) => reviewsApi.mine(undefined, signal), []);
  const [withdrawalsNonce, setWithdrawalsNonce] = React.useState(0);

  useRealtimeEvent("session.settled", () => analytics.reload());
  useRealtimeEvent<{ notification: AppNotification }>("notification.created", (d) => {
    const kind = d?.notification?.kind;
    if (kind === "withdrawal.updated") {
      analytics.reload();
      setWithdrawalsNonce((n) => n + 1);
    }
    if (kind === "review.received") {
      analytics.reload();
      reviews.reload();
    }
  });

  const a = analytics.data;
  const number = new Intl.NumberFormat(locale);
  const percent = (v: number) => new Intl.NumberFormat(locale, { style: "percent", maximumFractionDigits: 0 }).format(ratio(v));
  const unit = (v: number, u: "second" | "minute", digits = 0) =>
    new Intl.NumberFormat(locale, { style: "unit", unit: u, unitDisplay: "short", maximumFractionDigits: digits }).format(v);

  return (
    <div className="analytics" data-testid="psychic-analytics">
      <div className="analytics-head">
        <div>
          <h1 className="text-[28px] font-semibold">{text("Analytics")}</h1>
          <p className="text-sm font-light">{text("Your performance, earnings and payouts. Times are shown in {value0}.", { value0: tz })}</p>
        </div>
        <fieldset className="range-picker">
          <legend className="sr-only">{text("Period")}</legend>
          {RANGE_KEYS.map((key) => (
            <label key={key} className={cn("status-pill range-pill", range === key && "online")}>
              <input type="radio" name="analytics-range" className="sr-only" checked={range === key} onChange={() => setRange(key)} />
              {text(rangeCopy[key])}
            </label>
          ))}
        </fieldset>
      </div>

      {analytics.error ? (
        <div className="analytics-card" role="alert">
          <p className="text-sm">{text(errorMessage(analytics.error))}</p>
          <button type="button" className="mt-2 text-sm underline" onClick={analytics.reload}>
            {text("Try again")}
          </button>
        </div>
      ) : !a ? (
        <p className="py-12 text-center text-sm" role="status">
          {text("Loading your analytics…")}
        </p>
      ) : (
        <div className={cn("analytics-grid", analytics.loading && "opacity-70")} aria-busy={analytics.loading}>
          <div className="analytics-kpis">
            <Kpi
              testId="kpi-net"
              label={text("Net earnings")}
              value={number.format(a.earnings.net_credits)}
              detail={text("Worth {value0}", { value0: creditsValue(a.earnings.net_credits, a.payout.cents_per_credit, a.payout.currency, locale) })}
            />
            <Kpi label={text("Gross earnings")} value={number.format(a.earnings.gross_credits)} detail={text("Commission: {value0}", { value0: number.format(a.earnings.commission_credits) })} />
            <Kpi testId="kpi-sessions" label={text("Sessions")} value={number.format(a.sessions.completed)} detail={text("Requested: {value0}", { value0: number.format(a.sessions.requested) })} />
            <Kpi label={text("Minutes")} value={number.format(a.sessions.total_minutes)} />
            <Kpi label={text("Acceptance rate")} value={percent(a.sessions.acceptance_rate)} detail={text("Declined: {value0} · Missed: {value1}", { value0: a.sessions.declined, value1: a.sessions.expired })} />
            <Kpi label={text("Average response")} value={unit(a.sessions.avg_response_seconds, "second")} />
            <Kpi label={text("Average chat")} value={unit(a.sessions.avg_duration_seconds / 60, "minute", 1)} />
            <Kpi
              label={text("Clients")}
              value={number.format(a.clients.unique)}
              detail={text("Returning: {value0} · New: {value1}", { value0: number.format(a.clients.returning), value1: number.format(a.clients.new) })}
            />
            <Kpi
              label={text("Rating")}
              value={a.rating.count ? `${formatRating(a.rating.average, locale)} ★` : "—"}
              detail={t("analytics.ratings", { count: a.rating.count })}
            />
          </div>

          <EarningsCard a={a} />
          <div className="analytics-two">
            <ScoreCard score={a.score} />
            <Card title={text("Rating distribution")}>
              <RatingBars rating={a.rating} />
            </Card>
          </div>
          <HoursCard hours={a.hours} />
          <div className="analytics-two">
            <Card title={text("Top clients")}>
              {a.clients.top.length ? (
                <table className="analytics-table">
                  <thead>
                    <tr>
                      <th scope="col">{text("Client")}</th>
                      <th scope="col">{text("Sessions")}</th>
                      <th scope="col">{text("Minutes")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {a.clients.top.map((c, i) => (
                      <tr key={`${c.name}-${i}`}>
                        <td>{c.name}</td>
                        <td>{number.format(c.sessions)}</td>
                        <td>{number.format(c.minutes)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              ) : (
                <p className="text-sm font-light">{text("Your most loyal clients will appear here.")}</p>
              )}
            </Card>
            <Card title={text("Recent reviews")} testId="analytics-reviews">
              <RecentReviews reviews={reviews} />
            </Card>
          </div>
        </div>
      )}

      <Withdrawals key={withdrawalsNonce} payout={a?.payout ?? null} onChanged={analytics.reload} />
    </div>
  );
}

function RecentReviews({ reviews }: { reviews: AsyncState<Page<Review>> }) {
  const { text } = useI18n();
  if (reviews.error)
    return (
      <p role="alert" className="text-sm">
        {text(errorMessage(reviews.error))}
      </p>
    );
  if (reviews.loading && !reviews.data)
    return (
      <p className="text-sm" role="status">
        {text("Loading reviews…")}
      </p>
    );
  if (!reviews.data?.items.length) return <p className="text-sm font-light">{text("No reviews yet. Clients can rate their chat once it ends.")}</p>;
  return (
    <ul className="grid gap-3">
      {reviews.data.items.slice(0, 3).map((r) => (
        <ReviewItem key={r.id} review={r} />
      ))}
    </ul>
  );
}

function EarningsCard({ a }: { a: PsychicAnalytics }) {
  const { text, locale } = useI18n();
  const interval = a.range.interval || "day";
  const data = earningsChartData(a.earnings.series, interval, locale, a.range.tz);
  const kind = earningsChartKind(interval);
  const number = new Intl.NumberFormat(locale);
  const total = number.format(a.earnings.net_credits);
  const empty = data.net.every((v) => v === 0) && data.gross.every((v) => v === 0);
  return (
    <Card title={text("Earnings")} testId="earnings-card">
      {empty ? (
        <p className="text-sm font-light">{text("No earnings in this period yet.")}</p>
      ) : (
        <>
          <div className="analytics-chart">
            <EarningsChart
              kind={kind}
              labels={data.labels}
              net={data.net}
              gross={data.gross}
              netLabel={text("Net credits")}
              grossLabel={text("Gross credits")}
              ariaLabel={text("Earnings chart: {value0} net credits in this period.", { value0: total })}
              locale={locale}
            />
          </div>
          <details className="mt-3 text-sm">
            <summary className="cursor-pointer font-ui text-xs underline">{text("Show data table")}</summary>
            <div className="mt-2 max-h-64 overflow-auto">
              <table className="analytics-table">
                <thead>
                  <tr>
                    <th scope="col">{text("Period")}</th>
                    <th scope="col">{text("Net credits")}</th>
                    <th scope="col">{text("Gross credits")}</th>
                    <th scope="col">{text("Sessions")}</th>
                  </tr>
                </thead>
                <tbody>
                  {data.labels.map((label, i) => (
                    <tr key={`${label}-${i}`}>
                      <td>{label}</td>
                      <td>{number.format(data.net[i])}</td>
                      <td>{number.format(data.gross[i])}</td>
                      <td>{number.format(data.sessions[i])}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </details>
        </>
      )}
    </Card>
  );
}

function ScoreCard({ score }: { score: PsychicAnalytics["score"] }) {
  const { text, locale } = useI18n();
  const grade = gradeCopy[score.grade] ?? gradeCopy.needs_attention;
  const value = Math.round(Math.max(0, Math.min(100, score.value)));
  const percent = (v: number) => new Intl.NumberFormat(locale, { style: "percent", maximumFractionDigits: 0 }).format(v);
  const weakest = [...componentCopy].sort((x, y) => ratio(score.components[x.key]) - ratio(score.components[y.key]))[0];
  return (
    <Card title={text("Success score")} testId="score-card">
      <div className="score-layout">
        <div className="score-gauge">
          <ScoreGauge value={value} ariaLabel={text("Success score: {value0} out of 100", { value0: value })} />
          <div className="score-value" aria-hidden>
            <span className="font-display text-4xl">{value}</span>
            <span className="font-ui text-[11px] opacity-70">/ 100</span>
          </div>
        </div>
        <div className="score-text">
          <p className={cn("score-grade", `score-grade-${score.grade}`)}>{text(grade.label)}</p>
          <p className="mt-2 text-sm leading-relaxed font-light">{text(grade.guidance)}</p>
          {weakest && ratio(score.components[weakest.key]) < 0.85 ? (
            <p className="mt-2 text-xs">{text("Your biggest opportunity: {value0}.", { value0: text(weakest.label).toLowerCase() })}</p>
          ) : null}
        </div>
      </div>
      <ul className="score-components">
        {componentCopy.map((c) => {
          const v = ratio(score.components[c.key]);
          return (
            <li key={c.key}>
              <div className="flex items-baseline justify-between gap-2 text-xs">
                <span>
                  {text(c.label)} <span className="opacity-60">{text("({value0}% of the score)", { value0: c.weight })}</span>
                </span>
                <strong>{percent(v)}</strong>
              </div>
              <span className="analytics-bar" aria-hidden>
                <span style={{ width: `${Math.round(v * 100)}%` }} />
              </span>
            </li>
          );
        })}
      </ul>
    </Card>
  );
}

function HoursCard({ hours }: { hours: PsychicAnalytics["hours"] }) {
  const { text, t, locale } = useI18n();
  const cells = heatmapCells(hours);
  const best = bestHours(hours);
  const [focus, setFocus] = React.useState<HeatCell | null>(null);
  const describe = (c: { weekday: number; hour: number; sessions: number }) =>
    t("analytics.requestsAt", { day: weekdayName(c.weekday, locale, "long"), hour: hourLabel(c.hour, locale), count: c.sessions });
  const hoursAxis = Array.from({ length: 24 }, (_, h) => h);
  return (
    <Card title={text("Working hours")} testId="hours-card">
      <p className="mb-3 text-sm font-light">{text("When clients ask for readings, by day and hour. Be online at your peak hours to get more chats.")}</p>
      <div className="heatmap-scroll">
        <div className="heatmap" aria-hidden>
          <span />
          {hoursAxis.map((h) => (
            <span key={h} className="heatmap-hour">
              {h % 3 === 0 ? hourLabel(h, locale) : ""}
            </span>
          ))}
          {cells.map((row, d) => (
            <React.Fragment key={d}>
              <span className="heatmap-day">{weekdayName(d, locale)}</span>
              {row.map((c) => (
                <span
                  key={c.hour}
                  className="heatmap-cell"
                  data-level={c.level}
                  title={describe(c)}
                  onMouseEnter={() => setFocus(c)}
                  onMouseLeave={() => setFocus(null)}
                />
              ))}
            </React.Fragment>
          ))}
        </div>
      </div>
      <p className="mt-2 min-h-5 font-ui text-xs" aria-hidden>
        {focus ? describe(focus) : ""}
      </p>
      <div className="heatmap-legend">
        {(["peak", "medium", "low", "none"] as const).map((level) => (
          <span key={level} className="flex items-center gap-1.5">
            <span className="heatmap-cell heatmap-swatch" data-level={level} aria-hidden />
            {text(levelCopy[level])}
          </span>
        ))}
      </div>
      <div className="mt-4">
        <h3 className="text-sm font-semibold">{text("Best hours")}</h3>
        {best.length ? (
          <ol className="mt-2 flex flex-wrap gap-2">
            {best.map((b) => (
              <li key={`${b.weekday}-${b.hour}`} className="status-pill">
                {describe(b)}
              </li>
            ))}
          </ol>
        ) : (
          <p className="mt-1 text-sm font-light">{text("Your best hours will appear after your first chat requests.")}</p>
        )}
      </div>
      {/* sr-only on a wrapper: a table cannot shrink to 1px and would widen the page. */}
      <div className="sr-only">
        <table>
          <caption>{text("Chat requests by day and hour")}</caption>
          <thead>
            <tr>
              <th scope="col">{text("Day")}</th>
              {hoursAxis.map((h) => (
                <th key={h} scope="col">
                  {hourLabel(h, locale)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {cells.map((row, d) => (
              <tr key={d}>
                <th scope="row">{weekdayName(d, locale, "long")}</th>
                {row.map((c) => (
                  <td key={c.hour}>{c.sessions}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}

function RatingBars({ rating }: { rating: PsychicAnalytics["rating"] }) {
  const { text, t, locale } = useI18n();
  const counts = (["5", "4", "3", "2", "1"] as const).map((k) => ({ stars: Number(k), count: Number(rating.distribution?.[k] ?? 0) }));
  const max = Math.max(1, ...counts.map((c) => c.count));
  if (!rating.count) return <p className="text-sm font-light">{text("No ratings in this period yet.")}</p>;
  return (
    <div>
      <div className="mb-3 flex items-center gap-2">
        <StarDisplay rating={rating.average} />
        <span className="text-sm">{t("analytics.averageOf", { average: formatRating(rating.average, locale), count: rating.count })}</span>
      </div>
      <ul className="space-y-2">
        {counts.map((c) => (
          <li key={c.stars} className="rating-row">
            <span className="w-10 shrink-0 text-xs">{c.stars} ★</span>
            <span className="analytics-bar flex-1" aria-hidden>
              <span style={{ width: `${(c.count / max) * 100}%` }} />
            </span>
            <span className="w-8 shrink-0 text-right text-xs tabular-nums">{c.count.toLocaleString(locale)}</span>
            <span className="sr-only">{text("{count} out of 5 stars", { count: c.stars })}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
