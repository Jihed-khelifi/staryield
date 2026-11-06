import { describe, expect, it } from "vitest";

import { REVIEW_COMMENT_MAX, reviewSchema } from "@/lib/schemas/review";
import { fieldErrors, identityText as text } from "@/test/schema";

const schema = reviewSchema(text);
const stars = "Choose a rating from 1 to 5 stars.";

describe("reviewSchema", () => {
  it("accepts 1–5 stars with an optional comment", () => {
    for (const rating of [1, 3, 5]) expect(fieldErrors(schema, { rating, comment: "" })).toEqual({});
    expect(fieldErrors(schema, { rating: 4, comment: "Warm and precise." })).toEqual({});
    expect(schema.validateSync({ rating: 4, comment: "   " }).comment).toBe("");
  });

  it.each([0, 6, 3.5, "", null, undefined, "abc"])("rejects the rating %o", (rating) => {
    expect(fieldErrors(schema, { rating, comment: "" }).rating).toBe(stars);
  });

  it("limits the comment to 1,000 characters", () => {
    expect(fieldErrors(schema, { rating: 5, comment: "x".repeat(REVIEW_COMMENT_MAX) })).toEqual({});
    expect(fieldErrors(schema, { rating: 5, comment: "x".repeat(REVIEW_COMMENT_MAX + 1) }).comment).toBe(
      "Use at most 1000 characters."
    );
  });
});
