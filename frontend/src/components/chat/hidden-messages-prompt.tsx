"use client";

import { FiLock } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import { cn } from "@/lib/utils";
import { useChatHold } from "@/components/chat/chat-hold-context";
import { Button } from "@/components/ui/button";

/**
 * revisions 6.0 (§26.1): after the hold ended, the client's session card keeps
 * working and adds "Top up to read {n} hidden messages". It asks the same
 * question as a click on a locked message: pay one minute's price to reveal
 * them (confirm first; top up first when the wallet is short). Nothing is shown
 * while the hold is open (the banner owns that) or when nothing is hidden.
 */
export function HiddenMessagesPrompt({ className }: { className?: string }) {
  const { text } = useI18n();
  const { role, hold, hidden, openLocked, busy } = useChatHold();
  if (role !== "client" || !hidden || hold?.status === "open") return null;
  // The hold knows how many messages it hid; the loaded timeline only knows the ones it holds.
  const count = hold && hold.id === hidden.holdId ? Math.max(hold.held_messages, hidden.count) : hidden.count;
  return (
    <div className={cn("hidden-messages-prompt", className)} data-testid="hidden-messages-prompt">
      <FiLock aria-hidden className="size-4 shrink-0" />
      <Button type="button" variant="cta" size="sm" onClick={() => openLocked(hidden.holdId)} disabled={busy} aria-busy={busy || undefined} data-testid="hidden-messages-reveal">
        {count === 1 ? text("Top up to read 1 hidden message") : text("Top up to read {value0} hidden messages", { value0: count })}
      </Button>
    </div>
  );
}
