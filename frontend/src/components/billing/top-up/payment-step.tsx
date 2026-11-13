"use client";

import * as React from "react";
import { useWatch } from "react-hook-form";
import * as yup from "yup";
import { ArrowLeft, LockKeyhole } from "lucide-react";

import { COMPANY } from "@/config/company";
import { useI18n } from "@/i18n/i18n-provider";
import type { TranslateCopy } from "@/i18n/translate-copy";
import { billingApi, purchaseCredits, type PaymentMethod } from "@/lib/api/billing";
import { toApiError } from "@/lib/api/errors";
import type { PricingPack } from "@/lib/api/pricing";
import { useSession } from "@/lib/auth/session-provider";
import { classifyPurchaseFailure } from "@/lib/billing/top-up";
import type { AttemptKey } from "@/lib/billing/use-attempt-key";
import { purchaseConsent } from "@/lib/legal/versions";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import type { PaymentFieldsHandle } from "@/lib/payments/payment-fields";
import { formatPackPrice } from "@/lib/pricing";
import { emailRule } from "@/lib/schemas/common";
import { useAsync } from "@/lib/use-async";
import { AcceptedCards } from "@/components/billing/accepted-cards";
import { PaymentFields, type PaymentFieldsStatus } from "@/components/billing/payment-fields";
import { PolicyConsentField, policyConsentRule } from "@/components/billing/policy-consent";
import { SupportLine } from "@/components/account/support-contact";
import { useToast } from "@/components/feedback/toast-provider";
import { cardLabel, declineMessage, usable } from "@/components/account/settings/billing/billing-format";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Form, FormControl, FormField, FormItem, FormMessage, FormRootError } from "@/components/ui/form";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";

import { PurchaseAcknowledgement } from "./acknowledgement";

/** Pay form: the saved card (or a new one), an optional receipt address and the policy agreement (§25.6). */
export const paymentSchema = (text: TranslateCopy) =>
  yup.object({
    method: yup.string().default("new"),
    receipt: yup.boolean().default(false),
    consent: policyConsentRule(text),
    receipt_email: yup
      .string()
      .default("")
      .when("receipt", ([receipt], schema) => (receipt ? emailRule(text) : schema)),
  });

type PaymentValues = yup.InferType<ReturnType<typeof paymentSchema>>;

async function savedCards(signal: AbortSignal): Promise<PaymentMethod[]> {
  try {
    return (await billingApi.paymentMethods(signal)).filter(usable);
  } catch (err) {
    // Saved cards are optional here (e.g. switched off): pay with a new card.
    if (toApiError(err).code === "aborted") throw err;
    return [];
  }
}

/**
 * Step 2 (§24.7, §25.6): pay for the chosen pack. Card details are entered in
 * the payment provider's fields (`PaymentFields` adapters); only a provider
 * token or a saved card's id is sent. The buyer must tick "I agree to the Terms
 * of Use, Privacy Policy and Refund & Cancellation Policy" first; the versions
 * on screen travel with the purchase as consent evidence (§25.2). The
 * acknowledgement next to the button still states what is bought. When card
 * payments are off for the account, the provider's own checkout page is used.
 */
type StepProps = {
  pack: PricingPack;
  attempt: AttemptKey;
  onBack: () => void;
  /** The credits were added (or the payment is processing): close the dialog. */
  onPurchased: () => void;
  /** The Welcome Pack was refused: back to the packs with this message. */
  onOfferUnavailable: (message: string) => void;
  /** What the card statement shows (catalog `statement_descriptor`); the company's default when the API sends none. */
  statementDescriptor?: string;
};

export function PaymentStep(props: StepProps) {
  const { text } = useI18n();
  const { status, user } = useSession();
  const cards = useAsync(savedCards, [user?.id], status === "authenticated");
  // The form starts with the right card selected, so the saved cards load first.
  if (!cards.data)
    return (
      <div className="top-up-body top-up-pay-body" aria-busy="true" data-testid="top-up-payment-loading">
        <p role="status" className="sr-only">
          {text("Loading secure card fields…")}
        </p>
        <Skeleton className="h-11 w-full" />
        <Skeleton className="h-11 w-full" />
        <Skeleton className="h-11 w-full" />
      </div>
    );
  return <PaymentForm {...props} cards={cards.data} />;
}

function PaymentForm({ pack, attempt, onBack, onPurchased, onOfferUnavailable, statementDescriptor, cards }: StepProps & { cards: PaymentMethod[] }) {
  const { text, locale } = useI18n();
  const toast = useToast();
  const { user, setWallet } = useSession();
  const form = useSchemaForm(paymentSchema, {
    defaultValues: { method: (cards.find((m) => m.is_default) ?? cards[0])?.id ?? "new", receipt: false, receipt_email: user?.email ?? "", consent: false },
  });
  const method = useWatch({ control: form.control, name: "method" });
  const receipt = useWatch({ control: form.control, name: "receipt" });
  const fields = React.useRef<PaymentFieldsHandle>(null);
  const [fieldsStatus, setFieldsStatus] = React.useState<PaymentFieldsStatus>("loading");
  const receiptId = React.useId();
  const methodId = React.useId();
  const price = formatPackPrice(pack.price_cents, pack.currency, locale);
  const newCard = method === "new";
  // Ready when the provider fields loaded, or when card payments are off for this account.
  const paymentReady = !newCard || fieldsStatus === "ready" || fieldsStatus === "unavailable";

  async function pay(values: PaymentValues) {
    try {
      let payment: { payment_method_id?: string; payment_token?: string; provider?: string } = {};
      if (!newCard) payment = { payment_method_id: values.method };
      else if (fieldsStatus === "ready") {
        const card = await fields.current?.tokenize();
        if (!card) return;
        payment = { payment_token: card.token, provider: card.provider };
      }
      const purchase = await purchaseCredits({
        pack_id: pack.id,
        idempotency_key: attempt.get(`${pack.id}:${values.method}`),
        consent: purchaseConsent(),
        receipt_email: values.receipt ? values.receipt_email.trim() : undefined,
        ...payment,
      });
      attempt.renew();
      if (purchase.next_action?.url && /^https:\/\//.test(purchase.next_action.url)) {
        window.location.assign(purchase.next_action.url);
        return;
      }
      if (purchase.status === "succeeded") {
        // The response carries the new wallet; the realtime wallet.updated event says the same.
        if (purchase.wallet) setWallet(purchase.wallet);
        toast.show({ tone: "success", message: text("{value0} credits added to your balance.", { value0: purchase.credits.toLocaleString(locale) }) });
      } else {
        toast.show({ tone: "info", message: text("Your payment is processing. Credits appear as soon as it is confirmed.") });
      }
      onPurchased();
    } catch (err) {
      const e = toApiError(err);
      if (e.code === "aborted") return;
      const failure = classifyPurchaseFailure(e, declineMessage);
      switch (failure.kind) {
        case "declined":
          // Final for this key: the next try is a new purchase.
          attempt.renew();
          form.setError("root.server", { type: "declined", message: text(failure.message) });
          return;
        case "offer":
          attempt.renew();
          onOfferUnavailable(text(failure.message));
          return;
        case "retry":
          form.setError("root.server", { type: "retry", message: text(failure.message) });
          return;
        case "unavailable":
          form.setError("root.server", { type: "unavailable", message: text(failure.message) });
          return;
        case "age_gate":
          // Answered globally (birth date / restricted account): nothing to say here.
          return;
        default:
          if (e.code === "validation_failed") attempt.renew();
          applyApiError(form, err, { text, fieldMap: { payment_token: "method", payment_method_id: "method" } });
      }
    }
  }

  const submitting = form.formState.isSubmitting;
  const serverError = form.formState.errors.root?.server;
  const retry = serverError?.type === "retry";

  return (
    <Form {...form}>
      <form onSubmit={(event) => void form.handleSubmit(pay)(event)} noValidate className="top-up-form" data-testid="top-up-payment">
        <div className="top-up-body top-up-pay-body">
          <button type="button" className="top-up-back" onClick={onBack} disabled={submitting} data-testid="top-up-back">
            <ArrowLeft aria-hidden size={16} />
            {text("Back to packs")}
          </button>
          <div className="top-up-pay-summary" data-testid="top-up-pay-summary">
            <p>{text("{value0} · {value1} credits", { value0: pack.name, value1: pack.credits.toLocaleString(locale) })}</p>
            <p>{text("{value0} USD", { value0: price })}</p>
          </div>
          <p className="top-up-descriptor" data-testid="top-up-descriptor">
            {text("This charge appears on your card statement as {value0}.", { value0: statementDescriptor || COMPANY.statementDescriptor })}
          </p>
          {cards.length > 0 ? (
            <FormField
              control={form.control}
              name="method"
              render={({ field }) => (
                <FormItem>
                  <span id={methodId} className="top-up-label">
                    {text("Pay with")}
                  </span>
                  <Select value={field.value} onValueChange={field.onChange} disabled={submitting}>
                    <SelectTrigger ref={field.ref} aria-labelledby={methodId} className="top-up-method" data-testid="checkout-method">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {cards.map((card) => (
                        <SelectItem key={card.id} value={card.id}>
                          {cardLabel(card, text)}
                        </SelectItem>
                      ))}
                      <SelectItem value="new">{text("Use a new card")}</SelectItem>
                    </SelectContent>
                  </Select>
                </FormItem>
              )}
            />
          ) : null}
          {newCard ? (
            <PaymentFields purpose="credits" cardholder={user?.display_name} disabled={submitting} onStatus={setFieldsStatus} ref={fields} />
          ) : null}
          <p className="top-up-secure">
            <LockKeyhole size={14} aria-hidden className="shrink-0" />
            {text("Card details are entered with our payment provider and never reach or stay on Staryield servers.")}
          </p>
          <FormField
            control={form.control}
            name="receipt"
            render={({ field }) => (
              <FormItem>
                <div className="top-up-receipt">
                  <FormControl>
                    <Checkbox
                      ref={field.ref}
                      name={field.name}
                      checked={field.value}
                      onCheckedChange={(checked) => field.onChange(checked === true)}
                      onBlur={field.onBlur}
                      aria-labelledby={receiptId}
                      className="top-up-checkbox"
                    />
                  </FormControl>
                  <span id={receiptId}>{text("Send automated transaction confirmation and receipt notifications to this email address. ")}</span>
                </div>
              </FormItem>
            )}
          />
          {receipt ? (
            <FormField
              control={form.control}
              name="receipt_email"
              render={({ field }) => (
                <FormItem className="top-up-email">
                  <label className="top-up-label" htmlFor="top-up-receipt-email">
                    {text("Notification email ")}
                  </label>
                  <FormControl>
                    <input id="top-up-receipt-email" ref={field.ref} name={field.name} value={field.value} onChange={field.onChange} onBlur={field.onBlur} type="email" autoComplete="email" maxLength={254} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          ) : null}
          <FormRootError>
            {retry ? (
              <Button type="submit" variant="outline" size="lg" disabled={submitting} data-testid="top-up-retry">
                {text("Try again")}
              </Button>
            ) : null}
          </FormRootError>
        </div>
        <section className="top-up-dock" aria-label={text("Payment")}>
          <PolicyConsentField testId="top-up-consent" />
          <Button
            type="submit"
            variant="cta"
            className="top-up-cta"
            disabled={submitting || !paymentReady || serverError?.type === "unavailable"}
            aria-busy={submitting}
            onMouseDown={keepFocusOnPress}
            data-testid="checkout-pay"
          >
            {submitting ? text("Processing payment…") : text("Pay {value0} USD", { value0: price })}
          </Button>
          <PurchaseAcknowledgement pack={pack} />
          <AcceptedCards label={text("We accept")} className="top-up-cards" />
          <SupportLine className="top-up-support" />
        </section>
      </form>
    </Form>
  );
}
