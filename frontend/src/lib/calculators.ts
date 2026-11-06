/**
 * Path of Life numerology and birth-date helpers.
 *
 * The Path of Life calculator stays in the browser: it is pure arithmetic,
 * so its "calculated locally" promise is true. The rules mirror the Go
 * engine (backend/internal/astro/numerology.go) and both are checked
 * against the same fixtures (backend/internal/astro/testdata/
 * numerology_vectors.json) by `npm run test:calculators` and vitest, so the
 * two implementations cannot drift.
 *
 * Astrology (sun signs, charts, the natal spread) is computed by the API;
 * nothing here guesses a zodiac sign from a date table.
 *
 * Keep this file free of imports: scripts/check-calculators.mjs evaluates
 * it in a VM sandbox.
 */

/** 11, 22 and 33 are kept by the life path reduction. */
export function isMasterNumber(value: number): boolean {
  return value === 11 || value === 22 || value === 33;
}

/**
 * Repeated digit sums of `value` until one digit remains (master numbers are
 * kept when `masters`), with every step written out, e.g. 1990 →
 * ["1+9+9+0=19", "1+9=10", "1+0=1"].
 */
export function reduceWithSteps(value: number, masters = true): { value: number; steps: string[] } {
  let n = Math.abs(Math.trunc(value));
  const steps: string[] = [];
  while (n > 9 && !(masters && isMasterNumber(n))) {
    const digits = String(n).split("");
    const sum = digits.reduce((total, digit) => total + Number(digit), 0);
    steps.push(`${digits.join("+")}=${sum}`);
    n = sum;
  }
  return { value: n, steps };
}

export function reduceNumber(value: number, masters = true): number {
  return reduceWithSteps(value, masters).value;
}

export type ReductionStep = {
  /** "month", "day", "year" (life path) or "birth_month", "birth_day", "year" (personal year). */
  label: string;
  value: number;
  reduced: number;
  steps: string[];
};

/** Every step of a numerology calculation (the API's `life_path_reduction` shape). */
export type Reduction = {
  number: number;
  master: boolean;
  components: ReductionStep[];
  total: number;
  /** Digit sums applied to the total ([] when the total is already final). */
  final: string[];
};

/** [year, month, day] of a "YYYY-MM-DD" date, or null. */
export function parseIsoDate(date: string): [number, number, number] | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date);
  if (!match) return null;
  return [Number(match[1]), Number(match[2]), Number(match[3])];
}

function component(label: string, value: number, masters: boolean): ReductionStep {
  const reduced = reduceWithSteps(value, masters);
  return { label, value, reduced: reduced.value, steps: reduced.steps };
}

/**
 * Life path by component reduction: the month, the day and the year are each
 * reduced keeping 11/22/33, summed, and the sum is reduced keeping 11/22/33.
 * 29 November 1990: 11 + 11 + 1 = 23 → 5.
 */
export function lifePathReduction(date: string): Reduction {
  const parts = parseIsoDate(date);
  if (!parts) throw new RangeError(`Invalid birth date: ${date}`);
  const [year, month, day] = parts;
  const components = [component("month", month, true), component("day", day, true), component("year", year, true)];
  const total = components.reduce((sum, c) => sum + c.reduced, 0);
  const final = reduceWithSteps(total, true);
  return { number: final.value, master: isMasterNumber(final.value), components, total, final: final.steps };
}

export function lifePath(date: string): number {
  return lifePathReduction(date).number;
}

export type PersonalCycles = {
  personalYear: number;
  personalMonth: number;
  personalDay: number;
  /** The calendar date ("YYYY-MM-DD") the cycles are for. */
  asOf: string;
};

function pad(value: number, length = 2): string {
  return String(value).padStart(length, "0");
}

/** "YYYY-MM-DD" of a local calendar date. */
export function isoDate(year: number, month: number, day: number): string {
  return `${pad(year, 4)}-${pad(month)}-${pad(day)}`;
}

/** The local calendar date of `today` in the browser's time zone. */
export function localIsoDate(today: Date = new Date()): string {
  return isoDate(today.getFullYear(), today.getMonth() + 1, today.getDate());
}

/**
 * The calendar date ("YYYY-MM-DD") of `instant` in an IANA time zone (the
 * browser's own zone when omitted). Personal cycles change at local
 * midnight, exactly like the API's `as_of`.
 */
export function localDateIn(instant: Date, timeZone?: string): string {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(instant);
  const get = (type: string) => Number(parts.find((part) => part.type === type)?.value ?? NaN);
  return isoDate(get("year"), get("month"), get("day"));
}

/**
 * Personal year R(R(month) + R(day) + R(year)), personal month R(year + month)
 * and personal day R(month + day) for the birth month and day, on the
 * calendar date `asOf` ("YYYY-MM-DD", or a Date read in the local zone).
 * Personal cycles reduce to 1–9 (no master numbers).
 */
export function personalCycles(birth: string, asOf: string | Date = new Date()): PersonalCycles {
  const born = parseIsoDate(birth);
  const date = typeof asOf === "string" ? asOf : localIsoDate(asOf);
  const today = parseIsoDate(date);
  if (!born) throw new RangeError(`Invalid birth date: ${birth}`);
  if (!today) throw new RangeError(`Invalid date: ${date}`);
  const [, birthMonth, birthDay] = born;
  const [year, month, day] = today;
  const personalYear = reduceNumber(
    reduceNumber(birthMonth, false) + reduceNumber(birthDay, false) + reduceNumber(year, false),
    false,
  );
  const personalMonth = reduceNumber(personalYear + month, false);
  const personalDay = reduceNumber(personalMonth + day, false);
  return { personalYear, personalMonth, personalDay, asOf: date };
}

/** Upper-case Roman numeral for 1–3999 (the result cards' numerals). */
export function romanNumeral(value: number): string {
  const n = Math.trunc(value);
  if (!(n >= 1 && n <= 3999)) return String(value);
  const table: [number, string][] = [
    [1000, "M"], [900, "CM"], [500, "D"], [400, "CD"], [100, "C"], [90, "XC"],
    [50, "L"], [40, "XL"], [10, "X"], [9, "IX"], [5, "V"], [4, "IV"], [1, "I"],
  ];
  let rest = n;
  let out = "";
  for (const [amount, numeral] of table) {
    while (rest >= amount) {
      out += numeral;
      rest -= amount;
    }
  }
  return out;
}

/** "30 / 04 / 1945" for the result's date button. */
export function formatDateParts(date: string): string {
  const parts = parseIsoDate(date);
  if (!parts) return date;
  const [year, month, day] = parts;
  return `${pad(day)} / ${pad(month)} / ${pad(year, 4)}`;
}

/** The date 18 years before `today` ("YYYY-MM-DD"): the youngest adult birth date. */
export function minimumAdultDate(today = new Date()) {
  const cutoff = new Date(today);
  cutoff.setFullYear(cutoff.getFullYear() - 18);
  return isoDate(cutoff.getFullYear(), cutoff.getMonth() + 1, cutoff.getDate());
}

/** A real calendar date from 1900 up to `today` (local calendar), like the API's rule. */
export function validBirthDate(day: number, month: number, year: number, today = new Date()) {
  const date = new Date(Date.UTC(year, month - 1, day));
  return (
    year >= 1900 &&
    year <= today.getFullYear() &&
    date.getUTCFullYear() === year &&
    date.getUTCMonth() === month - 1 &&
    date.getUTCDate() === day &&
    date <= new Date(Date.UTC(today.getFullYear(), today.getMonth(), today.getDate()))
  );
}
