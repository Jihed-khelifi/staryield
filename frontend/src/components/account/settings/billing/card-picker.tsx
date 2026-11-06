"use client";

import * as React from "react";
import { CreditCard } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { billingApi, type PaymentMethod, type PaymentPurpose } from "@/lib/api/billing";
import type { PaymentFieldsHandle } from "@/lib/payments/payment-fields";
import { PaymentFields, type PaymentFieldsStatus } from "@/components/billing/payment-fields";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";

import { cardLabel, expiryLabel, usable } from "./billing-format";

export type CardPickerHandle = {
  /**
   * The saved card to charge: the selected one, or a new card tokenised by
   * the provider and saved first. Null when the new card's fields are invalid.
   */
  resolve: () => Promise<PaymentMethod | null>;
};

/** Choose a saved card or enter a new one (provider fields, never raw card data). */
export function CardPicker({
  methods,
  purpose,
  planVersionId,
  disabled,
  onMethodAdded,
  ref,
}: {
  methods: PaymentMethod[];
  purpose: PaymentPurpose;
  planVersionId?: string;
  disabled?: boolean;
  onMethodAdded: (method: PaymentMethod) => void;
  ref?: React.Ref<CardPickerHandle>;
}) {
  const { text } = useI18n();
  const choices = methods.filter(usable);
  const preferred = choices.find((m) => m.is_default) ?? choices[0];
  const [choice, setChoice] = React.useState<string>(preferred?.id ?? "new");
  const [fieldsStatus, setFieldsStatus] = React.useState<PaymentFieldsStatus>("loading");
  const fields = React.useRef<PaymentFieldsHandle>(null);
  const labelId = React.useId();
  const selected = choice === "new" || choices.some((m) => m.id === choice) ? choice : "new";

  React.useImperativeHandle(
    ref,
    () => ({
      async resolve() {
        if (selected !== "new") return choices.find((m) => m.id === selected) ?? null;
        if (fieldsStatus !== "ready" || !fields.current) return null;
        const card = await fields.current.tokenize();
        if (!card) return null;
        const method = await billingApi.attachMethod({
          provider: card.provider,
          token: card.token,
          billing_country: card.billingCountry,
          make_default: choices.length === 0,
        });
        onMethodAdded(method);
        setChoice(method.id);
        return method;
      },
    }),
    [selected, choices, fieldsStatus, onMethodAdded]
  );

  return (
    <div className="settings-card-picker">
      <p id={labelId} className="settings-label">
        {text("Pay with")}
      </p>
      <RadioGroup value={selected} onValueChange={setChoice} aria-labelledby={labelId} className="gap-2" disabled={disabled}>
        {choices.map((method) => (
          <label key={method.id} className="settings-row settings-radio-row">
            <RadioGroupItem value={method.id} aria-label={cardLabel(method, text)} />
            <CreditCard aria-hidden size={20} absoluteStrokeWidth strokeWidth={2} />
            <span className="settings-row-label">{cardLabel(method, text)}</span>
            <span className="settings-row-hint ml-auto">{expiryLabel(method, text)}</span>
          </label>
        ))}
        <label className="settings-row settings-radio-row">
          <RadioGroupItem value="new" aria-label={text("Use a new card")} data-testid="use-new-card" />
          <span className="settings-row-label">{text("Use a new card")}</span>
        </label>
      </RadioGroup>
      {selected === "new" ? (
        <PaymentFields
          purpose={purpose}
          planVersionId={planVersionId}
          disabled={disabled}
          variant="dialog"
          onStatus={setFieldsStatus}
          ref={fields}
        />
      ) : null}
    </div>
  );
}
