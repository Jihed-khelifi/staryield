"use client";
import Link from "next/link";
import type { ComponentProps } from "react";
import { useI18n } from "@/i18n/i18n-provider";
import { usePathname } from "next/navigation";

export function headingAnchor(label: string) {
  return label
    .replace(/^\d+\.\s*/, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

export function contentDestination(label: string) {
  const text = label.toLowerCase();
  const routes: [RegExp, string][] = [
    [/privacy/, "/privacy"],
    [/terms/, "/terms"],
    [/disclaimer/, "/disclaimer"],
    [/contact/, "/contact"],
    [/faq|questions/, "/faq"],
    [/integrity|standards/, "/standards"],
    [/reviews/, "/reviews"],
    [/about us/, "/about"],
    [/natal/, "/calculators/natal-chart"],
    [/path of life/, "/calculators/path-of-life"],
    [/astrolog|calculation/, "/calculators/astrology"],
    [/log in/, "/login"],
    [/sign|start today|get started|intro|continue/, "/signup"],
    [/chat/, "/chatroom"],
  ];
  return routes.find(([pattern]) => pattern.test(text))?.[1] ?? "/psychics";
}
export function ContentLink({
  label,
  children,
  href: suppliedHref,
  ...props
}: ComponentProps<"a"> & { label: string }) {
  const { locale } = useI18n();
  const pathname = usePathname();
  const legalSection =
    /\/(terms|privacy)$/.test(pathname) &&
    label === label.toUpperCase() &&
    !label.includes("INTRO");
  const reader =
    props["data-node-id" as keyof typeof props] === "461:1716"
      ? "theo"
      : "ramone";
  const support = /^(support|here)$/i.test(label);
  const href = legalSection
    ? `#${headingAnchor(label)}`
    : suppliedHref?.startsWith("mailto:") || suppliedHref?.startsWith("tel:")
      ? suppliedHref
      : `/${locale}${support ? "/contact" : contentDestination(label) + (/start 1-on-1 chat/i.test(label) ? `?reader=${reader}` : "")}`;
  if (
    String(props["data-name" as keyof typeof props] ?? "").startsWith(
      "Instance / State=Collapsed",
    )
  ) {
    return (
      <details className={`faq-item ${props.className ?? ""}`}>
        <summary>{children}</summary>
        <p className="mt-4 text-sm font-serif normal-case tracking-normal">
          For help with this question,{" "}
          <Link href={`/${locale}/contact`} className="underline">
            contact our support team
          </Link>
          .
        </p>
      </details>
    );
  }
  return (
    <Link {...props} href={href}>
      {children}
    </Link>
  );
}
