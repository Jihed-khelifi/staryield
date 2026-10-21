"use client";

import * as React from "react";
import { Star } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { cn } from "@/lib/utils";

/** Read-only stars ("Rated 4 out of 5"). */
export function StarDisplay({ rating, size = 14, className }: { rating: number; size?: number; className?: string }) {
  const { t } = useI18n();
  const rounded = Math.round(Math.max(0, Math.min(5, rating)));
  return (
    <span role="img" aria-label={t("chatroom.panel.ratingLabel", { rating })} className={cn("inline-flex items-center gap-0.5", className)}>
      {[1, 2, 3, 4, 5].map((n) => (
        <Star key={n} size={size} strokeWidth={1.5} aria-hidden className={n <= rounded ? "fill-amber text-amber" : "text-gold/60"} />
      ))}
    </span>
  );
}

/**
 * 1–5 star input as a native radio group: arrow keys move between stars,
 * each option has a spoken label, and the hover preview is visual only.
 */
export function StarRatingInput({
  value,
  onChange,
  name,
  legend,
  disabled,
  invalid,
}: {
  value: number;
  onChange: (rating: number) => void;
  name: string;
  legend: string;
  disabled?: boolean;
  invalid?: boolean;
}) {
  const { text } = useI18n();
  const [hover, setHover] = React.useState(0);
  const shown = hover || value;
  return (
    <fieldset className="star-rating" disabled={disabled} aria-invalid={invalid || undefined}>
      <legend className="mb-2 text-sm font-semibold">{legend}</legend>
      <div className="star-rating-options" onMouseLeave={() => setHover(0)}>
        {[1, 2, 3, 4, 5].map((n) => (
          <label key={n} className="star-option" data-active={n <= shown || undefined} onMouseEnter={() => setHover(n)}>
            <input
              type="radio"
              name={name}
              value={n}
              checked={value === n}
              onChange={() => onChange(n)}
              className="sr-only"
              aria-label={text("{count} out of 5 stars", { count: n })}
            />
            <Star size={28} strokeWidth={1.4} aria-hidden />
          </label>
        ))}
      </div>
    </fieldset>
  );
}
