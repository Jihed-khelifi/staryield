"use client";

import * as React from "react";
import Link from "next/link";
import { useWatch } from "react-hook-form";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { newId } from "@/lib/api/client";
import { requestsApi } from "@/lib/api/endpoints";
import type { Service } from "@/lib/api/types";
import { useRequireAdult } from "@/lib/auth/age-gate"; // revisions 5.0: 18+ age gate (§24.6)
import { useSession } from "@/lib/auth/session-provider";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import {
  emptyOrder,
  lastOrderDateIso,
  ORDER_NOTE_MAX,
  orderSchema,
  preferredAt,
  todayLocalIso,
  type OrderValues,
} from "@/lib/schemas/requests";
import { useToast } from "@/components/feedback/toast-provider";
import { Button } from "@/components/ui/button";
import { FieldRow } from "@/components/ui/field";
import { Form, FormRootError } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";

type Props = {
  service: Service;
  advisorName: string;
  slug: string;
  /** URL-encoded path to come back to after topping up. */
  returnHere: string;
};

/** Same height, border and background for the date and time inputs, so the pair lines up on one row. */
export const ORDER_FIELD_CLASS = "h-11 w-full min-w-0 appearance-none rounded-lg border border-input bg-white px-3 text-base leading-none";
/** Both fields start at the top of the row (labels on one line); helper and error text sit below the inputs. */
export const ORDER_DATETIME_ROW_CLASS = "grid grid-cols-1 items-start gap-4 sm:grid-cols-2";

/**
 * Order sheet body for a custom request (FR-07): an optional note and an
 * optional preferred date/time. Mounted per opened sheet, so each opening
 * gets one idempotency key that retries and double submits reuse.
 */
export function ServiceOrderForm({ service, advisorName, slug, returnHere }: Props) {
  const { text, locale } = useI18n();
  const { wallet, setWallet } = useSession();
  const requireAdult = useRequireAdult();
  const toast = useToast();
  const form = useSchemaForm(orderSchema, { defaultValues: emptyOrder });
  const [notice, setNotice] = React.useState("");
  const orderKey = React.useRef(newId());
  const date = useWatch({ control: form.control, name: "date" });
  const pending = form.formState.isSubmitting;
  const needsCredits = form.formState.errors.root?.server?.type === "insufficient_credits";

  async function submit(values: OrderValues) {
    if (!requireAdult()) return;
    setNotice("");
    track("service_order_started", { slug, credits: service.price_credits });
    try {
      const req = await requestsApi.order({
        service_id: service.id,
        note: values.note ?? "",
        preferred_at: preferredAt(values),
        idempotency_key: orderKey.current,
      });
      if (req.wallet) setWallet(req.wallet);
      orderKey.current = newId();
      form.reset(emptyOrder);
      setNotice(
        text("Your request was sent. {value0} credits are held until {value1} delivers it.", {
          value0: req.price_credits.toLocaleString(locale),
          value1: advisorName,
        })
      );
      toast.show({ tone: "success", message: text("Request sent to {value0}.", { value0: advisorName }) });
    } catch (err) {
      applyApiError(form, err, { text, fieldMap: { note: "note", preferred_at: "date" } });
    }
  }

  return (
    <Form {...form}>
      <form className="space-y-5 p-6" onSubmit={(event) => void form.handleSubmit(submit)(event)} noValidate data-testid="order-form">
        <p>{text(service.description)}</p>
        <FieldRow
          control={form.control}
          name="note"
          label={text("What would you like to explore?")}
          maxLength={ORDER_NOTE_MAX}
          showValid={false}
        >
          {(field) => (
            <Textarea
              {...field}
              rows={4}
              placeholder={text("Share the question or situation you want the advisor to focus on.")}
              className="min-h-28 resize-y bg-white"
            />
          )}
        </FieldRow>
        <div className={ORDER_DATETIME_ROW_CLASS} data-testid="order-datetime-row">
          <FieldRow control={form.control} name="date" label={text("Preferred date ")} labelClassName="leading-5" showValid={false}>
            {(field) => (
              <Input
                {...field}
                type="date"
                min={todayLocalIso()}
                max={lastOrderDateIso()}
                className={ORDER_FIELD_CLASS}
                data-testid="order-date"
                onChange={(event) => {
                  field.onChange(event);
                  // No date: no time either (the time field is disabled without one).
                  if (!event.target.value) form.setValue("time", "");
                  // The time rules depend on the date: re-check them once shown.
                  if (form.getFieldState("time").isTouched || form.formState.isSubmitted) void form.trigger("time");
                }}
              />
            )}
          </FieldRow>
          <FieldRow
            control={form.control}
            name="time"
            label={text("Preferred time ")}
            labelClassName="leading-5"
            description={date ? undefined : text("Pick a date first.")}
            showValid={false}
          >
            {(field) => <Input {...field} type="time" disabled={!date} className={ORDER_FIELD_CLASS} data-testid="order-time" />}
          </FieldRow>
        </div>
        <p>
          {text("{value0} Credits", { value0: service.price_credits.toLocaleString(locale) })} ·{" "}
          {text("Balance: {value0} credits", { value0: (wallet?.balance ?? 0).toLocaleString(locale) })}
        </p>
        <p className="text-sm" data-testid="order-how-it-works">
          {text("{value0} delivers this request. Your credits are held until it is delivered.", { value0: advisorName })}
          {service.delivery_days ? (
            <>
              {" "}
              {text("Delivery in about {value0} days.", { value0: service.delivery_days })}
            </>
          ) : null}
        </p>
        <p className="text-xs font-light">
          {text("Credits are held when you order and released to the advisor only when they deliver. Declined or expired requests are refunded.")}
        </p>
        <FormRootError>
          {needsCredits ? (
            <Link className="font-semibold underline" href={`/${locale}/credits?next=${returnHere}`}>
              {text("Top up credits")}
            </Link>
          ) : null}
        </FormRootError>
        <Button className="gold-button w-full" type="submit" disabled={pending} aria-busy={pending} onMouseDown={keepFocusOnPress}>
          {text(pending ? "Sending request…" : "Send request")}
        </Button>
        <p role="status" className="text-sm">
          {notice}
        </p>
      </form>
    </Form>
  );
}
