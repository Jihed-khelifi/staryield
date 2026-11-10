"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { authApi } from "@/lib/api/endpoints";
import { toApiError } from "@/lib/api/errors";
import { useSession } from "@/lib/auth/session-provider";
import { applyApiError } from "@/lib/forms/server-errors";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { nextForApi } from "@/lib/auth/google";
import { safeNext } from "@/lib/navigation";
import { loginSchema, type LoginValues } from "@/lib/schemas/auth";
import { useToast } from "@/components/feedback/toast-provider";
import { FieldRow } from "@/components/ui/field";
import { Form, FormRootError } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { PasswordInput } from "@/components/ui/password-input";
// account settings: reactivation offers after a self-deactivation or a scheduled closure.
import { ReactivationOffer, LoginNotice, reactivationFrom, type Reactivation } from "@/components/forms/auth/reactivation-offer";

import { AuthCard, AuthShell, readSignupEmail, useCountdown } from "./auth-card";
import { AuthDivider } from "./auth-divider";
import { GoogleButton, useGoogleAvailable } from "./google-button";

/** Sign in. Wrong credentials stay a form-level error (never says which field). */
export function LoginForm() {
  const { text, locale } = useI18n();
  const router = useRouter();
  const { setUser } = useSession();
  const toast = useToast();
  const googleAvailable = useGoogleAvailable();
  const form = useSchemaForm(loginSchema, { defaultValues: { email: "", password: "" } });
  const [lockedFor, setLockedFor] = React.useState<number | undefined>(undefined);
  const left = useCountdown(lockedFor);
  const [reactivation, setReactivation] = React.useState<Reactivation | null>(null);

  React.useEffect(() => {
    track("signup_step_viewed", { step: "login" });
    const saved = readSignupEmail();
    if (saved && !form.getValues("email")) form.setValue("email", saved);
  }, [form]);

  async function submit(values: LoginValues) {
    setLockedFor(undefined);
    setReactivation(null);
    try {
      const res = await authApi.login(values.email.trim(), values.password);
      setUser(res.user);
      // A password sign-in that also connected the Google identity waiting in this browser (§23.1).
      if (res.linked === "google") toast.show({ tone: "success", message: text("Google is now connected.") });
      const fallback = res.user.role === "psychic" ? `/${locale}/chatroom` : `/${locale}/profile`;
      router.push(safeNext(window.location.search, locale) ?? fallback);
    } catch (err) {
      const e = toApiError(err);
      const offer = reactivationFrom(e);
      if (offer) {
        setReactivation(offer);
        form.resetField("password", { keepError: false });
        track("error_displayed", { code: String(e.code), step: "login" });
        return;
      }
      if (e.code === "account_locked") {
        form.setError("root.server", { type: e.code, message: text("Too many sign-in attempts. Please try again later.") });
        setLockedFor(Math.max(1, Math.ceil(e.retryAfter ?? 60)));
      } else {
        applyApiError(form, err, { text });
      }
      form.resetField("password", { keepError: false });
      track("error_displayed", { code: String(e.code), step: "login" });
    }
  }

  return (
    <AuthShell promo={false}>
      <Form {...form}>
        <AuthCard
          title="Log In"
          onSubmit={form.handleSubmit(submit)}
          pending={form.formState.isSubmitting || left > 0}
          pendingLabel={left > 0 ? "Continue" : "Continuing…"}
          testId="login-form"
        >
          <LoginNotice />
          {reactivation ? (
            <ReactivationOffer
              key={reactivation.token}
              offer={reactivation}
              onSignedIn={(user) => {
                setUser(user);
                const fallback = user.role === "psychic" ? `/${locale}/chatroom` : `/${locale}/profile`;
                router.push(safeNext(window.location.search, locale) ?? fallback);
              }}
            />
          ) : null}
          <FormRootError className="mt-4">
            {left > 0 ? (
              <p role="timer" aria-live="off">
                {text("Try again in {value0}s", { value0: left })}
              </p>
            ) : null}
          </FormRootError>
          <FieldRow control={form.control} name="email" label={text("Email Address")} labelClassName="sr-only" className="mt-6" showValid={false}>
            {(field) => (
              <Input
                {...field}
                type="email"
                autoComplete="username"
                inputMode="email"
                placeholder={text("example@email.com")}
                maxLength={254}
                className="auth-input"
              />
            )}
          </FieldRow>
          <FieldRow control={form.control} name="password" label={text("Password")} labelClassName="auth-label" className="mt-6" showValid={false}>
            {(field) => (
              <PasswordInput
                {...field}
                autoComplete="current-password"
                placeholder={text("Enter your password")}
                maxLength={128}
                className="auth-input"
              />
            )}
          </FieldRow>
          <div className="mt-4 flex justify-between text-xs">
            <Link className="underline" href={`/${locale}/signup`}>
              {text("Create an account")}
            </Link>
            <Link className="underline" href={`/${locale}/forgot-password`}>
              {text("Forgot password? ")}
            </Link>
          </div>
        </AuthCard>
      </Form>
      {googleAvailable ? (
        <div className="mt-5 w-full max-w-[354px] text-center">
          <AuthDivider />
          <GoogleButton
            intent="signin"
            next={() => nextForApi(window.location.search, locale)}
            onError={(error) => {
              applyApiError(form, error, { text });
              track("error_displayed", { code: String(error.code), step: "login" });
            }}
          />
        </div>
      ) : null}
    </AuthShell>
  );
}
