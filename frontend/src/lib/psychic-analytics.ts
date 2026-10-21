/**
 * Pure helpers for the advisor analytics dashboard: date ranges, chart data
 * mapping, heatmap levels, score normalisation and withdrawal validation.
 * Kept free of React and Chart.js so they are unit-testable.
 */
import type { AnalyticsInterval, HourLevel, PsychicAnalytics, WithdrawalMethod } from "@/lib/api/types";

export type RangeKey = "7d" | "30d" | "90d" | "12m";
export const RANGE_KEYS: RangeKey[] = ["7d", "30d", "90d", "12m"];

/** Query range for a selector value (`to` is exclusive and is "now"). */
export function rangeFor(key: RangeKey, now: Date = new Date()): { from: string; to: string; interval: AnalyticsInterval } {
  const to = new Date(now.getTime());
  const from = new Date(now.getTime());
  let interval: AnalyticsInterval = "day";
  switch (key) {
    case "7d":
      from.setUTCDate(from.getUTCDate() - 7);
      break;
    case "30d":
      from.setUTCDate(from.getUTCDate() - 30);
      break;
    case "90d":
      from.setUTCDate(from.getUTCDate() - 90);
      interval = "week";
      break;
    case "12m":
      from.setUTCMonth(from.getUTCMonth() - 12);
      interval = "month";
      break;
  }
  return { from: from.toISOString(), to: to.toISOString(), interval };
}

/** The viewer's IANA time zone (falls back to UTC). */
export function viewerTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  } catch {
    return "UTC";
  }
}

const DATE_ONLY = /^\d{4}-\d{2}(-\d{2})?$/;

/** Axis label for a series bucket ("Oct 6", "Oct 26", "14:00"). */
export function bucketLabel(bucket: string, interval: string, locale: string, timeZone?: string): string {
  let date: Date;
  let zone = timeZone;
  if (DATE_ONLY.test(bucket)) {
    // Calendar buckets have no instant: format them as UTC dates.
    date = new Date(`${bucket.length === 7 ? `${bucket}-01` : bucket}T12:00:00Z`);
    zone = "UTC";
  } else {
    date = new Date(bucket);
  }
  if (Number.isNaN(date.getTime())) return bucket;
  const opts: Intl.DateTimeFormatOptions =
    interval === "month"
      ? { month: "short", year: "2-digit" }
      : interval === "hour"
        ? { hour: "numeric", minute: "2-digit" }
        : { month: "short", day: "numeric" };
  try {
    return new Intl.DateTimeFormat(locale, { ...opts, timeZone: zone }).format(date);
  } catch {
    return new Intl.DateTimeFormat(locale, opts).format(date);
  }
}

export type EarningsChartData = { labels: string[]; net: number[]; gross: number[]; sessions: number[] };

/** Map the earnings series to parallel arrays for Chart.js. */
export function earningsChartData(
  series: PsychicAnalytics["earnings"]["series"] | undefined,
  interval: string,
  locale: string,
  timeZone?: string,
): EarningsChartData {
  const rows = [...(series ?? [])].sort((a, b) => a.bucket.localeCompare(b.bucket));
  return {
    labels: rows.map((r) => bucketLabel(r.bucket, interval, locale, timeZone)),
    net: rows.map((r) => r.net_credits ?? 0),
    gross: rows.map((r) => r.gross_credits ?? 0),
    sessions: rows.map((r) => r.sessions ?? 0),
  };
}

/** Line for daily/hourly buckets, bars for weekly/monthly ones. */
export function earningsChartKind(interval: string): "line" | "bar" {
  return interval === "week" || interval === "month" ? "bar" : "line";
}

const LEVELS: HourLevel[] = ["peak", "medium", "low", "none"];

/** Terciles of the non-empty hours: top third peak, middle medium, rest low. */
export function computeLevels(grid: number[][]): HourLevel[][] {
  const values = grid.flat().filter((v) => v > 0).sort((a, b) => a - b);
  if (values.length === 0) return grid.map((row) => row.map(() => "none" as HourLevel));
  const t1 = values[Math.floor(values.length / 3)];
  const t2 = values[Math.floor((2 * values.length) / 3)];
  return grid.map((row) => row.map((v) => (v <= 0 ? "none" : v >= t2 ? "peak" : v >= t1 ? "medium" : "low")));
}

export type HeatCell = { weekday: number; hour: number; sessions: number; level: HourLevel };

/** A complete 7×24 grid (weekday 0 = Monday), using server levels when well-formed. */
export function heatmapCells(hours: PsychicAnalytics["hours"] | undefined): HeatCell[][] {
  const grid = Array.from({ length: 7 }, (_, d) => Array.from({ length: 24 }, (_, h) => Math.max(0, Number(hours?.grid?.[d]?.[h] ?? 0) || 0)));
  const serverLevels = hours?.levels;
  const valid =
    Array.isArray(serverLevels) &&
    serverLevels.length === 7 &&
    serverLevels.every((row) => Array.isArray(row) && row.length === 24 && row.every((l) => LEVELS.includes(l)));
  const levels = valid ? serverLevels : computeLevels(grid);
  return grid.map((row, d) => row.map((sessions, h) => ({ weekday: d, hour: h, sessions, level: sessions > 0 ? levels[d][h] : "none" })));
}

/** The busiest hours (server's pick, else computed). */
export function bestHours(hours: PsychicAnalytics["hours"] | undefined, n = 3): { weekday: number; hour: number; sessions: number }[] {
  if (hours?.best?.length) return hours.best.slice(0, n);
  return heatmapCells(hours)
    .flat()
    .filter((c) => c.sessions > 0)
    .sort((a, b) => b.sessions - a.sessions || a.weekday - b.weekday || a.hour - b.hour)
    .slice(0, n)
    .map(({ weekday, hour, sessions }) => ({ weekday, hour, sessions }));
}

/** Localized weekday name for weekday 0 = Monday. */
export function weekdayName(weekday: number, locale: string, style: "short" | "long" = "short"): string {
  // 2024-01-01 was a Monday.
  const date = new Date(Date.UTC(2024, 0, 1 + weekday, 12));
  return new Intl.DateTimeFormat(locale, { weekday: style, timeZone: "UTC" }).format(date);
}

/** Localized hour label ("14:00" / "2 PM"). */
export function hourLabel(hour: number, locale: string): string {
  return new Intl.DateTimeFormat(locale, { hour: "numeric", timeZone: "UTC" }).format(new Date(Date.UTC(2024, 0, 1, hour)));
}

/** Rates may arrive as 0..1 or 0..100; normalise to 0..1. */
export function ratio(value: number | undefined | null): number {
  const v = Number(value ?? 0);
  if (!Number.isFinite(v) || v <= 0) return 0;
  return Math.min(1, v > 1 ? v / 100 : v);
}

/** Grade from a 0..100 score (§8): ≥85 excellent, ≥70 good, ≥50 fair. */
export function gradeFor(score: number): PsychicAnalytics["score"]["grade"] {
  if (score >= 85) return "excellent";
  if (score >= 70) return "good";
  if (score >= 50) return "fair";
  return "needs_attention";
}

/** Money value of credits ("$12.40"). */
export function creditsValue(credits: number, centsPerCredit: number, currency: string, locale: string): string {
  const cents = Math.round(credits * centsPerCredit);
  try {
    return new Intl.NumberFormat(locale, { style: "currency", currency: currency || "USD" }).format(cents / 100);
  } catch {
    return (cents / 100).toFixed(2);
  }
}

/** Seconds as a short human duration ("45 s", "3 min 20 s"). Returns parts to translate. */
export function splitDuration(seconds: number): { minutes: number; seconds: number } {
  const s = Math.max(0, Math.round(seconds || 0));
  return { minutes: Math.floor(s / 60), seconds: s % 60 };
}

// --- withdrawals --------------------------------------------------------------

export type WithdrawalDraft = { amount: string; method: WithdrawalMethod | ""; destination: string };
export type FieldError = { message: string; values?: Record<string, string | number> };
export type WithdrawalErrors = Partial<Record<keyof WithdrawalDraft, FieldError>>;

const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/;
const ACCOUNT = /^[A-Za-z0-9][A-Za-z0-9 \-./]{6,}[A-Za-z0-9]$/;

/** Validate the withdrawal form (English copy keys; the server re-validates). */
export function validateWithdrawal(
  draft: WithdrawalDraft,
  payout: Pick<PsychicAnalytics["payout"], "available_credits" | "min_withdrawal_credits">,
): { errors: WithdrawalErrors; value?: { amount_credits: number; method: WithdrawalMethod; destination: string } } {
  const errors: WithdrawalErrors = {};
  const raw = draft.amount.trim();
  const amount = Number(raw);
  if (!raw) errors.amount = { message: "Enter an amount." };
  else if (!/^\d+$/.test(raw) || !Number.isSafeInteger(amount)) errors.amount = { message: "Enter a whole number of credits." };
  else if (amount < payout.min_withdrawal_credits)
    errors.amount = { message: "The minimum withdrawal is {min} credits.", values: { min: payout.min_withdrawal_credits } };
  else if (amount > payout.available_credits)
    errors.amount = { message: "You can withdraw at most {max} credits.", values: { max: payout.available_credits } };

  if (draft.method !== "bank_transfer" && draft.method !== "paypal") errors.method = { message: "Choose a payout method." };

  const destination = draft.destination.trim();
  if (!destination) errors.destination = { message: "Enter where we should send the money." };
  else if (destination.length > 200) errors.destination = { message: "Keep this under 200 characters." };
  else if (draft.method === "paypal" && !EMAIL.test(destination)) errors.destination = { message: "Enter the email address of your PayPal account." };
  else if (draft.method === "bank_transfer" && !ACCOUNT.test(destination)) errors.destination = { message: "Enter a valid IBAN or account number." };

  if (Object.keys(errors).length > 0) return { errors };
  return { errors, value: { amount_credits: amount, method: draft.method as WithdrawalMethod, destination } };
}
