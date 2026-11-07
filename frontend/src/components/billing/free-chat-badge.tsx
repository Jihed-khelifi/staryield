"use client";

import { MessageCircleHeart } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { useFreeMode } from "@/lib/site-config/site-config-provider";
import { cn } from "@/lib/utils";

/**
 * "Free live chat" pill shown while free mode is on (§15.4; display only, the
 * server decides billing). Renders nothing otherwise.
 */
export function FreeChatBadge({ className }: { className?: string }) {
  const { text } = useI18n();
  const free = useFreeMode();
  if (!free) return null;
  return (
    <span
      data-testid="free-chat-badge"
      className={cn(
        "inline-flex w-fit items-center gap-1 rounded-full border border-gold bg-gold-cta px-2.5 py-0.5 font-serif text-xs font-semibold text-ink",
        className,
      )}
    >
      <MessageCircleHeart size={12} aria-hidden />
      {text("Free live chat")}
    </span>
  );
}

/**
 * Credits page notice while free mode is on: live chat costs nothing right
 * now, credits still pay for custom requests (§15.4 rule 7).
 */
export function FreeModeCreditsNotice({ className }: { className?: string }) {
  const { text } = useI18n();
  const free = useFreeMode();
  if (!free) return null;
  return (
    <div
      role="note"
      data-testid="credits-free-notice"
      className={cn("flex items-start gap-2.5 rounded-xl border border-gold bg-cream px-4 py-3 text-sm text-ink", className)}
    >
      <MessageCircleHeart size={18} aria-hidden className="mt-0.5 shrink-0 text-amber" />
      <p>
        <strong className="font-semibold">{text("Live chat is free right now.")}</strong>{" "}
        {text("Credits still pay for custom requests, so you can top up whenever you like.")}
      </p>
    </div>
  );
}
