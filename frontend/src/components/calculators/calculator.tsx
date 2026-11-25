"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { placeLabel } from "@/lib/api/astrology";
import { decodeFragment, encodeFragment, natalFragmentFromSunSign } from "@/lib/astro/fragment";
import { useLocationHash, writeLocationHash } from "@/lib/astro/hash-store";
import type { HouseSystem } from "@/lib/api/astrology";
import {
  birthDateOf,
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
import { rememberResult, type CalculatorResult } from "@/components/calculators/calculate";
import { useCalculatorResult } from "@/components/calculators/use-calculator-result";
import { LifePathResultView, NatalResultView, ResultDate, SunSignResultView } from "@/components/calculators/results";
import { TarotBirthResultView } from "@/components/calculators/tarot-result";
import { ShareResultDialog, ShareResultProvider } from "@/components/calculators/share-result";
import { isReportable } from "@/components/calculators/report-email";
// revisions 6.0 (§26.4): the e-mailed report.
import { ReportEmailDialog } from "@/components/calculators/report-email-dialog";
import { useReportEmailPrompt } from "@/components/calculators/use-report-email-prompt";

export type { CalculatorKind };

/** The design's sample birth date, shown by the `/results` reference routes. */
const SAMPLE_FRAGMENT = "#v=1&d=1945-04-30";

/**
 * A public calculator (natal chart, astrology, path of life, tarot birth
 * card). The input and the result live in the URL fragment, so a refresh or a
 * shared link shows the same result while birth data never reaches the
 * server's logs. The long-form article (a server-rendered slot) follows the
 * calculator.
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
  // On `/results` the design's sample stands in for an empty fragment until
  // the visitor asks for the form; from then on an empty fragment shows the
  // form there too (clearing an already empty fragment changes nothing else).
  const [sampleDismissed, setSampleDismissed] = React.useState(false);
  const sample = !hash && showResults && !sampleDismissed;
  const fragment = hash || (sample ? SAMPLE_FRAGMENT : "");
  const decoded = React.useMemo(() => decodeFragment(kind, fragment, text), [kind, fragment, text]);
  const [draft, setDraft] = React.useState<{ values: CalculatorFormValues; version: number; expandCusp: boolean }>(() => ({
    values: emptyCalculatorValues(kind),
    version: 0,
    expandCusp: false,
  }));
  const focusResult = React.useRef(false);

  const showForm = React.useCallback((values: CalculatorFormValues, expandCusp = false) => {
    setDraft((d) => ({ values, version: d.version + 1, expandCusp }));
    setSampleDismissed(true);
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

  // Changing the house system recalculates the natal chart in place (same
  // request, new `h=`): the view is keyed without it, so the previous chart
  // stays on screen while the new one loads.
  const resultKey =
    decoded.status === "ok" && kind === "natal-chart" ? encodeFragment(kind, { ...(decoded.values as NatalFormValues), houseSystem: "placidus" }) : fragment;
  const onHouseSystem = React.useCallback(
    (houseSystem: HouseSystem) => {
      if (decoded.status !== "ok" || kind !== "natal-chart") return;
      writeLocationHash(encodeFragment(kind, { ...(decoded.values as NatalFormValues), houseSystem }), true);
    },
    [decoded, kind],
  );

  return (
    <PublicFrame>
      {decoded.status === "ok" ? (
        <ResultView
          key={resultKey}
          kind={kind}
          values={decoded.values}
          sample={sample}
          focusOnMountRef={focusResult}
          // The sample's date is the design's: the visitor starts from an empty form.
          onEdit={(expandCusp) => showForm(sample ? emptyCalculatorValues(kind) : decoded.values, expandCusp)}
          onCalculated={onCalculated}
          onHouseSystem={onHouseSystem}
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
  sample,
  focusOnMountRef,
  onEdit,
  onCalculated,
  onHouseSystem,
}: {
  kind: CalculatorKind;
  values: CalculatorFormValues;
  sample: boolean;
  focusOnMountRef: React.RefObject<boolean>;
  onEdit: (expandCusp?: boolean) => void;
  onCalculated: (values: CalculatorFormValues, result: CalculatorResult) => void;
  onHouseSystem: (system: HouseSystem) => void;
}) {
  const { text, locale } = useI18n();
  const key = `${kind}${encodeFragment(kind, values)}`;
  const { result, error, pending, reload } = useCalculatorResult(kind, values, key);
  const [shareOpen, setShareOpen] = React.useState(false);
  const openShare = React.useCallback(() => setShareOpen(true), []);
  // revisions 6.0 (§26.4): "Save your reading" opens once a computed result has been on screen (never on the sample,
  // and never for the tarot birth card, which has no e-mail report). It holds back while the share dialog is open.
  const emailable = isReportable(kind);
  const report = useReportEmailPrompt(kind, emailable && Boolean(result) && !error && !sample, shareOpen);

  React.useEffect(() => {
    if (!focusOnMountRef.current) return;
    focusOnMountRef.current = false;
    window.scrollTo({ top: 0 });
    document.getElementById("calculator-result-title")?.focus({ preventScroll: true });
  }, [focusOnMountRef]);

  // The API rejected the link's data (e.g. a time without a place, an unknown
  // place): back to the form with the server's messages on the fields.
  if (error && !result && error.code === "validation_failed") {
    return <CalculatorForm kind={kind} initialValues={values} serverError={error} onCalculated={onCalculated} />;
  }

  const details = result ? resultDetails(result, values, text) : null;
  // The sample date is the design's, not the visitor's: an image saying "My
  // tarot birth card is…" would not be theirs, so the sample offers no share.
  // Every share button of the result reads this opener from context.
  const share = result && !sample ? openShare : null;
  const fatal = error && !result ? error : null;
  return (
    <ShareResultProvider onShare={share}>
      <div data-testid="calculator-result" data-kind={kind} aria-busy={(!result && !error) || pending}>
        <ResultDate
          values={values}
          title={kind === "tarot-birth-card" ? text("Tarot birth card calculated for:") : undefined}
          details={details}
          onChange={() => onEdit(false)}
          sample={sample}
          onEmail={result && !sample && emailable && report.available ? report.openManually : undefined}
        />
        {fatal ? (
          <section className="bg-cream px-5 py-16">
            <ErrorState error={fatal} onRetry={reload} className="mx-auto max-w-xl" title={text("We could not calculate this result")} />
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
          <NatalResultView
            result={result.natal}
            houses={{ houseSystem: (values as NatalFormValues).houseSystem, onChange: onHouseSystem, updating: pending, error, onRetry: reload }}
            onEditBirth={() => onEdit(false)}
          />
        ) : result.kind === "astrology" ? (
          <SunSignResultView
            result={result.sun}
            onAddTime={() => onEdit(true)}
            chartHref={`/${locale}/calculators/natal-chart${natalFragmentFromSunSign(values as SunSignFormValues)}`}
          />
        ) : result.kind === "tarot-birth-card" ? (
          <TarotBirthResultView result={result.tarot} date={birthDateOf(values.birth)} />
        ) : (
          <LifePathResultView result={result.numbers} />
        )}
        {result && !sample ? <ShareResultDialog result={result} open={shareOpen} onOpenChange={setShareOpen} /> : null}
        {result && !sample && emailable ? (
          <ReportEmailDialog
            kind={kind}
            values={values}
            open={report.open}
            onOpenChange={report.setOpen}
            manual={report.manual}
            prechecked={report.prechecked}
            onSent={report.markSent}
          />
        ) : null}
      </div>
    </ShareResultProvider>
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
