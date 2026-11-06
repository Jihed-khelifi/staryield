"use client";

import * as React from "react";
import { useWatch, type Control } from "react-hook-form";

import { useI18n } from "@/i18n/i18n-provider";
import type { NewPasswordValues } from "@/lib/schemas/auth";
import { FieldRow } from "@/components/ui/field";
import { PasswordInput } from "@/components/ui/password-input";

import { PasswordChecklist } from "./auth-card";

/** "New password" with its live rule checklist, then "Confirm password". */
export function NewPasswordFields({ control }: { control: Control<NewPasswordValues> }) {
  const { text } = useI18n();
  const password = useWatch({ control, name: "password" }) ?? "";
  const checklistId = React.useId();
  return (
    <div className="mt-6 grid gap-5">
      <FieldRow control={control} name="password" label={text("New password")} labelClassName="auth-label">
        {(field) => (
          <PasswordInput
            {...field}
            autoComplete="new-password"
            placeholder={text("Enter your password")}
            maxLength={128}
            aria-describedby={checklistId}
            className="auth-input"
          />
        )}
      </FieldRow>
      <PasswordChecklist password={password} id={checklistId} />
      <FieldRow control={control} name="confirm" label={text("Confirm password")} labelClassName="auth-label">
        {(field) => (
          <PasswordInput
            {...field}
            autoComplete="new-password"
            placeholder={text("Enter your password")}
            maxLength={128}
            className="auth-input"
          />
        )}
      </FieldRow>
    </div>
  );
}
