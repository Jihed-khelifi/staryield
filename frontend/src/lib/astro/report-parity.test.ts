import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

import { encodeFragment } from "@/lib/astro/fragment";
import { lifePathSteps } from "@/lib/astro/report-steps";
import { lifePathReduction } from "@/lib/calculators";
import type { CalculatorKind } from "@/lib/schemas/calculator";
import { identityText } from "@/test/schema";

// revisions 6.0 (§26.4): the e-mailed report links back to the result ("View your
// report") and prints the life path's "How it adds up" lines. The Go side
// (backend/internal/mail) implements both again; this fixture is checked by both
// test suites, so the two implementations cannot drift.
const fixture = JSON.parse(
  readFileSync(new URL("../../../../backend/internal/mail/testdata/report_parity.json", import.meta.url), "utf8"),
) as {
  fragments: { kind: CalculatorKind; input: Record<string, string>; fragment: string }[];
  life_path_steps: { birth_date: string; steps: string[] }[];
};

function values(kind: CalculatorKind, input: Record<string, string>) {
  const [year, month, day] = input.birth_date.split("-");
  const time = input.birth_time ?? "";
  const place = input.place_id ? { id: input.place_id, label: "" } : null;
  if (kind === "natal-chart") return { birth: { day, month, year, time }, timeUnknown: !time, place, houseSystem: (input.house_system ?? "placidus") as never };
  if (kind === "astrology") return { birth: { day, month, year, time }, place };
  return { birth: { day, month, year, time: "" } };
}

describe("report parity fixture (shared with the Go renderer)", () => {
  it("is big enough to mean something", () => {
    expect(fixture.fragments.length).toBeGreaterThanOrEqual(10);
    expect(fixture.life_path_steps.length).toBeGreaterThanOrEqual(10);
  });

  it("the fragment that reopens a result is what the page writes", () => {
    for (const row of fixture.fragments) {
      expect(encodeFragment(row.kind, values(row.kind, row.input) as never), `${row.kind} ${JSON.stringify(row.input)}`).toBe(row.fragment);
    }
  });

  it("'How it adds up' prints the same lines", () => {
    for (const row of fixture.life_path_steps) {
      expect(lifePathSteps(lifePathReduction(row.birth_date), identityText), row.birth_date).toEqual(row.steps);
    }
  });
});
