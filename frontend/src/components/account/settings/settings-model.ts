/**
 * Mapping between the settings document (`GET /me/settings`) and the
 * settings form, and the single PATCH built from the fields the user
 * actually changed (react-hook-form `dirtyFields`). Pure functions, unit
 * tested.
 */
import type { FieldPath } from "react-hook-form";

import { isLocale, type Locale } from "@/i18n/config";
import type { Settings, SettingsPatch } from "@/lib/api/account";
import type { PlaceRef } from "@/lib/schemas/birth";
import { THEMES, type ThemeLabel } from "@/lib/theme";

export type SettingsFormValues = {
  account: { display_name: string };
  profile: { birth_date: string; birth_time: string; time_unknown: boolean; place: PlaceRef | null };
  notifications: { email: boolean; push: boolean; marketing: boolean; horoscope: boolean; events: boolean; chart: boolean };
  privacy: { share_birth_details_with_advisors: boolean };
  appearance: { theme: ThemeLabel; locale: Locale; time_zone: string };
};

/** The browser's IANA time zone ("" when unknown). */
export function browserTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone ?? "";
  } catch {
    return "";
  }
}

/**
 * Form values for a settings document. An unset time zone shows the
 * browser's zone; an unknown birth time is ticked when a date has no time.
 */
export function toFormValues(s: Settings, deviceZone = browserTimeZone()): SettingsFormValues {
  const theme = (THEMES as readonly string[]).includes(s.appearance.theme) ? (s.appearance.theme as ThemeLabel) : "Warm Beige";
  const locale = isLocale(s.appearance.locale) ? s.appearance.locale : "en";
  return {
    account: { display_name: s.account.display_name ?? "" },
    profile: {
      birth_date: s.profile.birth_date ?? "",
      birth_time: s.profile.birth_time ?? "",
      time_unknown: Boolean(s.profile.birth_date) && !s.profile.birth_time,
      place: s.profile.birth_place_id ? { id: s.profile.birth_place_id, label: s.profile.birth_place || s.profile.birth_place_id } : null,
    },
    notifications: { ...s.notifications },
    privacy: { share_birth_details_with_advisors: Boolean(s.privacy.share_birth_details_with_advisors) },
    appearance: { theme, locale, time_zone: s.appearance.time_zone || deviceZone },
  };
}

type Dirty = boolean | Dirty[] | { [key: string]: Dirty } | undefined;

/** True when anything under a dirtyFields node is dirty. */
export function anyDirty(node: unknown): boolean {
  if (node === true) return true;
  if (!node || typeof node !== "object") return false;
  return Object.values(node as Record<string, unknown>).some(anyDirty);
}

function dirtyAt(dirty: Dirty, path: string): boolean {
  let node: unknown = dirty;
  for (const key of path.split(".")) {
    if (node === true) return true;
    if (!node || typeof node !== "object") return false;
    node = (node as Record<string, unknown>)[key];
  }
  return anyDirty(node);
}

/**
 * The PATCH for the changed fields only, or null when nothing changed.
 * `expected_version` guards against overwriting a save made elsewhere.
 */
export function buildSettingsPatch(values: SettingsFormValues, dirty: Dirty, expectedVersion: number): SettingsPatch | null {
  const patch: SettingsPatch = {};
  if (dirtyAt(dirty, "account.display_name")) patch.account = { display_name: values.account.display_name.trim() };

  const profile: NonNullable<SettingsPatch["profile"]> = {};
  if (dirtyAt(dirty, "profile.birth_date")) profile.birth_date = values.profile.birth_date;
  if (dirtyAt(dirty, "profile.birth_time") || dirtyAt(dirty, "profile.time_unknown"))
    profile.birth_time = values.profile.time_unknown ? "" : values.profile.birth_time;
  if (dirtyAt(dirty, "profile.place")) {
    profile.birth_place_id = values.profile.place?.id ?? "";
    if (values.profile.place?.label) profile.birth_place_label = values.profile.place.label.slice(0, 120);
  }
  if (Object.keys(profile).length) patch.profile = profile;

  const notifications: NonNullable<SettingsPatch["notifications"]> = {};
  for (const key of Object.keys(values.notifications) as (keyof SettingsFormValues["notifications"])[])
    if (dirtyAt(dirty, `notifications.${key}`)) notifications[key] = values.notifications[key];
  if (Object.keys(notifications).length) patch.notifications = notifications;

  if (dirtyAt(dirty, "privacy.share_birth_details_with_advisors"))
    patch.privacy = { share_birth_details_with_advisors: values.privacy.share_birth_details_with_advisors };

  const appearance: NonNullable<SettingsPatch["appearance"]> = {};
  if (dirtyAt(dirty, "appearance.theme")) appearance.theme = values.appearance.theme;
  if (dirtyAt(dirty, "appearance.locale")) appearance.locale = values.appearance.locale;
  if (dirtyAt(dirty, "appearance.time_zone")) appearance.time_zone = values.appearance.time_zone;
  if (Object.keys(appearance).length) patch.appearance = appearance;

  if (Object.keys(patch).length === 0) return null;
  return { expected_version: expectedVersion, ...patch };
}

/** API field paths (422 `details.fields`) → form paths. */
export const settingsFieldMap: Partial<Record<string, FieldPath<SettingsFormValues>>> = {
  "profile.birth_place_id": "profile.place",
  "profile.birth_place_label": "profile.place",
  display_name: "account.display_name",
};
