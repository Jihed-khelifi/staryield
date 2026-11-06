"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { billingApi, type PaymentPurpose } from "@/lib/api/billing";
import { errorMessage } from "@/lib/api/errors";
import type { PaymentFieldsAdapter, PaymentFieldsHandle } from "@/lib/payments/payment-fields";
import { useAsync } from "@/lib/use-async";
import { SandboxCardFields } from "@/components/billing/sandbox-card-fields";
import { Skeleton } from "@/components/ui/skeleton";

/** Adapters by `ClientConfig.type`; real processors are added here. */
export const paymentAdapters: Record<string, PaymentFieldsAdapter> = {
  sandbox: { type: "sandbox", Fields: SandboxCardFields },
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
  ref,
}: {
  purpose: PaymentPurpose;
  planVersionId?: string;
  disabled?: boolean;
  variant?: "dialog" | "checkout";
  cardholder?: string;
  onStatus?: (status: PaymentFieldsStatus) => void;
  ref?: React.Ref<PaymentFieldsHandle>;
}) {
  const { text } = useI18n();
  const config = useAsync((signal) => billingApi.clientConfig(purpose, planVersionId, signal), [purpose, planVersionId]);
  const adapter = config.data ? paymentAdapters[config.data.type] : undefined;
  const status: PaymentFieldsStatus = config.error
    ? config.error.code === "feature_unavailable"
      ? "unavailable"
      : "error"
    : config.data === null || config.loading
      ? "loading"
      : adapter
        ? "ready"
        : "unavailable";

  const report = React.useRef(onStatus);
  React.useEffect(() => {
    report.current = onStatus;
  });
  React.useEffect(() => {
    report.current?.(status);
  }, [status]);

  if (status === "loading") {
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
  if (status === "error" && config.error) {
    return (
      <div className="payment-fields-note" role="alert">
        <p>{text(errorMessage(config.error))}</p>
        <button type="button" className="underline" onClick={config.reload}>
          {text("Try again")}
        </button>
      </div>
    );
  }
  if (status === "unavailable" || !adapter || !config.data) {
    return (
      <p className="payment-fields-note" role="status">
        {text("Card payments are not available for your account yet.")}
      </p>
    );
  }
  const Fields = adapter.Fields;
  return <Fields config={config.data} disabled={disabled} variant={variant} cardholder={cardholder} ref={ref} />;
}
