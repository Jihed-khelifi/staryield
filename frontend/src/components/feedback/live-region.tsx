"use client"

import { useCallback } from "react"

type Politeness = "polite" | "assertive"

const regions: Partial<Record<Politeness, HTMLElement>> = {}
const timers: Partial<Record<Politeness, number>> = {}

function region(politeness: Politeness): HTMLElement | null {
  if (typeof document === "undefined") return null
  const existing = regions[politeness]
  if (existing && document.body.contains(existing)) return existing
  const el = document.createElement("div")
  el.setAttribute("aria-live", politeness)
  el.setAttribute("aria-atomic", "true")
  el.setAttribute("role", politeness === "assertive" ? "alert" : "status")
  el.dataset.slot = "live-region"
  el.dataset.politeness = politeness
  Object.assign(el.style, {
    position: "absolute",
    width: "1px",
    height: "1px",
    margin: "-1px",
    padding: "0",
    overflow: "hidden",
    clip: "rect(0 0 0 0)",
    whiteSpace: "nowrap",
    border: "0",
  })
  document.body.appendChild(el)
  regions[politeness] = el
  return el
}

/**
 * Announce a short, already-translated message to screen readers (e.g.
 * "Note saved.", "Page 2 of 5"). One shared off-screen live region per
 * politeness level is created lazily, so no provider is needed. Repeating
 * the same message re-announces it.
 */
export function announce(message: string, politeness: Politeness = "polite") {
  const el = region(politeness)
  if (!el || !message) return
  el.textContent = ""
  window.clearTimeout(timers[politeness])
  timers[politeness] = window.setTimeout(() => {
    el.textContent = message
  }, 50)
}

/** Hook form of `announce`, for components that prefer injection. */
export function useAnnounce() {
  return useCallback((message: string, politeness: Politeness = "polite") => announce(message, politeness), [])
}
