/**
 * Structural checks for the structured content modules (legal documents and
 * calculator articles). Used by scripts/check-content.mjs (npm run
 * test:content) and by the vitest suite. Pure: imports types only.
 *
 * Every locale must mirror English: same sections, ids, block types, list
 * lengths, link targets, bold spans, numbers and images, and the same legal
 * version and dates. A translation that was not updated after an English
 * edit usually breaks one of these (for example a changed retention period
 * changes the numbers), so the check fails on stale translations.
 */
import type {
  LegalBlock,
  LegalDocumentContent,
} from "@/content/legal/types";
import type { ArticleBlock, ArticleContent } from "@/content/calculators/types";

export type Locales<T> = Record<string, T>;

const LINK = /\[([^\]]+)\]\(([^)\s]+)\)/g;
const BOLD = /\*\*([^*]+)\*\*/g;
const ALLOWED_HREF = /^(#[a-z0-9-]+|\/[a-zA-Z0-9/_?=&#.-]*|mailto:[^\s@]+@[^\s@]+)$/;
const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/;

export function linkTargets(text: string): string[] {
  return [...text.matchAll(LINK)].map((match) => match[2]);
}

function boldCount(text: string): number {
  return [...text.matchAll(BOLD)].length;
}

const ROMAN: Record<string, number> = { I: 1, V: 5, X: 10, L: 50, C: 100 };
function romanToInt(value: string): number {
  let total = 0;
  for (let i = 0; i < value.length; i++) {
    const current = ROMAN[value[i]];
    const next = ROMAN[value[i + 1]] ?? 0;
    total += current < next ? -current : current;
  }
  return total;
}

/**
 * Centuries are written in Roman numerals in Spanish and French ("siglo
 * XVIII", "XVIIIᵉ siècle") but in digits in English ("18th-century"), so
 * they are normalised before comparing numbers.
 */
function normaliseCenturies(text: string): string {
  return text
    .replace(/\b(siglos?)\s+([IVXLC]+)\b/g, (_, word: string, roman: string) => `${word} ${romanToInt(roman)}`)
    .replace(/\b([IVXLC]+)(?:ᵉ|e|ème)\s+(siècles?)/g, (_, roman: string, word: string) => `${romanToInt(roman)} ${word}`);
}

/** Digits found in a string (decimal separators normalised, sorted). */
export function numberTokens(text: string): string[] {
  const withoutLinks = normaliseCenturies(text.replace(LINK, (_, label: string) => label));
  return (withoutLinks.match(/\d+(?:[.,]\d+)*/g) ?? [])
    .map((token) => token.replace(/[.,]/g, ""))
    .sort();
}

type Entry = { path: string; text: string };

/** Flattens a legal document into addressable strings + a structure signature. */
function legalEntries(doc: LegalDocumentContent) {
  const entries: Entry[] = [];
  const signature: string[] = [];
  const ids: string[] = [];
  const blocks = (list: LegalBlock[], path: string) =>
    list.forEach((block, index) => {
      const at = `${path}[${index}]`;
      switch (block.type) {
        case "p":
          signature.push(`${at}:p`);
          entries.push({ path: at, text: block.text });
          break;
        case "ul":
        case "ol":
          signature.push(`${at}:${block.type}:${block.items.length}`);
          block.items.forEach((item, i) => entries.push({ path: `${at}.${i}`, text: item }));
          break;
        case "h3":
        case "h4":
          signature.push(`${at}:${block.type}#${block.id}:${block.n ?? ""}`);
          ids.push(block.id);
          entries.push({ path: at, text: block.title });
          break;
        case "table":
          signature.push(
            `${at}:table:${block.head.length}x${block.rows.length}:${block.rows.map((r) => r.length).join(",")}`,
          );
          block.head.forEach((cell, i) => entries.push({ path: `${at}.head${i}`, text: cell }));
          block.rows.forEach((row, r) =>
            row.forEach((cell, c) => entries.push({ path: `${at}.r${r}c${c}`, text: cell })),
          );
          break;
      }
    });
  entries.push({ path: "meta.badge", text: doc.meta.badge });
  entries.push({ path: "meta.title", text: doc.meta.title });
  for (const [key, value] of Object.entries(doc.labels))
    entries.push({ path: `labels.${key}`, text: value });
  blocks(doc.intro, "intro");
  doc.sections.forEach((section, index) => {
    signature.push(`section[${index}]#${section.id}:${section.n}`);
    ids.push(section.id);
    entries.push({ path: `${section.id}.title`, text: section.title });
    blocks(section.blocks, section.id);
  });
  return { entries, signature, ids };
}

function articleEntries(article: ArticleContent) {
  const entries: Entry[] = [];
  const signature: string[] = [];
  const ids: string[] = [];
  const images: string[] = [];
  const blocks = (list: ArticleBlock[], path: string) =>
    list.forEach((block, index) => {
      const at = `${path}[${index}]`;
      switch (block.type) {
        case "p":
          signature.push(`${at}:p:${block.weight ?? ""}`);
          entries.push({ path: at, text: block.text });
          break;
        case "h2":
        case "h3":
          signature.push(`${at}:${block.type}#${block.id}`);
          ids.push(block.id);
          entries.push({ path: at, text: block.text });
          break;
        case "ul":
        case "ol":
          signature.push(`${at}:${block.type}:${block.items.length}`);
          block.items.forEach((item, i) => entries.push({ path: `${at}.${i}`, text: item }));
          break;
        case "split":
          signature.push(`${at}:split:${block.image.src}`);
          images.push(block.image.src);
          entries.push({ path: `${at}.alt`, text: block.image.alt });
          blocks(block.blocks, `${at}.split`);
          break;
        case "image":
          signature.push(`${at}:image:${block.image.src}`);
          images.push(block.image.src);
          entries.push({ path: `${at}.alt`, text: block.image.alt });
          break;
        case "timeline":
          signature.push(`${at}:timeline:${block.items.length}`);
          entries.push({ path: `${at}.label`, text: block.label });
          block.items.forEach((item, i) => {
            entries.push({ path: `${at}.${i}.title`, text: item.title });
            if (item.period) entries.push({ path: `${at}.${i}.period`, text: item.period });
          });
          break;
        case "twoColumnList":
          signature.push(
            `${at}:twoColumnList:${block.items.map((i) => i.body.length).join(",")}`,
          );
          block.items.forEach((item, i) => {
            entries.push({ path: `${at}.${i}.title`, text: item.title });
            entries.push({ path: `${at}.${i}.subtitle`, text: item.subtitle });
            item.body.forEach((line, l) => entries.push({ path: `${at}.${i}.body${l}`, text: line }));
          });
          break;
        case "faq":
          signature.push(`${at}:faq:${block.items.length}`);
          block.items.forEach((item, i) => {
            entries.push({ path: `${at}.${i}.q`, text: item.q });
            entries.push({ path: `${at}.${i}.a`, text: item.a });
          });
          break;
        case "formula":
          signature.push(`${at}:formula:${block.formula}`);
          entries.push({ path: `${at}.label`, text: block.label });
          break;
      }
    });
  entries.push({ path: "label", text: article.label });
  article.sections.forEach((section, index) => {
    signature.push(`section[${index}]#${section.id}`);
    blocks(section.blocks, section.id);
  });
  return { entries, signature, ids, images };
}

function duplicates(values: string[]): string[] {
  const seen = new Set<string>();
  const dupes = new Set<string>();
  for (const value of values) (seen.has(value) ? dupes : seen).add(value);
  return [...dupes];
}

function compareEntries(
  name: string,
  locale: string,
  source: Entry[],
  target: Entry[],
  issues: string[],
) {
  // Keyed by path (section id + position) so a structural change in one
  // section does not hide stale numbers or links elsewhere.
  const byPath = new Map(target.map((entry) => [entry.path, entry]));
  const expected = new Set(source.map((entry) => entry.path));
  for (const entry of target)
    if (!expected.has(entry.path)) issues.push(`${name}/${locale}: unexpected text at ${entry.path}`);
  source.forEach((entry) => {
    const other = byPath.get(entry.path);
    if (!other) {
      issues.push(`${name}/${locale}: missing text at ${entry.path}`);
      return;
    }
    if (!other.text.trim()) issues.push(`${name}/${locale}: empty text at ${other.path}`);
    if (linkTargets(other.text).join(" ") !== linkTargets(entry.text).join(" "))
      issues.push(`${name}/${locale}: link targets differ at ${other.path}`);
    if (boldCount(other.text) !== boldCount(entry.text))
      issues.push(`${name}/${locale}: bold spans differ at ${other.path}`);
    if (numberTokens(other.text).join(" ") !== numberTokens(entry.text).join(" "))
      issues.push(
        `${name}/${locale}: numbers differ at ${other.path} (${numberTokens(entry.text).join(" ")} vs ${numberTokens(other.text).join(" ")}) — stale translation?`,
      );
  });
}

function checkLinks(
  name: string,
  locale: string,
  entries: Entry[],
  ownIds: Set<string>,
  anchorsByPath: Record<string, Set<string>>,
  issues: string[],
) {
  for (const entry of entries)
    for (const href of linkTargets(entry.text)) {
      if (!ALLOWED_HREF.test(href)) {
        issues.push(`${name}/${locale}: disallowed link ${href} at ${entry.path}`);
        continue;
      }
      if (href.startsWith("#")) {
        if (!ownIds.has(href.slice(1)))
          issues.push(`${name}/${locale}: broken anchor ${href} at ${entry.path}`);
        continue;
      }
      // resolveHref adds the reader's locale; a pinned one sends es/fr readers to English.
      if (/^\/(en|es|fr)(\/|$|[?#])/.test(href))
        issues.push(`${name}/${locale}: locale-pinned link ${href} at ${entry.path}`);
      const [path, hash] = href.split("#");
      const page = path.replace(/^\/(en|es|fr)(?=\/)/, "").split("?")[0];
      if (hash && anchorsByPath[page] && !anchorsByPath[page].has(hash))
        issues.push(`${name}/${locale}: broken anchor ${href} at ${entry.path}`);
      if (page === "/psychics")
        issues.push(`${name}/${locale}: link to /psychics at ${entry.path}`);
    }
}

/** Issues for one legal document across locales (English is the source). */
export function checkLegalDocument(
  name: string,
  docs: Locales<LegalDocumentContent>,
  anchorsByPath: Record<string, Set<string>> = {},
): string[] {
  const issues: string[] = [];
  const source = docs.en;
  if (!source) return [`${name}: missing en`];
  const base = legalEntries(source);
  for (const dupe of duplicates(base.ids)) issues.push(`${name}: duplicate id #${dupe}`);
  for (const [locale, doc] of Object.entries(docs)) {
    const current = legalEntries(doc);
    if (locale !== "en") {
      if (current.signature.join("\n") !== base.signature.join("\n"))
        issues.push(`${name}/${locale}: structure differs from en (${firstDifference(base.signature, current.signature)})`);
      for (const key of ["version", "effective", "lastUpdated"] as const)
        if (doc.meta[key] !== source.meta[key])
          issues.push(`${name}/${locale}: meta.${key} ${doc.meta[key]} differs from en ${source.meta[key]}`);
      compareEntries(name, locale, base.entries, current.entries, issues);
    }
    for (const key of ["effective", "lastUpdated"] as const)
      if (!ISO_DATE.test(doc.meta[key]))
        issues.push(`${name}/${locale}: meta.${key} is not an ISO date`);
    checkLinks(name, locale, current.entries, new Set(current.ids), anchorsByPath, issues);
  }
  return issues;
}

/** Issues for one calculator article across locales. */
export function checkArticle(
  name: string,
  articles: Locales<ArticleContent>,
  registeredUrls?: Set<string>,
): string[] {
  const issues: string[] = [];
  const source = articles.en;
  if (!source) return [`${name}: missing en`];
  const base = articleEntries(source);
  for (const dupe of duplicates(base.ids)) issues.push(`${name}: duplicate id #${dupe}`);
  for (const dupe of duplicates(source.sections.map((s) => s.id)))
    issues.push(`${name}: duplicate section ${dupe}`);
  for (const [locale, article] of Object.entries(articles)) {
    const current = articleEntries(article);
    if (article.kind !== source.kind)
      issues.push(`${name}/${locale}: kind ${article.kind} differs from en`);
    if (locale !== "en") {
      if (current.signature.join("\n") !== base.signature.join("\n"))
        issues.push(`${name}/${locale}: structure differs from en (${firstDifference(base.signature, current.signature)})`);
      compareEntries(name, locale, base.entries, current.entries, issues);
    }
    for (const src of current.images) {
      if (!src.startsWith("https://assets.staryield.net/assets/"))
        issues.push(`${name}/${locale}: image outside R2: ${src}`);
      else if (registeredUrls && !registeredUrls.has(src))
        issues.push(`${name}/${locale}: image not registered in r2-images.json: ${src}`);
    }
    checkLinks(name, locale, current.entries, new Set(current.ids), {}, issues);
  }
  return issues;
}

function firstDifference(a: string[], b: string[]): string {
  const length = Math.max(a.length, b.length);
  for (let i = 0; i < length; i++)
    if (a[i] !== b[i]) return `${a[i] ?? "∅"} vs ${b[i] ?? "∅"}`;
  return "same";
}

export function legalAnchorSet(doc: LegalDocumentContent): Set<string> {
  return new Set(legalEntries(doc).ids);
}
