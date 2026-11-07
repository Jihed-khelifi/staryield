"use client";
import * as React from "react";
import { Dialog } from "radix-ui";
import { useRouter } from "next/navigation";
import { useWatch } from "react-hook-form";
import * as yup from "yup";
import { LockKeyhole, X } from "lucide-react";
import { useI18n } from "@/i18n/i18n-provider";
import type { TranslateCopy } from "@/i18n/translate-copy";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Form, FormControl, FormField, FormItem, FormMessage, FormRootError } from "@/components/ui/form";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { walletApi } from "@/lib/api/endpoints";
import { FreeModeCreditsNotice } from "@/components/billing/free-chat-badge";
import { billingApi, purchaseCredits, type PaymentMethod } from "@/lib/api/billing";
import { errorMessage, toApiError } from "@/lib/api/errors";
import { newId } from "@/lib/api/client";
import type { CreditPack } from "@/lib/api/types";
import { track } from "@/lib/analytics";
import { useSession } from "@/lib/auth/session-provider";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { loginUrl, safeNext } from "@/lib/navigation";
import type { PaymentFieldsHandle } from "@/lib/payments/payment-fields";
import { consentRule, emailRule } from "@/lib/schemas/common";
import { useAsync } from "@/lib/use-async";
import { PaymentFields, type PaymentFieldsStatus } from "@/components/billing/payment-fields";
import { useToast } from "@/components/feedback/toast-provider";
import { DialogOverlay } from "@/components/ui/dialog";
import { cardLabel, declineMessage, usable } from "@/components/account/settings/billing/billing-format";

function price(cents: number, currency: string, locale: string) {
  return new Intl.NumberFormat(locale, { style: "currency", currency }).format(cents / 100);
}

/** Top-up form: a bundle, how to pay, the payment consent and an optional receipt address. */
export const checkoutSchema = (text: TranslateCopy) =>
  yup.object({
    pack_id: yup.string().required(text("Choose a credit bundle.")),
    method: yup.string().default("new"),
    consent: consentRule(text),
    receipt: yup.boolean().default(false),
    receipt_email: yup
      .string()
      .default("")
      .when("receipt", ([receipt], schema) => (receipt ? emailRule(text) : schema)),
  });

type CheckoutValues = yup.InferType<ReturnType<typeof checkoutSchema>>;

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
 * Top-up checkout (Figma 387:1599 desktop, 867:16866 mobile). Card details
 * are entered in the payment provider's fields and only a provider token —
 * or a saved card's id — is sent with the purchase. When card payments are
 * switched off for the account, the provider's own checkout is used.
 */
export function CreditCheckout() {
  const { text, locale } = useI18n();
  const router = useRouter();
  const toast = useToast();
  const { status, user, wallet, setWallet } = useSession();
  const packs = useAsync((signal) => walletApi.packs(signal), []);
  const cards = useAsync(savedCards, [user?.id], status === "authenticated");
  const form = useSchemaForm(checkoutSchema, {
    defaultValues: { pack_id: "", method: "new", consent: false, receipt: false, receipt_email: "" },
  });
  const method = useWatch({ control: form.control, name: "method" });
  const receipt = useWatch({ control: form.control, name: "receipt" });
  const fields = React.useRef<PaymentFieldsHandle>(null);
  const [fieldsStatus, setFieldsStatus] = React.useState<PaymentFieldsStatus>("loading");
  const consentId = React.useId();
  const receiptId = React.useId();
  const methodId = React.useId();
  // One idempotency key per purchase attempt: double clicks and retries can never charge twice.
  const attemptKey = React.useRef(newId());

  React.useEffect(() => {
    if (status === "anonymous") router.replace(loginUrl(locale));
  }, [status, locale, router]);

  React.useEffect(() => {
    track("checkout_opened");
  }, []);

  // Preselect the featured bundle, the default saved card and the account e-mail.
  const list = packs.data;
  React.useEffect(() => {
    if (!list?.length || form.getValues("pack_id")) return;
    form.setValue("pack_id", (list.find((p) => p.featured) ?? list[0]).id);
  }, [list, form]);
  const saved = cards.data;
  React.useEffect(() => {
    if (!saved?.length) return;
    form.setValue("method", (saved.find((m) => m.is_default) ?? saved[0]).id);
  }, [saved, form]);
  React.useEffect(() => {
    if (user?.email && !form.getValues("receipt_email")) form.setValue("receipt_email", user.email);
  }, [user?.email, form]);

  const close = () => router.push(safeNext(window.location.search, locale) ?? `/${locale}/profile`);
  const newCard = method === "new";
  // Ready when the provider fields loaded, or when card payments are off for this account.
  const paymentReady = !newCard || fieldsStatus === "ready" || fieldsStatus === "unavailable";

  async function pay(values: CheckoutValues) {
    try {
      let payment: { payment_method_id?: string; payment_token?: string; provider?: string } = {};
      if (!newCard) payment = { payment_method_id: values.method };
      else if (fieldsStatus === "ready") {
        const card = await fields.current?.tokenize();
        if (!card) return;
        payment = { payment_token: card.token, provider: card.provider };
      }
      const purchase = await purchaseCredits({
        pack_id: values.pack_id,
        idempotency_key: attemptKey.current,
        receipt_email: values.receipt ? values.receipt_email.trim() : undefined,
        ...payment,
      });
      attemptKey.current = newId();
      if (purchase.next_action?.url && /^https:\/\//.test(purchase.next_action.url)) {
        window.location.assign(purchase.next_action.url);
        return;
      }
      if (purchase.status === "succeeded") {
        if (purchase.wallet) setWallet(purchase.wallet);
        toast.show({ tone: "success", message: text("{value0} credits added to your balance.", { value0: purchase.credits.toLocaleString(locale) }) });
      } else {
        toast.show({ tone: "info", message: text("Your payment is processing. Credits appear as soon as it is confirmed.") });
      }
      close();
    } catch (err) {
      const e = toApiError(err);
      // A declined payment is final for this key; the next try is a new purchase.
      if (e.code === "payment_failed" || e.code === "validation_failed") attemptKey.current = newId();
      if (e.code === "payment_failed") {
        form.setError("root.server", { type: e.code, message: text(e.details?.reason ? declineMessage(e.details.reason) : errorMessage(e)) });
        return;
      }
      applyApiError(form, err, { text, fieldMap: { payment_token: "method", payment_method_id: "method" } });
    }
  }

  const submitting = form.formState.isSubmitting;

  return (
    <Dialog.Root
      open
      onOpenChange={(open) => {
        if (!open) close();
      }}
    >
      <Dialog.Portal>
        {/* Revisions 2.0: the page behind the top-up card is blurred. */}
        <DialogOverlay overlay="blur" className="checkout-overlay" />
        <Dialog.Content className="checkout-card" data-testid="checkout">
          <Form {...form}>
            <form onSubmit={(event) => void form.handleSubmit(pay)(event)} noValidate>
              <div className="checkout-body">
                <Dialog.Title className="checkout-title font-display text-2xl">{text("Top up credits ")}</Dialog.Title>
                <Dialog.Description className="checkout-subtitle mt-2 text-sm">{text("Secure payment checkout via StarYield ")}</Dialog.Description>
                <FreeModeCreditsNotice className="mt-4" />
                <Dialog.Close asChild>
                  <button type="button" aria-label={text("Close checkout")} className="checkout-close absolute top-8 right-6 rounded-full bg-warm-gray/30 p-2">
                    <X size={18} />
                  </button>
                </Dialog.Close>
                <div className="checkout-balance checkout-mobile-only">
                  <span>{text("Current balance")}</span>
                  <span>{text("{value0} credits", { value0: (wallet?.balance ?? 0).toLocaleString(locale) })}</span>
                </div>
                {packs.data === null && !packs.error ? (
                  <p className="mt-6 text-sm" role="status">
                    {text("Loading credit packs…")}
                  </p>
                ) : null}
                {packs.error ? (
                  <div className="mt-6 text-sm" role="alert">
                    <p className="text-danger">{text(errorMessage(packs.error))}</p>
                    <button type="button" className="mt-2 underline" onClick={packs.reload}>
                      {text("Try again")}
                    </button>
                  </div>
                ) : null}
                {packs.data && packs.data.length > 0 ? (
                  <FormField
                    control={form.control}
                    name="pack_id"
                    render={({ field }) => (
                      <FormItem className="checkout-bundles">
                        <p className="checkout-section-label checkout-mobile-only" aria-hidden>
                          {text("Choose a bundle")}
                        </p>
                        <fieldset className="bundle-grid">
                          <legend className="sr-only">{text("Credit bundle")}</legend>
                          {(packs.data as CreditPack[]).map((pack) => (
                            <label className={field.value === pack.id ? "selected" : ""} key={pack.id}>
                              <input
                                className="sr-only"
                                type="radio"
                                name={field.name}
                                value={pack.id}
                                checked={field.value === pack.id}
                                onChange={() => {
                                  field.onChange(pack.id);
                                  attemptKey.current = newId();
                                  track("checkout_pack_selected", { credits: pack.credits });
                                }}
                              />
                              <span className="text-xl">{pack.credits.toLocaleString(locale)}</span>
                              <span className="text-xs">{price(pack.price_cents, pack.currency, locale)}</span>
                            </label>
                          ))}
                        </fieldset>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                ) : null}
                {cards.data && cards.data.length > 0 ? (
                  <FormField
                    control={form.control}
                    name="method"
                    render={({ field }) => (
                      <FormItem className="checkout-section">
                        <span id={methodId} className="checkout-email-label">
                          {text("Pay with")}
                        </span>
                        <Select
                          value={field.value}
                          onValueChange={(value) => {
                            field.onChange(value);
                            attemptKey.current = newId();
                          }}
                        >
                          <SelectTrigger ref={field.ref} aria-labelledby={methodId} className="checkout-method" data-testid="checkout-method">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {(cards.data as PaymentMethod[]).map((card) => (
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
                  <div className="checkout-section">
                    <PaymentFields
                      purpose="credits"
                      variant="checkout"
                      cardholder={user?.display_name}
                      disabled={submitting}
                      onStatus={setFieldsStatus}
                      ref={fields}
                    />
                  </div>
                ) : null}
                <p className="checkout-notice checkout-desktop-only mt-6 flex items-start gap-2 rounded-lg bg-warm-gray/30 p-3 text-xs font-light">
                  <LockKeyhole size={14} className="mt-0.5 shrink-0" aria-hidden />
                  {text("Card details are entered with our payment provider and never reach or stay on Staryield servers.")}
                </p>
                <p className="checkout-notice checkout-mobile-only">
                  <LockKeyhole size={11} className="mr-1.5 inline shrink-0 align-[-1px]" aria-hidden />
                  {text("Encrypted 256-bit SSL transaction · cards never stored")}
                </p>
                <p className="checkout-section-label checkout-mobile-only">{text("Consent & notifications")}</p>
                <FormField
                  control={form.control}
                  name="consent"
                  render={({ field }) => (
                    <FormItem className="mt-6 gap-1">
                      <div className="checkout-consents flex items-start gap-3 text-sm font-light">
                        <FormControl>
                          <Checkbox
                            ref={field.ref}
                            name={field.name}
                            checked={field.value}
                            onCheckedChange={(checked) => field.onChange(checked === true)}
                            onBlur={field.onBlur}
                            aria-labelledby={consentId}
                            className="mt-0.5"
                            data-testid="checkout-consent"
                          />
                        </FormControl>
                        <span id={consentId}>{text("I consent to this payment and authorize STARYIELD to process the selected credit bundle. ")}</span>
                      </div>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="receipt"
                  render={({ field }) => (
                    <FormItem className="mt-4">
                      <div className="checkout-consents flex items-start gap-3 text-sm font-light">
                        <FormControl>
                          <Checkbox
                            ref={field.ref}
                            name={field.name}
                            checked={field.value}
                            onCheckedChange={(checked) => field.onChange(checked === true)}
                            onBlur={field.onBlur}
                            aria-labelledby={receiptId}
                            className="mt-0.5"
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
                      <FormItem className="checkout-email">
                        <label className="checkout-email-label" htmlFor="checkout-receipt-email">
                          {text("Notification email ")}
                        </label>
                        <FormControl>
                          <input
                            id="checkout-receipt-email"
                            ref={field.ref}
                            name={field.name}
                            value={field.value}
                            onChange={field.onChange}
                            onBlur={field.onBlur}
                            type="email"
                            autoComplete="email"
                            maxLength={254}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                ) : null}
                <FormRootError className="mt-4" />
              </div>
              <Button
                className="gold-button checkout-pay"
                type="submit"
                disabled={submitting || !packs.data?.length || !paymentReady}
                aria-busy={submitting}
                onMouseDown={keepFocusOnPress}
                data-testid="checkout-pay"
              >
                {submitting ? (
                  text("Processing payment…")
                ) : (
                  <>
                    <span className="checkout-pay-label-desktop">{text("Pay & secure credits ")}</span>
                    <span className="checkout-pay-label-mobile">✦ {text("Secure credits")}</span>
                  </>
                )}
              </Button>
              <p className="checkout-ssl flex items-center justify-center gap-2 py-6 text-xs text-sage">
                <LockKeyhole size={14} aria-hidden />
                {text("Encrypted 256-bit SSL transaction ")}
              </p>
              <p className="checkout-footnote checkout-mobile-only">{text("By continuing, you authorize a one-time charge. Credits do not expire.")}</p>
            </form>
          </Form>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
