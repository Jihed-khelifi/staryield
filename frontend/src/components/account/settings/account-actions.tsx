"use client";

import * as React from "react";
import Link from "next/link";
import { AlertTriangle, Lock, Trash } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { accountApi, type ClosureBlocker } from "@/lib/api/account";
import { toApiError } from "@/lib/api/errors";
import { useSession } from "@/lib/auth/session-provider";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import {
  CLOSURE_CONFIRMATION,
  closeAccountSchema,
  passwordConfirmSchema,
  type CloseAccountValues,
  type PasswordConfirmValues,
} from "@/lib/schemas/settings";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Form, FormRootError } from "@/components/ui/form";
import { PasswordInput } from "@/components/ui/password-input";

import { dialogSubmit } from "./dialog-form";
import { PasswordNotSetNotice, isPasswordNotSet } from "./password-not-set";
import { SettingsCard, SettingsField, SettingsRowButton } from "./settings-ui";

/** What blocks an advisor's closure, with where to resolve it. */
const BLOCKERS: Record<ClosureBlocker, { message: string; action: string; href: string }> = {
  earnings_balance: {
    message: "You still have earnings to withdraw.",
    action: "Request a payout",
    href: "/profile/analytics",
  },
  pending_withdrawals: {
    message: "A payout is still being processed. You can close your account once it is paid.",
    action: "View payouts",
    href: "/profile/analytics",
  },
  open_requests: {
    message: "You have custom requests in progress. Deliver, decline or resolve them first.",
    action: "View requests",
    href: "/profile",
  },
};

function DeactivateDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { text, locale } = useI18n();
  const { signOut } = useSession();
  const form = useSchemaForm(passwordConfirmSchema, { defaultValues: { password: "" } });
  const [noPassword, setNoPassword] = React.useState(false);

  async function submit(values: PasswordConfirmValues) {
    setNoPassword(false);
    try {
      await accountApi.deactivate(values.password);
      await signOut({ to: `/${locale}/login?notice=deactivated` });
    } catch (err) {
      if (isPasswordNotSet(err)) {
        setNoPassword(true);
        return;
      }
      applyApiError(form, err, { text });
      form.resetField("password", { keepError: true });
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          form.reset({ password: "" });
          setNoPassword(false);
        }
        onOpenChange(next);
      }}
    >
      <DialogContent size="md" className="settings-dialog" data-testid="deactivate-dialog">
        <DialogHeader>
          <DialogTitle>{text("Disable your account?")}</DialogTitle>
          <DialogDescription>
            {text("Your profile is hidden and you are signed out on every device. Your data, credits and chats are kept. Sign in again whenever you want to reactivate it.")}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={(event) => dialogSubmit(form, submit)(event)} noValidate className="settings-dialog-form">
            {noPassword ? <PasswordNotSetNotice /> : null}
            <FormRootError />
            <SettingsField<PasswordConfirmValues, "password"> name="password" label={text("Confirm your password")} showValid={false}>
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
                  {text("Keep my account active")}
                </Button>
              </DialogClose>
              <Button
                type="submit"
                variant="destructive"
                size="xl"
                className="settings-danger-button"
                disabled={form.formState.isSubmitting}
                aria-busy={form.formState.isSubmitting}
                onMouseDown={keepFocusOnPress}
                data-testid="deactivate-confirm"
              >
                {text(form.formState.isSubmitting ? "Disabling…" : "Disable account")}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}

const CLOSE_EMPTY: CloseAccountValues = { password: "", confirmation: "", reason: "" };

function CloseAccountDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { text, locale } = useI18n();
  const { signOut } = useSession();
  const form = useSchemaForm(closeAccountSchema, { defaultValues: CLOSE_EMPTY });
  const [blockers, setBlockers] = React.useState<ClosureBlocker[] | null>(null);
  const [noPassword, setNoPassword] = React.useState(false);

  async function submit(values: CloseAccountValues) {
    setBlockers(null);
    setNoPassword(false);
    try {
      await accountApi.close({ password: values.password, confirmation: values.confirmation.trim(), reason: values.reason?.trim() || undefined });
      await signOut({ to: `/${locale}/login?notice=closure_scheduled` });
    } catch (err) {
      const e = toApiError(err);
      if (e.code === "password_not_set") {
        setNoPassword(true);
        return;
      }
      if (e.code === "closure_blocked") {
        const reasons = Array.isArray(e.details?.reasons) ? (e.details.reasons as string[]) : [];
        setBlockers(reasons.filter((r): r is ClosureBlocker => r in BLOCKERS));
        return;
      }
      applyApiError(form, err, { text });
      form.resetField("password", { keepError: true });
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          form.reset(CLOSE_EMPTY);
          setBlockers(null);
          setNoPassword(false);
        }
        onOpenChange(next);
      }}
    >
      <DialogContent size="md" className="settings-dialog" data-testid="close-dialog">
        <DialogHeader>
          <DialogTitle>{text("Close your account permanently?")}</DialogTitle>
          <DialogDescription>
            {text("Your account is paused now and deleted for good after 30 days: your profile, chats, notes and any remaining credits are erased and cannot be recovered.")}
          </DialogDescription>
        </DialogHeader>
        <p className="settings-note">
          {text("Changed your mind? Sign in again within 30 days, or use the link in the email we send you, to keep your account.")}
        </p>
        {blockers ? (
          <div className="settings-blockers" role="alert" data-testid="closure-blocked">
            <p className="flex items-center gap-2 font-semibold">
              <AlertTriangle aria-hidden className="size-4 shrink-0" />
              {text("Your account cannot be closed yet.")}
            </p>
            <ul>
              {blockers.map((reason) => (
                <li key={reason}>
                  <span>{text(BLOCKERS[reason].message)}</span>{" "}
                  <Link href={`/${locale}${BLOCKERS[reason].href}`} className="underline">
                    {text(BLOCKERS[reason].action)}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        ) : null}
        <Form {...form}>
          <form onSubmit={(event) => dialogSubmit(form, submit)(event)} noValidate className="settings-dialog-form">
            {noPassword ? <PasswordNotSetNotice /> : null}
            <FormRootError />
            <SettingsField<CloseAccountValues, "password"> name="password" label={text("Confirm your password")} showValid={false}>
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
            <SettingsField<CloseAccountValues, "confirmation">
              name="confirmation"
              label={text("Type {value0} to confirm", { value0: CLOSURE_CONFIRMATION })}
            >
              {(field) => (
                <input
                  ref={field.ref}
                  name={field.name}
                  value={field.value}
                  onChange={field.onChange}
                  onBlur={field.onBlur}
                  autoComplete="off"
                  autoCapitalize="characters"
                  spellCheck={false}
                  maxLength={20}
                  className="settings-input"
                  data-testid="close-confirmation"
                />
              )}
            </SettingsField>
            <SettingsField<CloseAccountValues, "reason">
              name="reason"
              label={text("Why are you leaving? (optional)")}
              description={text("Only our team sees this. It helps us improve.")}
              showValid={false}
            >
              {(field) => (
                <textarea
                  ref={field.ref}
                  name={field.name}
                  value={field.value ?? ""}
                  onChange={field.onChange}
                  onBlur={field.onBlur}
                  rows={3}
                  maxLength={500}
                  className="settings-input settings-textarea"
                />
              )}
            </SettingsField>
            <DialogFooter>
              <DialogClose asChild>
                <Button type="button" variant="outline" size="xl" className="border-ink/40">
                  {text("Keep my account")}
                </Button>
              </DialogClose>
              <Button
                type="submit"
                variant="destructive"
                size="xl"
                className="settings-danger-button"
                disabled={form.formState.isSubmitting}
                aria-busy={form.formState.isSubmitting}
                onMouseDown={keepFocusOnPress}
                data-testid="close-confirm"
              >
                {text(form.formState.isSubmitting ? "Closing…" : "Close account permanently")}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}

/** Account Actions (Figma "Card / Account Actions"): disable or close. */
export function AccountActionsCard() {
  const { text } = useI18n();
  const [open, setOpen] = React.useState<"deactivate" | "close" | null>(null);
  return (
    <SettingsCard id="account-actions" title={text("Account Actions")} className="settings-card-compact" testId="settings-actions">
      <div className="settings-rows">
        <SettingsRowButton
          icon={<Lock size={20} absoluteStrokeWidth strokeWidth={2} />}
          label={text("Disable Account")}
          hint={text("Temporarily")}
          onClick={() => setOpen("deactivate")}
          testId="open-deactivate"
        />
        <SettingsRowButton
          icon={<Trash size={20} absoluteStrokeWidth strokeWidth={2} />}
          label={text("Close Account")}
          hint={text("Permanently")}
          tone="danger"
          onClick={() => setOpen("close")}
          testId="open-close"
        />
      </div>
      <DeactivateDialog open={open === "deactivate"} onOpenChange={(next) => setOpen(next ? "deactivate" : null)} />
      <CloseAccountDialog open={open === "close"} onOpenChange={(next) => setOpen(next ? "close" : null)} />
    </SettingsCard>
  );
}
