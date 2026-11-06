import { describe, expect, it } from "vitest";

import { contactSchema, isSupportTopic, SUPPORT_TOPICS, supportTopicLabels, type ContactValues } from "@/lib/schemas/contact";
import { fieldErrors, identityText as text } from "@/test/schema";

const schema = contactSchema(text);
const valid: ContactValues = {
  topic: "privacy",
  subject: "Export my data",
  email: "jane@example.com",
  message: "Please send me a copy of my data.",
  consent: true,
};
const errorsFor = (patch: Partial<Record<keyof ContactValues, unknown>>) => fieldErrors(schema, { ...valid, ...patch });

describe("contactSchema", () => {
  it("accepts a complete request", () => {
    expect(errorsFor({})).toEqual({});
    expect(errorsFor({ subject: "  Hey  ", message: `  ${"🌙".repeat(10)}  ` })).toEqual({});
  });

  it.each([
    [{ topic: "sales" }, "topic", "Choose a topic from the list."],
    [{ subject: "  " }, "subject", "This field is required."],
    [{ subject: "ab" }, "subject", "Use at least 3 characters."],
    [{ subject: "x".repeat(121) }, "subject", "Use at most 120 characters."],
    [{ email: "jane" }, "email", "Enter a valid email address."],
    [{ message: "Too short" }, "message", "Use at least 10 characters."],
    [{ message: "x".repeat(5001) }, "message", "Use at most 5000 characters."],
    [{ consent: false }, "consent", "Please confirm to continue."],
  ] as const)("rejects %o", (patch, field, message) => {
    expect(errorsFor(patch)[field]).toBe(message);
  });
});

describe("support topics", () => {
  it("match the API allow-list and all have labels", () => {
    expect([...SUPPORT_TOPICS]).toEqual(["general", "privacy", "billing", "advisor", "technical"]);
    expect(Object.keys(supportTopicLabels).sort()).toEqual([...SUPPORT_TOPICS].sort());
  });

  it("recognises only known topics (used for ?topic=)", () => {
    expect(isSupportTopic("privacy")).toBe(true);
    expect(isSupportTopic("PRIVACY")).toBe(false);
    expect(isSupportTopic("")).toBe(false);
    expect(isSupportTopic(null)).toBe(false);
  });
});
