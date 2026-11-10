"use client";

import * as React from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { FormProvider } from "react-hook-form";

import { localePath, type Locale } from "@/i18n/config";
import { useI18n } from "@/i18n/i18n-provider";
import { settingsApi, type Settings, type SettingsPatch } from "@/lib/api/account";
import { toApiError } from "@/lib/api/errors";
import { useSession } from "@/lib/auth/session-provider";
import { writeConsentCookie } from "@/lib/consent";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useUnsavedChangesGuard } from "@/lib/forms/unsaved-changes";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { settingsSchema } from "@/lib/schemas/settings";
import { applyTheme, themeSlug } from "@/lib/theme";
import { useAsync } from "@/lib/use-async";
import { useMediaQuery } from "@/lib/use-media-query";
import { ErrorState } from "@/components/feedback/error-state";
import { announce } from "@/components/feedback/live-region";
import { SaveStatus, type SaveState } from "@/components/feedback/save-status";
import { useToast } from "@/components/feedback/toast-provider";
import { Button } from "@/components/ui/button";
import { FormRootError } from "@/components/ui/form";
import { Skeleton } from "@/components/ui/skeleton";
import type { SharedSettingsSections } from "@/components/psychic/settings/psychic-settings";
import { useSettingsSection } from "@/components/psychic/settings/save-registry";

import { AccountActionsCard } from "./account-actions";
import { AccountInformationCard } from "./account-information";
import { AppearanceCard } from "./appearance-card";
import { BillingCard } from "./billing/billing-card";
import { ConflictDialog } from "./conflict-dialog";
import { LogoutCard } from "./logout-card";
import { NotificationsCard } from "./notifications-card";
import { PrivacyCard } from "./privacy-card";
import { SecurityCard } from "./security-card";
import { buildSettingsPatch, settingsFieldMap, toFormValues, type SettingsFormValues } from "./settings-model";
import { SettingsSyncContext, type SettingsSync } from "./settings-sync";

function Heading({ children }: { children?: React.ReactNode }) {
  const { text } = useI18n();
  return (
    <div className="settings-header">
      <div className="settings-heading">
        <p className="settings-eyebrow">{text("Profile")}</p>
        <h1 id="settings-title" className="settings-title">
          <span className="settings-title-desktop">{text("Profile Settings")}</span>
          <span className="settings-title-mobile">{text("Account settings")}</span>
        </h1>
        <p className="settings-subtitle">{text("Manage your account, security, preferences, and billing.")}</p>
      </div>
      {children}
    </div>
  );
}

function Footer() {
  const { text, locale } = useI18n();
  return (
    <footer className="settings-footer">
      <p>{text("By using our services, you agree to our")}</p>
      <div className="settings-footer-links">
        <Link href={`/${locale}/privacy`}>{text("Privacy Policy")}</Link>
        <Link href={`/${locale}/terms`}>{text("Terms of Service")}</Link>
      </div>
    </footer>
  );
}

function LoadingSettings() {
  const { text } = useI18n();
  return (
    <div className="account-settings-page" aria-busy="true">
      <Heading />
      <p role="status" className="sr-only">
        {text("Loading your settings…")}
      </p>
      <div className="settings-grid">
        {["settings-group-left", "settings-group-right"].map((column) => (
          <div className={`settings-group ${column}`} key={column}>
            {[0, 1].map((card) => (
              <div className="settings-card" key={card}>
                <Skeleton className="h-6 w-48" />
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}

type Conflict = { patch: SettingsPatch; currentVersion: number } | null;

type ControllerProps = { initial: Settings; reloadSettings: () => Promise<Settings>; setSettings: (s: Settings) => void; guardForm?: boolean };

/**
 * The state and save logic of the shared settings form: one PATCH
 * /me/settings with the changed fields and the version they were loaded at,
 * a version-conflict dialog, server field errors mapped onto the fields.
 * Used by the user Settings page and by the advisor Settings page, which
 * hosts the same cards next to its own.
 */
function useSettingsController({ initial, reloadSettings, setSettings, guardForm = true }: ControllerProps) {
  const { text, locale } = useI18n();
  const router = useRouter();
  const pathname = usePathname();
  const toast = useToast();
  const { reload: reloadSession } = useSession();
  const form = useSchemaForm<ReturnType<typeof settingsSchema>, SettingsFormValues>(settingsSchema, { defaultValues: toFormValues(initial) });
  // The version the form's values came from (sent as expected_version).
  const baseVersion = React.useRef(initial.version);
  const [saveState, setSaveState] = React.useState<SaveState>("idle");
  const [conflict, setConflict] = React.useState<Conflict>(null);
  const [passwordDirty, setPasswordDirty] = React.useState(false);
  const [settings, setLocalSettings] = React.useState(initial);
  const { isDirty, isSubmitting } = form.formState;
  const role = settings.account.role;

  // On the advisor page the page-level guard already covers the form.
  useUnsavedChangesGuard((guardForm && isDirty) || passwordDirty, text("You have unsaved changes. Leave this page without saving?"));

  // This browser keeps a copy of the privacy choices for client analytics.
  React.useEffect(() => {
    const c = settings.privacy.consents;
    writeConsentCookie(
      { analytics: Boolean(c.analytics), marketing_attribution: Boolean(c.marketing_attribution), personalization: c.personalization !== false },
      settings.privacy.policy_version
    );
  }, [settings.privacy]);

  const state: SaveState =
    isSubmitting || saveState === "saving" ? "saving" : isDirty ? (saveState === "error" ? "error" : "dirty") : saveState === "saved" ? "saved" : "idle";

  function adopt(next: Settings) {
    baseVersion.current = next.version;
    setLocalSettings(next);
    setSettings(next);
    form.reset(toFormValues(next));
  }

  async function send(patch: SettingsPatch): Promise<boolean> {
    setSaveState("saving");
    try {
      const saved = await settingsApi.update(patch);
      adopt(saved);
      setSaveState("saved");
      announce(text("All changes saved"));
      if (patch.appearance?.theme) applyTheme(themeSlug(saved.appearance.theme));
      void reloadSession();
      const nextLocale = patch.appearance?.locale as Locale | undefined;
      if (nextLocale && nextLocale !== locale) {
        document.cookie = `STARYIELD_LOCALE=${nextLocale}; path=/; max-age=31536000; samesite=lax`;
        router.replace(localePath(`${pathname}${window.location.hash}`, nextLocale), { scroll: false });
      }
      return true;
    } catch (err) {
      const e = toApiError(err);
      if (e.code === "version_conflict") {
        const current = Number(e.details?.current_version);
        setConflict({ patch, currentVersion: Number.isFinite(current) ? current : baseVersion.current });
        setSaveState("error");
        return false;
      }
      applyApiError(form, err, { text, fieldMap: settingsFieldMap });
      setSaveState("error");
      if (e.code !== "validation_failed") toast.showError(e);
      return false;
    }
  }

  async function save(values: SettingsFormValues): Promise<boolean> {
    const patch = buildSettingsPatch(values, form.formState.dirtyFields, baseVersion.current);
    if (!patch) {
      setSaveState("saved");
      return true;
    }
    return send(patch);
  }

  /** Validate and save; `focus: false` leaves the focus where it is. */
  async function saveNow({ focus }: { focus: boolean }): Promise<boolean> {
    if (!focus && !(await form.trigger(undefined, { shouldFocus: false }))) {
      setSaveState("error");
      return false;
    }
    let ok = false;
    await form.handleSubmit(
      async (values) => {
        ok = await save(values);
      },
      () => {
        setSaveState("error");
      }
    )();
    return ok;
  }

  const submit = () => void form.handleSubmit(save)();

  async function overwrite() {
    if (!conflict) return;
    const values = form.getValues();
    const patch = buildSettingsPatch(values, form.formState.dirtyFields, conflict.currentVersion) ?? { expected_version: conflict.currentVersion };
    if (await send(patch)) setConflict(null);
  }

  async function reloadLatest() {
    try {
      adopt(await reloadSettings());
      setConflict(null);
      setSaveState("idle");
      toast.show({ tone: "info", message: text("Settings reloaded.") });
    } catch (err) {
      toast.showError(err);
    }
  }

  // A switch that saved itself (push, §23.6): follow the server's version and
  // settings without touching the user's other unsaved edits.
  const sync = React.useMemo<SettingsSync>(
    () => ({
      adopt: (saved) => {
        baseVersion.current = saved.version;
        setLocalSettings((s) => ({ ...s, version: saved.version, notifications: { ...s.notifications, push: saved.notifications.push } }));
        setSettings(saved);
      },
    }),
    [setSettings],
  );

  async function refreshAfterEmailRequest() {
    // Show the pending address without touching unsaved edits.
    try {
      const fresh = await settingsApi.get();
      setLocalSettings((s) => ({ ...s, account: fresh.account }));
    } catch {
      // The dialog already confirmed the request; the note appears on reload.
    }
  }

  // Enter in a profile text field saves (the password form has its own submit).
  function onKeyDown(event: React.KeyboardEvent<HTMLDivElement>) {
    if (event.key !== "Enter" || event.defaultPrevented || event.nativeEvent.isComposing) return;
    const target = event.target as HTMLElement;
    if (!(target instanceof HTMLInputElement) || target.form || target.readOnly) return;
    if (["checkbox", "radio", "button", "submit", "file"].includes(target.type)) return;
    if (target.getAttribute("role") === "combobox") return;
    event.preventDefault();
    submit();
  }

  const conflictDialog = (
    <ConflictDialog
      open={conflict !== null}
      busy={isSubmitting || saveState === "saving"}
      onReload={() => void reloadLatest()}
      onOverwrite={() => void overwrite()}
      onCancel={() => setConflict(null)}
    />
  );

  return { form, settings, role, state, isDirty, isSubmitting, setPasswordDirty, submit, saveNow, onKeyDown, refreshAfterEmailRequest, conflictDialog, sync };
}

function SettingsForm(props: ControllerProps) {
  const { text, locale } = useI18n();
  const mobile = useMediaQuery("(max-width: 767px)");
  const { form, settings, role, state, isDirty, isSubmitting, setPasswordDirty, submit, onKeyDown, refreshAfterEmailRequest, conflictDialog, sync } =
    useSettingsController(props);

  const saveButton = (
    <Button
      type="button"
      className="settings-save"
      onClick={submit}
      onMouseDown={keepFocusOnPress}
      disabled={isSubmitting}
      aria-busy={isSubmitting}
      data-testid="settings-save"
    >
      {text(isSubmitting ? "Saving…" : "Save Changes")}
    </Button>
  );

  return (
    <FormProvider {...form}><SettingsSyncContext.Provider value={sync}>
      <div className="account-settings-page" role="form" aria-labelledby="settings-title" onKeyDown={onKeyDown} data-testid="account-settings">
        <Heading>
          <div className="settings-header-actions">
            <SaveStatus state={state} className="settings-save-status" />
            <Button asChild className="settings-back">
              <Link href={`/${locale}/profile`}>{text("Back to Dashboard")}</Link>
            </Button>
            {mobile ? null : saveButton}
          </div>
        </Heading>
        <FormRootError className="settings-root-error" />
        {/* DOM order = the mobile frame's order; on desktop the account
            actions sit under the left column (Figma 384:1498). */}
        <div className="settings-grid">
          <div className="settings-group settings-group-left">
            <AccountInformationCard settings={settings} onEmailChangeRequested={() => void refreshAfterEmailRequest()} />
            <SecurityCard email={settings.account.email} onDirtyChange={setPasswordDirty} />
            <PrivacyCard showShareSwitch={role === "client"} />
          </div>
          <div className="settings-group settings-group-right">
            <NotificationsCard />
            {role === "client" ? <BillingCard timeZone={settings.appearance.time_zone || undefined} /> : null}
            <AppearanceCard />
            <LogoutCard />
          </div>
          <div className="settings-group settings-group-actions">
            <AccountActionsCard />
          </div>
        </div>
        <Footer />
        {mobile && (isDirty || isSubmitting || state === "error") ? (
          <div className="settings-save-bar" role="region" aria-label={text("Unsaved changes")}>
            <SaveStatus state={state} />
            {saveButton}
          </div>
        ) : null}
        {conflictDialog}
      </div>
    </SettingsSyncContext.Provider></FormProvider>
  );
}

/**
 * The user Account Settings page (Figma 384:1498 desktop, 867:17015
 * mobile). The editable profile fields share one react-hook-form form:
 * "Save Changes" sends one PATCH /me/settings with only the changed fields
 * and the version they were loaded at; dialogs and the password form save
 * on their own.
 */
export function AccountSettings() {
  const { user } = useSession();
  const loaded = useAsync((signal) => settingsApi.get(signal), [user?.id], Boolean(user));

  if (loaded.error && !loaded.data) {
    return (
      <div className="account-settings-page">
        <Heading />
        <ErrorState error={loaded.error} onRetry={loaded.reload} />
      </div>
    );
  }
  if (!loaded.data) return <LoadingSettings />;
  return (
    <SettingsForm
      key={user?.id}
      initial={loaded.data}
      setSettings={(s) => loaded.setData(s)}
      reloadSettings={async () => {
        const fresh = await settingsApi.get();
        loaded.setData(fresh);
        return fresh;
      }}
    />
  );
}

/* ---------------------------------------------------------------------------
   Advisor Settings (Figma 1219:1284) hosts the same account sections next to
   the advisor cards. `SharedAccountSettings` loads /me/settings, wraps the
   advisor page in this form's context, and joins the page's single
   "Save Changes" through the advisor save registry.
   --------------------------------------------------------------------------- */

export type SharedSectionNodes = Required<SharedSettingsSections>;

function SharedSettingsForm({ order, children, ...props }: ControllerProps & { order: number; children: (shared: SharedSectionNodes) => React.ReactNode }) {
  const { form, settings, isDirty, setPasswordDirty, saveNow, refreshAfterEmailRequest, conflictDialog, sync } = useSettingsController({ ...props, guardForm: false });
  useSettingsSection("account-settings", order, isDirty, saveNow);
  const shared: SharedSectionNodes = {
    account: (
      <>
        <FormRootError className="settings-root-error" />
        <AccountInformationCard settings={settings} onEmailChangeRequested={() => void refreshAfterEmailRequest()} />
      </>
    ),
    security: <SecurityCard email={settings.account.email} onDirtyChange={setPasswordDirty} />,
    privacy: <PrivacyCard showShareSwitch={settings.account.role === "client"} />,
    notifications: <NotificationsCard />,
    appearance: <AppearanceCard />,
    session: (
      <>
        <LogoutCard />
        <AccountActionsCard />
      </>
    ),
  };
  return (
    <FormProvider {...form}>
      <SettingsSyncContext.Provider value={sync}>
        {children(shared)}
        {conflictDialog}
      </SettingsSyncContext.Provider>
    </FormProvider>
  );
}

/** The account sections for a page that has its own save registry (advisor Settings). */
export function SharedAccountSettings({ children, order = 0 }: { children: (shared: SharedSectionNodes) => React.ReactNode; order?: number }) {
  const { user } = useSession();
  const loaded = useAsync((signal) => settingsApi.get(signal), [user?.id], Boolean(user));

  if (loaded.error && !loaded.data) {
    return children({
      account: <ErrorState error={loaded.error} onRetry={loaded.reload} />,
      security: <></>,
      privacy: <></>,
      notifications: <></>,
      appearance: <></>,
      session: <LogoutCard />,
    });
  }
  // The whole page waits for /me/settings, so the advisor cards mount (and
  // fetch) once instead of remounting when the shared form appears.
  if (!loaded.data) return <LoadingSettings />;
  return (
    <SharedSettingsForm
      key={user?.id}
      order={order}
      initial={loaded.data}
      setSettings={(s) => loaded.setData(s)}
      reloadSettings={async () => {
        const fresh = await settingsApi.get();
        loaded.setData(fresh);
        return fresh;
      }}
    >
      {children}
    </SharedSettingsForm>
  );
}
