"use client";

import { useI18n } from "@/i18n/i18n-provider";
import { Separator } from "@/components/ui/separator";

/** "──── or ────" between the e-mail form and the Google button. */
export function AuthDivider() {
  const { text } = useI18n();
  return (
    <div className="mb-4 flex items-center gap-3 text-sm text-cream-light" data-testid="auth-divider">
      <Separator className="flex-1 bg-cream-light/50" />
      <span>{text("or")}</span>
      <Separator className="flex-1 bg-cream-light/50" />
    </div>
  );
}
