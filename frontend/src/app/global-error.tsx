"use client";

import { useEffect } from "react";

import { logger } from "@/lib/logger";

/**
 * Last-resort boundary for failures in the root layout (before i18n is
 * available). Kept minimal and self-contained, with inline brand styling.
 */
export default function GlobalError({ error, retry }: { error: Error & { digest?: string }; retry: () => void }) {
  useEffect(() => {
    logger.error(`root layout failed: ${error.message}`, error, { code: "root_failed", digest: error.digest ?? "" });
  }, [error]);
  return (
    <html lang="en">
      <body style={{ margin: 0, minHeight: "100dvh", display: "grid", placeItems: "center", background: "#e0d5c3", color: "#15100b", fontFamily: "Georgia, serif" }}>
        <main role="alert" style={{ textAlign: "center", padding: 24 }}>
          {/* i18n is unavailable when the root layout fails; English fallback. */}
          <h1 style={{ fontWeight: 400, letterSpacing: "0.08em" }}>{"STARYIELD"}</h1>
          <p>{"Something went wrong. Please try again."}</p>
          <button type="button" onClick={() => retry()} style={{ marginTop: 16, padding: "12px 24px", background: "#c29a3d", border: 0, cursor: "pointer" }}>
            {"Try again"}
          </button>
        </main>
      </body>
    </html>
  );
}
