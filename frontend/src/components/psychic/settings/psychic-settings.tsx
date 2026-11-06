"use client"

import * as React from "react"
import Link from "next/link"
import { Bell, Loader2, LockKeyhole, LogOut, Palette, Settings2, Shield } from "lucide-react"

import { useI18n } from "@/i18n/i18n-provider"
import { keepFocusOnPress } from "@/lib/forms/submit-press"
import { useUnsavedChangesGuard } from "@/lib/forms/unsaved-changes"
import { announce } from "@/components/feedback/live-region"
import { SaveStatus } from "@/components/feedback/save-status"
import { useToast } from "@/components/feedback/toast-provider"
import { Button } from "@/components/ui/button"
import { AdvisorProfileCard } from "@/components/psychic/settings/advisor-profile-card"
import { CustomsCard } from "@/components/psychic/settings/customs-card"
import { SettingsSaveProvider, useSettingsSave } from "@/components/psychic/settings/save-registry"
import { SettingsCard } from "@/components/psychic/settings/settings-card"

/**
 * ===========================================================================
 * SHARED SETTINGS SECTIONS SLOT
 * ---------------------------------------------------------------------------
 * The sections every account has (built by the user-settings work under
 * components/account/settings/) are passed in here; this page only owns the
 * advisor cards. Until they are plugged in, each slot shows a minimal
 * placeholder card (data-testid="settings-slot-<name>"). A shared section
 * can join the page's "Save Changes" button with `useSettingsSection` from
 * ./save-registry.
 * ===========================================================================
 */
export type SharedSettingsSections = {
  /** Account information: full name, e-mail address. */
  account?: React.ReactNode
  /** Password & security, logged-in devices. */
  security?: React.ReactNode
  /** Data & privacy: data export, privacy preferences. */
  privacy?: React.ReactNode
  /** Notification switches. */
  notifications?: React.ReactNode
  /** Theme, language, time zone. */
  appearance?: React.ReactNode
  /** Sign out (and account actions). */
  session?: React.ReactNode
}

type SlotName = keyof SharedSettingsSections

const SLOTS: Record<SlotName, { title: string; icon: React.ReactNode }> = {
  account: { title: "Account Information", icon: <Settings2 /> },
  security: { title: "Password & Security", icon: <Shield /> },
  privacy: { title: "Data & Privacy", icon: <LockKeyhole /> },
  notifications: { title: "Notifications", icon: <Bell /> },
  appearance: { title: "Appearance", icon: <Palette /> },
  session: { title: "Logout", icon: <LogOut /> },
}

/** Placeholder for a shared section that has not been plugged in yet. */
function SharedSlot({ name, content }: { name: SlotName; content?: React.ReactNode }) {
  const { text } = useI18n()
  if (content) return <>{content}</>
  const slot = SLOTS[name]
  return (
    <SettingsCard title={text(slot.title)} icon={slot.icon} data-testid={`settings-slot-${name}`} className="advisor-settings-slot">
      <p className="advisor-card-intro">{text("This section will be available here soon.")}</p>
    </SettingsCard>
  )
}

function SettingsPage({ shared }: { shared: SharedSettingsSections }) {
  const { text, locale } = useI18n()
  const toast = useToast()
  const { dirty, state, saveAll } = useSettingsSave()
  const saving = state === "saving"
  useUnsavedChangesGuard(dirty, text("You have unsaved changes. Leave this page without saving?"))

  async function save() {
    const ok = await saveAll()
    if (ok) {
      toast.show({ tone: "success", message: text("Your changes were saved.") })
    } else {
      announce(text("Some changes were not saved. Check the highlighted fields."), "assertive")
    }
  }

  const saveButton = (className: string, testId: string) => (
    <Button
      type="button"
      variant="cta"
      size="lg"
      className={className}
      onMouseDown={keepFocusOnPress}
      onClick={() => void save()}
      disabled={!dirty || saving}
      aria-busy={saving || undefined}
      data-testid={testId}
    >
      {saving ? <Loader2 aria-hidden className="size-4 animate-spin motion-reduce:animate-none" /> : null}
      {text(saving ? "Saving…" : "Save Changes")}
    </Button>
  )

  return (
    <div className="advisor-settings" data-testid="psychic-settings">
      <header className="advisor-settings-head">
        <div className="min-w-0">
          <p className="eyebrow advisor-settings-eyebrow">{text("Profile")}</p>
          <h1 className="advisor-title">{text("Profile Settings")}</h1>
          <p className="advisor-settings-subtitle">{text("Manage your account, security, preferences, and billing.")}</p>
        </div>
        <div className="advisor-settings-actions">
          <SaveStatus state={state} className="advisor-save-status" />
          <Button asChild variant="outline" size="lg" className="advisor-back-button">
            <Link href={`/${locale}/profile`}>{text("Back to Dashboard")}</Link>
          </Button>
          {saveButton("advisor-save-button", "settings-save")}
        </div>
      </header>

      <div className="advisor-settings-grid">
        <div className="advisor-settings-column">
          <SharedSlot name="account" content={shared.account} />
          <AdvisorProfileCard />
          <SharedSlot name="security" content={shared.security} />
          <SharedSlot name="privacy" content={shared.privacy} />
        </div>
        <div className="advisor-settings-column">
          <SharedSlot name="notifications" content={shared.notifications} />
          <CustomsCard />
          <SharedSlot name="appearance" content={shared.appearance} />
          <SharedSlot name="session" content={shared.session} />
        </div>
      </div>

      <footer className="advisor-settings-footer">
        <p>{text("By using our services, you agree to our")}</p>
        <p className="flex justify-center gap-4">
          <Link href={`/${locale}/privacy`}>{text("Privacy Policy")}</Link>
          <Link href={`/${locale}/terms`}>{text("Terms of Service")}</Link>
        </p>
      </footer>

      {/* Phones: the header button is out of reach, so unsaved changes get a bar above the tab bar. */}
      {dirty || saving ? (
        <div className="advisor-savebar" role="region" aria-label={text("Unsaved changes")} data-testid="settings-savebar">
          <SaveStatus state={state} />
          {saveButton("advisor-savebar-button", "settings-save-bar")}
        </div>
      ) : null}
    </div>
  )
}

/**
 * Advisor Settings (Figma 1219:1284; phones reflow to one column). The
 * advisor cards are the profile (phone, About, Modalities, photo) and
 * Customs; the sections every account shares are passed through `shared`.
 */
export function PsychicSettings({
  shared = {},
  SharedProvider,
}: {
  shared?: SharedSettingsSections
  /** Supplies the shared sections from inside the page's save registry (so they can join "Save Changes"). */
  SharedProvider?: React.ComponentType<{ children: (shared: SharedSettingsSections) => React.ReactNode }>
}) {
  return (
    <SettingsSaveProvider>
      {SharedProvider ? <SharedProvider>{(sections) => <SettingsPage shared={sections} />}</SharedProvider> : <SettingsPage shared={shared} />}
    </SettingsSaveProvider>
  )
}
