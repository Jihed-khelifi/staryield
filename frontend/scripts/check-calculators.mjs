/**
 * Contract checks for the in-browser Path of Life calculator
 * (src/lib/calculators.ts) against the fixtures the Go engine is tested with
 * (backend/internal/astro/testdata/numerology_vectors.json), so the two
 * implementations cannot drift, and for the in-browser tarot birth cards.
 * Astrology itself is computed by the API.
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
  tarotBirthCards,
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

// Tarot birth cards (Mary K. Greer): month + day + year, digit sums down to
// 22 or less (the personality card), then down to one digit (the soul card).
const tarot = (date) => tarotBirthCards(date);
equal(tarot("1990-11-29").total, 2030, "tarot worked example total");
equal(tarot("1990-11-29").steps.join(" "), "2+0+3+0=5", "tarot worked example steps");
equal(tarot("1990-11-29").cards.join("/"), "5", "tarot single card");
equal(tarot("1987-06-15").cards.join("/"), "10/1", "tarot two cards (the article's example)");
equal(tarot("1930-05-10").cards.join("/"), "19/10/1", "tarot three cards");
equal(tarot("1940-07-10").cards.join("/"), "22/4", "The Fool pairs with The Emperor");
equal(tarot("1985-07-04").steps.join(" "), "1+9+9+6=25 2+5=7", "a total above 22 is reduced again");
equal(tarot("1998-01-01").cards.join("/"), "2", "the smallest birth number");
equal(tarot("1990-2-3"), null, "malformed date (calendar validity is checked by the form schema)");
// Every valid date from 1900 to the end of 2030: a personality card from 2 to
// 22, a single-digit soul card, and each step a true digit sum.
let tarotDates = 0;
for (let day = new Date(Date.UTC(1900, 0, 1)); day.getUTCFullYear() <= 2030; day.setUTCDate(day.getUTCDate() + 1)) {
  const iso = day.toISOString().slice(0, 10);
  const r = tarot(iso);
  assert.ok(r && r.cards[0] >= 2 && r.cards[0] <= 22, `${iso}: personality card ${r?.cards[0]}`);
  assert.ok(r.cards.at(-1) >= 1 && r.cards.at(-1) <= 9, `${iso}: soul card ${r.cards.at(-1)}`);
  for (const step of r.steps) {
    const [digits, sum] = step.split("=");
    assert.equal(digits.split("+").reduce((t, d) => t + Number(d), 0), Number(sum), `${iso}: ${step}`);
  }
  tarotDates++;
}
checks++;

console.log(`Calculator checks passed (${checks} assertions, ${fixtures.vectors.length + fixtures.timezone_vectors.length} shared vectors, ${tarotDates} tarot birth dates).`);
