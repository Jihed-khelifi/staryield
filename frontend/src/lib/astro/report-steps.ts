import type { TranslateCopy } from "@/i18n/translate-copy";
import type { Reduction } from "@/lib/calculators";

/** The last value of each digit-sum step: "1+9+9+0=19" → "19". */
const result = (step: string) => step.split("=").at(-1) ?? step;

/**
 * The lines of "How it adds up" under a life path: the month, the day and the
 * year each reduced, then the total ("Month: 11", "Day: 29 → 11", "Year: 1990 → 1", "Total: 23 → 5").
 * The API's e-mailed report prints the same lines (backend service
 * `lifePathSteps`; both are checked against report_parity.json).
 */
export function lifePathSteps(reduction: Pick<Reduction, "components" | "total" | "final">, text: TranslateCopy): string[] {
  return [
    ...reduction.components.map((c) =>
      text("{value0}: {value1}", {
        value0: text(c.label === "month" ? "Month" : c.label === "day" ? "Day" : "Year"),
        value1: [String(c.value), ...c.steps.map(result)].join(" → "),
      }),
    ),
    text("Total: {value0}", { value0: [String(reduction.total), ...reduction.final.map(result)].join(" → ") }),
  ];
}
