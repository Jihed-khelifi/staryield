"use client"

import * as React from "react"

import { useI18n } from "@/i18n/i18n-provider"
import { cn } from "@/lib/utils"

export type DateParts = {
  day: string
  month: string
  year: string
  /** "HH:MM" (24 h) when `withTime` is set; "" when unknown. */
  time?: string
}

type Part = "day" | "month" | "year" | "time"

const LENGTH: Record<Part, number> = { day: 2, month: 2, year: 4, time: 5 }

export const emptyDateParts: DateParts = { day: "", month: "", year: "", time: "" }

/**
 * Parse a pasted date: "29/11/1990", "29-11-1990", "29.11.1990" (day first,
 * or month first when `order` is "mdy") or ISO "1990-11-29". Time is kept
 * when present ("1990-11-29 14:30", "29/11/1990 14:30").
 */
export function parsePastedDate(raw: string, order: "dmy" | "mdy" = "dmy"): DateParts | null {
  const value = raw.trim()
  const iso = value.match(/^(\d{4})-(\d{1,2})-(\d{1,2})(?:[ T](\d{1,2}):(\d{2}))?/)
  if (iso) {
    const [, year, month, day, hh, mm] = iso
    return {
      day: day.padStart(2, "0"),
      month: month.padStart(2, "0"),
      year,
      time: hh ? `${hh.padStart(2, "0")}:${mm}` : "",
    }
  }
  const local = value.match(/^(\d{1,2})[/.\-\s](\d{1,2})[/.\-\s](\d{4})(?:[ T,]+(\d{1,2})[:h.](\d{2}))?$/)
  if (local) {
    const [, a, b, year, hh, mm] = local
    const [day, month] = order === "dmy" ? [a, b] : [b, a]
    return {
      day: day.padStart(2, "0"),
      month: month.padStart(2, "0"),
      year,
      time: hh ? `${hh.padStart(2, "0")}:${mm}` : "",
    }
  }
  return null
}

function clean(part: Part, raw: string): string {
  if (part === "time") {
    const digits = raw.replace(/[^\d]/g, "").slice(0, 4)
    return digits.length > 2 ? `${digits.slice(0, 2)}:${digits.slice(2)}` : digits
  }
  return raw.replace(/\D/g, "").slice(0, LENGTH[part])
}

type DatePartsInputProps = {
  value: DateParts
  onChange: (value: DateParts) => void
  onBlur?: () => void
  /** Field order: day/month/year (default) or month/day/year. */
  order?: "dmy" | "mdy"
  withTime?: boolean
  disabled?: boolean
  id?: string
  name?: string
  className?: string
  /** Accessible name of the whole group (the visible field label). */
  "aria-label"?: string
  "aria-labelledby"?: string
  "aria-describedby"?: string
  "aria-invalid"?: boolean | "true" | "false"
  ref?: React.Ref<HTMLInputElement>
}

/**
 * Birth date as separate DD / MM / YYYY (and optional HH:MM) boxes, like
 * the Figma calculators and onboarding. Moves to the next box when one is
 * full, accepts a pasted full date, and opens the numeric keypad on phones.
 * The group is one form control: one label, one error message.
 */
export function DatePartsInput({
  value,
  onChange,
  onBlur,
  order = "dmy",
  withTime = false,
  disabled,
  id,
  name,
  className,
  ref,
  ...aria
}: DatePartsInputProps) {
  const { text } = useI18n()
  const generatedId = React.useId()
  const baseId = id ?? generatedId
  const refs = React.useRef<Partial<Record<Part, HTMLInputElement | null>>>({})
  const parts: Part[] = [...(order === "dmy" ? (["day", "month"] as const) : (["month", "day"] as const)), "year"]
  if (withTime) parts.push("time")

  const labels: Record<Part, string> = {
    day: text("Day"),
    month: text("Month"),
    year: text("Year"),
    time: text("Time"),
  }
  const placeholders: Record<Part, string> = {
    day: text("DD"),
    month: text("MM"),
    year: text("YYYY"),
    time: text("HH:MM"),
  }

  function update(part: Part, raw: string) {
    const next = { ...value, [part]: clean(part, raw) }
    onChange(next)
    const full = next[part]?.length === LENGTH[part]
    const index = parts.indexOf(part)
    if (full && index < parts.length - 1) refs.current[parts[index + 1]]?.focus()
  }

  function onPaste(event: React.ClipboardEvent<HTMLInputElement>) {
    const parsed = parsePastedDate(event.clipboardData.getData("text"), order)
    if (!parsed) return
    event.preventDefault()
    onChange({ ...value, ...parsed, time: withTime ? parsed.time || value.time || "" : value.time })
    refs.current[parts[parts.length - 1]]?.focus()
  }

  function onKeyDown(part: Part, event: React.KeyboardEvent<HTMLInputElement>) {
    const index = parts.indexOf(part)
    if (event.key === "Backspace" && !value[part] && index > 0) {
      refs.current[parts[index - 1]]?.focus()
    }
  }

  return (
    <div
      role="group"
      data-slot="date-parts-input"
      aria-label={aria["aria-label"]}
      aria-labelledby={aria["aria-labelledby"]}
      aria-describedby={aria["aria-describedby"]}
      className={cn("flex w-full gap-2", className)}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) onBlur?.()
      }}
    >
      {parts.map((part, index) => (
        <label
          key={part}
          className={cn(
            "flex min-w-0 flex-col gap-1 text-[11px] font-semibold tracking-wide uppercase",
            part === "year" ? "flex-[1.4]" : "flex-1"
          )}
        >
          <span>{labels[part]}</span>
          <input
            ref={(node) => {
              refs.current[part] = node
              if (index === 0) {
                if (typeof ref === "function") ref(node)
                else if (ref) (ref as React.RefObject<HTMLInputElement | null>).current = node
              }
            }}
            id={index === 0 ? baseId : `${baseId}-${part}`}
            name={name ? `${name}.${part}` : undefined}
            inputMode="numeric"
            autoComplete={
              part === "day" ? "bday-day" : part === "month" ? "bday-month" : part === "year" ? "bday-year" : "off"
            }
            maxLength={LENGTH[part]}
            disabled={disabled}
            placeholder={placeholders[part]}
            aria-invalid={aria["aria-invalid"]}
            value={value[part] ?? ""}
            onChange={(event) => update(part, event.target.value)}
            onPaste={onPaste}
            onKeyDown={(event) => onKeyDown(part, event)}
            className="h-10 w-full min-w-0 rounded-lg border border-gold bg-cream-light px-3 text-[15px] font-normal tracking-normal text-ink tabular-nums normal-case outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/40 disabled:opacity-50 aria-invalid:border-danger"
          />
        </label>
      ))}
    </div>
  )
}

/** "YYYY-MM-DD" for complete parts, otherwise null. */
export function datePartsToIso(value: DateParts): string | null {
  if (value.year.length !== 4 || !value.month || !value.day) return null
  return `${value.year}-${value.month.padStart(2, "0")}-${value.day.padStart(2, "0")}`
}

/** Split "YYYY-MM-DD" (and an optional "HH:MM") into parts. */
export function isoToDateParts(date?: string | null, time?: string | null): DateParts {
  const match = date?.match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (!match) return { ...emptyDateParts, time: time ?? "" }
  return { year: match[1], month: match[2], day: match[3], time: time ?? "" }
}
