import { expect, test } from "@playwright/test";

/**
 * Revisions 5.1 (docs/platform-extensions.md §25.6): /refunds, /refund, /reimburse and /refunds-policy, with or without a
 * locale prefix, redirect permanently (308) to /{locale}/refund-policy. A bare path uses the locale the app would pick
 * (cookie, then Accept-Language, then English). The redirect is made by src/proxy.ts (one hop) with a screen-router fallback.
 */
const ALIASES = ["refunds", "refund", "reimburse", "refunds-policy"] as const;
const LOCALES = ["en", "fr", "es"] as const;

const hop = async (request: import("@playwright/test").APIRequestContext, path: string, headers: Record<string, string> = {}) => {
  const response = await request.get(path, { maxRedirects: 0, headers });
  return { status: response.status(), location: response.headers()["location"] ?? "" };
};
const pathOf = (location: string) => new URL(location, "http://localhost").pathname;

test.describe("refund policy redirects", () => {
  for (const alias of ALIASES) {
    for (const locale of LOCALES)
      test(`/${locale}/${alias} redirects permanently to /${locale}/refund-policy`, async ({ request }) => {
        const result = await hop(request, `/${locale}/${alias}`);
        expect(result.status).toBe(308);
        expect(pathOf(result.location)).toBe(`/${locale}/refund-policy`);
      });

    test(`/${alias} (no locale) uses the English default and goes straight to /en/refund-policy`, async ({ request }) => {
      const result = await hop(request, `/${alias}`);
      expect(result.status).toBe(308);
      expect(pathOf(result.location)).toBe("/en/refund-policy");
    });

    test(`/${alias} (no locale) follows the locale cookie and Accept-Language`, async ({ request }) => {
      const cookie = await hop(request, `/${alias}`, { cookie: "STARYIELD_LOCALE=fr" });
      expect(cookie.status).toBe(308);
      expect(pathOf(cookie.location)).toBe("/fr/refund-policy");
      const header = await hop(request, `/${alias}`, { "accept-language": "es-ES,es;q=0.9" });
      expect(header.status).toBe(308);
      expect(pathOf(header.location)).toBe("/es/refund-policy");
    });
  }

  test("a different case redirects in one hop; a trailing slash first loses its slash (Next.js) and then redirects", async ({ request, page }) => {
    for (const path of ["/en/Refunds", "/REFUND"]) {
      const result = await hop(request, path);
      expect(result.status, path).toBe(308);
      expect(pathOf(result.location), path).toBe("/en/refund-policy");
    }
    for (const path of ["/en/refunds/", "/refund/"]) {
      await page.goto(path);
      await expect(page, path).toHaveURL(/\/en\/refund-policy$/);
    }
  });

  test("the real page is not redirected and unknown siblings are not captured", async ({ request }) => {
    expect((await request.get("/en/refund-policy", { maxRedirects: 0 })).status()).toBe(200);
    for (const path of ["/en/refunded", "/en/refunds/extra"]) expect((await request.get(path, { maxRedirects: 0 })).status(), path).toBe(404);
  });

  for (const alias of ["refunds", "refund", "reimburse", "refunds-policy"])
    test(`@responsive browsing to /${alias} lands on the Refund Policy page`, async ({ page }) => {
      await page.goto(`/${alias}`);
      await expect(page).toHaveURL(/\/en\/refund-policy$/);
      await expect(page.getByRole("heading", { level: 1 })).toHaveText("Refund & Cancellation Policy");
    });

  test("with a locale prefix the browser lands on that locale's policy", async ({ page }) => {
    await page.goto("/fr/reimburse");
    await expect(page).toHaveURL(/\/fr\/refund-policy$/);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText("Politique de remboursement et d’annulation");
  });
});
