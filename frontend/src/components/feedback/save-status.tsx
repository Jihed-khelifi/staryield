"use client"

import * as React from "react"
import { AlertCircle, CheckCircle2, Loader2 } from "lucide-react"

import { useI18n } from "@/i18n/i18n-provider"
import { cn } from "@/lib/utils"

export type SaveState = "idle" | "dirty" | "saving" | "saved" | "error"

/**
 * Inline save indicator next to "Save changes": announces progress and the
 * outcome politely, with an icon and text (never colour alone).
 */
export function SaveStatus({ state, className }: { state: SaveState; className?: string }) {
  const { text } = useI18n()
  const content =
    state === "saving" ? (
      <>
        <Loader2 aria-hidden className="size-4 animate-spin motion-reduce:animate-none" />
        {text("Saving…")}
      </>
    ) : state === "saved" ? (
      <>
        <CheckCircle2 aria-hidden className="size-4 text-success" />
        {text("All changes saved")}
      </>
    ) : state === "error" ? (
      <>
        <AlertCircle aria-hidden className="size-4 text-danger" />
        {text("Not saved")}
      </>
    ) : state === "dirty" ? (
      text("Unsaved changes")
    ) : null
  return (
    <p
      role="status"
      aria-live="polite"
      data-slot="save-status"
      data-state={state}
      className={cn("inline-flex min-h-5 items-center gap-1.5 text-sm", className)}
    >
      {content}
    </p>
  )
}
