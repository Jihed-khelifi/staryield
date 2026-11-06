"use client"

import * as React from "react"
import { Popover as PopoverPrimitive } from "radix-ui"
import { CheckIcon, Loader2, XIcon } from "lucide-react"

import { useI18n } from "@/i18n/i18n-provider"
import { errorMessage, toApiError } from "@/lib/api/errors"
import { cn } from "@/lib/utils"

type Status = "idle" | "loading" | "ready" | "error"

/**
 * A page of suggestions. `loadOptions` may resolve to a plain array (one
 * page) or to a page with a cursor; the combobox then offers "Show more
 * results" as the last option and passes the cursor back for the next page.
 */
export type ComboboxPage<T> = { items: T[]; nextCursor?: string | null }

function toPage<T>(result: T[] | ComboboxPage<T>): ComboboxPage<T> {
  return Array.isArray(result) ? { items: result, nextCursor: null } : { items: result.items ?? [], nextCursor: result.nextCursor ?? null }
}

export type ComboboxProps<T> = {
  id?: string
  name?: string
  /** The picked option, or null. Typed text alone never counts as a value. */
  value: T | null
  onChange: (value: T | null) => void
  onBlur?: () => void
  /**
   * Fetch suggestions; aborted when the query changes or the field unmounts.
   * Resolve to a page with `nextCursor` to paginate (`cursor` is then set
   * for the following pages).
   */
  loadOptions: (query: string, signal: AbortSignal, cursor?: string) => Promise<T[] | ComboboxPage<T>>
  getOptionLabel: (option: T) => string
  getOptionKey: (option: T) => string
  renderOption?: (option: T) => React.ReactNode
  placeholder?: string
  /** Characters needed before searching (default 2). */
  minChars?: number
  /** Delay after the last keystroke before searching (default 250 ms). */
  debounceMs?: number
  disabled?: boolean
  className?: string
  inputClassName?: string
  /** Shown when the search returns nothing (already translated). */
  emptyMessage?: string
  /** Small print under the suggestions, e.g. a data attribution (already translated). */
  footer?: React.ReactNode
  ref?: React.Ref<HTMLInputElement>
  "aria-invalid"?: boolean | "true" | "false"
  "aria-describedby"?: string
  "aria-labelledby"?: string
  "aria-label"?: string
  "aria-required"?: boolean
}

/**
 * Searchable single-select (ARIA 1.2 combobox + listbox), e.g. birthplace,
 * residence or time zone. Suggestions load asynchronously with debounce and
 * cancellation; loading, empty and error states are announced politely. The
 * value is only set by picking an option, so free text can never be saved
 * as a place.
 */
export function Combobox<T>({
  id,
  name,
  value,
  onChange,
  onBlur,
  loadOptions,
  getOptionLabel,
  getOptionKey,
  renderOption,
  placeholder,
  minChars = 2,
  debounceMs = 250,
  disabled,
  className,
  inputClassName,
  emptyMessage,
  footer,
  ref,
  ...aria
}: ComboboxProps<T>) {
  const { text } = useI18n()
  const generatedId = React.useId()
  const inputId = id ?? `${generatedId}-input`
  const listboxId = `${generatedId}-listbox`
  const anchorRef = React.useRef<HTMLDivElement>(null)
  const inputRef = React.useRef<HTMLInputElement | null>(null)
  const loader = React.useRef(loadOptions)
  React.useEffect(() => {
    loader.current = loadOptions
  })

  const selectedLabel = value === null ? "" : getOptionLabel(value)
  const [query, setQuery] = React.useState(selectedLabel)
  const [editing, setEditing] = React.useState(false)
  const [open, setOpen] = React.useState(false)
  const [options, setOptions] = React.useState<T[]>([])
  const [status, setStatus] = React.useState<Status>("idle")
  const [error, setError] = React.useState<string>("")
  const [active, setActive] = React.useState(-1)
  const [nonce, setNonce] = React.useState(0)
  const [nextCursor, setNextCursor] = React.useState<string | null>(null)
  const [loadingMore, setLoadingMore] = React.useState(false)
  const moreCtrl = React.useRef<AbortController | null>(null)

  // Follow external value changes (form reset, programmatic set) unless the
  // user is in the middle of typing (state adjusted during render).
  const [syncedLabel, setSyncedLabel] = React.useState(selectedLabel)
  if (!editing && syncedLabel !== selectedLabel) {
    setSyncedLabel(selectedLabel)
    setQuery(selectedLabel)
  }

  const term = query.trim()
  const searchable = editing && term.length >= minChars

  React.useEffect(() => {
    if (!searchable) return
    const ctrl = new AbortController()
    const timer = window.setTimeout(() => {
      moreCtrl.current?.abort()
      setLoadingMore(false)
      setStatus("loading")
      loader.current(term, ctrl.signal).then(
        (result) => {
          if (ctrl.signal.aborted) return
          const page = toPage(result)
          setOptions(page.items)
          setNextCursor(page.nextCursor || null)
          setActive(page.items.length > 0 ? 0 : -1)
          setStatus("ready")
        },
        (err) => {
          const e = toApiError(err)
          if (ctrl.signal.aborted || e.code === "aborted") return
          setOptions([])
          setNextCursor(null)
          setActive(-1)
          setError(errorMessage(e))
          setStatus("error")
        }
      )
    }, debounceMs)
    return () => {
      window.clearTimeout(timer)
      ctrl.abort()
      // A "Show more" page for the previous query must not land in the new list.
      moreCtrl.current?.abort()
    }
  }, [term, searchable, debounceMs, nonce])

  React.useEffect(() => () => moreCtrl.current?.abort(), [])

  const listOpen = open && searchable
  const hasMore = status === "ready" && nextCursor !== null
  // The "Show more results" row is the last option while another page exists.
  const itemCount = options.length + (hasMore ? 1 : 0)
  const moreIndex = hasMore ? options.length : -1

  function loadMore() {
    if (!nextCursor || loadingMore) return
    moreCtrl.current?.abort()
    const ctrl = new AbortController()
    moreCtrl.current = ctrl
    const firstNew = options.length
    setLoadingMore(true)
    loader.current(term, ctrl.signal, nextCursor).then(
      (result) => {
        if (ctrl.signal.aborted) return
        const page = toPage(result)
        const known = new Set(options.map(getOptionKey))
        const fresh = page.items.filter((option) => !known.has(getOptionKey(option)))
        setOptions((current) => [...current, ...fresh])
        setNextCursor(page.nextCursor || null)
        setActive(fresh.length > 0 ? firstNew : Math.max(0, firstNew - 1))
        setLoadingMore(false)
      },
      (err) => {
        const e = toApiError(err)
        if (ctrl.signal.aborted || e.code === "aborted") return
        setLoadingMore(false)
        setError(errorMessage(e))
        setStatus("error")
      }
    )
  }

  function pick(option: T) {
    onChange(option)
    setQuery(getOptionLabel(option))
    setEditing(false)
    setOpen(false)
    setActive(-1)
  }

  function clear() {
    onChange(null)
    setQuery("")
    setEditing(false)
    setOpen(false)
    setOptions([])
    setStatus("idle")
    inputRef.current?.focus()
  }

  function onKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (event.key === "ArrowDown") {
      event.preventDefault()
      if (!open) {
        setEditing(true)
        setOpen(true)
        return
      }
      setActive((i) => (itemCount === 0 ? -1 : (i + 1) % itemCount))
    } else if (event.key === "ArrowUp") {
      event.preventDefault()
      setActive((i) =>
        itemCount === 0 ? -1 : (i - 1 + itemCount) % itemCount
      )
    } else if (event.key === "Home" && listOpen && itemCount > 0) {
      event.preventDefault()
      setActive(0)
    } else if (event.key === "End" && listOpen && itemCount > 0) {
      event.preventDefault()
      setActive(itemCount - 1)
    } else if (event.key === "Enter") {
      if (listOpen && active >= 0 && active === moreIndex) {
        event.preventDefault()
        loadMore()
      } else if (listOpen && active >= 0 && options[active] !== undefined) {
        event.preventDefault()
        pick(options[active])
      }
    } else if (event.key === "Escape") {
      if (open) {
        event.preventDefault()
        setOpen(false)
      } else if (query && value === null) {
        event.preventDefault()
        setQuery("")
      }
    } else if (event.key === "Tab") {
      setOpen(false)
    }
  }

  const activeId = listOpen && active >= 0 ? `${listboxId}-${active}` : undefined
  const statusText =
    status === "loading"
      ? text("Searching…")
      : status === "error"
        ? text(error)
        : loadingMore
          ? text("Loading more results…")
          : status === "ready" && options.length === 0
            ? (emptyMessage ?? text("No matches found."))
            : ""

  return (
    <PopoverPrimitive.Root open={listOpen} onOpenChange={setOpen}>
      <PopoverPrimitive.Anchor asChild>
        <div
          ref={anchorRef}
          data-slot="combobox"
          className={cn("relative flex w-full items-center", className)}
        >
          <input
            ref={(node) => {
              inputRef.current = node
              if (typeof ref === "function") ref(node)
              else if (ref) (ref as React.RefObject<HTMLInputElement | null>).current = node
            }}
            id={inputId}
            name={name}
            type="text"
            role="combobox"
            autoComplete="off"
            spellCheck={false}
            aria-autocomplete="list"
            aria-expanded={listOpen}
            aria-controls={listboxId}
            aria-activedescendant={activeId}
            disabled={disabled}
            placeholder={placeholder}
            value={query}
            className={cn(
              "h-10 w-full min-w-0 rounded-lg border border-gold bg-cream-light px-3 pr-16 text-[15px] text-ink outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/40 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-danger",
              inputClassName
            )}
            onChange={(event) => {
              setQuery(event.target.value)
              setEditing(true)
              setOpen(true)
              if (value !== null) onChange(null)
            }}
            onFocus={() => {
              if (value === null && query.trim().length >= minChars) {
                setEditing(true)
                setOpen(true)
              }
            }}
            onBlur={() => {
              setEditing(value === null && query.length > 0)
              onBlur?.()
            }}
            onKeyDown={onKeyDown}
            {...aria}
          />
          <span className="pointer-events-none absolute right-10 flex items-center">
            {status === "loading" && listOpen ? (
              <Loader2 aria-hidden className="size-4 animate-spin motion-reduce:animate-none" />
            ) : null}
          </span>
          {query || value !== null ? (
            <button
              type="button"
              disabled={disabled}
              onClick={clear}
              className="absolute right-2 inline-flex size-7 items-center justify-center rounded-full text-ink/70 hover:bg-warm-gray/40 hover:text-ink"
            >
              <XIcon aria-hidden className="size-4" />
              <span className="sr-only">{text("Clear selection")}</span>
            </button>
          ) : null}
        </div>
      </PopoverPrimitive.Anchor>
      <PopoverPrimitive.Portal>
        <PopoverPrimitive.Content
          align="start"
          sideOffset={4}
          onOpenAutoFocus={(event) => event.preventDefault()}
          onCloseAutoFocus={(event) => event.preventDefault()}
          onInteractOutside={(event) => {
            if (anchorRef.current?.contains(event.target as Node)) event.preventDefault()
          }}
          style={{ width: "var(--radix-popper-anchor-width)" }}
          className="z-70 overflow-hidden rounded-lg border border-gold/60 bg-cream-light text-ink shadow-[0_12px_28px_rgb(21_16_11/0.16)] data-open:animate-in data-open:fade-in-0 motion-reduce:animate-none"
        >
          <div role="status" aria-live="polite" className={cn(statusText ? "px-3 py-2 text-sm" : "sr-only")}>
            {statusText}
            {status === "error" ? (
              <button
                type="button"
                className="ml-2 underline"
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => setNonce((n) => n + 1)}
              >
                {text("Try again")}
              </button>
            ) : null}
          </div>
          <ul
            id={listboxId}
            role="listbox"
            aria-label={aria["aria-label"]}
            aria-labelledby={aria["aria-labelledby"]}
            className={cn("max-h-64 overflow-y-auto p-1", itemCount === 0 && "hidden")}
          >
            {options.map((option, index) => {
              const selected = value !== null && getOptionKey(value) === getOptionKey(option)
              return (
                <li
                  key={getOptionKey(option)}
                  id={`${listboxId}-${index}`}
                  role="option"
                  aria-selected={index === active}
                  data-active={index === active || undefined}
                  className="flex cursor-pointer items-center justify-between gap-2 rounded-md px-2 py-2 text-[15px] data-active:bg-warm-gray/40"
                  onMouseDown={(event) => event.preventDefault()}
                  onMouseEnter={() => setActive(index)}
                  onClick={() => pick(option)}
                >
                  <span className="min-w-0 truncate">
                    {renderOption ? renderOption(option) : getOptionLabel(option)}
                  </span>
                  {selected ? <CheckIcon aria-hidden className="size-4 shrink-0" /> : null}
                </li>
              )
            })}
            {hasMore ? (
              <li
                id={`${listboxId}-${moreIndex}`}
                role="option"
                aria-selected={active === moreIndex}
                aria-disabled={loadingMore || undefined}
                data-active={active === moreIndex || undefined}
                data-slot="combobox-more"
                className="flex cursor-pointer items-center justify-center gap-2 rounded-md px-2 py-2 text-sm font-semibold underline-offset-4 hover:underline data-active:bg-warm-gray/40"
                onMouseDown={(event) => event.preventDefault()}
                onMouseEnter={() => setActive(moreIndex)}
                onClick={loadMore}
              >
                {loadingMore ? <Loader2 aria-hidden className="size-4 animate-spin motion-reduce:animate-none" /> : null}
                {text("Show more results")}
              </li>
            ) : null}
          </ul>
          {footer ? <div className="border-t border-gold/30 px-3 py-1.5 text-[11px] text-ink/75">{footer}</div> : null}
        </PopoverPrimitive.Content>
      </PopoverPrimitive.Portal>
    </PopoverPrimitive.Root>
  )
}
