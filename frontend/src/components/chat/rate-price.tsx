"use client";

import { useI18n } from "@/i18n/i18n-provider";
import { useMembershipsEnabled } from "@/lib/site-config/site-config-provider";
import { cn } from "@/lib/utils";
import type { RateView } from "@/components/chat/chat-model";

/**
 * A per-minute rate. With a member discount the list rate is struck
 * through visually; screen readers hear one plain sentence instead, since
 * strikethrough is not announced reliably. While memberships are switched
 * off (§25.1) no discount is advertised: the rate the viewer pays is shown
 * as a plain rate.
 */
export function RatePrice({ rate, className }: { rate: RateView; className?: string }) {
  const { text, locale } = useI18n();
  const memberships = useMembershipsEnabled();
  const fmt = (n: number) => n.toLocaleString(locale);
  if (!rate.discounted || !memberships) return <span className={className}>{text("{value0} credits/min", { value0: fmt(rate.rate) })}</span>;
  return (
    <span className={cn("rate-price", className)} data-testid="member-rate">
      <span aria-hidden="true">
        <s className="rate-price-list" data-testid="list-rate">
          {fmt(rate.listRate)}
        </s>{" "}
        <span className="rate-price-member">{text("{value0} credits/min", { value0: fmt(rate.rate) })}</span>
      </span>
      <span className="sr-only">
        {text("Member price: {value0} credits per minute instead of {value1} ({value2}% off).", {
          value0: fmt(rate.rate),
          value1: fmt(rate.listRate),
          value2: rate.discountPct.toLocaleString(locale, { maximumFractionDigits: 2 }),
        })}
      </span>
    </span>
  );
}
