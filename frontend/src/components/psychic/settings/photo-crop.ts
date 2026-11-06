/**
 * Square crop of the advisor photo, done in the browser before upload.
 *
 * The image is shown inside a square viewport of `view` CSS pixels. At zoom
 * 1 its shorter side exactly fills the viewport; zooming scales it around
 * the viewport centre. `x`/`y` are the image's top-left offset inside the
 * viewport (≤ 0), clamped so the image always covers the viewport. The
 * crop sent to the server is the part of the image under the viewport.
 */

/** Smallest photo side the API accepts (pixels). */
export const MIN_PHOTO_SIDE = 256
/** Longest side the API keeps; larger crops are scaled down to it. */
export const OUTPUT_MAX_SIDE = 1200
/** Upper bound of the zoom slider. */
export const MAX_ZOOM = 4

export type ImageSize = { width: number; height: number }
export type CropState = { zoom: number; x: number; y: number }
export type SourceRect = { sx: number; sy: number; size: number }

/** Accepted upload types (the server sniffs the content again). */
export const ACCEPTED_TYPES = ["image/jpeg", "image/png", "image/webp"] as const

export function isAcceptedType(type: string): boolean {
  return (ACCEPTED_TYPES as readonly string[]).includes(type)
}

/** Scale of the image at a zoom level, in viewport pixels per image pixel. */
export function scaleAt(image: ImageSize, view: number, zoom: number): number {
  return (view / Math.min(image.width, image.height)) * zoom
}

/**
 * The largest zoom that still leaves a crop of at least MIN_PHOTO_SIDE
 * image pixels (so the upload is never too small), capped at MAX_ZOOM.
 */
export function maxZoom(image: ImageSize): number {
  const short = Math.min(image.width, image.height)
  return Math.max(1, Math.min(MAX_ZOOM, short / MIN_PHOTO_SIDE))
}

/** Whether a photo is big enough to crop to the minimum size at all. */
export function largeEnough(image: ImageSize): boolean {
  return Math.min(image.width, image.height) >= MIN_PHOTO_SIDE
}

/** Keep the image covering the whole viewport. */
export function clampCrop(image: ImageSize, view: number, crop: CropState): CropState {
  const zoom = Math.min(Math.max(crop.zoom, 1), maxZoom(image))
  const scale = scaleAt(image, view, zoom)
  const minX = view - image.width * scale
  const minY = view - image.height * scale
  return {
    zoom,
    x: Math.min(0, Math.max(minX, crop.x)),
    y: Math.min(0, Math.max(minY, crop.y)),
  }
}

/**
 * The starting crop: zoom 1, centred horizontally. Portraits start a little
 * above centre, because the face belongs in the upper half of the photo.
 */
export function initialCrop(image: ImageSize, view: number): CropState {
  const scale = scaleAt(image, view, 1)
  const x = (view - image.width * scale) / 2
  const spareY = view - image.height * scale // ≤ 0
  const y = image.height > image.width ? spareY * 0.25 : spareY / 2
  return clampCrop(image, view, { zoom: 1, x, y })
}

/** Zoom to `zoom`, keeping the viewport centre on the same image point. */
export function zoomTo(image: ImageSize, view: number, crop: CropState, zoom: number): CropState {
  const before = scaleAt(image, view, crop.zoom)
  const next = Math.min(Math.max(zoom, 1), maxZoom(image))
  const after = scaleAt(image, view, next)
  const centre = view / 2
  const imgX = (centre - crop.x) / before
  const imgY = (centre - crop.y) / before
  return clampCrop(image, view, { zoom: next, x: centre - imgX * after, y: centre - imgY * after })
}

/** Move the image by a delta in viewport pixels. */
export function moveBy(image: ImageSize, view: number, crop: CropState, dx: number, dy: number): CropState {
  return clampCrop(image, view, { ...crop, x: crop.x + dx, y: crop.y + dy })
}

/** The square of the source image under the viewport (image pixels). */
export function sourceRect(image: ImageSize, view: number, crop: CropState): SourceRect {
  const scale = scaleAt(image, view, crop.zoom)
  const size = Math.min(view / scale, image.width, image.height)
  const sx = Math.min(Math.max(-crop.x / scale, 0), image.width - size)
  const sy = Math.min(Math.max(-crop.y / scale, 0), image.height - size)
  return { sx, sy, size }
}

/** Side of the exported square: the crop's own resolution, at most OUTPUT_MAX_SIDE. */
export function outputSide(rect: SourceRect): number {
  return Math.max(1, Math.min(OUTPUT_MAX_SIDE, Math.round(rect.size)))
}

/**
 * Draw the crop to a canvas and encode it as JPEG. Browsers apply the
 * photo's EXIF orientation when drawing, and the server strips metadata
 * again when it re-encodes the upload.
 */
export async function renderCrop(img: HTMLImageElement, rect: SourceRect, quality = 0.9): Promise<Blob> {
  const side = outputSide(rect)
  const canvas = document.createElement("canvas")
  canvas.width = side
  canvas.height = side
  const ctx = canvas.getContext("2d")
  if (!ctx) throw new Error("Canvas is not available")
  ctx.fillStyle = "#ffffff" // transparent PNGs become white, like on the server
  ctx.fillRect(0, 0, side, side)
  ctx.imageSmoothingQuality = "high"
  ctx.drawImage(img, rect.sx, rect.sy, rect.size, rect.size, 0, 0, side, side)
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error("The photo could not be encoded"))), "image/jpeg", quality)
  })
}
