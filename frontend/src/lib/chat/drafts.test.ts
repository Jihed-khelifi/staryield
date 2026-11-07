// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { MESSAGE_MAX_LENGTH } from "@/lib/schemas/message-schema";
import { DRAFT_PREFIX, DRAFT_TTL_MS, clearDraft, clearDrafts, draftKey, loadDraft, pruneDrafts, saveDraft } from "@/lib/chat/drafts";

beforeEach(() => window.localStorage.clear());
afterEach(() => vi.restoreAllMocks());

describe("drafts store", () => {
  it("uses the documented key", () => {
    expect(draftKey("u1", "c1")).toBe("staryield.chat.draft.v1:u1:c1");
    expect(DRAFT_PREFIX).toBe("staryield.chat.draft.v1:");
  });

  it("round-trips text per user and per conversation", () => {
    saveDraft("u1", "c1", "hello");
    saveDraft("u1", "c2", "other");
    saveDraft("u2", "c1", "someone else");
    expect(loadDraft("u1", "c1")).toBe("hello");
    expect(loadDraft("u1", "c2")).toBe("other");
    expect(loadDraft("u2", "c1")).toBe("someone else");
    expect(loadDraft("u1", "reader:theo")).toBe("");
  });

  it("works for an advisor slug key before a conversation exists", () => {
    saveDraft("u1", "reader:theo", "hi Theo");
    expect(loadDraft("u1", "reader:theo")).toBe("hi Theo");
  });

  it("removes the draft when the text is blank or cleared", () => {
    saveDraft("u1", "c1", "x");
    expect(saveDraft("u1", "c1", "   \n")).toBe(false);
    expect(loadDraft("u1", "c1")).toBe("");
    saveDraft("u1", "c1", "x");
    clearDraft("u1", "c1");
    expect(window.localStorage.getItem(draftKey("u1", "c1"))).toBeNull();
  });

  it("expires after seven days and deletes the stale entry", () => {
    const t0 = 1_700_000_000_000;
    saveDraft("u1", "c1", "old", t0);
    expect(loadDraft("u1", "c1", t0 + DRAFT_TTL_MS - 1)).toBe("old");
    expect(loadDraft("u1", "c1", t0 + DRAFT_TTL_MS + 1)).toBe("");
    expect(window.localStorage.getItem(draftKey("u1", "c1"))).toBeNull();
  });

  it("caps text at the message maximum", () => {
    saveDraft("u1", "c1", "a".repeat(MESSAGE_MAX_LENGTH + 500));
    expect(loadDraft("u1", "c1")).toHaveLength(MESSAGE_MAX_LENGTH);
  });

  it("ignores corrupt JSON and wrong shapes, and removes them", () => {
    for (const raw of ["{not json", "null", '{"text":5,"at":1}', '{"text":"x"}', "[]"]) {
      window.localStorage.setItem(draftKey("u1", "c1"), raw);
      expect(loadDraft("u1", "c1")).toBe("");
      expect(window.localStorage.getItem(draftKey("u1", "c1"))).toBeNull();
    }
  });

  it("never throws when storage is full (quota)", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("full", "QuotaExceededError");
    });
    expect(saveDraft("u1", "c1", "text")).toBe(false);
  });

  it("never throws when storage is blocked (private mode / disabled)", () => {
    vi.spyOn(window, "localStorage", "get").mockImplementation(() => {
      throw new DOMException("denied", "SecurityError");
    });
    expect(loadDraft("u1", "c1")).toBe("");
    expect(saveDraft("u1", "c1", "text")).toBe(false);
    expect(() => clearDraft("u1", "c1")).not.toThrow();
    expect(() => pruneDrafts("u1")).not.toThrow();
  });

  it("needs a user and a key", () => {
    expect(saveDraft("", "c1", "x")).toBe(false);
    expect(saveDraft("u1", "", "x")).toBe(false);
    expect(loadDraft("", "c1")).toBe("");
  });

  it("prunes only this user's expired drafts", () => {
    const t0 = 1_700_000_000_000;
    saveDraft("u1", "old", "a", t0);
    saveDraft("u1", "new", "b", t0 + DRAFT_TTL_MS);
    saveDraft("u2", "old", "c", t0);
    pruneDrafts("u1", t0 + DRAFT_TTL_MS + 10);
    expect(window.localStorage.getItem(draftKey("u1", "old"))).toBeNull();
    expect(window.localStorage.getItem(draftKey("u1", "new"))).not.toBeNull();
    expect(window.localStorage.getItem(draftKey("u2", "old"))).not.toBeNull();
  });
  it("clears every draft of one user on sign-out and keeps other users' drafts", () => {
    saveDraft("u1", "c1", "private reading");
    saveDraft("u1", "reader:theo", "draft for Theo");
    saveDraft("u2", "c1", "someone else");
    clearDrafts("u1");
    expect(loadDraft("u1", "c1")).toBe("");
    expect(loadDraft("u1", "reader:theo")).toBe("");
    expect(loadDraft("u2", "c1")).toBe("someone else");
    expect(() => clearDrafts("")).not.toThrow();
  });
});
