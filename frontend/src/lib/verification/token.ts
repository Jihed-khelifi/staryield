/**
 * The verification link carries its token in the URL fragment
 * (`/{locale}/verify-email#token=...`): fragments are never sent to a server,
 * written to an access log or forwarded in a Referer header.
 */

/** Longest token we accept (the API's own bound). */
const MAX_TOKEN_LENGTH = 128;

/** Read the token from a `location.hash` value; null when missing, malformed or too long. */
export function tokenFromHash(hash: string): string | null {
  const raw = new URLSearchParams(hash.startsWith("#") ? hash.slice(1) : hash).get("token");
  return raw && raw.length <= MAX_TOKEN_LENGTH ? raw : null;
}

/** Read the token once and remove the fragment from the address bar (history entry included). */
export function takeVerificationToken(): string | null {
  const token = tokenFromHash(window.location.hash);
  if (window.location.hash) window.history.replaceState(window.history.state, "", window.location.pathname + window.location.search);
  return token;
}
