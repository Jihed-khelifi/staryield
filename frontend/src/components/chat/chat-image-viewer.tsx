"use client";
/* eslint-disable @next/next/no-img-element */
import * as React from "react";
import { Dialog as DialogPrimitive } from "radix-ui";
import { RotateCcw, X, ZoomIn, ZoomOut } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { Dialog, DialogDescription, DialogOverlay, DialogPortal, DialogTitle } from "@/components/ui/dialog";

export const MIN_ZOOM = 1;
export const MAX_ZOOM = 5;
const DOUBLE_TAP_ZOOM = 2.5;
const STEP = 1.5;

export const clampZoom = (z: number) => Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, z));

/** New zoom for a pinch: the start zoom scaled by how much the fingers moved apart. */
export function pinchZoom(startZoom: number, startDistance: number, distance: number): number {
  if (startDistance <= 0) return startZoom;
  return clampZoom(startZoom * (distance / startDistance));
}

type Point = { x: number; y: number };
const dist = (a: Point, b: Point) => Math.hypot(a.x - b.x, a.y - b.y);

function Stage({ src, alt, caption }: { src: string; alt: string; caption?: string }) {
  const { text } = useI18n();
  const [zoom, setZoom] = React.useState(1);
  const [offset, setOffset] = React.useState<Point>({ x: 0, y: 0 });
  const [dragging, setDragging] = React.useState(false);
  const pointers = React.useRef(new Map<number, Point>());
  const pinch = React.useRef<{ distance: number; zoom: number } | null>(null);
  const drag = React.useRef<{ start: Point; origin: Point } | null>(null);

  const apply = (z: number) => {
    const next = clampZoom(z);
    setZoom(next);
    if (next === 1) setOffset({ x: 0, y: 0 });
  };

  const onPointerDown = (e: React.PointerEvent) => {
    e.currentTarget.setPointerCapture?.(e.pointerId);
    pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY });
    setDragging(true);
    if (pointers.current.size === 2) {
      const [a, b] = [...pointers.current.values()];
      pinch.current = { distance: dist(a, b), zoom };
      drag.current = null;
    } else {
      drag.current = { start: { x: e.clientX, y: e.clientY }, origin: offset };
    }
  };
  const onPointerMove = (e: React.PointerEvent) => {
    if (!pointers.current.has(e.pointerId)) return;
    pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY });
    if (pointers.current.size >= 2 && pinch.current) {
      const [a, b] = [...pointers.current.values()];
      apply(pinchZoom(pinch.current.zoom, pinch.current.distance, dist(a, b)));
    } else if (drag.current && zoom > 1) {
      setOffset({ x: drag.current.origin.x + e.clientX - drag.current.start.x, y: drag.current.origin.y + e.clientY - drag.current.start.y });
    }
  };
  const onPointerEnd = (e: React.PointerEvent) => {
    pointers.current.delete(e.pointerId);
    pinch.current = null;
    drag.current = null;
    setDragging(false);
  };

  return (
    <>
      <div
        className="lightbox-stage"
        data-testid="lightbox-stage"
        data-zoom={zoom.toFixed(2)}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerEnd}
        onPointerCancel={onPointerEnd}
        onDoubleClick={() => apply(zoom > 1 ? 1 : DOUBLE_TAP_ZOOM)}
        onWheel={(e) => apply(zoom * Math.exp(-e.deltaY * 0.002))}
        onKeyDown={(e) => {
          if (e.key === "+" || e.key === "=") apply(zoom * STEP);
          else if (e.key === "-") apply(zoom / STEP);
          else if (e.key === "0") apply(1);
        }}
        tabIndex={0}
        role="group"
        aria-label={text("Image viewer. Pinch, scroll or use + and - to zoom.")}
      >
        <img
          src={src}
          alt={alt}
          className="lightbox-image"
          draggable={false}
          style={{ transform: `translate(${offset.x}px, ${offset.y}px) scale(${zoom})`, transition: dragging ? "none" : undefined }}
        />
      </div>
      {caption ? (
        <p id="lightbox-caption" className="lightbox-caption">
          {caption}
        </p>
      ) : null}
      <div className="lightbox-tools" role="group" aria-label={text("Zoom")}>
        <button type="button" onClick={() => apply(zoom / STEP)} disabled={zoom <= MIN_ZOOM} aria-label={text("Zoom out")}>
          <ZoomOut size={18} aria-hidden />
        </button>
        <button type="button" onClick={() => apply(1)} disabled={zoom === 1} aria-label={text("Reset zoom")}>
          <RotateCcw size={16} aria-hidden />
        </button>
        <button type="button" onClick={() => apply(zoom * STEP)} disabled={zoom >= MAX_ZOOM} aria-label={text("Zoom in")}>
          <ZoomIn size={18} aria-hidden />
        </button>
      </div>
    </>
  );
}

/** Full-screen image viewer: focus trap, Esc and close button, alt text, pinch/wheel/double-tap zoom. */
export function ChatImageViewer({
  open,
  onOpenChange,
  src,
  alt,
  caption,
  returnFocusTo,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  src: string | null;
  alt: string;
  caption?: string;
  /** Focus goes back here on close, even when a tap opened the viewer without focusing the trigger. */
  returnFocusTo?: React.RefObject<HTMLElement | null>;
}) {
  const { text } = useI18n();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogPortal>
        <DialogOverlay overlay="dim" className="lightbox-overlay" />
        <DialogPrimitive.Content
          className="lightbox-content"
          aria-describedby={caption ? "lightbox-caption" : "lightbox-alt"}
          data-testid="image-viewer"
          onCloseAutoFocus={(event) => {
            const target = returnFocusTo?.current;
            if (!target) return;
            event.preventDefault();
            target.focus({ preventScroll: true });
          }}
        >
          <DialogTitle className="sr-only">{text("Image")}</DialogTitle>
          {caption ? null : (
            <DialogDescription id="lightbox-alt" className="sr-only">
              {alt}
            </DialogDescription>
          )}
          {src ? <Stage src={src} alt={alt} caption={caption} /> : null}
          <DialogPrimitive.Close asChild>
            <button type="button" className="lightbox-close" aria-label={text("Close image")}>
              <X size={20} aria-hidden />
            </button>
          </DialogPrimitive.Close>
        </DialogPrimitive.Content>
      </DialogPortal>
    </Dialog>
  );
}
