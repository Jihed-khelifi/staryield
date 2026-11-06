/**
 * Advisor payout request (§8). The balance limits come from the advisor's
 * payout snapshot, passed as yup `context` (react-hook-form forwards
 * `useForm({ context })` to the resolver), so the schema stays a pure
 * factory. The API re-validates everything.
 */
import * as yup from "yup"

import type { TranslateCopy } from "@/i18n/translate-copy"
import type { PsychicAnalytics, WithdrawalMethod } from "@/lib/api/types"

const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/
const ACCOUNT = /^[A-Za-z0-9][A-Za-z0-9 \-./]{6,}[A-Za-z0-9]$/

export const WITHDRAWAL_METHODS = ["bank_transfer", "paypal"] as const

export type WithdrawalContext = {
  payout: Pick<PsychicAnalytics["payout"], "available_credits" | "min_withdrawal_credits"> | null
  /** Formats credit amounts in messages (defaults to plain numbers). */
  format?: (credits: number) => string
}

export type WithdrawalValues = {
  amount: string
  method: WithdrawalMethod | ""
  destination: string
}

export const emptyWithdrawal: WithdrawalValues = { amount: "", method: "bank_transfer", destination: "" }

function contextOf(options: { context?: unknown }): WithdrawalContext {
  return (options.context as WithdrawalContext | undefined) ?? { payout: null }
}

export const withdrawalSchema = (text: TranslateCopy) =>
  yup.object({
    amount: yup
      .string()
      .trim()
      .required(text("Enter an amount."))
      .test("whole", text("Enter a whole number of credits."), (value) =>
        !value || (/^\d+$/.test(value) && Number.isSafeInteger(Number(value)))
      )
      .test("bounds", function (value) {
        if (!value || !/^\d+$/.test(value)) return true
        const { payout, format = String } = contextOf(this.options)
        if (!payout) return true
        const amount = Number(value)
        if (amount < payout.min_withdrawal_credits)
          return this.createError({
            message: text("The minimum withdrawal is {min} credits.", { min: format(payout.min_withdrawal_credits) }),
          })
        if (amount > payout.available_credits)
          return this.createError({
            message: text("You can withdraw at most {max} credits.", { max: format(payout.available_credits) }),
          })
        return true
      }),
    method: yup
      .mixed<WithdrawalMethod | "">()
      .test("method", text("Choose a payout method."), (value) =>
        (WITHDRAWAL_METHODS as readonly string[]).includes(String(value ?? ""))
      ),
    destination: yup
      .string()
      .trim()
      .required(text("Enter where we should send the money."))
      .max(200, text("Keep this under 200 characters."))
      .when("method", ([method], schema) =>
        method === "paypal"
          ? schema.matches(EMAIL, text("Enter the email address of your PayPal account."))
          : schema.matches(ACCOUNT, text("Enter a valid IBAN or account number."))
      ),
  })

/** The POST /psychic/withdrawals body for validated values. */
export function toWithdrawalRequest(values: WithdrawalValues): {
  amount_credits: number
  method: WithdrawalMethod
  destination: string
} {
  return {
    amount_credits: Number(values.amount.trim()),
    method: values.method as WithdrawalMethod,
    destination: values.destination.trim(),
  }
}
