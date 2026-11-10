"use client";

import * as React from "react";
import Link from "next/link";
import type { Control, FieldPath, FieldValues } from "react-hook-form";

import { useI18n } from "@/i18n/i18n-provider";
import { Checkbox } from "@/components/ui/checkbox";
import { FormControl, FormField, FormItem, FormMessage } from "@/components/ui/form";

/**
 * The Terms of Service / Privacy Policy consent checkbox shared by the e-mail
 * sign-up and the Google "create your account" step. The form needs a boolean
 * `terms` field validated with `signupEmailSchema`'s rule.
 */
export function TermsField<T extends FieldValues & { terms: boolean }>({ control }: { control: Control<T> }) {
  const { text, locale } = useI18n();
  const termsId = React.useId();
  return (
    <FormField
      control={control}
      name={"terms" as FieldPath<T>}
      render={({ field }) => (
        <FormItem className="mt-5 gap-1">
          <div className="flex items-start gap-2 text-xs font-light">
            <FormControl>
              <Checkbox
                ref={field.ref}
                name={field.name}
                checked={field.value}
                // Ticking a box is a finished answer: count it as touched so the
                // message updates at once. Otherwise it would only clear on blur,
                // i.e. on the press of the next button, and the layout shift
                // between mouse-down and mouse-up would swallow that click.
                onCheckedChange={(checked) => {
                  field.onChange(checked === true);
                  field.onBlur();
                }}
                onBlur={field.onBlur}
                aria-labelledby={termsId}
                className="mt-0.5"
              />
            </FormControl>
            <span id={termsId}>
              {text("I have read, understand and agree to the")}{" "}
              <Link className="underline" href={`/${locale}/terms`}>
                {text("Terms of Service ")}
              </Link>{" "}
              {text("and")}{" "}
              <Link className="underline" href={`/${locale}/privacy`}>
                {text("Privacy Policy ")}
              </Link>{" "}
              {text("about the service. ")}
            </span>
          </div>
          <FormMessage />
        </FormItem>
      )}
    />
  );
}
