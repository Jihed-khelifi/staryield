"use client";

import * as React from "react";

import { localeLabels, locales } from "@/i18n/config";
import { useI18n } from "@/i18n/i18n-provider";
import { THEMES } from "@/lib/theme";
import { Combobox } from "@/components/ui/combobox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

import type { SettingsFormValues } from "./settings-model";
import { SettingsCard, SettingsField } from "./settings-ui";
import { searchTimeZones, zoneLabel, type ZoneOption } from "./time-zones";

const THEME_LABELS: Record<(typeof THEMES)[number], string> = {
  "Warm Beige": "Warm Beige (Default)",
  "Light Cream": "Light Cream",
};


/**
 * Appearance: colour theme (applied app-wide once saved), the language used
 * for the site and our e-mails, and the time zone (defaults to the
 * browser's).
 */
export function AppearanceCard() {
  const { text, locale } = useI18n();

  return (
    <SettingsCard id="appearance" icon="palette" title={text("Appearance")} testId="settings-appearance">
      <div className="settings-stack">
        <SettingsField<SettingsFormValues, "appearance.theme"> name="appearance.theme" label={text("Theme Preference")} showValid={false}>
          {(field) => (
            <Select value={field.value} onValueChange={field.onChange} name={field.name}>
              <SelectTrigger ref={field.ref} onBlur={field.onBlur} className="settings-input settings-select" data-testid="theme-select" aria-label={text("Theme Preference")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {THEMES.map((theme) => (
                  <SelectItem key={theme} value={theme}>
                    {text(THEME_LABELS[theme])}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </SettingsField>
        <SettingsField<SettingsFormValues, "appearance.locale"> name="appearance.locale" label={text("Language")} showValid={false}>
          {(field) => (
            <Select value={field.value} onValueChange={field.onChange} name={field.name}>
              <SelectTrigger ref={field.ref} onBlur={field.onBlur} className="settings-input settings-select" data-testid="language-select" aria-label={text("Language")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {locales.map((code) => (
                  <SelectItem key={code} value={code} lang={code}>
                    {code === "en" ? text("English (US)") : localeLabels[code].name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </SettingsField>
        <SettingsField<SettingsFormValues, "appearance.time_zone"> name="appearance.time_zone" label={text("Time Zone")} showValid={false}>
          {(field) => (
            <Combobox<ZoneOption>
              value={field.value ? { id: field.value, label: zoneLabel(field.value, locale) } : null}
              onChange={(option) => field.onChange(option?.id ?? "")}
              onBlur={field.onBlur}
              ref={field.ref}
              name={field.name}
              loadOptions={async (query) => searchTimeZones(query, locale)}
              getOptionKey={(option) => option.id}
              getOptionLabel={(option) => option.label}
              placeholder={text("Search a city or time zone")}
              emptyMessage={text("No matching time zone.")}
              debounceMs={120}
              inputClassName="settings-input settings-combobox-input"
            />
          )}
        </SettingsField>
      </div>
    </SettingsCard>
  );
}
