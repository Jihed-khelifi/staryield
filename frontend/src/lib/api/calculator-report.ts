/**
 * The calculators' e-mail report (revisions 6.0, docs/platform-extensions.md
 * §26.4): the dialog's config, the report request and the marketing
 * unsubscribe. Wire types mirror `backend/internal/service/calculator_report.go`.
 *
 * The API recalculates the report from `input` (the calculator's own request
 * body), so nothing the browser computed is trusted, and it never stores the
 * birth details: they only travel in this POST body.
 */
import { api } from "@/lib/api/client";
import type { CalculatorKind } from "@/lib/schemas/calculator";

export type ReportEmailKind = CalculatorKind;
export type ReportEmailLocale = "en" | "fr" | "es";

/** GET /calculators/report-email/config, for the request's trusted country. */
export type ReportEmailConfig = {
  enabled: boolean;
  /** The marketing box starts ticked (only where a pre-ticked box is lawful). */
  marketing_prechecked: boolean;
};

export type ReportEmailRequest = {
  kind: ReportEmailKind;
  /** The calculator's own request shape (natal chart, sun sign, numerology). */
  input: Record<string, unknown>;
  email: string;
  locale: ReportEmailLocale;
  marketing_opt_in: boolean;
  /** The box started ticked and the visitor never touched it. */
  marketing_prechecked: boolean;
  /** Honeypot: real visitors never see the field. */
  website?: string;
};

export const reportEmailApi = {
  /** Never cached by the API (it answers per country). */
  config: (signal?: AbortSignal) => api<ReportEmailConfig>("/api/v1/calculators/report-email/config", { signal, retries: 1 }),
  /** 202 queued. Not retried by the client: a second request spends the address's daily budget. */
  send: (body: ReportEmailRequest, signal?: AbortSignal) =>
    api<{ status: "queued" }>("/api/v1/calculators/report-email", { method: "POST", body, signal }),
  /** The unsubscribe link's token travels in the body, never in a URL. */
  unsubscribe: (token: string, signal?: AbortSignal) =>
    api<{ status: "unsubscribed" }>("/api/v1/marketing/unsubscribe", { method: "POST", body: { token }, signal, idempotent: true }),
};
