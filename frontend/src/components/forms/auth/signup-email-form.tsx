"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { applyApiError } from "@/lib/forms/server-errors";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { signupEmailSchema, type SignupEmailValues } from "@/lib/schemas/auth";
import { FieldRow } from "@/components/ui/field";
import { Form, FormRootError } from "@/components/ui/form";
import { Input } from "@/components/ui/input";

import { AuthCard, AuthShell, readSignupEmail, writeSignupEmail } from "./auth-card";
import { AuthDivider } from "./auth-divider";
import { GoogleButton, useGoogleAvailable } from "./google-button";
import { TermsField } from "./terms-field";

/** Step 1 of sign-up: email + Terms/Privacy consent. */
export function SignupEmailForm() {
  const { text, locale } = useI18n();
  const router = useRouter();
  const form = useSchemaForm(signupEmailSchema, { defaultValues: { email: "", terms: false } });
  const googleAvailable = useGoogleAvailable();

  React.useEffect(() => {
    track("signup_step_viewed", { step: "email" });
    const saved = readSignupEmail();
    if (saved && !form.getValues("email")) form.setValue("email", saved);
  }, [form]);

  function submit(values: SignupEmailValues) {
    writeSignupEmail(values.email.trim());
    track("signup_step_completed", { step: "email" });
    router.push(`/${locale}/signup/password`);
  }

  return (
    <AuthShell>
      <Form {...form}>
        <AuthCard title="Enter your email:" onSubmit={form.handleSubmit(submit)} testId="signup-email-form">
          <FormRootError className="mt-4" />
          <FieldRow control={form.control} name="email" label={text("Email Address")} labelClassName="sr-only" className="mt-6">
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
          <TermsField control={form.control} />
        </AuthCard>
      </Form>
      <div className="mt-5 w-full max-w-[354px] text-center">
        {googleAvailable ? (
          <>
            <AuthDivider />
            <GoogleButton
              intent="signup"
              // Same message as the e-mail path when the terms box is still empty.
              beforeStart={() => form.trigger("terms", { shouldFocus: true })}
              onError={(error) => {
                applyApiError(form, error, { text });
                track("error_displayed", { code: String(error.code), step: "email" });
              }}
            />
          </>
        ) : null}
        <p className="mt-4 text-sm text-cream-light">
          <Link className="underline" href={`/${locale}/login`}>
            {text("Already have an account? Log in")}
          </Link>
        </p>
      </div>
    </AuthShell>
  );
}
