"use client";
/* eslint-disable @next/next/no-img-element */

import { useId, useMemo } from "react";

import { useI18n } from "@/i18n/i18n-provider";
import type { FeaturedAdvisor } from "@/lib/api/types";
import { WaitlistScope } from "@/lib/booking/waitlist-provider";
import { sortByStatusTier } from "@/lib/psychics-live";
import { useFeaturedAdvisors } from "@/lib/use-featured-advisors";
import { cn } from "@/lib/utils";
import { AdvisorStatusLine } from "@/components/booking/advisor-status-line";
import { BookButton } from "@/components/booking/book-button";
import { ErrorState } from "@/components/feedback/error-state";
import { Skeleton } from "@/components/ui/skeleton";

/** At most this many advisors show in a strip. */
export const STRIP_COUNT = 3;

function StripRow({ advisor, placement, onAvailable }: { advisor: FeaturedAdvisor; placement: string; onAvailable: () => void }) {
  const { text } = useI18n();
  const nameId = useId();
  return (
    <li className="flex flex-wrap items-center gap-x-4 gap-y-3 rounded-[16px] bg-[#e0d5c3] p-4 text-ink" data-testid="advisor-strip-item">
      <div className="relative size-12 shrink-0 overflow-hidden rounded-full bg-warm-gray">
        {advisor.image_url ? <img alt="" className="absolute inset-0 size-full object-cover" src={advisor.image_url} loading="lazy" decoding="async" /> : null}
      </div>
      <div className="min-w-0 flex-1 basis-40">
        <p id={nameId} className="truncate font-serif text-[18px] font-semibold">
          {text(advisor.name)}
        </p>
        <AdvisorStatusLine status={advisor.status} ratePerMinute={advisor.rate_per_minute} className="mt-0.5 text-[13px]" />
      </div>
      <BookButton advisor={advisor} placement={placement} describedBy={nameId} className="w-full sm:w-auto" onAvailable={onAvailable} />
    </li>
  );
}

function StripSkeleton() {
  return (
    <ul className="flex flex-col gap-3" aria-hidden>
      {Array.from({ length: STRIP_COUNT }, (_, i) => (
        <li key={i} className="flex items-center gap-4 rounded-[16px] bg-[#e0d5c3] p-4">
          <Skeleton className="size-12 rounded-full" />
          <div className="flex-1 space-y-2">
            <Skeleton className="h-5 w-1/3" />
            <Skeleton className="h-4 w-1/2" />
          </div>
          <Skeleton className="h-11 w-32" />
        </li>
      ))}
    </ul>
  );
}

/**
 * A compact "talk to an advisor" list (up to three, online first) with the
 * standard book button on each row, for the end of calculator results and
 * landing pages. It reads `GET /psychics/featured` and stays current like the
 * home cards (`useFeaturedAdvisors`: a refresh every 30 s while visible, which
 * is what visitors rely on, plus live status changes for members), and shares
 * one waiting-list read with the buttons (reusing the page's provider when
 * there is one). With `excludeSlug` it is the "other advisors" suggestion after a declined request.
 */
export function AdvisorStrip({
  heading,
  placement = "advisor_strip",
  headingLevel = 2,
  className,
  excludeSlug,
}: {
  /** The section heading (English source copy, translated here). */
  heading: string;
  /** Analytics `placement` of the buttons. */
  placement?: string;
  headingLevel?: 2 | 3;
  className?: string;
  /**
   * An advisor who must not appear (the one who just declined or missed a request): the strip then
   * asks for one advisor more, keeps up to three others who are online, and renders nothing at all
   * (no heading) while loading, on failure or when nobody else is online.
   */
  excludeSlug?: string;
}) {
  const { text } = useI18n();
  const headingId = useId();
  const featured = useFeaturedAdvisors(excludeSlug ? STRIP_COUNT + 1 : STRIP_COUNT);
  const { reload, markOnline } = featured;
  // With an exclusion the strip is the "online now" suggestion: only advisors who can chat, never the excluded one.
  const items = useMemo(() => {
    const all = featured.data?.items ?? [];
    const shown = excludeSlug ? all.filter((a) => a.slug !== excludeSlug && a.status === "online") : all;
    return sortByStatusTier(shown).slice(0, STRIP_COUNT);
  }, [featured.data, excludeSlug]);
  const Heading = headingLevel === 2 ? "h2" : "h3";

  // An optional suggestion never shows an orphan heading, a skeleton or an error.
  if (excludeSlug && (featured.data === null || items.length === 0)) return null;

  return (
    <WaitlistScope>
      <section aria-labelledby={headingId} aria-busy={featured.loading} className={cn("w-full", className)} data-testid="advisor-strip">
        <Heading id={headingId} className="mb-4 font-display text-[24px] text-ink">
          {text(heading)}
        </Heading>
        {featured.data === null ? (
          featured.error && !featured.loading ? (
            <ErrorState compact error={featured.error} onRetry={reload} title={text("We couldn't load the advisors.")} />
          ) : (
            <>
              <p role="status" className="sr-only">
                {text("Loading advisors…")}
              </p>
              <StripSkeleton />
            </>
          )
        ) : items.length === 0 ? (
          <p className="font-serif text-[15px] text-[var(--muted-ink)]">{text("No advisors are listed right now.")}</p>
        ) : (
          <ul className="flex flex-col gap-3">
            {items.map((advisor) => (
              <StripRow key={advisor.slug} advisor={advisor} placement={placement} onAvailable={() => markOnline(advisor.slug)} />
            ))}
          </ul>
        )}
      </section>
    </WaitlistScope>
  );
}
