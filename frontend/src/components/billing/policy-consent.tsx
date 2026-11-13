"use client";

import { useFormContext } from "react-hook-form";
import * as yup from "yup";

import type { TranslateCopy } from "@/i18n/translate-copy";
import { cn } from "@/lib/utils";
import { PolicyAgreementCopy } from "@/components/billing/policy-links";
import { Checkbox } from "@/components/ui/checkbox";
import { FormControl, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form";

/**
 * Required, unchecked-by-default agreement to the Terms of Use, Privacy Policy
 * and Refund & Cancellation Policy (§25.6). Shared by every checkout so the
 * wording, links and message are identical everywhere.
 */
export const policyConsentRule = (text: TranslateCopy) => {
  const message = text("Accept the Terms of Use, Privacy Policy and Refund & Cancellation Policy to continue.");
  return yup.boolean().default(false).oneOf([true], message).required(message);
};

/**
 * Checkbox + label inside a react-hook-form `<Form>`. The label is associated
 * with the box (clicking the sentence toggles it; the policy links do not),
 * a failed submit focuses the box and announces the message, and the 18 px
 * box has a 44 px touch target (`.policy-consent-box`).
 */
export function PolicyConsentField({ name = "consent", className, testId = "policy-consent" }: { name?: string; className?: string; testId?: string }) {
  const { control } = useFormContext();
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem className={cn("policy-consent", className)}>
          <div className="policy-consent-row">
            <FormControl>
              <Checkbox
                ref={field.ref}
                name={field.name}
                checked={field.value === true}
                onCheckedChange={(checked) => field.onChange(checked === true)}
                onBlur={field.onBlur}
                className="policy-consent-box"
                data-testid={testId}
              />
            </FormControl>
            <FormLabel className="policy-consent-label">
              <span>
                <PolicyAgreementCopy />
              </span>
            </FormLabel>
          </div>
          <FormMessage />
        </FormItem>
      )}
    />
  );
}
