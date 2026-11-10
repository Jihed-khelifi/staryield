"use client";

import * as React from "react";
import { KeyRound, Loader2 } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { signInMethodsApi } from "@/lib/api/endpoints";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import type { SignInMethods } from "@/lib/api/types";
import { startGoogle } from "@/lib/auth/google";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { passwordConfirmSchema, type PasswordConfirmValues } from "@/lib/schemas/settings";
import { GoogleG } from "@/components/brand/google-g";
import { announce } from "@/components/feedback/live-region";
import { AsyncContent } from "@/components/feedback/async-content";
import { useToast } from "@/components/feedback/toast-provider";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Form, FormRootError } from "@/components/ui/form";
import { PasswordInput } from "@/components/ui/password-input";
import { SkeletonList } from "@/components/ui/skeleton";

import { dialogSubmit } from "./dialog-form";
import { CreatePasswordAction, type CreatePasswordLink } from "./password-not-set";
import { SettingsField, SettingsSubheading } from "./settings-ui";

/** Where "Connect Google" returns to (a locale-less path, §23.1). */
export const SETTINGS_NEXT = "/profile/settings";

type MethodsState = {
  data: SignInMethods | null;
  error: ApiError | null;
  loading: boolean;
  reload: () => void;
};

/**
 * Settings → Password & Security → "Sign-in methods": whether the account has
 * a password, and the Google connection (connect, or disconnect after
 * confirming the password).
 */
export function SignInMethodsBlock({
  methods,
  createPassword,
}: {
  methods: MethodsState;
  createPassword: CreatePasswordLink;
}) {
  const { text } = useI18n();
  const headingId = React.useId();
  return (
    <div className="settings-methods grid gap-3" role="group" aria-labelledby={headingId} data-testid="sign-in-methods">
      <SettingsSubheading id={headingId}>{text("Sign-in methods")}</SettingsSubheading>
      <AsyncContent state={methods} skeleton={<SkeletonList rows={2} rowClassName="h-14" />} errorTitle={text("We couldn't load your sign-in methods.")}>
        {(data) => <MethodRows data={data} createPassword={createPassword} onChanged={methods.reload} />}
      </AsyncContent>
    </div>
  );
}

function MethodRows({ data, createPassword, onChanged }: { data: SignInMethods; createPassword: CreatePasswordLink; onChanged: () => void }) {
  const { text, locale } = useI18n();
  const [connecting, setConnecting] = React.useState(false);
  const [connectError, setConnectError] = React.useState<ApiError | null>(null);
  const [disconnectOpen, setDisconnectOpen] = React.useState(false);
  const google = data.google;

  async function connect() {
    if (connecting) return;
    setConnecting(true);
    setConnectError(null);
    try {
      await startGoogle({ intent: "link", next: SETTINGS_NEXT, locale });
      // Leaving for Google: stay busy.
    } catch (err) {
      setConnecting(false);
      setConnectError(toApiError(err));
    }
  }

  return (
    <>
      <ul className="settings-list" data-testid="sign-in-methods-list">
        <li className="settings-row" data-testid="method-password" data-state={data.password ? "set" : "not-set"}>
          <span className="settings-row-main">
            <KeyRound aria-hidden className="size-[18px] shrink-0" />
            <span className="settings-row-label">{text("Password")}</span>
          </span>
          <span className="settings-status" data-status={data.password ? "active" : "pending"}>
            {text(data.password ? "Set" : "Not set")}
          </span>
        </li>
        {google || data.google_available ? (
          <li className="settings-row" data-testid="method-google" data-state={google ? "connected" : "not-connected"}>
            <span className="settings-row-main">
              <GoogleG size={18} className="shrink-0" />
              <span className="grid gap-0.5">
                <span className="settings-row-label">{text("Google")}</span>
                {google ? (
                  <span className="settings-row-hint whitespace-normal break-all" data-testid="google-email">
                    {text("Connected as {value0}", { value0: google.email })}
                  </span>
                ) : null}
              </span>
            </span>
            {google ? (
              <button
                type="button"
                className="settings-link-button"
                onClick={() => setDisconnectOpen(true)}
                aria-label={text("Disconnect Google")}
                data-testid="disconnect-google"
              >
                {text("Disconnect")}
              </button>
            ) : (
              <Button
                type="button"
                variant="outline"
                size="lg"
                className="settings-button"
                onClick={() => void connect()}
                disabled={connecting}
                aria-busy={connecting}
                onMouseDown={keepFocusOnPress}
                data-testid="connect-google"
              >
                {connecting ? <Loader2 aria-hidden className="animate-spin motion-reduce:animate-none" /> : null}
                {text(connecting ? "Opening Google…" : "Connect Google")}
              </Button>
            )}
          </li>
        ) : null}
      </ul>
      {connectError ? (
        <p className="text-sm text-danger" role="alert" data-testid="connect-google-error">
          {text(errorMessage(connectError))}
        </p>
      ) : null}
      {data.password ? null : <CreatePasswordAction link={createPassword} />}
      {google ? (
        <DisconnectGoogleDialog
          open={disconnectOpen}
          onOpenChange={setDisconnectOpen}
          email={google.email}
          createPassword={createPassword}
          onDisconnected={onChanged}
        />
      ) : null}
    </>
  );
}

function DisconnectGoogleDialog({
  open,
  onOpenChange,
  email,
  createPassword,
  onDisconnected,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  email: string;
  createPassword: CreatePasswordLink;
  onDisconnected: () => void;
}) {
  const { text } = useI18n();
  const toast = useToast();
  const form = useSchemaForm(passwordConfirmSchema, { defaultValues: { password: "" } });
  const [needsPassword, setNeedsPassword] = React.useState(false);

  function change(next: boolean) {
    if (!next) {
      form.reset({ password: "" });
      setNeedsPassword(false);
    }
    onOpenChange(next);
  }

  async function submit(values: PasswordConfirmValues) {
    setNeedsPassword(false);
    try {
      await signInMethodsApi.disconnectGoogle(values.password);
      const message = text("Google has been disconnected.");
      toast.show({ tone: "success", message });
      announce(message);
      change(false);
      onDisconnected();
    } catch (err) {
      const e = toApiError(err);
      if (e.code === "last_sign_in_method" || e.code === "password_not_set") {
        // No password yet: disconnecting would lock the account.
        setNeedsPassword(true);
        form.resetField("password");
        return;
      }
      if (e.code === "invalid_credentials") {
        form.setError("password", { type: "server", message: text("The password is incorrect.") });
        form.resetField("password", { keepError: true });
        return;
      }
      if (e.code === "not_found") {
        // Already disconnected elsewhere: just refresh the list.
        change(false);
        onDisconnected();
        return;
      }
      applyApiError(form, err, { text });
      form.resetField("password", { keepError: true });
    }
  }

  return (
    <Dialog open={open} onOpenChange={change}>
      <DialogContent size="md" className="settings-dialog" data-testid="disconnect-google-dialog">
        <DialogHeader>
          <DialogTitle>{text("Disconnect Google?")}</DialogTitle>
          <DialogDescription>
            {text("You won't be able to sign in with {value0} through Google any more. You'll keep using your email and password.", { value0: email })}
          </DialogDescription>
        </DialogHeader>
        {needsPassword ? (
          <div className="settings-note grid gap-3" role="alert" data-testid="last-sign-in-method">
            <p>{text("Create a password before you disconnect Google, so you can still sign in.")}</p>
            <CreatePasswordAction link={createPassword} />
          </div>
        ) : null}
        <Form {...form}>
          <form onSubmit={(event) => dialogSubmit(form, submit)(event)} noValidate className="settings-dialog-form">
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
                  {text("Keep Google connected")}
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
                data-testid="disconnect-google-confirm"
              >
                {text(form.formState.isSubmitting ? "Disconnecting…" : "Disconnect Google")}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
