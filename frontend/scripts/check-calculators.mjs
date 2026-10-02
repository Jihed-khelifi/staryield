import assert from "node:assert/strict";
import fs from "node:fs";
import vm from "node:vm";
import ts from "typescript";
const code = ts.transpileModule(
  fs.readFileSync("src/lib/calculators.ts", "utf8"),
  {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  },
).outputText;
const sandbox = { exports: {}, Date };
vm.runInNewContext(code, sandbox);
const {
  lifePath,
  reduceNumber,
  zodiacForDate,
  validBirthDate,
  minimumAdultDate,
  personalCycles,
} = sandbox.exports;
assert.equal(lifePath("1990-01-01"), 3);
assert.equal(lifePath("1980-01-01"), 11);
assert.equal(lifePath("2000-11-29"), 6);
assert.equal(lifePath("1945-04-30"), 8);
assert.equal(reduceNumber(33), 33);
assert.equal(reduceNumber(33, false), 6);
assert.equal(zodiacForDate(12, 31), "Capricorn");
assert.equal(zodiacForDate(1, 19), "Capricorn");
assert.equal(zodiacForDate(1, 20), "Aquarius");
assert.equal(zodiacForDate(4, 30), "Taurus");
const today = new Date("2026-10-01T12:00:00");
assert.equal(validBirthDate(29, 2, 2024, today), true);
assert.equal(validBirthDate(29, 2, 2023, today), false);
assert.equal(validBirthDate(31, 4, 2000, today), false);
assert.equal(validBirthDate(2, 10, 2026, today), false);
assert.equal(validBirthDate(1, 10, 2026, today), true);
assert.equal(validBirthDate(1, 13, 2000, today), false);
assert.equal(minimumAdultDate(today), "2008-10-01");
const cycles = personalCycles("1945-04-30", today);
assert.equal(cycles.personalYear, 8);
assert.equal(cycles.personalMonth, 9);
console.log("Calculator checks passed (19 assertions).");
