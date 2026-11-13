#!/usr/bin/env node
/**
 * npm run test:content — validates the structured content modules:
 *   src/content/legal/{privacy,terms,refund}/{en,es,fr}.ts
 *   src/content/calculators/{natal,life-path,astrology}/{en,es,fr}.ts
 *
 * Every locale must mirror English (sections, ids, block types, list
 * lengths, link targets, bold spans, numbers, images, legal version/dates),
 * links must be internal/anchor/mailto and point at existing anchors, ids
 * must be unique, and article images must be registered R2 assets.
 * The rules live in src/content/validate.ts (shared with vitest).
 */
import { existsSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const src = join(root, "src");
const cache = new Map();

function resolveSpecifier(specifier, fromFile) {
  let base;
  if (specifier.startsWith("@/")) base = join(src, specifier.slice(2));
  else if (specifier.startsWith(".")) base = resolve(dirname(fromFile), specifier);
  else return null;
  for (const candidate of [base, `${base}.ts`, `${base}.tsx`, join(base, "index.ts")])
    if (existsSync(candidate) && candidate.match(/\.tsx?$/)) return candidate;
  throw new Error(`Cannot resolve ${specifier} from ${fromFile}`);
}

/** Transpiles a TS module (and its local imports) to a data: URL. */
function moduleUrl(file) {
  if (cache.has(file)) return cache.get(file);
  let { outputText } = ts.transpileModule(readFileSync(file, "utf8"), {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
    fileName: file,
  });
  for (const [, quote, specifier] of outputText.matchAll(/from\s+(["'])([^"']+)\1/g)) {
    const resolved = resolveSpecifier(specifier, file);
    if (!resolved) continue;
    outputText = outputText.split(`${quote}${specifier}${quote}`).join(`"${moduleUrl(resolved)}"`);
  }
  const url = `data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`;
  cache.set(file, url);
  return url;
}

const load = async (relativePath) => (await import(moduleUrl(join(src, relativePath)))).default;
const { checkLegalDocument, checkArticle, legalAnchorSet } = await import(
  moduleUrl(join(src, "content/validate.ts"))
);

const locales = ["en", "es", "fr"];
const loadAll = async (folder) =>
  Object.fromEntries(
    await Promise.all(locales.map(async (l) => [l, await load(`${folder}/${l}.ts`)])),
  );

const legal = {
  privacy: await loadAll("content/legal/privacy"),
  terms: await loadAll("content/legal/terms"),
  refund: await loadAll("content/legal/refund"), // revisions 5.0
};
const anchorsByPath = {
  "/privacy": legalAnchorSet(legal.privacy.en),
  "/terms": legalAnchorSet(legal.terms.en),
  "/refund-policy": legalAnchorSet(legal.refund.en), // revisions 5.0
};
const registry = new Set(
  JSON.parse(readFileSync(join(src, "data/r2-images.json"), "utf8")).map((e) => e.url),
);

const issues = [];
let sections = 0;
for (const [name, docs] of Object.entries(legal)) {
  issues.push(...checkLegalDocument(name, docs, anchorsByPath));
  sections += docs.en.sections.length;
}
const articles = {};
for (const kind of ["natal", "life-path", "astrology"]) {
  articles[kind] = await loadAll(`content/calculators/${kind}`);
  issues.push(...checkArticle(kind, articles[kind], registry));
}

if (issues.length) {
  console.error(`Content checks failed (${issues.length}):`);
  for (const issue of issues.slice(0, 80)) console.error(`  - ${issue}`);
  process.exit(1);
}
console.log(
  `Content checks passed: ${Object.keys(legal).length} legal documents (${sections} sections) and ${Object.keys(articles).length} calculator articles in ${locales.length} locales.`,
);
