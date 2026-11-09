import { describe, expect, it } from "vitest";

import { ApiError } from "@/lib/api/errors";
import { errorCopy } from "@/lib/api/errors";
import type { OffSessionAllowance } from "@/lib/api/types";
import { prepareProblemCopy } from "@/lib/chat/prepare-image";
import { composerAvailability, reasonCopy, type ComposerReason } from "@/components/chat/composer-availability";
import copyEn from "@/i18n/dictionaries/copy-en.json";
import copyFr from "@/i18n/dictionaries/copy-fr.json";
import copyEs from "@/i18n/dictionaries/copy-es.json";

const allowance = (over: Partial<OffSessionAllowance> = {}): OffSessionAllowance => ({
  live: false,
  enabled: true,
  eligible: true,
  unlimited: false,
  limit: 20,
  used: 3,
  remaining: 17,
  ...over,
});

const base = {
  role: "client" as const,
  hasConversation: true,
  live: false,
  requestPending: false,
  allowance: allowance(),
  allowanceLoading: false,
  allowanceError: null,
};

describe("composerAvailability (§22.3)", () => {
  it("is enabled in a live chat (active or paused), whatever the allowance says", () => {
    expect(composerAvailability({ ...base, live: true, allowance: null })).toEqual({ enabled: true, offSession: false });
    expect(composerAvailability({ ...base, live: true, allowance: allowance({ enabled: false, remaining: 0 }) })).toEqual({ enabled: true, offSession: false });
  });

  it("is enabled outside a live chat when enabled && eligible && remaining > 0", () => {
    expect(composerAvailability(base)).toEqual({ enabled: true, offSession: true });
  });

  it("the server saying a live chat exists enables it without a cap", () => {
    expect(composerAvailability({ ...base, allowance: allowance({ live: true, remaining: 0 }) })).toEqual({ enabled: true, offSession: false });
  });

  it("is enabled when unlimited (free mode), even with nothing left", () => {
    expect(composerAvailability({ ...base, allowance: allowance({ unlimited: true, remaining: 0 }) })).toEqual({ enabled: true, offSession: true });
  });

  it("a first-time client with a pending request waits for the live chat", () => {
    const state = composerAvailability({ ...base, requestPending: true, allowance: allowance({ eligible: false }) });
    expect(state).toMatchObject({ enabled: false, reason: "pending_request", canStartChat: false });
    expect(reasonCopy("pending_request", "client")).toBe("You can write once the live chat starts.");
  });

  it("a client who is eligible can write while a request is pending", () => {
    expect(composerAvailability({ ...base, requestPending: true })).toEqual({ enabled: true, offSession: true });
  });

  it("a client who never had a live chat is pointed at the start card (no second button)", () => {
    expect(composerAvailability({ ...base, allowance: allowance({ eligible: false }) })).toMatchObject({ enabled: false, reason: "not_eligible", canStartChat: false });
    expect(composerAvailability({ ...base, hasConversation: false, allowance: null })).toMatchObject({ enabled: false, reason: "no_conversation", canStartChat: false });
  });

  it("the daily cap disables it and offers the start CTA to clients only", () => {
    const capped = allowance({ remaining: 0, used: 20 });
    expect(composerAvailability({ ...base, allowance: capped })).toMatchObject({ enabled: false, reason: "limit_reached", canStartChat: true });
    expect(composerAvailability({ ...base, role: "psychic", allowance: capped })).toMatchObject({ enabled: false, reason: "limit_reached", canStartChat: false });
    expect(reasonCopy("limit_reached", "client")).toBe("You've used today's {value0} messages outside a live chat. Start a live chat to keep talking.");
  });

  it("the feature being off disables it", () => {
    expect(composerAvailability({ ...base, allowance: allowance({ enabled: false, limit: 0, remaining: 0 }) })).toMatchObject({ enabled: false, reason: "disabled" });
  });

  it("loading and failed allowance states are disabled with their own reason; a failure can be retried", () => {
    expect(composerAvailability({ ...base, allowance: null, allowanceLoading: true })).toMatchObject({ enabled: false, reason: "checking", canRetry: false });
    expect(composerAvailability({ ...base, allowance: null, allowanceError: new ApiError({ code: "network_error", message: "" }) })).toMatchObject({
      enabled: false,
      reason: "check_failed",
      canRetry: true,
    });
  });

  it("advisors never get the start CTA and have their own wording", () => {
    const state = composerAvailability({ ...base, role: "psychic", allowance: allowance({ eligible: false }) });
    expect(state).toMatchObject({ enabled: false, canStartChat: false });
    expect(reasonCopy("not_eligible", "psychic")).toBe("You can write once a live chat with this client has started.");
  });
});

describe("copy for the new states is translated (en/fr/es)", () => {
  const reasons: ComposerReason[] = ["no_conversation", "pending_request", "not_eligible", "disabled", "limit_reached", "checking", "check_failed"];
  const strings = [
    ...reasons.flatMap((r) => [reasonCopy(r, "client"), reasonCopy(r, "psychic")]),
    ...Object.values(prepareProblemCopy),
    errorCopy.off_session_disabled,
    errorCopy.off_session_not_eligible,
    errorCopy.off_session_limit_reached,
    errorCopy.recipient_unavailable,
  ];
  it.each([...new Set(strings)])("%s", (key) => {
    expect(copyEn).toHaveProperty([key]);
    expect(copyFr).toHaveProperty([key]);
    expect(copyEs).toHaveProperty([key]);
  });
});
