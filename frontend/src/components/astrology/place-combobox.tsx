"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { astroApi, placeLabel, type GeoPlace } from "@/lib/api/astrology";
import type { CalculatorPlace } from "@/lib/schemas/calculator";
import { Combobox, type ComboboxPage } from "@/components/ui/combobox";

type PlaceOption = CalculatorPlace & { name: string; region: string };

function toOption(place: GeoPlace): PlaceOption {
  const region = [place.admin1 && place.admin1 !== place.name ? place.admin1 : "", place.country].filter(Boolean).join(", ");
  return { id: place.id, label: placeLabel(place), name: place.name, region };
}

type PlaceComboboxProps = {
  value: CalculatorPlace | null;
  onChange: (value: CalculatorPlace | null) => void;
  onBlur?: () => void;
  name?: string;
  id?: string;
  placeholder?: string;
  inputClassName?: string;
  ref?: React.Ref<HTMLInputElement>;
  "aria-invalid"?: boolean | "true" | "false";
  "aria-describedby"?: string;
  "aria-labelledby"?: string;
  "aria-label"?: string;
};

/**
 * Birthplace search on the GeoNames gazetteer (`GET /astrology/places`):
 * debounced, cancellable, keyset-paginated ("Show more results") and fully
 * keyboard operable. Only a picked place counts as a value, and the
 * gazetteer's CC BY attribution is shown with the suggestions.
 */
export function PlaceCombobox({ value, onChange, onBlur, placeholder, inputClassName, ref, ...rest }: PlaceComboboxProps) {
  const { text } = useI18n();
  const [attribution, setAttribution] = React.useState("");

  const loadOptions = React.useCallback(async (query: string, signal: AbortSignal, cursor?: string): Promise<ComboboxPage<PlaceOption>> => {
    const page = await astroApi.places({ q: query.slice(0, 80), limit: 8, cursor }, signal);
    if (page.attribution) setAttribution(page.attribution);
    return { items: (page.items ?? []).filter((place) => place?.id && place.name).map(toOption), nextCursor: page.next_cursor ?? null };
  }, []);

  return (
    <Combobox<CalculatorPlace>
      {...rest}
      ref={ref}
      value={value}
      onChange={(next) => onChange(next ? { id: next.id, label: next.label } : null)}
      onBlur={onBlur}
      loadOptions={loadOptions}
      getOptionKey={(option) => option.id}
      getOptionLabel={(option) => option.label}
      renderOption={(option) => {
        const place = option as PlaceOption;
        return place.name ? (
          <>
            <span className="font-semibold">{place.name}</span>
            {place.region ? <span className="font-light">{`, ${place.region}`}</span> : null}
          </>
        ) : (
          option.label
        );
      }}
      placeholder={placeholder ?? text("Search a city")}
      emptyMessage={text("No matching place. Try the nearest larger city.")}
      footer={attribution ? <span>{attribution}</span> : null}
      inputClassName={inputClassName}
    />
  );
}
