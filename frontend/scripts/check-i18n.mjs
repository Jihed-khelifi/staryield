import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import ts from "typescript";

const read = (path) =>
  JSON.parse(readFileSync(new URL(path, import.meta.url), "utf8"));
const normalize = (value) => value.trim().replace(/\s+/g, " ");
const copies = Object.fromEntries(
  ["en", "fr", "es"].map((locale) => [
    locale,
    read(`../src/i18n/dictionaries/copy-${locale}.json`),
  ]),
);
const placeholders = (value) =>
  [...value.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();
const flatten = (value, prefix = "") =>
  Object.fromEntries(
    Object.entries(value).flatMap(([key, item]) =>
      typeof item === "string"
        ? [[`${prefix}${key}`, item]]
        : Object.entries(flatten(item, `${prefix}${key}.`)),
    ),
  );
for (const locale of ["fr", "es"]) {
  assert.deepEqual(
    Object.keys(copies[locale]).sort(),
    Object.keys(copies.en).sort(),
    `${locale}: copy keys differ`,
  );
  for (const [key, value] of Object.entries(copies[locale])) {
    assert.ok(value.trim(), `${locale}: empty translation: ${key}`);
    assert.deepEqual(
      placeholders(value),
      placeholders(key),
      `${locale}: broken interpolation: ${key}`,
    );
  }
  const source = flatten(read("../src/i18n/dictionaries/en.json"));
  const target = flatten(read(`../src/i18n/dictionaries/${locale}.json`));
  assert.deepEqual(
    Object.keys(target).sort(),
    Object.keys(source).sort(),
    `${locale}: dictionary keys differ`,
  );
  for (const key of Object.keys(source))
    assert.deepEqual(
      placeholders(target[key]),
      placeholders(source[key]),
      `${locale}: broken interpolation: ${key}`,
    );
}

async function loadTypeScript(path) {
  const source = readFileSync(new URL(path, import.meta.url), "utf8");
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  });
  return import(
    `data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`
  );
}
const { localePath } = await loadTypeScript("../src/i18n/config.ts");
for (const locale of ["en", "fr", "es"]) {
  assert.equal(
    localePath("/en/chatroom?reader=theo#saved", locale),
    `/${locale}/chatroom?reader=theo#saved`,
  );
  assert.equal(
    localePath("/fr/profile/settings", locale),
    `/${locale}/profile/settings`,
  );
  assert.equal(localePath("/es", locale), `/${locale}`);
  assert.equal(localePath("/", locale), `/${locale}`);
  assert.equal(localePath("/france", locale), `/${locale}/france`);
}
const { createTranslateCopy } = await loadTypeScript(
  "../src/i18n/translate-copy.ts",
);
const text = createTranslateCopy(copies.fr);
assert.equal(text("  Sign Up "), `  ${copies.fr["Sign Up"]} `);
assert.equal(text("SUPPORT"), copies.fr.Support.toUpperCase());
assert.equal(
  text("Remove {value0}", { value0: "photo.png" }),
  "Supprimer photo.png",
);
assert.equal(text(320), 320);
assert.equal(
  text("A message typed by the visitor"),
  "A message typed by the visitor",
);

const files = (dir) =>
  readdirSync(dir, { withFileTypes: true }).flatMap((entry) =>
    entry.isDirectory()
      ? files(join(dir, entry.name))
      : [join(dir, entry.name)],
  );
const known = new Set(Object.keys(copies.en).map((key) => key.toLowerCase()));
let checked = 0;
for (const file of files(new URL("../src", import.meta.url).pathname).filter(
  (file) => /\.tsx?$/.test(file),
)) {
  const source = ts.createSourceFile(
    file,
    readFileSync(file, "utf8"),
    ts.ScriptTarget.Latest,
    true,
  );
  function checkCopy(node) {
    if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
      const value = normalize(node.text);
      if (/[a-zA-Z]/.test(value)) {
        assert.ok(
          known.has(value.toLowerCase()),
          `${file}: missing display copy: ${value}`,
        );
        checked++;
      }
    } else if (ts.isConditionalExpression(node)) {
      checkCopy(node.whenTrue);
      checkCopy(node.whenFalse);
    } else if (
      ts.isCallExpression(node) &&
      node.expression.getText(source) === "text"
    ) {
      checkCopy(node.arguments[0]);
    }
  }
  function visit(node) {
    if (ts.isCallExpression(node) && node.expression.getText(source) === "text")
      checkCopy(node.arguments[0]);
    if (ts.isJsxText(node))
      assert.ok(
        !/[a-zA-Z]/.test(
          node.text.trim().replace(/&(?:#\d+|#x[\da-f]+|[a-z]+);/gi, ""),
        ) || node.text.trim() === "in",
        `${file}: untranslated JSX text: ${node.text.trim()}`,
      );
    ts.forEachChild(node, visit);
  }
  visit(source);
}
console.log(
  `i18n checks passed: ${Object.keys(copies.en).length} strings per locale, ${checked} display-copy references, route preservation, and interpolation.`,
);
