/**
 * Short links: `https://staryield.net/go/:code` → `${API}/l/:code`
 * (docs/platform-extensions.md §5.3). The API records the click and
 * redirects to the campaign destination with `sy_click`; unknown codes are
 * sent home by the API. Malformed codes never leave the site.
 *
 * Excluded from the locale proxy (src/proxy.ts) so the path is not prefixed.
 */
import { shortLinkTarget } from "@/lib/short-links";

export async function GET(request: Request, ctx: { params: Promise<{ code: string }> }) {
  const { code } = await ctx.params;
  return new Response(null, {
    status: 302,
    headers: {
      Location: shortLinkTarget(code, request.url),
      // Every visit must reach the API so clicks are counted.
      "Cache-Control": "no-store",
    },
  });
}
