"use client";
/* eslint-disable @next/next/no-img-element */
import { useId, type CSSProperties } from "react";

import { useI18n } from "@/i18n/i18n-provider";
import type { Locale } from "@/i18n/config";
import type { FeaturedAdvisor, PresenceStatus } from "@/lib/api/types";
import { WaitlistProvider } from "@/lib/booking/waitlist-provider";
import { useFeaturedAdvisors } from "@/lib/use-featured-advisors"; // revisions 6.0: live re-sort (§26.3)
import { formatRating } from "@/lib/format";
import { FreeChatBadge } from "@/components/billing/free-chat-badge";
import { BookButton } from "@/components/booking/book-button";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/feedback/error-state";
import { cn } from "@/lib/utils";

/** How many advisors the home page shows. */
export const FEATURED_COUNT = 3;
const imgStar = "https://assets.staryield.net/assets/figma/d9e90.svg";
const imgDivider = "https://assets.staryield.net/assets/figma/7bd7f.svg";
// Two columns (tablets): a last card alone on its row spans both, centred.
const LONE_LAST_CARD = "md:max-lg:[&:last-of-type:nth-of-type(odd)]:col-span-2";

const badgeCopy: Record<PresenceStatus, string> = {
  online: "Active Now",
  busy: "Busy",
  away: "Away",
  offline: "Offline",
};

/** "420+" for 423 reviews, the exact count below ten. */
export function reviewCountLabel(count: number, locale: Locale): string {
  if (count < 10) return count.toLocaleString(locale);
  return `${(Math.floor(count / 10) * 10).toLocaleString(locale)}+`;
}

function CardSkeleton() {
  return (
    <div className={cn("flex w-full max-w-[330px] flex-col gap-5 justify-self-center rounded-[16px] bg-[#e0d5c3] p-6", LONE_LAST_CARD)} aria-hidden>
      <div className="flex items-center justify-between">
        <Skeleton className="size-12 rounded-full" />
        <Skeleton className="h-6 w-20" />
      </div>
      <Skeleton className="h-6 w-1/2" />
      <Skeleton className="h-4 w-3/4" />
      <Skeleton className="h-px w-full" />
      <Skeleton className="h-4 w-full" />
      <Skeleton className="h-4 w-full" />
      <Skeleton className="h-11 w-full" />
    </div>
  );
}

function FeaturedCard({ advisor, onAvailable }: { advisor: FeaturedAdvisor; onAvailable: () => void }) {
  const { text, locale } = useI18n();
  const online = advisor.status === "online";
  // Buttons repeat on every card: they point at the advisor's name.
  const nameId = useId();
  // The card's five parts (top, name and headline, divider, rates, button)
  // sit on rows shared with the cards beside it (subgrid), so the divider,
  // the rates and the buttons line up whatever the length of each headline.
  return (
    <article
      className={cn(
        "relative row-span-5 grid w-full max-w-[330px] grid-rows-subgrid gap-y-5 justify-self-center rounded-[16px] bg-[#e0d5c3] p-6",
        LONE_LAST_CARD,
      )}
      data-testid="featured-advisor"
    >
      <div className="flex w-full flex-col items-start gap-5">
        <div className="flex w-full items-center justify-between">
          <div className="relative size-12 shrink-0 overflow-hidden rounded-full bg-warm-gray">
            {advisor.image_url ? (
              <img alt="" className="absolute inset-0 size-full object-cover" src={advisor.image_url} loading="lazy" decoding="async" />
            ) : null}
          </div>
          <span
            className={
              online
                ? "rounded-[4px] bg-[rgba(170,142,98,0.15)] px-[10px] py-1 font-serif text-[12px] font-semibold text-[#66502e]"
                : "rounded-[4px] bg-[rgba(151,143,110,0.15)] px-[10px] py-1 font-serif text-[12px] font-semibold text-[#5a5440]"
            }
          >
            {text(badgeCopy[advisor.status])}
          </span>
        </div>
        <FreeChatBadge />
      </div>
      <div className="flex w-full flex-col gap-1 text-black" data-testid="featured-advisor-intro">
        <h3 id={nameId} className="font-serif text-[22px] font-semibold">
          {text(advisor.name)}
        </h3>
        <p className="font-serif text-[14px] font-light">{text(advisor.headline)}</p>
      </div>
      <img alt="" aria-hidden className="block h-px w-full" src={imgDivider} data-testid="featured-advisor-divider" />
      <dl className="flex w-full flex-col gap-2 font-serif text-[13px] text-black">
        <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-0.5">
          <dt className="font-light">{text("Consultation Rate ")}</dt>
          <dd className="min-w-0 font-semibold">
            {text("{value0} credits / min", { value0: advisor.rate_per_minute.toLocaleString(locale) })}
          </dd>
        </div>
        <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-0.5">
          <dt className="font-light">{text("Rating ")}</dt>
          <dd className="flex min-w-0 items-center gap-1 font-semibold">
            <img alt="" aria-hidden className="size-3" src={imgStar} />
            {advisor.reviews_count > 0
              ? text("{value0} ({value1} reviews)", {
                  value0: formatRating(advisor.rating, locale),
                  value1: reviewCountLabel(advisor.reviews_count, locale),
                })
              : text("New advisor")}
          </dd>
        </div>
      </dl>
      <BookButton advisor={advisor} placement="home_featured" size="full" describedBy={nameId} onAvailable={onAvailable} />
    </article>
  );
}

function FeaturedCards() {
  const { text } = useI18n();
  // Refreshed while visible, re-sorted and refetched on live status changes.
  const featured = useFeaturedAdvisors(FEATURED_COUNT);
  const { reload, markOnline } = featured;

  const list = featured.data;
  // One column on phones, two on tablets, one per card (up to three) on desktop.
  const columns = list && list.items.length > 0 ? Math.min(list.items.length, FEATURED_COUNT) : FEATURED_COUNT;
  return (
    <div
      className="relative grid w-full grid-cols-[minmax(0,330px)] justify-center gap-6 md:grid-cols-[repeat(2,minmax(0,330px))] lg:grid-cols-[repeat(var(--featured-columns),minmax(0,330px))] lg:gap-x-[105px]"
      style={{ "--featured-columns": columns } as CSSProperties}
      data-name="Layout / Our Psychics / 02"
      aria-busy={featured.loading}
    >
      {list === null ? (
        featured.error && !featured.loading ? (
          <ErrorState
            className="col-span-full w-full max-w-md justify-self-center bg-[#e0d5c3]"
            error={featured.error}
            onRetry={reload}
            title={text("We couldn't load the advisors.")}
          />
        ) : (
          <>
            <p role="status" className="sr-only">
              {text("Loading advisors…")}
            </p>
            {Array.from({ length: FEATURED_COUNT }, (_, i) => (
              <CardSkeleton key={i} />
            ))}
          </>
        )
      ) : list.items.length === 0 ? (
        <p className="col-span-full text-center font-serif text-[16px] text-white">{text("No advisors are listed right now.")}</p>
      ) : (
        list.items.map((advisor) => <FeaturedCard key={advisor.slug} advisor={advisor} onAvailable={() => markOnline(advisor.slug)} />)
      )}
    </div>
  );
}

/**
 * The home page "Psychic readings built around your journey" cards: the
 * advisors who can talk now first, then the best rated by many reviews
 * (GET /psychics/featured). Statuses stay live through realtime events and a
 * refresh every 30 seconds while the page is visible. Each card carries the
 * standard book button (chat, sign up, or the waiting list).
 * The cards share one waiting-list read (and the joined state) through the
 * provider.
 */
export function FeaturedAdvisors() {
  return (
    <WaitlistProvider>
      <FeaturedCards />
    </WaitlistProvider>
  );
}
