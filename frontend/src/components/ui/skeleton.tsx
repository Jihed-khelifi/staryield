import * as React from "react"

import { cn } from "@/lib/utils"

/**
 * Loading placeholder. Purely decorative (aria-hidden): the surrounding
 * region announces "Loading…" once through a status element. The shimmer
 * respects prefers-reduced-motion.
 */
function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      aria-hidden
      data-slot="skeleton"
      className={cn(
        "rounded-md bg-warm-gray/45 motion-safe:animate-pulse",
        className
      )}
      {...props}
    />
  )
}

/** A few stacked text lines; the last one is shorter, like a paragraph. */
function SkeletonText({
  lines = 3,
  className,
}: {
  lines?: number
  className?: string
}) {
  return (
    <div aria-hidden className={cn("grid gap-2", className)}>
      {Array.from({ length: lines }, (_, i) => (
        <Skeleton
          key={i}
          className={cn("h-3.5", i === lines - 1 && lines > 1 ? "w-2/3" : "w-full")}
        />
      ))}
    </div>
  )
}

/** Rows for list surfaces (devices, notes, transactions, reviews…). */
function SkeletonList({
  rows = 3,
  className,
  rowClassName,
}: {
  rows?: number
  className?: string
  rowClassName?: string
}) {
  return (
    <ul aria-hidden className={cn("grid gap-3", className)}>
      {Array.from({ length: rows }, (_, i) => (
        <li key={i}>
          <Skeleton className={cn("h-14 w-full rounded-lg", rowClassName)} />
        </li>
      ))}
    </ul>
  )
}

export { Skeleton, SkeletonList, SkeletonText }
