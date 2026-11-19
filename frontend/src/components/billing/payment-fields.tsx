"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { billingApi, type PaymentPurpose } from "@/lib/api/billing";
import { errorMessage } from "@/lib/api/errors";
import type { PaymentFieldsAdapter, PaymentFieldsHandle, PaymentFieldsState } from "@/lib/payments/payment-fields";
import { useAsync } from "@/lib/use-async";
import { SandboxCardFields } from "@/components/billing/sandbox-card-fields";
import { StripeCardFields } from "@/components/billing/stripe-card-fields";
import { Skeleton } from "@/components/ui/skeleton";

/** Adapters by `ClientConfig.type`; real processors are added here. */
export const paymentAdapters: Record<string, PaymentFieldsAdapter> = {
  sandbox: { type: "sandbox", Fields: SandboxCardFields },
  // revisions 6.0 (§26.5): Stripe's Payment Element; it states how Stripe secures the card itself.
  stripe_elements: { type: "stripe_elements", Fields: StripeCardFields, securedBy: true, async: true },
};

export type PaymentFieldsStatus = "loading" | "ready" | "unavailable" | "error";

/**
 * Loads the routed provider's client configuration and renders its card
 * fields through the matching adapter. `ref.tokenize()` returns a provider
 * token (never card data); `onStatus` tells the parent whether card entry is
 * possible (e.g. to fall back when card payments are switched off).
 */
export function PaymentFields({
  purpose,
  planVersionId,
  disabled,
  variant = "dialog",
  cardholder,
  onStatus,
  genericNote,
  ref,
}: {
  purpose: PaymentPurpose;
  planVersionId?: string;
  disabled?: boolean;
  variant?: "dialog" | "checkout";
  cardholder?: string;
  onStatus?: (status: PaymentFieldsStatus) => void;
  /** Shown under the fields of providers that do not state their own security note (the sandbox). */
  genericNote?: React.ReactNode;
  ref?: React.Ref<PaymentFieldsHandle>;
}) {
  const { text } = useI18n();
  const config = useAsync((signal) => billingApi.clientConfig(purpose, planVersionId, signal), [purpose, planVersionId]);
  const adapter = config.data ? paymentAdapters[config.data.type] : undefined;
  // Adapters that load asynchronously (Stripe.js) report when they can tokenise.
  const [adapterState, setAdapterState] = React.useState<PaymentFieldsState>("loading");
  const configStatus: PaymentFieldsStatus = config.error
    ? config.error.code === "feature_unavailable"
      ? "unavailable"
      : "error"
    : config.data === null || config.loading
      ? "loading"
      : adapter
        ? "ready"
        : "unavailable";
  const status: PaymentFieldsStatus = configStatus === "ready" && adapter?.async ? adapterState : configStatus;

  const report = React.useRef(onStatus);
  React.useEffect(() => {
    report.current = onStatus;
  });
  React.useEffect(() => {
    report.current?.(status);
  }, [status]);

  if (configStatus === "loading") {
    return (
      <div className="payment-fields" data-variant={variant} aria-busy="true">
        <p role="status" className="sr-only">
          {text("Loading secure card fields…")}
        </p>
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
      </div>
    );
  }
  if (configStatus === "error" && config.error) {
    return (
      <div className="payment-fields-note" role="alert">
        <p>{text(errorMessage(config.error))}</p>
        <button type="button" className="underline" onClick={config.reload}>
          {text("Try again")}
        </button>
      </div>
    );
  }
  if (configStatus === "unavailable" || !adapter || !config.data) {
    return (
      <p className="payment-fields-note" role="status">
        {text("Card payments are not available for your account yet.")}
      </p>
    );
  }
  const Fields = adapter.Fields;
  return (
    <>
      <Fields config={config.data} disabled={disabled} variant={variant} cardholder={cardholder} onState={setAdapterState} ref={ref} />
      {adapter.securedBy ? null : genericNote}
    </>
  );
}
