import type { Locale } from "@/i18n/config";
import type {
  LegalBlock,
  LegalDocumentContent,
  LegalDocumentId,
} from "@/content/legal/types";
import privacyEn from "@/content/legal/privacy/en";
import privacyEs from "@/content/legal/privacy/es";
import privacyFr from "@/content/legal/privacy/fr";
import termsEn from "@/content/legal/terms/en";
import termsEs from "@/content/legal/terms/es";
import termsFr from "@/content/legal/terms/fr";

/**
 * Legal documents are imported only by server components, so their text is
 * rendered to HTML once and never shipped in the client dictionary.
 */
export const legalDocuments: Record<
  LegalDocumentId,
  Record<Locale, LegalDocumentContent>
> = {
  privacy: { en: privacyEn, es: privacyEs, fr: privacyFr },
  terms: { en: termsEn, es: termsEs, fr: termsFr },
};

export function getLegalDocument(
  id: LegalDocumentId,
  locale: Locale,
): LegalDocumentContent {
  return legalDocuments[id][locale] ?? legalDocuments[id].en;
}

/** Every anchor id a document defines, in document order. */
export function legalAnchorIds(content: LegalDocumentContent): string[] {
  const ids: string[] = [];
  const visit = (blocks: LegalBlock[]) => {
    for (const block of blocks)
      if (block.type === "h3" || block.type === "h4") ids.push(block.id);
  };
  visit(content.intro);
  for (const section of content.sections) {
    ids.push(section.id);
    visit(section.blocks);
  }
  return ids;
}
