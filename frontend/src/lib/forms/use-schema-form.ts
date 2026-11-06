"use client"

import { useMemo } from "react"
import { useForm, type FieldValues, type Resolver, type UseFormProps, type UseFormReturn } from "react-hook-form"
import { yupResolver } from "@hookform/resolvers/yup"
import type { AnyObjectSchema, InferType } from "yup"

import { useI18n } from "@/i18n/i18n-provider"
import type { TranslateCopy } from "@/i18n/translate-copy"

/** A schema factory: builds a yup schema whose messages use `text()`. */
export type SchemaFactory<S extends AnyObjectSchema> = (text: TranslateCopy) => S

/**
 * `useForm` wired to a yup schema factory, with the project defaults:
 * validate a field once it has been touched, re-validate on every change
 * after that, and focus the first invalid field on submit. Messages are
 * translated because the factory receives the active locale's `text()`.
 *
 * Keep `factory` stable (module-level function), otherwise the schema is
 * rebuilt on every render.
 */
export function useSchemaForm<S extends AnyObjectSchema, TValues extends FieldValues = InferType<S>>(
  factory: SchemaFactory<S>,
  options: Omit<UseFormProps<TValues>, "resolver"> = {}
): UseFormReturn<TValues> {
  const { text } = useI18n()
  const schema = useMemo(() => factory(text), [factory, text])
  return useForm<TValues>({
    mode: "onTouched",
    reValidateMode: "onChange",
    shouldFocusError: true,
    ...options,
    resolver: yupResolver(schema) as unknown as Resolver<TValues>,
  })
}
