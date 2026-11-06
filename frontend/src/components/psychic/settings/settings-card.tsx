"use client"

import * as React from "react"

import { cn } from "@/lib/utils"

/**
 * A settings card of the advisor Settings page (Figma 1219:1284): cream
 * card, gold hairline, Castellar title with a 24px line icon.
 */
export function SettingsCard({
  title,
  icon,
  actions,
  className,
  children,
  ...props
}: Omit<React.ComponentProps<"section">, "title"> & {
  title: React.ReactNode
  icon?: React.ReactNode
  /** Extra controls aligned with the title (e.g. a remove button). */
  actions?: React.ReactNode
}) {
  const headingId = React.useId()
  return (
    <section aria-labelledby={headingId} className={cn("advisor-settings-card", className)} {...props}>
      <div className="advisor-settings-card-head">
        <h2 id={headingId} className="advisor-settings-card-title">
          {icon ? (
            <span aria-hidden className="advisor-settings-card-icon">
              {icon}
            </span>
          ) : null}
          {title}
        </h2>
        {actions}
      </div>
      {children}
    </section>
  )
}
