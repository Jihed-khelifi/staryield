"use client"

import * as React from "react"
import { Slot } from "radix-ui"
import { AlertCircle, CheckCircle2 } from "lucide-react"
import {
  Controller,
  FormProvider,
  useFormContext,
  useFormState,
  useWatch,
  type ControllerProps,
  type FieldPath,
  type FieldValues,
} from "react-hook-form"

import { useI18n } from "@/i18n/i18n-provider"
import { cn } from "@/lib/utils"
import { Label } from "@/components/ui/label"

const Form = FormProvider

type FormFieldContextValue<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> = {
  name: TName
}

// `null` outside <FormField>, so misuse is detected before the context is read.
const FormFieldContext = React.createContext<FormFieldContextValue | null>(null)

function FormField<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>({ ...props }: ControllerProps<TFieldValues, TName>) {
  return (
    <FormFieldContext.Provider value={{ name: props.name }}>
      <Controller {...props} />
    </FormFieldContext.Provider>
  )
}

type FormItemContextValue = {
  id: string
}

const FormItemContext = React.createContext<FormItemContextValue | null>(null)

function useFormField() {
  const fieldContext = React.useContext(FormFieldContext)
  const itemContext = React.useContext(FormItemContext)
  if (!fieldContext) {
    throw new Error("useFormField should be used within <FormField>")
  }
  if (!itemContext) {
    throw new Error("useFormField should be used within <FormItem>")
  }

  const { getFieldState } = useFormContext()
  const formState = useFormState({ name: fieldContext.name })
  const fieldState = getFieldState(fieldContext.name, formState)
  const { id } = itemContext

  return {
    id,
    name: fieldContext.name,
    formItemId: `${id}-form-item`,
    formDescriptionId: `${id}-form-item-description`,
    formMessageId: `${id}-form-item-message`,
    isSubmitted: formState.isSubmitted,
    ...fieldState,
  }
}

function hasValue(value: unknown): boolean {
  if (value === null || value === undefined) return false
  if (typeof value === "string") return value.trim().length > 0
  if (Array.isArray(value)) return value.length > 0
  if (typeof value === "boolean") return value
  return true
}

/**
 * Validation state of the surrounding field. A field only counts as *valid*
 * once the user has interacted with it (or submitted), it has no error and it
 * holds a value — optional fields left empty never turn green.
 */
function useFieldValidity(showValid: boolean) {
  const { name, error, isTouched, isDirty, isSubmitted } = useFormField()
  const value = useWatch({ name })
  const invalid = Boolean(error)
  const valid =
    showValid && !invalid && (isTouched || isDirty || isSubmitted) && hasValue(value)
  return { invalid, valid }
}

function FormItem({
  className,
  showValid = false,
  ...props
}: React.ComponentProps<"div"> & {
  /** Mark the field green once it holds a valid value. */
  showValid?: boolean
}) {
  const id = React.useId()
  const field = React.useContext(FormFieldContext)

  return (
    <FormItemContext.Provider value={{ id }}>
      {field ? (
        <FormItemBody className={className} showValid={showValid} {...props} />
      ) : (
        <div data-slot="field" className={cn("grid gap-2", className)} {...props} />
      )}
    </FormItemContext.Provider>
  )
}

function FormItemBody({
  className,
  showValid,
  ...props
}: React.ComponentProps<"div"> & { showValid: boolean }) {
  const { invalid, valid } = useFieldValidity(showValid)
  return (
    <div
      data-slot="field"
      data-invalid={invalid || undefined}
      data-valid={valid || undefined}
      className={cn("grid gap-2", className)}
      {...props}
    />
  )
}

function FormLabel({
  className,
  ...props
}: React.ComponentProps<typeof Label>) {
  const { error, formItemId } = useFormField()

  return (
    <Label
      data-slot="form-label"
      data-error={!!error}
      className={cn("data-[error=true]:text-danger", className)}
      htmlFor={formItemId}
      {...props}
    />
  )
}

function FormControl({ ...props }: React.ComponentProps<typeof Slot.Root>) {
  const { error, formItemId, formDescriptionId, formMessageId } = useFormField()

  return (
    <Slot.Root
      data-slot="form-control"
      id={formItemId}
      aria-describedby={
        error ? `${formDescriptionId} ${formMessageId}` : formDescriptionId
      }
      aria-invalid={!!error}
      {...props}
    />
  )
}

function FormDescription({ className, ...props }: React.ComponentProps<"p">) {
  const { formDescriptionId } = useFormField()

  return (
    <p
      data-slot="form-description"
      id={formDescriptionId}
      className={cn("text-sm text-muted-foreground", className)}
      {...props}
    />
  )
}

/**
 * Field error, announced politely. Messages come from yup schemas (built
 * with `text()`) or from the API (already localized), so they are rendered
 * as-is.
 */
function FormMessage({ className, ...props }: React.ComponentProps<"p">) {
  const { error, formMessageId } = useFormField()
  const body = error ? String(error?.message ?? "") : props.children

  return (
    <p
      data-slot="form-message"
      data-tone={error ? "error" : undefined}
      id={formMessageId}
      aria-live="polite"
      className={cn("field-message", !body && "sr-only", className)}
      {...props}
    >
      {body && error ? <AlertCircle aria-hidden className="mt-0.5 size-3.5 shrink-0" /> : null}
      {body}
    </p>
  )
}

/** Small "valid" confirmation shown next to a field that passed validation. */
function FormValidMark({ className }: { className?: string }) {
  const { text } = useI18n()
  return (
    <span data-slot="form-valid" className={cn("field-message", className)} data-tone="success">
      <CheckCircle2 aria-hidden className="mt-0.5 size-3.5 shrink-0" />
      <span className="sr-only">{text("Looks good")}</span>
    </span>
  )
}

/**
 * Form-level error (`errors.root.server`): API failures that do not belong
 * to a single field, e.g. a network outage or "email already registered".
 */
function FormRootError({
  className,
  children,
}: {
  className?: string
  children?: React.ReactNode
}) {
  const { formState } = useFormContext()
  const root = formState.errors.root as
    | (Record<string, { message?: string }> & { message?: string })
    | undefined
  const message = root?.server?.message ?? root?.message
  if (!message && !children) return null
  return (
    <div
      role="alert"
      data-slot="form-root-error"
      className={cn(
        "flex items-start gap-2 rounded-lg border border-danger/40 bg-danger/5 p-3 text-sm text-danger",
        className
      )}
    >
      <AlertCircle aria-hidden className="mt-0.5 size-4 shrink-0" />
      <div className="grid gap-2">
        {message ? <p>{message}</p> : null}
        {children}
      </div>
    </div>
  )
}

export {
  useFormField,
  useFieldValidity,
  Form,
  FormItem,
  FormLabel,
  FormControl,
  FormDescription,
  FormMessage,
  FormValidMark,
  FormRootError,
  FormField,
}
