"use client";

import * as React from "react";
import * as yup from "yup";

import { useI18n } from "@/i18n/i18n-provider";
import type { TranslateCopy } from "@/i18n/translate-copy";
import { requestsApi } from "@/lib/api/endpoints";
import type { CustomRequest } from "@/lib/api/types";
import { applyApiError } from "@/lib/forms/server-errors";
import { useSchemaForm, type SchemaFactory } from "@/lib/forms/use-schema-form";
import { useToast } from "@/components/feedback/toast-provider";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { FieldRow } from "@/components/ui/field";
import { Form, FormRootError } from "@/components/ui/form";
import { Textarea } from "@/components/ui/textarea";

/** API limits (backend/internal/service/requests.go, max_delivery_length). */
export const DELIVERY_MAX = 10_000;
export const DECLINE_REASON_MAX = 500;
export const DISPUTE_REASON_MAX = 2_000;

function chars(value: string | undefined) {
  return value === undefined ? 0 : Array.from(value).length;
}

/** Trimmed text with a character cap; `required` carries its own message. */
function messageRule(text: TranslateCopy, max: number, required?: string) {
  const base = yup
    .string()
    .trim()
    .test("max-chars", text("Use at most {value0} characters.", { value0: max }), (value) => chars(value) <= max);
  return required ? base.required(required) : base.default("");
}

const deliverSchema = (text: TranslateCopy) =>
  yup.object({ message: messageRule(text, DELIVERY_MAX, text("Write the reading before you deliver it.")) });
const declineSchema = (text: TranslateCopy) => yup.object({ message: messageRule(text, DECLINE_REASON_MAX) });
const disputeSchema = (text: TranslateCopy) =>
  yup.object({ message: messageRule(text, DISPUTE_REASON_MAX, text("Tell us what went wrong with this delivery.")) });

type Values = { message: string };

type ActionDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called with the updated request after a successful action. */
  onDone: (request: CustomRequest) => void;
};

function RequestActionDialog({
  open,
  onOpenChange,
  onDone,
  schema,
  title,
  description,
  label,
  hint,
  max,
  rows,
  submitLabel,
  success,
  run,
  testId,
}: ActionDialogProps & {
  schema: SchemaFactory<yup.ObjectSchema<Values>>;
  /** Translated copy. */
  title: string;
  description: string;
  label: string;
  hint?: string;
  submitLabel: string;
  success: string;
  max: number;
  rows: number;
  run: (message: string) => Promise<CustomRequest>;
  testId: string;
}) {
  const { text } = useI18n();
  const toast = useToast();
  const form = useSchemaForm<yup.ObjectSchema<Values>, Values>(schema, { defaultValues: { message: "" } });
  const { reset } = form;

  React.useEffect(() => {
    if (!open) reset({ message: "" });
  }, [open, reset]);

  const submit = form.handleSubmit(async ({ message }) => {
    try {
      const updated = await run(message.trim());
      onDone(updated);
      toast.show({ tone: "success", message: success });
      onOpenChange(false);
    } catch (err) {
      applyApiError(form, err, { text, fieldMap: { delivery: "message", reason: "message" } });
    }
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent data-testid={testId}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form noValidate onSubmit={submit} className="grid gap-4">
            <FormRootError />
            <FieldRow control={form.control} name="message" label={label} description={hint} maxLength={max} showValid={false}>
              {(field) => <Textarea rows={rows} className="min-h-28 bg-cream-light" {...field} />}
            </FieldRow>
            <DialogFooter>
              <Button type="button" variant="outline" size="lg" className="border-ink/30 px-4" onClick={() => onOpenChange(false)}>
                {text("Cancel")}
              </Button>
              <Button type="submit" variant="cta" size="lg" className="px-4" disabled={form.formState.isSubmitting} aria-busy={form.formState.isSubmitting || undefined}>
                {submitLabel}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}

/** Advisor: deliver the written reading of an accepted request. */
export function DeliverDialog({ request, ...props }: ActionDialogProps & { request: CustomRequest }) {
  const { text } = useI18n();
  return (
    <RequestActionDialog
      {...props}
      schema={deliverSchema}
      title={text("Deliver your reading")}
      description={text("The client is notified and can read it in their requests.")}
      label={text("Your reading")}
      max={DELIVERY_MAX}
      rows={8}
      submitLabel={text("Deliver")}
      success={text("Reading delivered.")}
      run={(message) => requestsApi.deliver(request.id, message)}
      testId="deliver-dialog"
    />
  );
}

/** Advisor: decline a pending request (the client is refunded). */
export function DeclineDialog({ request, ...props }: ActionDialogProps & { request: CustomRequest }) {
  const { text } = useI18n();
  return (
    <RequestActionDialog
      {...props}
      schema={declineSchema}
      title={text("Decline this request?")}
      description={text("The client is refunded in full.")}
      label={text("Reason (optional)")}
      hint={text("Shared with the client.")}
      max={DECLINE_REASON_MAX}
      rows={4}
      submitLabel={text("Decline request")}
      success={text("Request declined. The client was refunded.")}
      run={(message) => requestsApi.decline(request.id, message || undefined)}
      testId="decline-dialog"
    />
  );
}

/** Client: report a problem with a delivered request. */
export function DisputeDialog({ request, ...props }: ActionDialogProps & { request: CustomRequest }) {
  const { text } = useI18n();
  return (
    <RequestActionDialog
      {...props}
      schema={disputeSchema}
      title={text("Report a problem")}
      description={text("Our team reviews every report and replies by email.")}
      label={text("What went wrong?")}
      max={DISPUTE_REASON_MAX}
      rows={5}
      submitLabel={text("Send report")}
      success={text("Thank you. Our team will review this request.")}
      run={(message) => requestsApi.dispute(request.id, message)}
      testId="dispute-dialog"
    />
  );
}
