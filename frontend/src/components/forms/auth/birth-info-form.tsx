"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { useWatch } from "react-hook-form";
import { Lock } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { astrologyApi, meApi } from "@/lib/api/endpoints";
import type { Place, User } from "@/lib/api/types";
import { useSession } from "@/lib/auth/session-provider";
import { applyApiError } from "@/lib/forms/server-errors";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { birthInfoPayload, birthInfoSchema, type BirthInfoValues, type PlaceRef } from "@/lib/schemas/birth";
import { Checkbox } from "@/components/ui/checkbox";
import { Combobox } from "@/components/ui/combobox";
import { DatePartsInput, isoToDateParts } from "@/components/ui/date-parts-input";
import { FieldRow } from "@/components/ui/field";
import { Form, FormControl, FormField, FormItem, FormRootError } from "@/components/ui/form";

import { AuthCard, AuthShell } from "./auth-card";

/** A geocoder result as "City, Region, Country" (region when the API sends it). */
export function placeLabel(place: Place & { admin1?: string }): string {
  return [place.name, place.admin1, place.country].filter((part) => part && part.trim()).join(", ");
}

export function placeToRef(place: Place & { admin1?: string }): PlaceRef {
  return { id: place.id, label: placeLabel(place) };
}

function valuesFrom(user: User | null | undefined): BirthInfoValues {
  const p = user?.profile;
  return {
    timeUnknown: false,
    birth: isoToDateParts(p?.birth_date || null, p?.birth_time || ""),
    place: p?.birth_place_id ? { id: p.birth_place_id, label: p.birth_place || p.birth_place_id } : null,
  };
}

async function searchPlaces(query: string, signal: AbortSignal): Promise<PlaceRef[]> {
  // Tolerates both the current and the upgraded geocoder shapes
  // ({ id, name, admin1?, country, time_zone } items, optional next_cursor).
  const places: Array<(Place & { admin1?: string }) | null | undefined> = (await astrologyApi.places(query, signal)) ?? [];
  return places.filter((place): place is Place & { admin1?: string } => Boolean(place?.id && place.name)).map(placeToRef);
}

/** Step 3 of sign-up: date and time of birth + birthplace (Figma 867:16198). */
export function BirthInfoForm() {
  const { text, locale } = useI18n();
  const router = useRouter();
  const { status, user, setUser } = useSession();
  const form = useSchemaForm(birthInfoSchema, { defaultValues: valuesFrom(user) });
  const timeUnknown = useWatch({ control: form.control, name: "timeUnknown" });
  const unknownId = React.useId();
  const prefilled = React.useRef(Boolean(user));

  React.useEffect(() => {
    track("signup_step_viewed", { step: "birth" });
  }, []);

  // The step needs an account; the session may still be loading on mount.
  React.useEffect(() => {
    if (status === "anonymous") router.replace(`/${locale}/login?next=${encodeURIComponent(`/${locale}/signup/birth`)}`);
  }, [status, locale, router]);

  React.useEffect(() => {
    if (!user || prefilled.current || form.formState.isDirty) return;
    prefilled.current = true;
    form.reset(valuesFrom(user));
  }, [user, form]);

  async function submit(values: BirthInfoValues) {
    try {
      const me = await meApi.update(birthInfoPayload(values));
      setUser(me);
      track("signup_step_completed", { step: "birth" });
      router.push(`/${locale}/signup/offer`);
    } catch (err) {
      const e = applyApiError(form, err, {
        text,
        fieldMap: { birth_date: "birth", birth_time: "birth", birth_place_id: "place" },
      });
      track("error_displayed", { code: String(e.code), step: "birth" });
    }
  }

  return (
    <AuthShell>
      <Form {...form}>
        <AuthCard
          title="Birth information"
          subtitle={text("Please enter your date of birth so we can generate your horoscope and detailed astrological compass.")}
          onSubmit={form.handleSubmit(submit)}
          pending={form.formState.isSubmitting}
          className="auth-card-birth"
          testId="birth-info-form"
        >
          <FormRootError className="mt-4" />
          <FieldRow control={form.control} name="birth" label={text("Date and time of birth")} className="mt-6" showValid={false}>
            {(field) => (
              <DatePartsInput
                value={field.value}
                onChange={field.onChange}
                onBlur={field.onBlur}
                ref={field.ref}
                name={field.name}
                withTime={!timeUnknown}
                aria-label={text("Date and time of birth")}
                className="auth-date-parts"
              />
            )}
          </FieldRow>
          <FormField
            control={form.control}
            name="timeUnknown"
            render={({ field }) => (
              <FormItem className="mt-3">
                <div className="flex items-center gap-2 text-sm">
                  <FormControl>
                    <Checkbox
                      ref={field.ref}
                      name={field.name}
                      checked={field.value}
                      aria-labelledby={unknownId}
                      onBlur={field.onBlur}
                      onCheckedChange={(checked) => {
                        const unknown = checked === true;
                        field.onChange(unknown);
                        if (unknown) form.setValue("birth", { ...form.getValues("birth"), time: "" });
                        if (form.formState.isSubmitted || form.getFieldState("birth").isTouched) void form.trigger(["birth", "place"]);
                      }}
                    />
                  </FormControl>
                  <span id={unknownId}>{text("I don't know my time")}</span>
                </div>
                {timeUnknown ? (
                  <p className="text-xs font-light">{text("No problem: we use noon, so your rising sign and houses stay hidden.")}</p>
                ) : null}
              </FormItem>
            )}
          />
          <FieldRow
            control={form.control}
            name="place"
            label={text("Location")}
            description={text("Please enter where you were born so we can generate your horoscope and detailed astrological compass.")}
            className="mt-6"
          >
            {(field) => (
              <Combobox<PlaceRef>
                value={field.value ?? null}
                onChange={(value) => field.onChange(value)}
                onBlur={field.onBlur}
                ref={field.ref}
                name={field.name}
                loadOptions={searchPlaces}
                getOptionKey={(option) => option.id}
                getOptionLabel={(option) => option.label}
                placeholder={text("US, New York")}
                emptyMessage={text("No matching place. Try the nearest larger city.")}
                inputClassName="auth-input"
              />
            )}
          </FieldRow>
          <p className="mt-6 flex items-center justify-center gap-1.5 text-xs font-light">
            <Lock aria-hidden className="size-3.5" />
            {text("Your birth time and exact location stay private. Advisors you chat with see your date of birth, which readings rely on.")}
          </p>
        </AuthCard>
      </Form>
    </AuthShell>
  );
}
