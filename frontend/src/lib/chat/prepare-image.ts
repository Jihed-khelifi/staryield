/**
 * Prepare a picked, pasted or dropped image for upload (§22.4).
 *
 * Phones hand over 5–15 MB photos, HEIC files and files whose `type` is empty
 * or wrong, none of which the API accepts as is. Before upload every image is
 * normalised in the browser:
 *
 * 1. the real format is sniffed from the first bytes (`file.type` is ignored);
 * 2. files above 40 MB are refused before anything is decoded;
 * 3. GIFs up to 8 MB go as they are (re-encoding would drop the animation);
 * 4. PNG/JPEG/WebP that are at most 2048 px on the longest side and at most
 *    1.5 MB go as they are;
 * 5. everything else (bigger, or HEIC/AVIF) is decoded with its EXIF
 *    orientation applied and re-encoded to at most 2048 px: JPEG (quality 0.85)
 *    or WebP/PNG when the image has transparency.
 *
 * A format the browser cannot decode (HEIC outside Safari) is reported with
 * clear copy instead of failing at the server.
 */
import { ATTACHMENT_MAX_BYTES } from "@/lib/api/endpoints";

/** Longest side after re-encoding. */
export const MAX_IMAGE_DIMENSION = 2048;
/** Files at or below this size (and within the dimension limit) are sent untouched. */
export const PASSTHROUGH_BYTES = 1.5 * 1024 * 1024;
/** Files above this are refused before decoding (decoding a huge photo can crash a phone tab). */
export const MAX_INPUT_BYTES = 40 * 1024 * 1024;
export const JPEG_QUALITY = 0.85;

export type ImageFormat = "png" | "jpeg" | "gif" | "webp" | "heic" | "avif";

const MIME: Record<"png" | "jpeg" | "gif" | "webp", string> = { png: "image/png", jpeg: "image/jpeg", gif: "image/gif", webp: "image/webp" };
const EXTENSION: Record<"png" | "jpeg" | "gif" | "webp", string> = { png: "png", jpeg: "jpg", gif: "gif", webp: "webp" };

export type PrepareProblem = "not_image" | "input_too_large" | "undecodable" | "too_large" | "empty";

/** English copy per problem (translate with text()). */
export const prepareProblemCopy: Record<PrepareProblem, string> = {
  not_image: "This file isn't an image we can send. Choose a PNG, JPEG, WebP or GIF image.",
  input_too_large: "This photo is too large to prepare. Choose a file under 40 MB.",
  undecodable: "This photo format can't be sent from this browser. Choose JPEG or PNG, or send a screenshot.",
  too_large: "Images must be 8 MB or smaller.",
  empty: "This image could not be read.",
};

export class PrepareImageError extends Error {
  constructor(readonly problem: PrepareProblem) {
    super(problem);
    this.name = "PrepareImageError";
  }
}

export type PreparedImage = {
  file: File;
  /** Pixel size of what will be sent (0 when unknown, e.g. a GIF sent as is). */
  width: number;
  height: number;
  /** False when the original goes as it is. */
  reencoded: boolean;
};

// --- sniffing --------------------------------------------------------------------

const ascii = (b: Uint8Array, from: number, to: number) => String.fromCharCode(...b.subarray(from, to));
const HEIC_BRANDS = new Set(["heic", "heix", "hevc", "hevx", "heim", "heis", "hevm", "hevs", "mif1", "msf1"]);

/** The real image format from the first bytes, or null when it is none we know. */
export function sniffFormat(head: Uint8Array): ImageFormat | null {
  if (head.length >= 8 && head[0] === 0x89 && ascii(head, 1, 4) === "PNG" && head[4] === 0x0d && head[5] === 0x0a && head[6] === 0x1a && head[7] === 0x0a) return "png";
  if (head.length >= 3 && head[0] === 0xff && head[1] === 0xd8 && head[2] === 0xff) return "jpeg";
  if (head.length >= 6 && (ascii(head, 0, 6) === "GIF87a" || ascii(head, 0, 6) === "GIF89a")) return "gif";
  if (head.length >= 12 && ascii(head, 0, 4) === "RIFF" && ascii(head, 8, 12) === "WEBP") return "webp";
  if (head.length >= 12 && ascii(head, 4, 8) === "ftyp") {
    const brand = ascii(head, 8, 12);
    if (brand === "avif" || brand === "avis") return "avif";
    if (HEIC_BRANDS.has(brand)) return "heic";
  }
  return null;
}

async function readHead(file: Blob, bytes = 32): Promise<Uint8Array> {
  const slice = file.slice(0, bytes);
  if (typeof slice.arrayBuffer === "function") return new Uint8Array(await slice.arrayBuffer());
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(new Uint8Array(reader.result as ArrayBuffer));
    reader.onerror = () => reject(reader.error);
    reader.readAsArrayBuffer(slice);
  });
}

// --- decode / encode (replaceable in tests) ------------------------------------------

export type Decoded = { width: number; height: number; source: CanvasImageSource; close?: () => void };

export type PrepareDeps = {
  /** Decode with the EXIF orientation applied; null when the browser cannot. */
  decode: (file: Blob) => Promise<Decoded | null>;
  /** True when any pixel is not fully opaque. */
  hasAlpha: (image: Decoded) => boolean;
  /** Draw `image` at the given size and encode it; null when the browser cannot produce that type. */
  encode: (image: Decoded, width: number, height: number, mime: string, quality?: number) => Promise<Blob | null>;
};

async function decodeBrowser(file: Blob): Promise<Decoded | null> {
  if (typeof createImageBitmap === "function") {
    try {
      const bitmap = await createImageBitmap(file, { imageOrientation: "from-image" });
      return { width: bitmap.width, height: bitmap.height, source: bitmap, close: () => bitmap.close?.() };
    } catch {
      // Fall through to <img>: some browsers decode formats createImageBitmap does not (and the reverse).
    }
  }
  if (typeof document === "undefined") return null;
  return new Promise((resolve) => {
    const url = URL.createObjectURL(file);
    const img = new Image();
    const done = (value: Decoded | null) => {
      URL.revokeObjectURL(url);
      resolve(value);
    };
    // <img> applies the EXIF orientation by default in current browsers.
    img.onload = () => done(img.naturalWidth && img.naturalHeight ? { width: img.naturalWidth, height: img.naturalHeight, source: img } : null);
    img.onerror = () => done(null);
    img.src = url;
  });
}

type Canvas2D = { canvas: OffscreenCanvas | HTMLCanvasElement; ctx: OffscreenCanvasRenderingContext2D | CanvasRenderingContext2D };

function canvas2d(width: number, height: number): Canvas2D | null {
  if (typeof OffscreenCanvas !== "undefined") {
    const canvas = new OffscreenCanvas(width, height);
    const ctx = canvas.getContext("2d");
    return ctx ? { canvas, ctx } : null;
  }
  if (typeof document === "undefined") return null;
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const ctx = canvas.getContext("2d");
  return ctx ? { canvas, ctx } : null;
}

function hasAlphaBrowser(image: Decoded): boolean {
  // A small copy is enough to find transparent areas and keeps memory low.
  const scale = Math.min(1, 256 / Math.max(image.width, image.height));
  const w = Math.max(1, Math.round(image.width * scale));
  const h = Math.max(1, Math.round(image.height * scale));
  const c = canvas2d(w, h);
  if (!c) return false;
  c.ctx.drawImage(image.source, 0, 0, w, h);
  const { data } = c.ctx.getImageData(0, 0, w, h);
  for (let i = 3; i < data.length; i += 4) if (data[i] < 255) return true;
  return false;
}

async function encodeBrowser(image: Decoded, width: number, height: number, mime: string, quality?: number): Promise<Blob | null> {
  const c = canvas2d(width, height);
  if (!c) return null;
  c.ctx.drawImage(image.source, 0, 0, width, height);
  if ("convertToBlob" in c.canvas) return c.canvas.convertToBlob({ type: mime, quality });
  const el = c.canvas as HTMLCanvasElement;
  return new Promise((resolve) => el.toBlob((blob) => resolve(blob), mime, quality));
}

const browserDeps: PrepareDeps = { decode: decodeBrowser, hasAlpha: hasAlphaBrowser, encode: encodeBrowser };

// --- the pipeline ----------------------------------------------------------------------

/** The pixel size to send: the longest side is at most `max`, never upscaled. */
export function targetSize(width: number, height: number, max = MAX_IMAGE_DIMENSION): { width: number; height: number } {
  const scale = Math.min(1, max / Math.max(width, height));
  return { width: Math.max(1, Math.round(width * scale)), height: Math.max(1, Math.round(height * scale)) };
}

/** What to do with an image of a known format, size and pixel size (the contract's decision table). */
export function needsReencode(format: ImageFormat, bytes: number, width: number, height: number): boolean {
  if (format === "heic" || format === "avif") return true;
  if (format === "gif") return false; // handled before: sent as is up to 8 MB
  return Math.max(width, height) > MAX_IMAGE_DIMENSION || bytes > PASSTHROUGH_BYTES;
}

function named(file: File, format: "png" | "jpeg" | "gif" | "webp"): File {
  // Keep the picked file when its name and type already say the right thing.
  if (file.type === MIME[format]) return file;
  const base = (file.name || "image").replace(/\.[^.]*$/, "") || "image";
  return new File([file], `${base}.${EXTENSION[format]}`, { type: MIME[format], lastModified: file.lastModified });
}

/**
 * Normalise an image for upload. Throws `PrepareImageError` with a problem the
 * UI explains; resolves with the file to send.
 */
export async function prepareImage(file: File, deps: PrepareDeps = browserDeps): Promise<PreparedImage> {
  if (file.size <= 0) throw new PrepareImageError("empty");
  if (file.size > MAX_INPUT_BYTES) throw new PrepareImageError("input_too_large");
  let format: ImageFormat | null;
  try {
    format = sniffFormat(await readHead(file));
  } catch {
    throw new PrepareImageError("empty");
  }
  if (!format) throw new PrepareImageError("not_image");

  if (format === "gif") {
    if (file.size > ATTACHMENT_MAX_BYTES) throw new PrepareImageError("too_large");
    return { file: named(file, "gif"), width: 0, height: 0, reencoded: false };
  }

  const image = await deps.decode(file);
  try {
    if (!image) {
      // HEIC/AVIF cannot be sent undecoded; a normal image the browser cannot read goes to the server within its limits.
      if (format === "heic" || format === "avif") throw new PrepareImageError("undecodable");
      if (file.size > PASSTHROUGH_BYTES) throw new PrepareImageError("undecodable");
      return { file: named(file, format), width: 0, height: 0, reencoded: false };
    }
    if (format === "heic" || format === "avif" || needsReencode(format, file.size, image.width, image.height)) {
      return await reencode(file, image, format, deps);
    }
    return { file: named(file, format), width: image.width, height: image.height, reencoded: false };
  } finally {
    image?.close?.();
  }
}

async function reencode(file: File, image: Decoded, format: ImageFormat, deps: PrepareDeps): Promise<PreparedImage> {
  // JPEG has no transparency; the other formats are checked.
  const alpha = format !== "jpeg" && deps.hasAlpha(image);
  const base = (file.name || "image").replace(/\.[^.]*$/, "") || "image";
  const size = targetSize(image.width, image.height);
  // Smaller steps when the result is still over the API limit (huge, noisy photos).
  for (const step of [1, 0.75, 0.5, 0.35]) {
    const w = Math.max(1, Math.round(size.width * step));
    const h = Math.max(1, Math.round(size.height * step));
    for (const quality of alpha ? [undefined] : [JPEG_QUALITY, 0.7]) {
      const blob = await deps.encode(image, w, h, alpha ? "image/webp" : "image/jpeg", quality);
      if (!blob) throw new PrepareImageError("undecodable");
      if (blob.size <= ATTACHMENT_MAX_BYTES) {
        // Browsers without WebP encoding return PNG for an image/webp request.
        const out: "png" | "jpeg" | "webp" = blob.type === "image/png" ? "png" : blob.type === "image/webp" ? "webp" : "jpeg";
        return { file: new File([blob], `${base}.${EXTENSION[out]}`, { type: MIME[out], lastModified: Date.now() }), width: w, height: h, reencoded: true };
      }
    }
  }
  throw new PrepareImageError("too_large");
}
