"use client";
/* eslint-disable @next/next/no-img-element */

import * as React from "react";
import type { ControllerRenderProps, FieldPath, FieldValues } from "react-hook-form";
import { CreditCard, Palette, Shield } from "lucide-react";

import { cn } from "@/lib/utils";
import { FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage, FormValidMark, useFieldValidity } from "@/components/ui/form";

/** Figma 384:1498 icons that are not in the icon library (exported to R2). */
const R2_ICONS = {
  gear: "https://assets.staryield.net/assets/settings/gear-9df80e4343.svg",
  bell: "https://assets.staryield.net/assets/settings/notification-outline-70df938e98.svg",
  logout: "https://assets.staryield.net/assets/settings/log-out-2190dc8cb1.svg",
} as const;

export type CardIconName = "gear" | "shield" | "bell" | "card" | "palette" | "logout";

/** The 24 px card-title icon (decorative). */
export function CardIcon({ name }: { name: CardIconName }) {
  if (name === "shield") return <Shield aria-hidden className="settings-card-icon" size={24} absoluteStrokeWidth strokeWidth={2} />;
  if (name === "card") return <CreditCard aria-hidden className="settings-card-icon" size={24} absoluteStrokeWidth strokeWidth={2} />;
  if (name === "palette") return <Palette aria-hidden className="settings-card-icon" size={24} absoluteStrokeWidth strokeWidth={2} />;
  return (
    <span aria-hidden className="settings-card-icon" data-icon={name}>
      <img src={R2_ICONS[name]} alt="" width={name === "gear" ? 22 : 24} height={name === "gear" ? 22 : 24} />
    </span>
  );
}

/** A settings card: Castellar title with an optional icon. */
export function SettingsCard({
  id,
  icon,
  title,
  children,
  className,
  testId,
}: {
  id: string;
  icon?: CardIconName;
  title: string;
  children: React.ReactNode;
  className?: string;
  testId?: string;
}) {
  const titleId = `${id}-title`;
  return (
    <section id={id} aria-labelledby={titleId} className={cn("settings-card", className)} data-testid={testId}>
      <div className="settings-card-header">
        {icon ? <CardIcon name={icon} /> : null}
        <h2 id={titleId} className="settings-card-title">
          {title}
        </h2>
      </div>
      {children}
    </section>
  );
}

/** A full-width row button ("Download Your Data — Export a copy"). */
export function SettingsRowButton({
  icon,
  label,
  hint,
  onClick,
  tone,
  testId,
  disabled,
}: {
  icon: React.ReactNode;
  label: string;
  hint?: string;
  onClick: () => void;
  tone?: "danger";
  testId?: string;
  disabled?: boolean;
}) {
  return (
    <button type="button" className="settings-row settings-row-button" data-tone={tone} onClick={onClick} data-testid={testId} disabled={disabled}>
      <span className="settings-row-main">
        <span aria-hidden className="settings-row-icon">
          {icon}
        </span>
        <span className="settings-row-label">{label}</span>
      </span>
      {hint ? <span className="settings-row-hint">{hint}</span> : null}
    </button>
  );
}

function FieldValid() {
  const { valid } = useFieldValidity(true);
  return valid ? <FormValidMark /> : null;
}

/**
 * A labelled settings field bound to react-hook-form: 13 px label, the
 * control, an optional description, the error (announced) and the valid
 * mark. An empty message takes no space, so fields keep Figma's 16 px rhythm.
 */
export function SettingsField<TValues extends FieldValues, TName extends FieldPath<TValues>>({
  name,
  label,
  description,
  className,
  showValid = true,
  children,
}: {
  name: TName;
  label: React.ReactNode;
  description?: React.ReactNode;
  className?: string;
  showValid?: boolean;
  children: (field: ControllerRenderProps<TValues, TName>) => React.ReactElement;
}) {
  return (
    <FormField<TValues, TName>
      name={name}
      render={({ field }) => (
        <FormItem showValid={showValid} className={cn("settings-field", className)}>
          <FormLabel className="settings-label">{label}</FormLabel>
          <FormControl>{children(field)}</FormControl>
          {description ? <FormDescription className="settings-description">{description}</FormDescription> : null}
          <FormMessage />
          {showValid ? <FieldValid /> : null}
        </FormItem>
      )}
    />
  );
}

/** Sub-heading inside a card ("Logged In Devices"). */
export function SettingsSubheading({ id, children }: { id?: string; children: React.ReactNode }) {
  return (
    <h3 id={id} className="settings-subheading">
      {children}
    </h3>
  );
}
