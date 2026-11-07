/**
 * Requester side of the support desk (docs/platform-extensions.md §16.2–16.3):
 * references, the private link token, status copy and error mapping.
 * Everything here is pure (or takes its environment), so it is unit-tested.
 */
import type { CustomerTicket, TicketStatus } from "@/lib/api/types";
import { toApiError } from "@/lib/api/errors";

/** Longest reply the API accepts (characters). */
export const REPLY_MAX = 5000;

/** A reference such as SY-7K2M9Q. */
export const SUPPORT_REFERENCE = /^SY-[A-Z0-9]{4,16}$/;

/** Upper-cased reference, or null when it is not one. */
export function normalizeReference(raw: string): string | null {
  const value = raw.trim().toUpperCase();
  return SUPPORT_REFERENCE.test(value) ? value : null;
}

/** Tokens are 32 random bytes, base64url (43 characters); accept a sane range. */
export const SUPPORT_TOKEN = /^[A-Za-z0-9_-]{16,128}$/;

export const supportTokenKey = (reference: string) => `staryield.support.${reference}`;

type TokenEnv = {
  hash: string;
  pathname: string;
  search: string;
  storage: Pick<Storage, "getItem" | "setItem" | "removeItem"> | null;
  replaceUrl: (url: string) => void;
};

/**
 * The private-link token for a request. It is read from the `#t=` fragment
 * (fragments never reach servers or referrers), removed from the address bar
 * at once and kept in sessionStorage for this tab only; a reload then still
 * works. Without a fragment the tab's stored copy is used. The token is never
 * put in a URL or a request body: callers send it as `X-Support-Token`.
 */
export function takeSupportToken(reference: string, env: TokenEnv): string | null {
  let token: string | null = null;
  if (env.hash.length > 1) {
    const fromFragment = new URLSearchParams(env.hash.slice(1)).get("t");
    // Strip the whole fragment, valid or not: nothing else uses it on this page.
    env.replaceUrl(env.pathname + env.search);
    if (fromFragment && SUPPORT_TOKEN.test(fromFragment)) {
      token = fromFragment;
      try {
        env.storage?.setItem(supportTokenKey(reference), token);
      } catch {
        // Storage is optional (private mode): the token stays in memory.
      }
    }
  }
  if (token) return token;
  try {
    const stored = env.storage?.getItem(supportTokenKey(reference)) ?? null;
    return stored && SUPPORT_TOKEN.test(stored) ? stored : null;
  } catch {
    return null;
  }
}

/** Browser environment for `takeSupportToken`. */
export function browserTokenEnv(): TokenEnv {
  let storage: TokenEnv["storage"] = null;
  try {
    storage = window.sessionStorage;
  } catch {
    storage = null;
  }
  return {
    hash: window.location.hash,
    pathname: window.location.pathname,
    search: window.location.search,
    storage,
    replaceUrl: (url) => {
      try {
        window.history.replaceState(window.history.state, "", url);
      } catch {
        // Some embedded browsers forbid replaceState.
      }
    },
  };
}

export function forgetSupportToken(reference: string, storage: TokenEnv["storage"] = browserTokenEnv().storage) {
  try {
    storage?.removeItem(supportTokenKey(reference));
  } catch {
    // Nothing more to do.
  }
}

/** Status chip copy (English source, translated by the caller). */
export const ticketStatusCopy: Record<TicketStatus, string> = {
  open: "Awaiting support",
  pending: "Awaiting your reply",
  resolved: "Resolved",
  closed: "Closed",
};

/** A ticket as the closed state after the server refused a reply. */
export function markClosed(ticket: CustomerTicket): CustomerTicket {
  return { ...ticket, status: "closed", awaiting: "none", can_reply: false };
}

export type SupportErrorKind =
  | "closed" // 409 invalid_state: no more replies
  | "link_invalid" // anonymous link: missing, wrong or expired token, unknown reference
  | "not_found" // signed-in: not your request
  | "rate_limited"
  | "sign_in"
  | "other";

/**
 * How a support failure is shown. `context` is "link" for the anonymous
 * private link (every 404 is "expired or invalid") and "account" for the
 * signed-in pages.
 */
export function classifySupportError(error: unknown, context: "link" | "account"): { kind: SupportErrorKind; retryAfter?: number } {
  const e = toApiError(error);
  switch (e.code) {
    case "invalid_state":
      return { kind: "closed" };
    case "not_found":
      return { kind: context === "link" ? "link_invalid" : "not_found" };
    case "rate_limited":
      return { kind: "rate_limited", retryAfter: e.retryAfter && e.retryAfter > 0 ? Math.ceil(e.retryAfter) : undefined };
    case "unauthenticated":
    case "session_expired":
      return { kind: context === "account" ? "sign_in" : "other" };
    default:
      return { kind: "other" };
  }
}

/** "about 2 minutes" style wait for the rate-limit hint, in seconds or minutes. */
export function waitHint(seconds: number | undefined): { unit: "seconds" | "minutes" | "unknown"; value: number } {
  if (!seconds) return { unit: "unknown", value: 0 };
  if (seconds < 90) return { unit: "seconds", value: seconds };
  return { unit: "minutes", value: Math.ceil(seconds / 60) };
}
