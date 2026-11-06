import * as React from "react"

import { cn } from "@/lib/utils"

/**
 * "Nothing here yet" block for empty lists, with an optional next step.
 * All copy is passed in already translated.
 */
export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
}: {
  icon?: React.ReactNode
  title: React.ReactNode
  description?: React.ReactNode
  action?: React.ReactNode
  className?: string
}) {
  return (
    <div
      data-slot="empty-state"
      className={cn(
        "flex flex-col items-center gap-2 rounded-xl border border-dashed border-gold/50 px-6 py-8 text-center",
        className
      )}
    >
      {icon ? (
        <span aria-hidden className="mb-1 text-gold [&_svg]:size-7">
          {icon}
        </span>
      ) : null}
      <p className="font-display text-lg">{title}</p>
      {description ? <p className="max-w-prose text-sm text-ink/75">{description}</p> : null}
      {action ? <div className="mt-2">{action}</div> : null}
    </div>
  )
}
