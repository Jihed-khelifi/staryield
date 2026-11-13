/**
 * Notification text is rendered by the client in the viewer's language from
 * `kind` + `data` (docs/platform-extensions.md §7.1). Templates are English
 * source copy translated with the i18n `text()` helper (copy-*.json); only
 * broadcasts carry admin-written copy per locale.
 */
import type { AppNotification, NotificationKind } from "@/lib/api/types";
import type { TranslationValues } from "@/i18n/types";

type Translate = <T>(value: T, values?: TranslationValues) => T;

export type NotificationCopy = { title: string; body: string };

/**
 * Every kind the API sends (docs §7.1, §23.6). The `NotificationKind` union is
 * checked against this list in both directions, so a kind added to the type
 * without copy fails the build, and the copy test walks the list.
 */
export const NOTIFICATION_KINDS = [
  "chat.requested",
  "chat.missed",
  "chat.review_prompt",
  "review.received",
  "request.created",
  "request.updated",
  "purchase.succeeded",
  "credits.adjusted",
  "withdrawal.updated",
  "photo.approved",
  "photo.rejected",
  "support.replied",
  "broadcast",
  "waitlist.available",
  "export.ready",
  "account.email_change_requested",
  "account.email_changed",
  "account.reactivated",
  "account.google_linked",
  "account.google_unlinked",
  "subscription.renewed",
  "subscription.payment_failed",
  "subscription.canceled",
  // revisions 5.0 (§24.4)
  "refund_request.received",
  "refund_request.resolved",
  // revisions 5.1 (§25.5)
  "payment.refunded",
  "subscription.started",
] as const satisfies readonly NotificationKind[];

type MissingKinds = Exclude<NotificationKind, (typeof NOTIFICATION_KINDS)[number]>;
// Compile-time guard: fails when a member of NotificationKind is not in the list.
const everyKindListed: [MissingKinds] extends [never] ? true : never = true;
void everyKindListed;

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
  photoApprovedTitle: "Photo approved",
  photoApprovedBody: "Your new profile photo is now on your profile.",
  photoRejectedTitle: "Photo not approved",
  photoRejectedBody: "Your new profile photo was not approved: {reason}",
  photoRejectedNoReason: "Your new profile photo was not approved. Upload another one from your settings.",
  supportRepliedTitle: "Support replied to your request",
  supportRepliedBody: "{reference}: {subject}",
  supportRepliedNoSubject: "We answered your request {reference}.",
  // §23.6: every remaining kind has its own copy.
  waitlistAvailableTitle: "Your advisor is online",
  waitlistAvailableBody: "{name} is online now. Start your reading while they are available.",
  waitlistAvailableNoName: "An advisor you waited for is online now.",
  exportReadyTitle: "Your data export is ready",
  exportReadyBody: "Download it from your account settings before the link expires.",
  emailChangeRequestedTitle: "Email change requested",
  emailChangeRequestedBody: "We sent a confirmation link to the new address. If this wasn't you, change your password right away.",
  emailChangedTitle: "Email address changed",
  emailChangedBody: "Your account now uses a new email address. If this wasn't you, contact support right away.",
  reactivatedTitle: "Welcome back",
  reactivatedBody: "Your account is active again.",
  googleLinkedTitle: "Google sign-in added",
  googleLinkedBody: "You can now sign in with the Google account {email}.",
  googleLinkedNoEmail: "You can now sign in with Google.",
  googleUnlinkedTitle: "Google sign-in removed",
  googleUnlinkedBody: "The Google account {email} can no longer be used to sign in.",
  googleUnlinkedNoEmail: "Google sign-in was removed from your account.",
  paymentRefundedTitle: "Refund issued",
  paymentRefundedBody: "{amount} was refunded to your payment method. Your bank may take 5–10 business days to show it.",
  paymentRefundedNoAmount: "A refund was issued to your payment method. Your bank may take 5–10 business days to show it.",
  subscriptionStartedTitle: "Membership started",
  subscriptionStartedBody: "Your membership is active. Thank you for joining.",
  subscriptionRenewedTitle: "Membership renewed",
  subscriptionRenewedBody: "Your membership was renewed. Thank you for staying with us.",
  subscriptionPaymentFailedTitle: "Membership payment failed",
  subscriptionPaymentFailedBody: "We couldn't charge your payment method. Update it to keep your benefits.",
  subscriptionPaymentFailedGrace: "We couldn't charge your payment method. Update it by {date} to keep your benefits.",
  subscriptionCanceledTitle: "Membership ended",
  // revisions 5.0 (§24.4): refund requests.
  refundReceivedTitle: "Refund request received",
  refundReceivedBody: "We received your request {reference}. We aim to acknowledge requests within 2 business days.",
  refundResolvedTitle: "Refund request update",
  fallbackTitle: "Notification",
  someone: "A client",
} as const;

/** Why a membership ended (`subscription.canceled` data `reason`, §12.1). */
export const subscriptionCanceledCopy: Record<string, string> = {
  period_ended: "Your membership has ended. You can subscribe again at any time.",
  payment_failed: "Your membership ended because we couldn't renew it. You can subscribe again at any time.",
  canceled_by_user: "You canceled your membership and it has now ended. You can subscribe again at any time.",
  admin: "Your membership was ended by Staryield. Contact support if you have questions.",
  dispute: "Your membership was ended after a payment dispute. Contact support if you have questions.",
  account_closed: "Your membership ended with your account.",
  other: "Your membership has ended. You can subscribe again at any time.",
};

/**
 * The same sentences without "You can subscribe again at any time.": while
 * memberships are switched off (§25.1) an old membership's end is still
 * reported, but nothing promotes a new one.
 */
export const subscriptionCanceledPlainCopy: Record<string, string> = {
  ...subscriptionCanceledCopy,
  period_ended: "Your membership has ended.",
  payment_failed: "Your membership ended because we couldn't renew it.",
  canceled_by_user: "You canceled your membership and it has now ended.",
  other: "Your membership has ended.",
};

/** One sentence per resolved refund request status (`refund_request.resolved`, §24.4). */
export const refundResolvedCopy: Record<string, string> = {
  approved: "Your refund request {reference} was approved.",
  denied: "Your refund request {reference} was declined.",
  credited: "Your refund request {reference} was settled with credits added to your balance.",
  other: "Your refund request {reference} was updated.",
};

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
export function notificationText(
  n: AppNotification,
  locale: string,
  text: Translate,
  /** Memberships are on (§25.1): off, a membership's end is reported without inviting a new one. Default on. */
  { memberships = true }: { memberships?: boolean } = {},
): NotificationCopy {
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
    // advisor settings: photo review decisions.
    case "photo.approved":
      return { title: text(T.photoApprovedTitle), body: text(T.photoApprovedBody) };
    case "photo.rejected": {
      const reason = str(d.reason).trim();
      return { title: text(T.photoRejectedTitle), body: reason ? text(T.photoRejectedBody, { reason }) : text(T.photoRejectedNoReason) };
    }
    // support desk: a staff reply to the customer's request (§16.3).
    case "support.replied": {
      const reference = str(d.reference).trim();
      const subject = str(d.subject).trim();
      const body = !reference
        ? ""
        : subject
          ? text(T.supportRepliedBody, { reference, subject })
          : text(T.supportRepliedNoSubject, { reference });
      return { title: text(T.supportRepliedTitle), body };
    }
    case "waitlist.available": {
      const advisor = str(d.psychic_name).trim();
      return {
        title: text(T.waitlistAvailableTitle),
        body: advisor ? text(T.waitlistAvailableBody, { name: advisor }) : text(T.waitlistAvailableNoName),
      };
    }
    case "export.ready":
      return { title: text(T.exportReadyTitle), body: text(T.exportReadyBody) };
    case "account.email_change_requested":
      return { title: text(T.emailChangeRequestedTitle), body: text(T.emailChangeRequestedBody) };
    case "account.email_changed":
      return { title: text(T.emailChangedTitle), body: text(T.emailChangedBody) };
    case "account.reactivated":
      return { title: text(T.reactivatedTitle), body: text(T.reactivatedBody) };
    // `email` is masked by the API ("j•••@gmail.com").
    case "account.google_linked": {
      const email = str(d.email).trim();
      return { title: text(T.googleLinkedTitle), body: email ? text(T.googleLinkedBody, { email }) : text(T.googleLinkedNoEmail) };
    }
    case "account.google_unlinked": {
      const email = str(d.email).trim();
      return { title: text(T.googleUnlinkedTitle), body: email ? text(T.googleUnlinkedBody, { email }) : text(T.googleUnlinkedNoEmail) };
    }
    case "subscription.renewed":
      return { title: text(T.subscriptionRenewedTitle), body: text(T.subscriptionRenewedBody) };
    // revisions 5.1 (§25.5)
    case "subscription.started":
      return { title: text(T.subscriptionStartedTitle), body: text(T.subscriptionStartedBody) };
    case "payment.refunded": {
      const cents = num(d.amount_cents) ?? 0;
      const currency = str(d.currency).trim().toUpperCase();
      const amount =
        cents > 0 && /^[A-Z]{3}$/.test(currency)
          ? new Intl.NumberFormat(locale, { style: "currency", currency }).format(cents / 100)
          : "";
      return { title: text(T.paymentRefundedTitle), body: amount ? text(T.paymentRefundedBody, { amount }) : text(T.paymentRefundedNoAmount) };
    }
    case "subscription.payment_failed": {
      const until = Date.parse(str(d.grace_until));
      const date = Number.isFinite(until) ? new Intl.DateTimeFormat(locale, { dateStyle: "medium" }).format(until) : "";
      return {
        title: text(T.subscriptionPaymentFailedTitle),
        body: date ? text(T.subscriptionPaymentFailedGrace, { date }) : text(T.subscriptionPaymentFailedBody),
      };
    }
    case "subscription.canceled": {
      const copy = memberships ? subscriptionCanceledCopy : subscriptionCanceledPlainCopy;
      const template = copy[str(d.reason)] ?? copy.other;
      return { title: text(T.subscriptionCanceledTitle), body: text(template) };
    }
    case "refund_request.received":
      return { title: text(T.refundReceivedTitle), body: text(T.refundReceivedBody, { reference: str(d.reference) }) };
    case "refund_request.resolved":
      return { title: text(T.refundResolvedTitle), body: text(refundResolvedCopy[str(d.status)] ?? refundResolvedCopy.other, { reference: str(d.reference) }) };
    case "broadcast":
      return { title: pickLocalized(d.title, locale) || text(T.fallbackTitle), body: pickLocalized(d.body, locale) };
    default:
      return { title: text(T.fallbackTitle), body: "" };
  }
}

/** Where a notification leads when the API sent no (usable) link; locale-less app path or null. */
export function notificationFallbackLink(n: AppNotification): string | null {
  switch (n.kind) {
    case "waitlist.available": {
      const slug = str(n.data?.psychic_slug).trim();
      return slug ? `/chatroom?reader=${encodeURIComponent(slug)}` : "/chatroom";
    }
    case "export.ready":
      return "/profile/settings#account-actions";
    case "account.email_change_requested":
    case "account.email_changed":
      return "/profile/settings#account";
    case "account.google_linked":
    case "account.google_unlinked":
      return "/profile/settings#security";
    case "subscription.started":
    case "subscription.renewed":
    case "subscription.payment_failed":
    case "subscription.canceled":
      return "/profile/settings";
    case "refund_request.received":
    case "refund_request.resolved":
    case "payment.refunded":
      return "/profile/settings#billing";
    case "account.reactivated":
      return "/profile";
    default:
      return null;
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

/** The in-app destination of a notification: the API's link, else the kind's default. */
export function notificationDestination(n: AppNotification, locale: string): string | null {
  return notificationHref(n.link, locale) ?? notificationHref(notificationFallbackLink(n) ?? "", locale);
}
