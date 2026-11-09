"use client";

import * as React from "react";
import { useFormContext } from "react-hook-form";
import { Lock, Trash } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { FormControl, FormField, FormItem } from "@/components/ui/form";
import { Switch } from "@/components/ui/switch";

import { ConsentDialog } from "./consent-dialog";
import { ExportDialog } from "./export-dialog";
import type { SettingsFormValues } from "./settings-model";
import { SettingsCard, SettingsRowButton } from "./settings-ui";

/**
 * Data & Privacy (Figma "Card / Data & Privacy"): data export, consent
 * choices and — for clients — whether advisors may see the birth city
 * (off by default, part of the settings form). The date of birth is always
 * shown to the advisors a client chats with (§22.6).
 */
export function PrivacyCard({ showShareSwitch }: { showShareSwitch: boolean }) {
  const { text } = useI18n();
  const form = useFormContext<SettingsFormValues>();
  const [exportOpen, setExportOpen] = React.useState(false);
  const [consentOpen, setConsentOpen] = React.useState(false);
  const shareId = React.useId();

  return (
    <SettingsCard id="privacy" title={text("Data & Privacy")} className="settings-card-compact" testId="settings-privacy">
      <div className="settings-rows">
        <SettingsRowButton
          icon={<Lock size={20} absoluteStrokeWidth strokeWidth={2} />}
          label={text("Download Your Data")}
          hint={text("Export a copy")}
          onClick={() => setExportOpen(true)}
          testId="open-export"
        />
        <SettingsRowButton
          icon={<Trash size={20} absoluteStrokeWidth strokeWidth={2} />}
          label={text("Privacy Preferences")}
          hint={text("Manage consent")}
          onClick={() => setConsentOpen(true)}
          testId="open-consent"
        />
      </div>
      {showShareSwitch ? (
        <FormField
          control={form.control}
          name="privacy.share_birth_details_with_advisors"
          render={({ field }) => (
            <FormItem className="settings-switch-row settings-switch-row-described">
              <div>
                <p id={shareId} className="settings-switch-label">
                  {text("Share my birth city with advisors")}
                </p>
                <p id={`${shareId}-description`} className="settings-description">
                  {text("Advisors you chat with always see your date of birth, which readings rely on. Your birth time and exact location are never shown.")}
                </p>
              </div>
              <FormControl>
                <Switch
                  ref={field.ref}
                  name={field.name}
                  checked={field.value}
                  onCheckedChange={field.onChange}
                  onBlur={field.onBlur}
                  aria-labelledby={shareId}
                  aria-describedby={`${shareId}-description`}
                  data-testid="share-birth-details"
                />
              </FormControl>
            </FormItem>
          )}
        />
      ) : null}
      <ExportDialog open={exportOpen} onOpenChange={setExportOpen} />
      <ConsentDialog open={consentOpen} onOpenChange={setConsentOpen} />
    </SettingsCard>
  );
}
