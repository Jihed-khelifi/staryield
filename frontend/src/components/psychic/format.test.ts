import { describe, expect, it } from "vitest";

import type { StatsKPIs } from "@/lib/api/types";
import { formatDelta, formatKpi, formatPercent, formatWallet, kpiTrend, kpiValue, rangeDays } from "@/components/psychic/format";

const kpis = (over: Partial<StatsKPIs> = {}): StatsKPIs => ({
  free_to_paid: { pct: 18, numerator: 9, denominator: 50 },
  paid_to_retained: { pct: 22.5, numerator: 9, denominator: 40 },
  paid_minutes: 260,
  retained_repeat_days: { median_days: 4, sample: 12 },
  active_hours: { hours: 42, in_session_hours: 10 },
  ...over,
});

const points = (value: string) => `${value} pts`;
/** French uses (narrow) no-break spaces; compare with plain ones. */
const plain = (value: string) => value.replace(/\s/g, " ");

describe("formatWallet", () => {
  it("shows whole amounts without decimals and the currency code after the number", () => {
    expect(formatWallet(100_000, "usd", "en")).toBe("1,000 USD");
    expect(formatWallet(0, "USD", "en")).toBe("0 USD");
  });
  it("keeps cents when there are any, in the viewer's locale", () => {
    expect(formatWallet(123_456, "EUR", "en")).toBe("1,234.56 EUR");
    expect(formatWallet(123_450, "EUR", "fr")).toBe("1 234,50 EUR");
  });
});

describe("KPI values", () => {
  it("formats each card with its unit", () => {
    expect(formatKpi("free_to_paid", 18, "en")).toBe("18%");
    expect(formatKpi("paid_to_retained", 22.5, "en")).toBe("22.5%");
    expect(formatKpi("paid_minutes", 260, "en")).toBe("260 min");
    expect(formatKpi("retained_repeat_days", 4, "en")).toBe("4 days");
    expect(formatKpi("retained_repeat_days", 1, "en")).toBe("1 day");
    expect(formatKpi("active_hours", 42, "en")).toBe("42 hr");
    expect(plain(formatKpi("retained_repeat_days", 4, "fr"))).toBe("4 jours");
    expect(plain(formatPercent(18, "fr"))).toBe("18 %");
  });

  it("reports missing data as null instead of 0 %", () => {
    const empty = kpis({ free_to_paid: { pct: null, numerator: 0, denominator: 0 }, retained_repeat_days: { median_days: null, sample: 0 } });
    expect(kpiValue(empty, "free_to_paid")).toBeNull();
    expect(kpiValue(empty, "retained_repeat_days")).toBeNull();
    expect(kpiValue(empty, "paid_minutes")).toBe(260);
  });
});

describe("kpiTrend", () => {
  it("compares with the previous period", () => {
    const before = kpis({ free_to_paid: { pct: 14, numerator: 7, denominator: 50 }, paid_minutes: 300 });
    expect(kpiTrend(kpis(), before, "free_to_paid")).toEqual({ direction: "up", delta: 4 });
    expect(kpiTrend(kpis(), before, "paid_minutes")).toEqual({ direction: "down", delta: 40 });
    expect(kpiTrend(kpis(), kpis(), "active_hours")).toEqual({ direction: "flat", delta: 0 });
  });

  it("has no trend when either period has no data", () => {
    const none = kpis({ paid_to_retained: { pct: null, numerator: 0, denominator: 0 } });
    expect(kpiTrend(kpis(), none, "paid_to_retained")).toBeNull();
    expect(kpiTrend(none, kpis(), "paid_to_retained")).toBeNull();
  });

  it("rounds to one decimal so float noise is not a change", () => {
    const a = kpis({ paid_to_retained: { pct: 0.1 + 0.2, numerator: 0, denominator: 0 } });
    const b = kpis({ paid_to_retained: { pct: 0.3, numerator: 0, denominator: 0 } });
    expect(kpiTrend(a, b, "paid_to_retained")).toEqual({ direction: "flat", delta: 0 });
  });

  it("describes the change in points for ratios and in units otherwise", () => {
    expect(formatDelta("free_to_paid", 4, "en", points)).toBe("4 pts");
    expect(formatDelta("paid_minutes", 40, "en", points)).toBe("40 min");
    expect(formatDelta("active_hours", 1.5, "en", points)).toBe("1.5 hr");
  });
});

describe("rangeDays", () => {
  const today = new Date(2026, 10, 5, 23, 30); // 5 Nov 2026, late evening local time
  it("covers whole calendar days ending today", () => {
    expect(rangeDays("7d", today)).toEqual({ from: "2026-10-30", to: "2026-11-05" });
    expect(rangeDays("30d", today)).toEqual({ from: "2026-10-07", to: "2026-11-05" });
    expect(rangeDays("90d", today)).toEqual({ from: "2026-08-08", to: "2026-11-05" });
  });
  it("covers twelve months for 12m", () => {
    expect(rangeDays("12m", today)).toEqual({ from: "2025-11-06", to: "2026-11-05" });
  });
});
