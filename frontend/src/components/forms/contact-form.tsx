"use client";

import * as React from "react";
import Link from "next/link";
import { CheckCircle2 } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { supportApi } from "@/lib/api/endpoints";
import type { TicketReceipt } from "@/lib/api/types";
import { useSession } from "@/lib/auth/session-provider";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { contactSchema, isSupportTopic, SUPPORT_TOPICS, supportTopicLabels, type ContactValues } from "@/lib/schemas/contact";
import { PublicFrame } from "@/components/layout/public-frame";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { FieldRow } from "@/components/ui/field";
import { Form, FormControl, FormField, FormItem, FormMessage, FormRootError } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";

const MESSAGE_MAX = 5000;

function blank(email = ""): ContactValues {
  return { topic: "general", subject: "", email, message: "", consent: false };
}

/** Support portal: sends a ticket and shows its reference (SY-XXXXXX). */
export function ContactForm() {
  const { text, locale } = useI18n();
  const { user } = useSession();
  const form = useSchemaForm(contactSchema, { defaultValues: blank() });
  const [receipt, setReceipt] = React.useState<(TicketReceipt & { email: string }) | null>(null);
  const consentId = React.useId();

  // ?topic=privacy (linked from the Privacy Notice) preselects the topic;
  // signed-in visitors get their account email prefilled.
  React.useEffect(() => {
    const topic = new URLSearchParams(window.location.search).get("topic");
    if (isSupportTopic(topic)) form.setValue("topic", topic);
  }, [form]);
  React.useEffect(() => {
    if (user?.email && !form.getValues("email")) form.setValue("email", user.email);
  }, [user?.email, form]);

  async function submit(values: ContactValues) {
    try {
      const res = await supportApi.createTicket({
        topic: values.topic,
        subject: values.subject.trim(),
        email: values.email.trim(),
        message: values.message.trim(),
        consent: values.consent,
      });
      setReceipt({ ...res, email: values.email.trim() });
    } catch (err) {
      applyApiError(form, err, { text });
    }
  }

  return (
    <PublicFrame>
      <section className="contact-page">
        <p className="eyebrow text-center">{text("Support portal")}</p>
        <h1 className="mt-4 text-center font-display text-[44px]">{text("Contact us ")}</h1>
        <div className="mx-auto my-6 h-px w-24 bg-gold" />
        {receipt ? (
          <div className="contact-card paper-page" role="status" data-testid="contact-success">
            <CheckCircle2 aria-hidden className="mx-auto size-10 text-success" />
            <h2 className="mt-4 text-center text-2xl font-medium">{text("Your request was sent")}</h2>
            <p className="mt-3 text-center font-light">
              {text("Your reference is {value0}. Keep it handy if you write to us again about this request.", {
                value0: receipt.reference,
              })}
            </p>
            <p className="mt-2 text-center text-sm font-light">
              {text("We will reply to {value0}.", { value0: receipt.email })}
            </p>
            <p className="mt-6 text-center font-display text-2xl tracking-wider" data-testid="contact-reference">
              {receipt.reference}
            </p>
            {user ? (
              <p className="mt-4 text-center text-sm" data-testid="contact-follow">
                {text("Follow this request and read our replies in")}{" "}
                <Link className="font-semibold underline underline-offset-4" href={`/${locale}/support/requests/${receipt.reference}`}>
                  {text("your support requests")}
                </Link>
                .
              </p>
            ) : (
              <p className="mt-4 text-center text-sm" data-testid="contact-private-link">
                {text("We e-mailed you a private link to follow your request")}
              </p>
            )}
            <div className="mt-6 flex flex-wrap justify-center gap-3">
              <Button
                type="button"
                variant="outline"
                className="border-gold bg-cream"
                onClick={() => {
                  form.reset(blank(user?.email ?? ""));
                  setReceipt(null);
                }}
              >
                {text("Send another request")}
              </Button>
              <Button asChild className="gold-button">
                <Link href={`/${locale}/faq`}>{text("Browse the FAQ")}</Link>
              </Button>
            </div>
          </div>
        ) : (
          <Form {...form}>
            <form className="contact-card paper-page" onSubmit={form.handleSubmit(submit)} noValidate data-testid="contact-form">
              <p className="mb-7 border-b border-gold/30 pb-6 font-light leading-relaxed">
                {text(
                  "Have questions about your spiritual readings, natal chart accuracy, or billing? Our dedicated support circle is here to offer guidance and align your technical path. "
                )}
              </p>
              <FormRootError className="mb-6" />
              <div className="grid gap-6">
                <FormField
                  control={form.control}
                  name="topic"
                  render={({ field }) => (
                    <FormItem>
                      <span className="text-sm" id={`${consentId}-topic`}>
                        {text("Topic")}
                      </span>
                      {/* Radix echoes "" through its hidden <select> when the value
                          changes before the options mount (e.g. ?topic=): ignore it. */}
                      <Select
                        name={field.name}
                        value={field.value}
                        onValueChange={(value) => {
                          if (isSupportTopic(value)) field.onChange(value);
                        }}
                      >
                        <FormControl>
                          <SelectTrigger aria-labelledby={`${consentId}-topic`} onBlur={field.onBlur} ref={field.ref} className="bg-white">
                            <SelectValue />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {SUPPORT_TOPICS.map((topic) => (
                            <SelectItem key={topic} value={topic}>
                              {text(supportTopicLabels[topic])}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FieldRow control={form.control} name="subject" label={text("Subject")} maxLength={120}>
                  {(field) => <Input {...field} placeholder={text("Enter your subject")} maxLength={160} className="bg-white" />}
                </FieldRow>
                <FieldRow control={form.control} name="email" label={text("Email")}>
                  {(field) => (
                    <Input
                      {...field}
                      type="email"
                      autoComplete="email"
                      inputMode="email"
                      placeholder={text("example@email.com")}
                      maxLength={254}
                      className="bg-white"
                    />
                  )}
                </FieldRow>
                <FieldRow
                  control={form.control}
                  name="message"
                  label={text("Describe your issue")}
                  description={text("Please describe your situation in as much detail as possible so we can help you quickly and accurately.")}
                  maxLength={MESSAGE_MAX}
                >
                  {(field) => (
                    <Textarea
                      {...field}
                      placeholder={text("Describe your issue in detail…")}
                      rows={6}
                      className="min-h-36 resize-y bg-white"
                    />
                  )}
                </FieldRow>
                <FormField
                  control={form.control}
                  name="consent"
                  render={({ field }) => (
                    <FormItem className="gap-1">
                      <div className="flex items-start gap-3 text-xs">
                        <FormControl>
                          <Checkbox
                            ref={field.ref}
                            name={field.name}
                            checked={field.value}
                            onCheckedChange={(checked) => field.onChange(checked === true)}
                            onBlur={field.onBlur}
                            aria-labelledby={consentId}
                            className="mt-0.5"
                          />
                        </FormControl>
                        <span id={consentId}>
                          {text("I accept Staryield’s")}{" "}
                          <Link className="underline" href={`/${locale}/terms`}>
                            {text("Terms of Use ")}
                          </Link>{" "}
                          {text("and")}{" "}
                          <Link className="underline" href={`/${locale}/privacy`}>
                            {text("Privacy Policy ")}
                          </Link>{" "}
                          {text("to the extent they apply to the processing of this request. ")}
                        </span>
                      </div>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <Button
                type="submit"
                onMouseDown={keepFocusOnPress}
                className="gold-button mx-auto mt-8 flex h-12 w-full max-w-sm"
                disabled={form.formState.isSubmitting}
                aria-busy={form.formState.isSubmitting}
              >
                {text(form.formState.isSubmitting ? "Sending…" : "Submit ")}
              </Button>
            </form>
          </Form>
        )}
      </section>
    </PublicFrame>
  );
}
