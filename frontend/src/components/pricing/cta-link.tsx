import Link from "next/link";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/** The gold call to action of the pricing page (Figma "Pricing action"): a link, 48 px tall, 8 px corners. */
export function CtaLink({ href, className, children, ...props }: Omit<React.ComponentProps<typeof Link>, "href"> & { href: string }) {
  return (
    <Button asChild variant="cta" size="xl" className={cn("h-12 rounded-md px-6 text-base", className)}>
      <Link href={href} {...props}>
        {children}
      </Link>
    </Button>
  );
}

/** Shared section frame: full-bleed band, content capped at 1280 px (Figma 96 px side padding on desktop). */
export function PricingSection({
  className,
  innerClassName,
  children,
  ...props
}: React.ComponentProps<"section"> & { innerClassName?: string }) {
  return (
    <section {...props} className={cn("w-full px-6 py-12 min-[768px]:px-12 min-[768px]:py-16 min-[1200px]:px-24", className)}>
      <div className={cn("mx-auto w-full max-w-[1280px]", innerClassName)}>{children}</div>
    </section>
  );
}

/*
 * Note: Tailwind emits arbitrary `min-[…px]:` variants before the named
 * breakpoints, so the pricing components use `min-[768px]:` (not Tailwind's `md`)
 * wherever they combine with `min-[1200px]:`; the cascade then follows width.
 */
