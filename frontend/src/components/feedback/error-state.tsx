"use client"

import * as React from "react"
import Link from "next/link"
import { AlertCircle } from "lucide-react"

import { useI18n } from "@/i18n/i18n-provider"
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors"
import { loginUrl } from "@/lib/navigation"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"

/** What the user can do about an error. */
export type ErrorAction =
  | { kind: "retry" }
  | { kind: "retry-after"; seconds: number }
  | { kind: "sign-in" }
  | { kind: "top-up" }
  | { kind: "home" }
  | { kind: "support" }
  | { kind: "none" }

/**
 * Recovery action for an API error. Pure, so the mapping is unit-tested:
 * transient failures can be retried (after `Retry-After` when rate limited),
 * auth failures lead to sign-in, missing credits to top-up, server faults to
 * support with the request reference, and switched-off features to nothing.
 */
export function errorAction(error: ApiError, canRetry: boolean): ErrorAction {
  switch (error.code) {
    case "unauthenticated":
    case "session_expired":
      return { kind: "sign-in" }
    case "insufficient_credits":
      return { kind: "top-up" }
    case "feature_unavailable":
      return { kind: "none" }
    case "not_found":
    case "forbidden":
      return { kind: "home" }
    case "rate_limited":
      if (!canRetry) return { kind: "none" }
      return error.retryAfter && error.retryAfter > 0
        ? { kind: "retry-after", seconds: Math.ceil(error.retryAfter) }
        : { kind: "retry" }
  }
  if (error.retryable && canRetry) return { kind: "retry" }
  if (error.status >= 500 || error.code === "internal_error") return { kind: "support" }
  return canRetry ? { kind: "retry" } : { kind: "none" }
}

/** Seconds left of a `Retry-After` wait; restarts when `seconds` changes. */
function useCountdown(seconds: number | null): number {
  const [clock, setClock] = React.useState({ seconds, elapsed: 0 })
  if (clock.seconds !== seconds) {
    // A new error arrived: restart the countdown (state adjusted during render).
    setClock({ seconds, elapsed: 0 })
  }
  const left = seconds ? Math.max(0, seconds - clock.elapsed) : 0
  React.useEffect(() => {
    if (!seconds || left === 0) return
    const timer = window.setInterval(() => setClock((c) => ({ ...c, elapsed: c.elapsed + 1 })), 1000)
    return () => window.clearInterval(timer)
  }, [seconds, left])
  return left
}

const noSubscription = () => () => {}

/** Current path + query (client) for "come back here afterwards" links. */
function useHere(fallback: string): string {
  return React.useSyncExternalStore(
    noSubscription,
    () => window.location.pathname + window.location.search,
    () => fallback
  )
}

/**
 * Inline error block with a clear message and the matching call to action.
 * Use for failed loads and blocking errors; transient feedback belongs in a
 * toast. Server faults show the request reference so support can trace them.
 */
export function ErrorState({
  error,
  onRetry,
  title,
  className,
  compact = false,
}: {
  error: unknown
  onRetry?: () => void
  /** Optional heading above the message (already translated). */
  title?: string
  className?: string
  compact?: boolean
}) {
  const { text, locale } = useI18n()
  const e = toApiError(error)
  const action = errorAction(e, Boolean(onRetry))
  const left = useCountdown(action.kind === "retry-after" ? action.seconds : null)
  const next = useHere(`/${locale}`)

  let cta: React.ReactNode = null
  if (action.kind === "retry" || action.kind === "retry-after") {
    const waiting = action.kind === "retry-after" && left > 0
    cta = (
      <Button type="button" variant="cta" size="lg" className="px-4" onClick={onRetry} disabled={waiting}>
        {waiting ? text("Try again in {value0}s", { value0: left }) : text("Try again")}
      </Button>
    )
  } else if (action.kind === "sign-in") {
    cta = (
      <Button asChild variant="cta" size="lg" className="px-4">
        <Link href={loginUrl(locale)}>{text("Sign In")}</Link>
      </Button>
    )
  } else if (action.kind === "top-up") {
    cta = (
      <Button asChild variant="cta" size="lg" className="px-4">
        <Link href={`/${locale}/credits?next=${encodeURIComponent(next)}`}>{text("Top up credits")}</Link>
      </Button>
    )
  } else if (action.kind === "home") {
    cta = (
      <Button asChild variant="outline" size="lg" className="border-ink/40 px-4">
        <Link href={`/${locale}`}>{text("Back to home")}</Link>
      </Button>
    )
  } else if (action.kind === "support") {
    cta = (
      <Button asChild variant="outline" size="lg" className="border-ink/40 px-4">
        <Link href={`/${locale}/contact`}>{text("Contact support")}</Link>
      </Button>
    )
  }

  return (
    <div
      role="alert"
      data-slot="error-state"
      data-code={e.code}
      className={cn(
        "flex flex-col items-center gap-3 text-center",
        compact ? "py-4" : "rounded-xl border border-gold/40 bg-cream-light/60 px-6 py-8",
        className
      )}
    >
      <AlertCircle aria-hidden className="size-6 text-danger" />
      {title ? <p className="font-display text-lg">{title}</p> : null}
      <p className="max-w-prose text-sm">{text(errorMessage(e))}</p>
      {cta}
      {action.kind === "support" && e.requestId ? (
        <p className="text-xs text-ink/70">{text("Reference: {value0}", { value0: e.requestId })}</p>
      ) : null}
    </div>
  )
}
