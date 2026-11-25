/**
 * Conversion follow-ups, slice E (docs/platform-extensions.md §27.1): the FAQ page answers its questions. Every
 * question of the four support groups opens on a real answer (no "contact our support team" placeholder), the
 * "Free minutes, invites and your account" group is there, and the FAQPage structured data has one Question per
 * answered item.
 */
import { expect, test, type Page } from "@playwright/test";

import { expectNoHorizontalScroll } from "./helpers";

type Question = { "@type": string; name: string; acceptedAnswer: { text: string } };

async function faqQuestions(page: Page): Promise<Question[]> {
  const data = await page.evaluate(() =>
    [...document.querySelectorAll('script[type="application/ld+json"]')].flatMap((script) => JSON.parse(script.textContent ?? "null")),
  );
  const faq = data.find((item: { "@type": string }) => item["@type"] === "FAQPage");
  return faq?.mainEntity ?? [];
}

test.describe("FAQ answers", () => {
  test("How to reset password? opens on an answer about the reset link", async ({ page }) => {
    await page.goto("/en/faq");
    const item = page.locator("details.faq-item", { hasText: "How to reset staryield Website account's password?" });
    await item.locator("summary").click();
    await expect(item).toHaveAttribute("open", "");
    const answer = item.locator("summary + p");
    await expect(answer).toContainText("reset link");
    await expect(answer).toContainText("works once");
    await expect(answer.getByRole("link", { name: "Log In" })).toHaveAttribute("href", "/en/login");
  });

  test("no question is left with the support-team placeholder", async ({ page }) => {
    await page.goto("/en/faq");
    await expect(page.locator("main")).not.toContainText("For help with this question");
    const items = page.locator("details.faq-item");
    expect(await items.count()).toBeGreaterThanOrEqual(26);
    for (const text of await items.locator("summary + p").allTextContents()) expect(text.trim().length).toBeGreaterThan(80);
  });

  test("the billing questions expand too, and the refund answer links the Refund Policy", async ({ page }) => {
    await page.goto("/en/faq");
    const item = page.locator("details.faq-item", { hasText: "How can I receive a refund or compensation for STARYIELD?" });
    await item.locator("summary").click();
    await expect(item.locator("summary + p")).toBeVisible();
    await expect(item.getByRole("link", { name: "Refund Policy" })).toHaveAttribute("href", "/en/refund-policy");
  });

  test("the group on free minutes, invites and your account answers its four topics", async ({ page }) => {
    await page.goto("/en/faq");
    const group = page.locator("#account-and-offers");
    await expect(group.getByRole("heading", { name: "Free minutes, invites and your account" })).toBeVisible();
    for (const [question, phrase] of [
      ["How many free minutes do I get when I sign up?", "without a payment method"],
      ["How do I earn free minutes by inviting friends?", "confirms their e-mail address"],
      ["Do I have to confirm my e-mail address?", "Not to use Staryield or to buy credits"],
      ["Are the calculators free?", "need no account"],
    ]) {
      const item = group.locator("details.faq-item", { hasText: question });
      await item.locator("summary").click();
      await expect(item.locator("summary + p")).toContainText(phrase);
    }
  });

  test("the FAQPage JSON-LD parses, has at least 26 questions and each is on the page", async ({ page }) => {
    await page.goto("/en/faq");
    const questions = await faqQuestions(page);
    expect(questions.length).toBeGreaterThanOrEqual(26);
    expect(questions.every((question) => question["@type"] === "Question" && question.acceptedAnswer.text.length > 80)).toBe(true);
    expect(new Set(questions.map((question) => question.name)).size).toBe(questions.length);
    for (const question of questions) await expect(page.getByText(question.name, { exact: true }).first()).toBeAttached();
    const reset = questions.find((question) => question.name.includes("reset staryield Website account's password"));
    expect(reset?.acceptedAnswer.text).toContain("reset link");
    expect(reset?.acceptedAnswer.text).not.toContain("](");
  });

  for (const locale of ["fr", "es"]) {
    test(`${locale}: the questions are answered in the page language`, async ({ page }) => {
      await page.goto(`/${locale}/faq`);
      const questions = await faqQuestions(page);
      expect(questions.length).toBeGreaterThanOrEqual(26);
      await expect(page.locator("main")).not.toContainText(/For help with this question|contact our support team/i);
      // The first question is translated, not the English source text.
      expect(questions[0].name).not.toBe("Why is my astrological info wrong on STARYIELD Website?");
    });
  }

  test("@responsive a phone shows the answers without scrolling sideways", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 800 });
    await page.goto("/en/faq");
    for (const item of await page.locator("details.faq-item").all()) await item.locator("summary").click();
    await expectNoHorizontalScroll(page);
  });
});
