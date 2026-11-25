/**
 * The one source of truth of the "book" button: what an advisor's booking
 * action is, and how it reads, for every combination of advisor status,
 * session, role, wallet, sign-up offer, free mode, waiting list and age
 * state. Pure (no React, no translation): copy is returned as English source
 * keys with values, which `BookButton` runs through `text()`.
 */
import type { AgeStatus, PresenceStatus, Role } from "@/lib/api/types";

export type BookingAction =
  /** Open the chatroom with this advisor. */
  | "chat"
  /** Visitors create an account first (then come back to chat or to wait). */
  | "signup"
  | "join_waitlist"
  | "leave_waitlist"
  /** Nothing to book from here: open the profile. */
  | "view"
  /** No booking action at all (the label explains why). */
  | "none";

/** Why a state is not the plain one; `null` for the plain states. */
export type BookingReason =
  | "session_loading"
  | "waitlist_loading"
  | "pending"
  /** The account must finish the age step first: the action runs the existing age flow. */
  | "age_gate"
  | "advisor_account"
  | "unknown_status";

/** How the button is dressed: primary (gold call to action), secondary (glass), success (joined), muted (no action). */
export type BookingTone = "primary" | "secondary" | "success" | "muted";

/** English source copy (a key of the copy dictionaries) and its placeholder values. */
export type BookingCopy = { key: string; values?: Record<string, number> };

export type BookingInput = {
  /** The advisor's status. Anything outside the four known statuses yields a "view" state. */
  advisorStatus: PresenceStatus | (string & {});
  ratePerMinute: number;
  session: "loading" | "anonymous" | "authenticated";
  role: Role | null;
  /** The signed-in client's free minutes (wallet). */
  freeMinutes: number;
  /** Free minutes a new member receives (site configuration; 0 when the offer is off). */
  signupOfferMinutes: number;
  /** Free mode: live chat costs nothing for everyone right now. */
  freeMode: boolean;
  /** The client's place on this advisor's waiting list; "unknown" while it loads. */
  waitlist: "joined" | "not_joined" | "unknown";
  /** A join or leave request is in flight. */
  pending: "join" | "leave" | null;
  age: AgeStatus;
  /** The signed-in client is in an open reading with this advisor (who then shows busy). */
  inReading?: boolean;
};

/** What the line under "Chat now" says: free mode, the client's own free minutes, the new-member offer, or the rate. */
export type ChatLine = "free_mode" | "own_minutes" | "signup_offer" | "rate";

/** The viewer and site facts that decide the line under "Chat now". */
export type ChatLineInput = Pick<BookingInput, "freeMode" | "session" | "role" | "freeMinutes" | "signupOfferMinutes">;

export type BookingState = {
  action: BookingAction;
  label: BookingCopy;
  sublabel: BookingCopy | null;
  tone: BookingTone;
  disabled: boolean;
  reason: BookingReason | null;
};

const COPY = {
  chat: { key: "Chat now" },
  join: { key: "Join the waiting list" },
  notify: { key: "Notify me when online" },
  joined: { key: "✓ On the waiting list" },
  joining: { key: "Joining…" },
  view: { key: "View profile" },
  backToChat: { key: "Back to chat" },
  advisorAccount: { key: "Advisor accounts can't book readings" },
  inReading: { key: "In a reading" },
  free: { key: "Free for a limited time" },
} as const satisfies Record<string, BookingCopy>;

const KNOWN: readonly string[] = ["online", "busy", "away", "offline"];

function minutesCopy(one: string, many: string, minutes: number): BookingCopy {
  return minutes === 1 ? { key: one } : { key: many, values: { value0: minutes } };
}

/**
 * The best true reason to start, by priority. While the session loads only
 * site-wide facts are known (free mode, else the rate): an offer is never
 * shown to a viewer who may not be able to get it.
 */
export function chatLine(i: ChatLineInput): ChatLine {
  if (i.freeMode) return "free_mode";
  if (i.session === "authenticated" && i.role === "client" && i.freeMinutes > 0) return "own_minutes";
  if (i.session === "anonymous" && i.signupOfferMinutes > 0) return "signup_offer";
  return "rate";
}

/** The line under "Chat now". */
function chatSublabel(i: BookingInput): BookingCopy {
  switch (chatLine(i)) {
    case "free_mode":
      return COPY.free;
    case "own_minutes":
      return minutesCopy("Use your free minute", "Use your {value0} free minutes", i.freeMinutes);
    case "signup_offer":
      return minutesCopy("1 free minute for new members", "{value0} free minutes for new members", i.signupOfferMinutes);
    case "rate":
      return { key: "{value0} credits/min", values: { value0: i.ratePerMinute } };
  }
}

/** The line under the waiting-list label: only a busy advisor has something to say. */
function waitSublabel(status: PresenceStatus): BookingCopy | null {
  return status === "busy" ? COPY.inReading : null;
}

function waitLabel(status: PresenceStatus): BookingCopy {
  return status === "busy" ? COPY.join : COPY.notify;
}

/** The standard booking state of one advisor for one viewer. */
export function bookingState(i: BookingInput): BookingState {
  if (!KNOWN.includes(i.advisorStatus)) {
    return { action: "view", label: COPY.view, sublabel: null, tone: "secondary", disabled: false, reason: "unknown_status" };
  }
  const status = i.advisorStatus as PresenceStatus;
  const online = status === "online";

  // The plain, enabled state of a visitor or a client who may act.
  const base = (action: BookingAction, label: BookingCopy, sublabel: BookingCopy | null, tone: BookingTone): BookingState => ({
    action,
    label,
    sublabel,
    tone,
    disabled: false,
    reason: null,
  });

  if (i.session === "loading") {
    // Same shape as the settled button of this advisor, so nothing jumps; the
    // line is free mode or the rate until the viewer is known.
    const shape = online
      ? base("chat", COPY.chat, chatSublabel(i), "primary")
      : base("join_waitlist", waitLabel(status), waitSublabel(status), "secondary");
    return { ...shape, disabled: true, reason: "session_loading" };
  }

  if (i.session === "authenticated" && i.role !== "client") {
    return { action: "none", label: COPY.advisorAccount, sublabel: null, tone: "muted", disabled: true, reason: "advisor_account" };
  }

  if (i.session === "anonymous") {
    return online
      ? base("signup", COPY.chat, chatSublabel(i), "primary")
      : base("signup", waitLabel(status), waitSublabel(status), "secondary");
  }

  // A signed-in client. One in a reading with this advisor goes back to it
  // (never onto the waiting list for their own reading).
  if (i.inReading) return base("chat", COPY.backToChat, null, "primary");
  const gated = i.age !== "adult";
  const gate = <T extends BookingState>(state: T): T => (gated ? { ...state, reason: "age_gate" } : state);

  if (online) return gate(base("chat", COPY.chat, chatSublabel(i), "primary"));

  if (i.waitlist === "joined") {
    const joined = base("leave_waitlist", COPY.joined, null, "success");
    return i.pending === "leave" ? { ...joined, disabled: true, reason: "pending" } : joined;
  }
  if (i.pending === "join") {
    return { action: "join_waitlist", label: COPY.joining, sublabel: waitSublabel(status), tone: "secondary", disabled: true, reason: "pending" };
  }
  const join = base("join_waitlist", waitLabel(status), waitSublabel(status), "secondary");
  if (i.waitlist === "unknown") return { ...join, disabled: true, reason: "waitlist_loading" };
  return gate(join);
}
