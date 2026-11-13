import { deflateSync } from "node:zlib";
import { expect, type Browser, type BrowserContext, type Page } from "@playwright/test";

import { purchaseConsent } from "../src/lib/legal/versions";

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
  // Phones show no site header in the chatroom (where advisors land): the chat header is the top bar.
  await expect(page.locator("[data-testid=user-menu-trigger], [data-testid=mobile-menu-trigger], [data-testid=chat-header], main.chat-workspace").filter({ visible: true }).first()).toBeVisible();
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

/**
 * Set the signed-in advisor's availability from the header menu
 * (`availability-menu`), waiting for the API to save it. Does nothing when
 * the advisor already has that availability.
 */
export async function setAvailability(page: Page, value: "online" | "away" | "offline") {
  const trigger = page.getByTestId("availability-menu");
  await expect(trigger).toBeVisible();
  // A menu that is still closing (exit animation) would swallow the click.
  await expect(page.getByRole("menu")).toHaveCount(0);
  // The trigger shows the effective status; "busy" means online with a live chat.
  const current = await trigger.getAttribute("data-status");
  if (current === value || (value === "online" && current === "busy")) return;
  await trigger.click();
  const saved = page.waitForResponse((r) => r.url().endsWith("/api/v1/psychic/availability") && r.request().method() === "PATCH" && r.ok());
  await page.getByTestId(`availability-${value}`).click();
  await saved;
  // Let the menu finish closing, so the next interaction does not race it.
  await expect(page.getByRole("menu")).toHaveCount(0);
  await expect(trigger).toHaveAttribute("data-status", value === "online" ? /online|busy/ : value);
}

/** A short valid mono 16-bit WAV (a quiet 440 Hz tone), generated so no binary fixture is committed. */
export function wavFixture(seconds = 0.5, sampleRate = 8000): Buffer {
  const samples = Math.floor(seconds * sampleRate);
  const data = Buffer.alloc(samples * 2);
  for (let i = 0; i < samples; i++) data.writeInt16LE(Math.round(Math.sin((2 * Math.PI * 440 * i) / sampleRate) * 3000), i * 2);
  const header = Buffer.alloc(44);
  header.write("RIFF", 0, "ascii");
  header.writeUInt32LE(36 + data.length, 4);
  header.write("WAVEfmt ", 8, "ascii");
  header.writeUInt32LE(16, 16);
  header.writeUInt16LE(1, 20); // PCM
  header.writeUInt16LE(1, 22); // mono
  header.writeUInt32LE(sampleRate, 24);
  header.writeUInt32LE(sampleRate * 2, 28);
  header.writeUInt16LE(2, 32);
  header.writeUInt16LE(16, 34);
  header.write("data", 36, "ascii");
  header.writeUInt32LE(data.length, 40);
  return Buffer.concat([header, data]);
}

/** A pack of the signed-in account's top-up catalog (`GET /credits/catalog`, §24.1). */
export type CatalogPack = { id: string; code: string; kind: "standard" | "intro"; name: string; credits: number; price_cents: number };

/** The packs this account can buy now (the eligible Welcome Pack first), and whether it is eligible for it. */
export async function topUpCatalog(page: Page): Promise<{ packs: CatalogPack[]; welcomeEligible: boolean }> {
  const res = await page.context().request.get(`${API_URL}/api/v1/credits/catalog`);
  expect(res.ok(), `credit catalog: ${res.status()} ${await res.text()}`).toBe(true);
  const body = (await res.json()) as { packs: CatalogPack[]; welcome_offer: { eligible: boolean } };
  return { packs: body.packs, welcomeEligible: body.welcome_offer.eligible };
}

/** The signed-in account's credit balance (`GET /wallet`). */
export async function walletBalance(page: Page): Promise<number> {
  const wallet = (await (await page.context().request.get(`${API_URL}/api/v1/wallet`)).json()) as { balance?: number };
  return wallet.balance ?? 0;
}

/**
 * Buy a pack through the API with a sandbox payment (no card: the sandbox
 * charges the default route). Returns what was bought. The Welcome Pack is
 * never bought this way: its redemption is what the top-up specs test.
 */
export async function buyStandardPack(page: Page, pick: (packs: CatalogPack[]) => CatalogPack = (packs) => packs.reduce((a, b) => (b.credits > a.credits ? b : a))) {
  const req = page.context().request;
  const { packs } = await topUpCatalog(page);
  const pack = pick(packs.filter((p) => p.kind === "standard"));
  const { csrf_token } = (await (await req.get(`${API_URL}/api/v1/auth/csrf`)).json()) as { csrf_token: string };
  const res = await req.post(`${API_URL}/api/v1/credits/purchases`, {
    headers: { "X-CSRF-Token": csrf_token },
    // The API refuses a purchase without the consent versions (§25.2): the ones the web checkout sends.
    data: { pack_id: pack.id, idempotency_key: `e2e-credits-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`, consent: purchaseConsent() },
  });
  expect(res.ok(), `credit purchase: ${res.status()} ${await res.text()}`).toBe(true);
  return pack;
}

/**
 * Make sure the client signed in on `page` can pay for live chats. The suite
 * shares one seed client whose credits run out over many chat specs, and a
 * request it cannot pay for never reaches the advisor. Buys the largest
 * standard pack of the top-up catalog (sandbox payments in development) until
 * the balance reaches `min`.
 */
export async function ensureCredits(page: Page, min = 400) {
  for (let attempt = 0; attempt < 5; attempt++) {
    if ((await walletBalance(page)) >= min) return;
    await buyStandardPack(page);
  }
  throw new Error(`could not top the client up to ${min} credits`);
}

const NEW_CLIENT_PASSWORD = "E2e!Secure#2026x";

/**
 * Register a brand-new client through the API and sign in with the login
 * form. Specs that depend on "never bought credits" (the Welcome Pack, the
 * first refund) use their own account, so the shared seed client is never changed.
 */
export async function newClient(page: Page, tag = "client"): Promise<{ email: string; password: string }> {
  const email = `e2e+${tag}-${Date.now()}-${Math.random().toString(36).slice(2, 7)}@staryield.test`;
  const csrf = ((await (await page.request.get(`${API_URL}/api/v1/auth/csrf`)).json()) as { csrf_token: string }).csrf_token;
  const res = await page.request.post(`${API_URL}/api/v1/auth/register`, {
    headers: { "X-CSRF-Token": csrf, "Content-Type": "application/json" },
    data: { email, password: NEW_CLIENT_PASSWORD, accept_terms: true, display_name: "Top-up Tester", birth_date: "1990-03-14" },
  });
  expect(res.status(), await res.text()).toBe(201);
  await page.request.post(`${API_URL}/api/v1/auth/logout`, { headers: { "X-CSRF-Token": csrf } });
  await signIn(page, email, NEW_CLIENT_PASSWORD);
  return { email, password: NEW_CLIENT_PASSWORD };
}

// revisions 5.0: the Welcome Pack is limited to one per payment method (§24.2), so specs that buy
// it need a card no earlier run used. The sandbox fingerprints brand + last 4 + expiry.
/** A Luhn-valid Visa test number with random last digits and a random future expiry ("MMYY"). */
export function uniqueSandboxCard(): { number: string; expiry: string } {
  for (;;) {
    const body = "424242424242" + String(Math.floor(Math.random() * 1000)).padStart(3, "0");
    let sum = 0;
    for (let i = 0; i < body.length; i++) {
      let d = Number(body[body.length - 1 - i]);
      if (i % 2 === 0) d = d * 2 > 9 ? d * 2 - 9 : d * 2;
      sum += d;
    }
    const number = body + String((10 - (sum % 10)) % 10);
    // Last 4 "0002" (declined) and "9995" (soft decline) are the sandbox's failure cards.
    if (number.endsWith("0002") || number.endsWith("9995")) continue;
    const month = String(1 + Math.floor(Math.random() * 12)).padStart(2, "0");
    const year = String(30 + Math.floor(Math.random() * 10));
    return { number, expiry: month + year };
  }
}
