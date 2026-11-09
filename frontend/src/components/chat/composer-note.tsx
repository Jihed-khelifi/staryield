"use client";

import { useI18n } from "@/i18n/i18n-provider";
import type { OffSessionAllowance } from "@/lib/api/types";
import { RatePrice } from "@/components/chat/rate-price";
import type { RateView } from "@/components/chat/chat-model";

/**
 * The line under the composer. Clients see the rate and balance (or free
 * minutes); while chats are free (free mode or a sponsored session) it says
 * Staryield covers the chat and never mentions credits. Advisors keep
 * their usual label (their earnings display is unchanged).
 *
 * Outside a live chat (`offSession`, §22.3) messages are free for clients and
 * not billed for advisors: the note says so and how many are left today.
 */
export function ComposerNote({
  role,
  free,
  freeMinutes,
  balance,
  rate,
  advisorLabel,
  offSession,
}: {
  role: "client" | "psychic";
  /** Free mode or a sponsored session: display only, the server decides billing. */
  free: boolean;
  freeMinutes: number;
  balance: number;
  rate: RateView | null;
  advisorLabel: string;
  /** The viewer writes outside a live chat: show the allowance instead of the rate. */
  offSession?: OffSessionAllowance | null;
}) {
  const { text, locale } = useI18n();
  if (offSession) {
    const left = offSession.unlimited
      ? text("unlimited during free mode")
      : text("{value0} of {value1} left today", { value0: offSession.remaining.toLocaleString(locale), value1: offSession.limit.toLocaleString(locale) });
    return (
      <span data-testid="composer-offsession-note">
        {text(role === "client" ? "Outside a live chat: free" : "Outside a live chat: not billed")} · {left}
      </span>
    );
  }
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
