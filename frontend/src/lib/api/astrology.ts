/**
 * Astrology engine API (backend/internal/api/handler/astrology.go and
 * service/astrology.go): the stateless calculators, the GeoNames gazetteer,
 * the signed-in client's chart, horoscope and numbers, and the public sky
 * events. Birth data only ever travels in POST bodies (answered no-store) or
 * as the signed-in user's stored profile; it is never put in a URL.
 */
import { api } from "@/lib/api/client";

export type HouseSystem = "placidus" | "whole_sign" | "equal";
export const HOUSE_SYSTEMS: readonly HouseSystem[] = ["placidus", "whole_sign", "equal"];

export type ZodiacSign =
  | "Aries"
  | "Taurus"
  | "Gemini"
  | "Cancer"
  | "Leo"
  | "Virgo"
  | "Libra"
  | "Scorpio"
  | "Sagittarius"
  | "Capricorn"
  | "Aquarius"
  | "Pisces";

/** A body or chart point placed in the zodiac. */
export type AstroPosition = {
  body: string;
  longitude: number;
  latitude: number;
  /** Degrees per day; negative when retrograde. */
  speed: number;
  sign: string;
  degree: number;
  minute: number;
  second: number;
  /** "Cancer, 8°22′47″". */
  formatted: string;
  house?: number;
  /** English label, e.g. "First house". */
  house_label?: string;
  retrograde: boolean;
};

export type ChartAngles = { asc: number; mc: number; dsc: number; ic: number };
export type HouseCusp = { number: number; cusp: number; sign: string };
export type AspectType = "conjunction" | "sextile" | "square" | "trine" | "opposition";

export type ChartAspect = {
  a: string;
  b: string;
  type: AspectType | (string & {});
  exact_angle: number;
  angle: number;
  orb: number;
  applying: boolean;
};

/** A gazetteer entry (GeoNames, CC BY 4.0). */
export type GeoPlace = {
  id: string;
  name: string;
  admin1?: string;
  country: string;
  country_code: string;
  latitude: number;
  longitude: number;
  time_zone: string;
  population?: number;
};

export type EngineInfo = { version: string; tzdata: string; gazetteer: string; horoscope?: string };

/** A computed natal chart with the birth data it came from (ChartView). */
export type NatalChart = {
  instant: string;
  time_known: boolean;
  has_location: boolean;
  house_system?: HouseSystem | (string & {});
  /** "porphyry" when Placidus is undefined at polar latitudes. */
  house_system_used?: string;
  fallback_reason?: string;
  sun_sign: string;
  sun_sign_uncertain: boolean;
  sun_sign_alternatives?: string[];
  moon_sign: string;
  moon_sign_uncertain: boolean;
  moon_sign_alternatives?: string[];
  rising_sign?: string;
  angles?: ChartAngles;
  positions: AstroPosition[];
  house_cusps?: number[];
  houses?: HouseCusp[];
  aspects: ChartAspect[];
  elements: Record<string, number>;
  modalities: Record<string, number>;
  birth_date: string;
  birth_time?: string;
  /** "ok" | "skipped" | "ambiguous" | "lmt". */
  wall_time_status?: string;
  place?: GeoPlace;
  life_path: number;
  /** "birth_place_missing" | "pluto_out_of_range". */
  warnings?: string[];
  engine: EngineInfo;
  disclaimer: string;
  attribution?: string;
};

export type TarotCardSource = {
  kind: "planet" | "cusp_sign" | (string & {});
  body?: string;
  sign: string;
  score?: number;
};

/** One card of the Twelve Houses spread (Golden Dawn attributions). */
export type TarotCard = {
  house: number;
  /** Artwork slug, e.g. "the-sun". */
  card: string;
  reversed: boolean;
  source: TarotCardSource;
  uncertain: boolean;
};

export type TarotSpread = { house_system: string; uncertain: boolean; cards: TarotCard[] };

export type ReductionStep = { label: string; value: number; reduced: number; steps?: string[] };
export type Reduction = { number: number; master: boolean; components: ReductionStep[]; final?: string[]; total: number };

export type NumerologyView = {
  life_path: number;
  life_path_master: boolean;
  life_path_reduction: Reduction;
  personal_year: number;
  personal_month: number;
  personal_day: number;
  as_of: string;
  tz: string;
};

export type NatalInclude = "aspects" | "tarot" | "numerology";

export type NatalInput = {
  birth_date: string;
  birth_time?: string;
  place_id?: string;
  house_system?: HouseSystem;
  include?: NatalInclude[];
  /** The visitor's IANA zone ("today" for numerology). */
  tz?: string;
};

export type NatalResult = {
  chart: NatalChart;
  tarot?: TarotSpread;
  numerology?: NumerologyView;
  engine: EngineInfo;
  disclaimer: string;
  attribution?: string;
};

export type SunSignInput = { birth_date: string; birth_time?: string; place_id?: string };

export type SunSignResult = {
  sun_sign: string;
  /** True on a cusp day whose sign depends on an unknown time or place. */
  uncertain: boolean;
  alternatives?: string[];
  element: string;
  modality: string;
};

export type PlacesPage = { items: GeoPlace[]; next_cursor?: string; attribution: string };

export type LunarEvent = { kind: "new_moon" | "first_quarter" | "full_moon" | "last_quarter" | (string & {}); at: string; sign: string };
export type RetrogradePeriod = { body: string; start: string; end: string };
export type SkyEvents = { lunar: LunarEvent[]; retrogrades: RetrogradePeriod[] };

/** A daily reading (horoscope.Reading); display fields are localised by the API. */
export type HoroscopeReading = {
  sign: string;
  sign_name: string;
  date: string;
  current_date: string;
  locale: string;
  tz: string;
  text: string;
  focus: string;
  affirmation: string;
  mood: string;
  mood_key: string;
  lucky_number: number;
  lucky_hour: number;
  lucky_time: string;
  color: string;
  color_key: string;
  compatibility: number;
  compatibility_score: number;
  compatibility_label: string;
  compatibility_label_key: string;
  compatible_sign: string;
  compatible_sign_name: string;
  moon_sign: string;
  moon_sign_name: string;
  moon_phase: string;
  moon_phase_name: string;
  date_range: { start: string; end: string; label: string };
  provider: string;
  engine_version: string;
};

/** GET /me/horoscope: the reading for the client's own sun sign plus sky events. */
export type MyHoroscope = HoroscopeReading & {
  sun_sign_uncertain: boolean;
  sun_sign_alternatives?: string[];
  events: SkyEvents;
};

/** The browser's IANA time zone, or undefined when the platform hides it. */
export function browserTimeZone(): string | undefined {
  try {
    const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    return zone && zone.length <= 64 ? zone : undefined;
  } catch {
    return undefined;
  }
}

/**
 * Call `request` with the visitor's zone, retrying once without it when the
 * API does not know that zone (`tz` is the only rejected field): "today"
 * then falls back to UTC instead of blocking the visitor on something they
 * cannot fix.
 */
async function withZone<T>(request: (tz: string | undefined) => Promise<T>): Promise<T> {
  const tz = browserTimeZone();
  try {
    return await request(tz);
  } catch (error) {
    const fields = (error as { code?: string; fields?: Record<string, string> }).fields ?? {};
    const onlyZone = (error as { code?: string }).code === "validation_failed" && Object.keys(fields).length === 1 && "tz" in fields;
    if (tz && onlyZone) return request(undefined);
    throw error;
  }
}

export const astroApi = {
  /** Anonymous natal chart, Twelve Houses spread and numerology (never stored). */
  natal: (input: NatalInput, signal?: AbortSignal) =>
    withZone((tz) => api<NatalResult>("/api/v1/astrology/natal", { method: "POST", body: { ...input, tz }, signal, idempotent: true, csrf: false })),
  /** The canonical sun sign; cusp births report `uncertain` with both signs. */
  sunSign: (input: SunSignInput, signal?: AbortSignal) =>
    api<SunSignResult>("/api/v1/astrology/sun-sign", { method: "POST", body: input, signal, idempotent: true, csrf: false }),
  numerology: (birth_date: string, signal?: AbortSignal) =>
    withZone((tz) => api<NumerologyView>("/api/v1/astrology/numerology", { method: "POST", body: { birth_date, tz }, signal, idempotent: true, csrf: false })),
  /** Gazetteer search (≥ 2 characters), keyset-paginated. */
  places: (params: { q: string; limit?: number; cursor?: string }, signal?: AbortSignal) =>
    api<PlacesPage>("/api/v1/astrology/places", { query: params, signal }),
  place: (id: string, signal?: AbortSignal) => api<GeoPlace>(`/api/v1/astrology/places/${encodeURIComponent(id)}`, { signal }),
  signHoroscope: (sign: string, params: { day?: "yesterday" | "today" | "tomorrow"; date?: string } = {}, signal?: AbortSignal) =>
    api<HoroscopeReading>(`/api/v1/astrology/horoscope/${encodeURIComponent(sign.toLowerCase())}`, {
      query: { ...params, tz: browserTimeZone() },
      signal,
    }),
  events: (count?: number, signal?: AbortSignal) => api<SkyEvents>("/api/v1/astrology/events", { query: { count }, signal }),
  /** The signed-in client's chart; without `house_system` the stored preference is used. */
  myChart: (houseSystem?: HouseSystem, signal?: AbortSignal) =>
    api<NatalChart>("/api/v1/me/chart", { query: { house_system: houseSystem }, signal }),
  myHoroscope: (params: { day?: "yesterday" | "today" | "tomorrow"; date?: string } = {}, signal?: AbortSignal) =>
    withZone((tz) => api<MyHoroscope>("/api/v1/me/horoscope", { query: { ...params, tz }, signal })),
  myNumerology: (signal?: AbortSignal) =>
    withZone((tz) => api<NumerologyView>("/api/v1/me/numerology", { query: { tz }, signal })),
};

/** "City, Region, Country" without repeating equal parts (like the API's Label). */
export function placeLabel(place: Pick<GeoPlace, "name" | "country"> & { admin1?: string }): string {
  const parts = [place.name];
  if (place.admin1 && place.admin1 !== place.name) parts.push(place.admin1);
  if (place.country) parts.push(place.country);
  return parts.filter((part) => part && part.trim()).join(", ");
}
