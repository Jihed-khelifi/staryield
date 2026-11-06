/**
 * Contract checks for the in-browser Path of Life calculator
 * (src/lib/calculators.ts) against the fixtures the Go engine is tested with
 * (backend/internal/astro/testdata/numerology_vectors.json), so the two
 * implementations cannot drift. Astrology itself is computed by the API.
 */
import assert from "node:assert/strict";
import fs from "node:fs";
import vm from "node:vm";
import ts from "typescript";

const code = ts.transpileModule(fs.readFileSync("src/lib/calculators.ts", "utf8"), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText;
const sandbox = { exports: {}, Date };
vm.runInNewContext(code, sandbox);
const {
  lifePath,
  lifePathReduction,
  reduceNumber,
  reduceWithSteps,
  personalCycles,
  localDateIn,
  validBirthDate,
  minimumAdultDate,
  romanNumeral,
  formatDateParts,
} = sandbox.exports;

const fixtures = JSON.parse(
  fs.readFileSync(new URL("../../backend/internal/astro/testdata/numerology_vectors.json", import.meta.url), "utf8"),
);
assert.ok(Array.isArray(fixtures.vectors) && fixtures.vectors.length >= 20, "shared numerology vectors are missing");
assert.ok(Array.isArray(fixtures.timezone_vectors) && fixtures.timezone_vectors.length > 0, "time zone vectors are missing");

let checks = 0;
const equal = (actual, expected, message) => {
  assert.equal(actual, expected, message);
  checks++;
};

for (const v of fixtures.vectors) {
  const label = `${v.birth_date} as of ${v.as_of}`;
  equal(lifePath(v.birth_date), v.life_path, `${label}: life path`);
  equal(lifePathReduction(v.birth_date).master, v.life_path_master, `${label}: master flag`);
  const cycles = personalCycles(v.birth_date, v.as_of);
  equal(cycles.personalYear, v.personal_year, `${label}: personal year`);
  equal(cycles.personalMonth, v.personal_month, `${label}: personal month`);
  equal(cycles.personalDay, v.personal_day, `${label}: personal day`);
}

// "Today" is the calendar date in the visitor's own time zone.
for (const v of fixtures.timezone_vectors) {
  const asOf = localDateIn(new Date(v.instant), v.tz);
  equal(asOf, v.as_of, `${v.instant} in ${v.tz}`);
  const cycles = personalCycles(v.birth_date, asOf);
  equal(cycles.personalYear, v.personal_year, `${v.tz}: personal year`);
  equal(cycles.personalMonth, v.personal_month, `${v.tz}: personal month`);
  equal(cycles.personalDay, v.personal_day, `${v.tz}: personal day`);
}

// The article's worked example, step by step.
const worked = lifePathReduction("1990-11-29");
equal(worked.total, 23, "worked example total");
equal(worked.final.join(" "), "2+3=5", "worked example final reduction");
equal(worked.components.map((c) => c.reduced).join(","), "11,11,1", "worked example components");
equal(reduceWithSteps(1990).steps.join(" "), "1+9+9+0=19 1+9=10 1+0=1", "year steps");
equal(reduceNumber(33), 33, "33 is a master number");
equal(reduceNumber(33, false), 6, "personal cycles reduce masters");

// Birth-date validation shared by every calculator form.
const today = new Date("2026-10-01T12:00:00");
equal(validBirthDate(29, 2, 2024, today), true, "leap day");
equal(validBirthDate(29, 2, 2023, today), false, "not a leap year");
equal(validBirthDate(31, 4, 2000, today), false, "30-day month");
equal(validBirthDate(2, 10, 2026, today), false, "future date");
equal(validBirthDate(1, 10, 2026, today), true, "today");
equal(validBirthDate(1, 13, 2000, today), false, "month 13");
equal(validBirthDate(31, 12, 1899, today), false, "before 1900 (the API's lower bound)");
equal(minimumAdultDate(today), "2008-10-01", "adult cutoff");

// Formatting helpers.
equal(romanNumeral(4), "IV", "roman 4");
equal(romanNumeral(33), "XXXIII", "roman 33");
equal(formatDateParts("1945-04-30"), "30 / 04 / 1945", "date button");

console.log(`Calculator checks passed (${checks} assertions, ${fixtures.vectors.length + fixtures.timezone_vectors.length} shared vectors).`);
