/* eslint-disable @next/next/no-img-element */
import { cn } from "@/lib/utils";

const SUN_SHAPE = "https://assets.staryield.net/assets/figma/6d1f0.png";
const SUN_FACE = "https://assets.staryield.net/assets/figma/3e3d0.svg";

/**
 * The Staryield sun. Small marks (header, footer, chat brand) are the plain
 * eight-point sun — revisions 2.0 removed the face from every small logo;
 * only the large logo-page sun keeps its face.
 */
export function SunLogo({ large = false }: { large?: boolean }) {
  return (
    <span
      aria-hidden="true"
      data-face={large || undefined}
      className={cn("sun-logo", large && "sun-logo-large")}
    >
      <img src={SUN_SHAPE} alt="" className="sun-shape" />
      {large ? <img src={SUN_FACE} alt="" className="sun-face" /> : null}
    </span>
  );
}
