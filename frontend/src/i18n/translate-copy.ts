import type { TranslationValues } from "@/i18n/types";

export type TranslateCopy = <T>(value: T, values?: TranslationValues) => T;

/** Translate display copy without changing route IDs, form values, or data keys. */
export function createTranslateCopy(
  copy: Record<string, string>,
): TranslateCopy {
  const caseInsensitive = new Map(
    Object.keys(copy).map((key) => [key.toLowerCase(), key]),
  );
  return <T>(value: T, values?: TranslationValues): T => {
    if (typeof value !== "string") return value;
    const key = value.trim().replace(/\s+/g, " ");
    const sourceKey =
      copy[key] !== undefined ? key : caseInsensitive.get(key.toLowerCase());
    if (!sourceKey) return value;
    let translated = copy[sourceKey];
    if (key === key.toUpperCase() && /[A-Z]/.test(key))
      translated = translated.toUpperCase();
    if (values)
      translated = translated.replace(/\{(\w+)\}/g, (match, name: string) =>
        name in values ? String(values[name]) : match,
      );
    return (value.match(/^\s*/)?.[0] +
      translated +
      value.match(/\s*$/)?.[0]) as T;
  };
}
