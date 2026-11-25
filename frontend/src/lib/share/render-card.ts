/**
 * Draws a share card (1080 × 1920, 9:16) on a canvas and encodes it as a PNG.
 *
 * `drawShareCard` is synchronous and only talks to a 2D context, so the
 * layout (wrapping, auto-fitted type, artwork placement, fallbacks) is unit
 * tested with a fake context. `renderShareCard` adds what needs a browser:
 * the site's fonts (read from the `--font-cinzel` / `--font-source-serif` CSS
 * variables and awaited with `document.fonts.load`), the R2 artwork loaded
 * with `crossOrigin = "anonymous"` and the PNG encoder. An image that cannot
 * be loaded is simply left out: the canvas never gets a cross-origin picture
 * it may not read, so it is never tainted and always exports.
 */
import { SHARE_DOMAIN, type ShareArtwork, type ShareCardSpec } from "@/lib/share/card-spec";

export const CARD_WIDTH = 1080;
export const CARD_HEIGHT = 1920;

/** Brand palette: cream, paper, gold, call-to-action gold and ink. */
export const SHARE_PALETTE = {
  cream: "#E0D5C3",
  paper: "#F4F0EA",
  gold: "#AA8E62",
  cta: "#C29A3D",
  ink: "#15100B",
  goldDark: "#8A6A1F",
} as const;

/** The faceless sun mark of the site (R2, same file as the favicon). */
export const BRAND_MARK_URL = "https://assets.staryield.net/assets/brand/sun-mark-192-c139a00d54.png";

export type ShareFonts = { display: string; body: string };
export type DrawableImage = CanvasImageSource & { width: number; height: number };

export type CardLayout = {
  headline: { size: number; lines: string[] };
  lines: string[][];
  /** Pictures drawn (tarot cards or the sign's emblem). */
  artworkDrawn: number;
  /** The large figure was drawn in the medallion. */
  badgeDrawn: boolean;
  /** The medallion (emblem or figure) was drawn. */
  medallionDrawn: boolean;
  brandMarkDrawn: boolean;
  /** Where the body (picture, headline, sentences) ends; never below BODY_BOTTOM. */
  bottom: number;
};

const MARGIN_X = 120;
const CONTENT_WIDTH = CARD_WIDTH - MARGIN_X * 2;
/** The body sits between the eyebrow and the footer (sun mark at 1596, address plate at 1730). */
export const BODY_TOP = 330;
export const BODY_BOTTOM = 1540;
const BLOCK_GAP = 64;
const CARD_GAP = 40;
const CAPTION_HEIGHT = 76;
const CARD_MAX_HEIGHT = 700;
const MEDALLION_MAX = 420;
const FIGURE_MIN = 220;

type Ctx = CanvasRenderingContext2D;

function setSpacing(ctx: Ctx, px: number) {
  if ("letterSpacing" in ctx) (ctx as Ctx & { letterSpacing: string }).letterSpacing = `${px}px`;
}

/** Break `value` into lines no wider than `maxWidth` (words; a word wider than the line is split). */
export function wrapText(ctx: Ctx, value: string, maxWidth: number): string[] {
  const lines: string[] = [];
  let line = "";
  const push = () => {
    if (line) lines.push(line);
    line = "";
  };
  for (const word of value.split(/\s+/).filter(Boolean)) {
    const candidate = line ? `${line} ${word}` : word;
    if (ctx.measureText(candidate).width <= maxWidth) {
      line = candidate;
      continue;
    }
    push();
    if (ctx.measureText(word).width <= maxWidth) {
      line = word;
      continue;
    }
    // A single word wider than the line: split it by characters.
    for (const char of Array.from(word)) {
      if (line && ctx.measureText(line + char).width > maxWidth) push();
      line += char;
    }
  }
  push();
  return lines;
}

/** The largest size from `start` down to `min` at which `value` wraps into at most `maxLines` lines. */
export function fitText(ctx: Ctx, value: string, font: (size: number) => string, maxWidth: number, maxLines: number, start: number, min: number, step = 4) {
  let size = start;
  for (;;) {
    ctx.font = font(size);
    const lines = wrapText(ctx, value, maxWidth);
    if (lines.length <= maxLines || size <= min) return { size, lines: lines.slice(0, Math.max(maxLines, 1)) };
    size = Math.max(size - step, min);
  }
}

function roundedRect(ctx: Ctx, x: number, y: number, w: number, h: number, r: number) {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.arcTo(x + w, y, x + w, y + h, r);
  ctx.arcTo(x + w, y + h, x, y + h, r);
  ctx.arcTo(x, y + h, x, y, r);
  ctx.arcTo(x, y, x + w, y, r);
  ctx.closePath();
}

function diamond(ctx: Ctx, cx: number, cy: number, r: number) {
  ctx.beginPath();
  ctx.moveTo(cx, cy - r);
  ctx.lineTo(cx + r, cy);
  ctx.lineTo(cx, cy + r);
  ctx.lineTo(cx - r, cy);
  ctx.closePath();
  ctx.fill();
}

function circle(ctx: Ctx, cx: number, cy: number, r: number) {
  ctx.beginPath();
  ctx.arc(cx, cy, r, 0, Math.PI * 2);
  ctx.closePath();
}

/** A thin rule with a diamond in the middle. */
function ornament(ctx: Ctx, cy: number, half: number) {
  const cx = CARD_WIDTH / 2;
  ctx.strokeStyle = SHARE_PALETTE.gold;
  ctx.lineWidth = 2;
  ctx.beginPath();
  ctx.moveTo(cx - half, cy);
  ctx.lineTo(cx - 22, cy);
  ctx.moveTo(cx + 22, cy);
  ctx.lineTo(cx + half, cy);
  ctx.stroke();
  ctx.fillStyle = SHARE_PALETTE.gold;
  diamond(ctx, cx, cy, 9);
}

function drawBackground(ctx: Ctx) {
  const gradient = ctx.createLinearGradient(0, 0, CARD_WIDTH, CARD_HEIGHT);
  gradient.addColorStop(0, SHARE_PALETTE.cream);
  gradient.addColorStop(1, SHARE_PALETTE.paper);
  ctx.fillStyle = gradient;
  ctx.fillRect(0, 0, CARD_WIDTH, CARD_HEIGHT);
  // Double frame with a diamond in each corner, in the site's gold.
  ctx.strokeStyle = SHARE_PALETTE.gold;
  ctx.lineWidth = 5;
  ctx.strokeRect(40, 40, CARD_WIDTH - 80, CARD_HEIGHT - 80);
  ctx.lineWidth = 1.5;
  ctx.strokeRect(62, 62, CARD_WIDTH - 124, CARD_HEIGHT - 124);
  ctx.fillStyle = SHARE_PALETTE.gold;
  for (const [x, y] of [
    [40, 40],
    [CARD_WIDTH - 40, 40],
    [40, CARD_HEIGHT - 40],
    [CARD_WIDTH - 40, CARD_HEIGHT - 40],
  ])
    diamond(ctx, x, y, 14);
}

/** The medallion of signs and numbers: a paper disc in a double gold ring with four diamonds. */
function drawMedallion(ctx: Ctx, cx: number, cy: number, r: number) {
  ctx.fillStyle = SHARE_PALETTE.paper;
  circle(ctx, cx, cy, r);
  ctx.fill();
  ctx.strokeStyle = SHARE_PALETTE.gold;
  ctx.lineWidth = 6;
  circle(ctx, cx, cy, r);
  ctx.stroke();
  ctx.lineWidth = 1.5;
  circle(ctx, cx, cy, r - 18);
  ctx.stroke();
  ctx.fillStyle = SHARE_PALETTE.gold;
  for (const [dx, dy] of [
    [0, -r],
    [r, 0],
    [0, r],
    [-r, 0],
  ])
    diamond(ctx, cx + dx, cy + dy, 12);
}

function ratioOf(image: DrawableImage | null, art: ShareArtwork) {
  const natural = image && image.width > 0 && image.height > 0 ? image.width / image.height : art.aspect;
  return natural > 0 ? natural : art.aspect > 0 ? art.aspect : 1;
}

/**
 * Draws the card; `images[i]` is the loaded picture of `spec.artwork[i]`
 * (null when it could not load: the card is drawn without it). The picture
 * shrinks to leave room for the text, so the body always stays between
 * BODY_TOP and BODY_BOTTOM.
 */
export function drawShareCard(ctx: Ctx, spec: ShareCardSpec, images: (DrawableImage | null)[], fonts: ShareFonts, brandMark: DrawableImage | null): CardLayout {
  ctx.save();
  ctx.textAlign = "center";
  ctx.textBaseline = "top";
  drawBackground(ctx);
  const cx = CARD_WIDTH / 2;

  // Eyebrow.
  ctx.fillStyle = SHARE_PALETTE.goldDark;
  setSpacing(ctx, 8);
  const eyebrow = fitText(ctx, spec.eyebrow.toUpperCase(), (size) => `600 ${size}px ${fonts.display}`, CONTENT_WIDTH, 2, 36, 24, 2);
  eyebrow.lines.forEach((line, i) => ctx.fillText(line, cx, 190 + i * 48));
  setSpacing(ctx, 0);
  ornament(ctx, 190 + eyebrow.lines.length * 48 + 34, 90);

  // Measure the text first: the picture takes what is left.
  const headline = fitText(ctx, spec.headline, (size) => `600 ${size}px ${fonts.display}`, CONTENT_WIDTH, 4, 92, 44);
  const headlineLine = Math.round(headline.size * 1.2);
  const headlineHeight = headline.lines.length * headlineLine;
  const lines = spec.lines.map((line) => fitText(ctx, line, (size) => `400 ${size}px ${fonts.body}`, CONTENT_WIDTH, 3, 46, 30, 2));
  const lineHeights = lines.map((line) => line.lines.length * Math.round(line.size * 1.4));
  const linesHeight = lines.length ? 80 + lineHeights.reduce((sum, h) => sum + h, 0) + (lines.length - 1) * 18 : 0;
  const textHeight = headlineHeight + linesHeight;
  const room = BODY_BOTTOM - BODY_TOP - textHeight - BLOCK_GAP;

  const shown = spec.artwork.map((art, i) => ({ art, image: images[i] ?? null })).filter((entry): entry is { art: ShareArtwork; image: DrawableImage } => entry.image !== null);
  const cards = shown.filter((entry) => entry.art.style === "card");
  const emblem = cards.length ? null : (shown.find((entry) => entry.art.style === "emblem") ?? null);
  let figureHeight = 0;
  let cardHeight = 0;
  let captionHeight = 0;
  if (cards.length) {
    captionHeight = cards.some((entry) => entry.art.caption) ? CAPTION_HEIGHT : 0;
    const widths = cards.reduce((sum, entry) => sum + ratioOf(entry.image, entry.art), 0);
    cardHeight = Math.max(FIGURE_MIN, Math.min(CARD_MAX_HEIGHT, (CONTENT_WIDTH - CARD_GAP * (cards.length - 1)) / widths, room - captionHeight));
    figureHeight = cardHeight + captionHeight;
  } else if (emblem || spec.badge) {
    figureHeight = Math.max(FIGURE_MIN, Math.min(MEDALLION_MAX, room));
  }
  const total = figureHeight + (figureHeight ? BLOCK_GAP : 0) + textHeight;
  let y = BODY_TOP + Math.max(0, (BODY_BOTTOM - BODY_TOP - total) / 2);

  // The picture: framed cards side by side, or the medallion with the sign or the number.
  let artworkDrawn = 0;
  let badgeDrawn = false;
  let medallionDrawn = false;
  if (cards.length) {
    const widths = cards.map((entry) => cardHeight * ratioOf(entry.image, entry.art));
    let x = (CARD_WIDTH - (widths.reduce((sum, w) => sum + w, 0) + CARD_GAP * (cards.length - 1))) / 2;
    cards.forEach((entry, i) => {
      const w = widths[i];
      ctx.save();
      roundedRect(ctx, x, y, w, cardHeight, 18);
      ctx.clip();
      ctx.drawImage(entry.image, x, y, w, cardHeight);
      ctx.restore();
      ctx.strokeStyle = SHARE_PALETTE.gold;
      ctx.lineWidth = 5;
      roundedRect(ctx, x, y, w, cardHeight, 18);
      ctx.stroke();
      if (entry.art.caption) {
        ctx.fillStyle = SHARE_PALETTE.goldDark;
        ctx.font = `600 40px ${fonts.display}`;
        setSpacing(ctx, 4);
        ctx.fillText(entry.art.caption, x + w / 2, y + cardHeight + 22);
        setSpacing(ctx, 0);
      }
      artworkDrawn++;
      x += w + CARD_GAP;
    });
  } else if (figureHeight) {
    const r = figureHeight / 2;
    const cy = y + r;
    drawMedallion(ctx, cx, cy, r);
    medallionDrawn = true;
    if (emblem) {
      // The glyph fits a box of half the medallion, keeping its proportions.
      const box = r;
      const ratio = ratioOf(emblem.image, emblem.art);
      const w = ratio >= 1 ? box : box * ratio;
      const h = ratio >= 1 ? box / ratio : box;
      ctx.drawImage(emblem.image, cx - w / 2, cy - h / 2, w, h);
      artworkDrawn++;
    } else if (spec.badge) {
      ctx.fillStyle = SHARE_PALETTE.goldDark;
      ctx.font = `600 ${Math.round(r * 0.95)}px ${fonts.display}`;
      ctx.textBaseline = "middle";
      ctx.fillText(spec.badge, cx, cy);
      ctx.textBaseline = "top";
      badgeDrawn = true;
    }
  }
  if (figureHeight) y += figureHeight + BLOCK_GAP;

  // Headline.
  ctx.fillStyle = SHARE_PALETTE.ink;
  ctx.font = `600 ${headline.size}px ${fonts.display}`;
  headline.lines.forEach((line, i) => ctx.fillText(line, cx, y + i * headlineLine));
  y += headlineHeight;

  // Sentences.
  if (lines.length) {
    ornament(ctx, y + 40, 120);
    y += 80;
    ctx.fillStyle = SHARE_PALETTE.ink;
    lines.forEach((line, index) => {
      ctx.font = `400 ${line.size}px ${fonts.body}`;
      const height = Math.round(line.size * 1.4);
      line.lines.forEach((text, i) => ctx.fillText(text, cx, y + i * height));
      y += lineHeights[index] + (index < lines.length - 1 ? 18 : 0);
    });
  }
  const bottom = y;

  // Footer: the sun mark and the address on a gold plate.
  let brandMarkDrawn = false;
  if (brandMark) {
    ctx.drawImage(brandMark, (CARD_WIDTH - 96) / 2, 1596, 96, 96);
    brandMarkDrawn = true;
  }
  const plateWidth = 460;
  ctx.fillStyle = SHARE_PALETTE.cta;
  roundedRect(ctx, (CARD_WIDTH - plateWidth) / 2, 1730, plateWidth, 92, 46);
  ctx.fill();
  ctx.fillStyle = SHARE_PALETTE.ink;
  ctx.font = `600 46px ${fonts.display}`;
  setSpacing(ctx, 3);
  ctx.textBaseline = "middle";
  ctx.fillText(SHARE_DOMAIN, cx, 1777);
  setSpacing(ctx, 0);

  ctx.restore();
  return { headline: { size: headline.size, lines: headline.lines }, lines: lines.map((line) => line.lines), artworkDrawn, badgeDrawn, medallionDrawn, brandMarkDrawn, bottom };
}

/**
 * A glyph redrawn in one colour on its own canvas, large enough to stay
 * sharp in the medallion (an SVG is rasterised at that size, not scaled up
 * from its 51 px). Returns the original picture when the browser cannot.
 */
export function tintImage(image: DrawableImage, color: string, createCanvas: RenderDeps["createCanvas"], size = 512): DrawableImage {
  const ratio = image.width > 0 && image.height > 0 ? image.width / image.height : 1;
  const w = Math.round(ratio >= 1 ? size : size * ratio);
  const h = Math.round(ratio >= 1 ? size / ratio : size);
  try {
    const canvas = createCanvas(w, h);
    const ctx = canvas.getContext("2d");
    if (!ctx) return image;
    ctx.drawImage(image, 0, 0, w, h);
    ctx.globalCompositeOperation = "source-in";
    ctx.fillStyle = color;
    ctx.fillRect(0, 0, w, h);
    return canvas;
  } catch {
    return image;
  }
}

/** What `renderShareCard` needs from the browser (replaceable in tests). */
export type RenderDeps = {
  createCanvas: (width: number, height: number) => HTMLCanvasElement;
  loadImage: (url: string) => Promise<DrawableImage | null>;
  loadFonts: (spec: ShareCardSpec) => Promise<ShareFonts>;
};

const IMAGE_TIMEOUT_MS = 8000;

function loadOnce(url: string): Promise<DrawableImage | null> {
  return new Promise((resolve) => {
    const image = new Image();
    // Anonymous CORS: the canvas must stay readable, so a picture without CORS headers is not drawn.
    image.crossOrigin = "anonymous";
    image.decoding = "async";
    const timer = window.setTimeout(() => resolve(null), IMAGE_TIMEOUT_MS);
    image.onload = () => {
      window.clearTimeout(timer);
      resolve(image);
    };
    image.onerror = () => {
      window.clearTimeout(timer);
      resolve(null);
    };
    image.src = url;
  });
}

/**
 * Load a picture for the canvas, or null. The browser may hold the same URL
 * from a plain `<img>` (no CORS headers in that cached answer), which fails
 * the anonymous request: one retry with a throwaway query forces a fresh,
 * CORS-enabled response.
 */
export async function loadShareImage(url: string): Promise<DrawableImage | null> {
  const first = await loadOnce(url);
  if (first) return first;
  return loadOnce(`${url}${url.includes("?") ? "&" : "?"}share=1`);
}

function cssFamily(name: string, fallback: string): string {
  try {
    const value = getComputedStyle(document.body).getPropertyValue(name).trim();
    return value || fallback;
  } catch {
    return fallback;
  }
}

/** The page's own Cinzel and Source Serif, loaded for the characters this card uses. */
export async function loadShareFonts(spec: ShareCardSpec): Promise<ShareFonts> {
  const fonts: ShareFonts = {
    display: cssFamily("--font-cinzel", "Cinzel, serif"),
    body: cssFamily("--font-source-serif", "Georgia, serif"),
  };
  const sample = [spec.eyebrow, spec.headline, ...spec.lines, ...spec.artwork.map((art) => art.caption ?? ""), spec.badge ?? "", SHARE_DOMAIN].join(" ");
  try {
    if (document.fonts?.load) {
      await Promise.race([
        Promise.all([document.fonts.load(`600 48px ${fonts.display}`, sample.toUpperCase() + sample), document.fonts.load(`400 40px ${fonts.body}`, sample)]),
        new Promise((resolve) => window.setTimeout(resolve, 4000)),
      ]);
    }
  } catch {
    // The fallback faces still draw a readable card.
  }
  return fonts;
}

const browserDeps: RenderDeps = {
  createCanvas: (width, height) => {
    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;
    return canvas;
  },
  loadImage: loadShareImage,
  loadFonts: loadShareFonts,
};

/** Render the card as a 1080 × 1920 PNG. Rejects when the browser cannot draw or encode it. */
export async function renderShareCard(spec: ShareCardSpec, deps: Partial<RenderDeps> = {}): Promise<Blob> {
  const { createCanvas, loadImage, loadFonts } = { ...browserDeps, ...deps };
  const [fonts, loaded, brandMark] = await Promise.all([loadFonts(spec), Promise.all(spec.artwork.map((art) => loadImage(art.url))), loadImage(BRAND_MARK_URL)]);
  // Sign glyphs are drawn in the card's gold.
  const images = loaded.map((image, i) => (image && spec.artwork[i].style === "emblem" ? tintImage(image, SHARE_PALETTE.goldDark, createCanvas) : image));
  const canvas = createCanvas(CARD_WIDTH, CARD_HEIGHT);
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("Canvas is not available");
  drawShareCard(ctx, spec, images, fonts, brandMark);
  return new Promise<Blob>((resolve, reject) => {
    canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error("The image could not be encoded"))), "image/png");
  });
}
