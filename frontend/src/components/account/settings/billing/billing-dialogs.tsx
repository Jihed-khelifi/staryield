"use client";

import * as React from "react";
import Link from "next/link";
import * as yup from "yup";
import type { FieldValues, UseFormReturn } from "react-hook-form";
import { Check } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import type { TranslateCopy } from "@/i18n/translate-copy";
import { newId } from "@/lib/api/client";
import { billingApi, type PaymentMethod, type Plan, type Subscription, type SubscriptionResult } from "@/lib/api/billing";
import { toApiError } from "@/lib/api/errors";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import type { PaymentFieldsHandle } from "@/lib/payments/payment-fields";
import { consentRule } from "@/lib/schemas/common";
import { PaymentFields, type PaymentFieldsStatus } from "@/components/billing/payment-fields";
import { useToast } from "@/components/feedback/toast-provider";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Form, FormControl, FormField, FormItem, FormMessage, FormRootError } from "@/components/ui/form";

import { dialogSubmit } from "../dialog-form";
import { CardPicker, type CardPickerHandle } from "./card-picker";
import { declineMessage, planBenefits, planPrice } from "./billing-format";

const subscribeSchema = (text: TranslateCopy) => yup.object({ consent: consentRule(text) });

/** The outcome of a charge that is not a plain success. */
function nextActionUrl(result: SubscriptionResult): string | null {
  const url = result.next_action?.url;
  return url && /^https:\/\//.test(url) ? url : null;
}

/** Map a failed membership charge onto the form. */
function failCharge<T extends FieldValues>(form: Pick<UseFormReturn<T>, "setError" | "getValues" | "setFocus">, err: unknown, text: TranslateCopy) {
  const e = toApiError(err);
  if (e.code === "payment_failed") {
    form.setError("root.server", { type: e.code, message: text(declineMessage(e.details?.reason)) });
    return e;
  }
  return applyApiError(form, err, { text });
}

/** Subscribe to a membership plan: summary, benefits, card, auto-renewal consent. */
export function SubscribeDialog({
  open,
  onOpenChange,
  plan,
  methods,
  onMethodAdded,
  onSubscribed,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  plan: Plan;
  methods: PaymentMethod[];
  onMethodAdded: (method: PaymentMethod) => void;
  onSubscribed: (subscription: Subscription) => void;
}) {
  const { text, locale } = useI18n();
  const toast = useToast();
  const form = useSchemaForm(subscribeSchema, { defaultValues: { consent: false } });
  const picker = React.useRef<CardPickerHandle>(null);
  // One key per subscription attempt: retries and double clicks never charge twice.
  const attempt = React.useRef(newId());
  const [verifyUrl, setVerifyUrl] = React.useState<string | null>(null);
  const consentId = React.useId();
  const price = planPrice(plan, locale, text);
  const benefits = planBenefits(plan, text);

  async function submit() {
    try {
      const method = await picker.current?.resolve();
      if (!method) return;
      const result = await billingApi.subscribe({ plan_version_id: plan.version_id, payment_method_id: method.id }, attempt.current);
      attempt.current = newId();
      const url = nextActionUrl(result);
      if (url) {
        setVerifyUrl(url);
        onSubscribed(result.subscription);
        return;
      }
      onSubscribed(result.subscription);
      toast.show({
        tone: result.subscription.status === "active" ? "success" : "info",
        message:
          result.subscription.status === "active"
            ? text("Welcome to {value0}!", { value0: plan.name })
            : text("Your payment is being confirmed. Your membership starts as soon as it goes through."),
      });
      onOpenChange(false);
    } catch (err) {
      const e = failCharge(form, err, text);
      if (e.code === "payment_failed" || e.code === "validation_failed") attempt.current = newId();
      if (e.code === "subscription_exists") {
        toast.show({ tone: "info", message: text("You already have a membership.") });
        onOpenChange(false);
      }
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          form.reset({ consent: false });
          setVerifyUrl(null);
        }
        onOpenChange(next);
      }}
    >
      <DialogContent size="md" className="settings-dialog" data-testid="subscribe-dialog">
        <DialogHeader>
          <DialogTitle>{text("Join {value0}", { value0: plan.name })}</DialogTitle>
          <DialogDescription>{plan.description || text("Monthly membership with member benefits.")}</DialogDescription>
        </DialogHeader>
        {verifyUrl ? (
          <div className="settings-dialog-success" role="status">
            <p>{text("Your bank needs to confirm this payment.")}</p>
            <Button asChild variant="cta" size="xl">
              <a href={verifyUrl} rel="noopener noreferrer">
                {text("Confirm with my bank")}
              </a>
            </Button>
          </div>
        ) : (
          <Form {...form}>
            <form onSubmit={(event) => dialogSubmit(form, submit)(event)} noValidate className="settings-dialog-form">
              <div className="settings-plan-summary">
                <span className="settings-row-label">{plan.name}</span>
                <span className="settings-price">{price}</span>
              </div>
              {benefits.length ? (
                <ul className="settings-benefits" aria-label={text("Member benefits")}>
                  {benefits.map((benefit) => (
                    <li key={benefit}>
                      <Check aria-hidden className="size-4 shrink-0" />
                      {benefit}
                    </li>
                  ))}
                </ul>
              ) : null}
              <CardPicker
                methods={methods}
                purpose="subscription"
                planVersionId={plan.version_id}
                disabled={form.formState.isSubmitting}
                onMethodAdded={onMethodAdded}
                ref={picker}
              />
              <FormRootError />
              <FormField
                control={form.control}
                name="consent"
                render={({ field }) => (
                  <FormItem className="settings-consent-check">
                    <div className="flex items-start gap-3">
                      <FormControl>
                        <Checkbox
                          ref={field.ref}
                          name={field.name}
                          checked={field.value}
                          onCheckedChange={(checked) => field.onChange(checked === true)}
                          onBlur={field.onBlur}
                          aria-labelledby={consentId}
                          className="mt-0.5"
                          data-testid="subscribe-consent"
                        />
                      </FormControl>
                      <span id={consentId} className="text-sm">
                        {text("I agree to be charged {value0} now and at the start of every period until I cancel. I can cancel at any time in Settings.", {
                          value0: price,
                        })}{" "}
                        <Link href={`/${locale}/terms`} className="underline" target="_blank" rel="noopener">
                          {text("Terms of Use")}
                        </Link>
                      </span>
                    </div>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <DialogFooter>
                <DialogClose asChild>
                  <Button type="button" variant="outline" size="xl" className="border-ink/40">
                    {text("Cancel")}
                  </Button>
                </DialogClose>
                <Button
                  type="submit"
                  variant="cta"
                  size="xl"
                  disabled={form.formState.isSubmitting}
                  aria-busy={form.formState.isSubmitting}
                  onMouseDown={keepFocusOnPress}
                  data-testid="subscribe-confirm"
                >
                  {text(form.formState.isSubmitting ? "Processing payment…" : "Subscribe for {value0}", { value0: price })}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        )}
      </DialogContent>
    </Dialog>
  );
}

const noFields = () => yup.object({});

/** Pay a membership with another card (a past-due membership is charged at once). */
export function ChangeCardDialog({
  open,
  onOpenChange,
  subscription,
  methods,
  onMethodAdded,
  onChanged,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  subscription: Subscription;
  methods: PaymentMethod[];
  onMethodAdded: (method: PaymentMethod) => void;
  onChanged: (subscription: Subscription) => void;
}) {
  const { text } = useI18n();
  const toast = useToast();
  const form = useSchemaForm(noFields, { defaultValues: {} });
  const picker = React.useRef<CardPickerHandle>(null);
  const others = methods.filter((m) => m.id !== subscription.payment_method?.id);

  async function submit() {
    try {
      const method = await picker.current?.resolve();
      if (!method) return;
      const result = await billingApi.changeMethod(method.id);
      onChanged(result.subscription);
      toast.show({
        tone: "success",
        message: subscription.status === "past_due" ? text("Card updated and payment retried.") : text("Your membership now renews with this card."),
      });
      onOpenChange(false);
    } catch (err) {
      failCharge(form, err, text);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) form.reset({});
        onOpenChange(next);
      }}
    >
      <DialogContent size="md" className="settings-dialog" data-testid="change-card-dialog">
        <DialogHeader>
          <DialogTitle>{text("Change membership card")}</DialogTitle>
          <DialogDescription>
            {subscription.status === "past_due"
              ? text("Your last payment failed. Choose a card and we'll retry the payment right away.")
              : text("Choose the card your membership renews with.")}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={(event) => dialogSubmit(form, submit)(event)} noValidate className="settings-dialog-form">
            <CardPicker methods={others} purpose="subscription" disabled={form.formState.isSubmitting} onMethodAdded={onMethodAdded} ref={picker} />
            <FormRootError />
            <DialogFooter>
              <DialogClose asChild>
                <Button type="button" variant="outline" size="xl" className="border-ink/40">
                  {text("Cancel")}
                </Button>
              </DialogClose>
              <Button
                type="submit"
                variant="cta"
                size="xl"
                disabled={form.formState.isSubmitting}
                aria-busy={form.formState.isSubmitting}
                onMouseDown={keepFocusOnPress}
              >
                {text(form.formState.isSubmitting ? "Saving…" : subscription.status === "past_due" ? "Update card and pay" : "Use this card")}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}

const addCardSchema = () => yup.object({ make_default: yup.boolean().default(false) });
type AddCardValues = yup.InferType<ReturnType<typeof addCardSchema>>;

/** Save a new card through the provider's fields. */
export function AddCardDialog({
  open,
  onOpenChange,
  firstCard,
  onAdded,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  firstCard: boolean;
  onAdded: (method: PaymentMethod) => void;
}) {
  const { text } = useI18n();
  const toast = useToast();
  const form = useSchemaForm(addCardSchema, { defaultValues: { make_default: firstCard } });
  const fields = React.useRef<PaymentFieldsHandle>(null);
  const [status, setStatus] = React.useState<PaymentFieldsStatus>("loading");
  const defaultId = React.useId();

  async function submit(values: AddCardValues) {
    if (status !== "ready" || !fields.current) return;
    try {
      const card = await fields.current.tokenize();
      if (!card) return;
      const method = await billingApi.attachMethod({
        provider: card.provider,
        token: card.token,
        billing_country: card.billingCountry,
        make_default: firstCard || values.make_default,
      });
      onAdded(method);
      toast.show({ tone: "success", message: text("Card saved.") });
      onOpenChange(false);
    } catch (err) {
      const e = toApiError(err);
      if (e.code === "limit_reached") {
        form.setError("root.server", { type: e.code, message: text("You can save up to 5 cards. Remove one to add another.") });
        return;
      }
      failCharge(form, err, text);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) form.reset({ make_default: firstCard });
        onOpenChange(next);
      }}
    >
      <DialogContent size="md" className="settings-dialog" data-testid="add-card-dialog">
        <DialogHeader>
          <DialogTitle>{text("Add a card")}</DialogTitle>
          <DialogDescription>{text("Card details go straight to our payment provider and never reach Staryield's servers.")}</DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={(event) => dialogSubmit(form, submit)(event)} noValidate className="settings-dialog-form">
            <PaymentFields purpose="subscription" variant="dialog" disabled={form.formState.isSubmitting} onStatus={setStatus} ref={fields} />
            {firstCard ? null : (
              <FormField
                control={form.control}
                name="make_default"
                render={({ field }) => (
                  <FormItem className="flex items-center gap-2">
                    <FormControl>
                      <Checkbox
                        ref={field.ref}
                        name={field.name}
                        checked={field.value}
                        onCheckedChange={(checked) => field.onChange(checked === true)}
                        onBlur={field.onBlur}
                        aria-labelledby={defaultId}
                      />
                    </FormControl>
                    <span id={defaultId} className="text-sm">
                      {text("Make this my default card")}
                    </span>
                  </FormItem>
                )}
              />
            )}
            <FormRootError />
            <DialogFooter>
              <DialogClose asChild>
                <Button type="button" variant="outline" size="xl" className="border-ink/40">
                  {text("Cancel")}
                </Button>
              </DialogClose>
              <Button
                type="submit"
                variant="cta"
                size="xl"
                disabled={form.formState.isSubmitting || status !== "ready"}
                aria-busy={form.formState.isSubmitting}
                onMouseDown={keepFocusOnPress}
                data-testid="add-card-save"
              >
                {text(form.formState.isSubmitting ? "Saving…" : "Save card")}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
