"use client";

import * as React from "react";
import { Dialog } from "radix-ui";
import { CircleX } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { chatApi, reviewsApi } from "@/lib/api/endpoints";
import { errorMessage, toApiError } from "@/lib/api/errors";
import type { Review } from "@/lib/api/types";
import { applyApiError } from "@/lib/forms/server-errors";
import { keepFocusOnPress } from "@/lib/forms/submit-press";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { REVIEW_COMMENT_MAX, reviewSchema, type ReviewValues } from "@/lib/schemas/review";
import { useAsync } from "@/lib/use-async";
import { Button } from "@/components/ui/button";
import { FieldRow } from "@/components/ui/field";
import { Form, FormField, FormRootError } from "@/components/ui/form";
import { StarDisplay, StarRatingInput } from "@/components/reviews/star-rating";

/** "Thank you" state once a session has been rated. */
export function ReviewThanks({ review }: { review: Review }) {
  const { text } = useI18n();
  return (
    <div className="review-thanks" role="status" data-testid="review-thanks">
      <p className="font-semibold">{text("Thank you for your review!")}</p>
      <StarDisplay rating={review.rating} className="mt-1" />
      {review.comment ? <p className="mt-1 text-xs whitespace-pre-wrap">{review.comment}</p> : null}
    </div>
  );
}

/** Rate a finished session: 1–5 stars and an optional comment (§9.2). */
export function ReviewForm({
  sessionId,
  advisorName,
  onSubmitted,
}: {
  sessionId: string;
  advisorName: string;
  onSubmitted: (review: Review) => void;
}) {
  const { text } = useI18n();
  const form = useSchemaForm(reviewSchema, { defaultValues: { rating: 0, comment: "" } });
  const [conflict, setConflict] = React.useState(false);
  const id = React.useId();
  const busy = form.formState.isSubmitting;

  async function submit(values: ReviewValues) {
    try {
      const review = await reviewsApi.submit(sessionId, values.rating, values.comment?.trim() || undefined);
      onSubmitted(review);
    } catch (err) {
      const e = toApiError(err);
      if (e.code === "conflict") setConflict(true);
      else applyApiError(form, err, { text });
    }
  }

  if (conflict) {
    return (
      <p className="review-thanks" role="status">
        {text("You have already rated this chat. Thank you!")}
      </p>
    );
  }

  return (
    <Form {...form}>
      <form className="review-form" onSubmit={form.handleSubmit(submit)} noValidate data-testid="review-form">
        <FormField
          control={form.control}
          name="rating"
          render={({ field, fieldState }) => (
            <div>
              <StarRatingInput
                name={`rating-${id}`}
                value={field.value ?? 0}
                onChange={(n) => field.onChange(n)}
                legend={text("How was your chat with {value0}?", { value0: advisorName })}
                disabled={busy}
                invalid={Boolean(fieldState.error)}
              />
              {fieldState.error ? (
                <p role="alert" className="field-message mt-2" data-tone="error">
                  {fieldState.error.message}
                </p>
              ) : null}
            </div>
          )}
        />
        <FieldRow control={form.control} name="comment" label={text("Add a comment (optional)")} maxLength={REVIEW_COMMENT_MAX} className="mt-3 text-xs" showValid={false}>
          {(field) => (
            <textarea
              {...field}
              rows={3}
              disabled={busy}
              placeholder={text("What did you appreciate about this reading?")}
            />
          )}
        </FieldRow>
        <FormRootError className="mt-2" />
        <Button type="submit" className="gold-button mt-3" disabled={busy} aria-busy={busy} onMouseDown={keepFocusOnPress}>
          {text(busy ? "Sending…" : "Submit review")}
        </Button>
      </form>
    </Form>
  );
}

/**
 * Rating dialog opened from the `chat.review_prompt` notification
 * (`/chatroom?reader=<slug>&review=<session_id>`).
 */
export function ReviewDialog({ sessionId, open, onOpenChange }: { sessionId: string; open: boolean; onOpenChange: (open: boolean) => void }) {
  const { text } = useI18n();
  const summary = useAsync(() => chatApi.summary(sessionId), [sessionId], open);
  const [submitted, setSubmitted] = React.useState<Review | null>(null);
  const data = summary.data;
  const advisor = data?.session.psychic.name ?? "";
  const review = submitted ?? data?.review ?? null;

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="planet-overlay" />
        <Dialog.Content className="review-dialog" data-testid="review-dialog">
          <div className="flex items-start justify-between gap-3">
            <Dialog.Title className="font-display text-lg">{advisor ? text("Rate your chat with {value0}", { value0: advisor }) : text("Rate your chat")}</Dialog.Title>
            <Dialog.Close asChild>
              <button type="button" aria-label={text("Close")}>
                <CircleX size={20} aria-hidden />
              </button>
            </Dialog.Close>
          </div>
          <Dialog.Description className="text-xs font-light">
            {text("Your rating helps other clients and helps advisors grow. Only your first name is shown.")}
          </Dialog.Description>
          {summary.error ? (
            <p role="alert" className="text-sm">
              {text(summary.error.code === "not_found" || summary.error.code === "forbidden" ? "This chat could not be found." : errorMessage(summary.error))}
            </p>
          ) : summary.loading || !data ? (
            <p className="text-sm" role="status">
              {text("Loading…")}
            </p>
          ) : review ? (
            <ReviewThanks review={review} />
          ) : data.can_review ? (
            <ReviewForm sessionId={sessionId} advisorName={advisor} onSubmitted={setSubmitted} />
          ) : (
            <p className="text-sm" role="status">
              {text("This chat can no longer be rated.")}
            </p>
          )}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
