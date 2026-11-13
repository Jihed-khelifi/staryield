"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { AlertCircle, CheckCircle2, Loader2 } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { authApi, meApi } from "@/lib/api/endpoints";
import { ApiError, errorMessage, toApiError } from "@/lib/api/errors";
import type { GoogleIntent, GooglePending, User } from "@/lib/api/types";
import {
  forgetGoogleIntent,
  localizedNext,
  recallGoogleIntent,
  startGoogle,
  withAttributionFallback,
} from "@/lib/auth/google";
import { ageLanding } from "@/lib/auth/age-status"; // revisions 5.0: 18+ age gate (§24.6)
import { useSession } from "@/lib/auth/session-provider";
import { applyApiError } from "@/lib/forms/server-errors";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { googleConsentSchema } from "@/lib/schemas/auth";
import { ErrorState } from "@/components/feedback/error-state";
import { useToast } from "@/components/feedback/toast-provider";
import { Button } from "@/components/ui/button";
import { Form, FormRootError } from "@/components/ui/form";

import { AuthCard, AuthLegal, AuthShell, writeSignupEmail } from "./auth-card";
import { ReactivationOffer, type Reactivation } from "./reactivation-offer";
import { TermsField } from "./terms-field";

/** Results that are plain failures (copy and CTA by code). */
const ERROR_RESULTS = new Set([
  "account_disabled",
  "google_cancelled",
  "google_state_invalid",
  "google_email_unverified",
  "google_account_in_use",
  "google_not_allowed",
  "google_unavailable",
]);
/** Failures where starting the same sign-in again can succeed. */
const RETRY_RESULTS = new Set(["google_cancelled", "google_state_invalid", "google_unavailable"]);
const SESSION_RESULTS = new Set(["signed_in", "signed_up", "linked"]);
const PENDING_RESULTS = new Set(["signup_consent", "link_password", "reactivation"]);

type Phase =
  | { kind: "init" }
  | { kind: "signing_in"; result: string }
  | { kind: "session_failed"; error: ApiError; result: string }
  | { kind: "loading_pending" }
  | { kind: "pending_failed"; error: ApiError }
  | { kind: "consent"; email: string }
  | { kind: "reactivation"; offer: Reactivation }
  | { kind: "error"; code: string };

type Landing = { result: string | null; next: string | null };

const sessionEnded = (e: ApiError) => e.code === "unauthenticated" || e.code === "session_expired";

/**
 * `/{locale}/auth/google`: where the API sends the browser after Google
 * (docs §23.1 "Callback results"). Reads `result` and `next` once, removes
 * them from the address bar, then signs in, creates the account after the
 * terms are accepted, hands over to the password sign-in, offers
 * reactivation, or explains the failure with the matching next step.
 */
export function GoogleReturn() {
  const { text, locale } = useI18n();
  const router = useRouter();
  const toast = useToast();
  const { setUser } = useSession();
  const landing = React.useRef<Landing | null>(null);
  const [phase, setPhase] = React.useState<Phase>({ kind: "init" });

  const goTo = React.useCallback((path: string) => router.replace(path), [router]);
  const defaultLanding = React.useCallback((user: User) => (user.role === "psychic" ? `/${locale}/chatroom` : `/${locale}/profile`), [locale]);

  const finishSignIn = React.useCallback(
    async (result: string) => {
      setPhase({ kind: "signing_in", result });
      // The callback already set the session cookies: pick up the CSRF token and the auth hint.
      authApi.adoptSession();
      try {
        const me = await meApi.get();
        setUser(me);
        forgetGoogleIntent();
        const next = landing.current?.next ?? null;
        if (result === "signed_up") return goTo(`/${locale}/signup/birth`);
        if (result === "linked") {
          toast.show({ tone: "success", message: text("Google is now connected.") });
          return goTo(next ?? `/${locale}/profile/settings#security`);
        }
        return goTo(ageLanding(me, locale, next ?? defaultLanding(me)));
      } catch (err) {
        const e = toApiError(err);
        // No session came with the redirect (cookies blocked, or the attempt is stale): starting again is the fix.
        if (sessionEnded(e)) setPhase({ kind: "error", code: "google_state_invalid" });
        else setPhase({ kind: "session_failed", error: e, result });
      }
    },
    [defaultLanding, goTo, locale, setUser, text, toast],
  );

  const loadPending = React.useCallback(async () => {
    setPhase({ kind: "loading_pending" });
    let pending: GooglePending;
    try {
      pending = await authApi.googlePending();
    } catch (err) {
      const e = toApiError(err);
      if (e.code === "not_found" || e.status === 404) setPhase({ kind: "error", code: "google_state_invalid" });
      else setPhase({ kind: "pending_failed", error: e });
      return;
    }
    if (pending.kind === "signup_consent") {
      setPhase({ kind: "consent", email: pending.email });
    } else if (pending.kind === "link_password") {
      // /login explains it and prefills the e-mail; the password sign-in links Google.
      writeSignupEmail(pending.email);
      const next = landing.current?.next;
      goTo(`/${locale}/login?notice=google_link${next ? `&next=${encodeURIComponent(next)}` : ""}`);
    } else if (pending.kind === "reactivation") {
      setPhase({
        kind: "reactivation",
        offer: { kind: pending.code, token: pending.reactivation_token, deletionAt: pending.deletion_scheduled_at },
      });
    } else {
      setPhase({ kind: "error", code: "google_state_invalid" });
    }
  }, [goTo, locale]);

  React.useEffect(() => {
    // Once per mount (Strict Mode runs effects twice; the URL is stripped by then).
    if (landing.current) return;
    const params = new URLSearchParams(window.location.search);
    const result = params.get("result");
    landing.current = { result, next: localizedNext(params.get("next"), locale) };
    // Neither the result nor the destination should stay in history or be re-read on reload.
    window.history.replaceState(window.history.state, "", window.location.pathname + window.location.hash);
    // Started from a microtask: the handlers set state, which an effect body must not do synchronously.
    queueMicrotask(() => {
      if (result && SESSION_RESULTS.has(result)) void finishSignIn(result);
      else if (result && PENDING_RESULTS.has(result)) void loadPending();
      else setPhase({ kind: "error", code: result && ERROR_RESULTS.has(result) ? result : "unknown" });
    });
    // The landing is read once; the callbacks only matter for this first run.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  React.useEffect(() => {
    if (phase.kind === "error") track("error_displayed", { code: phase.code, step: "google" });
  }, [phase]);

  const titleId = React.useId();

  if (phase.kind === "consent") {
    return <ConsentCard email={phase.email} onExpired={() => setPhase({ kind: "error", code: "google_state_invalid" })} />;
  }

  let icon: React.ReactNode = <Loader2 aria-hidden className="email-link-icon animate-spin motion-reduce:animate-none" />;
  let title = "Signing you in…";
  let body: React.ReactNode = null;
  let extra: React.ReactNode = null;

  if (phase.kind === "init" || phase.kind === "signing_in" || phase.kind === "loading_pending") {
    title = phase.kind === "loading_pending" ? "One moment…" : "Signing you in…";
  } else if (phase.kind === "session_failed") {
    icon = <AlertCircle aria-hidden className="email-link-icon text-danger" />;
    title = "We couldn't finish signing you in";
    extra = <ErrorState error={phase.error} onRetry={() => void finishSignIn(phase.result)} compact />;
  } else if (phase.kind === "pending_failed") {
    icon = <AlertCircle aria-hidden className="email-link-icon text-danger" />;
    title = "Something went wrong";
    extra = <ErrorState error={phase.error} onRetry={() => void loadPending()} compact />;
  } else if (phase.kind === "reactivation") {
    icon = <CheckCircle2 aria-hidden className="email-link-icon text-success" />;
    title = "Welcome back";
    extra = (
      <>
        <ReactivationOffer
          offer={phase.offer}
          onSignedIn={(user) => {
            setUser(user);
            forgetGoogleIntent();
            goTo(ageLanding(user, locale, landing.current?.next ?? defaultLanding(user)));
          }}
        />
        <Link className="text-sm underline" href={`/${locale}/login`} data-testid="google-back">
          {text("Back to sign in")}
        </Link>
      </>
    );
  } else if (phase.kind === "error") {
    icon = <AlertCircle aria-hidden className="email-link-icon text-danger" />;
    title = phase.code === "google_cancelled" ? "Google sign-in was cancelled" : "We couldn't sign you in with Google";
    body = <p>{text(phase.code === "unknown" ? "We couldn't complete your Google sign-in. Please sign in again." : errorMessage(new ApiError({ code: phase.code, message: "", status: 0 })))}</p>;
    extra = <ErrorActions code={phase.code} />;
  }

  return (
    <AuthShell promo={false}>
      <section
        className="auth-card paper-page email-link-card"
        aria-labelledby={titleId}
        aria-busy={phase.kind === "init" || phase.kind === "signing_in" || phase.kind === "loading_pending"}
        data-testid="google-return"
        data-phase={phase.kind}
        data-code={phase.kind === "error" ? phase.code : undefined}
      >
        <div className="auth-card-body">
          {icon}
          <h1 id={titleId} className="text-center text-[28px] font-medium leading-tight" tabIndex={-1}>
            {text(title)}
          </h1>
          <div role="status" aria-live="polite" className="text-center text-sm font-light">
            {body}
          </div>
          {extra}
        </div>
        <AuthLegal />
      </section>
    </AuthShell>
  );
}

/** The buttons under a failure: "Try again" restarts the same sign-in; always a way out. */
function ErrorActions({ code }: { code: string }) {
  const { text, locale } = useI18n();
  const [busy, setBusy] = React.useState(false);
  const [problem, setProblem] = React.useState<string | null>(null);
  // What the visitor was doing when they left (sign in by default).
  const remembered = React.useMemo(() => recallGoogleIntent(), []);
  const intent: GoogleIntent = remembered?.intent ?? "signin";
  const linking = intent === "link";
  const canRetry = RETRY_RESULTS.has(code);

  async function retry() {
    setBusy(true);
    setProblem(null);
    try {
      await startGoogle({ intent, next: remembered?.next, locale });
    } catch (err) {
      setBusy(false);
      setProblem(text(errorMessage(toApiError(err))));
    }
  }

  return (
    <div className="email-link-actions" data-testid="google-error-actions">
      {problem ? (
        <p className="text-center text-sm text-danger" role="alert">
          {problem}
        </p>
      ) : null}
      {canRetry ? (
        <Button type="button" className="auth-continue gold-button w-full" onClick={() => void retry()} disabled={busy} aria-busy={busy} data-testid="google-retry">
          {text(busy ? "Opening Google…" : "Try again")}
        </Button>
      ) : null}
      {code === "account_disabled" ? (
        <Button asChild className="auth-continue gold-button w-full">
          <Link href={`/${locale}/contact`} data-testid="google-contact-support">
            {text("Contact support")}
          </Link>
        </Button>
      ) : null}
      <Button asChild variant="outline" size="lg" className="w-full border-ink/40">
        <Link href={linking ? `/${locale}/profile/settings#security` : `/${locale}/login`} data-testid="google-back">
          {text(linking ? "Back to settings" : "Back to sign in")}
        </Link>
      </Button>
    </div>
  );
}

/** "Create your Staryield account": a Google identity with no account yet, until the terms are accepted. */
function ConsentCard({ email, onExpired }: { email: string; onExpired: () => void }) {
  const { text, locale } = useI18n();
  const router = useRouter();
  const { setUser } = useSession();
  const form = useSchemaForm(googleConsentSchema, { defaultValues: { terms: false } });
  const [emailTaken, setEmailTaken] = React.useState(false);

  React.useEffect(() => {
    track("signup_step_viewed", { step: "google_consent" });
  }, []);

  async function submit() {
    setEmailTaken(false);
    try {
      // First-touch campaign attribution, dropped when the API rejects it (never blocks a sign-up).
      const res = await withAttributionFallback((attribution) => authApi.googleComplete({ accept_terms: true, locale, attribution }));
      setUser(res.user);
      forgetGoogleIntent();
      track("signup_step_completed", { step: "google_consent" });
      router.replace(`/${locale}/signup/birth`);
    } catch (err) {
      const e = toApiError(err);
      if (e.code === "google_state_invalid" || e.code === "not_found") {
        onExpired();
        return;
      }
      applyApiError(form, err, { text });
      if (e.code === "email_taken") setEmailTaken(true);
      track("error_displayed", { code: String(e.code), step: "google_consent" });
    }
  }

  return (
    <AuthShell promo={false}>
      <Form {...form}>
        <AuthCard
          title="Create your Staryield account"
          subtitle={<span data-testid="google-consent-email">{text("You're signing up with the Google account {value0}.", { value0: email })}</span>}
          onSubmit={form.handleSubmit(submit)}
          pending={form.formState.isSubmitting}
          submitLabel="Create account"
          pendingLabel="Creating your account…"
          testId="google-consent-form"
        >
          <FormRootError className="mt-4">
            {emailTaken ? (
              <Link className="font-semibold underline" href={`/${locale}/login`}>
                {text("Log In")}
              </Link>
            ) : null}
          </FormRootError>
          <TermsField control={form.control} />
          <p className="mt-5 text-xs">
            <Link className="underline" href={`/${locale}/login`}>
              {text("Back to sign in")}
            </Link>
          </p>
        </AuthCard>
      </Form>
    </AuthShell>
  );
}
