/**
 * Chat view-model helpers. The server owns session state, billing and
 * message identity; these helpers only merge what the server sends and derive
 * display values (timers are computed from server timestamps, never from a
 * client-side counter).
 *
 * Ordering: every message has a per-conversation `seq` (1, 2, 3, … without
 * gaps). Confirmed messages are ordered strictly by it; local messages that
 * the server has not confirmed yet follow, in the order they were typed. A
 * seq beyond the contiguous head means events were missed: the gap is
 * filled from REST (`/messages/since?after_seq=`), one walk at a time.
 */
import type { ChatMessage, ChatSession, SincePage } from "@/lib/api/types";

/** Local delivery state of a message typed in this tab. */
export type DeliveryState = "queued" | "sending" | "failed";

export type UiMessage = ChatMessage & {
  /** Local delivery state for messages typed in this tab (absent once confirmed). */
  pending?: DeliveryState;
  /** The picked image of an optimistic image message (kept for retries). */
  localFile?: File;
  /** Error code of the last failed delivery attempt. */
  failure?: string;
  /** The server's own explanation of the failure (e.g. `fields.image`), shown as is. */
  failureDetail?: string;
};

const isLocal = (m: UiMessage) => Boolean(m.pending);

/** Confirmed messages by seq, then local ones in the order they were typed. */
export function compareMessages(a: UiMessage, b: UiMessage): number {
  const localA = isLocal(a);
  const localB = isLocal(b);
  if (localA !== localB) return localA ? 1 : -1;
  if (!localA) {
    const seqA = a.seq ?? 0;
    const seqB = b.seq ?? 0;
    if (seqA > 0 && seqB > 0 && seqA !== seqB) return seqA - seqB;
  }
  return a.created_at === b.created_at ? a.id.localeCompare(b.id) : a.created_at.localeCompare(b.created_at);
}

function sameClientMessage(a: UiMessage, b: UiMessage): boolean {
  return Boolean(a.client_msg_id) && a.client_msg_id === b.client_msg_id && a.sender_id === b.sender_id;
}

/**
 * Insert or replace messages — by id, by seq, or by client id for optimistic
 * ones (a confirmed copy replaces its optimistic original) — and return them
 * in display order. Never returns two entries for the same message.
 */
export function mergeMessages(current: UiMessage[], incoming: UiMessage[]): UiMessage[] {
  const out = [...current];
  for (const m of incoming) {
    let idx = out.findIndex((x) => x.id === m.id);
    if (idx < 0 && m.seq && !m.pending) idx = out.findIndex((x) => !x.pending && x.seq === m.seq);
    if (idx < 0 && m.client_msg_id) idx = out.findIndex((x) => sameClientMessage(x, m));
    if (idx < 0) {
      out.push(m);
      continue;
    }
    const prev = out[idx];
    // A late optimistic update never downgrades a confirmed message.
    if (m.pending && !prev.pending) continue;
    out[idx] = {
      ...prev,
      ...m,
      // Events and acks omit the viewer's bookmark flag; keep the known one.
      saved: m.saved ?? prev.saved,
      pending: m.pending,
      failure: m.pending ? m.failure : undefined,
      localFile: m.pending ? (m.localFile ?? prev.localFile) : undefined,
    };
    // Two confirmed copies (id and client id) can coexist only transiently.
    if (!m.pending) {
      for (let j = out.length - 1; j >= 0; j--) if (j !== idx && out[j].pending && sameClientMessage(out[j], m)) out.splice(j, 1);
    }
  }
  return out.sort(compareMessages);
}

// --- Timeline: confirmed history of one conversation -------------------------

export type Timeline = {
  /** Confirmed messages in seq order, without duplicates. */
  messages: UiMessage[];
  /** Every seq from the oldest loaded message up to this one is present. */
  headSeq: number;
  /** The newest page arrived, so later seqs can be checked for gaps. */
  loaded: boolean;
  /** History exists before the oldest loaded message. */
  hasOlder: boolean;
};

export const EMPTY_TIMELINE: Timeline = { messages: [], headSeq: 0, loaded: false, hasOlder: false };

function mergeConfirmed(current: UiMessage[], incoming: ChatMessage[]): UiMessage[] {
  return mergeMessages(
    current,
    incoming.filter((m) => (m.seq ?? 0) > 0).map((m) => ({ ...m, pending: undefined })),
  ).filter((m) => !m.pending);
}

function contiguousHead(messages: UiMessage[], from: number): number {
  const seqs = new Set(messages.map((m) => m.seq ?? 0));
  let head = from;
  while (seqs.has(head + 1)) head++;
  return head;
}

/** Seq of the newest confirmed message (0 when none). */
export function newestSeq(t: Timeline): number {
  return t.messages.at(-1)?.seq ?? 0;
}

/** Seq of the oldest loaded message (0 when none). */
export function oldestSeq(t: Timeline): number {
  return t.messages[0]?.seq ?? 0;
}

/**
 * The newest history page (initial load). It is contiguous and ends at the
 * conversation head; events merged before it arrived are kept.
 */
export function withNewestPage(t: Timeline, page: { items: ChatMessage[]; has_more: boolean }): Timeline {
  const messages = mergeConfirmed(t.messages, page.items);
  const pageHead = page.items.reduce((max, m) => Math.max(max, m.seq ?? 0), 0);
  return { messages, headSeq: contiguousHead(messages, Math.max(pageHead, t.loaded ? t.headSeq : 0)), loaded: true, hasOlder: page.has_more };
}

/** An older page (`before_seq`), contiguous with the oldest loaded message. */
export function withOlderPage(t: Timeline, page: { items: ChatMessage[]; has_more: boolean }): Timeline {
  return { ...t, messages: mergeConfirmed(t.messages, page.items), hasOlder: page.has_more };
}

/** Confirmed messages from events, acks or gap fills. */
export function withMessages(t: Timeline, items: ChatMessage[]): Timeline {
  if (items.length === 0) return t;
  const messages = mergeConfirmed(t.messages, items);
  return { ...t, messages, headSeq: t.loaded ? contiguousHead(messages, t.headSeq) : t.headSeq };
}

/** Messages are missing between the contiguous head and a newer one. */
export function hasGap(t: Timeline): boolean {
  return t.loaded && newestSeq(t) > t.headSeq;
}

/** An event with this seq would leave a hole (it is past head + 1). */
export function isGapAfter(t: Timeline, seq: number | undefined): boolean {
  return t.loaded && typeof seq === "number" && seq > t.headSeq + 1;
}

/** The newest seq of a message the viewer did not send. */
export function newestIncomingSeq(messages: UiMessage[], meId: string): number {
  for (let i = messages.length - 1; i >= 0; i--) {
    const m = messages[i];
    if (!m.pending && m.sender_id !== meId && (m.seq ?? 0) > 0) return m.seq!;
  }
  return 0;
}

// --- Gap filling ---------------------------------------------------------------

export type SinceFetcher = (afterSeq: number, signal: AbortSignal) => Promise<SincePage>;

export type BackfillOptions = {
  fetchSince: SinceFetcher;
  /** The contiguous head to continue from (read when each walk starts). */
  head: () => number;
  /** Merge a page of messages. */
  apply: (items: ChatMessage[]) => void;
  /** Called when more than `maxPages` pages were missed (reload instead). */
  onOverflow?: () => void;
  /** Pages walked at most per request (200 messages each by default). */
  maxPages?: number;
  /** Whether a gap remains (a coalesced, non-forced request walks again only then). */
  needed?: () => boolean;
};

/**
 * Single-flight gap filling over REST: one walk (`after_seq`, following
 * `has_more`) at a time. Requests made while a walk runs are coalesced into
 * one follow-up walk from the head reached by then, so a burst of
 * out-of-order events costs one fetch, not one per event.
 */
export class Backfill {
  private running: Promise<void> | null = null;
  private again = false;
  private againForced = false;
  private ctrl: AbortController | null = null;
  private stopped = false;

  constructor(private readonly opts: BackfillOptions) {}

  /**
   * Fill from the current head; resolves when no walk is pending, rejects
   * with the walk's error. A request made during a walk is coalesced into
   * one follow-up walk: always when `force` (a reconnect: anything may have
   * been missed), otherwise only if a gap remains after the walk.
   */
  request(force = true): Promise<void> {
    if (this.stopped) return Promise.resolve();
    if (this.running) {
      this.again = true;
      this.againForced ||= force;
      return this.running;
    }
    this.running = (async () => {
      try {
        let walk = true;
        while (walk && !this.stopped) {
          this.again = false;
          this.againForced = false;
          await this.walk();
          walk = this.again && (this.againForced || (this.opts.needed?.() ?? true));
        }
      } finally {
        this.running = null;
        this.ctrl = null;
      }
    })();
    return this.running;
  }

  get busy(): boolean {
    return this.running !== null;
  }

  /** Abort the walk in flight and ignore later requests. */
  stop() {
    this.stopped = true;
    this.ctrl?.abort();
  }

  private async walk() {
    const ctrl = new AbortController();
    this.ctrl = ctrl;
    const maxPages = this.opts.maxPages ?? 5;
    let after = this.opts.head();
    for (let pages = 0; ; pages++) {
      if (pages >= maxPages) {
        this.opts.onOverflow?.();
        return;
      }
      const page = await this.opts.fetchSince(after, ctrl.signal);
      if (ctrl.signal.aborted || this.stopped) return;
      this.opts.apply(page.items);
      const last = page.items.at(-1)?.seq ?? 0;
      if (!page.has_more || last <= after) return;
      after = last;
    }
  }
}

// --- Sessions ----------------------------------------------------------------

/** A session snapshot with the local time it was received (clock sync). */
export type SessionSnapshot = { session: ChatSession; receivedAt: number };

export function snapshot(session: ChatSession): SessionSnapshot {
  return { session, receivedAt: Date.now() };
}

/** Server clock offset in ms (server − local) at the time of the snapshot. */
export function clockOffset(s: SessionSnapshot): number {
  return new Date(s.session.server_time).getTime() - s.receivedAt;
}

/** Elapsed active seconds, extrapolated from the server value while active. */
export function elapsedSeconds(s: SessionSnapshot | null, now = Date.now()): number {
  if (!s) return 0;
  const base = s.session.active_seconds;
  if (s.session.status !== "active") return base;
  return base + Math.max(0, Math.floor((now - s.receivedAt) / 1000));
}

/** Seconds until an ISO deadline, using the server clock. */
export function secondsUntil(iso: string | undefined, s: SessionSnapshot | null, now = Date.now()): number {
  if (!iso || !s) return 0;
  return Math.max(0, Math.ceil((new Date(iso).getTime() - (now + clockOffset(s))) / 1000));
}

export const OPEN_STATUSES = new Set(["requested", "accepted", "active", "disconnected"]);

export function isOpen(session: ChatSession | null | undefined): boolean {
  return Boolean(session && OPEN_STATUSES.has(session.status));
}

// --- Rates and earnings ----------------------------------------------------------

export type RateView = {
  /** What the viewer pays (clients) or earns on (advisors) per minute. */
  rate: number;
  /** The advisor's list rate. */
  listRate: number;
  /** Member discount in whole percent (0 without one). */
  discountPct: number;
  /** A member discount lowers the rate below the list rate. */
  discounted: boolean;
};

/**
 * The per-minute rate of a session as the viewer should see it. Clients
 * with a member discount get `rate_per_minute` (discounted) and the list
 * rate; advisors always see their own list rate.
 */
export function sessionRate(session: Pick<ChatSession, "rate_per_minute" | "list_rate_per_minute" | "discount_pct"> | null | undefined, fallback = 0): RateView {
  const rate = session?.rate_per_minute ?? fallback;
  const listRate = Math.max(session?.list_rate_per_minute ?? rate, rate);
  const discountPct = Math.max(0, Math.round((session?.discount_pct ?? 0) * 100) / 100);
  return { rate, listRate, discountPct, discounted: listRate > rate };
}

/** Money in minor units ("$40.33"), in the viewer's locale. */
export function formatMoney(cents: number, currency: string | undefined, locale: string): string {
  const value = (Number.isFinite(cents) ? cents : 0) / 100;
  try {
    return new Intl.NumberFormat(locale, { style: "currency", currency: currency || "USD" }).format(value);
  } catch {
    // Unknown currency code: never break the header.
    return `${value.toFixed(2)} ${currency ?? ""}`.trim();
  }
}

/** The advisor's live session earnings, when the session view carries them. */
export function sessionEarnings(session: ChatSession | null | undefined): { credits: number; cents?: number; currency?: string } | null {
  if (!session || typeof session.psychic_earnings !== "number") return null;
  return {
    credits: session.psychic_earnings,
    cents: typeof session.psychic_earnings_cents === "number" ? session.psychic_earnings_cents : undefined,
    currency: session.currency,
  };
}

// --- Copy --------------------------------------------------------------------------

/** English copy for end reasons (translated with text()). */
export const endReasonCopy: Record<string, string> = {
  client_ended: "The client ended the chat.",
  psychic_ended: "The advisor ended the chat.",
  credits_exhausted: "The chat ended because the credit balance ran out.",
  grace_expired: "The chat ended because the connection could not be restored.",
  admin_terminated: "The chat was ended by Staryield support.",
  account_disabled: "The chat ended because an account was deactivated.",
  declined: "The advisor declined this request.",
  request_timeout: "The advisor did not answer in time. No credits were used.",
  not_started: "The chat could not start in time. No credits were used.",
  cancelled: "The request was cancelled.",
};

/** English copy for a system message body ("session.started", "session.ended:<reason>"). */
export function systemCopy(body: string): string {
  if (body === "session.started") return "Live chat started";
  if (body.startsWith("session.ended")) return "Live chat ended";
  return body;
}

export function newClientMsgId(): string {
  const rand = typeof crypto !== "undefined" && "randomUUID" in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random()}`;
  return `m-${rand}`.replace(/[^A-Za-z0-9_\-:.]/g, "").slice(0, 64);
}
