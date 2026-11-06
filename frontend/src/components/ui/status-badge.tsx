"use client"

import * as React from "react"
import { CircleIcon, ClockIcon, MinusCircleIcon, MoonIcon } from "lucide-react"

import { useI18n } from "@/i18n/i18n-provider"
import { cn } from "@/lib/utils"

export type PresenceStatus = "online" | "busy" | "away" | "offline"

const STATUS: Record<
  PresenceStatus,
  { label: string; dot: string; badge: string; Icon: React.ComponentType<{ className?: string; "aria-hidden"?: boolean }> }
> = {
  online: { label: "Online", dot: "bg-success", badge: "bg-success/10 text-success", Icon: CircleIcon },
  busy: { label: "Busy", dot: "bg-amber", badge: "bg-amber/20 text-ink", Icon: ClockIcon },
  away: { label: "Away", dot: "bg-sage", badge: "bg-sage/25 text-ink", Icon: MoonIcon },
  offline: { label: "Offline", dot: "bg-ink/40", badge: "bg-warm-gray/40 text-ink/75", Icon: MinusCircleIcon },
}

/**
 * Advisor/device presence. The state is always written out (and has its own
 * icon shape), so it never depends on colour alone.
 */
export function StatusBadge({
  status,
  label,
  className,
  variant = "badge",
}: {
  status: PresenceStatus
  /** Override the default label (already translated), e.g. "Active now". */
  label?: string
  className?: string
  /** "badge" = filled pill; "dot" = dot + text, for dense rows. */
  variant?: "badge" | "dot"
}) {
  const { text } = useI18n()
  const config = STATUS[status]
  const content = label ?? text(config.label)
  if (variant === "dot") {
    return (
      <span data-slot="status-badge" data-status={status} className={cn("inline-flex items-center gap-2 text-sm", className)}>
        <span aria-hidden className={cn("size-2.5 shrink-0 rounded-full", config.dot)} />
        <span>{content}</span>
      </span>
    )
  }
  return (
    <span
      data-slot="status-badge"
      data-status={status}
      className={cn(
        "inline-flex items-center gap-1.5 rounded px-2 py-0.5 text-xs font-medium",
        config.badge,
        className
      )}
    >
      <config.Icon aria-hidden className={cn("size-3", status === "online" && "fill-current")} />
      <span>{content}</span>
    </span>
  )
}
