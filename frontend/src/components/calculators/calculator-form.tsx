"use client";

import * as React from "react";
import { useWatch, type FieldValues, type UseFormReturn } from "react-hook-form";
import { ChevronDown, LockKeyhole } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { astroApi, type HouseSystem } from "@/lib/api/astrology";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import {
  CALCULATOR_FIELD_MAP,
  calculatorSchemas,
  type CalculatorFormValues,
  type CalculatorKind,
  type CalculatorPlace,
} from "@/lib/schemas/calculator";
import { calculate, type CalculatorResult } from "@/components/calculators/calculate";
import { PlaceCombobox } from "@/components/astrology/place-combobox";
import { RadiantPanel } from "@/components/forms/radiant-panel";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { DatePartsInput } from "@/components/ui/date-parts-input";
import { FieldRow } from "@/components/ui/field";
import { Form, FormControl, FormDescription, FormField, FormItem, FormMessage, FormRootError } from "@/components/ui/form";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";

const DESCRIPTIONS: Record<CalculatorKind, string> = {
  "natal-chart":
    "Discover your cosmic blueprint — enter your birth date, time, and location to generate a personalized natal chart.",
  "path-of-life":
    "Your birth date holds the mathematical blueprint of your destiny. We use it to calculate your Life Path number—the primary force shaping your journey.",
  astrology:
    "Discover your zodiac sign instantly — enter your birth date and uncover the traits, elements, and cosmic insights tied to your star sign.",
};

/** Privacy line under each calculator (natal and astrology run on the server). */
const PRIVACY: Record<CalculatorKind, string> = {
  "natal-chart": "Your birth data is used only to draw this spread and is never stored.",
  astrology: "Your data is processed securely & never stored.",
  "path-of-life": "Your data is calculated locally & never stored.",
};

const HOUSE_OPTIONS: { value: HouseSystem; label: string; hint: string }[] = [
  { value: "placidus", label: "Placidus (default)", hint: "Time-based houses, the modern Western standard." },
  { value: "whole_sign", label: "Whole Sign", hint: "Each sign is one house, counted from your rising sign." },
  { value: "equal", label: "Equal", hint: "Twelve 30° houses measured from your Ascendant." },
];

type Props = {
  kind: CalculatorKind;
  initialValues: CalculatorFormValues;
  /** A failed calculation of `initialValues` (e.g. a 422 for a shared link). */
  serverError?: unknown;
  /** The link's fragment could not be used. */
  invalidLink?: boolean;
  /** Astrology: open "Add your birth time and place" (from a cusp result). */
  expandCusp?: boolean;
  onCalculated: (values: CalculatorFormValues, result: CalculatorResult) => void;
};

/** The calculator's input card (Figma 478:1781, 478:2031, 478:2245). */
export function CalculatorForm({ kind, initialValues, serverError, invalidLink = false, expandCusp = false, onCalculated }: Props) {
  const { text } = useI18n();
  const form = useSchemaForm(calculatorSchemas[kind] as never, { defaultValues: initialValues as never }) as unknown as UseFormReturn<FieldValues>;
  const appliedError = React.useRef<unknown>(null);
  const unknownId = React.useId();

  React.useEffect(() => {
    track("calculator_started", { kind });
  }, [kind]);

  // A calculation that failed before this form was shown (a shared link the
  // API rejected): put its field messages on the fields once.
  React.useEffect(() => {
    if (!serverError || appliedError.current === serverError) return;
    appliedError.current = serverError;
    applyApiError(form, serverError, { text, fieldMap: CALCULATOR_FIELD_MAP });
  }, [serverError, form, text]);

  const timeUnknown = useWatch({ control: form.control, name: "timeUnknown" }) as boolean | undefined;
  const place = useWatch({ control: form.control, name: "place" }) as CalculatorPlace | null | undefined;
  const birthTime = useWatch({ control: form.control, name: "birth.time" }) as string | undefined;
  const [cuspOpen, setCuspOpen] = React.useState(
    () => kind === "astrology" && (expandCusp || Boolean(initialValues.birth.time || ("place" in initialValues && initialValues.place))),
  );

  // A place restored from a link carries only its id: look its name up.
  const placeId = place?.id && !place.label ? place.id : null;
  React.useEffect(() => {
    if (!placeId) return;
    const ctrl = new AbortController();
    astroApi.place(placeId, ctrl.signal).then(
      (p) => {
        if (!ctrl.signal.aborted) form.setValue("place", { id: p.id, label: [p.name, p.admin1 !== p.name ? p.admin1 : "", p.country].filter(Boolean).join(", ") });
      },
      () => {
        if (!ctrl.signal.aborted) form.setError("place", { type: "server", message: text("Choose a birth place from the list.") });
      },
    );
    return () => ctrl.abort();
  }, [placeId, form, text]);

  async function submit(values: FieldValues) {
    try {
      const result = await calculate(kind, values as CalculatorFormValues);
      track("calculator_completed", { kind });
      onCalculated(values as CalculatorFormValues, result);
    } catch (err) {
      const e = applyApiError(form, err, { text, fieldMap: CALCULATOR_FIELD_MAP });
      if (e.code !== "aborted") track("error_displayed", { code: String(e.code), area: "calculator" });
    }
  }

  const pending = form.formState.isSubmitting;
  const withTime = kind === "natal-chart" ? !timeUnknown : kind === "astrology" ? cuspOpen : false;

  return (
    <RadiantPanel>
      <Form {...form}>
        <form
          noValidate
          onSubmit={form.handleSubmit(submit)}
          className="calculator-card paper-page calc-card"
          data-testid="calculator-form"
          data-kind={kind}
          aria-labelledby="calculator-title"
        >
          <div className="flex flex-col items-center gap-3 text-center">
            <h1 id="calculator-title" className="text-[30px] leading-tight font-normal sm:text-[36px]">
              {text("Date of Birth")}
            </h1>
            <p className="text-sm leading-normal font-light">{text(DESCRIPTIONS[kind])}</p>
          </div>

          {invalidLink ? (
            <p role="alert" className="mt-6 rounded-lg border border-danger/40 bg-danger/5 p-3 text-sm text-danger">
              {text("This link's birth details are incomplete or invalid. Please check them below.")}
            </p>
          ) : null}
          <FormRootError className="mt-6" />

          <div className="mt-8 grid gap-5">
            <FieldRow control={form.control} name="birth" label={text("Date of birth")} labelClassName="sr-only" showValid={false}>
              {(field) => (
                <DatePartsInput
                  value={field.value}
                  onChange={field.onChange}
                  onBlur={field.onBlur}
                  ref={field.ref}
                  name={field.name}
                  withTime={withTime}
                  aria-label={withTime ? text("Date and time of birth") : text("Date of birth")}
                  className="calc-date-parts"
                />
              )}
            </FieldRow>

            {kind === "natal-chart" ? (
              <>
                <FormField
                  control={form.control}
                  name="timeUnknown"
                  render={({ field }) => (
                    <FormItem className="-mt-2">
                      <div className="flex items-center gap-2 text-sm">
                        <FormControl>
                          <Checkbox
                            ref={field.ref}
                            name={field.name}
                            checked={Boolean(field.value)}
                            aria-labelledby={unknownId}
                            onBlur={field.onBlur}
                            onMouseDown={keepFocusOnPress}
                            onCheckedChange={(checked) => {
                              const unknown = checked === true;
                              field.onChange(unknown);
                              if (unknown) form.setValue("birth", { ...form.getValues("birth"), time: "" });
                              if (form.formState.isSubmitted || form.getFieldState("birth").isTouched) void form.trigger(["birth", "place"]);
                            }}
                          />
                        </FormControl>
                        <span id={unknownId}>{text("I don't know my birth time")}</span>
                      </div>
                      {timeUnknown ? (
                        <p className="text-xs leading-snug font-light">
                          {text("No problem: we use noon and whole-sign houses from your Sun, so a few cards may vary.")}
                        </p>
                      ) : null}
                    </FormItem>
                  )}
                />
                <PlaceField
                  form={form}
                  hint={
                    birthTime && !place?.id
                      ? text("Needed to use your birth time: it sets your time zone and houses.")
                      : text("Your birthplace sets the time zone of your birth time.")
                  }
                />
                <AdvancedSettings form={form} disabled={Boolean(timeUnknown)} />
              </>
            ) : null}

            {kind === "astrology" ? (
              <Collapsible
                open={cuspOpen}
                onOpenChange={(open) => {
                  setCuspOpen(open);
                  if (!open) {
                    form.setValue("birth", { ...form.getValues("birth"), time: "" });
                    form.setValue("place", null);
                    form.clearErrors(["place"]);
                  }
                }}
              >
                <CollapsibleTrigger className="calc-disclosure group" data-testid="cusp-toggle">
                  <span>{text("Born near a sign change? Add your birth time and place")}</span>
                  <ChevronDown aria-hidden className="size-4 shrink-0 transition-transform group-data-[state=open]:rotate-180 motion-reduce:transition-none" />
                </CollapsibleTrigger>
                <CollapsibleContent className="mt-4">
                  <PlaceField form={form} hint={text("On a cusp day your birth time and place decide which sign the Sun was in.")} />
                </CollapsibleContent>
              </Collapsible>
            ) : null}
          </div>

          <div className="mt-8 flex flex-col items-center gap-4">
            <Button
              type="submit"
              className="gold-button calc-cta h-12 w-full"
              disabled={pending}
              aria-busy={pending}
              onMouseDown={keepFocusOnPress}
              data-testid="calculator-submit"
            >
              {pending ? text("Calculating…") : text("Reveal your path")}
            </Button>
            <p className="flex items-center justify-center gap-1.5 text-center text-xs text-[#6b6550]">
              <LockKeyhole aria-hidden className="size-3 shrink-0" />
              {text(PRIVACY[kind])}
            </p>
          </div>
        </form>
      </Form>
    </RadiantPanel>
  );
}

function PlaceField({ form, hint }: { form: UseFormReturn<FieldValues>; hint: string }) {
  const { text } = useI18n();
  return (
    <FieldRow control={form.control} name="place" label={text("Birthplace")} description={hint} labelClassName="calc-label">
      {(field) => (
        <PlaceCombobox
          value={(field.value as CalculatorPlace | null) ?? null}
          onChange={(value) => field.onChange(value)}
          onBlur={field.onBlur}
          ref={field.ref}
          name={field.name}
          placeholder={text("Search a city")}
          inputClassName="calc-input"
        />
      )}
    </FieldRow>
  );
}

function AdvancedSettings({ form, disabled }: { form: UseFormReturn<FieldValues>; disabled: boolean }) {
  const { text } = useI18n();
  const [open, setOpen] = React.useState(() => form.getValues("houseSystem") !== "placidus");
  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="calc-disclosure group" data-testid="advanced-settings">
        <span>{text("Advanced settings")}</span>
        <ChevronDown aria-hidden className="size-4 shrink-0 transition-transform group-data-[state=open]:rotate-180 motion-reduce:transition-none" />
      </CollapsibleTrigger>
      <CollapsibleContent className="mt-4">
        <FormField
          control={form.control}
          name="houseSystem"
          render={({ field }) => (
            <FormItem>
              <p id="house-system-label" className="calc-label">
                {text("House system")}
              </p>
              <FormControl>
                <RadioGroup
                  value={field.value}
                  onValueChange={field.onChange}
                  name={field.name}
                  aria-labelledby="house-system-label"
                  className="gap-2.5"
                >
                  {HOUSE_OPTIONS.map((option) => (
                    <label key={option.value} className="flex cursor-pointer items-start gap-2.5 text-sm">
                      <RadioGroupItem
                        value={option.value}
                        className="mt-0.5"
                        aria-labelledby={`house-name-${option.value}`}
                        aria-describedby={`house-hint-${option.value}`}
                        data-testid={`house-${option.value}`}
                      />
                      <span>
                        <span id={`house-name-${option.value}`} className="font-semibold">
                          {text(option.label)}
                        </span>
                        <span id={`house-hint-${option.value}`} className="block text-xs font-light">
                          {text(option.hint)}
                        </span>
                      </span>
                    </label>
                  ))}
                </RadioGroup>
              </FormControl>
              <FormDescription className="text-xs text-ink/75">
                {disabled
                  ? text("Without a birth time, houses are counted whole-sign from your Sun.")
                  : text("Choose how the sky around your birthplace is divided into houses.")}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      </CollapsibleContent>
    </Collapsible>
  );
}
