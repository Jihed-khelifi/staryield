"use client"

import * as React from "react"
import { CreditCard, EyeOff, Loader2, Trash2 } from "lucide-react"

import { useI18n } from "@/i18n/i18n-provider"
import { advisorApi } from "@/lib/api/endpoints"
import type { CustomServiceState } from "@/lib/api/types"
import { applyApiError } from "@/lib/forms/server-errors"
import { useSchemaForm } from "@/lib/forms/use-schema-form"
import {
  customServiceSchema,
  customServiceValues,
  toCustomServiceInput,
  type CustomServiceContext,
  type CustomServiceValues,
} from "@/lib/schemas/advisor"
import { useAsync } from "@/lib/use-async"
import { announce } from "@/components/feedback/live-region"
import { ErrorState } from "@/components/feedback/error-state"
import { useToast } from "@/components/feedback/toast-provider"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { FieldRow } from "@/components/ui/field"
import { Form, FormRootError } from "@/components/ui/form"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { useSettingsSection, type SaveOptions } from "@/components/psychic/settings/save-registry"
import { SettingsCard } from "@/components/psychic/settings/settings-card"

const FIELD_ORDER = ["name", "price", "days", "description"] as const satisfies readonly (keyof CustomServiceValues)[]

/**
 * A numeric text field with its unit ("2000 credits", "8 days") shown after
 * the value. The label, description and error ids that <FormControl> passes
 * go to the input itself, so it stays properly labelled.
 */
function UnitInput({ unit, ...props }: React.ComponentProps<typeof Input> & { unit: string }) {
  return (
    <span className="advisor-input-unit">
      <Input {...props} inputMode="numeric" autoComplete="off" className="advisor-input" />
      <span aria-hidden>{unit}</span>
    </span>
  )
}
/** Registry order: after the advisor profile. */
export const CUSTOMS_SECTION_ORDER = 20

function CustomsForm({ state, onSaved }: { state: CustomServiceState; onSaved: (next: CustomServiceState) => void }) {
  const { text, locale } = useI18n()
  const toast = useToast()
  const { limits, service } = state
  const format = React.useCallback((n: number) => n.toLocaleString(locale), [locale])
  const context = React.useMemo<CustomServiceContext>(() => ({ limits, format }), [limits, format])
  const form = useSchemaForm<ReturnType<typeof customServiceSchema>, CustomServiceValues>(customServiceSchema, {
    defaultValues: customServiceValues(service),
    context,
    shouldFocusError: false,
  })

  const save = React.useCallback(
    ({ focus }: SaveOptions) =>
      new Promise<boolean>((resolve) => {
        void form.handleSubmit(
          async (values) => {
            try {
              const next = await advisorApi.saveCustomService(toCustomServiceInput(values))
              form.reset(customServiceValues(next.service))
              onSaved(next)
              if (!service) {
                toast.show({ tone: "success", message: text("Your custom offering is now on your profile.") })
              }
              resolve(true)
            } catch (err) {
              applyApiError(form, err, {
                text,
                focus,
                fieldMap: { price_credits: "price", delivery_days: "days" },
              })
              resolve(false)
            }
          },
          (errors) => {
            if (focus) {
              const first = FIELD_ORDER.find((name) => errors[name])
              if (first) form.setFocus(first)
            }
            resolve(false)
          }
        )()
      }),
    [form, onSaved, service, text, toast]
  )
  useSettingsSection("customs", CUSTOMS_SECTION_ORDER, form.formState.isDirty, save)

  return (
    <Form {...form}>
      <form
        noValidate
        className="advisor-settings-fields"
        aria-label={text("Custom offering")}
        data-testid="customs-form"
        onSubmit={(event) => {
          event.preventDefault()
          void save({ focus: true })
        }}
      >
        <FieldRow control={form.control} name="name" label={text("Custom name")} className="advisor-field" labelClassName="advisor-label" maxLength={limits.name_max}>
          {(field) => <Input {...field} autoComplete="off" placeholder={text("e.g. Soulmate sketch")} className="advisor-input" />}
        </FieldRow>
        <FieldRow
          control={form.control}
          name="price"
          label={text("Custom rate")}
          description={text("In credits, between {value0} and {value1}.", {
            value0: format(limits.min_price_credits),
            value1: format(limits.max_price_credits),
          })}
          className="advisor-field"
          labelClassName="advisor-label"
        >
          {(field) => <UnitInput {...field} unit={text("credits")} placeholder="2000" />}
        </FieldRow>
        <FieldRow
          control={form.control}
          name="days"
          label={text("Average delivery time")}
          description={text("In days, between {value0} and {value1}. Shown on your profile.", {
            value0: limits.min_delivery_days,
            value1: limits.max_delivery_days,
          })}
          className="advisor-field"
          labelClassName="advisor-label"
        >
          {(field) => <UnitInput {...field} unit={text("days")} placeholder="7" />}
        </FieldRow>
        <FieldRow
          control={form.control}
          name="description"
          label={text("Description (optional)")}
          description={text("What clients receive and what to include in their request.")}
          showValid={false}
          maxLength={limits.description_max}
          className="advisor-field"
          labelClassName="advisor-label"
        >
          {(field) => <Textarea {...field} rows={3} className="advisor-input" />}
        </FieldRow>
        <FormRootError />
        <button type="submit" hidden tabIndex={-1} aria-hidden />
      </form>
    </Form>
  )
}

function RemoveOffering({ name, onRemoved }: { name: string; onRemoved: () => void }) {
  const { text } = useI18n()
  const toast = useToast()
  const [busy, setBusy] = React.useState(false)
  async function remove() {
    setBusy(true)
    try {
      await advisorApi.deleteCustomService()
      onRemoved()
      toast.show({ tone: "info", message: text("Custom offering removed.") })
      announce(text("Custom offering removed."))
    } catch (err) {
      toast.showError(err)
    } finally {
      setBusy(false)
    }
  }
  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>
        <button type="button" className="advisor-icon-button" disabled={busy} aria-label={text("Remove the custom offering")} data-testid="customs-remove">
          {busy ? <Loader2 aria-hidden className="size-4 animate-spin motion-reduce:animate-none" /> : <Trash2 aria-hidden className="size-4" />}
        </button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{text("Remove “{value0}”?", { value0: name })}</AlertDialogTitle>
          <AlertDialogDescription>
            {text("It disappears from your profile. Requests clients already placed are not affected.")}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{text("Cancel")}</AlertDialogCancel>
          <AlertDialogAction destructive onClick={() => void remove()}>
            {text("Remove offering")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

/**
 * "Customs" (Figma 1219:1613): the advisor's own custom-request offering,
 * with its name, credit rate and average delivery time. It is listed on
 * the public profile after the services curated by Staryield and ordered
 * through the usual request flow.
 */
export function CustomsCard() {
  const { text } = useI18n()
  const state = useAsync((signal) => advisorApi.customService(signal), [])
  const service = state.data?.service ?? null

  return (
    <SettingsCard
      title={text("Customs")}
      icon={<CreditCard />}
      data-testid="customs-card"
      actions={service ? <RemoveOffering name={service.name} onRemoved={() => state.setData((s) => (s ? { ...s, service: null } : s))} /> : null}
    >
      {state.error && !state.data ? (
        <ErrorState error={state.error} onRetry={state.reload} compact title={text("Your custom offering could not be loaded.")} />
      ) : !state.data ? (
        <div className="advisor-settings-fields" aria-busy="true" data-testid="customs-loading">
          <p role="status" className="sr-only">
            {text("Loading your custom offering…")}
          </p>
          {[0, 1, 2].map((i) => (
            <div key={i} className="grid gap-1.5">
              <Skeleton className="h-3.5 w-32" />
              <Skeleton className="h-11 w-full rounded-lg" />
            </div>
          ))}
        </div>
      ) : (
        <>
          {!service ? (
            <p className="advisor-card-intro" data-testid="customs-empty">
              {text("You don't offer a custom request yet. Fill in the details and save to list it on your profile.")}
            </p>
          ) : !service.active ? (
            <p className="advisor-card-notice" role="note">
              <EyeOff aria-hidden className="size-4 shrink-0" />
              {text("The Staryield team has hidden this offering from your profile. Contact support if you think this is a mistake.")}
            </p>
          ) : null}
          <CustomsForm key={service ? `${service.id}:${service.updated_at}` : "new"} state={state.data} onSaved={(next) => state.setData(next)} />
        </>
      )}
    </SettingsCard>
  )
}
