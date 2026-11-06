"use client"

import { useCallback, useEffect, useReducer, useRef } from "react"

import { toApiError, type ApiError } from "@/lib/api/errors"
import type { Page } from "@/lib/api/types"

type Cursor = string | undefined

type State<T> = {
  /** Identity of the list (serialized deps); a change resets everything. */
  key: string
  /** 0-based index of the page on screen. */
  index: number
  /** Cursor that loads each page index (`cursors[0]` is undefined). */
  cursors: Cursor[]
  /** Pages fetched so far, by index — makes "Previous" instant. */
  pages: Record<number, Page<T>>
  error: ApiError | null
  /** Bumped to force a refetch of the current page. */
  nonce: number
}

type Action<T> =
  | { type: "reset"; key: string }
  | { type: "loaded"; key: string; index: number; page: Page<T> }
  | { type: "failed"; key: string; error: ApiError }
  | { type: "go"; index: number }
  | { type: "reload" }
  | { type: "patch"; update: (items: T[], index: number) => T[] }

function initial<T>(key: string): State<T> {
  return { key, index: 0, cursors: [undefined], pages: {}, error: null, nonce: 0 }
}

function reducer<T>(state: State<T>, action: Action<T>): State<T> {
  switch (action.type) {
    case "reset":
      return initial<T>(action.key)
    case "loaded": {
      if (action.key !== state.key) return state
      // The API sends a next_cursor whenever a page is full, so the page
      // after the last one can be empty: step back and stop offering "Next".
      const before = state.pages[action.index - 1]
      if (action.page.items.length === 0 && action.index > 0 && before) {
        return {
          ...state,
          index: action.index - 1,
          cursors: state.cursors.slice(0, action.index),
          pages: { ...state.pages, [action.index - 1]: { ...before, next_cursor: undefined } },
          error: null,
        }
      }
      const cursors = state.cursors.slice(0, action.index + 1)
      if (action.page.next_cursor) cursors[action.index + 1] = action.page.next_cursor
      return { ...state, cursors, pages: { ...state.pages, [action.index]: action.page }, error: null }
    }
    case "failed":
      return action.key === state.key ? { ...state, error: action.error } : state
    case "go":
      return { ...state, index: action.index, error: null }
    case "reload": {
      const pages = { ...state.pages }
      delete pages[state.index]
      return { ...state, pages, error: null, nonce: state.nonce + 1 }
    }
    case "patch": {
      const pages: Record<number, Page<T>> = {}
      for (const [index, page] of Object.entries(state.pages))
        pages[Number(index)] = { ...page, items: action.update(page.items, Number(index)) }
      return { ...state, pages }
    }
  }
}

export type PagedList<T> = {
  items: T[]
  /** 1-based page number, for display. */
  page: number
  hasPrev: boolean
  hasNext: boolean
  loading: boolean
  error: ApiError | null
  next: () => void
  prev: () => void
  /** Back to the first page and refetch everything. */
  reset: () => void
  /** Refetch the page on screen (e.g. after an error). */
  reload: () => void
  replaceItem: (match: (item: T) => boolean, next: T) => void
  removeItem: (match: (item: T) => boolean) => void
  /** Insert at the top of the first page (realtime/optimistic inserts). */
  prependItem: (item: T) => void
}

/**
 * Cursor-paginated list with Previous/Next: `fetchPage(cursor, signal)`
 * returns the API's `{items, next_cursor}`. Visited pages are cached, so
 * going back never refetches; changing `deps` starts over; in-flight
 * requests are aborted on page change and unmount.
 */
export function usePagedList<T>(
  fetchPage: (cursor: Cursor, signal: AbortSignal) => Promise<Page<T>>,
  deps: unknown[],
  { enabled = true }: { enabled?: boolean } = {}
): PagedList<T> {
  const key = deps.map((dep) => String(dep)).join("\u0001")
  const [state, dispatch] = useReducer(reducer<T>, key, initial<T>)
  const fetcher = useRef(fetchPage)
  useEffect(() => {
    fetcher.current = fetchPage
  })

  // A new list identity starts from the first page.
  useEffect(() => {
    if (state.key !== key) dispatch({ type: "reset", key })
  }, [key, state.key])

  const current = state.key === key ? state : initial<T>(key)
  const cached = current.pages[current.index]
  const cursor = current.cursors[current.index]

  useEffect(() => {
    if (!enabled || cached || state.key !== key) return
    const ctrl = new AbortController()
    const index = state.index
    fetcher.current(cursor, ctrl.signal).then(
      (page) => {
        if (!ctrl.signal.aborted) dispatch({ type: "loaded", key, index, page })
      },
      (err) => {
        const error = toApiError(err)
        if (!ctrl.signal.aborted && error.code !== "aborted") dispatch({ type: "failed", key, error })
      }
    )
    return () => ctrl.abort()
    // `cursor` and `index` change together with the page; nonce forces reloads.
  }, [enabled, cached, key, state.key, state.index, cursor, state.nonce])

  const hasNext = Boolean(cached?.next_cursor) && (cached?.items.length ?? 0) > 0
  const next = useCallback(() => {
    if (hasNext) dispatch({ type: "go", index: current.index + 1 })
  }, [hasNext, current.index])
  const prev = useCallback(() => {
    if (current.index > 0) dispatch({ type: "go", index: current.index - 1 })
  }, [current.index])
  const reset = useCallback(() => dispatch({ type: "reset", key }), [key])
  const reload = useCallback(() => dispatch({ type: "reload" }), [])
  const replaceItem = useCallback(
    (match: (item: T) => boolean, nextItem: T) =>
      dispatch({ type: "patch", update: (items) => items.map((item) => (match(item) ? nextItem : item)) }),
    []
  )
  const removeItem = useCallback(
    (match: (item: T) => boolean) => dispatch({ type: "patch", update: (items) => items.filter((item) => !match(item)) }),
    []
  )
  const prependItem = useCallback(
    (item: T) => dispatch({ type: "patch", update: (items, index) => (index === 0 ? [item, ...items] : items) }),
    []
  )

  return {
    items: cached?.items ?? [],
    page: current.index + 1,
    hasPrev: current.index > 0,
    hasNext,
    loading: enabled && !cached && !current.error,
    error: current.error,
    next,
    prev,
    reset,
    reload,
    replaceItem,
    removeItem,
    prependItem,
  }
}
