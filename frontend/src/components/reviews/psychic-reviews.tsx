"use client";
/* eslint-disable @next/next/no-img-element */
import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { reviewsApi } from "@/lib/api/endpoints";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import type { Page, Review } from "@/lib/api/types";
import { formatDate } from "@/lib/format";
import { useAsync } from "@/lib/use-async";
import { Button } from "@/components/ui/button";
import { StarDisplay } from "@/components/reviews/star-rating";

export function ReviewItem({ review }: { review: Review }) {
  const { locale } = useI18n();
  const name = review.author?.name || "—";
  return (
    <li className="review-item">
      <span className="review-avatar" aria-hidden>
        {review.author?.avatar_url ? <img src={review.author.avatar_url} alt="" className="size-full object-cover" /> : name.charAt(0).toUpperCase()}
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
          <p className="font-semibold">{name}</p>
          <time className="text-xs font-light" dateTime={review.created_at}>
            {formatDate(review.created_at, locale)}
          </time>
        </div>
        <StarDisplay rating={review.rating} className="mt-1" />
        {review.comment ? <p className="mt-2 text-sm leading-relaxed font-light whitespace-pre-wrap">{review.comment}</p> : null}
      </div>
    </li>
  );
}

/** Public, visible reviews of an advisor with "load more" (§9.2). */
export function PsychicReviews({ slug, total }: { slug: string; total?: number }) {
  const { text, locale } = useI18n();
  const first = useAsync((signal) => reviewsApi.forPsychic(slug, undefined, 10, signal), [slug]);
  const [more, setMore] = React.useState<Page<Review>[]>([]);
  const [busy, setBusy] = React.useState(false);
  const [moreError, setMoreError] = React.useState<ApiError | null>(null);
  const items = [...(first.data?.items ?? []), ...more.flatMap((p) => p.items)];
  const cursor = more.length ? more.at(-1)!.next_cursor : first.data?.next_cursor;

  async function loadMore() {
    if (!cursor) return;
    setBusy(true);
    setMoreError(null);
    try {
      const page = await reviewsApi.forPsychic(slug, cursor, 10);
      setMore((m) => [...m, page]);
    } catch (err) {
      setMoreError(toApiError(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="psychic-reviews" aria-labelledby="reviews-heading" data-testid="psychic-reviews">
      <div className="flex flex-wrap items-baseline justify-between gap-3">
        <h2 id="reviews-heading" className="font-display text-[32px]">
          {text("Client reviews")}
        </h2>
        {typeof total === "number" && total > 0 ? <p className="text-sm font-light">{text("{count} reviews", { count: total.toLocaleString(locale) })}</p> : null}
      </div>
      {first.error ? (
        <div role="alert" className="mt-4 text-sm">
          <p>{text(errorMessage(first.error))}</p>
          <button type="button" className="mt-2 underline" onClick={first.reload}>
            {text("Try again")}
          </button>
        </div>
      ) : first.loading && !first.data ? (
        <p className="mt-4 text-sm" role="status">
          {text("Loading reviews…")}
        </p>
      ) : items.length === 0 ? (
        <p className="mt-4 text-sm font-light">{text("No reviews yet. Clients can rate their chat once it ends.")}</p>
      ) : (
        <>
          <ul className="review-list">
            {items.map((r) => (
              <ReviewItem key={r.id} review={r} />
            ))}
          </ul>
          {moreError ? (
            <p role="alert" className="mt-3 text-sm">
              {text(errorMessage(moreError))}
            </p>
          ) : null}
          {cursor ? (
            <div className="mt-6 text-center">
              <Button variant="outline" className="h-10 border-gold bg-transparent px-6" onClick={() => void loadMore()} disabled={busy} aria-busy={busy}>
                {text(busy ? "Loading…" : "Load more reviews")}
              </Button>
            </div>
          ) : null}
        </>
      )}
    </section>
  );
}
