"use client";

import Link from "next/link";

import { useI18n } from "@/i18n/i18n-provider";
import { LinkedCopy } from "@/components/billing/linked-copy";

/** The policy pages a buyer agrees to (docs/platform-extensions.md §25.6). */
export type PolicyPage = "terms" | "privacy" | "refund-policy";

/**
 * A renderer for `LinkedCopy`: a link to a policy page that opens in a new
 * tab, so a live chat or a half-filled checkout is never left. The label is
 * the translated, bracketed text of the sentence; screen readers also hear
 * that the link opens a new tab.
 */
export function usePolicyLink() {
  const { text, locale } = useI18n();
  return (page: PolicyPage) =>
    function PolicyLink(label: string) {
      return (
        <Link href={`/${locale}/${page}`} target="_blank" rel="noopener noreferrer" className="underline">
          {label}
          <span className="sr-only"> {text("(opens in a new tab)")}</span>
        </Link>
      );
    };
}

/** "I agree to the [Terms of Use], [Privacy Policy] and [Refund & Cancellation Policy]." */
export function PolicyAgreementCopy() {
  const { text } = useI18n();
  const link = usePolicyLink();
  return (
    <LinkedCopy
      copy={text("I agree to the [Terms of Use], [Privacy Policy] and [Refund & Cancellation Policy].")}
      links={[link("terms"), link("privacy"), link("refund-policy")]}
    />
  );
}
