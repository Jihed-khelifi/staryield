/**
 * Logic of the calculators' "Save your reading" prompt (revisions 6.0, §26.4):
 * what is sent to the API, when the dialog opens by itself and how that is
 * remembered. Everything here is plain functions so it can be tested without a
 * browser; the React side is in use-report-email-prompt.ts and
 * report-email-dialog.tsx.
 */
import { browserTimeZone } from "@/lib/api/astrology";
import type { ReportEmailRequest } from "@/lib/api/calculator-report";
import type { CalculatorFormValues, CalculatorKind, LifePathFormValues, NatalFormValues, SunSignFormValues } from "@/lib/schemas/calculator";
import { birthDateOf } from "@/lib/schemas/calculator";
import { natalPayload, sunSignPayload } from "@/components/calculators/calculate";

/** Seconds on screen after which the dialog opens by itself. */
export const PROMPT_AFTER_MS = 6000;
/** Share of the page scrolled after which the dialog opens by itself. */
export const PROMPT_SCROLL_SHARE = 0.6;
/** After the dialog was shown, it does not open by itself again for this long. */
export const PROMPT_QUIET_MS = 30 * 24 * 60 * 60 * 1000;

const STORAGE_PREFIX = "staryield.report-email.";

/** The e-mail report supports the three original calculators only (the tarot birth card has none). */
export function isReportable(kind: CalculatorKind): boolean {
  return kind !== "tarot-birth-card";
}

/**
 * The request body of the calculator's own API: the same payload the page
 * sent to calculate the result, so the server validates it with the same
 * rules. The path of life has no API call of its own on the page; its input is
 * the numerology request (birth date and the visitor's zone for "today").
 */
export function reportInput(kind: CalculatorKind, values: CalculatorFormValues, timeZone = browserTimeZone()): ReportEmailRequest["input"] {
  if (!isReportable(kind)) throw new RangeError(`${kind} has no e-mail report`);
  if (kind === "natal-chart") return natalPayload(values as NatalFormValues);
  if (kind === "astrology") return sunSignPayload(values as SunSignFormValues);
  return { birth_date: birthDateOf((values as LifePathFormValues).birth), ...(timeZone ? { tz: timeZone } : {}) };
}

/** When the dialog was last shown (opened by itself) for a calculator, or null. */
export function lastPromptedAt(kind: CalculatorKind, storage?: Pick<Storage, "getItem">): number | null {
  try {
    const raw = (storage ?? window.localStorage).getItem(STORAGE_PREFIX + kind);
    const at = raw === null ? NaN : Number(raw);
    return Number.isFinite(at) && at > 0 ? at : null;
  } catch {
    return null; // storage blocked (private mode, cleared site data): behave as "never shown"
  }
}

/** The dialog may open by itself: never shown, or last shown more than 30 days ago. */
export function mayPrompt(kind: CalculatorKind, now = Date.now(), storage?: Pick<Storage, "getItem">): boolean {
  const at = lastPromptedAt(kind, storage);
  return at === null || now - at >= PROMPT_QUIET_MS || at > now;
}

/** Remember that the dialog was shown (or answered) for a calculator. */
export function rememberPrompt(kind: CalculatorKind, now = Date.now(), storage?: Pick<Storage, "setItem">) {
  try {
    (storage ?? window.localStorage).setItem(STORAGE_PREFIX + kind, String(now));
  } catch {
    // Blocked storage only means the prompt may open again next time.
  }
}

/** Share (0..1) of the scrollable height the visitor has scrolled; 0 when the page does not scroll. */
export function scrolledShare(doc: Pick<Document, "documentElement"> = document, view: Pick<Window, "scrollY" | "innerHeight"> = window): number {
  const max = doc.documentElement.scrollHeight - view.innerHeight;
  if (max <= 0) return 0;
  return Math.min(Math.max(view.scrollY / max, 0), 1);
}
