"use client"

import * as React from "react"

import { useI18n } from "@/i18n/i18n-provider"
import type { ApiError } from "@/lib/api/errors"
import { ErrorState } from "@/components/feedback/error-state"
import { SkeletonList } from "@/components/ui/skeleton"

type AsyncLike<T> = {
  data: T | null
  error: ApiError | null
  loading: boolean
  reload: () => void
}

/**
 * Renders the four states of an async surface — loading (skeleton + one
 * "Loading…" announcement), error (message + call to action), empty, data —
 * for `useAsync` results. Paged lists use `usePagedList` the same way.
 */
export function AsyncContent<T>({
  state,
  skeleton,
  empty,
  isEmpty,
  errorTitle,
  children,
}: {
  state: AsyncLike<T>
  skeleton?: React.ReactNode
  /** Rendered instead of children when `isEmpty(data)` is true. */
  empty?: React.ReactNode
  isEmpty?: (data: T) => boolean
  errorTitle?: string
  children: (data: T) => React.ReactNode
}) {
  const { text } = useI18n()
  const { data, error, loading, reload } = state
  if (error && !loading) return <ErrorState error={error} onRetry={reload} title={errorTitle} />
  if (data === null || (loading && data === null)) {
    return (
      <div data-slot="async-loading" aria-busy="true">
        <p role="status" className="sr-only">
          {text("Loading…")}
        </p>
        {skeleton ?? <SkeletonList />}
      </div>
    )
  }
  if (empty && isEmpty?.(data)) return <>{empty}</>
  return <>{children(data)}</>
}
