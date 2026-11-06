"use client";

import * as React from "react";
import * as yup from "yup";
import { ShieldCheck } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { consentsApi, type ConsentCategory, type Consents } from "@/lib/api/account";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { writeConsentCookie } from "@/lib/consent";
import { useAsync } from "@/lib/use-async";
import { AsyncContent } from "@/components/feedback/async-content";
import { useToast } from "@/components/feedback/toast-provider";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Form, FormControl, FormField, FormItem, FormRootError } from "@/components/ui/form";
import { Switch } from "@/components/ui/switch";

const CATEGORIES: { id: ConsentCategory; label: string; description: string }[] = [
  {
    id: "analytics",
    label: "Analytics",
    description: "Anonymous statistics about how pages are used, so we can improve Staryield.",
  },
  {
    id: "marketing_attribution",
    label: "Marketing attribution",
    description: "Remember which campaign or link brought you to Staryield.",
  },
  {
    id: "personalization",
    label: "Personalization",
    description: "Tailor readings to you, and let your advisors share their notes about you with your other advisors.",
  },
];

// The dialog's switches; categories mirror GET /consent-policy.
const consentSchema = () =>
  yup.object({
    analytics: yup.boolean().default(false),
    marketing_attribution: yup.boolean().default(false),
    personalization: yup.boolean().default(true),
  });

type ConsentValues = yup.InferType<ReturnType<typeof consentSchema>>;

function valuesOf(c: Consents): ConsentValues {
  const granted = (id: ConsentCategory, fallback: boolean) => c.items.find((item) => item.category === id)?.granted ?? fallback;
  return {
    analytics: granted("analytics", false),
    marketing_attribution: granted("marketing_attribution", false),
    personalization: granted("personalization", true),
  };
}

function gpcEnabled(): boolean {
  if (typeof navigator === "undefined") return false;
  return (navigator as Navigator & { globalPrivacyControl?: boolean }).globalPrivacyControl === true || navigator.doNotTrack === "1";
}

function ConsentForm({ consents, onSaved }: { consents: Consents; onSaved: (c: Consents) => void }) {
  const { text } = useI18n();
  const form = useSchemaForm(consentSchema, { defaultValues: valuesOf(consents) });
  const [gpc] = React.useState(gpcEnabled);
  const essentialId = React.useId();

  async function submit(values: ConsentValues) {
    try {
      const saved = await consentsApi.update(values);
      writeConsentCookie(valuesOf(saved), saved.policy_version);
      onSaved(saved);
    } catch (err) {
      applyApiError(form, err, { text });
    }
  }

  return (
    <Form {...form}>
      <form
        noValidate
        className="settings-dialog-form"
        onSubmit={(event) => {
          event.stopPropagation();
          void form.handleSubmit(submit)(event);
        }}
      >
        <FormRootError />
        {consents.outdated ? <p className="settings-note" role="status">{text("Our privacy notice changed. Please review your choices.")}</p> : null}
        {gpc ? (
          <p className="settings-note">
            {text("Your browser sends a Global Privacy Control signal, so analytics and attribution stay off unless you turn them on here.")}
          </p>
        ) : null}
        <ul className="settings-consents">
          <li className="settings-consent">
            <div>
              <p className="settings-consent-label" id={essentialId}>
                {text("Essential")}
              </p>
              <p className="settings-consent-description">{text("Sign-in, security and language. Always on: the site cannot work without them.")}</p>
            </div>
            <Switch checked disabled aria-labelledby={essentialId} />
          </li>
          {CATEGORIES.map((category) => (
            <FormField
              key={category.id}
              control={form.control}
              name={category.id}
              render={({ field }) => (
                <FormItem className="settings-consent">
                  <div>
                    <p className="settings-consent-label" id={`consent-${category.id}`}>
                      {text(category.label)}
                    </p>
                    <p className="settings-consent-description" id={`consent-${category.id}-description`}>
                      {text(category.description)}
                    </p>
                  </div>
                  <FormControl>
                    <Switch
                      ref={field.ref}
                      name={field.name}
                      checked={field.value}
                      onCheckedChange={field.onChange}
                      onBlur={field.onBlur}
                      aria-labelledby={`consent-${category.id}`}
                      aria-describedby={`consent-${category.id}-description`}
                      data-testid={`consent-${category.id}`}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          ))}
        </ul>
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
            data-testid="consent-save"
          >
            {text(form.formState.isSubmitting ? "Saving…" : "Save preferences")}
          </Button>
        </DialogFooter>
      </form>
    </Form>
  );
}

/** Privacy Preferences: the consent categories and their legal basis. */
export function ConsentDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { text } = useI18n();
  const toast = useToast();
  const consents = useAsync((signal) => consentsApi.get(signal), [open], open);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="md" className="settings-dialog" data-testid="consent-dialog">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <ShieldCheck aria-hidden className="size-6 shrink-0" />
            {text("Privacy preferences")}
          </DialogTitle>
          <DialogDescription>{text("Choose how we may use your data. You can change these choices at any time.")}</DialogDescription>
        </DialogHeader>
        <AsyncContent state={consents}>
          {(data) => (
            <ConsentForm
              consents={data}
              onSaved={(saved) => {
                consents.setData(saved);
                toast.show({ tone: "success", message: text("Privacy preferences saved.") });
                onOpenChange(false);
              }}
            />
          )}
        </AsyncContent>
      </DialogContent>
    </Dialog>
  );
}
