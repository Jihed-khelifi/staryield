/**
 * Pure rules of the free-trial hold on the web (docs/platform-extensions.md
 * §26.1): the countdown to the server's deadline, which copy of a hold wins
 * when events and API answers race, the placeholder shown for a locked message,
 * and the small derivations the chatroom needs (hidden messages, the advisor's
 * marker, inbox previews). No React here, so every rule is unit-tested.
 */
import type { ChatHold, HoldReason, HoldStatus } from "@/lib/api/holds";
import type { ChatMessage, Conversation } from "@/lib/api/types";
import type { TopUpHold } from "@/lib/billing/top-up";

// --- Countdown ---------------------------------------------------------------

/** Whole seconds until the server's deadline (never below 0). `offsetMs` is server time minus local time. */
export function holdSecondsLeft(expiresAt: string | undefined, now = Date.now(), offsetMs = 0): number {
  if (!expiresAt) return 0;
  const deadline = new Date(expiresAt).getTime();
  if (!Number.isFinite(deadline)) return 0;
  return Math.max(0, Math.ceil((deadline - (now + offsetMs)) / 1000));
}

/** "0:59", "1:05": minutes and zero-padded seconds. */
export function formatCountdown(seconds: number): string {
  const whole = Math.max(0, Math.floor(Number.isFinite(seconds) ? seconds : 0));
  return `${Math.floor(whole / 60)}:${String(whole % 60).padStart(2, "0")}`;
}

// --- Which copy of a hold is current -----------------------------------------

/** `open` is the only state that can still change; `revealed` is the end of an `expired` hold. */
const RANK: Record<HoldStatus, number> = { open: 0, converted: 1, expired: 1, revealed: 2 };

export const isHoldOpen = (hold: ChatHold | null | undefined): hold is ChatHold => hold?.status === "open";

/**
 * The newer of two copies of the same hold. A hold only moves forward (open →
 * converted or expired → revealed) and its deadline only moves out, so a late
 * event or an older page can never bring back an earlier state.
 */
export function mergeHold(current: ChatHold | undefined | null, incoming: ChatHold): ChatHold {
  if (!current || current.id !== incoming.id) return incoming;
  if (RANK[incoming.status] > RANK[current.status]) return incoming;
  if (RANK[incoming.status] < RANK[current.status]) return current;
  if (incoming.status !== "open") return { ...current, ...incoming };
  const later = incoming.expires_at >= current.expires_at ? incoming : current;
  return { ...current, ...incoming, expires_at: later.expires_at, held_messages: Math.max(current.held_messages, incoming.held_messages) };
}

export type HoldStore = {
  /** Newest known copy of each hold, by id. */
  holds: Record<string, ChatHold>;
  /** Deadlines of `session.hold_extended` events whose hold was not known yet. */
  extended: Record<string, string>;
};

export const EMPTY_HOLD_STORE: HoldStore = { holds: {}, extended: {} };

export type HoldAction = { type: "hold"; hold: ChatHold } | { type: "extended"; holdId: string; expiresAt: string };

/** Events (`session.held`, `session.hold_extended`, `session.hold_ended`) and API answers, folded into one store. */
export function holdStoreReducer(state: HoldStore, action: HoldAction): HoldStore {
  if (action.type === "hold") {
    const known = state.holds[action.hold.id];
    const early = state.extended[action.hold.id];
    // A deadline that arrived before the hold itself applies to it.
    const incoming = early && action.hold.status === "open" && early > action.hold.expires_at ? { ...action.hold, expires_at: early } : action.hold;
    const next = mergeHold(known, incoming);
    return next === known ? state : { ...state, holds: { ...state.holds, [next.id]: next } };
  }
  const known = state.holds[action.holdId];
  if (!known) {
    const prev = state.extended[action.holdId];
    return prev && prev >= action.expiresAt ? state : { ...state, extended: { ...state.extended, [action.holdId]: action.expiresAt } };
  }
  if (known.status !== "open" || known.expires_at >= action.expiresAt) return state;
  return { ...state, holds: { ...state.holds, [known.id]: { ...known, expires_at: action.expiresAt } } };
}

/** The hold to show: the session's own copy or the newer one the store knows. */
export function currentHold(fromSession: ChatHold | undefined, store: HoldStore): ChatHold | null {
  if (!fromSession) return null;
  const known = store.holds[fromSession.id];
  const base = known ? mergeHold(fromSession, known) : fromSession;
  const early = store.extended[base.id];
  return base.status === "open" && early && early > base.expires_at ? { ...base, expires_at: early } : base;
}

// --- Copy ----------------------------------------------------------------------

/** English copy (translate with `text()`): why the reading was held. */
export const HOLD_REASON_COPY: Record<HoldReason, string> = {
  trial_ended: "Your free minutes are over.",
  credits_exhausted: "Your credits ran out.",
};

export function holdReasonCopy(reason: HoldReason | undefined): string {
  return HOLD_REASON_COPY[reason === "credits_exhausted" ? "credits_exhausted" : "trial_ended"];
}

// --- Locked messages -------------------------------------------------------------

const SYLLABLES = ["la", "ve", "mo", "ri", "san", "tu", "e", "lu", "na", "po", "di", "ka", "so", "men", "tia", "ro", "ba", "le", "no", "shi"];
const MAX_PLACEHOLDER = 2000;

/** Small seeded generator (mulberry32): the same message always blurs the same way. */
function seeded(seed: string): () => number {
  let h = 1779033703;
  for (let i = 0; i < seed.length; i++) {
    h = Math.imul(h ^ seed.charCodeAt(i), 3432918353);
    h = (h << 13) | (h >>> 19);
  }
  let a = h >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/**
 * Stand-in text for a locked message, exactly `length` characters (clamped to
 * 1..2000) of invented words. It is made here from the length alone: the
 * hidden text never reaches the browser, and nothing about it is in the result.
 */
export function lockedPlaceholder(length: number | undefined, seed: string): string {
  const total = Math.min(MAX_PLACEHOLDER, Math.max(1, Math.floor(Number.isFinite(length) ? (length as number) : 0) || 24));
  const next = seeded(seed);
  let out = "";
  while (out.length < total) {
    const syllables = 1 + Math.floor(next() * 3);
    let word = "";
    for (let i = 0; i < syllables; i++) word += SYLLABLES[Math.floor(next() * SYLLABLES.length)];
    out += (out ? " " : "") + word;
  }
  return out.slice(0, total).trimEnd().padEnd(total, "a");
}

/** What the advisor sees on a message the client cannot read yet (§26.1). */
export type HeldMarker = "hidden" | "not_revealed";

/**
 * The advisor's marker for one of their messages: "Hidden from client" while
 * the hold is open, "Not revealed" once it expired. None once it was revealed
 * or converted. A hold that is not known is an older, settled one.
 */
export function heldMarker(message: Pick<ChatMessage, "held" | "hold_id">, hold: ChatHold | null | undefined): HeldMarker | null {
  if (!message.held) return null;
  if (hold && (!message.hold_id || hold.id === message.hold_id)) {
    if (hold.status === "open") return "hidden";
    if (hold.status === "expired") return "not_revealed";
    return null;
  }
  return "not_revealed";
}

/** The newest hold that still has hidden messages in the loaded timeline, with how many are loaded. */
export function lockedSummary(messages: readonly Pick<ChatMessage, "locked" | "hold_id" | "seq">[]): { holdId: string; loaded: number } | null {
  let newest: { holdId: string; seq: number } | null = null;
  const counts = new Map<string, number>();
  for (const m of messages) {
    if (!m.locked || !m.hold_id) continue;
    counts.set(m.hold_id, (counts.get(m.hold_id) ?? 0) + 1);
    if (!newest || (m.seq ?? 0) >= newest.seq) newest = { holdId: m.hold_id, seq: m.seq ?? 0 };
  }
  return newest ? { holdId: newest.holdId, loaded: counts.get(newest.holdId) ?? 0 } : null;
}

/** A revealed message that is the preview of an inbox row replaces the locked preview. */
export function revealInbox(items: Conversation[], messages: ChatMessage[]): Conversation[] {
  if (messages.length === 0) return items;
  const byId = new Map(messages.map((m) => [m.id, m]));
  let changed = false;
  const next = items.map((c) => {
    const revealed = c.last_message ? byId.get(c.last_message.id) : undefined;
    if (!revealed) return c;
    changed = true;
    return { ...c, last_message: { ...revealed, locked: undefined } };
  });
  return changed ? next : items;
}

/** The hold as the top-up dialog needs it. */
export function toTopUpHold(hold: ChatHold | null | undefined, clockOffsetMs: number): TopUpHold | undefined {
  if (!isHoldOpen(hold)) return undefined;
  return { reason: hold.reason, expiresAt: hold.expires_at, priceCredits: hold.price_credits, clockOffsetMs };
}
