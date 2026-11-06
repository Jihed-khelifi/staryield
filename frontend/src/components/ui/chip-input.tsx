"use client"

import * as React from "react"
import { MinusIcon, PlusIcon } from "lucide-react"

import { useI18n } from "@/i18n/i18n-provider"
import { announce } from "@/components/feedback/live-region"
import { cn } from "@/lib/utils"

type ChipInputProps = {
  value: string[]
  onChange: (value: string[]) => void
  onBlur?: () => void
  /** Maximum number of chips (e.g. 8 modalities). */
  max?: number
  /** Maximum length of one chip. */
  maxLength?: number
  placeholder?: string
  disabled?: boolean
  id?: string
  name?: string
  className?: string
  "aria-label"?: string
  "aria-labelledby"?: string
  "aria-describedby"?: string
  "aria-invalid"?: boolean | "true" | "false"
  ref?: React.Ref<HTMLInputElement>
}

/** Normalize a typed chip (trim, collapse spaces). */
export function normalizeChip(raw: string): string {
  return raw.trim().replace(/\s+/g, " ")
}

/** Add a chip unless it is empty, a case-insensitive duplicate or over the limit. */
export function addChip(list: string[], raw: string, max = Infinity): string[] {
  const chip = normalizeChip(raw)
  if (!chip || list.length >= max) return list
  if (list.some((item) => item.toLowerCase() === chip.toLowerCase())) return list
  return [...list, chip]
}

/**
 * Editable list of short labels (advisor "Modalities": Tarot − Astrology −
 * … +). Enter or "+" adds the typed label, "−" removes one; changes are
 * announced to screen readers.
 */
export function ChipInput({
  value,
  onChange,
  onBlur,
  max = Infinity,
  maxLength = 40,
  placeholder,
  disabled,
  id,
  name,
  className,
  ref,
  ...aria
}: ChipInputProps) {
  const { text } = useI18n()
  const [draft, setDraft] = React.useState("")
  const full = value.length >= max

  function commit() {
    const next = addChip(value, draft, max)
    if (next !== value) {
      onChange(next)
      announce(text("{value0} added.", { value0: normalizeChip(draft) }))
    }
    setDraft("")
  }

  function remove(chip: string) {
    onChange(value.filter((item) => item !== chip))
    announce(text("{value0} removed.", { value0: chip }))
  }

  return (
    <div
      data-slot="chip-input"
      className={cn("flex flex-wrap items-center gap-2", className)}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) onBlur?.()
      }}
    >
      <ul className="contents" aria-label={aria["aria-label"]}>
        {value.map((chip) => (
          <li
            key={chip}
            className="inline-flex items-center gap-1.5 rounded-md border border-gold/50 bg-cream-light py-1 pr-1 pl-3 text-[15px]"
          >
            <span>{chip}</span>
            <button
              type="button"
              disabled={disabled}
              onClick={() => remove(chip)}
              aria-label={text("Remove {value0}", { value0: chip })}
              className="inline-flex size-6 items-center justify-center rounded text-ink/70 hover:bg-warm-gray/40 hover:text-ink"
            >
              <MinusIcon aria-hidden className="size-3.5" />
            </button>
          </li>
        ))}
      </ul>
      {!full ? (
        <span className="inline-flex items-center gap-1">
          <input
            ref={ref}
            id={id}
            name={name}
            value={draft}
            disabled={disabled}
            maxLength={maxLength}
            placeholder={placeholder}
            aria-labelledby={aria["aria-labelledby"]}
            aria-describedby={aria["aria-describedby"]}
            aria-invalid={aria["aria-invalid"]}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault()
                commit()
              }
            }}
            className="h-9 w-36 min-w-0 rounded-md border border-gold/60 bg-cream-light px-2 text-[15px] outline-none focus-visible:ring-3 focus-visible:ring-ring/40 aria-invalid:border-danger"
          />
          <button
            type="button"
            disabled={disabled || !normalizeChip(draft)}
            onClick={commit}
            aria-label={text("Add {value0}", { value0: normalizeChip(draft) || placeholder || "" }).trim()}
            className="inline-flex size-9 items-center justify-center rounded-md border border-ink/60 bg-cream-light hover:bg-warm-gray/30 disabled:opacity-50"
          >
            <PlusIcon aria-hidden className="size-4" />
          </button>
        </span>
      ) : (
        <p className="text-xs text-ink/70">{text("You can add up to {value0}.", { value0: max })}</p>
      )}
    </div>
  )
}
