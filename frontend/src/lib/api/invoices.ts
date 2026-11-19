/**
 * Invoices and credit notes (revisions 6.0, docs/platform-extensions.md §26.6).
 * Every paid credit purchase has an invoice and every refund a credit note,
 * whichever processor took the payment. Both are immutable once issued; the
 * PDF is rendered on demand by the API.
 */
import { api } from "@/lib/api/client";
import { API_URL } from "@/lib/api/config";

export type InvoiceLine = { description: string; quantity: number; unit_amount_cents: number; amount_cents: number; credits?: number };

export type Invoice = {
  id: string;
  /** "SY-2026-000001" (invoices) or "SYC-2026-000001" (credit notes). */
  number: string;
  kind: "invoice" | "credit_note";
  transaction_id: string;
  /** Credit notes: the invoice they correct. */
  original_invoice?: { id: string; number: string };
  currency: string;
  subtotal_cents: number;
  tax_cents: number;
  total_cents: number;
  lines: InvoiceLine[];
  buyer: { name: string; email: string; country?: string };
  seller: { legal_name: string; address: string[]; email: string; website: string };
  payment: {
    provider: string;
    card?: { brand: string; last4: string };
    reference?: string;
    paid_at?: string;
    refunded_at?: string;
    statement_descriptor?: string;
  };
  /** The language of the PDF. */
  locale: string;
  issued_at: string;
};

/** The signed-in owner's invoice (404 for anyone else's). */
export function getInvoice(id: string, signal?: AbortSignal): Promise<Invoice> {
  return api<Invoice>(`/api/v1/billing/invoices/${encodeURIComponent(id)}`, { signal });
}

/**
 * Where the PDF is downloaded from (`Content-Disposition: attachment`). A plain
 * link: the browser sends the session cookie and saves the file.
 */
export function invoicePdfUrl(id: string): string {
  return `${API_URL}/api/v1/billing/invoices/${encodeURIComponent(id)}/pdf`;
}

/** Where the signed-in page of an invoice lives. */
export function invoicePagePath(locale: string, id: string): string {
  return `/${locale}/profile/invoices/${encodeURIComponent(id)}`;
}
