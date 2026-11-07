import { describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api/errors";
import type { CustomerTicket } from "@/lib/api/types";
import {
  classifySupportError,
  markClosed,
  normalizeReference,
  supportTokenKey,
  takeSupportToken,
  waitHint,
} from "@/lib/support/support";

const TOKEN = "Zm9vYmFyYmF6cXV4MTIzNDU2Nzg5MC1fQUJDREVGR0g";

function env(hash: string, stored: Record<string, string> = {}) {
  const store = new Map(Object.entries(stored));
  return {
    hash,
    pathname: "/en/support/SY-7K2M9Q",
    search: "?x=1",
    storage: {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: vi.fn((k: string, v: string) => void store.set(k, v)),
      removeItem: vi.fn((k: string) => void store.delete(k)),
    },
    replaceUrl: vi.fn(),
    store,
  };
}

describe("takeSupportToken", () => {
  it("reads #t=, strips the fragment from the address bar and keeps it for the tab", () => {
    const e = env(`#t=${TOKEN}`);
    expect(takeSupportToken("SY-7K2M9Q", e)).toBe(TOKEN);
    expect(e.replaceUrl).toHaveBeenCalledWith("/en/support/SY-7K2M9Q?x=1");
    expect(e.store.get(supportTokenKey("SY-7K2M9Q"))).toBe(TOKEN);
  });

  it("falls back to the tab's stored copy after the fragment is gone (reload)", () => {
    const e = env("", { [supportTokenKey("SY-7K2M9Q")]: TOKEN });
    expect(takeSupportToken("SY-7K2M9Q", e)).toBe(TOKEN);
    expect(e.replaceUrl).not.toHaveBeenCalled();
  });

  it("strips but ignores a malformed token and never stores it", () => {
    for (const bad of ["#t=<script>", "#t=short", "#other=1", "#t="]) {
      const e = env(bad);
      expect(takeSupportToken("SY-7K2M9Q", e), bad).toBeNull();
      expect(e.replaceUrl).toHaveBeenCalled();
      expect(e.storage.setItem).not.toHaveBeenCalled();
    }
  });

  it("returns null without a fragment or stored copy, and survives unusable storage", () => {
    expect(takeSupportToken("SY-7K2M9Q", env(""))).toBeNull();
    const broken = { ...env(`#t=${TOKEN}`), storage: { getItem: () => { throw new Error("blocked"); }, setItem: () => { throw new Error("blocked"); }, removeItem: () => {} } };
    expect(takeSupportToken("SY-7K2M9Q", broken)).toBe(TOKEN);
    expect(takeSupportToken("SY-7K2M9Q", { ...broken, hash: "" })).toBeNull();
  });

  it("scopes the stored token to its reference", () => {
    const e = env("", { [supportTokenKey("SY-OTHER1")]: TOKEN });
    expect(takeSupportToken("SY-7K2M9Q", e)).toBeNull();
  });
});

describe("normalizeReference", () => {
  it("accepts references case-insensitively and rejects everything else", () => {
    expect(normalizeReference("sy-7k2m9q")).toBe("SY-7K2M9Q");
    for (const bad of ["requests", "SY-", "SY-12", "../x", "SY-7K2M9Q/../a", ""]) expect(normalizeReference(bad), bad).toBeNull();
  });
});

describe("classifySupportError", () => {
  const err = (code: string, extra: Partial<{ status: number; retryAfter: number }> = {}) => new ApiError({ code, message: code, status: extra.status ?? 400, retryAfter: extra.retryAfter });
  it("maps the documented codes", () => {
    expect(classifySupportError(err("invalid_state", { status: 409 }), "account").kind).toBe("closed");
    expect(classifySupportError(err("not_found", { status: 404 }), "link").kind).toBe("link_invalid");
    expect(classifySupportError(err("not_found", { status: 404 }), "account").kind).toBe("not_found");
    expect(classifySupportError(err("rate_limited", { status: 429, retryAfter: 12.2 }), "account")).toEqual({ kind: "rate_limited", retryAfter: 13 });
    expect(classifySupportError(err("rate_limited", { status: 429 }), "link")).toEqual({ kind: "rate_limited", retryAfter: undefined });
    expect(classifySupportError(err("session_expired", { status: 401 }), "account").kind).toBe("sign_in");
    expect(classifySupportError(err("internal_error", { status: 500 }), "link").kind).toBe("other");
    expect(classifySupportError(new Error("boom"), "link").kind).toBe("other");
  });
});

describe("helpers", () => {
  it("waitHint", () => {
    expect(waitHint(undefined)).toEqual({ unit: "unknown", value: 0 });
    expect(waitHint(30)).toEqual({ unit: "seconds", value: 30 });
    expect(waitHint(600)).toEqual({ unit: "minutes", value: 10 });
  });
  it("markClosed", () => {
    const t = { reference: "SY-1", status: "open", awaiting: "support", can_reply: true, messages: [] } as unknown as CustomerTicket;
    expect(markClosed(t)).toMatchObject({ status: "closed", awaiting: "none", can_reply: false });
  });
});
