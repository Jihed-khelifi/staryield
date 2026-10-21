"use client";

import * as React from "react";
import { Dialog } from "radix-ui";
import { CircleX } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { chatApi, reviewsApi } from "@/lib/api/endpoints";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import type { Review } from "@/lib/api/types";
import { useAsync } from "@/lib/use-async";
import { Button } from "@/components/ui/button";
import { StarDisplay, StarRatingInput } from "@/components/reviews/star-rating";

const COMMENT_MAX = 1000;

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
  const [rating, setRating] = React.useState(0);
  const [comment, setComment] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [apiError, setApiError] = React.useState<ApiError | null>(null);
  const [conflict, setConflict] = React.useState(false);
  const id = React.useId();

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setApiError(null);
    if (rating < 1 || rating > 5) {
      setError(text("Choose a rating from 1 to 5 stars."));
      return;
    }
    setBusy(true);
    try {
      const review = await reviewsApi.submit(sessionId, rating, comment.trim() || undefined);
      onSubmitted(review);
    } catch (err) {
      const e2 = toApiError(err);
      if (e2.code === "conflict") setConflict(true);
      else setApiError(e2);
    } finally {
      setBusy(false);
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
    <form className="review-form" onSubmit={submit} noValidate data-testid="review-form">
      <StarRatingInput
        name={`rating-${id}`}
        value={rating}
        onChange={(n) => {
          setRating(n);
          setError(null);
        }}
        legend={text("How was your chat with {value0}?", { value0: advisorName })}
        disabled={busy}
        invalid={Boolean(error)}
      />
      <label className="field mt-3 text-xs">
        {text("Add a comment (optional)")}
        <textarea
          rows={3}
          maxLength={COMMENT_MAX}
          value={comment}
          onChange={(e) => setComment(e.target.value)}
          disabled={busy}
          placeholder={text("What did you appreciate about this reading?")}
        />
        {comment.length >= COMMENT_MAX * 0.8 ? (
          <span className="self-end font-ui text-[10px] opacity-70">
            {comment.length}/{COMMENT_MAX}
          </span>
        ) : null}
      </label>
      {error || apiError ? (
        <p role="alert" className="mt-2 text-xs text-red-800">
          {error ?? text(errorMessage(apiError))}
        </p>
      ) : null}
      <Button type="submit" className="gold-button mt-3" disabled={busy} aria-busy={busy}>
        {text(busy ? "Sending…" : "Submit review")}
      </Button>
    </form>
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
