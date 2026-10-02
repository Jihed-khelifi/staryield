"use client";
import { useI18n } from "@/i18n/i18n-provider";
/* eslint-disable @next/next/no-img-element */
import { SunLogo } from "@/components/brand/sun-logo";
import { LocaleSwitcher } from "@/components/layout/locale-switcher";

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
      <div aria-hidden="true" className="radiant-rays">
        <img src="https://assets.staryield.net/assets/figma/c8a2b.svg" alt="" />
        <img src="https://assets.staryield.net/assets/figma/70ebb.svg" alt="" />
      </div>
      <div className="radiant-brand">
        <SunLogo large />
        <p className="font-display text-[40px] tracking-wide sm:text-5xl">
          {text("Staryield ")}
        </p>
      </div>
      {children}
    </section>
  );
}
