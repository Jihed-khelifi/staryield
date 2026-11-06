"use client"

import * as React from "react"
import type {
  Control,
  ControllerRenderProps,
  FieldPath,
  FieldValues,
} from "react-hook-form"

import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
  FormValidMark,
  useFieldValidity,
} from "@/components/ui/form"
import { cn } from "@/lib/utils"

type FieldRowProps<
  TFieldValues extends FieldValues,
  TName extends FieldPath<TFieldValues>,
> = {
  control?: Control<TFieldValues>
  name: TName
  label: React.ReactNode
  description?: React.ReactNode
  /** Mark the field as valid (green + check) once it holds a valid value. */
  showValid?: boolean
  /** Show a "used / max" character counter for string values. */
  maxLength?: number
  className?: string
  labelClassName?: string
  /** Renders the control; spread `field` onto it (value, onChange, ref…). */
  children: (
    field: ControllerRenderProps<TFieldValues, TName>
  ) => React.ReactElement
}

/**
 * One labelled form field wired to react-hook-form: label, control,
 * description, error message (aria-live), valid mark and optional counter.
 * The label, description and message are linked to the control through
 * `aria-describedby` / `aria-invalid` by <FormControl>.
 */
export function FieldRow<
  TFieldValues extends FieldValues,
  TName extends FieldPath<TFieldValues>,
>({
  control,
  name,
  label,
  description,
  showValid = true,
  maxLength,
  className,
  labelClassName,
  children,
}: FieldRowProps<TFieldValues, TName>) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem showValid={showValid} className={className}>
          <FormLabel className={labelClassName}>{label}</FormLabel>
          <FormControl>{children(field)}</FormControl>
          {description ? <FormDescription>{description}</FormDescription> : null}
          <FieldFooter
            showValid={showValid}
            maxLength={maxLength}
            value={field.value}
          />
        </FormItem>
      )}
    />
  )
}

function FieldFooter({
  showValid,
  maxLength,
  value,
}: {
  showValid: boolean
  maxLength?: number
  value: unknown
}) {
  const { valid } = useFieldValidity(showValid)
  const length = typeof value === "string" ? Array.from(value).length : 0
  const over = maxLength !== undefined && length > maxLength
  return (
    <div className="flex min-h-0 items-start gap-3">
      <FormMessage />
      {valid ? <FormValidMark /> : null}
      {maxLength !== undefined ? (
        <span
          aria-hidden
          data-slot="field-counter"
          className={cn(
            "ml-auto text-xs tabular-nums text-muted-foreground",
            over && "text-danger"
          )}
        >
          {length}/{maxLength}
        </span>
      ) : null}
    </div>
  )
}
