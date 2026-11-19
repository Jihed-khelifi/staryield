/**
 * Generates backend/internal/mail/reportcopy/report-copy.json (revisions 6.0,
 * §26.4): the copy of the calculators' result views that the API prints in the
 * e-mailed report, in English with its French and Spanish translations.
 *
 *   npm run gen:report-copy
 *
 * Sources (English, in the web app):
 *   src/lib/astro/zodiac.ts          SIGNS, SIGN_FACTS, SIGN_INSIGHT, TAROT_CARDS, HOUSE_KEYWORDS, houseLabel
 *   src/lib/astro/life-path-copy.ts  LIFE_PATH_COPY, NUMBER_CARDS
 *   src/lib/astro/report-labels.ts   the headings and sentence templates the result views render
 * Translations: src/i18n/dictionaries/copy-fr.json and copy-es.json, looked up
 * exactly like the page's `text()` does (trimmed, whitespace collapsed, case
 * insensitive). A string without a translation fails the run: an e-mail must
 * never fall back to English silently.
 *
 * How the TypeScript sources are read: they are transpiled one by one with the
 * TypeScript compiler (already a dev dependency; check-i18n.mjs does the same)
 * and imported as data URLs, so no build step or test runner is needed. That
 * only works while those files stay free of runtime imports (type imports are
 * erased) — the generator fails with a clear message when one gains an import.
 *
 * The output has a stable key order and a trailing newline.
 * `src/lib/astro/report-copy.test.ts` re-runs `buildReportCopy()` in memory and
 * fails when the committed file differs (the pattern of company.ts/company.go).
 */
import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const here = (path) => new URL(path, import.meta.url);
const OUTPUT = "../../backend/internal/mail/reportcopy/report-copy.json";
const LOCALES = ["fr", "es"];

/** Import a TypeScript source file that has no runtime imports. */
async function loadTypeScript(path) {
  const source = readFileSync(here(path), "utf8");
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
  });
  if (/^\s*(import|export)\s[^;]*\sfrom\s/m.test(outputText.replace(/^export\s/gm, ""))) {
    throw new Error(`${path} has a runtime import: gen-report-copy.mjs reads it without a bundler, keep it import-free`);
  }
  return import(`data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`);
}

const normalize = (value) => value.trim().replace(/\s+/g, " ");

function dictionary(locale) {
  const copy = JSON.parse(readFileSync(here(`../src/i18n/dictionaries/copy-${locale}.json`), "utf8"));
  const lower = new Map(Object.keys(copy).map((key) => [key.toLowerCase(), key]));
  return (english) => {
    const key = normalize(english);
    const source = copy[key] !== undefined ? key : lower.get(key.toLowerCase());
    return source === undefined ? undefined : copy[source];
  };
}

function sorted(object) {
  return Object.fromEntries(Object.entries(object).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)));
}

export async function buildReportCopy() {
  const zodiac = await loadTypeScript("../src/lib/astro/zodiac.ts");
  const lifePath = await loadTypeScript("../src/lib/astro/life-path-copy.ts");
  const labels = await loadTypeScript("../src/lib/astro/report-labels.ts");

  const signs = zodiac.SIGNS.map((name) => ({
    name,
    element: zodiac.SIGN_FACTS[name].element,
    ruler: zodiac.SIGN_FACTS[name].ruler,
    insight: zodiac.SIGN_INSIGHT[name],
  }));
  const houses = zodiac.HOUSE_KEYWORDS.map((keyword, index) => ({ label: zodiac.houseLabel(index + 1), keyword }));
  const lifePaths = Object.fromEntries(
    Object.entries(lifePath.LIFE_PATH_COPY)
      .sort(([a], [b]) => Number(a) - Number(b))
      .map(([number, copy]) => [number, { title: copy.title, body: copy.body }]),
  );
  const numbers = lifePath.NUMBER_CARDS.map(({ key, label, body }) => ({ key, label, body }));

  const copy = {
    version: 1,
    labels: { ...labels.REPORT_LABELS },
    signs,
    tarot: { ...zodiac.TAROT_CARDS },
    houses,
    life_path: lifePaths,
    numbers,
    house_systems: { ...labels.HOUSE_SYSTEM_NAMES },
    bodies: [...labels.CARD_BODIES],
    elements: [...labels.ELEMENTS],
    modalities: [...labels.MODALITIES],
  };

  // Every English string the renderer may translate.
  const english = new Set();
  const add = (value) => value && english.add(normalize(value));
  Object.values(copy.labels).forEach(add);
  signs.forEach((s) => [s.name, s.element, s.ruler, s.insight].forEach(add));
  Object.values(copy.tarot).forEach(add);
  houses.forEach((h) => [h.label, h.keyword].forEach(add));
  Object.values(lifePaths).forEach((c) => [c.title, c.body].forEach(add));
  numbers.forEach((n) => [n.label, n.body].forEach(add));
  Object.values(copy.house_systems).forEach(add);
  [...copy.bodies, ...copy.elements, ...copy.modalities].forEach(add);

  const translations = {};
  const missing = [];
  for (const locale of LOCALES) {
    const translate = dictionary(locale);
    translations[locale] = {};
    for (const value of [...english].sort()) {
      const translated = translate(value);
      if (translated === undefined || !translated.trim()) missing.push(`${locale}: ${value}`);
      else translations[locale][value] = translated;
    }
    translations[locale] = sorted(translations[locale]);
  }
  if (missing.length) {
    throw new Error(`report copy without a translation (add it to src/i18n/dictionaries/copy-*.json):\n${missing.join("\n")}`);
  }
  return `${JSON.stringify({ ...copy, translations }, null, 2)}\n`;
}

/** Path of the generated file (relative to this script), for tests. */
export const REPORT_COPY_PATH = OUTPUT;

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  const json = await buildReportCopy();
  writeFileSync(here(OUTPUT), json);
  console.log(`wrote ${fileURLToPath(here(OUTPUT))} (${json.length} bytes)`);
}
