"use client";

import * as React from "react";
import Link from "next/link";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { authApi } from "@/lib/api/endpoints";
import { toApiError } from "@/lib/api/errors";
import { applyApiError } from "@/lib/forms/server-errors";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { newPasswordSchema, type NewPasswordValues } from "@/lib/schemas/auth";
import { Form, FormRootError } from "@/components/ui/form";

import { AuthCard, AuthShell } from "./auth-card";
import { NewPasswordFields } from "./new-password-fields";

const noSubscription = () => () => {};

/** The reset token from the e-mailed link (read on the client only). */
function useResetToken(): string | null {
  return React.useSyncExternalStore(
    noSubscription,
    () => new URLSearchParams(window.location.search).get("token") ?? "",
    () => null
  );
}

/** "Choose a new password" from the e-mailed reset link. */
export function ResetPasswordForm() {
  const { text, locale } = useI18n();
  const token = useResetToken();
  const form = useSchemaForm(newPasswordSchema, { defaultValues: { password: "", confirm: "" } });
  const [done, setDone] = React.useState(false);
  const [badLink, setBadLink] = React.useState(false);

  React.useEffect(() => {
    track("signup_step_viewed", { step: "newPassword" });
  }, []);

  const missing = token === "";
  const requestNew = (
    <Link className="font-semibold underline" href={`/${locale}/forgot-password`}>
      {text("Request a new link")}
    </Link>
  );

  async function submit(values: NewPasswordValues) {
    if (!token) return;
    try {
      await authApi.resetPassword(token, values.password);
      setDone(true);
      track("signup_step_completed", { step: "newPassword" });
    } catch (err) {
      const e = toApiError(err);
      setBadLink(e.code === "invalid_token");
      applyApiError(form, err, { text, fieldMap: { password: "password" } });
      track("error_displayed", { code: String(e.code), step: "newPassword" });
    }
  }

  return (
    <AuthShell>
      <Form {...form}>
        <AuthCard
          title="Choose a new password"
          onSubmit={form.handleSubmit(submit)}
          pending={form.formState.isSubmitting || done || missing}
          testId="reset-password-form"
        >
          {missing ? (
            <FormRootError className="mt-4">
              <p>{text("This link is invalid or has expired.")}</p>
              {requestNew}
            </FormRootError>
          ) : (
            <FormRootError className="mt-4">{badLink ? requestNew : null}</FormRootError>
          )}
          {done ? (
            <p role="status" className="field-message mt-4" data-tone="success">
              {text("Your password has been updated. You can now sign in.")}{" "}
              <Link className="underline" href={`/${locale}/login`}>
                {text("Log In")}
              </Link>
            </p>
          ) : (
            <NewPasswordFields control={form.control} />
          )}
        </AuthCard>
      </Form>
    </AuthShell>
  );
}
