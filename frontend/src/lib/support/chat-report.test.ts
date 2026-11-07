import { describe, expect, it } from "vitest";

import { ApiError } from "@/lib/api/errors";
import { classifyReportError, existingReference, isChatReport, reportConversationHref } from "@/lib/support/chat-report";

const err = (code: string, status: number, extra: Partial<{ retryAfter: number; details: Record<string, unknown> }> = {}) => new ApiError({ code, message: code, status, ...extra });

describe("classifyReportError", () => {
  it.each([
    [err("validation_failed", 422), "fields"],
    [err("rate_limited", 429), "rate_limited"],
    [err("unauthenticated", 401), "sign_in"],
    [err("session_expired", 401), "sign_in"],
    [err("forbidden", 403), "forbidden"],
    [err("not_found", 404), "not_found"],
    [err("network_error", 0), "retry"],
    [err("timeout", 0), "retry"],
    [err("internal_error", 500), "retry"],
    [err("bad_request", 400), "other"],
    [new TypeError("x"), "retry"],
  ])("%#", (error, kind) => expect(classifyReportError(error).kind).toBe(kind));

  it("carries the retry-after seconds and the existing reference", () => {
    expect(classifyReportError(err("rate_limited", 429, { retryAfter: 29.2 })).retryAfter).toBe(30);
    expect(classifyReportError(err("report_exists", 409, { details: { reference: "SY-ABC123" } }))).toEqual({ kind: "exists", reference: "SY-ABC123" });
    expect(classifyReportError(err("report_exists", 409, { details: { reference: "<b>x</b>" } })).reference).toBeUndefined();
    expect(existingReference(undefined)).toBeUndefined();
  });
});

describe("report helpers", () => {
  it("treats rows without kind as contact tickets", () => {
    expect(isChatReport({})).toBe(false);
    expect(isChatReport({ kind: "chat_report" })).toBe(false);
    expect(isChatReport({ kind: "chat_report", report: { reason: "other" } })).toBe(true);
  });
  it("links to the conversation only with a slug, encoded", () => {
    expect(reportConversationHref("fr", { report: { reason: "other", advisor: { name: "T", slug: "a b" } } })).toBe("/fr/chatroom?reader=a%20b");
    expect(reportConversationHref("en", { report: { reason: "other", advisor: { name: "T" } } })).toBeNull();
    expect(reportConversationHref("en", {})).toBeNull();
  });
});
