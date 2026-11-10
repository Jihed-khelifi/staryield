"use client";

import * as React from "react";
import { MailCheck } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { emailChangeApi } from "@/lib/api/account";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { emailChangeSchema, type EmailChangeValues } from "@/lib/schemas/settings";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Form, FormRootError } from "@/components/ui/form";
import { PasswordInput } from "@/components/ui/password-input";

import { dialogSubmit } from "./dialog-form";
import { PasswordNotSetNotice, isPasswordNotSet } from "./password-not-set";
import { SettingsField } from "./settings-ui";

const EMPTY: EmailChangeValues = { email: "", password: "" };

/**
 * Change the sign-in address: the new address must be confirmed from the
 * link we e-mail to it, and the current address is told (with a link to
 * cancel). The API answers the same way whether or not the address is
 * already used by another account.
 */
export function EmailChangeDialog({
  open,
  onOpenChange,
  currentEmail,
  onRequested,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentEmail: string;
  onRequested: () => void;
}) {
  const { text } = useI18n();
  const form = useSchemaForm(emailChangeSchema, { defaultValues: EMPTY, context: { currentEmail } });
  const [sentTo, setSentTo] = React.useState<string | null>(null);
  const [noPassword, setNoPassword] = React.useState(false);

  function change(next: boolean) {
    if (!next) {
      form.reset(EMPTY);
      setSentTo(null);
      setNoPassword(false);
    }
    onOpenChange(next);
  }

  async function submit(values: EmailChangeValues) {
    const email = values.email.trim();
    setNoPassword(false);
    try {
      await emailChangeApi.request(email, values.password);
      setSentTo(email);
      form.reset(EMPTY);
      onRequested();
    } catch (err) {
      // Google-only account: offer to create a password instead of a dead end.
      if (isPasswordNotSet(err)) {
        setNoPassword(true);
        return;
      }
      applyApiError(form, err, { text });
      form.resetField("password", { keepError: true });
    }
  }

  return (
    <Dialog open={open} onOpenChange={change}>
      <DialogContent size="md" className="settings-dialog" data-testid="email-change-dialog">
        <DialogHeader>
          <DialogTitle>{text("Change your email address")}</DialogTitle>
          <DialogDescription>
            {sentTo
              ? text("Almost done. Confirm the change from your new inbox.")
              : text("We send a confirmation link to the new address. Your current address keeps working until you confirm.")}
          </DialogDescription>
        </DialogHeader>
        {sentTo ? (
          <div className="settings-dialog-success" role="status">
            <MailCheck aria-hidden className="size-6 text-success" />
            <p>{text("We sent a confirmation link to {value0}. It expires in 30 minutes.", { value0: sentTo })}</p>
            <p className="text-sm">{text("If you don't see it, check your spam folder or request the change again.")}</p>
            <DialogFooter>
              <DialogClose asChild>
                <Button type="button" variant="cta" size="xl">
                  {text("Done")}
                </Button>
              </DialogClose>
            </DialogFooter>
          </div>
        ) : (
          <Form {...form}>
            <form onSubmit={(event) => dialogSubmit(form, submit)(event)} noValidate className="settings-dialog-form">
              {noPassword ? <PasswordNotSetNotice email={currentEmail} /> : null}
              <FormRootError />
              <SettingsField<EmailChangeValues, "email"> name="email" label={text("New email address")}>
                {(field) => (
                  <input
                    ref={field.ref}
                    name={field.name}
                    value={field.value}
                    onChange={field.onChange}
                    onBlur={field.onBlur}
                    type="email"
                    inputMode="email"
                    autoComplete="email"
                    maxLength={254}
                    className="settings-input"
                  />
                )}
              </SettingsField>
              <SettingsField<EmailChangeValues, "password"> name="password" label={text("Current password")} showValid={false}>
                {(field) => (
                  <PasswordInput
                    ref={field.ref}
                    name={field.name}
                    value={field.value}
                    onChange={field.onChange}
                    onBlur={field.onBlur}
                    autoComplete="current-password"
                    maxLength={128}
                    className="settings-input"
                  />
                )}
              </SettingsField>
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
                  {text(form.formState.isSubmitting ? "Sending…" : "Send confirmation link")}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        )}
      </DialogContent>
    </Dialog>
  );
}
