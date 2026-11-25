"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import type { ApiError } from "@/lib/api/errors";
import { ErrorState } from "@/components/feedback/error-state";
import { Skeleton } from "@/components/ui/skeleton";

export type NumbersSummaryValues = { lifePath: number; personalYear: number; personalMonth: number };

/**
 * "Your numbers": life path, personal year and personal month. The signed-in
 * chart gets them from the API, the public calculator computes them in the
 * browser; the markup is the same. `numbers` null shows a skeleton.
 */
export function NumbersSummary({
  numbers,
  error,
  onRetry,
  className,
}: {
  numbers: NumbersSummaryValues | null;
  /** The numbers could not be loaded (signed-in chart): an inline error with a retry. */
  error?: ApiError | null;
  onRetry?: () => void;
  className?: string;
}) {
  const { text } = useI18n();
  const headingId = React.useId();
  return (
    <section aria-labelledby={headingId} className={className ?? "rounded-lg border border-gold/50 px-4 py-4"}>
      <h3 id={headingId} className="text-sm font-semibold">
        {text("Your numbers")}
      </h3>
      {error ? (
        <ErrorState error={error} onRetry={onRetry} compact />
      ) : numbers ? (
        <dl className="mt-3 grid grid-cols-3 gap-3 text-center" data-testid="my-numbers">
          {(
            [
              ["Life Path", numbers.lifePath],
              ["Personal year", numbers.personalYear],
              ["Personal month", numbers.personalMonth],
            ] as const
          ).map(([label, value]) => (
            <div key={label} className="flex flex-col gap-1">
              <dt className="text-[10px] font-semibold tracking-wide uppercase">{text(label)}</dt>
              <dd className="font-display text-2xl text-[#8a6a1f]">{value}</dd>
            </div>
          ))}
        </dl>
      ) : (
        <Skeleton className="mt-3 h-12 w-full" />
      )}
    </section>
  );
}
