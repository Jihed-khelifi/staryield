// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  MAX_INPUT_BYTES,
  PASSTHROUGH_BYTES,
  PrepareImageError,
  needsReencode,
  prepareImage,
  sniffFormat,
  targetSize,
  type Decoded,
  type PrepareDeps,
} from "@/lib/chat/prepare-image";

const bytes = (...b: number[]) => new Uint8Array(b);
const ascii = (s: string) => Array.from(s, (c) => c.charCodeAt(0));
const PNG = [0x89, ...ascii("PNG"), 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0];
const JPEG = [0xff, 0xd8, 0xff, 0xe0, 0, 0, 0, 0, 0, 0, 0, 0];
const GIF = [...ascii("GIF89a"), 0, 0, 0, 0, 0, 0];
const WEBP = [...ascii("RIFF"), 0, 0, 0, 0, ...ascii("WEBP")];
const ftyp = (brand: string) => [0, 0, 0, 24, ...ascii("ftyp"), ...ascii(brand)];

function file(head: number[], opts: { name?: string; type?: string; size?: number } = {}) {
  const f = new File([new Uint8Array(head)], opts.name ?? "photo", { type: opts.type ?? "" });
  if (opts.size !== undefined) Object.defineProperty(f, "size", { value: opts.size });
  return f;
}

const blob = (size: number, type: string) => {
  const b = new Blob([new Uint8Array(1)], { type });
  Object.defineProperty(b, "size", { value: size });
  return b;
};

function deps(over: Partial<PrepareDeps> & { width?: number; height?: number; alpha?: boolean; decoded?: boolean } = {}) {
  const decode = vi.fn(async (): Promise<Decoded | null> => (over.decoded === false ? null : { width: over.width ?? 4000, height: over.height ?? 3000, source: {} as CanvasImageSource, close: vi.fn() }));
  const encode = vi.fn(async (_i: Decoded, _w: number, _h: number, mime: string) => blob(400_000, mime));
  const hasAlpha = vi.fn(() => over.alpha ?? false);
  const d: PrepareDeps = { decode: over.decode ?? decode, encode: over.encode ?? encode, hasAlpha: over.hasAlpha ?? hasAlpha };
  return { d, decode, encode, hasAlpha };
}

afterEach(() => vi.restoreAllMocks());

describe("sniffFormat (the real type from the bytes)", () => {
  it.each([
    ["png", PNG],
    ["jpeg", JPEG],
    ["gif", GIF],
    ["webp", WEBP],
    ["heic", ftyp("heic")],
    ["heic", ftyp("mif1")],
    ["avif", ftyp("avif")],
  ])("%s", (format, head) => expect(sniffFormat(bytes(...head))).toBe(format));

  it("returns null for anything else", () => {
    expect(sniffFormat(bytes(...ascii("%PDF-1.7 hello world")))).toBeNull();
    expect(sniffFormat(bytes(1, 2, 3))).toBeNull();
    expect(sniffFormat(bytes(...ascii("RIFF....WAVE")))).toBeNull();
    expect(sniffFormat(bytes(...ftyp("mp42")))).toBeNull();
  });
});

describe("decision table", () => {
  it("re-encodes above 2048 px, above 1.5 MB, and for HEIC/AVIF; sends the rest as is", () => {
    expect(needsReencode("jpeg", 500_000, 2048, 1536)).toBe(false);
    expect(needsReencode("png", PASSTHROUGH_BYTES, 1000, 1000)).toBe(false);
    expect(needsReencode("webp", PASSTHROUGH_BYTES + 1, 1000, 1000)).toBe(true);
    expect(needsReencode("jpeg", 500_000, 2049, 100)).toBe(true);
    expect(needsReencode("jpeg", 500_000, 100, 2049)).toBe(true);
    expect(needsReencode("heic", 100, 10, 10)).toBe(true);
    expect(needsReencode("avif", 100, 10, 10)).toBe(true);
  });

  it("scales the longest side to 2048 without upscaling", () => {
    expect(targetSize(4000, 3000)).toEqual({ width: 2048, height: 1536 });
    expect(targetSize(3000, 4000)).toEqual({ width: 1536, height: 2048 });
    expect(targetSize(800, 600)).toEqual({ width: 800, height: 600 });
    expect(targetSize(10_000, 1)).toEqual({ width: 2048, height: 1 });
  });
});

describe("prepareImage", () => {
  it("accepts a file with an empty type by its bytes, and names it right", async () => {
    const { d, decode } = deps({ width: 800, height: 600 });
    const out = await prepareImage(file(PNG, { name: "IMG_0001", type: "" }), d);
    expect(decode).toHaveBeenCalled();
    expect(out.reencoded).toBe(false);
    expect(out.file.type).toBe("image/png");
    expect(out.file.name).toBe("IMG_0001.png");
    expect(out).toMatchObject({ width: 800, height: 600 });
  });

  it("ignores a lying file.type", async () => {
    const { d } = deps({ width: 100, height: 100 });
    const out = await prepareImage(file(JPEG, { name: "a.png", type: "image/png" }), d);
    expect(out.file.type).toBe("image/jpeg");
    expect(out.file.name).toBe("a.jpg");
  });

  it("keeps the picked file when it is small, in range and correctly typed", async () => {
    const f = file(JPEG, { name: "a.jpg", type: "image/jpeg" });
    const { d, encode } = deps({ width: 1200, height: 800 });
    const out = await prepareImage(f, d);
    expect(out.file).toBe(f);
    expect(encode).not.toHaveBeenCalled();
  });

  it("downscales a 4000x3000 photo to 2048x1536 JPEG q 0.85", async () => {
    const { d, encode } = deps({ width: 4000, height: 3000 });
    const out = await prepareImage(file(JPEG, { name: "big.jpg", type: "image/jpeg", size: 6_000_000 }), d);
    expect(encode).toHaveBeenCalledWith(expect.anything(), 2048, 1536, "image/jpeg", 0.85);
    expect(out).toMatchObject({ reencoded: true, width: 2048, height: 1536 });
    expect(out.file.type).toBe("image/jpeg");
    expect(out.file.name).toBe("big.jpg");
  });

  it("re-encodes a large opaque PNG as JPEG and a transparent one as WebP", async () => {
    const opaque = deps({ width: 1000, height: 1000, alpha: false });
    await prepareImage(file(PNG, { type: "image/png", size: 3_000_000 }), opaque.d);
    expect(opaque.encode).toHaveBeenCalledWith(expect.anything(), 1000, 1000, "image/jpeg", 0.85);
    const clear = deps({ width: 3000, height: 1000, alpha: true });
    const out = await prepareImage(file(PNG, { type: "image/png", size: 3_000_000 }), clear.d);
    expect(clear.encode).toHaveBeenCalledWith(expect.anything(), 2048, 683, "image/webp", undefined);
    expect(out.file.type).toBe("image/webp");
  });

  it("keeps PNG when the browser cannot encode WebP (it hands back a PNG)", async () => {
    const { d } = deps({ width: 3000, height: 1000, alpha: true, encode: async () => blob(500_000, "image/png") });
    const out = await prepareImage(file(PNG, { type: "image/png", size: 3_000_000 }), d);
    expect(out.file.type).toBe("image/png");
    expect(out.file.name.endsWith(".png")).toBe(true);
  });

  it("never checks transparency for a JPEG", async () => {
    const { d, hasAlpha } = deps({ width: 5000, height: 4000 });
    await prepareImage(file(JPEG, { size: 9_000_000 }), d);
    expect(hasAlpha).not.toHaveBeenCalled();
  });

  it("converts HEIC through the decoder", async () => {
    const { d, encode } = deps({ width: 3024, height: 4032 });
    const out = await prepareImage(file(ftyp("heic"), { name: "IMG_1.HEIC", type: "" }), d);
    expect(encode).toHaveBeenCalledWith(expect.anything(), 1536, 2048, "image/jpeg", 0.85);
    expect(out.file.name).toBe("IMG_1.jpg");
  });

  it("explains a HEIC (or AVIF) the browser cannot decode", async () => {
    const { d } = deps({ decoded: false });
    await expect(prepareImage(file(ftyp("heic"), { type: "image/heic" }), d)).rejects.toMatchObject({ problem: "undecodable" });
    await expect(prepareImage(file(ftyp("avif")), d)).rejects.toMatchObject({ problem: "undecodable" });
  });

  it("lowers quality, then size, when the result is still over 8 MB; fails clearly if it never fits", async () => {
    const sizes = [9_000_000, 9_000_000, 7_000_000];
    const encode = vi.fn<PrepareDeps["encode"]>(async (_i, _w, _h, mime) => blob(sizes.shift() ?? 9_000_000, mime));
    const { d } = deps({ width: 4000, height: 3000, encode });
    const out = await prepareImage(file(JPEG, { size: 12_000_000 }), d);
    expect(encode.mock.calls.map((c) => [c[1], c[4]])).toEqual([[2048, 0.85], [2048, 0.7], [1536, 0.85]]);
    expect(out.reencoded).toBe(true);
    const never = deps({ width: 4000, height: 3000, encode: async (_i, _w, _h, mime) => blob(20_000_000, mime) });
    await expect(prepareImage(file(JPEG, { size: 12_000_000 }), never.d)).rejects.toMatchObject({ problem: "too_large" });
  });

  it("passes a GIF up to 8 MB through untouched (no decoding), and refuses a larger one", async () => {
    const f = file(GIF, { name: "wave.gif", type: "image/gif", size: 7_900_000 });
    const { d, decode } = deps();
    const out = await prepareImage(f, d);
    expect(out.file).toBe(f);
    expect(out.reencoded).toBe(false);
    expect(decode).not.toHaveBeenCalled();
    await expect(prepareImage(file(GIF, { size: 8 * 1024 * 1024 + 1 }), d)).rejects.toMatchObject({ problem: "too_large" });
  });

  it("refuses files above 40 MB before decoding anything", async () => {
    const { d, decode } = deps();
    const huge = file(JPEG, { size: MAX_INPUT_BYTES + 1 });
    await expect(prepareImage(huge, d)).rejects.toMatchObject({ problem: "input_too_large" });
    expect(decode).not.toHaveBeenCalled();
    // The boundary itself is allowed.
    await expect(prepareImage(file(JPEG, { size: MAX_INPUT_BYTES }), deps({ width: 100, height: 100 }).d)).resolves.toMatchObject({ reencoded: true });
  });

  it("refuses non-images and empty files", async () => {
    const { d } = deps();
    await expect(prepareImage(file(ascii("%PDF-1.7 hello"), { type: "application/pdf" }), d)).rejects.toBeInstanceOf(PrepareImageError);
    await expect(prepareImage(file(ascii("%PDF-1.7 hello"), { type: "image/png" }), d)).rejects.toMatchObject({ problem: "not_image" });
    await expect(prepareImage(new File([], "empty.png", { type: "image/png" }), d)).rejects.toMatchObject({ problem: "empty" });
  });

  it("sends a small image the browser cannot decode as is (the server decides), but not a large one", async () => {
    const { d } = deps({ decoded: false });
    const small = file(PNG, { type: "image/png", size: 100_000 });
    await expect(prepareImage(small, d)).resolves.toMatchObject({ file: small, reencoded: false });
    await expect(prepareImage(file(PNG, { size: 5_000_000 }), d)).rejects.toMatchObject({ problem: "undecodable" });
  });

  it("releases the decoded image", async () => {
    const close = vi.fn();
    const { d } = deps({ decode: async () => ({ width: 5000, height: 5000, source: {} as CanvasImageSource, close }) });
    await prepareImage(file(JPEG, { size: 9_000_000 }), d);
    expect(close).toHaveBeenCalledOnce();
  });

  it("default decoding: createImageBitmap with the EXIF orientation, then the <img> fallback, then undecodable", async () => {
    const bitmap = { width: 3000, height: 2000, close: vi.fn() };
    const cib = vi.fn(async () => bitmap);
    vi.stubGlobal("createImageBitmap", cib);
    // jsdom has no canvas: the pipeline stops at encoding, which is what we want to see reached.
    const probe = file(JPEG, { type: "image/jpeg", size: 4_000_000 });
    await prepareImage(probe).catch(() => undefined);
    expect(cib).toHaveBeenCalledWith(probe, { imageOrientation: "from-image" });
    expect(bitmap.close).toHaveBeenCalled();

    // createImageBitmap rejects and the <img> cannot load either: HEIC is reported as undecodable.
    vi.stubGlobal("createImageBitmap", vi.fn(async () => Promise.reject(new Error("unsupported"))));
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: vi.fn(() => "blob:x"), revokeObjectURL: vi.fn() }));
    const OriginalImage = globalThis.Image;
    vi.stubGlobal(
      "Image",
      class {
        onload?: () => void;
        onerror?: () => void;
        set src(_v: string) {
          queueMicrotask(() => this.onerror?.());
        }
      },
    );
    await expect(prepareImage(file(ftyp("heic"), { name: "a.heic", type: "" }))).rejects.toMatchObject({ problem: "undecodable" });
    vi.stubGlobal("Image", OriginalImage);
    vi.unstubAllGlobals();
  });
});
