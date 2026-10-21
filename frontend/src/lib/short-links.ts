import { API_URL } from "@/lib/api/config";

/** Short-link codes accepted by the API (§5.2). */
export const SHORT_LINK_CODE = /^[A-Za-z0-9_-]{3,32}$/;

/** Where `/go/:code` redirects: the API's click tracker, or the site root for malformed codes. */
export function shortLinkTarget(code: string, requestUrl: string, apiUrl: string = API_URL): string {
  if (SHORT_LINK_CODE.test(code)) return `${apiUrl}/l/${encodeURIComponent(code)}`;
  return new URL("/", requestUrl).toString();
}
