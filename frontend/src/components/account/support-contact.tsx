"use client";

import { COMPANY, supportMailto } from "@/config/company";
import { useI18n } from "@/i18n/i18n-provider";

/** The support hours and reply time, in one sentence (docs/platform-extensions.md §25.4). */
export function SupportHours({ className }: { className?: string }) {
  const { text } = useI18n();
  return <span className={className}>{text("Support is available 24/7. We reply within 1–2 days.")}</span>;
}

/** The support e-mail as a `mailto:` link (a subject is optional). */
export function SupportEmailLink({ subject, className }: { subject?: string; className?: string }) {
  return (
    <a href={supportMailto(subject)} className={className ?? "underline"} data-testid="support-email">
      {COMPANY.supportEmail}
    </a>
  );
}

/** "Questions? support@staryield.net · Support is available 24/7. We reply within 1–2 days." */
export function SupportLine({ className }: { className?: string }) {
  const { text } = useI18n();
  return (
    <p className={className} data-testid="support-line">
      {text("Questions?")} <SupportEmailLink /> · <SupportHours />
    </p>
  );
}
