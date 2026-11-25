"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import type { NatalChart } from "@/lib/api/astrology";

const ELEMENTS = ["Fire", "Earth", "Air", "Water"] as const;
const MODALITIES = ["Cardinal", "Fixed", "Mutable"] as const;

function Group({ title, names, counts }: { title: string; names: readonly string[]; counts: Record<string, number> }) {
  const { text } = useI18n();
  const total = names.reduce((sum, name) => sum + Math.max(counts[name] ?? 0, 0), 0);
  return (
    <div>
      <h4 className="mb-2 text-xs font-semibold tracking-wide uppercase">{text(title)}</h4>
      <ul className="grid gap-2">
        {names.map((name) => {
          const value = Math.max(counts[name] ?? 0, 0);
          return (
            <li key={name} className="grid grid-cols-[5.5rem_1fr_1.5rem] items-center gap-3 text-sm" data-balance={name} data-count={value}>
              <span>{text(name)}</span>
              <span aria-hidden="true" className="h-2 overflow-hidden rounded-full bg-[#c8c0a8]/50">
                <span className="block h-full rounded-full bg-[#aa8e62]" style={{ width: total ? `${Math.round((value / total) * 100)}%` : "0%" }} />
              </span>
              <span className="text-right font-semibold tabular-nums">{value}</span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/**
 * Element and modality balance: how many of the chart's points sit in fire,
 * earth, air and water signs and in cardinal, fixed and mutable signs (the
 * API's `elements` and `modalities`). Rendered as a list with
 * counts; the bars are decoration.
 */
export function ChartBalance({ chart }: { chart: Pick<NatalChart, "elements" | "modalities"> }) {
  const { text } = useI18n();
  const headingId = React.useId();
  if (!chart.elements || !chart.modalities) return null;
  return (
    <section aria-labelledby={headingId} className="rounded-lg border border-gold/50 px-4 py-4" data-testid="chart-balance">
      <h3 id={headingId} className="text-sm font-semibold">
        {text("Elements and modalities")}
      </h3>
      <p className="mt-1 text-xs text-[var(--muted-ink)]">{text("Where your planets gather: the temperament of your chart.")}</p>
      <div className="mt-4 grid gap-6 sm:grid-cols-2">
        <Group title="By element" names={ELEMENTS} counts={chart.elements} />
        <Group title="By modality" names={MODALITIES} counts={chart.modalities} />
      </div>
    </section>
  );
}
