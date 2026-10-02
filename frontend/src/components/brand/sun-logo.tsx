/* eslint-disable @next/next/no-img-element */
import { cn } from "@/lib/utils";
export function SunLogo({ large = false }: { large?: boolean }) {
  return (
    <span
      aria-hidden="true"
      className={cn("sun-logo", large && "sun-logo-large")}
    >
      <img src="https://assets.staryield.net/assets/figma/6d1f0.png" alt="" className="sun-shape" />
      <img src="https://assets.staryield.net/assets/figma/3e3d0.svg" alt="" className="sun-face" />
    </span>
  );
}
