import { deflateSync } from "node:zlib";
import { expect, type Browser, type BrowserContext, type Page } from "@playwright/test";

/** API origin of the stack under test. */
export const API_URL = (process.env.E2E_API_URL ?? "http://localhost:8080").replace(/\/+$/, "");

/** Seed accounts (backend/internal/seed). */
export const CLIENT = { email: "client@staryield.test", password: "Cl1ent!Secure#2026", name: "client" };
export const PSYCHIC_PASSWORD = "Psych1c!Secure#2026";
export const advisor = (slug: "ramone" | "theo" | "solomon") => ({ slug, email: `${slug}@advisors.staryield.net`, password: PSYCHIC_PASSWORD });

/** Sign in through the login form and wait for the post-login page. */
export async function signIn(page: Page, email: string, password: string, locale = "en") {
  await page.goto(`/${locale}/login`);
  await page.locator("input[name=email]").fill(email);
  await page.locator("input[name=password]").fill(password);
  await Promise.all([page.waitForURL((url) => !url.pathname.endsWith("/login")), page.locator("form button[type=submit]").click()]);
  await expect(accountTrigger(page)).toBeVisible();
}

/** The account menu button (header avatar on desktop, menu sheet trigger on phones). */
export function accountTrigger(page: Page) {
  return page.locator("[data-testid=user-menu-trigger], [data-testid=mobile-menu-trigger]").filter({ visible: true }).first();
}

/** A fresh context + page signed in as the given account. */
export async function signedInPage(browser: Browser, email: string, password: string, viewport = { width: 1440, height: 900 }) {
  const context: BrowserContext = await browser.newContext({ viewport, locale: "en-US", timezoneId: "Europe/Paris" });
  const page = await context.newPage();
  await signIn(page, email, password);
  return { context, page };
}

/** The page never scrolls horizontally. */
export async function expectNoHorizontalScroll(page: Page) {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow).toBeLessThanOrEqual(1);
}

/** A small valid PNG (gradient), generated so no binary fixture is committed. */
export function pngFixture(width = 48, height = 32): Buffer {
  const table = Array.from({ length: 256 }, (_, n) => {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    return c >>> 0;
  });
  const crc = (buf: Buffer) => {
    let c = 0xffffffff;
    for (const b of buf) c = table[(c ^ b) & 0xff] ^ (c >>> 8);
    return (c ^ 0xffffffff) >>> 0;
  };
  const chunk = (type: string, data: Buffer) => {
    const len = Buffer.alloc(4);
    len.writeUInt32BE(data.length);
    const body = Buffer.concat([Buffer.from(type, "ascii"), data]);
    const sum = Buffer.alloc(4);
    sum.writeUInt32BE(crc(body));
    return Buffer.concat([len, body, sum]);
  };
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width, 0);
  header.writeUInt32BE(height, 4);
  header[8] = 8; // bit depth
  header[9] = 2; // truecolour
  const raw = Buffer.alloc((width * 3 + 1) * height);
  for (let y = 0; y < height; y++) {
    const row = y * (width * 3 + 1);
    for (let x = 0; x < width; x++) {
      raw[row + 1 + x * 3] = 194 - x;
      raw[row + 2 + x * 3] = 154 - y;
      raw[row + 3 + x * 3] = 61 + x;
    }
  }
  return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]), chunk("IHDR", header), chunk("IDAT", deflateSync(raw)), chunk("IEND", Buffer.alloc(0))]);
}

/** Wait until an <img> inside the locator has decoded (blob URL loaded). */
export async function expectImageLoaded(page: Page, selector: string) {
  await expect
    .poll(async () => page.locator(selector).evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0).catch(() => false))
    .toBe(true);
}
