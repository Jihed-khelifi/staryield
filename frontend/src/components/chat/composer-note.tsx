"use client";

import { useI18n } from "@/i18n/i18n-provider";
import { RatePrice } from "@/components/chat/rate-price";
import type { RateView } from "@/components/chat/chat-model";

/**
 * The line under the composer. Clients see the rate and balance (or free
 * minutes); while chats are free (free mode or a sponsored session) it says
 * Staryield covers the chat and never mentions credits. Advisors keep
 * their usual label (their earnings display is unchanged).
 */
export function ComposerNote({
  role,
  free,
  freeMinutes,
  balance,
  rate,
  advisorLabel,
}: {
  role: "client" | "psychic";
  /** Free mode or a sponsored session: display only, the server decides billing. */
  free: boolean;
  freeMinutes: number;
  balance: number;
  rate: RateView | null;
  advisorLabel: string;
}) {
  const { text, locale } = useI18n();
  if (role !== "client") return <>{advisorLabel}</>;
  if (free) return <span data-testid="composer-free-note">{text("This chat is free. Staryield covers it.")}</span>;
  const rateNode = rate && rate.rate > 0 ? <RatePrice rate={rate} /> : null;
  return freeMinutes > 0 ? (
    <>
      {text("{value0} free minutes, then", { value0: freeMinutes })} {rateNode}
    </>
  ) : (
    <>
      {rateNode} · {text("balance {value0}", { value0: balance.toLocaleString(locale) })}
    </>
  );
}
