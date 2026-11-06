"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { attributionForSignup } from "@/lib/attribution";
import { authApi } from "@/lib/api/endpoints";
import { toApiError } from "@/lib/api/errors";
import { useSession } from "@/lib/auth/session-provider";
import { applyApiError } from "@/lib/forms/server-errors";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { newPasswordSchema, type NewPasswordValues } from "@/lib/schemas/auth";
import { Form, FormRootError } from "@/components/ui/form";

import { AuthCard, AuthShell, clearSignupEmail, readSignupEmail } from "./auth-card";
import { NewPasswordFields } from "./new-password-fields";

/** Step 2 of sign-up: choose a password, then the account is created. */
export function SignupPasswordForm() {
  const { text, locale } = useI18n();
  const router = useRouter();
  const { setUser } = useSession();
  const form = useSchemaForm(newPasswordSchema, { defaultValues: { password: "", confirm: "" } });
  // The email belongs to step 1, so its problems get a way back there.
  const [emailProblem, setEmailProblem] = React.useState<"taken" | "invalid" | null>(null);

  React.useEffect(() => {
    track("signup_step_viewed", { step: "password" });
    if (!readSignupEmail()) router.replace(`/${locale}/signup`);
  }, [locale, router]);

  async function submit(values: NewPasswordValues) {
    setEmailProblem(null);
    const input = { email: readSignupEmail(), password: values.password, accept_terms: true, locale };
    if (!input.email) {
      router.replace(`/${locale}/signup`);
      return;
    }
    // First-touch campaign attribution (UTM / short link), when any.
    const attribution = attributionForSignup();
    try {
      let res;
      try {
        res = await authApi.register({ ...input, attribution });
      } catch (err) {
        // Attribution is best effort: never let it block a sign-up.
        const e = toApiError(err);
        if (!attribution || e.code !== "validation_failed" || !Object.keys(e.fields).some((f) => f.startsWith("attribution"))) throw err;
        res = await authApi.register(input);
      }
      setUser(res.user);
      clearSignupEmail();
      track("signup_step_completed", { step: "password" });
      router.push(`/${locale}/signup/birth`);
    } catch (err) {
      const e = toApiError(err);
      if (e.code === "validation_failed" && e.fields.email) {
        form.setError("root.server", { type: "server", message: e.fields.email });
        setEmailProblem("invalid");
      } else {
        applyApiError(form, err, { text });
        if (e.code === "email_taken") setEmailProblem("taken");
      }
      track("error_displayed", { code: String(e.code), step: "password" });
    }
  }

  return (
    <AuthShell>
      <Form {...form}>
        <AuthCard
          title="Create password"
          onSubmit={form.handleSubmit(submit)}
          pending={form.formState.isSubmitting}
          testId="signup-password-form"
        >
          <FormRootError className="mt-4">
            {emailProblem === "taken" ? (
              <Link className="font-semibold underline" href={`/${locale}/login`}>
                {text("Log In")}
              </Link>
            ) : emailProblem === "invalid" ? (
              <Link className="font-semibold underline" href={`/${locale}/signup`}>
                {text("Change email")}
              </Link>
            ) : null}
          </FormRootError>
          <NewPasswordFields control={form.control} />
        </AuthCard>
      </Form>
    </AuthShell>
  );
}
