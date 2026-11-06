"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { signupEmailSchema, type SignupEmailValues } from "@/lib/schemas/auth";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { FieldRow } from "@/components/ui/field";
import { Form, FormControl, FormField, FormItem, FormMessage, FormRootError } from "@/components/ui/form";
import { Input } from "@/components/ui/input";

import { AuthCard, AuthShell, readSignupEmail, writeSignupEmail } from "./auth-card";

/** Step 1 of sign-up: email + Terms/Privacy consent. */
export function SignupEmailForm() {
  const { text, locale } = useI18n();
  const router = useRouter();
  const form = useSchemaForm(signupEmailSchema, { defaultValues: { email: "", terms: false } });
  const termsId = React.useId();

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
          <FormField
            control={form.control}
            name="terms"
            render={({ field }) => (
              <FormItem className="mt-5 gap-1">
                <div className="flex items-start gap-2 text-xs font-light">
                  <FormControl>
                    <Checkbox
                      ref={field.ref}
                      name={field.name}
                      checked={field.value}
                      onCheckedChange={(checked) => field.onChange(checked === true)}
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
        </AuthCard>
      </Form>
      <div className="mt-5 w-full max-w-[354px] text-center">
        <p className="mb-4 text-sm text-cream-light">{text("or")}</p>
        <Button
          type="button"
          variant="outline"
          className="w-full border-ink bg-cream"
          onClick={() =>
            form.setError("root.server", {
              type: "feature_unavailable",
              message: text("Google sign-up is not available yet. Please continue with your email."),
            })
          }
        >
          {text("Sign up with Google ")}
        </Button>
        <p className="mt-4 text-sm text-cream-light">
          <Link className="underline" href={`/${locale}/login`}>
            {text("Already have an account? Log in")}
          </Link>
        </p>
      </div>
    </AuthShell>
  );
}
