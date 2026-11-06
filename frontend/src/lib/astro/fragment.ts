/**
 * Calculator state in the URL fragment: `#v=1&d=1990-03-14&t=08:45&p=gn:2988507&h=placidus`.
 *
 * The fragment survives a refresh and can be shared, yet it is never sent to
 * the server (not in requests, logs, the Referer header or analytics, which
 * record `location.pathname` only). It is not trusted either: decoding
 * accepts only the keys a calculator knows, rejects duplicates, oversized
 * values and unknown versions, and then runs the very same yup schema as the
 * form. Birth data never goes into query strings or sessionStorage.
 */
import type { TranslateCopy } from "@/i18n/translate-copy";
import { HOUSE_SYSTEMS, type HouseSystem } from "@/lib/api/astrology";
import {
  PLACE_ID_RULE,
  birthDateOf,
  calculatorSchemas,
  type CalculatorFormValues,
  type CalculatorKind,
  type LifePathFormValues,
  type NatalFormValues,
  type SunSignFormValues,
} from "@/lib/schemas/calculator";

export const FRAGMENT_VERSION = "1";

/** Keys each calculator accepts, in their canonical order. */
const KEYS: Record<CalculatorKind, readonly string[]> = {
  "path-of-life": ["v", "d"],
  astrology: ["v", "d", "t", "p"],
  "natal-chart": ["v", "d", "t", "p", "h"],
};

const MAX_FRAGMENT = 256;
const FORMAT: Record<string, RegExp> = {
  v: /^\d{1,2}$/,
  d: /^\d{4}-\d{2}-\d{2}$/,
  t: /^\d{2}:\d{2}$/,
  p: PLACE_ID_RULE,
  h: /^[a-z_]{1,16}$/,
};

export type FragmentInvalidReason = "too_long" | "malformed" | "unknown_key" | "duplicate" | "version" | "format" | "schema";

export type DecodedFragment<V extends CalculatorFormValues = CalculatorFormValues> =
  | { status: "empty" }
  | { status: "invalid"; reason: FragmentInvalidReason }
  | { status: "ok"; values: V };

function isHouseSystem(value: string): value is HouseSystem {
  return (HOUSE_SYSTEMS as readonly string[]).includes(value);
}

/** The fragment (with its leading "#") for validated form values. */
export function encodeFragment(kind: CalculatorKind, values: CalculatorFormValues): string {
  const params = new URLSearchParams();
  params.set("v", FRAGMENT_VERSION);
  params.set("d", birthDateOf(values.birth));
  if (kind === "natal-chart") {
    const natal = values as NatalFormValues;
    if (!natal.timeUnknown && natal.birth.time) params.set("t", natal.birth.time);
    if (natal.place?.id) params.set("p", natal.place.id);
    if (natal.houseSystem && natal.houseSystem !== "placidus") params.set("h", natal.houseSystem);
  } else if (kind === "astrology") {
    const sun = values as SunSignFormValues;
    if (sun.birth.time) params.set("t", sun.birth.time);
    if (sun.place?.id) params.set("p", sun.place.id);
  }
  return `#${params.toString()}`;
}

/**
 * Decode and validate a fragment for `kind`. `text` builds the same schema
 * the form uses (its messages are irrelevant here).
 */
export function decodeFragment(kind: CalculatorKind, hash: string, text: TranslateCopy): DecodedFragment {
  const raw = hash.startsWith("#") ? hash.slice(1) : hash;
  if (!raw) return { status: "empty" };
  if (raw.length > MAX_FRAGMENT) return { status: "invalid", reason: "too_long" };

  let params: URLSearchParams;
  try {
    params = new URLSearchParams(raw);
  } catch {
    return { status: "invalid", reason: "malformed" };
  }
  const allowed = KEYS[kind];
  const seen = new Map<string, string>();
  for (const [key, value] of params) {
    if (!allowed.includes(key)) return { status: "invalid", reason: "unknown_key" };
    if (seen.has(key)) return { status: "invalid", reason: "duplicate" };
    if (!FORMAT[key].test(value)) return { status: "invalid", reason: "format" };
    seen.set(key, value);
  }
  if (seen.get("v") !== FRAGMENT_VERSION) return { status: "invalid", reason: "version" };
  const date = seen.get("d");
  if (!date) return { status: "invalid", reason: "format" };
  const [year, month, day] = date.split("-");
  const time = seen.get("t") ?? "";
  const placeId = seen.get("p");
  const place = placeId ? { id: placeId, label: "" } : null;

  let values: CalculatorFormValues;
  if (kind === "natal-chart") {
    const house = seen.get("h") ?? "placidus";
    if (!isHouseSystem(house)) return { status: "invalid", reason: "format" };
    values = { birth: { day, month, year, time }, timeUnknown: !time, place, houseSystem: house } satisfies NatalFormValues;
  } else if (kind === "astrology") {
    values = { birth: { day, month, year, time }, place } satisfies SunSignFormValues;
  } else {
    values = { birth: { day, month, year, time: "" } } satisfies LifePathFormValues;
  }

  try {
    calculatorSchemas[kind](text).validateSync(values, { abortEarly: true });
  } catch {
    return { status: "invalid", reason: "schema" };
  }
  return { status: "ok", values };
}

/** True when `hash` holds calculator state (used to clear stale fragments). */
export function hasCalculatorFragment(hash: string): boolean {
  return /(^#|^)v=/.test(hash);
}
