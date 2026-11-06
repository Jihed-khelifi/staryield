/**
 * Pure formatting helpers for the advisor header and overview (unit-tested).
 * Range keys and the viewer's time zone come from `lib/psychic-analytics`.
 */
import type { StatsKPIs } from "@/lib/api/types";
import type { RangeKey } from "@/lib/psychic-analytics";

/** "1,000 USD" (number, then the currency code, as in the design). */
export function formatWallet(cents: number, currency: string, locale: string): string {
  const amount = cents / 100;
  const digits = Number.isInteger(amount) ? 0 : 2;
  const number = new Intl.NumberFormat(locale, { minimumFractionDigits: digits, maximumFractionDigits: 2 }).format(amount);
  return `${number} ${currency.toUpperCase()}`;
}

/** Percentage with at most one decimal ("18%", "18 %" in French). */
export function formatPercent(value: number, locale: string): string {
  return new Intl.NumberFormat(locale, { style: "percent", maximumFractionDigits: 1 }).format(value / 100);
}

export type KpiKey = "free_to_paid" | "paid_to_retained" | "paid_minutes" | "retained_repeat_days" | "active_hours";

export const KPI_ORDER: KpiKey[] = ["free_to_paid", "paid_to_retained", "paid_minutes", "retained_repeat_days", "active_hours"];

/** Ratios are shown in percent; their trend is in percentage points. */
export function isRatio(key: KpiKey): key is "free_to_paid" | "paid_to_retained" {
  return key === "free_to_paid" || key === "paid_to_retained";
}

/** The numeric value behind a KPI card, or null when there is no data yet. */
export function kpiValue(k: StatsKPIs, key: KpiKey): number | null {
  switch (key) {
    case "free_to_paid":
      return k.free_to_paid.pct;
    case "paid_to_retained":
      return k.paid_to_retained.pct;
    case "paid_minutes":
      return k.paid_minutes;
    case "retained_repeat_days":
      return k.retained_repeat_days.median_days;
    case "active_hours":
      return k.active_hours.hours;
  }
}

function unit(value: number, u: "minute" | "day" | "hour", locale: string, display: "short" | "long") {
  return new Intl.NumberFormat(locale, { style: "unit", unit: u, unitDisplay: display, maximumFractionDigits: 1 }).format(value);
}

/**
 * A KPI value in the viewer's locale: "18%", "260 min", "4 days", "42 hr".
 * Units and plurals come from Intl, so no copy is needed per language.
 */
export function formatKpi(key: KpiKey, value: number, locale: string): string {
  switch (key) {
    case "free_to_paid":
    case "paid_to_retained":
      return formatPercent(value, locale);
    case "paid_minutes":
      return unit(value, "minute", locale, "short");
    case "retained_repeat_days":
      return unit(value, "day", locale, "long");
    case "active_hours":
      return unit(value, "hour", locale, "short");
  }
}

export type Trend = { direction: "up" | "down" | "flat"; delta: number } | null;

/** Change versus the previous window; null when either side has no data. */
export function kpiTrend(current: StatsKPIs, previous: StatsKPIs, key: KpiKey): Trend {
  const now = kpiValue(current, key);
  const before = kpiValue(previous, key);
  if (now === null || before === null) return null;
  const delta = Math.round((now - before) * 10) / 10;
  if (delta === 0) return { direction: "flat", delta: 0 };
  return { direction: delta > 0 ? "up" : "down", delta: Math.abs(delta) };
}

/** The size of a change, without a sign: "4 pts" (ratios) or "20 min". */
export function formatDelta(key: KpiKey, delta: number, locale: string, points: (value: string) => string): string {
  if (isRatio(key)) return points(new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(delta));
  return formatKpi(key, delta, locale);
}

function isoDay(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}

/**
 * Calendar-day window ending today (inclusive), as YYYY-MM-DD in the
 * browser's zone; the API resolves the days in the `tz` it is sent.
 */
export function rangeDays(range: RangeKey, today: Date = new Date()): { from: string; to: string } {
  const end = new Date(today.getFullYear(), today.getMonth(), today.getDate());
  const start = new Date(end);
  if (range === "12m") {
    start.setFullYear(start.getFullYear() - 1);
    start.setDate(start.getDate() + 1);
  } else {
    start.setDate(start.getDate() - (Number.parseInt(range, 10) - 1));
  }
  return { from: isoDay(start), to: isoDay(end) };
}
