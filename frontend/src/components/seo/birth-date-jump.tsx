"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import type { FieldValues, UseFormReturn } from "react-hook-form";
import { LockKeyhole } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { birthDateOf, lifePathSchema, type LifePathFormValues } from "@/lib/schemas/calculator";
import { calculatorResultHref, type JumpCalculator } from "@/lib/seo/calculator-links";
import { BIRTH_DATA_NOTICE } from "@/components/calculators/birth-data-notice";
import { Button } from "@/components/ui/button";
import { DatePartsInput } from "@/components/ui/date-parts-input";
import { FieldRow } from "@/components/ui/field";
import { Form } from "@/components/ui/form";

const SUBMIT: Record<Exclude<JumpCalculator, "astrology">, string> = {
  "path-of-life": "Calculate my Life Path",
  "tarot-birth-card": "Reveal my birth cards",
};

/**
 * The calculator that ends a Life Path or tarot birth card page: a birth
 * date, validated like the calculator's own form, that opens the calculator
 * with the date in the URL fragment (`#v=1&d=…`), so the result shows at once.
 */
export function BirthDateJump({ calculator, id }: { calculator: Exclude<JumpCalculator, "astrology">; id: string }) {
  const { text, locale } = useI18n();
  const router = useRouter();
  const [pending, startTransition] = React.useTransition();
  const form = useSchemaForm(lifePathSchema, {
    defaultValues: { birth: { day: "", month: "", year: "", time: "" } },
  }) as unknown as UseFormReturn<FieldValues>;

  function submit(values: FieldValues) {
    const href = calculatorResultHref(locale, calculator, birthDateOf((values as LifePathFormValues).birth));
    track("calculator_started", { kind: calculator, source: "seo_page" });
    startTransition(() => router.push(href));
  }

  return (
    <Form {...form}>
      <form
        noValidate
        onSubmit={form.handleSubmit(submit)}
        className="calculator-card paper-page calc-card mx-auto"
        data-testid="seo-birth-form"
        data-calculator={calculator}
        aria-labelledby={`${id}-title`}
        aria-busy={pending}
      >
        <FieldRow control={form.control} name="birth" label={text("Date of birth")} labelClassName="calc-label" showValid={false}>
          {(field) => (
            <DatePartsInput
              value={field.value}
              onChange={field.onChange}
              onBlur={field.onBlur}
              ref={field.ref}
              name={field.name}
              aria-label={text("Date of birth")}
              className="calc-date-parts"
            />
          )}
        </FieldRow>
        <div className="mt-6 flex flex-col items-center gap-4">
          <Button
            type="submit"
            className="gold-button calc-cta h-12 w-full"
            disabled={pending}
            aria-busy={pending}
            onMouseDown={keepFocusOnPress}
            data-testid="seo-birth-submit"
          >
            {pending ? text("Opening the calculator…") : text(SUBMIT[calculator])}
          </Button>
          <p className="flex items-center justify-center gap-1.5 text-center text-xs text-[#6b6550]">
            <LockKeyhole aria-hidden className="size-3 shrink-0" />
            {text(BIRTH_DATA_NOTICE)}
          </p>
        </div>
      </form>
    </Form>
  );
}
