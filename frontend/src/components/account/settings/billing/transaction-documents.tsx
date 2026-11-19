"use client";

import Link from "next/link";
import { FileText } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import type { Transaction } from "@/lib/api/billing";
import { invoicePagePath } from "@/lib/api/invoices";

/**
 * "Invoice" under a paid purchase and "Credit note" under a refund in the
 * payment history (revisions 6.0, §26.6). Both open the invoice page; renders
 * nothing while neither exists yet (the API issues them just after the payment).
 */
export function TransactionDocuments({ transaction }: { transaction: Pick<Transaction, "invoice" | "credit_note"> }) {
  const { text, locale } = useI18n();
  const { invoice, credit_note: creditNote } = transaction;
  if (!invoice && !creditNote) return null;
  return (
    <div className="settings-transaction-refund settings-transaction-documents" data-testid="transaction-documents">
      {invoice ? (
        <Link href={invoicePagePath(locale, invoice.id)} className="settings-link-button settings-refund-action" data-testid="transaction-invoice">
          <FileText aria-hidden className="size-3.5" />
          {text("Invoice")} <span className="sr-only">{invoice.number}</span>
        </Link>
      ) : null}
      {creditNote ? (
        <Link href={invoicePagePath(locale, creditNote.id)} className="settings-link-button settings-refund-action" data-testid="transaction-credit-note">
          <FileText aria-hidden className="size-3.5" />
          {text("Credit note")} <span className="sr-only">{creditNote.number}</span>
        </Link>
      ) : null}
    </div>
  );
}
