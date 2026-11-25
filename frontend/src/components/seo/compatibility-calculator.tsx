"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { FieldValues, UseFormReturn } from "react-hook-form";
import { ExternalLink, LockKeyhole } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import type { Locale } from "@/i18n/config";
import { track } from "@/lib/analytics";
import { astroApi, type SunSignResult, type ZodiacSign } from "@/lib/api/astrology";
import { ApiError } from "@/lib/api/errors";
import { SIGNS, signIndex } from "@/lib/astro/zodiac";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { birthDateOf } from "@/lib/schemas/calculator";
import {
  compatibilitySchema,
  emptyCompatibilityValues,
  type CompatibilityFormValues,
  type CompatibilityPerson,
} from "@/lib/schemas/compatibility";
import { calculatorResultHref } from "@/lib/seo/calculator-links";
import { pairSlug } from "@/content/seo/compatibility/data";
import { SIGN_NAMES } from "@/content/seo/names";
import { BIRTH_DATA_NOTICE } from "@/components/calculators/birth-data-notice";
import { announce } from "@/components/feedback/live-region";
import { Button } from "@/components/ui/button";
import { DatePartsInput } from "@/components/ui/date-parts-input";
import { FieldRow } from "@/components/ui/field";
import { Form, FormRootError } from "@/components/ui/form";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";

const PEOPLE: CompatibilityPerson[] = ["you", "them"];

const LABELS: Record<CompatibilityPerson, { date: string; sign: string; cusp: string }> = {
  you: {
    date: "Your date of birth",
    sign: "Your sign",
    cusp: "You were born on a cusp: the Sun changed sign that day. Choose your sign, or add your birth time to know for sure.",
  },
  them: {
    date: "Their date of birth",
    sign: "Their sign",
    cusp: "Their birthday falls on a cusp: the Sun changed sign that day. Choose their sign, or add their birth time to know for sure.",
  },
};

/** Both sun signs, the API's answers and the dates they came from. */
export type FoundSigns = Record<CompatibilityPerson, { sign: ZodiacSign; result: SunSignResult; date: string }>;

/** The zodiac sign of an API answer ("Aries", "aries"), or null for anything else. */
function toSign(value: string): ZodiacSign | null {
  const index = signIndex(value);
  return index >= 0 ? SIGNS[index] : null;
}

/** The sign to use for a person: their pick on a cusp day, otherwise the API's answer. */
function signFor(found: FoundSigns, choice: Partial<Record<CompatibilityPerson, ZodiacSign>>, person: CompatibilityPerson): ZodiacSign {
  return choice[person] ?? found[person].sign;
}

/** The signs a cusp result offers, in zodiac order (the API's own answer alone when it sent none). */
function cuspOptions(found: FoundSigns[CompatibilityPerson]): ZodiacSign[] {
  const options = (found.result.alternatives ?? []).map(toSign).filter((sign): sign is ZodiacSign => sign !== null);
  return options.length >= 2 ? options : [found.sign];
}

/**
 * The compatibility calculator (hub and pair pages): two birth dates,
 * validated like the calculators' date, each turned into a sun sign by
 * POST /astrology/sun-sign (both requests in parallel). A cusp birthday
 * shows its two possible signs to choose from, with a link to settle it with
 * a birth time; then the canonical pair page opens, or, when the visitor is
 * already on it, the pair is confirmed in place.
 */
export function CompatibilityCalculator({ current, id }: { current?: string; id: string }) {
  const { text, locale } = useI18n();
  const router = useRouter();
  const [navigating, startTransition] = React.useTransition();
  const form = useSchemaForm(compatibilitySchema, {
    defaultValues: emptyCompatibilityValues(),
  }) as unknown as UseFormReturn<FieldValues>;
  const [found, setFound] = React.useState<FoundSigns | null>(null);
  const [choice, setChoice] = React.useState<Partial<Record<CompatibilityPerson, ZodiacSign>>>({});
  const [here, setHere] = React.useState<[ZodiacSign, ZodiacSign] | null>(null);
  const inFlight = React.useRef<AbortController | null>(null);
  const resultRef = React.useRef<HTMLDivElement>(null);
  const names = SIGN_NAMES[locale as Locale];

  React.useEffect(() => {
    const requests = inFlight;
    return () => requests.current?.abort();
  }, []);

  const open = React.useCallback(
    (a: ZodiacSign, b: ZodiacSign) => {
      const slug = pairSlug(a, b);
      if (slug === current) {
        setFound(null);
        setHere([a, b]);
        announce(text("Your signs are {value0} and {value1}: this is your pairing.", { value0: names[a], value1: names[b] }));
        window.requestAnimationFrame(() => resultRef.current?.focus());
        return;
      }
      startTransition(() => router.push(`/${locale}/compatibility/${slug}`));
    },
    [current, locale, names, router, text],
  );

  async function submit(raw: FieldValues) {
    const values = raw as CompatibilityFormValues;
    inFlight.current?.abort();
    const ctrl = new AbortController();
    inFlight.current = ctrl;
    setFound(null);
    setHere(null);
    setChoice({});
    track("calculator_started", { kind: "compatibility" });
    const dates = { you: birthDateOf(values.you), them: birthDateOf(values.them) };
    const settled = await Promise.allSettled(PEOPLE.map((person) => astroApi.sunSign({ birth_date: dates[person] }, ctrl.signal)));
    if (ctrl.signal.aborted) return;

    let failed = false;
    settled.forEach((outcome, index) => {
      if (outcome.status === "fulfilled") return;
      failed = true;
      const e = applyApiError(form, outcome.reason, { text, fieldMap: { birth_date: PEOPLE[index] } });
      if (e.code !== "aborted") track("error_displayed", { code: String(e.code), area: "compatibility" });
    });
    if (failed) return;

    const [you, them] = settled.map((outcome) => (outcome as PromiseFulfilledResult<SunSignResult>).value);
    const [signA, signB] = [toSign(you.sun_sign), toSign(them.sun_sign)];
    if (!signA || !signB) {
      applyApiError(form, new ApiError({ code: "client_error", message: "Unexpected sun sign." }), { text });
      track("error_displayed", { code: "client_error", area: "compatibility" });
      return;
    }
    track("calculator_completed", { kind: "compatibility" });
    const next: FoundSigns = { you: { sign: signA, result: you, date: dates.you }, them: { sign: signB, result: them, date: dates.them } };
    if (you.uncertain || them.uncertain) {
      setFound(next);
      announce(text("A birthday falls on a cusp: choose the sign to use."));
      window.requestAnimationFrame(() => resultRef.current?.focus());
      return;
    }
    open(signA, signB);
  }

  const pending = form.formState.isSubmitting || navigating;
  const serverError = Boolean((form.formState.errors.root as { server?: unknown } | undefined)?.server);

  return (
    <div className="mx-auto flex w-full max-w-[624px] flex-col gap-6">
      <Form {...form}>
        <form
          noValidate
          onSubmit={(event) => void form.handleSubmit(submit)(event)}
          className="calculator-card paper-page calc-card"
          data-testid="compatibility-form"
          aria-labelledby={`${id}-title`}
          aria-busy={pending}
        >
          <FormRootError className="mb-6">
            {serverError ? (
              <Button type="button" variant="outline" size="sm" className="justify-self-start" onClick={() => void form.handleSubmit(submit)()}>
                {text("Try again")}
              </Button>
            ) : null}
          </FormRootError>
          <div className="grid gap-5">
            {PEOPLE.map((person) => (
              <FieldRow key={person} control={form.control} name={person} label={text(LABELS[person].date)} labelClassName="calc-label" showValid={false}>
                {(field) => (
                  <DatePartsInput
                    value={field.value}
                    onChange={field.onChange}
                    onBlur={field.onBlur}
                    ref={field.ref}
                    name={field.name}
                    aria-label={text(LABELS[person].date)}
                    className="calc-date-parts"
                  />
                )}
              </FieldRow>
            ))}
          </div>
          <div className="mt-8 flex flex-col items-center gap-4">
            <Button
              type="submit"
              className="gold-button calc-cta h-12 w-full"
              disabled={pending}
              aria-busy={pending}
              onMouseDown={keepFocusOnPress}
              data-testid="compatibility-submit"
            >
              {form.formState.isSubmitting ? text("Finding your signs…") : navigating ? text("Opening your pairing…") : text("Reveal our compatibility")}
            </Button>
            <p className="flex items-center justify-center gap-1.5 text-center text-xs text-[#6b6550]">
              <LockKeyhole aria-hidden className="size-3 shrink-0" />
              {text(BIRTH_DATA_NOTICE)}
            </p>
          </div>
        </form>
      </Form>

      {found ? (
        <div
          ref={resultRef}
          tabIndex={-1}
          className="rounded-xl border border-gold/60 bg-cream-light/80 p-5 outline-none sm:p-6"
          data-testid="compatibility-cusp"
          aria-labelledby={`${id}-cusp-title`}
          role="group"
        >
          <h3 id={`${id}-cusp-title`} className="font-display text-lg tracking-[0.6px] uppercase">
            {text("Born on a cusp")}
          </h3>
          <div className="mt-4 grid gap-5">
            {PEOPLE.filter((person) => found[person].result.uncertain).map((person) => (
              <fieldset key={person} className="grid gap-3" data-testid={`cusp-${person}`}>
                <legend className="text-sm leading-normal">{text(LABELS[person].cusp)}</legend>
                <RadioGroup
                  value={signFor(found, choice, person)}
                  onValueChange={(value) => {
                    const sign = toSign(value);
                    if (sign) setChoice((c) => ({ ...c, [person]: sign }));
                  }}
                  aria-label={text(LABELS[person].sign)}
                  className="flex flex-wrap gap-3"
                >
                  {cuspOptions(found[person]).map((sign) => (
                    <label key={sign} className="flex cursor-pointer items-center gap-2 rounded-lg border border-warm-gray bg-cream px-3 py-2 text-sm font-semibold">
                      <RadioGroupItem value={sign} data-testid={`cusp-${person}-${sign.toLowerCase()}`} />
                      {names[sign]}
                    </label>
                  ))}
                </RadioGroup>
                <Link
                  href={calculatorResultHref(locale, "astrology", found[person].date)}
                  target="_blank"
                  rel="noopener"
                  className="inline-flex items-center gap-1.5 text-sm font-semibold text-[#6b4f1d] underline decoration-[#c29a3d] underline-offset-2 hover:text-ink"
                >
                  {text("Add a birth time in the zodiac sign calculator")}
                  <ExternalLink aria-hidden className="size-3.5" />
                  <span className="sr-only">{text("(opens in a new tab)")}</span>
                </Link>
              </fieldset>
            ))}
          </div>
          <Button
            type="button"
            variant="cta"
            size="xl"
            className="mt-6 w-full"
            disabled={navigating}
            aria-busy={navigating}
            onClick={() => open(signFor(found, choice, "you"), signFor(found, choice, "them"))}
            data-testid="compatibility-continue"
          >
            {navigating ? text("Opening your pairing…") : text("See our compatibility")}
          </Button>
        </div>
      ) : null}

      {here ? (
        <div
          ref={resultRef}
          tabIndex={-1}
          role="status"
          className="rounded-xl border border-gold/60 bg-cream-light/80 p-5 text-center outline-none sm:p-6"
          data-testid="compatibility-here"
        >
          <p className="text-base leading-normal">
            {text("Your signs are {value0} and {value1}: this is your pairing.", { value0: names[here[0]], value1: names[here[1]] })}
          </p>
          <a href="#overview" className="mt-3 inline-block text-sm font-semibold text-[#6b4f1d] underline decoration-[#c29a3d] underline-offset-2 hover:text-ink">
            {text("Read your compatibility")}
          </a>
        </div>
      ) : null}
    </div>
  );
}
