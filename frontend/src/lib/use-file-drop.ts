"use client";
import * as React from "react";

/** True when a drag carries files (not text or a link dragged inside the page). */
export function dragHasFiles(e: { dataTransfer?: DataTransfer | null }): boolean {
  return Array.from(e.dataTransfer?.types ?? []).includes("Files");
}

export type FileDropOptions = {
  /** Receives the dropped files (all of them; the caller picks). */
  onFiles: (files: File[]) => void;
  /** Dropping is not possible right now: the drop is swallowed (never navigates) and `onBlocked` says why. */
  disabled?: boolean;
  onBlocked?: () => void;
};

export type FileDrop = {
  /** A file is being dragged over the target (never while `disabled`). */
  dragging: boolean;
  /** Spread on the drop target element. */
  handlers: {
    onDragEnter: (e: React.DragEvent) => void;
    onDragOver: (e: React.DragEvent) => void;
    onDragLeave: (e: React.DragEvent) => void;
    onDrop: (e: React.DragEvent) => void;
  };
};

/**
 * One drag-and-drop implementation for file drops. A depth counter tracks
 * `dragenter`/`dragleave` (they fire for every child element, so a plain
 * boolean flickers), only drags that carry files count, and the drop effect is
 * "copy". `window` `drop`/`dragend` reset the state when a drag ends elsewhere.
 */
export function useFileDrop({ onFiles, disabled = false, onBlocked }: FileDropOptions): FileDrop {
  const [dragging, setDragging] = React.useState(false);
  const depth = React.useRef(0);
  const latest = React.useRef({ onFiles, onBlocked });
  React.useEffect(() => {
    latest.current = { onFiles, onBlocked };
  });

  const reset = React.useCallback(() => {
    depth.current = 0;
    setDragging(false);
  }, []);

  React.useEffect(() => {
    window.addEventListener("drop", reset);
    window.addEventListener("dragend", reset);
    return () => {
      window.removeEventListener("drop", reset);
      window.removeEventListener("dragend", reset);
    };
  }, [reset]);
  // Becoming disabled mid-drag removes the overlay (adjusted during render).
  if (disabled && dragging) setDragging(false);

  const handlers: FileDrop["handlers"] = {
    onDragEnter: (e) => {
      if (!dragHasFiles(e)) return;
      e.preventDefault();
      depth.current += 1;
      if (!disabled) setDragging(true);
    },
    onDragOver: (e) => {
      if (!dragHasFiles(e)) return;
      e.preventDefault();
      e.dataTransfer.dropEffect = disabled ? "none" : "copy";
    },
    onDragLeave: (e) => {
      if (!dragHasFiles(e)) return;
      depth.current = Math.max(0, depth.current - 1);
      if (depth.current === 0) setDragging(false);
    },
    onDrop: (e) => {
      if (!dragHasFiles(e)) return;
      e.preventDefault();
      reset();
      const files = Array.from(e.dataTransfer.files ?? []);
      if (disabled) latest.current.onBlocked?.();
      else if (files.length) latest.current.onFiles(files);
    },
  };
  return { dragging: dragging && !disabled, handlers };
}

/**
 * While mounted, a file dropped anywhere on the page outside a drop target
 * does nothing instead of navigating the browser to the file.
 */
export function useBlockStrayFileDrops(active = true) {
  React.useEffect(() => {
    if (!active) return;
    const guard = (e: DragEvent) => {
      // A drop target already took it (its handlers ran first and called preventDefault).
      if (e.defaultPrevented || !e.dataTransfer || !Array.from(e.dataTransfer.types ?? []).includes("Files")) return;
      // Real file inputs keep their native behaviour.
      if ((e.target as Element | null)?.closest?.("input[type=file]")) return;
      e.preventDefault();
      if (e.type === "dragover") e.dataTransfer.dropEffect = "none";
    };
    window.addEventListener("dragover", guard);
    window.addEventListener("drop", guard);
    return () => {
      window.removeEventListener("dragover", guard);
      window.removeEventListener("drop", guard);
    };
  }, [active]);
}
