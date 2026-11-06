/**
 * One calculation per calculator. Natal and Astrology are computed by the
 * API (stateless POSTs answered no-store); Path of Life is pure arithmetic
 * in the browser. Results are kept in memory by fragment so going back and
 * forward never repeats a request.
 */
import { astroApi, type NatalResult, type SunSignResult } from "@/lib/api/astrology";
import { lifePathReduction, localIsoDate, personalCycles, type PersonalCycles, type Reduction } from "@/lib/calculators";
import {
  birthDateOf,
  type CalculatorFormValues,
  type CalculatorKind,
  type NatalFormValues,
  type SunSignFormValues,
} from "@/lib/schemas/calculator";

export type LifePathResult = { lifePath: Reduction; cycles: PersonalCycles };

export type CalculatorResult =
  | { kind: "natal-chart"; natal: NatalResult }
  | { kind: "astrology"; sun: SunSignResult }
  | { kind: "path-of-life"; numbers: LifePathResult };

/** The natal request for validated form values (time only with a place). */
export function natalPayload(values: NatalFormValues) {
  const time = values.timeUnknown ? "" : (values.birth.time ?? "");
  return {
    birth_date: birthDateOf(values.birth),
    ...(time ? { birth_time: time } : {}),
    ...(values.place?.id ? { place_id: values.place.id } : {}),
    house_system: values.houseSystem,
    include: ["tarot" as const],
  };
}

export function sunSignPayload(values: SunSignFormValues) {
  return {
    birth_date: birthDateOf(values.birth),
    ...(values.birth.time ? { birth_time: values.birth.time } : {}),
    ...(values.place?.id ? { place_id: values.place.id } : {}),
  };
}

export async function calculate(kind: CalculatorKind, values: CalculatorFormValues, signal?: AbortSignal): Promise<CalculatorResult> {
  if (kind === "natal-chart") return { kind, natal: await astroApi.natal(natalPayload(values as NatalFormValues), signal) };
  if (kind === "astrology") return { kind, sun: await astroApi.sunSign(sunSignPayload(values as SunSignFormValues), signal) };
  const date = birthDateOf(values.birth);
  return { kind, numbers: { lifePath: lifePathReduction(date), cycles: personalCycles(date, localIsoDate()) } };
}

const MAX_CACHED = 6;
const cache = new Map<string, CalculatorResult>();

export function cachedResult(key: string): CalculatorResult | undefined {
  return cache.get(key);
}

export function rememberResult(key: string, result: CalculatorResult) {
  cache.delete(key);
  cache.set(key, result);
  while (cache.size > MAX_CACHED) cache.delete(cache.keys().next().value as string);
}

/** Forget every cached result (tests). */
export function clearResults() {
  cache.clear();
}
