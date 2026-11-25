/**
 * Invite links (growth: referrals). An invite link is any page with
 * `?ref=CODE`. The first valid code seen in this browser is kept for 30 days
 * in localStorage (`staryield.referral`) and sent with the sign-up
 * (`attribution.referral_code`, password and Google sign-ups alike); a later
 * link does not replace it while it is valid. The parameter is removed from
 * the address bar, so a link copied from the page never forwards someone
 * else's code.
 *
 * Privacy: only the code and when it was first seen are stored; analytics
 * record that an invite link was opened, never the code (it identifies the
 * inviter). Storage failures (private mode, blocked site data) are ignored:
 * the invite is best effort and must never break the page.
 */
import type { SiteConfig } from "@/lib/api/types";
import { track } from "@/lib/analytics";

export const REFERRAL_KEY = "staryield.referral";
export const REFERRAL_TTL_MS = 30 * 24 * 60 * 60 * 1000;
export const REFERRAL_PARAM = "ref";
/** 8 characters without 0, O, 1, I, L or U (backend NormalizeReferralCode). */
const CODE = /^[ABCDEFGHJKMNPQRSTVWXYZ2-9]{8}$/;

type Storage = Pick<globalThis.Storage, "getItem" | "setItem" | "removeItem">;

export type StoredReferral = { code: string; first_seen_at: string };

export type ReferralEnv = {
  /** The full landing URL. */
  href: string;
  storage: Storage | null;
  now: number;
  /** Replace the address bar URL without navigating. */
  replaceUrl: (url: string) => void;
  track: typeof track;
};

/** The code in canonical form (trimmed, upper case), or null when it cannot be one. */
export function normalizeReferralCode(raw: string | null | undefined): string | null {
  if (typeof raw !== "string") return null;
  const code = raw.trim().toUpperCase();
  return CODE.test(code) ? code : null;
}

function browserStorage(): Storage | null {
  try {
    return typeof window !== "undefined" ? window.localStorage : null;
  } catch {
    return null;
  }
}

function remove(storage: Storage) {
  try {
    storage.removeItem(REFERRAL_KEY);
  } catch {
    // Nothing more to do.
  }
}

/** The kept invite, or null when missing, malformed or expired (bad entries are removed). */
export function loadReferral(storage: Storage | null, now = Date.now()): StoredReferral | null {
  if (!storage) return null;
  let raw: string | null = null;
  try {
    raw = storage.getItem(REFERRAL_KEY);
  } catch {
    return null;
  }
  if (!raw) return null;
  try {
    const value = JSON.parse(raw) as Partial<StoredReferral> | null;
    const code = normalizeReferralCode(value?.code);
    const seen = typeof value?.first_seen_at === "string" ? Date.parse(value.first_seen_at) : NaN;
    // Expired, or dated in the future (a clock change): start over.
    if (!code || !Number.isFinite(seen) || now - seen > REFERRAL_TTL_MS || seen - now > 60_000) throw new Error("invalid");
    return { code, first_seen_at: value!.first_seen_at! };
  } catch {
    remove(storage);
    return null;
  }
}

/** The URL without `ref` (and only it), or null when there is nothing to strip. */
export function stripReferralParam(href: string): string | null {
  try {
    const url = new URL(href);
    if (!url.searchParams.has(REFERRAL_PARAM)) return null;
    url.searchParams.delete(REFERRAL_PARAM);
    return url.pathname + url.search + url.hash;
  } catch {
    return null;
  }
}

function defaultEnv(): ReferralEnv | null {
  if (typeof window === "undefined") return null;
  return {
    href: window.location.href,
    storage: browserStorage(),
    now: Date.now(),
    replaceUrl: (url) => {
      try {
        // Keep the router's history state so client navigation still works.
        window.history.replaceState(window.history.state, "", url);
      } catch {
        // Some embedded browsers forbid replaceState; the code is harmless in the URL.
      }
    },
    track,
  };
}

/** Event other components listen to when a new invite is kept (the welcome banner). */
export const REFERRAL_STORED_EVENT = "staryield:referral-stored";

/**
 * Read `?ref=` from the landing URL, keep the first valid code, strip the
 * parameter from the address bar and record a `referral_landing` event.
 */
export function captureReferral(env: ReferralEnv | null = defaultEnv()): { landed: boolean; stored: boolean } {
  if (!env) return { landed: false, stored: false };
  let url: URL;
  try {
    url = new URL(env.href);
  } catch {
    return { landed: false, stored: false };
  }
  if (!url.searchParams.has(REFERRAL_PARAM)) return { landed: false, stored: false };
  const code = normalizeReferralCode(url.searchParams.get(REFERRAL_PARAM));
  const stripped = stripReferralParam(env.href);
  if (stripped !== null) env.replaceUrl(stripped);
  if (!code) return { landed: false, stored: false };

  let stored = false;
  if (env.storage && !loadReferral(env.storage, env.now)) {
    try {
      env.storage.setItem(REFERRAL_KEY, JSON.stringify({ code, first_seen_at: new Date(env.now).toISOString() } satisfies StoredReferral));
      stored = true;
    } catch {
      stored = false;
    }
  }
  env.track("referral_landing", { stored });
  if (stored && typeof window !== "undefined") window.dispatchEvent(new Event(REFERRAL_STORED_EVENT));
  return { landed: true, stored };
}

/** The invite code to send with the sign-up, if one is kept. */
export function referralCodeForSignup(now = Date.now()): string | undefined {
  return loadReferral(browserStorage(), now)?.code;
}

/** Whether an invite is kept in this browser (the welcome banner). */
export function hasStoredReferral(now = Date.now()): boolean {
  return loadReferral(browserStorage(), now) !== null;
}

/** Forget the invite once it was used by a sign-up, so a later account in this browser does not reuse it. */
export function forgetReferral() {
  const storage = browserStorage();
  if (storage) remove(storage);
}

/** What the public site configuration says about the program (off when absent or malformed). */
export type PublicReferralProgram = { enabled: boolean; reward_minutes: number; launched: boolean };

export function referralProgramOf(config: SiteConfig | null | undefined): PublicReferralProgram {
  const r = config?.referrals;
  const minutes = typeof r?.reward_minutes === "number" && Number.isFinite(r.reward_minutes) ? Math.max(0, Math.round(r.reward_minutes)) : 0;
  return { enabled: r?.enabled === true && minutes > 0, reward_minutes: minutes, launched: r?.launched === true };
}
