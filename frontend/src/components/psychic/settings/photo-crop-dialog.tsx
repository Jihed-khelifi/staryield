"use client"
/* eslint-disable @next/next/no-img-element */

import * as React from "react"
import { Loader2, ZoomIn, ZoomOut } from "lucide-react"

import { useI18n } from "@/i18n/i18n-provider"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import {
  clampCrop,
  initialCrop,
  largeEnough,
  maxZoom,
  MIN_PHOTO_SIDE,
  moveBy,
  renderCrop,
  scaleAt,
  sourceRect,
  zoomTo,
  type CropState,
  type ImageSize,
} from "@/components/psychic/settings/photo-crop"

/** Side of the square crop viewport in CSS pixels (fits a 390px phone). */
export const CROP_VIEW = 264

type Loaded = { url: string; size: ImageSize } | { url: string; error: "unreadable" | "small" }

/** Read a picked file into an object URL and its natural size. */
function useImageFile(file: File | null) {
  const [loaded, setLoaded] = React.useState<{ file: File; value: Loaded } | null>(null)
  React.useEffect(() => {
    if (!file) return
    const url = URL.createObjectURL(file)
    const img = new Image()
    let live = true
    img.onload = () => {
      if (!live) return
      const size = { width: img.naturalWidth, height: img.naturalHeight }
      setLoaded({ file, value: largeEnough(size) ? { url, size } : { url, error: "small" } })
    }
    img.onerror = () => {
      if (live) setLoaded({ file, value: { url, error: "unreadable" } })
    }
    img.src = url
    return () => {
      live = false
      URL.revokeObjectURL(url)
    }
  }, [file])
  return loaded && loaded.file === file ? loaded.value : null
}

/**
 * Square crop before upload: drag (or use the arrow keys) to place the
 * face inside the oval in the upper half, zoom with the slider (or + and
 * −). Confirming encodes the crop as JPEG in the browser; the server
 * re-encodes and checks it again.
 */
export function PhotoCropDialog({
  file,
  busy,
  onCancel,
  onConfirm,
}: {
  file: File | null
  busy: boolean
  onCancel: () => void
  onConfirm: (photo: Blob) => void
}) {
  const { text } = useI18n()
  const image = useImageFile(file)
  const ready = image && "size" in image ? image : null
  const [crop, setCrop] = React.useState<CropState | null>(null)
  const [failed, setFailed] = React.useState(false)
  const imgRef = React.useRef<HTMLImageElement>(null)
  const drag = React.useRef<{ id: number; x: number; y: number } | null>(null)
  const helpId = React.useId()
  const zoomId = React.useId()
  const maskId = `crop-mask-${helpId.replace(/[^a-zA-Z0-9_-]/g, "")}`

  // A new picture starts from its own centred crop (state adjusted while rendering).
  const [cropFor, setCropFor] = React.useState<string | null>(null)
  if (ready && cropFor !== ready.url) {
    setCropFor(ready.url)
    setCrop(initialCrop(ready.size, CROP_VIEW))
    setFailed(false)
  }

  const size = ready?.size
  const current = size && crop ? clampCrop(size, CROP_VIEW, crop) : null
  const scale = size && current ? scaleAt(size, CROP_VIEW, current.zoom) : 1
  const zoomMax = size ? maxZoom(size) : 1

  function update(next: (size: ImageSize, crop: CropState) => CropState) {
    if (!size || !current) return
    setCrop(next(size, current))
  }

  async function confirm() {
    if (!size || !current || !imgRef.current) return
    try {
      const blob = await renderCrop(imgRef.current, sourceRect(size, CROP_VIEW, current))
      setFailed(false)
      onConfirm(blob)
    } catch {
      setFailed(true)
    }
  }

  const problem =
    image && "error" in image
      ? image.error === "small"
        ? text("Use a photo at least {value0} pixels wide and {value0} pixels high.", { value0: MIN_PHOTO_SIDE })
        : text("This image could not be read.")
      : failed
        ? text("The photo could not be prepared. Try another image.")
        : ""

  return (
    <Dialog open={Boolean(file)} onOpenChange={(open) => !open && !busy && onCancel()}>
      <DialogContent size="md" data-testid="photo-crop-dialog" onInteractOutside={(event) => busy && event.preventDefault()}>
        <DialogHeader>
          <DialogTitle>{text("Crop your photo")}</DialogTitle>
          <DialogDescription>{text("Make sure your face is clearly displayed in the upper half of the image")}</DialogDescription>
        </DialogHeader>

        {problem ? (
          <p role="alert" className="field-message" data-tone="error">
            {problem}
          </p>
        ) : null}

        {ready && current ? (
          <div className="photo-crop">
            <div
              className="photo-crop-viewport"
              style={{ width: CROP_VIEW, height: CROP_VIEW }}
              role="group"
              tabIndex={0}
              aria-roledescription={text("Photo cropper")}
              aria-label={text("Photo position")}
              aria-describedby={helpId}
              data-testid="photo-crop-viewport"
              onPointerDown={(event) => {
                if (busy) return
                event.currentTarget.setPointerCapture(event.pointerId)
                drag.current = { id: event.pointerId, x: event.clientX, y: event.clientY }
              }}
              onPointerMove={(event) => {
                const d = drag.current
                if (!d || d.id !== event.pointerId) return
                const dx = event.clientX - d.x
                const dy = event.clientY - d.y
                drag.current = { id: d.id, x: event.clientX, y: event.clientY }
                update((s, c) => moveBy(s, CROP_VIEW, c, dx, dy))
              }}
              onPointerUp={() => (drag.current = null)}
              onPointerCancel={() => (drag.current = null)}
              onKeyDown={(event) => {
                const step = event.shiftKey ? 32 : 8
                const moves: Record<string, [number, number]> = {
                  ArrowLeft: [step, 0],
                  ArrowRight: [-step, 0],
                  ArrowUp: [0, step],
                  ArrowDown: [0, -step],
                }
                if (moves[event.key]) {
                  event.preventDefault()
                  const [dx, dy] = moves[event.key]
                  update((s, c) => moveBy(s, CROP_VIEW, c, dx, dy))
                } else if (event.key === "+" || event.key === "=") {
                  event.preventDefault()
                  update((s, c) => zoomTo(s, CROP_VIEW, c, c.zoom + 0.1))
                } else if (event.key === "-" || event.key === "_") {
                  event.preventDefault()
                  update((s, c) => zoomTo(s, CROP_VIEW, c, c.zoom - 0.1))
                }
              }}
            >
              <img
                ref={imgRef}
                src={ready.url}
                alt=""
                draggable={false}
                className="photo-crop-image"
                style={{
                  width: ready.size.width * scale,
                  height: ready.size.height * scale,
                  transform: `translate(${current.x}px, ${current.y}px)`,
                }}
              />
              {/* Face guide: an oval in the upper half, the rest dimmed. */}
              <svg className="photo-crop-guide" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden>
                <defs>
                  <mask id={maskId}>
                    <rect width="100" height="100" fill="white" />
                    <ellipse cx="50" cy="40" rx="25" ry="31" fill="black" />
                  </mask>
                </defs>
                <rect width="100" height="100" fill="rgb(21 16 11 / 0.35)" mask={`url(#${maskId})`} />
                <ellipse cx="50" cy="40" rx="25" ry="31" fill="none" stroke="white" strokeWidth="0.8" strokeDasharray="2 1.5" />
                <line x1="0" y1="50" x2="100" y2="50" stroke="white" strokeOpacity="0.55" strokeWidth="0.4" strokeDasharray="1.5 1.5" />
              </svg>
            </div>
            <p id={helpId} className="photo-crop-help">
              {text("Drag the photo, or use the arrow keys, to fit your face inside the oval. Zoom with the slider or the + and − keys.")}
            </p>
            <div className="photo-crop-zoom">
              <ZoomOut aria-hidden className="size-4 shrink-0" />
              <label htmlFor={zoomId} className="sr-only">
                {text("Zoom")}
              </label>
              <input
                id={zoomId}
                type="range"
                min={1}
                max={zoomMax}
                step={0.01}
                value={current.zoom}
                disabled={busy || zoomMax <= 1}
                aria-valuetext={`${Math.round(current.zoom * 100)}%`}
                onChange={(event) => {
                  const zoom = Number(event.target.value)
                  update((s, c) => zoomTo(s, CROP_VIEW, c, zoom))
                }}
              />
              <ZoomIn aria-hidden className="size-4 shrink-0" />
            </div>
          </div>
        ) : !problem ? (
          <p role="status" className="flex items-center gap-2 text-sm">
            <Loader2 aria-hidden className="size-4 animate-spin motion-reduce:animate-none" />
            {text("Preparing your photo…")}
          </p>
        ) : null}

        <DialogFooter>
          <Button type="button" variant="outline" size="lg" className="border-ink/40 px-4" onClick={onCancel} disabled={busy}>
            {text("Cancel")}
          </Button>
          <Button
            type="button"
            variant="cta"
            size="lg"
            className="px-4"
            onClick={() => void confirm()}
            disabled={busy || !ready}
            aria-busy={busy || undefined}
            data-testid="photo-crop-confirm"
          >
            {busy ? <Loader2 aria-hidden className="size-4 animate-spin motion-reduce:animate-none" /> : null}
            {text(busy ? "Uploading…" : "Use this photo")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
