"use client";

import * as React from "react";

import { logger } from "@/lib/logger";
import { renderShareCard } from "@/lib/share/render-card";
import type { ShareCardSpec } from "@/lib/share/card-spec";

export type ShareImageState =
  | { status: "idle" }
  | { status: "rendering" }
  | { status: "ready"; blob: Blob; url: string }
  | { status: "error" };

type Render = (spec: ShareCardSpec) => Promise<Blob>;

/** What the last render produced, and for which card and attempt. */
type Rendered = { spec: ShareCardSpec; attempt: number } & ({ blob: Blob; url: string } | { failed: true });

/**
 * Renders the share image while `active` (the dialog is open) and owns its
 * object URL: it is revoked when the dialog closes, the card changes, a retry
 * starts or the component unmounts. The status is derived from the last
 * render (it belongs to the current card and attempt or it is stale), so the
 * effect only sets state from the render's callbacks. `retry` renders again
 * after a failure.
 */
export function useShareImage(spec: ShareCardSpec, active: boolean, render: Render = renderShareCard) {
  const [attempt, setAttempt] = React.useState(0);
  const [rendered, setRendered] = React.useState<Rendered | null>(null);
  const renderRef = React.useRef(render);
  React.useEffect(() => {
    renderRef.current = render;
  });

  React.useEffect(() => {
    if (!active) return;
    let cancelled = false;
    let url: string | null = null;
    renderRef.current(spec).then(
      (blob) => {
        if (cancelled) return;
        url = URL.createObjectURL(blob);
        setRendered({ spec, attempt, blob, url });
      },
      (error: unknown) => {
        if (cancelled) return;
        logger.warn("share image could not be rendered", error);
        setRendered({ spec, attempt, failed: true });
      },
    );
    return () => {
      cancelled = true;
      if (url) URL.revokeObjectURL(url);
      setRendered(null);
    };
  }, [active, spec, attempt]);

  const retry = React.useCallback(() => setAttempt((n) => n + 1), []);
  let state: ShareImageState;
  if (!active) state = { status: "idle" };
  else if (!rendered || rendered.spec !== spec || rendered.attempt !== attempt) state = { status: "rendering" };
  else state = "failed" in rendered ? { status: "error" } : { status: "ready", blob: rendered.blob, url: rendered.url };
  return { state, retry };
}
