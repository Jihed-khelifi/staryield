import type { ApiError } from "@/lib/api/errors";
import type { OffSessionAllowance } from "@/lib/api/types";

/** Why the composer cannot send now (English source copy keys, translated with text()). */
export type ComposerReason =
  | "no_conversation"
  | "pending_request"
  | "not_eligible"
  | "disabled"
  | "limit_reached"
  | "checking"
  | "check_failed";

export type ComposerAvailability =
  | { enabled: true; /** Messages go outside a live chat (free, capped). */ offSession: boolean }
  | { enabled: false; reason: ComposerReason; /** Clients get a "Start a live chat" button next to the reason (the cap, the feature being off): the start card in the feed may be replaced by the chat summary. */ canStartChat: boolean; /** The allowance could not be loaded: offer a retry. */ canRetry: boolean };

/**
 * §22.3: the composer is enabled while a live chat is active or paused, or
 * when the allowance says `enabled && eligible && (unlimited || remaining > 0)`.
 * Everything else is disabled with a reason the user can act on.
 */
export function composerAvailability(input: {
  role: "client" | "psychic";
  hasConversation: boolean;
  /** A live chat of this conversation is active or paused. */
  live: boolean;
  /** A chat request is waiting for the advisor (requested/accepted). */
  requestPending: boolean;
  allowance: OffSessionAllowance | null;
  allowanceLoading: boolean;
  allowanceError: ApiError | null;
}): ComposerAvailability {
  if (input.live) return { enabled: true, offSession: false };
  const client = input.role === "client";
  const blocked = (reason: ComposerReason, extra: { canStartChat?: boolean; canRetry?: boolean } = {}): ComposerAvailability => ({
    enabled: false,
    reason,
    canStartChat: Boolean(extra.canStartChat) && client && !input.requestPending,
    canRetry: Boolean(extra.canRetry),
  });
  if (!input.hasConversation) return blocked("no_conversation");
  const a = input.allowance;
  if (!a) return input.allowanceError ? blocked("check_failed", { canRetry: true }) : blocked("checking");
  if (a.live) return { enabled: true, offSession: false };
  if (!a.enabled) return blocked("disabled", { canStartChat: true });
  // Nothing to offer here: the start-chat card above the composer (SessionPanel) is the call to action.
  if (!a.eligible) return input.requestPending ? blocked("pending_request") : blocked("not_eligible");
  if (!a.unlimited && a.remaining <= 0) return blocked("limit_reached", { canStartChat: true });
  return { enabled: true, offSession: true };
}

/** English reason copy (call sites translate with text(); `{limit}`/`{name}` are filled in). */
export function reasonCopy(reason: ComposerReason, role: "client" | "psychic"): string {
  switch (reason) {
    case "pending_request":
      return "You can write once the live chat starts.";
    case "checking":
      return "Checking whether you can write…";
    case "check_failed":
      return "We couldn't check your messages outside a live chat.";
    case "limit_reached":
      return role === "client"
        ? "You've used today's {value0} messages outside a live chat. Start a live chat to keep talking."
        : "You've used today's {value0} messages outside a live chat. You can write again tomorrow or in the next live chat.";
    case "disabled":
      return role === "client" ? "Start a live chat to send messages" : "Messages outside a live chat are turned off right now.";
    case "not_eligible":
    case "no_conversation":
      return role === "client" ? "Start a live chat to send messages" : "You can write once a live chat with this client has started.";
  }
}
