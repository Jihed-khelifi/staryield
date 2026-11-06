"use client";

import * as React from "react";
import { FlaskConical } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { ApiError } from "@/lib/api/errors";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { cardBrand, cardFieldsSchema, digits, formatCardNumber, formatExpiry, parseExpiry, type CardFieldsValues } from "@/lib/payments/card";
import { sandboxToken, sandboxTokenPrefix, type PaymentFieldsProps } from "@/lib/payments/payment-fields";
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form";

const BEHAVIOUR: Record<string, string> = {
  approved: "approved",
  soft_decline: "declined (insufficient funds)",
  hard_decline: "declined",
};

const BRAND_NAMES: Record<string, string> = { visa: "Visa", mastercard: "Mastercard", amex: "American Express", discover: "Discover" };

/**
 * Development sandbox card fields. They imitate a provider's hosted fields:
 * the values stay in this component, and `tokenize()` returns only the
 * sandbox token (`sbx_a_visa_4242_08_2029`) that the API accepts. The inputs
 * have no `name`, so they can never be submitted with a surrounding form.
 */
export function SandboxCardFields({ config, disabled, variant = "dialog", cardholder = "", ref }: PaymentFieldsProps) {
  const { text } = useI18n();
  const form = useSchemaForm(cardFieldsSchema, {
    defaultValues: { cardholder, number: "", expiry: "", cvc: "" } satisfies CardFieldsValues,
  });

  React.useImperativeHandle(
    ref,
    () => ({
      async tokenize() {
        const valid = await form.trigger(undefined, { shouldFocus: true });
        if (!valid) return null;
        const values = form.getValues();
        const brand = cardBrand(values.number);
        const expiry = parseExpiry(values.expiry);
        const prefix = sandboxTokenPrefix(config);
        if (!brand || !expiry) return null;
        if (!prefix) throw new ApiError({ code: "feature_unavailable", message: "Unsupported sandbox provider." });
        const last4 = digits(values.number).slice(-4);
        return { provider: config.provider, token: sandboxToken(prefix, { brand, last4, month: expiry.month, year: expiry.year }), brand, last4 };
      },
    }),
    [config, form]
  );

  const testCards = config.test_cards ?? [];

  return (
    <Form {...form}>
      <div className="payment-fields" data-variant={variant} role="group" aria-label={text("Card details")}>
        <FormField
          control={form.control}
          name="cardholder"
          render={({ field }) => (
            <FormItem className="payment-field payment-field-wide">
              <FormLabel className="payment-label">{text("Cardholder name")}</FormLabel>
              <FormControl>
                <input
                  ref={field.ref}
                  value={field.value}
                  onChange={field.onChange}
                  onBlur={field.onBlur}
                  className="payment-input"
                  autoComplete="cc-name"
                  maxLength={100}
                  disabled={disabled}
                  placeholder={text("e.g. Cassandra Moon")}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name="number"
          render={({ field }) => (
            <FormItem className="payment-field payment-field-wide">
              <FormLabel className="payment-label">{text("Card number")}</FormLabel>
              <FormControl>
                <input
                  ref={field.ref}
                  value={field.value}
                  onChange={(event) => field.onChange(formatCardNumber(event.target.value))}
                  onBlur={field.onBlur}
                  className="payment-input"
                  inputMode="numeric"
                  autoComplete="cc-number"
                  maxLength={23}
                  disabled={disabled}
                  placeholder="0000 0000 0000 0000"
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name="expiry"
          render={({ field }) => (
            <FormItem className="payment-field">
              <FormLabel className="payment-label">{text("Expiry date")}</FormLabel>
              <FormControl>
                <input
                  ref={field.ref}
                  value={field.value}
                  onChange={(event) => field.onChange(formatExpiry(event.target.value))}
                  onBlur={field.onBlur}
                  className="payment-input"
                  inputMode="numeric"
                  autoComplete="cc-exp"
                  maxLength={7}
                  disabled={disabled}
                  placeholder={text("MM / YY")}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name="cvc"
          render={({ field }) => (
            <FormItem className="payment-field payment-field-cvc">
              <FormLabel className="payment-label">{text("CVV")}</FormLabel>
              <FormControl>
                <input
                  ref={field.ref}
                  value={field.value}
                  onChange={(event) => field.onChange(digits(event.target.value).slice(0, 4))}
                  onBlur={field.onBlur}
                  className="payment-input"
                  inputMode="numeric"
                  autoComplete="cc-csc"
                  maxLength={4}
                  disabled={disabled}
                  placeholder="123"
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        {testCards.length > 0 ? (
          <div className="payment-sandbox-note" data-testid="sandbox-note">
            <FlaskConical aria-hidden className="size-3.5 shrink-0" />
            <p>
              {text("Test mode: no real card is charged. Use any future date and any security code.")}{" "}
              {testCards
                .map((card) =>
                  text("{value0} ending in {value1}: {value2}", {
                    value0: BRAND_NAMES[card.brand] ?? card.brand,
                    value1: card.last4,
                    value2: text(BEHAVIOUR[card.behaviour] ?? card.behaviour),
                  })
                )
                .join(" · ")}
            </p>
          </div>
        ) : null}
      </div>
    </Form>
  );
}
