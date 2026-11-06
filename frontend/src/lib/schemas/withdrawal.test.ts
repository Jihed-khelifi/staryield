import { describe, expect, it } from "vitest";

import { emptyWithdrawal, toWithdrawalRequest, withdrawalSchema, type WithdrawalContext, type WithdrawalValues } from "@/lib/schemas/withdrawal";
import { fieldErrors, identityText as text } from "@/test/schema";

const payout = { available_credits: 5000, min_withdrawal_credits: 2500 };
const ok: WithdrawalValues = { amount: "3000", method: "bank_transfer", destination: "FR76 3000 6000 0112 3456 7890 189" };
const schema = withdrawalSchema(text);
const errorsFor = (patch: Partial<WithdrawalValues>, context: WithdrawalContext = { payout }) =>
  fieldErrors(schema, { ...ok, ...patch }, context);

describe("withdrawalSchema", () => {
  it("accepts a valid request and builds the trimmed API body", () => {
    expect(errorsFor({ destination: `  ${ok.destination}  ` })).toEqual({});
    expect(errorsFor({ amount: "2500", method: "paypal", destination: "me@example.com" })).toEqual({});
    expect(errorsFor({ amount: "5000" })).toEqual({});
    const values = schema.validateSync({ ...ok, amount: " 3000 ", destination: `  ${ok.destination}  ` }, { context: { payout } });
    expect(toWithdrawalRequest(values as WithdrawalValues)).toEqual({ amount_credits: 3000, method: "bank_transfer", destination: ok.destination });
  });

  it.each([
    [{ amount: "" }, "amount", "Enter an amount."],
    [{ amount: "12.5" }, "amount", "Enter a whole number of credits."],
    [{ amount: "-3" }, "amount", "Enter a whole number of credits."],
    [{ amount: "99999999999999999999" }, "amount", "Enter a whole number of credits."],
    [{ amount: "100" }, "amount", "The minimum withdrawal is 2500 credits."],
    [{ amount: "6000" }, "amount", "You can withdraw at most 5000 credits."],
    [{ method: "" }, "method", "Choose a payout method."],
    [{ destination: "  " }, "destination", "Enter where we should send the money."],
    [{ destination: "x".repeat(201) }, "destination", "Keep this under 200 characters."],
    [{ method: "paypal", destination: "not-an-email" }, "destination", "Enter the email address of your PayPal account."],
    [{ method: "paypal", destination: ok.destination }, "destination", "Enter the email address of your PayPal account."],
    [{ destination: "12" }, "destination", "Enter a valid IBAN or account number."],
    [{ destination: "me@example.com" }, "destination", "Enter a valid IBAN or account number."],
  ] as const)("rejects %o", (patch, field, message) => {
    expect(errorsFor(patch)[field]).toBe(message);
  });

  it("formats the balance limits through the yup context", () => {
    const format = (n: number) => new Intl.NumberFormat("en").format(n);
    expect(errorsFor({ amount: "1" }, { payout, format }).amount).toBe("The minimum withdrawal is 2,500 credits.");
    expect(errorsFor({ amount: "9999" }, { payout, format }).amount).toBe("You can withdraw at most 5,000 credits.");
  });

  it("only checks the balance bounds once the payout snapshot is known", () => {
    expect(errorsFor({ amount: "1" }, { payout: null })).toEqual({});
    expect(fieldErrors(schema, { ...ok, amount: "1" })).toEqual({});
    expect(errorsFor({ amount: "abc" }, { payout: null }).amount).toBe("Enter a whole number of credits.");
  });

  it("starts empty on bank transfer", () => {
    expect(emptyWithdrawal).toEqual({ amount: "", method: "bank_transfer", destination: "" });
    expect(Object.keys(fieldErrors(schema, emptyWithdrawal, { payout })).sort()).toEqual(["amount", "destination"]);
  });
});
