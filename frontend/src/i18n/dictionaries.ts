import "server-only";

import type { Locale } from "@/i18n/config";
import type { Dictionary } from "@/i18n/types";

const dictionaries: Record<Locale, () => Promise<Dictionary>> = {
  en: async () => ({
    ...(await import("@/i18n/dictionaries/en.json")).default,
    copy: (await import("@/i18n/dictionaries/copy-en.json")).default,
  }),
  fr: async () => ({
    ...(await import("@/i18n/dictionaries/fr.json")).default,
    copy: (await import("@/i18n/dictionaries/copy-fr.json")).default,
  }),
  es: async () => ({
    ...(await import("@/i18n/dictionaries/es.json")).default,
    copy: (await import("@/i18n/dictionaries/copy-es.json")).default,
  }),
};

export const getDictionary = async (locale: Locale): Promise<Dictionary> =>
  dictionaries[locale]();

export type { Dictionary };
