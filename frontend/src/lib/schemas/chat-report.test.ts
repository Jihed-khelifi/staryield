import { describe, expect, it } from "vitest";

import { CHAT_REPORT_REASONS, chatReportReasonLabels, chatReportSchema, isChatReportReason } from "@/lib/schemas/chat-report";
import { fieldErrors, identityText as text } from "@/test/schema";

const schema = chatReportSchema(text);
const errorsFor = (values: Record<string, unknown>) => fieldErrors(schema, values);

describe("chatReportSchema", () => {
  it("lists the five API reasons, each with a label", () => {
    expect(CHAT_REPORT_REASONS).toEqual(["advisor_unresponsive", "inappropriate", "billing", "technical", "other"]);
    for (const r of CHAT_REPORT_REASONS) expect(chatReportReasonLabels[r]).toBeTruthy();
    expect(isChatReportReason("billing")).toBe(true);
    expect(isChatReportReason("spam")).toBe(false);
  });

  it("requires a reason from the list", () => {
    expect(errorsFor({ details: "" }).reason).toBe("Choose what went wrong.");
    expect(errorsFor({ reason: "spam", details: "" }).reason).toBe("Choose what went wrong.");
  });

  it("accepts any other reason with empty details", () => {
    for (const reason of ["advisor_unresponsive", "inappropriate", "billing", "technical"]) expect(errorsFor({ reason, details: "" })).toEqual({});
  });

  it("requires 10+ characters of details for 'other'", () => {
    expect(errorsFor({ reason: "other", details: "" }).details).toBe("Tell us what happened (at least 10 characters).");
    expect(errorsFor({ reason: "other", details: "   short   " }).details).toBe("Tell us what happened (at least 10 characters).");
    expect(errorsFor({ reason: "other", details: "a".repeat(10) })).toEqual({});
  });

  it("caps details at 2000 characters, counted as characters not bytes", () => {
    expect(errorsFor({ reason: "billing", details: "🌙".repeat(2000) })).toEqual({});
    expect(errorsFor({ reason: "billing", details: "x".repeat(2001) }).details).toBe("Use at most 2000 characters.");
  });
});
