"use client";

import * as React from "react";
import { useFormContext, useWatch } from "react-hook-form";
import { MailCheck } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { astrologyApi } from "@/lib/api/endpoints";
import type { Settings } from "@/lib/api/account";
import type { Place } from "@/lib/api/types";
import type { PlaceRef } from "@/lib/schemas/birth";
import { isoDay } from "@/lib/schemas/settings";
import { Checkbox } from "@/components/ui/checkbox";
import { Combobox } from "@/components/ui/combobox";
import { FormControl, FormField, FormItem } from "@/components/ui/form";

import { EmailChangeDialog } from "./email-change-dialog";
import type { SettingsFormValues } from "./settings-model";
import { SettingsCard, SettingsField } from "./settings-ui";

/** "City, Region, Country" for a gazetteer result. */
function placeRef(place: Place & { admin1?: string }): PlaceRef {
  return { id: place.id, label: [place.name, place.admin1, place.country].filter((part) => part && part.trim()).join(", ") };
}

async function searchPlaces(query: string, signal: AbortSignal): Promise<PlaceRef[]> {
  const places: Array<(Place & { admin1?: string }) | null | undefined> = (await astrologyApi.places(query, signal)) ?? [];
  return places.filter((place): place is Place & { admin1?: string } => Boolean(place?.id && place.name)).map(placeRef);
}

/**
 * Account Information (Figma 384:1498 "Card / Account Info"): name, e-mail
 * (changed through a confirmation link), birthplace ("Location") and the
 * date and time of birth. Birth time and its "unknown" box are a deliberate
 * addition: houses and the rising sign are wrong without them.
 */
export function AccountInformationCard({ settings, onEmailChangeRequested }: { settings: Settings; onEmailChangeRequested: () => void }) {
  const { text } = useI18n();
  const form = useFormContext<SettingsFormValues>();
  const timeUnknown = useWatch({ control: form.control, name: "profile.time_unknown" });
  const [emailOpen, setEmailOpen] = React.useState(false);
  const unknownId = React.useId();
  const pendingId = React.useId();
  const pending = settings.account.pending_email;
  const admin = settings.account.role === "admin";

  return (
    <SettingsCard id="account" icon="gear" title={text("Account Information")} testId="settings-account">
      <div className="settings-stack">
        <SettingsField<SettingsFormValues, "account.display_name"> name="account.display_name" label={text("Full Name")}>
          {(field) => (
            <input
              ref={field.ref}
              name={field.name}
              value={field.value}
              onChange={field.onChange}
              onBlur={field.onBlur}
              className="settings-input"
              autoComplete="name"
              maxLength={80}
            />
          )}
        </SettingsField>

        <div className="settings-field" data-slot="field">
          <label className="settings-label" htmlFor="settings-email">
            {text("Email Address")}
          </label>
          <div className="settings-input-wrap">
            <input
              id="settings-email"
              className="settings-input settings-input-readonly"
              value={settings.account.email}
              readOnly
              aria-describedby={pending ? pendingId : undefined}
              autoComplete="email"
            />
            {admin ? null : (
              <button type="button" className="settings-inline-action" onClick={() => setEmailOpen(true)} data-testid="change-email">
                {text("Change")}
                <span className="sr-only"> {text("Email Address")}</span>
              </button>
            )}
          </div>
          {pending ? (
            <p id={pendingId} className="settings-pending" role="status" data-testid="pending-email">
              <MailCheck aria-hidden className="size-4 shrink-0" />
              <span>{text("Waiting for you to confirm {value0}. Check that inbox for the link.", { value0: pending })}</span>
            </p>
          ) : null}
        </div>

        <SettingsField<SettingsFormValues, "profile.place"> name="profile.place" label={text("Location")}>
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
              placeholder={text("Search your birth city")}
              emptyMessage={text("No matching place. Try the nearest larger city.")}
              inputClassName="settings-input settings-combobox-input"
            />
          )}
        </SettingsField>

        <div className="settings-birth">
          <SettingsField<SettingsFormValues, "profile.birth_date"> name="profile.birth_date" label={text("Date of Birth")} className="settings-birth-date">
            {(field) => (
              <input
                ref={field.ref}
                name={field.name}
                value={field.value}
                onChange={(event) => {
                  field.onChange(event.target.value);
                  if (form.formState.isSubmitted || form.getFieldState("profile.birth_time").isTouched) void form.trigger(["profile.place"]);
                }}
                onBlur={field.onBlur}
                type="date"
                min="1900-01-01"
                max={isoDay()}
                className="settings-input settings-input-date"
                autoComplete="bday"
              />
            )}
          </SettingsField>
          <SettingsField<SettingsFormValues, "profile.birth_time"> name="profile.birth_time" label={text("Time of birth")} className="settings-birth-time">
            {(field) => (
              <input
                ref={field.ref}
                name={field.name}
                value={timeUnknown ? "" : field.value}
                onChange={(event) => {
                  field.onChange(event.target.value);
                  void form.trigger(["profile.place", "profile.birth_date"]);
                }}
                onBlur={field.onBlur}
                type="time"
                disabled={timeUnknown}
                className="settings-input settings-input-time"
              />
            )}
          </SettingsField>
          <FormField
            control={form.control}
            name="profile.time_unknown"
            render={({ field }) => (
              <FormItem className="settings-check">
                <div className="flex items-center gap-2">
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
                        if (unknown && form.getValues("profile.birth_time"))
                          form.setValue("profile.birth_time", "", { shouldDirty: true, shouldValidate: true });
                        void form.trigger(["profile.place", "profile.birth_date"]);
                      }}
                    />
                  </FormControl>
                  <span id={unknownId}>{text("I don't know my time")}</span>
                </div>
                {timeUnknown ? <p className="settings-description">{text("No problem: we use noon, so your rising sign and houses stay hidden.")}</p> : null}
              </FormItem>
            )}
          />
        </div>
      </div>
      <EmailChangeDialog
        open={emailOpen}
        onOpenChange={setEmailOpen}
        currentEmail={settings.account.email}
        onRequested={onEmailChangeRequested}
      />
    </SettingsCard>
  );
}
