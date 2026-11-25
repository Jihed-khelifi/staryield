/**
 * Revisions 6.0, §26.2 and §26.3 (docs/platform-extensions.md): the advisor
 * card layout on phones and the order of the directory and home cards.
 *
 * Card layout. On a phone the text column used to overflow (status pill,
 * skills, price and headline cut off at the right edge), worse with Android's
 * larger-text setting. Every card is audited at 320, 360, 390 and 412 px with
 * text at 100 %, 130 % and 200 %.
 *
 * Text scaling. Android's "font size" setting multiplies the size of text only;
 * widths, paddings and every box that is not text keep their CSS pixels. That
 * is what `scaleText` reproduces: it reads each element's computed font size
 * (and pixel line height) inside the cards, then writes it back multiplied by
 * the factor. Alternatives that were rejected: CSS `zoom` and Chromium's
 * `--force-device-scale-factor` enlarge boxes as well as text (and change the
 * viewport in CSS pixels), so they hide exactly the overflow this test is for;
 * a stylesheet that scales `html { font-size }` only reaches rem-based sizes,
 * while the cards use pixel sizes.
 *
 * Order. Advisors online, away, busy, offline; a live status change re-sorts
 * the cards on screen and the page refetches at most every 5 seconds.
 */
import { expect, test, type Browser, type Page } from "@playwright/test";

import { advisor, CLIENT, PSYCHIC_PASSWORD, setAvailability, signedInPage, signIn } from "./helpers";

const PHONE_WIDTHS = [320, 360, 390, 412];
const TEXT_SCALES = [1, 1.3, 2];

/** Multiplies the text size inside `selector` (see the file comment). */
async function scaleText(page: Page, selector: string, factor: number) {
  await page.evaluate(
    ([sel, scale]) => {
      const roots = [...document.querySelectorAll<HTMLElement>(sel)];
      const els = roots.flatMap((root) => [root, ...root.querySelectorAll<HTMLElement>("*")]);
      // Read everything first, then write: a write must not change a later read.
      const sizes = els.map((el) => {
        const cs = getComputedStyle(el);
        return { el, font: parseFloat(cs.fontSize), line: cs.lineHeight.endsWith("px") ? parseFloat(cs.lineHeight) : null };
      });
      for (const { el, font, line } of sizes) {
        el.style.setProperty("font-size", `${font * scale}px`, "important");
        if (line !== null) el.style.setProperty("line-height", `${line * scale}px`, "important");
      }
    },
    [selector, factor] as const,
  );
}

/**
 * Layout findings of every advisor card (`article.advisor-card`): the card's
 * parts lie inside the card, nothing overflows horizontally, the image column
 * is kept (phones) and the headline is clamped to two lines.
 */
async function auditAdvisorCards(page: Page, opts: { phone: boolean }): Promise<string[]> {
  return page.evaluate(({ phone }) => {
    const out: string[] = [];
    const root = document.documentElement;
    if (root.scrollWidth > root.clientWidth) out.push(`page scrolls horizontally (${root.scrollWidth} > ${root.clientWidth})`);
    const cards = [...document.querySelectorAll<HTMLElement>("article.advisor-card")];
    if (cards.length === 0) out.push("no advisor cards on the page");
    const inside = (outer: DOMRect, r: DOMRect) => r.left >= outer.left - 0.5 && r.right <= outer.right + 0.5;
    for (const card of cards) {
      const name = card.querySelector("h2")?.textContent?.split(",")[0] ?? "?";
      const box = card.getBoundingClientRect();
      if (card.scrollWidth > card.clientWidth + 1) out.push(`${name}: card content wider than the card (${card.scrollWidth} > ${card.clientWidth})`);
      if (box.right > window.innerWidth + 0.5) out.push(`${name}: card runs off the screen (${box.right} > ${window.innerWidth})`);
      const image = card.querySelector<HTMLElement>("img.advisor-card-image");
      if (!image) out.push(`${name}: no image`);
      else if (phone && image.getBoundingClientRect().width < box.width * 0.3) out.push(`${name}: image column lost (${image.getBoundingClientRect().width} of ${box.width})`);
      const parts: [string, string][] = [
        ["h2", "name"],
        [".status-pill", "status pill"],
        [".advisor-skills", "skills"],
        [".advisor-card-meta", "rating and price"],
        [".advisor-card-headline", "headline"],
        [".advisor-profile-link", "profile link"],
        ['[data-testid="free-chat-badge"]', "free badge"],
      ];
      for (const [selector, label] of parts) {
        const el = card.querySelector<HTMLElement>(selector);
        if (!el) continue;
        const r = el.getBoundingClientRect();
        if (r.width === 0 || r.height === 0) continue; // not shown at this size (desktop hides the pill)
        if (!inside(box, r)) out.push(`${name}: ${label} outside the card (${Math.round(r.left)}–${Math.round(r.right)} vs ${Math.round(box.left)}–${Math.round(box.right)})`);
      }
      const pill = card.querySelector<HTMLElement>(".status-pill");
      if (phone && pill && pill.getBoundingClientRect().width > 0) {
        const cs = getComputedStyle(pill);
        if (pill.scrollWidth > pill.clientWidth + 1) out.push(`${name}: status pill clipped`);
        if (cs.flexShrink !== "0") out.push(`${name}: status pill may shrink`);
      }
      const headline = card.querySelector<HTMLElement>(".advisor-card-headline");
      if (headline) {
        const lines = Math.round(headline.getBoundingClientRect().height / parseFloat(getComputedStyle(headline).lineHeight));
        if (lines > 2) out.push(`${name}: headline takes ${lines} lines`);
      }
      for (const el of card.querySelectorAll<HTMLElement>("*")) {
        if (el.closest("svg")) continue;
        const r = el.getBoundingClientRect();
        if (r.width === 0 || r.height === 0) continue;
        if (el.closest(".sr-only") || el.classList.contains("sr-only")) continue;
        // Inside a box that clips (the skills line): its own box is what must fit.
        let clipped = false;
        for (let p = el.parentElement; p && p !== card; p = p.parentElement) {
          if (getComputedStyle(p).overflowX !== "visible") clipped = true;
        }
        if (clipped) continue;
        if (!inside(box, r)) out.push(`${name}: <${el.tagName.toLowerCase()} class="${String(el.className).split(" ")[0]}"> outside the card`);
        const cs = getComputedStyle(el);
        // Text that is cut with an ellipsis (name, skills) clips inside its own
        // box on purpose; anything else must fit its box.
        const clipsOnPurpose = cs.overflowX !== "visible";
        if (!clipsOnPurpose && cs.display !== "inline" && el.scrollWidth > el.clientWidth + 1) {
          out.push(`${name}: <${el.tagName.toLowerCase()} class="${String(el.className).split(" ")[0]}"> overflows (${el.scrollWidth} > ${el.clientWidth})`);
        }
      }
    }
    return out;
  }, opts);
}

/** Free mode on (the "Free live chat" badge shows): the client's config request is answered by the test. */
async function freeModeOn(page: Page) {
  await page.route("**/api/v1/site/config*", (route) =>
    route.fulfill({
      json: {
        banners: { rotation_seconds: 4, items: [] },
        signup_offer: { enabled: true, free_minutes: 5 },
        free_mode: { active: true },
        generated_at: new Date().toISOString(),
      },
    }),
  );
  await page.clock.install();
}

async function showDirectory(page: Page, free: boolean) {
  if (free) await freeModeOn(page);
  await page.goto("/en/psychics");
  if (free) {
    // The first paint comes from the server; moving the clock past the five-minute refresh loads the intercepted config.
    await page.waitForLoadState("networkidle");
    await page.clock.fastForward(5 * 60_000 + 1_000);
    await expect(page.getByTestId("free-chat-badge").first()).toBeVisible();
  }
  await expect(page.locator("article.advisor-card").first()).toBeVisible();
  await expect(page.getByTestId("advisor-grid-skeleton")).toHaveCount(0);
}

test.describe("advisor cards: layout (§26.2)", () => {
  for (const free of [false, true]) {
    for (const width of PHONE_WIDTHS) {
      for (const scale of TEXT_SCALES) {
        test(`phone ${width}px, text ${Math.round(scale * 100)} %${free ? ", free mode" : ""}`, async ({ page }) => {
          await page.setViewportSize({ width, height: 800 });
          await signIn(page, CLIENT.email, CLIENT.password);
          await showDirectory(page, free);
          if (scale !== 1) await scaleText(page, "article.advisor-card", scale);
          const findings = await auditAdvisorCards(page, { phone: true });
          expect(findings, findings.join("\n")).toEqual([]);
        });
      }
    }
  }

  test("the card keeps the proportions of the design at 390 px", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await signIn(page, CLIENT.email, CLIENT.password);
    await showDirectory(page, false);
    const card = page.locator("article.advisor-card:has(.advisor-skills)").first();
    const sizes = await card.evaluate((el) => {
      const px = (selector: string, prop: "fontSize" | "textTransform") => getComputedStyle(el.querySelector(selector)!)[prop];
      const box = el.getBoundingClientRect();
      const image = el.querySelector("img.advisor-card-image")!.getBoundingClientRect();
      const pill = el.querySelector(".status-pill")!.getBoundingClientRect();
      const name = el.querySelector("h2")!.getBoundingClientRect();
      return {
        name: px("h2", "fontSize"),
        pill: px(".status-pill", "fontSize"),
        pillCase: px(".status-pill", "textTransform"),
        skills: px(".advisor-skills", "fontSize"),
        headline: px(".advisor-card-headline", "fontSize"),
        imageShare: image.width / box.width,
        pillBesideName: Math.abs(pill.top + pill.height / 2 - (name.top + name.height / 2)) < 12,
      };
    });
    expect(sizes).toMatchObject({ name: "23px", pill: "10px", pillCase: "uppercase", skills: "10px", headline: "11px", pillBesideName: true });
    expect(sizes.imageShare).toBeGreaterThan(0.3);
    expect(sizes.imageShare).toBeLessThan(0.36);
  });

  for (const width of [768, 1024, 1440]) {
    test(`tablet and desktop ${width}px: no regression`, { tag: "@responsive" }, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await signIn(page, CLIENT.email, CLIENT.password);
      await showDirectory(page, false);
      const findings = await auditAdvisorCards(page, { phone: false });
      expect(findings, findings.join("\n")).toEqual([]);
    });
  }

  test("home section cards stay inside their box on phones with larger text", async ({ page }) => {
    for (const width of PHONE_WIDTHS) {
      await page.setViewportSize({ width, height: 800 });
      await page.goto("/en");
      await page.locator("#psychics").scrollIntoViewIfNeeded();
      await expect(page.getByTestId("featured-advisor").first()).toBeVisible();
      for (const scale of [1, 2]) {
        await scaleText(page, '[data-testid="featured-advisor"]', scale);
        const findings = await page.evaluate(() => {
          const out: string[] = [];
          const root = document.documentElement;
          if (root.scrollWidth > root.clientWidth) out.push(`page scrolls horizontally (${root.scrollWidth} > ${root.clientWidth})`);
          for (const card of document.querySelectorAll<HTMLElement>('[data-testid="featured-advisor"]')) {
            const box = card.getBoundingClientRect();
            if (card.scrollWidth > card.clientWidth + 1) out.push(`card wider than itself (${card.scrollWidth} > ${card.clientWidth})`);
            for (const el of card.querySelectorAll<HTMLElement>("*")) {
              const r = el.getBoundingClientRect();
              if (r.width === 0 || el.closest("[aria-hidden=true]")) continue;
              if (r.left < box.left - 0.5 || r.right > box.right + 0.5) out.push(`<${el.tagName.toLowerCase()}> "${(el.textContent ?? "").slice(0, 24)}" outside the card`);
            }
          }
          return out;
        });
        expect(findings, `${width}px, text ${scale * 100} %: ${findings.join("; ")}`).toEqual([]);
      }
    }
  });

  test("home section cards line up their divider, rates and button whatever the headline length", async ({ page }) => {
    // Real cards with headlines of very different lengths, one advisor online ("Chat now" with its
    // price line is taller than "Notify me when online").
    const headlines = [
      "Clarity in love.",
      "Many people approach me to help them discover more about themselves and the type of relationships that they encounter every single day, and where they lead.",
      "I'll guide you towards the path you should take in order to resolve any future difficulties.",
    ];
    await page.route("**/api/v1/psychics/featured*", async (route) => {
      const response = await route.fetch();
      const body = await response.json();
      body.items = body.items.map((item: Record<string, unknown>, i: number) => ({
        ...item,
        headline: headlines[i % headlines.length],
        ...(i === 0 ? { status: "online", cta: "chat" } : { status: "offline", cta: "waitlist" }),
      }));
      await route.fulfill({ response, json: body });
    });
    for (const width of [1440, 1024, 768]) {
      await page.setViewportSize({ width, height: 900 });
      await page.goto("/en");
      await page.locator("#psychics").scrollIntoViewIfNeeded();
      await expect(page.getByTestId("featured-advisor")).toHaveCount(3);
      const cards = await page.getByTestId("featured-advisor").evaluateAll((els) =>
        els.map((card) => {
          const top = (sel: string) => Math.round(card.querySelector(sel)!.getBoundingClientRect().top);
          const box = card.getBoundingClientRect();
          const button = card.querySelector('[data-testid="book-button"]')!.getBoundingClientRect();
          return {
            row: Math.round(box.top),
            center: Math.round(box.left + box.width / 2),
            height: Math.round(box.height),
            headline: Math.round(card.querySelector('[data-testid="featured-advisor-intro"] p')!.getBoundingClientRect().height),
            divider: top('[data-testid="featured-advisor-divider"]'),
            rates: top("dl"),
            buttonTop: Math.round(button.top),
            buttonBottom: Math.round(button.bottom),
          };
        }),
      );
      const rows = new Map<number, typeof cards>();
      for (const c of cards) rows.set(c.row, [...(rows.get(c.row) ?? []), c]);
      expect(rows.size, `${width}px: rows ${JSON.stringify(cards)}`).toBe(width >= 1024 ? 1 : 2);
      for (const row of rows.values()) {
        for (const key of ["height", "divider", "rates", "buttonTop", "buttonBottom"] as const) {
          const values = row.map((c) => c[key]);
          expect(Math.max(...values) - Math.min(...values), `${width}px ${key}: ${values.join(", ")}`).toBeLessThanOrEqual(1);
        }
      }
      // The headlines really differ in length (the test would prove nothing otherwise).
      const headlineHeights = cards.map((c) => c.headline);
      expect(Math.max(...headlineHeights) - Math.min(...headlineHeights), `${width}px headlines: ${headlineHeights.join(", ")}`).toBeGreaterThan(30);
      if (width === 768) {
        // Two columns: the third card, alone on its row, is centred under the first two.
        const [a, b, c] = cards;
        expect(Math.abs(c.center - (a.center + b.center) / 2)).toBeLessThanOrEqual(1);
      }
    }
  });
});

/** The advisor's own session, kept open so the advisor stays connected (online). */
async function advisorSession(browser: Browser, slug: "ramone" | "theo" | "solomon") {
  const session = await signedInPage(browser, advisor(slug).email, PSYCHIC_PASSWORD);
  // Advisors land in the chatroom, whose own top bar carries a second (hidden) availability menu:
  // the profile page has the site header's single one, which setAvailability drives.
  await session.page.goto("/en/profile");
  return session;
}

const TIER: Record<string, number> = { online: 0, away: 1, busy: 2, offline: 3 };

/** The directory's cards as [name, status], top to bottom. */
async function directoryCards(page: Page): Promise<[string, string][]> {
  return page.locator("article.advisor-card").evaluateAll((cards) =>
    cards.map((c) => [c.querySelector("h2")?.textContent?.split(",")[0] ?? "", c.getAttribute("data-status") ?? ""] as [string, string]),
  );
}

function expectTierOrder(cards: [string, string][]) {
  const tiers = cards.map(([, status]) => TIER[status]);
  expect(tiers, JSON.stringify(cards)).toEqual([...tiers].sort((a, b) => a - b));
}

// Desktop project only: the advisors are driven through the header's availability menu
// (e2e/helpers.ts), which has two copies in the DOM in the phone project.
test.describe("directory order and live re-sort (§26.3)", () => {
  test("online, away, busy, offline; status changes re-sort the cards and refetch at most every 5 s", async ({ browser, page }) => {
    test.setTimeout(240_000); // four sign-ins, two pages and a 6 s observation window
    const sessions = {
      ramone: await advisorSession(browser, "ramone"),
      theo: await advisorSession(browser, "theo"),
      solomon: await advisorSession(browser, "solomon"),
    };
    try {
      await setAvailability(sessions.ramone.page, "online");
      await setAvailability(sessions.theo.page, "offline");
      await setAvailability(sessions.solomon.page, "away");

      await signIn(page, CLIENT.email, CLIENT.password);
      await page.goto("/en/psychics");
      await expect(page.locator("article.advisor-card").first()).toBeVisible();
      await expect.poll(async () => (await directoryCards(page)).find(([n]) => n === "Ramone")?.[1]).toBe("online");
      let cards = await directoryCards(page);
      expectTierOrder(cards);
      const names = cards.map(([n]) => n);
      expect(names.indexOf("Ramone")).toBeLessThan(names.indexOf("Solomon"));
      expect(cards.find(([n]) => n === "Solomon")?.[1]).toBe("away");
      // Statuses that are not online come after every online card.
      expect(cards[0][1]).toBe("online");

      // The home selection uses the same order: the online advisor, then the away one.
      await page.goto("/en");
      await expect(page.getByTestId("featured-advisor").first()).toBeVisible();
      const featured = page.getByTestId("featured-advisor");
      await expect(featured.nth(0).getByRole("heading")).toHaveText("Ramone");
      await expect(featured.nth(1).getByRole("heading")).toHaveText("Solomon");
      await expect(featured.nth(1)).toContainText("Away");
      await page.goto("/en/psychics");
      await expect(page.locator("article.advisor-card").first()).toBeVisible();

      // A live change re-sorts the cards on screen: Solomon comes online and Ramone goes away.
      await setAvailability(sessions.solomon.page, "online");
      await expect
        .poll(async () => {
          const now = await directoryCards(page);
          return now.find(([n]) => n === "Solomon")?.[1];
        })
        .toBe("online");
      await setAvailability(sessions.ramone.page, "away");
      await expect
        .poll(async () => {
          const now = await directoryCards(page);
          const order = now.map(([n]) => n);
          return now.find(([n]) => n === "Ramone")?.[1] === "away" && order.indexOf("Solomon") < order.indexOf("Ramone");
        })
        .toBe(true);
      cards = await directoryCards(page);
      expectTierOrder(cards);

      // A burst of status changes asks the API for the page at most every 5 s.
      let listRequests = 0;
      page.on("request", (r) => {
        if (new URL(r.url()).pathname === "/api/v1/psychics" && r.method() === "GET") listRequests++;
      });
      for (const value of ["away", "online", "away", "online"] as const) await setAvailability(sessions.theo.page, value);
      await page.waitForTimeout(6_000);
      expect(listRequests).toBeGreaterThanOrEqual(1);
      expect(listRequests).toBeLessThanOrEqual(2);
      expectTierOrder(await directoryCards(page));
    } finally {
      for (const slug of ["ramone", "theo", "solomon"] as const) {
        await setAvailability(sessions[slug].page, "offline").catch(() => undefined);
        await sessions[slug].context.close();
      }
    }
  });
});
