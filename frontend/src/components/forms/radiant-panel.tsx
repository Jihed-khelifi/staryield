/* eslint-disable @next/next/no-img-element */
import { SunLogo } from "@/components/brand/sun-logo";

export function RadiantPanel({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section className={`radiant-panel paper-page ${className}`}>
      <div aria-hidden="true" className="radiant-rays">
        <img src="https://assets.staryield.net/assets/figma/c8a2b.svg" alt="" />
        <img src="https://assets.staryield.net/assets/figma/70ebb.svg" alt="" />
      </div>
      <div className="radiant-brand">
        <SunLogo large />
        <p className="font-display text-[40px] tracking-wide sm:text-5xl">
          Staryield
        </p>
      </div>
      {children}
    </section>
  );
}
