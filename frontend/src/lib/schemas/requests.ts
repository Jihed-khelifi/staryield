/**
 * Custom request order sheet on an advisor profile (FR-07): an optional note
 * for the advisor and an optional preferred date/time. The API stores the
 * preference as an instant (internal/service/requests.go accepts up to one
 * year ahead); we build it from the visitor's local time.
 */
import * as yup from "yup"

import type { TranslateCopy } from "@/i18n/translate-copy"
import { TIME_RULE, trimmedText } from "@/lib/schemas/common"

export const ORDER_NOTE_MAX = 2000

const pad = (n: number) => String(n).padStart(2, "0")

/** Today's date (YYYY-MM-DD) in the visitor's time zone. */
export function todayLocalIso(now: Date = new Date()): string {
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
}

/** The last preferred date the API accepts (the day before one year from today). */
export function lastOrderDateIso(now: Date = new Date()): string {
  return todayLocalIso(new Date(now.getFullYear() + 1, now.getMonth(), now.getDate() - 1))
}

/** A real YYYY-MM-DD calendar day (rejects 2026-02-30). */
function isCalendarDate(value: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const [y, m, d] = value.split("-").map(Number)
  return todayLocalIso(new Date(y, m - 1, d)) === value
}

export const orderSchema = (text: TranslateCopy) =>
  yup.object({
    note: trimmedText(text, { max: ORDER_NOTE_MAX, required: false }),
    date: yup
      .string()
      .default("")
      .test("date", text("Enter a valid date."), (value) => !value || isCalendarDate(value))
      .test("future", text("Choose today or a later date."), (value) => !value || value >= todayLocalIso())
      .test("within-year", text("Choose a date within the next year."), (value) => !value || value <= lastOrderDateIso()),
    time: yup
      .string()
      .default("")
      .test("time24", text("Use the 24-hour format HH:MM."), (value) => !value || TIME_RULE.test(value))
      .test("needs-date", text("Choose a date for this time."), function (value) {
        return !value || Boolean(this.parent?.date)
      })
      .test("later-today", text("Choose a time later than now."), function (value) {
        if (!value || !TIME_RULE.test(value) || this.parent?.date !== todayLocalIso()) return true
        const now = new Date()
        return value >= `${pad(now.getHours())}:${pad(now.getMinutes())}`
      }),
  })

export type OrderValues = yup.InferType<ReturnType<typeof orderSchema>>

export const emptyOrder: OrderValues = { note: "", date: "", time: "" }

/** The preferred instant (ISO, UTC) or undefined when no date was picked. */
export function preferredAt(values: Pick<OrderValues, "date" | "time">): string | undefined {
  if (!values.date) return undefined
  const local = new Date(`${values.date}T${values.time || "12:00"}`)
  return Number.isNaN(local.getTime()) ? undefined : local.toISOString()
}
