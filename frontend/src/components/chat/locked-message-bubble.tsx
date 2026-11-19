"use client";

import * as React from "react";
import { FiLock } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import { lockedPlaceholder } from "@/lib/chat/hold";
import { useChatHold } from "@/components/chat/chat-hold-context";
import type { UiMessage } from "@/components/chat/chat-model";

/**
 * revisions 6.0 (§26.1): the body of a message the hold hides from the client.
 * The text shown is made here from `body_length` alone: `body` is never read,
 * so even a server that sent it by mistake could not show it. The placeholder
 * is blurred by CSS and hidden from assistive technology; the whole body is one
 * button named "Hidden message from {name}. Top up to read it.", which opens
 * the top-up while the hold is open and the reveal once it expired.
 */
export function LockedMessageBody({ message, authorName }: { message: Pick<UiMessage, "id" | "kind" | "body_length" | "hold_id">; authorName: string }) {
  const { text } = useI18n();
  const { openLocked, busy } = useChatHold();
  const image = message.kind === "image";
  const placeholder = React.useMemo(() => (image ? "" : lockedPlaceholder(message.body_length, message.id)), [image, message.body_length, message.id]);
  return (
    <button
      type="button"
      className="locked-message"
      data-testid="locked-message"
      data-kind={message.kind}
      aria-label={text("Hidden message from {value0}. Top up to read it.", { value0: authorName })}
      aria-busy={busy || undefined}
      onClick={() => openLocked(message.hold_id)}
    >
      {image ? <span className="locked-message-image" aria-hidden /> : null}
      {placeholder ? (
        <span className="locked-message-text" aria-hidden data-testid="locked-message-text">
          {placeholder}
        </span>
      ) : null}
      <span className="locked-message-hint" aria-hidden>
        <FiLock className="size-3" />
        {text("Hidden · Top up to read")}
      </span>
    </button>
  );
}

/** The excerpt line of a quote or an inbox preview for a message the viewer cannot read. */
export function HiddenMessageLabel({ className }: { className?: string }) {
  const { text } = useI18n();
  return (
    <span className={className} data-testid="hidden-message-label">
      <FiLock aria-hidden className="inline size-3 shrink-0" /> {text("Hidden message")}
    </span>
  );
}
