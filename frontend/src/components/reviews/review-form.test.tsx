// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { ApiError } from "@/lib/api/errors";
import { axeViolations, renderWithI18n } from "@/test/render";
import { ReviewForm } from "@/components/reviews/review-form";

const submitReview = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api/endpoints", () => ({ reviewsApi: { submit: submitReview }, chatApi: { summary: vi.fn() } }));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

function setup() {
  const onSubmitted = vi.fn();
  renderWithI18n(<ReviewForm sessionId="s1" advisorName="Luna" onSubmitted={onSubmitted} />);
  return { onSubmitted, form: screen.getByTestId("review-form"), u: userEvent.setup() };
}

describe("ReviewForm", () => {
  it("asks for a rating before sending", async () => {
    const { form, u } = setup();
    await u.click(screen.getByRole("button", { name: "Submit review" }));
    expect((await within(form).findByRole("alert")).textContent).toBe("Choose a rating from 1 to 5 stars.");
    expect(submitReview).not.toHaveBeenCalled();
    expect(await axeViolations()).toEqual([]);
  });

  it("sends the stars and the trimmed comment", async () => {
    const { form, onSubmitted, u } = setup();
    const review = { id: "r1", rating: 4, comment: "Warm and precise." };
    submitReview.mockResolvedValue(review);
    await u.click(within(form).getByRole("radio", { name: "4 out of 5 stars" }));
    await u.type(within(form).getByRole("textbox"), "  Warm and precise.  ");
    await u.click(screen.getByRole("button", { name: "Submit review" }));
    await waitFor(() => expect(onSubmitted).toHaveBeenCalledWith(review));
    expect(submitReview).toHaveBeenCalledWith("s1", 4, "Warm and precise.");
  });

  it("limits the comment to 1,000 characters", async () => {
    const { form, u } = setup();
    await u.click(within(form).getByRole("radio", { name: "5 out of 5 stars" }));
    const box = within(form).getByRole("textbox");
    await u.click(box);
    await u.paste("x".repeat(1001));
    await u.click(screen.getByRole("button", { name: "Submit review" }));
    expect(await screen.findByText("Use at most 1000 characters.")).toBeTruthy();
    expect(screen.getByText("1001/1000")).toBeTruthy();
    expect(submitReview).not.toHaveBeenCalled();
  });

  it("treats an existing review as already rated and shows other errors", async () => {
    const { form, u } = setup();
    submitReview.mockRejectedValueOnce(new ApiError({ code: "internal_error", message: "boom", status: 500 }));
    await u.click(within(form).getByRole("radio", { name: "3 out of 5 stars" }));
    await u.click(screen.getByRole("button", { name: "Submit review" }));
    expect((await within(form).findByRole("alert")).textContent).toContain("Something went wrong on our side.");

    submitReview.mockRejectedValueOnce(new ApiError({ code: "conflict", message: "exists", status: 409 }));
    await u.click(screen.getByRole("button", { name: "Submit review" }));
    expect((await screen.findByRole("status")).textContent).toBe("You have already rated this chat. Thank you!");
  });
});
