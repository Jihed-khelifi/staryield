import { describe, expect, it, vi } from "vitest";

import { SHARE_DOMAIN, type ShareCardSpec } from "@/lib/share/card-spec";
import {
  BODY_BOTTOM,
  BODY_TOP,
  BRAND_MARK_URL,
  CARD_HEIGHT,
  CARD_WIDTH,
  SHARE_PALETTE,
  drawShareCard,
  fitText,
  renderShareCard,
  tintImage,
  wrapText,
  type DrawableImage,
} from "@/lib/share/render-card";

type Call = { op: string; args: unknown[]; font: string };

/**
 * A 2D context that records what is drawn. Text is measured as 0.55 em per
 * character at the current font size, which is close enough to a serif face
 * to exercise wrapping and fitting.
 */
function fakeContext() {
  const calls: Call[] = [];
  let font = "10px serif";
  const record =
    (op: string) =>
    (...args: unknown[]) => {
      calls.push({ op, args, font });
    };
  const ctx = {
    get font() {
      return font;
    },
    set font(value: string) {
      font = value;
    },
    fillStyle: "",
    strokeStyle: "",
    globalCompositeOperation: "source-over",
    lineWidth: 1,
    textAlign: "start",
    textBaseline: "alphabetic",
    letterSpacing: "0px",
    save: record("save"),
    restore: record("restore"),
    fillRect: record("fillRect"),
    strokeRect: record("strokeRect"),
    beginPath: record("beginPath"),
    closePath: record("closePath"),
    moveTo: record("moveTo"),
    lineTo: record("lineTo"),
    arcTo: record("arcTo"),
    arc: record("arc"),
    stroke: record("stroke"),
    fill: record("fill"),
    clip: record("clip"),
    drawImage: record("drawImage"),
    fillText: record("fillText"),
    createLinearGradient: () => ({ addColorStop: () => undefined }),
    measureText: (value: string) => ({ width: value.length * 0.55 * Number(/(\d+)px/.exec(font)?.[1] ?? 10) }),
  };
  return { ctx: ctx as unknown as CanvasRenderingContext2D, calls };
}

const fonts = { display: "Cinzel, serif", body: "Source Serif, serif" };
const image = (width: number, height: number) => ({ width, height }) as DrawableImage;
const texts = (calls: Call[]) => calls.filter((c) => c.op === "fillText").map((c) => c.args[0] as string);

const tarot: ShareCardSpec = {
  kind: "tarot-birth-card",
  eyebrow: "Tarot birth card",
  headline: "My tarot birth card is Wheel Of Fortune",
  lines: ["Soul card: The Magician"],
  artwork: [
    { url: "https://assets.staryield.net/assets/tarot/wheel-of-fortune.png", aspect: 210 / 355, style: "card", caption: "X" },
    { url: "https://assets.staryield.net/assets/tarot/the-magician.png", aspect: 210 / 355, style: "card", caption: "I" },
  ],
  shareText: "",
  alt: "",
};
const lifePath: ShareCardSpec = { kind: "path-of-life", eyebrow: "Path of life", headline: "My life path number is 7", lines: ["The seeker"], badge: "7", artwork: [], shareText: "", alt: "" };
const sign: ShareCardSpec = {
  kind: "astrology",
  eyebrow: "Zodiac sign",
  headline: "I'm a Leo",
  lines: ["Fire · Fixed"],
  artwork: [{ url: "https://assets.staryield.net/assets/figma/c6980.svg", aspect: 37.4 / 51, style: "emblem" }],
  shareText: "",
  alt: "",
};
const arcs = (calls: Call[]) => calls.filter((c) => c.op === "arc");

describe("wrapText and fitText", () => {
  it("wraps on words within the width and splits a word wider than the line", () => {
    const { ctx } = fakeContext();
    ctx.font = "10px serif"; // 5.5 px per character
    expect(wrapText(ctx, "My tarot birth card is The Star", 61)).toEqual(["My tarot", "birth card", "is The Star"]);
    expect(wrapText(ctx, "Supercalifragilistic", 33)).toEqual(["Superc", "alifra", "gilist", "ic"]);
    expect(wrapText(ctx, "   ", 100)).toEqual([]);
  });

  it("shrinks the type until the text fits the line budget, never below the minimum", () => {
    const { ctx } = fakeContext();
    const font = (size: number) => `600 ${size}px Cinzel`;
    const long = "Sun in Sagittarius · Moon in Capricorn · Sagittarius rising";
    const fitted = fitText(ctx, long, font, 840, 2, 92, 44);
    expect(fitted.lines.length).toBeLessThanOrEqual(2);
    expect(fitted.size).toBeLessThan(92);
    expect(fitted.size).toBeGreaterThanOrEqual(44);
    const floor = fitText(ctx, long.repeat(6), font, 840, 2, 92, 44);
    expect(floor.size).toBe(44);
    expect(floor.lines).toHaveLength(2);
  });
});

describe("drawShareCard", () => {
  it("draws the eyebrow, both cards with their numerals, the headline, the soul card and the address", () => {
    const { ctx, calls } = fakeContext();
    const brand = image(192, 192);
    const layout = drawShareCard(ctx, tarot, [image(210, 355), image(210, 355)], fonts, brand);
    expect(layout.artworkDrawn).toBe(2);
    expect(layout.brandMarkDrawn).toBe(true);
    expect(layout.badgeDrawn).toBe(false);
    const drawn = texts(calls);
    expect(drawn).toContain("TAROT BIRTH CARD");
    expect(drawn).toContain("X");
    expect(drawn).toContain("I");
    expect(drawn).toContain(SHARE_DOMAIN);
    expect(layout.headline.lines.join(" ")).toBe(tarot.headline);
    expect(drawn).toEqual(expect.arrayContaining(layout.headline.lines));
    expect(layout.lines).toEqual([["Soul card: The Magician"]]);
    // Everything stays inside the 1080 × 1920 card and its margins.
    for (const call of calls.filter((c) => c.op === "drawImage")) {
      const [, x, y, w, h] = call.args as number[];
      expect(x).toBeGreaterThanOrEqual(0);
      expect(x + w).toBeLessThanOrEqual(CARD_WIDTH);
      expect(y + h).toBeLessThanOrEqual(CARD_HEIGHT);
    }
    // Text uses the site's faces.
    expect(calls.find((c) => c.op === "fillText" && c.args[0] === SHARE_DOMAIN)!.font).toContain("Cinzel");
    expect(calls.find((c) => c.op === "fillText" && c.args[0] === "Soul card: The Magician")!.font).toContain("Source Serif");
  });

  it("leaves out pictures that could not be loaded and still draws the text", () => {
    const { ctx, calls } = fakeContext();
    const layout = drawShareCard(ctx, tarot, [null, image(210, 355)], fonts, null);
    expect(layout.artworkDrawn).toBe(1);
    expect(layout.brandMarkDrawn).toBe(false);
    expect(calls.filter((c) => c.op === "drawImage")).toHaveLength(1);
    expect(calls.every((c) => c.op !== "drawImage" || c.args[0] !== null)).toBe(true);
    expect(texts(calls)).toContain(SHARE_DOMAIN);

    const none = fakeContext();
    expect(drawShareCard(none.ctx, tarot, [null, null], fonts, null).artworkDrawn).toBe(0);
    expect(none.calls.filter((c) => c.op === "drawImage")).toHaveLength(0);
  });

  it("draws the number in the medallion when there is no artwork", () => {
    const { ctx, calls } = fakeContext();
    const layout = drawShareCard(ctx, lifePath, [], fonts, null);
    expect(layout.badgeDrawn).toBe(true);
    expect(layout.medallionDrawn).toBe(true);
    expect(texts(calls)).toContain("7");
    expect(arcs(calls).length).toBeGreaterThanOrEqual(2); // disc and double ring
  });

  it("draws a sign glyph inside the medallion, keeping its proportions", () => {
    const { ctx, calls } = fakeContext();
    const layout = drawShareCard(ctx, sign, [image(374, 510)], fonts, null);
    expect(layout).toMatchObject({ medallionDrawn: true, artworkDrawn: 1, badgeDrawn: false });
    const [, x, , w, h] = calls.find((c) => c.op === "drawImage")!.args as number[];
    expect(w / h).toBeCloseTo(374 / 510, 3);
    expect(x + w / 2).toBeCloseTo(CARD_WIDTH / 2, 3);
    // Without the glyph there is no empty medallion.
    expect(drawShareCard(fakeContext().ctx, sign, [null], fonts, null)).toMatchObject({ medallionDrawn: false, artworkDrawn: 0 });
  });

  it("keeps the body between the eyebrow and the footer whatever the content", () => {
    const long = { ...tarot, headline: "Ma carte de naissance du tarot est La Roue de Fortune", lines: ["Carte d’âme : Le Bateleur"] };
    const three = { ...long, artwork: [...tarot.artwork, { ...tarot.artwork[1], caption: "XIX" }] };
    for (const [spec, imgs] of [
      [tarot, [image(210, 355), image(210, 355)]],
      [long, [image(210, 355), image(210, 355)]],
      [three, [image(210, 355), image(210, 355), image(210, 355)]],
      [lifePath, []],
      [sign, [image(374, 510)]],
    ] as const) {
      const { ctx, calls } = fakeContext();
      const layout = drawShareCard(ctx, spec as ShareCardSpec, [...imgs], fonts, image(192, 192));
      expect(layout.bottom, spec.headline).toBeLessThanOrEqual(BODY_BOTTOM);
      for (const call of calls.filter((c) => c.op === "drawImage" && (c.args[3] as number) !== 96)) {
        const [, , y, , h] = call.args as number[];
        expect(y).toBeGreaterThanOrEqual(BODY_TOP);
        expect(y + h).toBeLessThanOrEqual(BODY_BOTTOM);
      }
    }
  });

  it("wraps a long headline on several lines within the margins", () => {
    const { ctx } = fakeContext();
    const spec = { ...lifePath, badge: undefined, headline: "Sun in Sagittarius · Moon in Capricorn · Sagittarius rising" };
    const layout = drawShareCard(ctx, spec, [], fonts, null);
    expect(layout.headline.lines.length).toBeGreaterThan(1);
    expect(layout.headline.lines.length).toBeLessThanOrEqual(4);
    for (const line of layout.headline.lines) expect(line.length * 0.55 * layout.headline.size).toBeLessThanOrEqual(840);
  });
});

describe("renderShareCard", () => {
  function fakeCanvas(blob: Blob | null = new Blob(["png"], { type: "image/png" }), context = fakeContext().ctx) {
    return {
      width: 0,
      height: 0,
      getContext: vi.fn(() => context),
      toBlob: vi.fn((done: (b: Blob | null) => void, type: string) => {
        expect(type).toBe("image/png");
        done(blob);
      }),
    } as unknown as HTMLCanvasElement;
  }

  it("draws sign glyphs in gold on their own canvas, sharp at the medallion's size", async () => {
    const glyph = fakeContext();
    const glyphCanvas = { width: 0, height: 0, getContext: () => glyph.ctx } as unknown as HTMLCanvasElement;
    const createCanvas = vi.fn((width: number, height: number) => Object.assign(glyphCanvas, { width, height }));
    const tinted = tintImage(image(37.4, 51), SHARE_PALETTE.goldDark, createCanvas);
    expect(tinted).toBe(glyphCanvas);
    expect(createCanvas).toHaveBeenCalledWith(375, 512);
    expect(glyph.calls.find((c) => c.op === "drawImage")!.args.slice(1)).toEqual([0, 0, 375, 512]);
    expect((glyph.ctx as unknown as { globalCompositeOperation: string }).globalCompositeOperation).toBe("source-in");
    expect(glyph.ctx.fillStyle).toBe(SHARE_PALETTE.goldDark);
    // A browser that cannot: the glyph as it is.
    const original = image(51, 51);
    expect(tintImage(original, SHARE_PALETTE.goldDark, () => ({ getContext: () => null }) as unknown as HTMLCanvasElement)).toBe(original);
  });

  it("loads the fonts, the artwork and the sun mark, and encodes a 1080 × 1920 PNG", async () => {
    const canvas = fakeCanvas();
    const createCanvas = vi.fn(() => canvas);
    const loadImage = vi.fn(async (url: string) => (url.includes("magician") ? null : image(210, 355)));
    const loadFonts = vi.fn(async () => fonts);
    const blob = await renderShareCard(tarot, { createCanvas, loadImage, loadFonts });
    expect(blob.type).toBe("image/png");
    expect(createCanvas).toHaveBeenCalledWith(CARD_WIDTH, CARD_HEIGHT);
    expect(loadImage.mock.calls.map(([url]) => url)).toEqual([tarot.artwork[0].url, tarot.artwork[1].url, BRAND_MARK_URL]);
    expect(loadFonts).toHaveBeenCalledWith(tarot);
  });

  it("rejects when the browser cannot draw or encode the image", async () => {
    const deps = { loadImage: async () => null, loadFonts: async () => fonts };
    const noContext = { getContext: () => null } as unknown as HTMLCanvasElement;
    await expect(renderShareCard(lifePath, { ...deps, createCanvas: () => noContext })).rejects.toThrow("Canvas is not available");
    await expect(renderShareCard(lifePath, { ...deps, createCanvas: () => fakeCanvas(null) })).rejects.toThrow("could not be encoded");
  });
});
