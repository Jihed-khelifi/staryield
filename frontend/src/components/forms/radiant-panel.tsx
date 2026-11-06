"use client";
import { useI18n } from "@/i18n/i18n-provider";
/* eslint-disable @next/next/no-img-element */
import { SunLogo } from "@/components/brand/sun-logo";
import { LocaleSwitcher } from "@/components/layout/locale-switcher";

const RAYS = "https://assets.staryield.net/assets/brand/auth-rays-92bd14a495.svg";

export function RadiantPanel({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  const { text } = useI18n();
  return (
    <section className={`radiant-panel paper-page ${className}`}>
      {className.includes("auth-panel") && (
        <LocaleSwitcher className="absolute right-4 top-4 z-10 rounded-full border border-gold/40 bg-cream/90 md:hidden" />
      )}
      <div className="radiant-brand">
        {/* Revisions 2.0: one clean ray set, centred on the sun (Figma 6a3de). */}
        <div aria-hidden="true" className="radiant-rays">
          <img src={RAYS} alt="" />
        </div>
        <SunLogo large />
        <p className="radiant-wordmark font-display">{text("Staryield ")}</p>
      </div>
      {children}
    </section>
  );
}
