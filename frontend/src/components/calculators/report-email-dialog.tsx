"use client";

import * as React from "react";
import Link from "next/link";
import { AlertCircle, CheckCircle2, Mail } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { reportEmailApi, type ReportEmailLocale, type ReportEmailRequest } from "@/lib/api/calculator-report";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import { useSession } from "@/lib/auth/session-provider";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { signupUrl } from "@/lib/navigation";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import type { CalculatorFormValues, CalculatorKind } from "@/lib/schemas/calculator";
import { reportEmailSchema, type ReportEmailValues } from "@/lib/schemas/report-email";
import { AdvisorStrip } from "@/components/booking/advisor-strip";
import { announce } from "@/components/feedback/live-region";
import { reportInput } from "@/components/calculators/report-email";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { FieldRow } from "@/components/ui/field";
import { Form, FormControl, FormField, FormItem } from "@/components/ui/form";
import { Input } from "@/components/ui/input";

export type ReportEmailDialogProps = {
  kind: CalculatorKind;
  /** The values the result on screen was calculated from. */
  values: CalculatorFormValues;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The dialog was opened by the visitor's click (it may focus the e-mail field). */
  manual?: boolean;
  /** The marketing box starts ticked (the API's answer for the visitor's country). */
  prechecked: boolean;
  /** The report was e-mailed. */
  onSent?: () => void;
};

const noSubscription = () => () => {};

/**
 * The calculator's own path (locale-prefixed, no query and no `#v=` fragment: that holds the birth details),
 * for the sign-up link's `?next=`. Empty on the server and during hydration.
 */
function useCalculatorPath(locale: string, kind: CalculatorKind): string {
  const pathname = React.useSyncExternalStore(noSubscription, () => window.location.pathname, () => "");
  return pathname.startsWith(`/${locale}/`) ? pathname : `/${locale}/calculators/${kind}`;
}

/** English copy of a failed request (translated with `text()`); the report has its own wording for a few codes. */
export function reportEmailErrorCopy(error: ApiError): string {
  switch (error.code) {
    case "rate_limited":
      return "Too many requests. Please try again later.";
    case "feature_unavailable":
      return "The e-mail report isn't available right now.";
    case "validation_failed":
      return "We couldn't prepare this report. Go back, check your birth details and try again.";
    default:
      return errorMessage(error);
  }
}

/** A retry may help: the network, a timeout or a server problem (never the limit or a switched-off report). */
function retryable(error: ApiError): boolean {
  return error.code !== "rate_limited" && error.code !== "feature_unavailable" && error.code !== "validation_failed" && error.retryable;
}

/**
 * "Save your reading": e-mail the report on screen (revisions 6.0, §26.4).
 * The e-mail field (prefilled for signed-in visitors), the marketing box (ticked only
 * where the API says a pre-ticked box is lawful, never required), a hidden honeypot and
 * "Send my report". The API recalculates the report from the calculator's own request:
 * nothing computed here is trusted.
 */
export function ReportEmailDialog({ kind, values, open, onOpenChange, manual = false, prechecked, onSent }: ReportEmailDialogProps) {
  const { text } = useI18n();
  const contentRef = React.useRef<HTMLDivElement>(null);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        ref={contentRef}
        size="md"
        data-testid="report-email-dialog"
        // By itself the dialog must not pop the keyboard on a phone: focus the dialog, not the field.
        onOpenAutoFocus={(event) => {
          if (manual) return;
          event.preventDefault();
          contentRef.current?.focus();
        }}
      >
        <DialogHeader>
          <DialogTitle>{text("Save your reading")}</DialogTitle>
          <DialogDescription>{text("We'll e-mail you this report so you can read it again whenever you like. It stays on this page too.")}</DialogDescription>
        </DialogHeader>
        {/* Mounted while the dialog is open: every opening starts a fresh request. */}
        <ReportEmailBody kind={kind} values={values} onClose={() => onOpenChange(false)} prechecked={prechecked} onSent={onSent} />
      </DialogContent>
    </Dialog>
  );
}

function ReportEmailBody({
  kind,
  values,
  prechecked,
  onClose,
  onSent,
}: Pick<ReportEmailDialogProps, "kind" | "values" | "prechecked" | "onSent"> & { onClose: () => void }) {
  const { text, locale } = useI18n();
  const { user, status } = useSession();
  const calculatorPath = useCalculatorPath(locale, kind);
  const form = useSchemaForm(reportEmailSchema, { defaultValues: { email: user?.email ?? "", marketing: prechecked, website: "" } });
  const [sentTo, setSentTo] = React.useState<string | null>(null);
  const [failure, setFailure] = React.useState<ApiError | null>(null);
  const [submitting, setSubmitting] = React.useState(false);
  // Once the visitor touches the box, what they chose is theirs (never "pre-ticked").
  const [touched, setTouched] = React.useState(false);
  const markingId = React.useId();

  // The config may arrive after the dialog opened: an untouched box follows it.
  React.useEffect(() => {
    if (!touched) form.setValue("marketing", prechecked);
  }, [prechecked, touched, form]);
  // Signed-in visitors get their account's address (the session may load late).
  React.useEffect(() => {
    if (user?.email && !form.getValues("email")) form.setValue("email", user.email);
  }, [user?.email, form]);

  async function send(valuesToSend: ReportEmailValues): Promise<void> {
    const email = valuesToSend.email.trim();
    const body: ReportEmailRequest = {
      kind,
      input: reportInput(kind, values),
      email,
      locale: locale as ReportEmailLocale,
      marketing_opt_in: valuesToSend.marketing,
      marketing_prechecked: valuesToSend.marketing && prechecked && !touched,
      website: valuesToSend.website,
    };
    setSubmitting(true);
    setFailure(null);
    try {
      try {
        await reportEmailApi.send(body);
      } catch (error) {
        // The visitor's zone is only "today" for the path of life: an unknown zone must not block the report.
        const e = toApiError(error);
        const fields = Object.keys(e.fields);
        if (e.code === "validation_failed" && body.input.tz && fields.length === 1 && fields[0] === "input.tz") {
          const input = { ...body.input };
          delete input.tz;
          await reportEmailApi.send({ ...body, input });
        } else {
          throw error;
        }
      }
      setSentTo(email);
      announce(text("Your report is on its way to {value0}.", { value0: email }));
      onSent?.();
    } catch (error) {
      const e = toApiError(error);
      if (e.code === "aborted") return;
      if (e.code === "validation_failed" && e.fields.email) {
        form.setError("email", { type: "server", message: e.fields.email });
        form.setFocus("email");
      } else {
        setFailure(e);
      }
    } finally {
      setSubmitting(false);
    }
  }

  if (sentTo) {
    return (
      <div role="status" className="grid justify-items-center gap-3 py-2 text-center" data-testid="report-email-success">
        <CheckCircle2 aria-hidden className="size-10 text-success" />
        <p className="text-lg leading-snug font-medium">{text("Your report is on its way to {value0}.", { value0: sentTo })}</p>
        <p className="text-sm font-light">{text("It usually arrives within a minute. If you don't see it, check your spam folder.")}</p>
        {/* A next step: someone to ask about the result (the analytics placement measures it), and a free account for visitors. */}
        <AdvisorStrip heading="Ask an advisor about your results" placement="report_success" headingLevel={3} className="mt-2 text-left" />
        {status === "anonymous" ? (
          <Link className="text-sm underline underline-offset-2" href={signupUrl(locale, calculatorPath)} data-testid="report-email-signup">
            {text("Create a free account")}
          </Link>
        ) : null}
        <Button type="button" className="gold-button mt-2 h-12 w-full max-w-sm" onClick={onClose} data-testid="report-email-close">
          {text("Close")}
        </Button>
      </div>
    );
  }
  return (
    <Form {...form}>
      <form className="grid gap-4" onSubmit={form.handleSubmit(send)} noValidate data-testid="report-email-form">
        <FieldRow control={form.control} name="email" label={text("Email")} showValid={false}>
          {(field) => (
            <Input
              {...field}
              type="email"
              autoComplete="email"
              inputMode="email"
              placeholder={text("example@email.com")}
              maxLength={254}
              className="bg-white"
              data-testid="report-email-input"
            />
          )}
        </FieldRow>
        <FormField
          control={form.control}
          name="marketing"
          render={({ field }) => (
            <FormItem>
              <div className="flex items-start gap-3 text-sm">
                <FormControl>
                  <Checkbox
                    ref={field.ref}
                    name={field.name}
                    checked={field.value}
                    onCheckedChange={(checked) => {
                      setTouched(true);
                      field.onChange(checked === true);
                    }}
                    onBlur={field.onBlur}
                    aria-labelledby={markingId}
                    className="mt-0.5"
                    data-testid="report-email-marketing"
                  />
                </FormControl>
                <span id={markingId} className="font-light">
                  {text("Send me horoscopes, offers and news from Staryield by e-mail. You can unsubscribe at any time.")}
                </span>
              </div>
            </FormItem>
          )}
        />
        {/* Honeypot: off-screen, out of the tab order and hidden from assistive technology. */}
        <FormField
          control={form.control}
          name="website"
          render={({ field }) => (
            <div className="report-honeypot" aria-hidden="true">
              <label>
                {text("Leave this field empty")}
                <input
                  type="text"
                  name="hp-contact"
                  tabIndex={-1}
                  autoComplete="off"
                  value={field.value}
                  onChange={field.onChange}
                  data-testid="report-email-website"
                />
              </label>
            </div>
          )}
        />
        {failure ? (
          <div role="alert" className="flex items-start gap-2 rounded-lg border border-danger/50 bg-danger/5 p-3 text-sm" data-testid="report-email-error">
            <AlertCircle aria-hidden className="mt-0.5 size-4 shrink-0 text-danger" />
            <div className="grid gap-2">
              <p>{text(reportEmailErrorCopy(failure))}</p>
              {retryable(failure) ? (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="w-fit border-gold bg-cream"
                  onClick={() => void form.handleSubmit(send)()}
                  disabled={submitting}
                  data-testid="report-email-retry"
                >
                  {text("Try again")}
                </Button>
              ) : null}
            </div>
          </div>
        ) : null}
        <Button
          type="submit"
          onMouseDown={keepFocusOnPress}
          className="gold-button h-12 w-full"
          disabled={submitting}
          aria-busy={submitting}
          data-testid="report-email-submit"
        >
          <Mail aria-hidden className="size-4" />
          {text(submitting ? "Sending…" : "Send my report")}
        </Button>
        <Button type="button" variant="ghost" className="-mt-1 h-10 w-full" onClick={onClose} data-testid="report-email-dismiss">
          {text("No thanks")}
        </Button>
        <p className="text-center text-xs font-light text-[var(--muted-ink)]">
          {text(
            "We use your e-mail address to send this report and, only if the box above is ticked, to send you news and offers. We keep your birth details only until the report is sent.",
          )}{" "}
          <Link className="underline underline-offset-2" href={`/${locale}/privacy`}>
            {text("Privacy Policy")}
          </Link>
        </p>
      </form>
    </Form>
  );
}
