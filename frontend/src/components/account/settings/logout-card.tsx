"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { useSession } from "@/lib/auth/session-provider";
import { Button } from "@/components/ui/button";

import { SettingsCard } from "./settings-ui";

/** Logout (Figma "Card / Logout"). */
export function LogoutCard() {
  const { text } = useI18n();
  const { signOut } = useSession();
  const [busy, setBusy] = React.useState(false);
  return (
    <SettingsCard id="logout" icon="logout" title={text("Logout")} className="settings-card-compact" testId="settings-logout">
      <p className="settings-muted-text">{text("You are currently signed in on this device. Sign out to protect your account.")}</p>
      <div>
        <Button
          type="button"
          className="settings-button"
          disabled={busy}
          aria-busy={busy}
          onClick={() => {
            setBusy(true);
            void signOut();
          }}
          data-testid="settings-sign-out"
        >
          {text("Sign Out")}
        </Button>
      </div>
    </SettingsCard>
  );
}
