import { describe, expect, it } from "vitest";

import en from "@/i18n/dictionaries/copy-en.json";
import es from "@/i18n/dictionaries/copy-es.json";
import fr from "@/i18n/dictionaries/copy-fr.json";
import type { PsychicAnalytics } from "@/lib/api/types";
import {
  bestHours,
  bucketLabel,
  computeLevels,
  creditsValue,
  earningsChartData,
  earningsChartKind,
  gradeFor,
  heatmapCells,
  rangeFor,
  ratio,
  validateWithdrawal,
  weekdayName,
} from "@/lib/psychic-analytics";
import { componentCopy, gradeCopy, levelCopy, methodLabel, rangeCopy, withdrawalStatusLabel } from "@/components/analytics/analytics-copy";

const emptyGrid = () => Array.from({ length: 7 }, () => Array.from({ length: 24 }, () => 0));

describe("rangeFor", () => {
  const now = new Date("2026-10-20T12:00:00Z");
  it("maps selector values to from/to/interval", () => {
    expect(rangeFor("7d", now)).toEqual({ from: "2026-10-13T12:00:00.000Z", to: "2026-10-20T12:00:00.000Z", interval: "day" });
    expect(rangeFor("30d", now).interval).toBe("day");
    expect(rangeFor("90d", now)).toMatchObject({ from: "2026-07-22T12:00:00.000Z", interval: "week" });
    expect(rangeFor("12m", now)).toMatchObject({ from: "2025-10-20T12:00:00.000Z", interval: "month" });
  });
});

describe("earnings series", () => {
  const series: PsychicAnalytics["earnings"]["series"] = [
    { bucket: "2026-10-02", net_credits: 80, gross_credits: 120, sessions: 2 },
    { bucket: "2026-10-01", net_credits: 40, gross_credits: 60, sessions: 1 },
  ];

  it("sorts buckets and maps parallel arrays", () => {
    const d = earningsChartData(series, "day", "en", "Europe/Paris");
    expect(d.labels).toEqual(["Oct 1", "Oct 2"]);
    expect(d.net).toEqual([40, 80]);
    expect(d.gross).toEqual([60, 120]);
    expect(d.sessions).toEqual([1, 2]);
  });

  it("labels buckets per interval and locale", () => {
    expect(bucketLabel("2026-10-01", "day", "fr")).toBe(new Intl.DateTimeFormat("fr", { month: "short", day: "numeric", timeZone: "UTC" }).format(new Date("2026-10-01T12:00:00Z")));
    expect(bucketLabel("2026-10-01T00:00:00Z", "month", "en", "UTC")).toBe("Oct 26");
    expect(bucketLabel("2026-10", "month", "en")).toBe("Oct 26");
    expect(bucketLabel("2026-10-01T14:00:00Z", "hour", "en", "UTC")).toBe(new Intl.DateTimeFormat("en", { hour: "numeric", minute: "2-digit", timeZone: "UTC" }).format(new Date("2026-10-01T14:00:00Z")));
    expect(bucketLabel("garbage", "day", "en")).toBe("garbage");
  });

  it("handles a missing series and picks the chart type", () => {
    expect(earningsChartData(undefined, "day", "en")).toEqual({ labels: [], net: [], gross: [], sessions: [] });
    expect(earningsChartKind("day")).toBe("line");
    expect(earningsChartKind("hour")).toBe("line");
    expect(earningsChartKind("week")).toBe("bar");
    expect(earningsChartKind("month")).toBe("bar");
  });
});

describe("heatmap", () => {
  it("splits hours with sessions into terciles", () => {
    const grid = emptyGrid();
    grid[0][9] = 1;
    grid[1][10] = 2;
    grid[2][18] = 5;
    const levels = computeLevels(grid);
    expect(levels[0][9]).toBe("low");
    expect(levels[1][10]).toBe("medium");
    expect(levels[2][18]).toBe("peak");
    expect(levels[3][3]).toBe("none");
  });

  it("marks everything none without sessions and a single busy hour as peak", () => {
    expect(computeLevels(emptyGrid()).flat().every((l) => l === "none")).toBe(true);
    const grid = emptyGrid();
    grid[4][20] = 3;
    expect(computeLevels(grid)[4][20]).toBe("peak");
  });

  it("uses well-formed server levels and repairs malformed input", () => {
    const grid = emptyGrid();
    grid[0][0] = 4;
    const levels = emptyGrid().map((row) => row.map(() => "none" as const)) as PsychicAnalytics["hours"]["levels"];
    levels[0][0] = "medium";
    expect(heatmapCells({ grid, levels, best: [] })[0][0]).toEqual({ weekday: 0, hour: 0, sessions: 4, level: "medium" });
    // Wrong shape: levels are computed client-side.
    expect(heatmapCells({ grid, levels: [["peak"]], best: [] })[0][0].level).toBe("peak");
    // Ragged grid: missing cells are zero.
    const cells = heatmapCells({ grid: [[1]], levels: [], best: [] });
    expect(cells).toHaveLength(7);
    expect(cells.every((row) => row.length === 24)).toBe(true);
    expect(cells[6][23]).toEqual({ weekday: 6, hour: 23, sessions: 0, level: "none" });
  });

  it("best hours: server pick first, else the busiest cells", () => {
    expect(bestHours({ grid: emptyGrid(), levels: [], best: [{ weekday: 2, hour: 19, sessions: 9 }] })).toEqual([{ weekday: 2, hour: 19, sessions: 9 }]);
    const grid = emptyGrid();
    grid[1][9] = 2;
    grid[5][21] = 6;
    grid[0][20] = 4;
    grid[3][8] = 1;
    expect(bestHours({ grid, levels: [], best: [] })).toEqual([
      { weekday: 5, hour: 21, sessions: 6 },
      { weekday: 0, hour: 20, sessions: 4 },
      { weekday: 1, hour: 9, sessions: 2 },
    ]);
  });

  it("weekday 0 is Monday", () => {
    expect(weekdayName(0, "en", "long")).toBe("Monday");
    expect(weekdayName(6, "fr", "long")).toBe("dimanche");
  });
});

describe("score helpers", () => {
  it("normalises 0..1 and 0..100 ratios", () => {
    expect(ratio(0.87)).toBe(0.87);
    expect(ratio(87)).toBe(0.87);
    expect(ratio(-1)).toBe(0);
    expect(ratio(undefined)).toBe(0);
    expect(ratio(250)).toBe(1);
  });

  it("grades like the server", () => {
    expect(gradeFor(85)).toBe("excellent");
    expect(gradeFor(70)).toBe("good");
    expect(gradeFor(50)).toBe("fair");
    expect(gradeFor(49.9)).toBe("needs_attention");
  });

  it("values credits in money", () => {
    expect(creditsValue(2500, 2, "USD", "en")).toBe("$50.00");
  });
});

describe("validateWithdrawal", () => {
  const payout = { available_credits: 5000, min_withdrawal_credits: 2500 };
  const ok = { amount: "3000", method: "bank_transfer" as const, destination: "FR76 3000 6000 0112 3456 7890 189" };

  it("accepts a valid request and trims the destination", () => {
    expect(validateWithdrawal({ ...ok, destination: `  ${ok.destination}  ` }, payout)).toEqual({
      errors: {},
      value: { amount_credits: 3000, method: "bank_transfer", destination: ok.destination },
    });
    expect(validateWithdrawal({ amount: "2500", method: "paypal", destination: "me@example.com" }, payout).value).toBeDefined();
  });

  it.each([
    [{ amount: "" }, "amount", "Enter an amount."],
    [{ amount: "12.5" }, "amount", "Enter a whole number of credits."],
    [{ amount: "-3" }, "amount", "Enter a whole number of credits."],
    [{ amount: "100" }, "amount", "The minimum withdrawal is {min} credits."],
    [{ amount: "6000" }, "amount", "You can withdraw at most {max} credits."],
    [{ method: "" }, "method", "Choose a payout method."],
    [{ destination: "  " }, "destination", "Enter where we should send the money."],
    [{ destination: "x".repeat(201) }, "destination", "Keep this under 200 characters."],
    [{ method: "paypal", destination: "not-an-email" }, "destination", "Enter the email address of your PayPal account."],
    [{ destination: "12" }, "destination", "Enter a valid IBAN or account number."],
  ] as const)("rejects %o", (patch, field, message) => {
    const result = validateWithdrawal({ ...ok, ...patch }, payout);
    expect(result.value).toBeUndefined();
    expect(result.errors[field]?.message).toBe(message);
  });

  it("passes limits as interpolation values", () => {
    expect(validateWithdrawal({ ...ok, amount: "1" }, payout).errors.amount?.values).toEqual({ min: 2500 });
    expect(validateWithdrawal({ ...ok, amount: "9999" }, payout).errors.amount?.values).toEqual({ max: 5000 });
  });
});

describe("dictionary coverage of analytics copy", () => {
  const messages = [
    "Enter an amount.",
    "Enter a whole number of credits.",
    "The minimum withdrawal is {min} credits.",
    "You can withdraw at most {max} credits.",
    "Choose a payout method.",
    "Enter where we should send the money.",
    "Keep this under 200 characters.",
    "Enter the email address of your PayPal account.",
    "Enter a valid IBAN or account number.",
    ...Object.values(rangeCopy),
    ...Object.values(gradeCopy).flatMap((g) => [g.label, g.guidance]),
    ...componentCopy.map((c) => c.label),
    ...Object.values(levelCopy),
    ...Object.values(withdrawalStatusLabel),
    ...Object.values(methodLabel),
  ];
  it.each(messages.map((m) => [m]))("%s exists in every locale", (key) => {
    for (const dict of [en, fr, es] as Record<string, string>[]) expect(dict[key], key).toBeTruthy();
  });
});
