"use client";

import * as React from "react";
import { useWatch } from "react-hook-form";

import { useI18n } from "@/i18n/i18n-provider";
import { authApi, signInMethodsApi } from "@/lib/api/endpoints";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { changePasswordSchema, type ChangePasswordValues } from "@/lib/schemas/settings";
import { useAsync } from "@/lib/use-async";
import { useToast } from "@/components/feedback/toast-provider";
import { PasswordChecklist } from "@/components/forms/auth/auth-card";
import { Button } from "@/components/ui/button";
import { Form, FormRootError } from "@/components/ui/form";
import { PasswordInput } from "@/components/ui/password-input";

import { DevicesList } from "./devices-list";
import { dialogSubmit } from "./dialog-form";
import { isPasswordNotSet, useCreatePasswordLink } from "./password-not-set";
import { SignInMethodsBlock } from "./sign-in-methods";
import { SettingsCard, SettingsField } from "./settings-ui";

const EMPTY: ChangePasswordValues = { current_password: "", new_password: "", confirm_password: "" };

/**
 * Password & Security: change the password (every other device is signed
 * out; this one gets a fresh session) and manage signed-in devices.
 */
export function SecurityCard({ email, onDirtyChange }: { email: string; onDirtyChange?: (dirty: boolean) => void }) {
  const { text } = useI18n();
  const toast = useToast();
  const form = useSchemaForm(changePasswordSchema, { defaultValues: EMPTY });
  const newPassword = useWatch({ control: form.control, name: "new_password" });
  const checklistId = React.useId();
  const [devicesKey, setDevicesKey] = React.useState(0);
  const methods = useAsync((signal) => signInMethodsApi.get(signal), []);
  const createPassword = useCreatePasswordLink(email);
  // Google-only accounts have no current password to ask for (§23.1).
  const noPassword = methods.data?.password === false;
  const [raceNoPassword, setRaceNoPassword] = React.useState(false);
  const dirty = form.formState.isDirty;

  const report = React.useRef(onDirtyChange);
  React.useEffect(() => {
    report.current = onDirtyChange;
  });
  React.useEffect(() => {
    report.current?.(dirty);
  }, [dirty]);

  async function submit(values: ChangePasswordValues) {
    try {
      await authApi.changePassword(values.current_password, values.new_password);
      form.reset(EMPTY);
      toast.show({ tone: "success", message: text("Password updated. Your other devices were signed out.") });
      setDevicesKey((n) => n + 1);
    } catch (err) {
      // The methods list may be stale (or failed to load): the API is the authority.
      if (isPasswordNotSet(err)) {
        setRaceNoPassword(true);
        methods.reload();
        return;
      }
      applyApiError(form, err, { text });
    }
  }

  return (
    <SettingsCard id="security" icon="shield" title={text("Password & Security")} testId="settings-security">
      <SignInMethodsBlock methods={methods} createPassword={createPassword} />
      {noPassword || raceNoPassword ? (
        <div className="settings-note grid gap-3" data-testid="password-create-state">
          <p>{text("You haven't created a password yet, so there is nothing to change here. Create one first with the “Create a password” button under Sign-in methods.")}</p>
        </div>
      ) : (
      <Form {...form}>
        <form onSubmit={(event) => dialogSubmit(form, submit)(event)} noValidate className="settings-stack" aria-label={text("Change password")} data-testid="password-form">
          {/* Lets password managers file the new password under this account. */}
          <input type="text" name="username" autoComplete="username" value={email} readOnly hidden />
          <FormRootError />
          <SettingsField<ChangePasswordValues, "current_password"> name="current_password" label={text("Current Password")} showValid={false}>
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
          <SettingsField<ChangePasswordValues, "new_password"> name="new_password" label={text("New Password")}>
            {(field) => (
              <PasswordInput
                ref={field.ref}
                name={field.name}
                value={field.value}
                onChange={field.onChange}
                onBlur={field.onBlur}
                autoComplete="new-password"
                maxLength={128}
                placeholder={text("Enter new password")}
                aria-describedby={checklistId}
                className="settings-input"
              />
            )}
          </SettingsField>
          {newPassword ? <PasswordChecklist password={newPassword} id={checklistId} /> : null}
          <SettingsField<ChangePasswordValues, "confirm_password"> name="confirm_password" label={text("Confirm New Password")}>
            {(field) => (
              <PasswordInput
                ref={field.ref}
                name={field.name}
                value={field.value}
                onChange={field.onChange}
                onBlur={field.onBlur}
                autoComplete="new-password"
                maxLength={128}
                placeholder={text("Confirm new password")}
                className="settings-input"
              />
            )}
          </SettingsField>
          {dirty || form.formState.isSubmitting ? (
            <div className="settings-inline-actions">
              <Button type="button" variant="ghost" size="lg" onClick={() => form.reset(EMPTY)} disabled={form.formState.isSubmitting}>
                {text("Cancel")}
              </Button>
              <Button
                type="submit"
                className="settings-button"
                disabled={form.formState.isSubmitting}
                aria-busy={form.formState.isSubmitting}
                onMouseDown={keepFocusOnPress}
                data-testid="update-password"
              >
                {text(form.formState.isSubmitting ? "Updating…" : "Update password")}
              </Button>
            </div>
          ) : null}
        </form>
      </Form>
      )}
      <DevicesList key={devicesKey} />
    </SettingsCard>
  );
}
