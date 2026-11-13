# Legal archive

Text that was removed from the published legal documents, kept verbatim so it can be restored.

## 2026-11-13-memberships (revisions 5.1)

Memberships (the "Premium Membership" subscription) are built but switched off and not sold
(`STARYIELD_BILLING_MEMBERSHIPS_ENABLED=false`, docs/platform-extensions.md §25.1). A payment processor's
underwriting team reads the public Terms and Privacy Policy, so on 2026-11-13 the membership and subscription
text was removed from them (Terms and Privacy Policy version 2026-11-13).

`en.md`, `fr.md` and `es.md` hold, per locale, the text exactly as it was published in version 2026-11-12:

- Terms: the "Important notice regarding automatic renewals" notice, section 6.2 "Subscription Terms"
  (6.2.1 to 6.2.8, with their anchor ids), and every other paragraph that mentioned memberships or
  subscriptions (sections 1, 1.2, 3.6, 3.7, 4.8, 4.11, 4.13, 4.15, 6, 6.1.1, 6.1.2 and the right-of-withdrawal
  clause 6.2.9), complete, so the old wording can be compared sentence by sentence.
- Privacy Policy: the paragraphs and table entries that mentioned the Premium Membership.

Two paragraphs of the old 6.2.5 (introductory free minutes and promotional free access, "free mode") are not
membership terms: they stayed in the live Terms as section 6.1.4 (anchor `#free-trials-and-promotional-offers`).

### When memberships launch

1. Restore the text from these files into `frontend/src/content/legal/{terms,privacy}/{en,fr,es}.ts`, adapted to
   the product as it is then (plans, benefits, billing periods, cancellation path in Settings, prices).
2. Re-add the section anchors listed in the files (`#subscription-terms`, `#auto-renewal-subscription-continuity-and-cancellation`, ...)
   and the "Important notice regarding automatic renewals" notice at the top of the Terms.
3. Remove or adjust the sentence "We do not currently offer subscriptions, memberships or automatic top-ups"
   in Terms 6.1.1 and the matching sentence in Refund Policy section 1.
4. Bump `meta.version` / `effective` / `lastUpdated` in the three documents and `LEGAL_VERSIONS`
   (`frontend/src/lib/legal/versions.ts`); update `revisions-5-1.test.ts`, which asserts that no membership clause is published.
5. Show the membership surfaces again (`GET /site/config` `features.memberships`), and make the checkout carry the
   renewal disclosures required by docs/platform-extensions.md §25.1.
