"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { authApi } from "@/lib/api/endpoints";
import { applyApiError } from "@/lib/forms/server-errors";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { forgotPasswordSchema, type ForgotPasswordValues } from "@/lib/schemas/auth";
import { FieldRow } from "@/components/ui/field";
import { Form, FormRootError } from "@/components/ui/form";
import { Input } from "@/components/ui/input";

import { AuthCard, AuthShell } from "./auth-card";

/** "Forgot password": always answers the same way so accounts cannot be probed. */
export function ForgotPasswordForm() {
  const { text, locale } = useI18n();
  const form = useSchemaForm(forgotPasswordSchema, { defaultValues: { email: "" } });
  const [sentTo, setSentTo] = React.useState("");

  React.useEffect(() => {
    track("signup_step_viewed", { step: "reset" });
  }, []);

  async function submit(values: ForgotPasswordValues) {
    setSentTo("");
    try {
      await authApi.forgotPassword(values.email.trim(), locale);
      setSentTo(values.email.trim());
      track("signup_step_completed", { step: "reset" });
    } catch (err) {
      const e = applyApiError(form, err, { text });
      track("error_displayed", { code: String(e.code), step: "reset" });
    }
  }

  return (
    <AuthShell>
      <Form {...form}>
        <AuthCard
          title="Reset your password"
          onSubmit={form.handleSubmit(submit)}
          pending={form.formState.isSubmitting}
          submitLabel={sentTo ? "Send again" : "Continue"}
          testId="forgot-password-form"
        >
          <FormRootError className="mt-4" />
          <FieldRow control={form.control} name="email" label={text("Email Address")} labelClassName="sr-only" className="mt-6" showValid={false}>
            {(field) => (
              <Input
                {...field}
                type="email"
                autoComplete="email"
                inputMode="email"
                placeholder={text("example@email.com")}
                maxLength={254}
                className="auth-input"
              />
            )}
          </FieldRow>
          {sentTo ? (
            <p role="status" className="field-message mt-4" data-tone="success">
              {text("If an account exists for this email, a reset link is on its way.")}
            </p>
          ) : null}
        </AuthCard>
      </Form>
    </AuthShell>
  );
}
