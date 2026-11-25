import { describe, expect, it } from "vitest";

import type { AgeStatus, PresenceStatus, Role } from "@/lib/api/types";
import { bookingState, chatLine, type BookingAction, type BookingInput, type BookingState } from "@/lib/booking/booking-state";

const base: BookingInput = {
  advisorStatus: "online",
  ratePerMinute: 30,
  session: "authenticated",
  role: "client",
  freeMinutes: 0,
  signupOfferMinutes: 0,
  freeMode: false,
  waitlist: "not_joined",
  pending: null,
  age: "adult",
};
const state = (over: Partial<BookingInput>) => bookingState({ ...base, ...over });
const copy = (s: BookingState) => [s.label.key, s.sublabel?.key ?? null];

describe("bookingState: advisor online", () => {
  it("lets a client chat, with the rate when nothing better applies", () => {
    expect(state({})).toEqual({
      action: "chat",
      label: { key: "Chat now" },
      sublabel: { key: "{value0} credits/min", values: { value0: 30 } },
      tone: "primary",
      disabled: false,
      reason: null,
    });
  });

  it("picks the sub-line by priority: free mode, own free minutes, new-member offer, rate", () => {
    // Free mode beats everything.
    expect(copy(state({ freeMode: true, freeMinutes: 5, signupOfferMinutes: 10 }))[1]).toBe("Free for a limited time");
    expect(copy(state({ session: "anonymous", role: null, freeMode: true, signupOfferMinutes: 10 }))[1]).toBe("Free for a limited time");
    // A client with free minutes uses them; the sign-up offer is not for existing members.
    expect(state({ freeMinutes: 5, signupOfferMinutes: 10 }).sublabel).toEqual({ key: "Use your {value0} free minutes", values: { value0: 5 } });
    expect(state({ freeMinutes: 1 }).sublabel).toEqual({ key: "Use your free minute" });
    expect(state({ freeMinutes: 0, signupOfferMinutes: 10 }).sublabel?.key).toBe("{value0} credits/min");
    // A visitor sees the new-member offer.
    expect(state({ session: "anonymous", role: null, signupOfferMinutes: 10 }).sublabel).toEqual({
      key: "{value0} free minutes for new members",
      values: { value0: 10 },
    });
    expect(state({ session: "anonymous", role: null, signupOfferMinutes: 1 }).sublabel).toEqual({ key: "1 free minute for new members" });
    // Offer off: the rate. A visitor's wallet minutes are ignored.
    expect(state({ session: "anonymous", role: null, freeMinutes: 9 }).sublabel).toEqual({ key: "{value0} credits/min", values: { value0: 30 } });
  });

  it("sends a visitor to sign up (then the chat) instead of a chatroom they cannot use", () => {
    const s = state({ session: "anonymous", role: null });
    expect(s.action).toBe("signup");
    expect(s.label).toEqual({ key: "Chat now" });
    expect(s.tone).toBe("primary");
    expect(s.disabled).toBe(false);
  });

  it("keeps chat for a client who has joined the list before the advisor came online", () => {
    expect(state({ waitlist: "joined" }).action).toBe("chat");
    expect(state({ waitlist: "unknown" }).disabled).toBe(false);
    expect(state({ pending: "leave", waitlist: "joined" }).action).toBe("chat");
  });
});

describe("bookingState: advisor not online", () => {
  it("offers the waiting list for a busy advisor", () => {
    expect(state({ advisorStatus: "busy" })).toEqual({
      action: "join_waitlist",
      label: { key: "Join the waiting list" },
      sublabel: { key: "In a reading" },
      tone: "secondary",
      disabled: false,
      reason: null,
    });
  });

  it.each(["away", "offline"] as const)("offers to notify the client when a %s advisor is online", (advisorStatus) => {
    expect(state({ advisorStatus })).toMatchObject({
      action: "join_waitlist",
      label: { key: "Notify me when online" },
      sublabel: null,
      tone: "secondary",
      disabled: false,
      reason: null,
    });
  });

  it("shows the joined state, which leaves the list when selected", () => {
    for (const advisorStatus of ["busy", "away", "offline"] as const) {
      expect(state({ advisorStatus, waitlist: "joined" })).toEqual({
        action: "leave_waitlist",
        label: { key: "✓ On the waiting list" },
        sublabel: null,
        tone: "success",
        disabled: false,
        reason: null,
      });
    }
  });

  it("disables the button while a request is in flight, keeping the label stable", () => {
    expect(state({ advisorStatus: "busy", pending: "join" })).toMatchObject({
      action: "join_waitlist",
      label: { key: "Joining…" },
      disabled: true,
      reason: "pending",
    });
    expect(state({ advisorStatus: "busy", waitlist: "joined", pending: "leave" })).toMatchObject({
      action: "leave_waitlist",
      label: { key: "✓ On the waiting list" },
      disabled: true,
      reason: "pending",
    });
  });

  it("waits for the list before it says join or joined", () => {
    expect(state({ advisorStatus: "offline", waitlist: "unknown" })).toMatchObject({
      action: "join_waitlist",
      disabled: true,
      reason: "waitlist_loading",
    });
  });

  it("asks a visitor to sign up first, with the same wording", () => {
    expect(state({ session: "anonymous", role: null, advisorStatus: "busy" })).toMatchObject({
      action: "signup",
      label: { key: "Join the waiting list" },
      sublabel: { key: "In a reading" },
      tone: "secondary",
      disabled: false,
    });
    expect(state({ session: "anonymous", role: null, advisorStatus: "offline" })).toMatchObject({
      action: "signup",
      label: { key: "Notify me when online" },
    });
  });
});

describe("bookingState: accounts and gates", () => {
  it.each(["psychic", "admin"] as const)("gives %s accounts no booking action, and says why", (role) => {
    for (const advisorStatus of ["online", "busy", "away", "offline"] as const) {
      expect(state({ role, advisorStatus })).toEqual({
        action: "none",
        label: { key: "Advisor accounts can't book readings" },
        sublabel: null,
        tone: "muted",
        disabled: true,
        reason: "advisor_account",
      });
    }
  });

  it("treats a signed-in account without a role like a non-client", () => {
    expect(state({ role: null }).action).toBe("none");
  });

  it("holds the button while the session loads, looking like the settled one", () => {
    const online = state({ session: "loading", role: null });
    expect(online).toMatchObject({ action: "chat", label: { key: "Chat now" }, disabled: true, reason: "session_loading", tone: "primary" });
    // No account is assumed either way: neither the wallet's minutes nor the
    // new-member offer (an existing client may be loading), only the rate...
    expect(state({ session: "loading", role: null, freeMinutes: 5, signupOfferMinutes: 10 }).sublabel).toEqual({ key: "{value0} credits/min", values: { value0: 30 } });
    // ...or free mode, which is true for everyone.
    expect(state({ session: "loading", role: null, freeMode: true, signupOfferMinutes: 10 }).sublabel).toEqual({ key: "Free for a limited time" });
    expect(state({ session: "loading", role: null, advisorStatus: "away" })).toMatchObject({
      action: "join_waitlist",
      label: { key: "Notify me when online" },
      disabled: true,
      reason: "session_loading",
    });
  });

  it.each(["birth_date_required", "restricted"] as const)("hands a client with the age state %s to the age flow", (age) => {
    expect(state({ age })).toMatchObject({ action: "chat", disabled: false, reason: "age_gate" });
    expect(state({ age, advisorStatus: "busy" })).toMatchObject({ action: "join_waitlist", disabled: false, reason: "age_gate" });
    // Leaving a list is not gated.
    expect(state({ age, advisorStatus: "busy", waitlist: "joined" })).toMatchObject({ action: "leave_waitlist", reason: null });
  });

  it("ignores the age of visitors and advisor accounts", () => {
    expect(state({ session: "anonymous", role: null, age: "restricted" }).reason).toBeNull();
    expect(state({ role: "psychic", age: "restricted" }).reason).toBe("advisor_account");
  });

  it("takes a client in a reading with the advisor back to it, never onto the waiting list for their own reading", () => {
    const back = { action: "chat", label: { key: "Back to chat" }, sublabel: null, tone: "primary", disabled: false, reason: null };
    for (const advisorStatus of ["busy", "away", "offline", "online"] as const) expect(state({ advisorStatus, inReading: true })).toEqual(back);
    // Whatever the list or the age step says: they are already in the reading.
    expect(state({ advisorStatus: "busy", inReading: true, waitlist: "joined", age: "restricted" })).toEqual(back);
    // Only a signed-in client can be in a reading.
    expect(state({ advisorStatus: "busy", inReading: true, session: "anonymous", role: null }).action).toBe("signup");
    expect(state({ advisorStatus: "busy", inReading: true, role: "psychic" }).action).toBe("none");
  });

  it("falls back to the profile for a status it does not know", () => {
    expect(state({ advisorStatus: "dnd" })).toMatchObject({ action: "view", label: { key: "View profile" }, reason: "unknown_status", disabled: false });
    expect(state({ advisorStatus: "dnd", role: "psychic" }).action).toBe("view");
  });
});

describe("bookingState: every combination", () => {
  const statuses: (PresenceStatus | "dnd")[] = ["online", "busy", "away", "offline", "dnd"];
  const sessions: [BookingInput["session"], Role | null][] = [
    ["loading", null],
    ["anonymous", null],
    ["authenticated", "client"],
    ["authenticated", "psychic"],
    ["authenticated", "admin"],
    ["authenticated", null],
  ];
  const ages: AgeStatus[] = ["adult", "birth_date_required", "restricted"];
  const all: { input: BookingInput; out: BookingState }[] = [];
  for (const advisorStatus of statuses)
    for (const [session, role] of sessions)
      for (const age of ages)
        for (const waitlist of ["joined", "not_joined", "unknown"] as const)
          for (const pending of [null, "join", "leave"] as const)
            for (const freeMode of [false, true])
              for (const freeMinutes of [0, 1, 7])
                for (const signupOfferMinutes of [0, 1, 10])
                  for (const inReading of [false, true]) {
                    const input: BookingInput = { advisorStatus, ratePerMinute: 25, session, role, freeMinutes, signupOfferMinutes, freeMode, waitlist, pending, age, inReading };
                    all.push({ input, out: bookingState(input) });
                  }

  it("covers a large grid and is deterministic", () => {
    expect(all.length).toBe(5 * 6 * 3 * 3 * 3 * 2 * 3 * 3 * 2);
    for (const { input, out } of all.slice(0, 500)) expect(bookingState(input)).toEqual(out);
  });

  it("always returns a complete, translatable state", () => {
    const actions: BookingAction[] = ["chat", "signup", "join_waitlist", "leave_waitlist", "view", "none"];
    for (const { out } of all) {
      expect(actions).toContain(out.action);
      expect(out.label.key.length).toBeGreaterThan(0);
      expect(["primary", "secondary", "success", "muted"]).toContain(out.tone);
      // Every placeholder in a key has its value.
      for (const c of [out.label, out.sublabel]) {
        if (!c) continue;
        const names = [...c.key.matchAll(/\{(\w+)\}/g)].map((m) => m[1]);
        expect(Object.keys(c.values ?? {}).sort()).toEqual(names.sort());
        for (const v of Object.values(c.values ?? {})) expect(Number.isFinite(v)).toBe(true);
      }
      // A disabled state always says why; an enabled one never does, except the age hand-over.
      if (out.disabled) expect(out.reason).not.toBeNull();
      else expect([null, "age_gate", "unknown_status"]).toContain(out.reason);
    }
  });

  it("never offers an anonymous visitor or an advisor account a join or leave request", () => {
    for (const { input, out } of all) {
      if (input.session === "authenticated" && input.role === "client") continue;
      expect(["join_waitlist", "leave_waitlist"].includes(out.action) && input.session !== "loading").toBe(false);
    }
  });

  it("only offers the chat for an online advisor (or the client's own reading), and the waiting list for the others", () => {
    for (const { input, out } of all) {
      const client = input.session === "authenticated" && input.role === "client";
      if (out.action === "chat") expect(input.advisorStatus === "online" || (client && input.inReading)).toBe(true);
      if (client && input.inReading && out.action !== "view") expect(out.action).toBe("chat");
      if (out.action === "join_waitlist" || out.action === "leave_waitlist") expect(input.advisorStatus).not.toBe("online");
    }
  });

  it("never promises free minutes a viewer cannot get", () => {
    for (const { input, out } of all) {
      const line = out.sublabel?.key ?? "";
      if (line.startsWith("Use your")) {
        expect(input.session === "authenticated" && input.role === "client" && input.freeMinutes > 0 && !input.freeMode).toBe(true);
      }
      if (line.includes("for new members")) {
        expect(input.session).toBe("anonymous");
        expect(input.signupOfferMinutes).toBeGreaterThan(0);
        expect(input.freeMode).toBe(false);
      }
      if (line === "Free for a limited time") expect(input.freeMode).toBe(true);
    }
  });
});

describe("chatLine", () => {
  const viewer = { freeMode: false, session: "authenticated", role: "client", freeMinutes: 0, signupOfferMinutes: 10 } as const;
  it("names the line under 'Chat now' by the same priority as the button", () => {
    expect(chatLine({ ...viewer, freeMode: true, freeMinutes: 5 })).toBe("free_mode");
    expect(chatLine({ ...viewer, freeMinutes: 5 })).toBe("own_minutes");
    expect(chatLine(viewer)).toBe("rate");
    expect(chatLine({ ...viewer, session: "anonymous", role: null })).toBe("signup_offer");
    expect(chatLine({ ...viewer, session: "anonymous", role: null, signupOfferMinutes: 0 })).toBe("rate");
    expect(chatLine({ ...viewer, session: "loading", role: null, freeMinutes: 5 })).toBe("rate");
    expect(chatLine({ ...viewer, role: "psychic", freeMinutes: 5 })).toBe("rate");
  });
});
