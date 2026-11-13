import { expect, test, type Page } from "@playwright/test";

import { expectNoA11yViolations } from "./a11y";
import { expectNoHorizontalScroll } from "./helpers";

/**
 * Revisions 5.1 (docs/platform-extensions.md §25): what a payment processor's underwriters read on the public site.
 * Company and customer-service details in the footer and on the Contact page, the card marks, the "entertainment purposes
 * only" statements, and the legal documents without memberships.
 */
const LOCALES = ["en", "fr", "es"] as const;
const WIDTHS = [320, 360, 390, 768, 1024, 1440];
const HOURS = {
  en: "Support is available 24/7. We reply within 1–2 days.",
  fr: "L’assistance est disponible 24/7. Nous répondons sous 1 à 2 jours.",
  es: "El soporte está disponible 24/7. Respondemos en un plazo de 1 a 2 días.",
};
const TAGLINE = {
  en: "Astrology and psychic readings for entertainment purposes only.",
  fr: "Astrologie et consultations de voyance à des fins de divertissement uniquement.",
  es: "Astrología y consultas psíquicas únicamente con fines de entretenimiento.",
};
const COPYRIGHT = {
  en: "© 2026 Staryield Media LLC. All rights reserved. For entertainment purposes only.",
  fr: "© 2026 Staryield Media LLC. Tous droits réservés. À des fins de divertissement uniquement.",
  es: "© 2026 Staryield Media LLC. Todos los derechos reservados. Únicamente con fines de entretenimiento.",
};
const ADDRESS = "30 N Gould St, Ste N, Sheridan, WY 82801, United States";

async function expectFooterFacts(page: Page, locale: (typeof LOCALES)[number]) {
  const footer = page.locator("footer.site-footer");
  await expect(footer.getByTestId("footer-company-name")).toHaveText("Staryield Media LLC");
  await expect(footer.getByTestId("footer-company-address")).toHaveText(ADDRESS);
  await expect(footer.getByTestId("footer-support-email")).toHaveAttribute("href", "mailto:support@staryield.net");
  await expect(footer.getByTestId("footer-support-hours")).toHaveText(HOURS[locale]);
  await expect(footer).toContainText(TAGLINE[locale]);
  await expect(footer).toContainText(COPYRIGHT[locale]);
  await expect(footer.getByTestId("accepted-cards").getByRole("img")).toHaveCount(2);
  await expect(footer.getByTestId("footer-age-line")).toBeVisible();
}

test.describe("footer: company and customer-service details", () => {
  for (const locale of LOCALES)
    test(`@responsive ${locale}: legal name, address, support e-mail, hours, card marks and entertainment statements`, async ({ page }) => {
      await page.goto(`/${locale}`);
      await expectFooterFacts(page, locale);
    });

  test("the same footer is on the legal, pricing, contact, about, FAQ and reviews pages", async ({ page }) => {
    for (const path of ["terms", "privacy", "refund-policy", "pricing", "contact", "about", "faq", "reviews", "standards", "disclaimer"]) {
      await page.goto(`/en/${path}`);
      await expectFooterFacts(page, "en");
    }
  });

  test("@responsive the footer has no horizontal overflow and its marks and address stay inside it from 320 px", async ({ page }) => {
    for (const width of WIDTHS) {
      await page.setViewportSize({ width, height: 800 });
      await page.goto("/en/contact");
      await expectNoHorizontalScroll(page);
      const footer = page.locator("footer.site-footer");
      const box = (await footer.boundingBox())!;
      for (const id of ["footer-company-address", "footer-support-email", "footer-support-hours", "accepted-cards"]) {
        const target = (await footer.getByTestId(id).boundingBox())!;
        expect(target.x, `${id} @${width}`).toBeGreaterThanOrEqual(box.x - 1);
        expect(target.x + target.width, `${id} @${width}`).toBeLessThanOrEqual(box.x + box.width + 1);
      }
    }
  });

  test("the footer has no accessibility violations", async ({ page }) => {
    await page.goto("/en/about");
    await expectNoA11yViolations(page, ["footer.site-footer"]);
  });
});

test.describe("contact page: customer-service details", () => {
  for (const locale of LOCALES)
    test(`@responsive ${locale}: e-mail, hours and reply time, legal name and postal address sit next to the form`, async ({ page }) => {
      await page.goto(`/${locale}/contact`);
      const details = page.getByTestId("contact-details");
      await expect(details).toBeVisible();
      await expect(details.getByTestId("contact-support-email")).toHaveAttribute("href", "mailto:support@staryield.net");
      await expect(details.getByTestId("contact-support-hours")).toHaveText(HOURS[locale]);
      await expect(details.getByTestId("contact-company-name")).toHaveText("Staryield Media LLC");
      await expect(details.getByTestId("contact-company-address")).toHaveText(ADDRESS);
      await expect(page.getByTestId("contact-form")).toBeVisible();
      await expectNoHorizontalScroll(page);
    });

  test("the details block has no accessibility violations", async ({ page }) => {
    await page.goto("/en/contact");
    await expectNoA11yViolations(page, ['[data-testid="contact-details"]']);
  });
});

test.describe("legal documents without memberships", () => {
  test("Terms: section 6 is about credits, the withdrawal and refunds anchors stay, no subscription anchor", async ({ page }) => {
    await page.goto("/en/terms");
    const root = page.locator("[data-legal-document]");
    await expect(root.locator("#credits-payments-and-refunds")).toBeVisible();
    for (const id of ["right-of-withdrawal", "refunds", "chargebacks-and-payment-disputes", "free-trials-and-promotional-offers"]) await expect(root.locator(`[id="${id}"]`)).toHaveCount(1);
    await expect(root.locator("#subscription-terms")).toHaveCount(0);
    await expect(root).not.toContainText(/Premium Membership|auto-renew|15 business days/i);
    await expect(root).toContainText("We do not currently offer subscriptions, memberships or automatic top-ups: every purchase is a one-time credit pack.");
    await expect(root.locator("#right-of-withdrawal")).toContainText("Right of Withdrawal");
    // The Refund Policy links to the withdrawal clause.
    await page.goto("/en/refund-policy#refund-right-of-withdrawal");
    await expect(page.locator("[data-legal-document=refund]").getByRole("link", { name: "Right of Withdrawal in the Terms of Use" })).toHaveAttribute("href", "/en/terms#right-of-withdrawal");
  });

  for (const locale of LOCALES)
    test(`${locale}: support e-mail and hours in Terms 16.1, Privacy 14 and the Refund Policy contact section`, async ({ page }) => {
      for (const doc of ["terms", "privacy"] as const) {
        await page.goto(`/${locale}/${doc}`);
        // The e-mail link and the hours sit in the same paragraph as the existing Support center / form links.
        const paragraph = page.locator("[data-legal-document] p", { hasText: HOURS[locale] }).first();
        await expect(paragraph).toBeVisible();
        await expect(paragraph.getByRole("link", { name: "support@staryield.net" })).toHaveAttribute("href", "mailto:support@staryield.net");
        await expect(paragraph.locator("a[href*='/contact']").first()).toBeVisible();
      }
      await page.goto(`/${locale}/refund-policy`);
      const refund = page.locator("[data-legal-document=refund]");
      await expect(refund.getByText(HOURS[locale])).toBeVisible();
      await expect(refund.getByRole("link", { name: "support@staryield.net" }).last()).toHaveAttribute("href", "mailto:support@staryield.net");
    });

  test("Privacy discloses the purchase consent record and the payment-method data", async ({ page }) => {
    await page.goto("/en/privacy");
    const root = page.locator("[data-legal-document]");
    await expect(root).toContainText("Purchase consent record.");
    await expect(root).toContainText("one-way hash of your IP address");
    await expect(root).toContainText("Payment-method data and card-network rules.");
    await expect(root).not.toContainText(/Premium Membership/i);
  });

  test("the documents are dated 13.11.2026 / 2026-11-13 in the header", async ({ page }) => {
    for (const doc of ["terms", "privacy"]) {
      await page.goto(`/en/${doc}`);
      await expect(page.locator("[data-legal-document] header time").first()).toHaveAttribute("datetime", "2026-11-13");
    }
  });
});

test.describe("public pages: entertainment-only wording", () => {
  const CLAIMS = /certified|verified master|vetted|accurate|guarantee|therapy|healing|karmic debt|audit|top-rated|10K\+|Reynolds|Thayna|Surinder|Mariana|competitors|premium membership|web subscription/i;
  for (const path of ["", "about", "reviews", "standards", "faq", "contact", "pricing"])
    test(`/en/${path || "(home)"} makes no certification, accuracy, healing, statistic or testimonial claim`, async ({ page }) => {
      await page.goto(`/en/${path}`);
      const text = ((await page.locator("main").innerText()) as string)
        .replace(/We do not certify or verify any advisor's credentials, abilities or predictions\./g, "")
        .replace(/we do not certify or verify advisors' abilities/g, "");
      expect(text.match(CLAIMS)).toBeNull();
    });

  test("the home page and the reviews page keep the Trustpilot widget and show no hard-coded figures", async ({ page }) => {
    for (const path of ["", "reviews"]) {
      await page.goto(`/en/${path}`);
      await expect(page.locator("main .trustpilot-widget, main [data-testid=trustpilot-widget]").first()).toBeAttached();
    }
    await page.goto("/en/reviews");
    await expect(page.locator("main")).not.toContainText(/17,419|4\.4 rating|1000|millions/);
  });
});
