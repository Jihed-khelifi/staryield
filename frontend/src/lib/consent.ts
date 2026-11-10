/**
 * The `sy_consent` cookie (listed in the Privacy Notice): this browser's copy
 * of the signed-in user's privacy choices, written whenever they are loaded
 * or saved in Settings, so client-side analytics honour an explicit refusal
 * at once. Without the cookie the existing defaults apply (Do Not Track and
 * Global Privacy Control are always honoured by `track()`).
 */

export const CONSENT_COOKIE = "sy_consent";

export type ConsentChoices = { analytics: boolean; marketing_attribution: boolean; personalization: boolean };

/** Serialize choices: "v=2026-11&a=0&m=0&p=1". */
export function encodeConsent(choices: ConsentChoices, version: string): string {
  const flag = (on: boolean) => (on ? "1" : "0");
  return `v=${version}&a=${flag(choices.analytics)}&m=${flag(choices.marketing_attribution)}&p=${flag(choices.personalization)}`;
}

/** Parse the cookie value (null when absent or malformed). */
export function decodeConsent(value: string | null | undefined): (ConsentChoices & { version: string }) | null {
  if (!value) return null;
  const params = new URLSearchParams(value);
  const read = (key: string) => {
    const v = params.get(key);
    return v === "1" ? true : v === "0" ? false : null;
  };
  const analytics = read("a");
  const marketing = read("m");
  const personalization = read("p");
  const version = params.get("v") ?? "";
  if (analytics === null || marketing === null || personalization === null || !/^[\w.-]{1,32}$/.test(version)) return null;
  return { analytics, marketing_attribution: marketing, personalization, version };
}

/** Read the choices from `document.cookie` (or a cookie header). */
export function readConsentCookie(cookie: string = typeof document === "undefined" ? "" : document.cookie) {
  const match = cookie.match(/(?:^|;\s*)sy_consent=([^;]*)/);
  if (!match) return null;
  try {
    return decodeConsent(decodeURIComponent(match[1]));
  } catch {
    return null;
  }
}

/** Remember the choices for a year (Lax, Secure on HTTPS). */
export function writeConsentCookie(choices: ConsentChoices, version: string) {
  if (typeof document === "undefined") return;
  const secure = window.location.protocol === "https:" ? "; secure" : "";
  document.cookie = `${CONSENT_COOKIE}=${encodeURIComponent(encodeConsent(choices, version))}; path=/; max-age=31536000; samesite=lax${secure}`;
}

/** True when the user explicitly turned analytics off in this browser. */
export function analyticsRefused(): boolean {
  const choices = readConsentCookie();
  return choices !== null && !choices.analytics;
}

/**
 * True when the browser asks not to be tracked: Global Privacy Control or Do
 * Not Track. Shared by analytics and by third-party embeds (one definition).
 */
export function browserPrivacySignal(): boolean {
  if (typeof navigator === "undefined") return false;
  const gpc = (navigator as Navigator & { globalPrivacyControl?: boolean }).globalPrivacyControl;
  return gpc === true || navigator.doNotTrack === "1";
}

/**
 * Whether a third-party embed (the Trustpilot widget) may be loaded. There is
 * no anonymous consent banner, so the only suppression is the browser's own
 * signal; a plain link is shown instead (Privacy Policy, section 4).
 */
export function thirdPartyEmbedsAllowed(): boolean {
  return typeof navigator !== "undefined" && !browserPrivacySignal();
}
