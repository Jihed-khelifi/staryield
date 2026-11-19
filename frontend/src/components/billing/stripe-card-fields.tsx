"use client";

import * as React from "react";
import { Elements, PaymentElement, useElements, useStripe } from "@stripe/react-stripe-js";
import type { Stripe, StripeElementsOptions, StripeError } from "@stripe/stripe-js";

import { STRIPE_FONTS } from "@/config/payments";
import { useI18n } from "@/i18n/i18n-provider";
import type { NextAction } from "@/lib/api/billing";
import { ApiError } from "@/lib/api/errors";
import { finishStripeAction, runNextAction } from "@/lib/payments/next-action";
import type { NextActionResult, PaymentFieldsHandle, PaymentFieldsProps, PaymentFieldsState, TokenizedCard } from "@/lib/payments/payment-fields";
import { getStripe, readPaymentTokens, stripeAppearance, stripeLocale } from "@/lib/payments/stripe";
import { SecuredByStripe } from "@/components/billing/secured-by-stripe";
import { Skeleton } from "@/components/ui/skeleton";

type Loaded = { stripe: Stripe } | { error: true } | null;

/** Stripe's validation errors are shown by the element itself; anything else is ours to show. */
function isValidation(error: StripeError): boolean {
  return error.type === "validation_error";
}

/**
 * Stripe's Payment Element in our design (revisions 6.0, §26.5). The card
 * details live in Stripe's iframe and never reach Staryield. `tokenize()`
 * returns a ConfirmationToken (payment mode, deferred intent) or the payment
 * method id of a confirmed SetupIntent (setup mode, "Add a card").
 */
export function StripeCardFields({ config, disabled, variant = "dialog", cardholder, onState, ref }: PaymentFieldsProps) {
  const { locale } = useI18n();
  const [attempt, setAttempt] = React.useState(0);
  const [loaded, setLoaded] = React.useState<Loaded>(null);
  const key = config.publishable_key;

  React.useEffect(() => {
    let cancelled = false;
    // Resolve here, not inside <Elements>, so a failed script load is ours to show (with a retry).
    if (!key) {
      Promise.resolve().then(() => !cancelled && setLoaded({ error: true }));
      return;
    }
    getStripe(key, locale).then(
      (stripe) => !cancelled && setLoaded(stripe ? { stripe } : { error: true }),
      () => !cancelled && setLoaded({ error: true }),
    );
    return () => {
      cancelled = true;
    };
  }, [key, locale, attempt]);

  const stripe = loaded && "stripe" in loaded ? loaded.stripe : null;
  const failed = loaded !== null && "error" in loaded;
  const setup = config.mode === "setup";
  const clientSecret = config.client_secret;
  const amount = config.amount_cents;
  const currency = config.currency;
  const options = React.useMemo<StripeElementsOptions>(() => {
    const appearance = stripeAppearance(readPaymentTokens());
    const common = { appearance, fonts: STRIPE_FONTS, locale: stripeLocale(locale) };
    if (setup && clientSecret) return { clientSecret, ...common };
    return {
      mode: "payment",
      amount: amount ?? 0,
      currency: (currency ?? "usd").toLowerCase(),
      paymentMethodTypes: ["card"],
      ...common,
    };
  }, [setup, clientSecret, amount, currency, locale]);

  const retry = React.useCallback(() => setAttempt((n) => n + 1), []);
  return (
    <div className="payment-stripe" data-variant={variant} data-testid="stripe-fields">
      {stripe ? (
        <Elements stripe={stripe} options={options} key={attempt}>
          <PaymentElementFields config={config} disabled={disabled} variant={variant} cardholder={cardholder} onState={onState} onRetry={retry} ref={ref} />
        </Elements>
      ) : (
        <UnavailableOrLoading failed={failed} onRetry={retry} onState={onState} ref={ref} />
      )}
      <SecuredByStripe livemode={config.livemode} />
    </div>
  );
}

/** While Stripe.js loads (skeleton) or when it could not (message, retry). */
function UnavailableOrLoading({
  failed,
  onRetry,
  onState,
  ref,
}: {
  failed: boolean;
  onRetry: () => void;
  onState?: (state: PaymentFieldsState) => void;
  ref?: React.Ref<PaymentFieldsHandle>;
}) {
  const { text } = useI18n();
  useReportState(onState, failed ? "error" : "loading");
  // Nothing to tokenise yet: the parent's pay button is off, and tokenising says so.
  React.useImperativeHandle(ref, () => notReadyHandle(), []);
  if (failed)
    return (
      <div className="payment-fields-note" role="alert">
        <p>{text("The secure card form could not be loaded. Check your connection and try again.")}</p>
        <button type="button" className="underline" onClick={onRetry}>
          {text("Try again")}
        </button>
      </div>
    );
  return <ElementSkeleton />;
}

function ElementSkeleton() {
  const { text } = useI18n();
  return (
    <div className="payment-stripe-skeleton" aria-busy="true" data-testid="stripe-skeleton">
      <p role="status" className="sr-only">
        {text("Loading secure card fields…")}
      </p>
      <Skeleton className="h-11 w-full" />
      <Skeleton className="h-11 w-full" />
      <Skeleton className="h-11 w-full" />
    </div>
  );
}

function notReadyHandle(): PaymentFieldsHandle {
  return {
    tokenize: () => Promise.reject(new ApiError({ code: "feature_unavailable", message: "The card form is not ready." })),
    handleNextAction: (next: NextAction) => runNextAction(next),
  };
}

/** Report the state to the parent whenever it changes (not on every render). */
function useReportState(onState: ((state: PaymentFieldsState) => void) | undefined, state: PaymentFieldsState) {
  const report = React.useRef(onState);
  React.useEffect(() => {
    report.current = onState;
  });
  React.useEffect(() => {
    report.current?.(state);
  }, [state]);
}

function PaymentElementFields({ config, disabled, onState, onRetry, ref }: PaymentFieldsProps & { onRetry: () => void }) {
  const { text, locale } = useI18n();
  const stripe = useStripe();
  const elements = useElements();
  const [ready, setReady] = React.useState(false);
  const [loadError, setLoadError] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const setup = config.mode === "setup";
  useReportState(onState, loadError ? "error" : ready ? "ready" : "loading");

  /** Stripe shows validation errors in the element; anything else is shown here. */
  const invalid = React.useCallback(
    (stripeError: StripeError): null => {
      if (!isValidation(stripeError)) setError(stripeError.message || text("We couldn't read your card details. Please check them and try again."));
      return null;
    },
    [text],
  );
  const failure = React.useCallback((): null => {
    setError(text("We couldn't read your card details. Please check them and try again."));
    return null;
  }, [text]);

  React.useImperativeHandle(
    ref,
    () => ({
      async tokenize(): Promise<TokenizedCard | null> {
        if (!stripe || !elements || !ready) return null;
        setError(null);
        const provider = config.provider || "stripe";
        if (setup) {
          // The SetupIntent is confirmed here (3-D Secure in Stripe's modal); only the payment method id is sent on.
          const { error: submitError } = await elements.submit();
          if (submitError) return invalid(submitError);
          const result = await stripe.confirmSetup({
            elements,
            redirect: "if_required",
            confirmParams: { return_url: `${window.location.origin}/${locale}/profile/settings` },
          });
          if (result.error) return invalid(result.error);
          const method = result.setupIntent.payment_method;
          const id = typeof method === "string" ? method : method?.id;
          if (!id || result.setupIntent.status !== "succeeded") return failure();
          return { provider, token: id };
        }
        const { error: submitError } = await elements.submit();
        if (submitError) return invalid(submitError);
        const { confirmationToken, error: tokenError } = await stripe.createConfirmationToken({ elements });
        if (tokenError) return invalid(tokenError);
        if (!confirmationToken) return failure();
        const preview = confirmationToken.payment_method_preview;
        return {
          provider,
          token: confirmationToken.id,
          brand: preview.card?.brand,
          last4: preview.card?.last4,
          billingCountry: preview.billing_details?.address?.country ?? undefined,
        };
      },
      async handleNextAction(next: NextAction): Promise<NextActionResult> {
        if (next.type === "use_stripe_sdk" && next.client_secret && stripe) return finishStripeAction(stripe, next.client_secret);
        return runNextAction(next);
      },
    }),
    [stripe, elements, ready, setup, config.provider, locale, invalid, failure],
  );

  return (
    <div className="payment-stripe-frame" data-ready={ready} aria-disabled={disabled || undefined}>
      {!ready && !loadError ? <ElementSkeleton /> : null}
      {loadError ? (
        <div className="payment-fields-note" role="alert">
          <p>{text("The secure card form could not be loaded. Check your connection and try again.")}</p>
          <button type="button" className="underline" onClick={onRetry}>
            {text("Try again")}
          </button>
        </div>
      ) : (
        <div className="payment-stripe-element" aria-hidden={!ready} style={disabled ? { pointerEvents: "none", opacity: 0.6 } : undefined}>
          <PaymentElement
            options={{ layout: "tabs", readOnly: disabled }}
            onReady={() => setReady(true)}
            onLoadError={() => setLoadError(true)}
            onChange={() => error && setError(null)}
          />
        </div>
      )}
      {error ? (
        <p className="payment-error" role="alert" data-testid="stripe-error">
          {error}
        </p>
      ) : null}
    </div>
  );
}
