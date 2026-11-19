"use client";

/**
 * `/{locale}/profile/invoices/:id` (revisions 6.0, §26.6): a Staryield invoice
 * or credit note on screen, with print styles (`@media print`, see
 * `invoice.css`) and "Download PDF". Loading, not-found and error states.
 */
import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Download, FileText, Printer } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { getInvoice, invoicePagePath, invoicePdfUrl, type Invoice } from "@/lib/api/invoices";
import { useSession } from "@/lib/auth/session-provider";
import { useAsync } from "@/lib/use-async";
import { formatDay, formatMoney } from "@/components/account/settings/format";
import { brandName } from "@/components/account/settings/billing/billing-format";
import { ErrorState } from "@/components/feedback/error-state";
import { PublicFrame } from "@/components/layout/public-frame";
import { Button } from "@/components/ui/button";
import { Skeleton, SkeletonText } from "@/components/ui/skeleton";

function InvoiceSkeleton() {
  const { text } = useI18n();
  return (
    <div aria-busy="true" data-testid="invoice-skeleton" className="grid gap-5">
      <p role="status" className="sr-only">
        {text("Loading your invoice…")}
      </p>
      <Skeleton className="h-8 w-56" />
      <Skeleton className="h-24 w-full" />
      <SkeletonText lines={4} />
    </div>
  );
}

function NotFound() {
  const { text, locale } = useI18n();
  return (
    <div role="alert" data-testid="invoice-not-found" className="grid justify-items-center gap-4 text-center">
      <FileText aria-hidden className="size-9 text-amber" />
      <h2 className="text-2xl font-semibold">{text("We could not find this invoice")}</h2>
      <p className="max-w-prose font-light">{text("It may belong to another account, or the link is not right. Your invoices are listed in your payment history.")}</p>
      <Button asChild variant="cta" size="lg" className="px-4">
        <Link href={`/${locale}/profile/settings#billing`}>{text("Back to billing")}</Link>
      </Button>
    </div>
  );
}

function Row({ label, children, testId }: { label: string; children: React.ReactNode; testId?: string }) {
  return (
    <div className="invoice-row" data-testid={testId}>
      <dt>{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}

/** The invoice document: seller, buyer, lines, totals, payment and descriptor. */
export function InvoiceDocument({ invoice }: { invoice: Invoice }) {
  const { text, locale } = useI18n();
  const money = (cents: number) => formatMoney(cents, invoice.currency, locale);
  const creditNote = invoice.kind === "credit_note";
  const { payment } = invoice;
  const dated = creditNote ? payment.refunded_at : payment.paid_at;
  return (
    <article className="invoice-sheet" data-testid="invoice" data-kind={invoice.kind} aria-labelledby="invoice-title">
      <header className="invoice-head">
        <div>
          <p className="eyebrow">{creditNote ? text("Credit note") : text("Invoice")}</p>
          <h1 id="invoice-title" className="invoice-number" data-testid="invoice-number">
            {invoice.number}
          </h1>
          {invoice.original_invoice ? (
            <p className="invoice-muted" data-testid="invoice-original">
              {text("Refund of invoice")}{" "}
              <Link href={invoicePagePath(locale, invoice.original_invoice.id)} className="underline">
                {invoice.original_invoice.number}
              </Link>
            </p>
          ) : null}
        </div>
        <p className="invoice-date">
          <span className="invoice-muted">{text("Issued")}</span> <time dateTime={invoice.issued_at}>{formatDay(invoice.issued_at, locale)}</time>
        </p>
      </header>

      <div className="invoice-parties">
        <section aria-labelledby="invoice-seller" data-testid="invoice-seller">
          <h2 id="invoice-seller" className="invoice-label">
            {text("From")}
          </h2>
          <p className="font-semibold">{invoice.seller.legal_name}</p>
          {invoice.seller.address.map((line) => (
            <p key={line}>{line}</p>
          ))}
          <p>{invoice.seller.email}</p>
          <p>{invoice.seller.website}</p>
        </section>
        <section aria-labelledby="invoice-buyer" data-testid="invoice-buyer">
          <h2 id="invoice-buyer" className="invoice-label">
            {text("Billed to")}
          </h2>
          <p className="font-semibold">{invoice.buyer.name}</p>
          <p>{invoice.buyer.email}</p>
          {invoice.buyer.country ? <p>{invoice.buyer.country}</p> : null}
        </section>
      </div>

      <div className="invoice-table-wrap">
        <table className="invoice-lines" data-testid="invoice-lines">
          <caption className="sr-only">{text("Items")}</caption>
          <thead>
            <tr>
              <th scope="col">{text("Description")}</th>
              <th scope="col" className="invoice-num">
                {text("Qty")}
              </th>
              <th scope="col" className="invoice-num">
                {text("Unit price")}
              </th>
              <th scope="col" className="invoice-num">
                {text("Amount")}
              </th>
            </tr>
          </thead>
          <tbody>
            {invoice.lines.map((line, index) => (
              <tr key={`${line.description}-${index}`}>
                <td>
                  {line.description}
                  {line.credits ? <span className="invoice-muted"> · {text("{value0} credits", { value0: line.credits.toLocaleString(locale) })}</span> : null}
                </td>
                <td className="invoice-num">{line.quantity}</td>
                <td className="invoice-num">{money(line.unit_amount_cents)}</td>
                <td className="invoice-num">{money(line.amount_cents)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <dl className="invoice-totals" data-testid="invoice-totals">
        <Row label={text("Subtotal")}>{money(invoice.subtotal_cents)}</Row>
        <Row label={text("Tax")}>{invoice.tax_cents > 0 ? money(invoice.tax_cents) : text("Not applicable")}</Row>
        <Row label={creditNote ? text("Total refunded") : text("Total")} testId="invoice-total">
          <strong>
            {money(invoice.total_cents)} {invoice.currency.toUpperCase()}
          </strong>
        </Row>
      </dl>

      <section aria-labelledby="invoice-payment" className="invoice-payment" data-testid="invoice-payment">
        <h2 id="invoice-payment" className="invoice-label">
          {text("Payment")}
        </h2>
        <dl>
          {payment.card ? <Row label={text("Card")}>{text("{value0} ending in {value1}", { value0: brandName(payment.card.brand), value1: payment.card.last4 })}</Row> : null}
          {dated ? <Row label={creditNote ? text("Refunded on") : text("Paid on")}>{formatDay(dated, locale)}</Row> : null}
          {payment.reference ? <Row label={text("Reference")}>{payment.reference}</Row> : null}
          {payment.statement_descriptor ? (
            <Row label={text("Appears on your statement as")} testId="invoice-descriptor">
              {payment.statement_descriptor}
            </Row>
          ) : null}
        </dl>
      </section>

      <footer className="invoice-footer">
        <p>{text("Credits are for entertainment purposes only and have no cash value.")}</p>
        <p>
          {text("Questions about this document? Write to {value0}.", { value0: invoice.seller.email })}
        </p>
      </footer>
    </article>
  );
}

/** The signed-in page of one invoice or credit note. */
export function InvoicePage({ id }: { id: string }) {
  const { text, locale } = useI18n();
  const router = useRouter();
  const { status } = useSession();
  const path = invoicePagePath(locale, id);
  React.useEffect(() => {
    if (status === "anonymous") router.replace(`/${locale}/login?next=${encodeURIComponent(path)}`);
  }, [status, locale, router, path]);

  const invoice = useAsync((signal) => getInvoice(id, signal), [id], status === "authenticated");
  const { data, error, loading, reload } = invoice;

  let body: React.ReactNode;
  if (data) body = <InvoiceDocument invoice={data} />;
  else if (error && !loading) body = error.status === 404 || error.code === "not_found" ? <NotFound /> : <ErrorState error={error} onRetry={reload} title={text("We could not load this invoice.")} />;
  else body = <InvoiceSkeleton />;

  return (
    <PublicFrame promo={false}>
      <section className="invoice-page contact-page">
        <div className="invoice-actions" data-testid="invoice-actions">
          <Button asChild variant="outline" size="lg" className="border-ink/40 px-4">
            <Link href={`/${locale}/profile/settings#billing`}>{text("Back to billing")}</Link>
          </Button>
          {data ? (
            <>
              <Button type="button" variant="outline" size="lg" className="border-ink/40 px-4" onClick={() => window.print()} data-testid="invoice-print">
                <Printer aria-hidden className="size-4" />
                {text("Print")}
              </Button>
              <Button asChild variant="cta" size="lg" className="px-4">
                <a href={invoicePdfUrl(data.id)} download rel="noopener noreferrer" data-testid="invoice-download">
                  <Download aria-hidden className="size-4" />
                  {text("Download PDF")}
                </a>
              </Button>
            </>
          ) : null}
        </div>
        <div className="contact-card paper-page invoice-card">{body}</div>
      </section>
    </PublicFrame>
  );
}
