import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

/** Raise the complete positioned layer, including its rotated card face. */
export function TarotStackCard({
  label,
  className,
  children,
  ...props
}: Omit<ComponentProps<"div">, "aria-label" | "role" | "tabIndex"> & {
  label: string;
}) {
  return (
    <div
      {...props}
      className={cn("tarot-stack-card", className)}
      role="img"
      aria-label={label}
      tabIndex={0}
    >
      {children}
    </div>
  );
}
