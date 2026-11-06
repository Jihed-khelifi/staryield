import { describe, expect, it } from "vitest"

import {
  clampCrop,
  initialCrop,
  isAcceptedType,
  largeEnough,
  maxZoom,
  MIN_PHOTO_SIDE,
  moveBy,
  outputSide,
  scaleAt,
  sourceRect,
  zoomTo,
} from "@/components/psychic/settings/photo-crop"

const VIEW = 264
const landscape = { width: 1600, height: 1200 }
const portrait = { width: 900, height: 1600 }

describe("photo crop", () => {
  it("accepts only the image types the API takes", () => {
    expect(isAcceptedType("image/jpeg")).toBe(true)
    expect(isAcceptedType("image/webp")).toBe(true)
    expect(isAcceptedType("image/gif")).toBe(false)
    expect(isAcceptedType("image/heic")).toBe(false)
  })

  it("fills the square with the shorter side at zoom 1", () => {
    expect(scaleAt(landscape, VIEW, 1)).toBeCloseTo(VIEW / 1200)
    const crop = initialCrop(landscape, VIEW)
    const rect = sourceRect(landscape, VIEW, crop)
    expect(rect.size).toBeCloseTo(1200)
    expect(rect.sx).toBeCloseTo(200) // centred horizontally
    expect(rect.sy).toBeCloseTo(0)
  })

  it("starts portraits above the centre, where the face is", () => {
    const rect = sourceRect(portrait, VIEW, initialCrop(portrait, VIEW))
    expect(rect.size).toBeCloseTo(900)
    expect(rect.sy).toBeCloseTo((1600 - 900) * 0.25)
    expect(rect.sy).toBeLessThan((1600 - 900) / 2)
  })

  it("never lets the image leave the square", () => {
    const crop = initialCrop(landscape, VIEW)
    const left = moveBy(landscape, VIEW, crop, 10_000, 10_000)
    expect(left.x).toBe(0)
    expect(left.y).toBe(0)
    const right = moveBy(landscape, VIEW, crop, -10_000, -10_000)
    const rect = sourceRect(landscape, VIEW, right)
    expect(rect.sx + rect.size).toBeCloseTo(1600)
    expect(rect.sy + rect.size).toBeCloseTo(1200)
  })

  it("zooms around the centre and keeps at least the minimum size", () => {
    expect(maxZoom(landscape)).toBe(4) // capped
    expect(maxZoom({ width: 800, height: 600 })).toBeCloseTo(600 / MIN_PHOTO_SIDE)
    expect(maxZoom({ width: 5000, height: 5000 })).toBe(4)
    expect(maxZoom({ width: 300, height: 300 })).toBeCloseTo(300 / 256)
    const zoomed = zoomTo(landscape, VIEW, initialCrop(landscape, VIEW), 2)
    const rect = sourceRect(landscape, VIEW, zoomed)
    expect(rect.size).toBeCloseTo(600)
    expect(rect.sx + rect.size / 2).toBeCloseTo(800) // same centre
    expect(rect.sy + rect.size / 2).toBeCloseTo(600)
    const tooFar = zoomTo(landscape, VIEW, zoomed, 50)
    expect(sourceRect(landscape, VIEW, tooFar).size).toBeGreaterThanOrEqual(MIN_PHOTO_SIDE - 0.001)
    expect(clampCrop(landscape, VIEW, { zoom: 0.2, x: 5, y: 5 }).zoom).toBe(1)
  })

  it("exports at the crop's resolution, capped at 1200 px", () => {
    expect(outputSide({ sx: 0, sy: 0, size: 600.4 })).toBe(600)
    expect(outputSide({ sx: 0, sy: 0, size: 3000 })).toBe(1200)
    expect(largeEnough({ width: 256, height: 900 })).toBe(true)
    expect(largeEnough({ width: 255, height: 900 })).toBe(false)
  })
})
