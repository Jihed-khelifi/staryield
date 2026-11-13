/* eslint-disable @next/next/no-img-element */
import { cn } from "@/lib/utils";

/**
 * Card acceptance marks (Visa, Mastercard) shown wherever a customer pays or
 * learns how to pay: the checkout's pay step, the pricing page and the footer
 * (docs/platform-extensions.md §25.6). Full-colour marks from the
 * `payment-icons` package (MPL-2.0), served from R2 like every other asset;
 * the brand names are the alt text, so screen readers announce them.
 *
 * Server- and client-safe: it renders plain markup, and the caller passes the
 * already translated label ("We accept") when one should be visible. The marks
 * (1–3 KB each) load eagerly: reviewers and their crawlers screenshot pages,
 * and a lazy image below the fold can be captured before it loads.
 */
export const CARD_MARKS = [
  { brand: "visa", name: "Visa", src: "https://assets.staryield.net/assets/payments/visa-52743e9957.svg" },
  { brand: "mastercard", name: "Mastercard", src: "https://assets.staryield.net/assets/payments/mastercard-d3d09ebf1c.svg" },
] as const;

/** The marks' artwork is 750 × 471. */
const RATIO = 750 / 471;

export function AcceptedCards({
  height = 24,
  label,
  className,
}: {
  /** Mark height in CSS pixels (24 by default; never below 20 so the marks stay legible). */
  height?: number;
  /** Visible, translated label before the marks ("We accept"); omitted: the list is labelled for screen readers only. */
  label?: string;
  className?: string;
}) {
  const h = Math.max(20, height);
  const w = Math.round(h * RATIO);
  return (
    <div className={cn("accepted-cards inline-flex flex-wrap items-center gap-2", className)} data-testid="accepted-cards">
      {label ? <span className="text-xs">{label}</span> : null}
      <ul className="flex items-center gap-1.5" aria-label={label ? undefined : "Visa, Mastercard"}>
        {CARD_MARKS.map((mark) => (
          <li key={mark.brand} className="flex">
            <img src={mark.src} alt={mark.name} width={w} height={h} decoding="async" className="rounded-[3px]" />
          </li>
        ))}
      </ul>
    </div>
  );
}
