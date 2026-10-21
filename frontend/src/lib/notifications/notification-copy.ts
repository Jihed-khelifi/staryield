/**
 * Notification text is rendered by the client in the viewer's language from
 * `kind` + `data` (docs/platform-extensions.md §7.1). Templates are English
 * source copy translated with the i18n `text()` helper (copy-*.json); only
 * broadcasts carry admin-written copy per locale.
 */
import type { AppNotification } from "@/lib/api/types";
import type { TranslationValues } from "@/i18n/types";

type Translate = <T>(value: T, values?: TranslationValues) => T;

export type NotificationCopy = { title: string; body: string };

/** English source copy, keyed for tests that check every locale has it. */
export const notificationTemplates = {
  chatRequestedTitle: "New chat request",
  chatRequestedBody: "{name} would like to start a live chat.",
  chatMissedTitle: "Missed chat request",
  chatMissedBody: "{name}'s chat request expired before you could answer.",
  reviewPromptTitle: "How was your reading?",
  reviewPromptBody: "Rate your chat with {name}.",
  reviewReceivedTitle: "New review",
  reviewReceivedBody: "A client rated your chat {rating} out of 5.",
  requestCreatedTitle: "New request",
  requestCreatedBody: "{client} ordered {service}.",
  requestUpdatedTitle: "Request update",
  requestUpdatedBody: "{service} · {status}",
  purchaseTitle: "Credits added",
  purchaseBody: "{credits} credits were added to your balance.",
  creditsAddedTitle: "Credits added",
  creditsAddedBody: "Staryield added {amount} credits to your balance.",
  creditsRemovedTitle: "Balance adjusted",
  creditsRemovedBody: "{amount} credits were removed from your balance.",
  withdrawalTitle: "Withdrawal update",
  fallbackTitle: "Notification",
  someone: "A client",
} as const;

/** Request statuses (same copy as the orders list). */
export const requestStatusCopy: Record<string, string> = {
  pending: "Waiting for the advisor",
  accepted: "In progress",
  delivered: "Delivered",
  declined: "Declined — refunded",
  cancelled: "Cancelled — refunded",
  expired: "Expired — refunded",
  disputed: "Under review",
  refunded: "Refunded",
};

/** One sentence per withdrawal status, for natural translations. */
export const withdrawalStatusCopy: Record<string, string> = {
  pending: "Your withdrawal of {amount} credits is pending review.",
  approved: "Your withdrawal of {amount} credits was approved.",
  paid: "Your withdrawal of {amount} credits has been paid.",
  rejected: "Your withdrawal of {amount} credits was rejected. The credits are available again.",
  cancelled: "Your withdrawal of {amount} credits was cancelled.",
};

function str(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  return "";
}

function num(value: unknown): number | null {
  const n = typeof value === "number" ? value : typeof value === "string" && value.trim() ? Number(value) : NaN;
  return Number.isFinite(n) ? n : null;
}

/** Admin-written copy: the viewer's locale, else English, else any value. */
export function pickLocalized(value: unknown, locale: string): string {
  if (typeof value === "string") return value;
  if (!value || typeof value !== "object") return "";
  const map = value as Record<string, unknown>;
  const exact = str(map[locale]).trim();
  if (exact) return exact;
  const en = str(map.en).trim();
  if (en) return en;
  for (const v of Object.values(map)) if (str(v).trim()) return str(v).trim();
  return "";
}

/** Localized title and body for a notification. */
export function notificationText(n: AppNotification, locale: string, text: Translate): NotificationCopy {
  const T = notificationTemplates;
  const d = n.data ?? {};
  const fmt = (v: number) => new Intl.NumberFormat(locale).format(v);
  const name = (key: string) => str(d[key]).trim() || text(T.someone);
  switch (n.kind) {
    case "chat.requested":
      return { title: text(T.chatRequestedTitle), body: text(T.chatRequestedBody, { name: name("client_name") }) };
    case "chat.missed":
      return { title: text(T.chatMissedTitle), body: text(T.chatMissedBody, { name: name("client_name") }) };
    case "chat.review_prompt":
      return { title: text(T.reviewPromptTitle), body: text(T.reviewPromptBody, { name: str(d.psychic_name) || "Staryield" }) };
    case "review.received": {
      const rating = num(d.rating);
      return { title: text(T.reviewReceivedTitle), body: text(T.reviewReceivedBody, { rating: rating === null ? "?" : fmt(rating) }) };
    }
    case "request.created":
      return {
        title: text(T.requestCreatedTitle),
        body: text(T.requestCreatedBody, { client: name("client_name"), service: text(str(d.service_name)) }),
      };
    case "request.updated": {
      const status = str(d.status);
      return {
        title: text(T.requestUpdatedTitle),
        body: text(T.requestUpdatedBody, { service: text(str(d.service_name)), status: text(requestStatusCopy[status] ?? status) }),
      };
    }
    case "purchase.succeeded":
      return { title: text(T.purchaseTitle), body: text(T.purchaseBody, { credits: fmt(num(d.credits) ?? 0) }) };
    case "credits.adjusted": {
      const amount = num(d.amount) ?? 0;
      return amount >= 0
        ? { title: text(T.creditsAddedTitle), body: text(T.creditsAddedBody, { amount: fmt(amount) }) }
        : { title: text(T.creditsRemovedTitle), body: text(T.creditsRemovedBody, { amount: fmt(Math.abs(amount)) }) };
    }
    case "withdrawal.updated": {
      const template = withdrawalStatusCopy[str(d.status)] ?? withdrawalStatusCopy.pending;
      return { title: text(T.withdrawalTitle), body: text(template, { amount: fmt(num(d.amount_credits) ?? 0) }) };
    }
    case "broadcast":
      return { title: pickLocalized(d.title, locale) || text(T.fallbackTitle), body: pickLocalized(d.body, locale) };
    default:
      return { title: text(T.fallbackTitle), body: "" };
  }
}

const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 365 * 86400],
  ["month", 30 * 86400],
  ["week", 7 * 86400],
  ["day", 86400],
  ["hour", 3600],
  ["minute", 60],
];

/** "now", "5 min ago", "yesterday" … in the viewer's language. */
export function relativeTime(iso: string, locale: string, now = Date.now()): string {
  const at = Date.parse(iso);
  if (!Number.isFinite(at)) return "";
  const seconds = (at - now) / 1000;
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto", style: "short" });
  const abs = Math.abs(seconds);
  if (abs < 45) return rtf.format(0, "second");
  for (const [unit, size] of UNITS) {
    if (abs >= size || unit === "minute") return rtf.format(Math.round(seconds / size), unit);
  }
  return rtf.format(0, "second");
}

/** A locale-prefixed in-app path for a notification link, or null when unsafe. */
export function notificationHref(link: string, locale: string): string | null {
  if (!link || !link.startsWith("/") || link.startsWith("//") || link.includes("\\") || /[\u0000-\u001f]/.test(link)) return null;
  try {
    const url = new URL(link, "https://staryield.invalid");
    if (url.origin !== "https://staryield.invalid") return null;
    const path = url.pathname.replace(/^\/(en|fr|es)(?=\/|$)/, "");
    return `/${locale}${path === "/" ? "" : path}${url.search}${url.hash}`;
  } catch {
    return null;
  }
}
