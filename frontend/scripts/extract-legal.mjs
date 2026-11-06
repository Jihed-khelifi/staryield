#!/usr/bin/env node
/**
 * One-time migration tool for the legal pages.
 *
 * The Privacy Policy and Terms of Use used to be Figma exports: inline JSX in
 * which every sentence was wrapped in `text("English source")` and translated
 * through the flat copy dictionaries (`copy-{en,es,fr}.json`). This script
 * reads those `text()` calls in document order, classifies each one from its
 * JSX context (hero, table of contents, chapter heading, sub-heading, bullet,
 * paragraph, date line), looks up the Spanish and French values exactly like
 * `createTranslateCopy` does, and emits typed content modules:
 *
 *   src/content/legal/{privacy,terms}/{en,es,fr}.ts
 *
 * It verifies that every locale keeps the same number of words before and
 * after the migration (table-of-contents entries are regenerated from the
 * headings, so they are excluded from both counts), then removes the migrated
 * keys from the copy dictionaries — except keys still referenced elsewhere in
 * `src/` — and writes the removed list to `scripts/legal-removed-keys.json`.
 *
 * Usage:
 *   node scripts/extract-legal.mjs            # extract, verify, write, prune
 *   node scripts/extract-legal.mjs --dry-run  # extract + verify only
 *   node scripts/extract-legal.mjs --dump out.json  # also dump the raw model
 *
 * The exported helpers are pure so the migration can be unit-tested against a
 * small fixture (src/content/legal/extract-legal.test.ts).
 */
import { readFileSync, writeFileSync, readdirSync, mkdirSync } from "node:fs";
import { join, dirname, relative } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";

export const LOCALES = ["en", "es", "fr"];

/** Words are runs of letters/digits; bullets and punctuation do not count. */
export function wordCount(value) {
  return (String(value).match(/[\p{L}\p{N}]+/gu) ?? []).length;
}

const normalize = (value) => value.trim().replace(/\s+/g, " ");

/** Same lookup rules as src/i18n/translate-copy.ts. */
export function createLookup(copy) {
  const caseInsensitive = new Map(
    Object.keys(copy).map((key) => [key.toLowerCase(), key]),
  );
  return (value) => {
    const key = normalize(value);
    const sourceKey =
      copy[key] !== undefined ? key : caseInsensitive.get(key.toLowerCase());
    if (!sourceKey) return { key, value: undefined };
    let translated = copy[sourceKey];
    if (key === key.toUpperCase() && /[A-Z]/.test(key))
      translated = translated.toUpperCase();
    return { key: sourceKey, value: translated };
  };
}

export function slugify(value) {
  return value
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

function attributes(opening, sourceFile) {
  const result = {};
  for (const property of opening.attributes.properties) {
    if (!ts.isJsxAttribute(property) || !property.initializer) continue;
    const name = property.name.getText(sourceFile);
    const init = property.initializer;
    if (ts.isStringLiteral(init)) result[name] = init.text;
    else if (
      ts.isJsxExpression(init) &&
      init.expression &&
      (ts.isStringLiteral(init.expression) ||
        ts.isNoSubstitutionTemplateLiteral(init.expression) ||
        ts.isNumericLiteral(init.expression))
    )
      result[name] = init.expression.text;
  }
  return result;
}

/**
 * Collects every `text("…")` literal with the JSX element stack around it.
 * Returns [{ text, stack: [{ tag, attrs }] }] in document order.
 */
export function collectTexts(source, fileName = "legal.tsx") {
  const sourceFile = ts.createSourceFile(
    fileName,
    source,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  const entries = [];
  function visit(node, stack) {
    let next = stack;
    if (ts.isJsxElement(node) || ts.isJsxSelfClosingElement(node)) {
      const opening = ts.isJsxElement(node) ? node.openingElement : node;
      next = [
        ...stack,
        {
          tag: opening.tagName.getText(sourceFile),
          attrs: attributes(opening, sourceFile),
        },
      ];
    }
    if (
      ts.isCallExpression(node) &&
      node.expression.getText(sourceFile) === "text"
    ) {
      const argument = node.arguments[0];
      if (
        argument &&
        (ts.isStringLiteral(argument) ||
          ts.isNoSubstitutionTemplateLiteral(argument))
      )
        entries.push({ text: argument.text, stack: next });
    }
    ts.forEachChild(node, (child) => visit(child, next));
  }
  visit(sourceFile, []);
  return entries;
}

const HEADING_NUMBER = /^\s*(\d+(?:\.\d+)*)\.?\s+(.+?)\s*$/;
const BULLET = /^\s*•\s*/;
const INDENTED = /^ {3,}\S/;
const DATE_LINE = /^\s*(Effective as of|Last updated)\s*:\s*(.+?)\s*$/i;

function within(stack, predicate) {
  return stack.some((frame) => predicate(frame));
}
const dataName = (frame) => frame.attrs["data-name"] ?? "";
const classOf = (frame) => frame.attrs.className ?? "";

/** Classifies one collected entry. */
export function classify(entry) {
  const { text, stack } = entry;
  const nearest = stack[stack.length - 1] ?? { tag: "", attrs: {} };
  if (within(stack, (f) => /Table Of Contents/i.test(dataName(f))))
    return { kind: "toc" };
  if (within(stack, (f) => /Section \/ Hero/i.test(dataName(f)))) {
    return nearest.tag === "h1" ? { kind: "title" } : { kind: "badge" };
  }
  const date = text.match(DATE_LINE);
  if (date) return { kind: "date", label: date[1], value: date[2] };
  const h2 = [...stack].reverse().find((f) => f.tag === "h2");
  if (h2) {
    const match = text.match(HEADING_NUMBER);
    return {
      kind: "section",
      id: h2.attrs.id,
      n: match ? match[1] : "",
      title: match ? match[2] : normalize(text),
    };
  }
  if (BULLET.test(text) || INDENTED.test(text)) return { kind: "li" };
  const trimmed = normalize(text);
  const bold = /font-(bold|semibold)/.test(classOf(nearest));
  const numbered = trimmed.match(HEADING_NUMBER);
  if (
    numbered &&
    numbered[1].includes(".") &&
    trimmed.length <= 120 &&
    !/[.:;]$/.test(trimmed)
  ) {
    const depth = numbered[1].split(".").length;
    return {
      kind: depth >= 3 ? "h4" : "h3",
      n: numbered[1],
      title: numbered[2],
      bold,
    };
  }
  if (
    trimmed.length <= 80 &&
    trimmed === trimmed.toUpperCase() &&
    /[A-Z]/.test(trimmed) &&
    !/[.;]$/.test(trimmed)
  )
    return { kind: "h3", n: "", title: trimmed, bold: true };
  return { kind: "p" };
}

function stripHeadingNumber(value) {
  const match = normalize(value).match(HEADING_NUMBER);
  return match ? match[2] : normalize(value);
}
function stripBullet(value) {
  return normalize(value.replace(BULLET, ""));
}

/**
 * Builds the locale-independent skeleton plus per-locale strings.
 * `lookups` maps locale → lookup(value) (en returns the source itself).
 */
export function buildDocument(entries, lookups, { docId }) {
  const missing = [];
  const keys = new Set();
  const translate = (value) => {
    const result = {};
    for (const locale of LOCALES) {
      if (locale === "en") {
        result.en = value;
        continue;
      }
      const found = lookups[locale](value);
      if (found.value === undefined) {
        missing.push({ locale, value: normalize(value) });
        result[locale] = value;
      } else result[locale] = found.value;
    }
    return result;
  };
  const record = (value) => {
    const found = lookups.en(value);
    keys.add(found.key);
  };

  const doc = {
    id: docId,
    badge: null,
    title: null,
    tocTitle: null,
    date: null,
    intro: [],
    sections: [],
  };
  const usedIds = new Set();
  const uniqueId = (base) => {
    let id = base || "section";
    let suffix = 2;
    while (usedIds.has(id)) id = `${base}-${suffix++}`;
    usedIds.add(id);
    return id;
  };
  const target = () =>
    doc.sections.length ? doc.sections[doc.sections.length - 1].blocks : doc.intro;
  const tocStrings = [];

  for (const entry of entries) {
    const kind = classify(entry);
    record(entry.text);
    const strings = translate(entry.text);
    switch (kind.kind) {
      case "toc":
        if (/^table of contents$/i.test(normalize(entry.text)))
          doc.tocTitle = mapLocales(strings, normalize);
        tocStrings.push(normalize(entry.text));
        break;
      case "badge":
        doc.badge = mapLocales(strings, normalize);
        break;
      case "title":
        doc.title = mapLocales(strings, normalize);
        break;
      case "date": {
        doc.date = {
          label: kind.label,
          value: kind.value,
          text: mapLocales(strings, normalize),
        };
        break;
      }
      case "section": {
        const id = uniqueId(kind.id || slugify(kind.title));
        doc.sections.push({
          id,
          n: kind.n,
          title: mapLocales(strings, stripHeadingNumber),
          blocks: [],
        });
        break;
      }
      case "h3":
      case "h4": {
        const base = slugify(
          kind.n ? kind.title.replace(/^(\d+\.?)+\s*/, "") : kind.title,
        );
        const id = uniqueId(base);
        target().push({
          type: kind.kind,
          id,
          n: kind.n,
          title: mapLocales(strings, kind.n ? stripHeadingNumber : normalize),
        });
        break;
      }
      case "li": {
        const blocks = target();
        const last = blocks[blocks.length - 1];
        const item = mapLocales(strings, stripBullet);
        if (last && last.type === "ul") last.items.push(item);
        else blocks.push({ type: "ul", items: [item] });
        break;
      }
      default:
        target().push({ type: "p", text: mapLocales(strings, normalize) });
    }
  }
  return { doc, missing, keys: [...keys], tocStrings };
}

function mapLocales(strings, fn) {
  return Object.fromEntries(LOCALES.map((l) => [l, fn(strings[l])]));
}

/** Chrome labels for the rendered document (not part of the legal text). */
export const LABEL_DEFAULTS = {
  en: { toc: "Table of contents", effective: "Effective as of", lastUpdated: "Last updated", backToTop: "Back to top" },
  es: { toc: "Índice", effective: "Vigente desde", lastUpdated: "Última actualización", backToTop: "Volver arriba" },
  fr: { toc: "Table des matières", effective: "En vigueur au", lastUpdated: "Dernière mise à jour", backToTop: "Retour en haut" },
};

/** Projects the multi-locale model onto one locale's content object. */
export function projectLocale(doc, locale, meta) {
  const pick = (value) => (value ? value[locale] : "");
  const labels = { ...(LABEL_DEFAULTS[locale] ?? LABEL_DEFAULTS.en) };
  const block = (b) => {
    switch (b.type) {
      case "p":
        return { type: "p", text: pick(b.text) };
      case "ul":
        return { type: "ul", items: b.items.map(pick) };
      case "h3":
      case "h4":
        return {
          type: b.type,
          id: b.id,
          ...(b.n ? { n: b.n } : {}),
          title: pick(b.title),
        };
      default:
        return b;
    }
  };
  return {
    meta: { ...meta, badge: pick(doc.badge), title: pick(doc.title) },
    labels,
    intro: doc.intro.map(block),
    sections: doc.sections.map((s) => ({
      id: s.id,
      n: s.n,
      title: pick(s.title),
      blocks: s.blocks.map(block),
    })),
  };
}

/** Words in everything a reader sees (excluding the regenerated TOC). */
export function contentWords(content) {
  let total = wordCount(content.meta.badge) + wordCount(content.meta.title);
  const block = (b) => {
    if (b.type === "p") return wordCount(b.text);
    if (b.type === "ul" || b.type === "ol")
      return b.items.reduce((sum, item) => sum + wordCount(item), 0);
    if (b.type === "h3" || b.type === "h4")
      return wordCount(b.n ?? "") + wordCount(b.title);
    return 0;
  };
  for (const b of content.intro) total += block(b);
  for (const s of content.sections) {
    total += wordCount(s.n) + wordCount(s.title);
    for (const b of s.blocks) total += block(b);
  }
  return total;
}

export function sourceWords(entries, lookup) {
  return entries
    .filter((entry) => classify(entry).kind !== "toc")
    .filter((entry) => classify(entry).kind !== "date")
    .reduce((sum, entry) => sum + wordCount(lookup(entry.text)), 0);
}

export function toModule(content, typeName = "LegalDocumentContent") {
  return `// Generated by scripts/extract-legal.mjs, then edited by hand.\n// Keep section and block ids identical across locales (npm run test:content).\nimport type { ${typeName} } from "@/content/legal/types";\n\nconst content: ${typeName} = ${JSON.stringify(content, null, 2)};\n\nexport default content;\n`;
}

/** String literals used anywhere else in src (keys that must survive). */
export function referencedElsewhere(files, keys) {
  const wanted = new Map(keys.map((key) => [key.toLowerCase(), key]));
  const keep = new Set();
  for (const file of files) {
    const sourceFile = ts.createSourceFile(
      file,
      readFileSync(file, "utf8"),
      ts.ScriptTarget.Latest,
      true,
    );
    const visit = (node) => {
      if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
        const match = wanted.get(normalize(node.text).toLowerCase());
        if (match) keep.add(match);
      }
      ts.forEachChild(node, visit);
    };
    visit(sourceFile);
  }
  return keep;
}

function listFiles(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) =>
    entry.isDirectory()
      ? listFiles(join(dir, entry.name))
      : [join(dir, entry.name)],
  );
}

async function main() {
  const root = join(dirname(fileURLToPath(import.meta.url)), "..");
  const args = process.argv.slice(2);
  const dryRun = args.includes("--dry-run");
  // --no-write: verify and prune the copy dictionaries without regenerating
  // the content modules (they are edited by hand after the first run).
  const noWrite = args.includes("--no-write");
  const dumpIndex = args.indexOf("--dump");
  const dumpPath = dumpIndex >= 0 ? args[dumpIndex + 1] : null;
  const dictDir = join(root, "src/i18n/dictionaries");
  const copies = Object.fromEntries(
    LOCALES.map((l) => [
      l,
      JSON.parse(readFileSync(join(dictDir, `copy-${l}.json`), "utf8")),
    ]),
  );
  const lookups = Object.fromEntries(
    LOCALES.map((l) => [l, createLookup(copies[l])]),
  );
  const docs = {
    privacy: {
      file: "src/components/content/privacy.tsx",
      meta: {
        version: "2026-11-04",
        effective: "2026-11-04",
        lastUpdated: "2026-11-04",
      },
    },
    terms: {
      file: "src/components/content/terms.tsx",
      meta: {
        version: "2026-11-04",
        effective: "2026-11-04",
        lastUpdated: "2026-11-04",
      },
    },
  };
  const allKeys = new Set();
  const dump = {};
  let failed = false;
  for (const [docId, { file, meta }] of Object.entries(docs)) {
    const entries = collectTexts(readFileSync(join(root, file), "utf8"), file);
    const built = buildDocument(entries, lookups, { docId });
    dump[docId] = built.doc;
    built.keys.forEach((key) => allKeys.add(key));
    if (built.missing.length) {
      console.error(`${docId}: ${built.missing.length} strings have no translation`);
      for (const m of built.missing.slice(0, 10)) console.error(`  ${m.locale}: ${m.value}`);
      failed = true;
    }
    for (const locale of LOCALES) {
      const content = projectLocale(built.doc, locale, meta);
      const lookup = (value) =>
        locale === "en" ? value : lookups[locale](value).value ?? value;
      const before = sourceWords(entries, lookup);
      const after = contentWords(content);
      const status = before === after ? "ok" : "MISMATCH";
      console.log(
        `${docId}/${locale}: ${built.doc.sections.length} sections, words before=${before} after=${after} ${status}`,
      );
      if (before !== after) failed = true;
      if (!dryRun && !noWrite && !failed) {
        const out = join(root, `src/content/legal/${docId}/${locale}.ts`);
        mkdirSync(dirname(out), { recursive: true });
        writeFileSync(out, toModule(content));
      }
    }
  }
  if (dumpPath) writeFileSync(dumpPath, JSON.stringify(dump, null, 2));
  if (failed) {
    console.error("Extraction verification failed; nothing was pruned.");
    process.exit(1);
  }
  if (dryRun) return;
  const others = listFiles(join(root, "src")).filter(
    (f) =>
      /\.tsx?$/.test(f) &&
      !/\/content\/(privacy|terms)\.tsx$/.test(f) &&
      !/\/content\/legal\//.test(f),
  );
  const keep = referencedElsewhere(others, [...allKeys]);
  const removed = [...allKeys].filter((key) => !keep.has(key)).sort();
  for (const locale of LOCALES) {
    const path = join(dictDir, `copy-${locale}.json`);
    const copy = copies[locale];
    for (const key of removed) delete copy[key];
    writeFileSync(path, `${JSON.stringify(copy, null, 2)}\n`);
  }
  writeFileSync(
    join(root, "scripts/legal-removed-keys.json"),
    `${JSON.stringify({ generatedBy: "scripts/extract-legal.mjs", count: removed.length, keys: removed }, null, 2)}\n`,
  );
  console.log(
    `Removed ${removed.length} migrated keys from copy-{en,es,fr}.json (kept ${keep.size} still referenced elsewhere: ${[...keep].join(" | ")}).`,
  );
  console.log(`Wrote ${relative(root, join(root, "scripts/legal-removed-keys.json"))}`);
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  await main();
}
