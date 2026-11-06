"use client"

import * as React from "react"
import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react"

import { useI18n } from "@/i18n/i18n-provider"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"

type CursorPagerProps = {
  /** 1-based index of the page on screen. */
  page: number
  hasPrev: boolean
  hasNext: boolean
  onPrev: () => void
  onNext: () => void
  loading?: boolean
  /** Total number of pages, when the API returns a total. */
  pageCount?: number
  /** Accessible name of the navigation landmark (translated). */
  label?: string
  className?: string
}

/**
 * Previous / Next pager for cursor (keyset) pages — the API's
 * `{items, next_cursor}` lists. Pair it with `usePagedList`, which keeps
 * the cursor stack so "Previous" is instant. Renders nothing for a single
 * page.
 */
export function CursorPager({
  page,
  hasPrev,
  hasNext,
  onPrev,
  onNext,
  loading = false,
  pageCount,
  label,
  className,
}: CursorPagerProps) {
  const { text } = useI18n()
  if (!hasPrev && !hasNext) return null
  const status = pageCount
    ? text("Page {value0} of {value1}", { value0: page, value1: pageCount })
    : text("Page {value0}", { value0: page })
  return (
    <nav
      aria-label={label ?? text("Pagination")}
      data-slot="pagination"
      className={cn("flex items-center justify-between gap-3 pt-3", className)}
    >
      <Button
        type="button"
        variant="outline"
        size="lg"
        className="border-gold/60 px-3"
        onClick={onPrev}
        disabled={!hasPrev || loading}
      >
        <ChevronLeftIcon aria-hidden />
        {text("Previous")}
      </Button>
      <p className="text-sm tabular-nums" aria-live="polite" aria-atomic="true">
        {status}
      </p>
      <Button
        type="button"
        variant="outline"
        size="lg"
        className="border-gold/60 px-3"
        onClick={onNext}
        disabled={!hasNext || loading}
        aria-busy={loading || undefined}
      >
        {text("Next")}
        <ChevronRightIcon aria-hidden />
      </Button>
    </nav>
  )
}

export type PageItem = number | "ellipsis-start" | "ellipsis-end"

/**
 * Page numbers to show: first, last, the current page and `siblings` pages
 * on each side, with ellipses for the gaps (1 … 4 5 6 … 20).
 */
export function pageItems(page: number, pageCount: number, siblings = 1): PageItem[] {
  if (pageCount <= 0) return []
  const current = Math.min(Math.max(1, page), pageCount)
  // first + last + current + 2 siblings + 2 ellipses
  const slots = siblings * 2 + 5
  if (pageCount <= slots) return Array.from({ length: pageCount }, (_, i) => i + 1)
  const start = Math.max(2, current - siblings)
  const end = Math.min(pageCount - 1, current + siblings)
  const items: PageItem[] = [1]
  if (start > 2) items.push("ellipsis-start")
  for (let p = start; p <= end; p++) items.push(p)
  if (end < pageCount - 1) items.push("ellipsis-end")
  items.push(pageCount)
  return items
}

type NumberedPagerProps = {
  page: number
  pageCount: number
  onPageChange: (page: number) => void
  siblings?: number
  loading?: boolean
  label?: string
  className?: string
}

/** Numbered pager, only for lists whose API returns a total. */
export function NumberedPager({
  page,
  pageCount,
  onPageChange,
  siblings = 1,
  loading = false,
  label,
  className,
}: NumberedPagerProps) {
  const { text } = useI18n()
  if (pageCount <= 1) return null
  const items = pageItems(page, pageCount, siblings)
  return (
    <nav
      aria-label={label ?? text("Pagination")}
      data-slot="pagination"
      className={cn("flex flex-wrap items-center justify-center gap-1 pt-3", className)}
    >
      <Button
        type="button"
        variant="ghost"
        size="icon-lg"
        onClick={() => onPageChange(page - 1)}
        disabled={page <= 1 || loading}
      >
        <ChevronLeftIcon aria-hidden />
        <span className="sr-only">{text("Previous")}</span>
      </Button>
      <ul className="flex items-center gap-1">
        {items.map((item) =>
          typeof item === "number" ? (
            <li key={item}>
              <Button
                type="button"
                variant={item === page ? "outline" : "ghost"}
                size="icon-lg"
                className={cn(
                  "tabular-nums",
                  item === page && "border-amber bg-amber/20 font-semibold"
                )}
                aria-current={item === page ? "page" : undefined}
                aria-label={text("Go to page {value0}", { value0: item })}
                disabled={loading}
                onClick={() => item !== page && onPageChange(item)}
              >
                {item}
              </Button>
            </li>
          ) : (
            <li key={item} aria-hidden className="px-1 text-ink/60">
              …
            </li>
          )
        )}
      </ul>
      <Button
        type="button"
        variant="ghost"
        size="icon-lg"
        onClick={() => onPageChange(page + 1)}
        disabled={page >= pageCount || loading}
      >
        <ChevronRightIcon aria-hidden />
        <span className="sr-only">{text("Next")}</span>
      </Button>
    </nav>
  )
}
