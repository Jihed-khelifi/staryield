"use client"

import * as React from "react"
import { RotateCcw, UserRound } from "lucide-react"

import { useI18n } from "@/i18n/i18n-provider"
import { advisorApi } from "@/lib/api/endpoints"
import type { AdvisorSettings } from "@/lib/api/types"
import { useSession } from "@/lib/auth/session-provider"
import { applyApiError } from "@/lib/forms/server-errors"
import { useSchemaForm } from "@/lib/forms/use-schema-form"
import {
  advisorProfileSchema,
  advisorProfileValues,
  advisorSettingsPatch,
  MODALITIES_MAX,
  MODALITY_MAX,
  type AdvisorProfileValues,
} from "@/lib/schemas/advisor"
import { useAsync } from "@/lib/use-async"
import { ErrorState } from "@/components/feedback/error-state"
import { Button } from "@/components/ui/button"
import { ChipInput } from "@/components/ui/chip-input"
import { FieldRow } from "@/components/ui/field"
import { Form, FormRootError } from "@/components/ui/form"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { ProfilePhoto } from "@/components/psychic/settings/profile-photo"
import { useSettingsSection, type SaveOptions } from "@/components/psychic/settings/save-registry"
import { SettingsCard } from "@/components/psychic/settings/settings-card"

const FIELD_ORDER = ["phone", "about", "modalities"] as const satisfies readonly (keyof AdvisorProfileValues)[]

/** Registry order: the advisor profile saves before the Customs card. */
export const PROFILE_SECTION_ORDER = 10

/**
 * The advisor's contact number, About text and Modalities (Figma 1219:1509,
 * the advisor-specific rows of "Account Information"). One form, saved by
 * the page's "Save Changes" with optimistic concurrency: a save from another
 * tab or device is reported instead of silently overwritten.
 */
function ProfileForm({ saved, onSaved, onReload }: { saved: AdvisorSettings; onSaved: (next: AdvisorSettings) => void; onReload: () => void }) {
  const { text } = useI18n()
  const form = useSchemaForm<ReturnType<typeof advisorProfileSchema>, AdvisorProfileValues>(advisorProfileSchema, {
    defaultValues: advisorProfileValues(saved),
    shouldFocusError: false,
  })
  const conflict = form.formState.errors.root?.server?.type === "version_conflict"

  const save = React.useCallback(
    ({ focus }: SaveOptions) =>
      new Promise<boolean>((resolve) => {
        void form.handleSubmit(
          async (values) => {
            const patch = advisorSettingsPatch(values, saved)
            if (!patch) {
              form.reset(advisorProfileValues(saved))
              resolve(true)
              return
            }
            try {
              const next = await advisorApi.updateSettings(patch)
              form.reset(advisorProfileValues(next))
              onSaved(next)
              resolve(true)
            } catch (err) {
              const e = applyApiError(form, err, { text, focus })
              if (e.code === "version_conflict") {
                form.setError("root.server", {
                  type: "version_conflict",
                  message: text("These settings were changed in another tab or on another device. Reload them to see the latest version."),
                })
              }
              resolve(false)
            }
          },
          (errors) => {
            if (focus) {
              const first = FIELD_ORDER.find((name) => errors[name])
              if (first) {
                try {
                  form.setFocus(first)
                } catch {
                  // A custom control without a focusable ref.
                }
              }
            }
            resolve(false)
          }
        )()
      }),
    [form, saved, onSaved, text]
  )
  useSettingsSection("advisor-profile", PROFILE_SECTION_ORDER, form.formState.isDirty, save)

  return (
    <Form {...form}>
      <form
        noValidate
        className="advisor-settings-fields"
        aria-label={text("Advisor profile")}
        data-testid="advisor-profile-form"
        onSubmit={(event) => {
          event.preventDefault()
          void save({ focus: true })
        }}
      >
        <FieldRow
          control={form.control}
          name="phone"
          label={text("Phone number")}
          description={text("Private: never shown on your public profile. Use international format, e.g. +14155552671.")}
          className="advisor-field"
          labelClassName="advisor-label"
        >
          {(field) => <Input {...field} type="tel" inputMode="tel" autoComplete="tel" placeholder="+14155552671" className="advisor-input" />}
        </FieldRow>
        <FieldRow
          control={form.control}
          name="about"
          label={text("About section")}
          description={text("Shown on your public profile. Separate paragraphs with a blank line (up to 6).")}
          showValid={false}
          className="advisor-field"
          labelClassName="advisor-label"
        >
          {(field) => <Textarea {...field} rows={6} className="advisor-input advisor-textarea" />}
        </FieldRow>
        <FieldRow
          control={form.control}
          name="modalities"
          label={text("Modalities")}
          description={text("Up to {value0}, shown on your public profile.", { value0: MODALITIES_MAX })}
          showValid={false}
          className="advisor-field"
          labelClassName="advisor-label"
        >
          {(field) => (
            <ChipInput
              {...field}
              value={field.value ?? []}
              max={MODALITIES_MAX}
              maxLength={MODALITY_MAX}
              placeholder={text("Add a modality")}
              aria-label={text("Modalities")}
              className="advisor-chips"
            />
          )}
        </FieldRow>
        <FormRootError>
          {conflict ? (
            <Button type="button" variant="outline" size="sm" className="w-fit border-danger/40" onClick={onReload}>
              <RotateCcw aria-hidden />
              {text("Reload settings")}
            </Button>
          ) : null}
        </FormRootError>
        {/* Enter in a field submits; the visible button is the page's "Save Changes". */}
        <button type="submit" hidden tabIndex={-1} aria-hidden />
      </form>
    </Form>
  )
}

/** "Advisor profile": contact number, About, Modalities and the profile picture. */
export function AdvisorProfileCard() {
  const { text } = useI18n()
  const { user } = useSession()
  const settings = useAsync((signal) => advisorApi.settings(signal), [])
  const name = user?.psychic?.name || user?.display_name || ""

  return (
    <SettingsCard title={text("Advisor profile")} icon={<UserRound />} data-testid="advisor-profile-card">
      {settings.error && !settings.data ? (
        <ErrorState error={settings.error} onRetry={settings.reload} compact title={text("Your profile settings could not be loaded.")} />
      ) : !settings.data ? (
        <div className="advisor-settings-fields" aria-busy="true" data-testid="advisor-profile-loading">
          <p role="status" className="sr-only">
            {text("Loading your profile settings…")}
          </p>
          {[40, 120, 40].map((height, i) => (
            <div key={i} className="grid gap-1.5">
              <Skeleton className="h-3.5 w-28" />
              <Skeleton className="w-full rounded-lg" style={{ height }} />
            </div>
          ))}
        </div>
      ) : (
        <ProfileForm key={settings.data.version} saved={settings.data} onSaved={(next) => settings.setData(next)} onReload={settings.reload} />
      )}
      <div className="advisor-field">
        <p className="advisor-label">{text("Profile picture")}</p>
        <ProfilePhoto name={name} />
      </div>
    </SettingsCard>
  )
}
