"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { placeLabel } from "@/lib/api/astrology";
import { decodeFragment, encodeFragment } from "@/lib/astro/fragment";
import { useLocationHash, writeLocationHash } from "@/lib/astro/hash-store";
import { useAsync } from "@/lib/use-async";
import {
  emptyCalculatorValues,
  type CalculatorFormValues,
  type CalculatorKind,
  type NatalFormValues,
  type SunSignFormValues,
} from "@/lib/schemas/calculator";
import { PublicFrame } from "@/components/layout/public-frame";
import { ErrorState } from "@/components/feedback/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { CalculatorForm } from "@/components/calculators/calculator-form";
import { cachedResult, calculate, rememberResult, type CalculatorResult } from "@/components/calculators/calculate";
import { LifePathResultView, NatalResultView, ResultDate, SunSignResultView } from "@/components/calculators/results";

export type { CalculatorKind };

/** The design's sample birth date, shown by the `/results` reference routes. */
const SAMPLE_FRAGMENT = "#v=1&d=1945-04-30";

/**
 * A public calculator (natal chart, astrology, path of life). The input and
 * the result live in the URL fragment, so a refresh or a shared link shows
 * the same result while birth data never reaches the server's logs. The
 * long-form article (a server-rendered slot) follows the calculator.
 */
export function Calculator({
  kind,
  showResults = false,
  article,
}: {
  kind: CalculatorKind;
  /** `/calculators/<kind>/results`: show the design's sample result when no fragment is set. */
  showResults?: boolean;
  article?: React.ReactNode;
}) {
  const { text } = useI18n();
  const hash = useLocationHash();
  const sample = !hash && showResults;
  const fragment = hash || (showResults ? SAMPLE_FRAGMENT : "");
  const decoded = React.useMemo(() => decodeFragment(kind, fragment, text), [kind, fragment, text]);
  const [draft, setDraft] = React.useState<{ values: CalculatorFormValues; version: number; expandCusp: boolean }>(() => ({
    values: emptyCalculatorValues(kind),
    version: 0,
    expandCusp: false,
  }));
  const focusResult = React.useRef(false);

  const showForm = React.useCallback((values: CalculatorFormValues, expandCusp = false) => {
    setDraft((d) => ({ values, version: d.version + 1, expandCusp }));
    writeLocationHash("", true);
    window.requestAnimationFrame(() => {
      window.scrollTo({ top: 0 });
      document.querySelector<HTMLInputElement>('[data-testid="calculator-form"] input')?.focus();
    });
  }, []);

  const onCalculated = React.useCallback(
    (values: CalculatorFormValues, result: CalculatorResult) => {
      let next = values;
      if (result.kind === "natal-chart" && result.natal.chart.place && "place" in values && values.place) {
        next = { ...values, place: { id: values.place.id, label: placeLabel(result.natal.chart.place) } } as NatalFormValues;
      }
      const nextFragment = encodeFragment(kind, next);
      rememberResult(`${kind}${nextFragment}`, result);
      setDraft((d) => ({ values: next, version: d.version, expandCusp: false }));
      focusResult.current = true;
      writeLocationHash(nextFragment, true);
    },
    [kind],
  );

  return (
    <PublicFrame>
      {decoded.status === "ok" ? (
        <ResultView
          key={fragment}
          kind={kind}
          values={decoded.values}
          fragment={fragment}
          sample={sample}
          focusOnMountRef={focusResult}
          onEdit={(expandCusp) => showForm(decoded.values, expandCusp)}
          onCalculated={onCalculated}
        />
      ) : (
        <CalculatorForm
          key={`${kind}-${draft.version}`}
          kind={kind}
          initialValues={draft.values}
          invalidLink={decoded.status === "invalid"}
          expandCusp={draft.expandCusp}
          onCalculated={onCalculated}
        />
      )}
      {article}
    </PublicFrame>
  );
}

function ResultView({
  kind,
  values,
  fragment,
  sample,
  focusOnMountRef,
  onEdit,
  onCalculated,
}: {
  kind: CalculatorKind;
  values: CalculatorFormValues;
  fragment: string;
  sample: boolean;
  focusOnMountRef: React.RefObject<boolean>;
  onEdit: (expandCusp?: boolean) => void;
  onCalculated: (values: CalculatorFormValues, result: CalculatorResult) => void;
}) {
  const { text } = useI18n();
  const key = `${kind}${fragment}`;
  const cached = cachedResult(key);
  const state = useAsync(
    async (signal) => {
      const result = await calculate(kind, values, signal);
      rememberResult(key, result);
      return result;
    },
    [key],
    !cached,
  );
  const result = cached ?? state.data;
  const error = cached ? null : state.error;

  React.useEffect(() => {
    if (!focusOnMountRef.current) return;
    focusOnMountRef.current = false;
    window.scrollTo({ top: 0 });
    document.getElementById("calculator-result-title")?.focus({ preventScroll: true });
  }, [focusOnMountRef]);

  // The API rejected the link's data (e.g. a time without a place, an unknown
  // place): back to the form with the server's messages on the fields.
  if (error && error.code === "validation_failed") {
    return <CalculatorForm kind={kind} initialValues={values} serverError={error} onCalculated={onCalculated} />;
  }

  const details = result ? resultDetails(result, values, text) : null;
  return (
    <div data-testid="calculator-result" data-kind={kind} aria-busy={!result && !error}>
      <ResultDate values={values} details={details} onChange={() => onEdit(false)} sample={sample} />
      {error ? (
        <section className="bg-cream px-5 py-16">
          <ErrorState error={error} onRetry={state.reload} className="mx-auto max-w-xl" title={text("We could not calculate this result")} />
        </section>
      ) : !result ? (
        <section className="flex flex-col items-center gap-6 bg-cream px-5 py-16" data-testid="calculator-loading">
          <p role="status" className="sr-only">
            {text("Calculating…")}
          </p>
          <Skeleton className="h-10 w-72" />
          <Skeleton className="aspect-square w-full max-w-[560px] rounded-full" />
        </section>
      ) : result.kind === "natal-chart" ? (
        <NatalResultView result={result.natal} />
      ) : result.kind === "astrology" ? (
        <SunSignResultView result={result.sun} onAddTime={() => onEdit(true)} />
      ) : (
        <LifePathResultView result={result.numbers} />
      )}
    </div>
  );
}

/** "08:45 · Paris, Île-de-France, France · Placidus houses" under the date. */
function resultDetails(result: CalculatorResult, values: CalculatorFormValues, text: ReturnType<typeof useI18n>["text"]): React.ReactNode {
  if (result.kind === "natal-chart") {
    const v = values as NatalFormValues;
    const chart = result.natal.chart;
    const parts = [
      chart.time_known && chart.birth_time ? chart.birth_time : text("Birth time unknown"),
      chart.place ? placeLabel(chart.place) : v.place?.label || "",
    ].filter(Boolean);
    return parts.join(" · ");
  }
  if (result.kind === "astrology") {
    const v = values as SunSignFormValues;
    const parts = [v.birth.time || "", v.place?.label || ""].filter(Boolean);
    return parts.length ? parts.join(" · ") : null;
  }
  return null;
}
