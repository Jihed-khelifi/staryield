"use client";

import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/i18n/i18n-provider";
import { PackCard } from "@/components/pricing/pack-card";
import { PricingSection } from "@/components/pricing/cta-link";
import { usePricingView } from "@/components/pricing/pricing-context";
import { AcceptedCards } from "@/components/billing/accepted-cards"; // revisions 5.1

/** 3 × 2 from 1200 px, two columns from 768 px, one column below; the API's canonical order always. */
const GRID = "grid grid-cols-1 gap-6 min-[768px]:grid-cols-2 min-[1200px]:grid-cols-3";

function PackSkeleton() {
  return (
    <ul className={GRID} aria-hidden data-testid="pricing-skeleton">
      {Array.from({ length: 6 }, (_, index) => (
        <li key={index}>
          <Skeleton className="h-[580px] w-full rounded-md" />
        </li>
      ))}
    </ul>
  );
}

/** Calm notice instead of the grid: never invented prices. */
function Unavailable({ loading, retry }: { loading: boolean; retry: () => void }) {
  const { text } = useI18n();
  return (
    <div role="status" data-testid="pricing-unavailable" className="mx-auto flex max-w-[640px] flex-col items-center gap-4 rounded-md border border-warm-gray bg-cream p-6 text-center font-serif">
      <p className="text-lg text-ink">{text("Our prices can’t be shown right now. Please try again in a moment.")}</p>
      <Button type="button" variant="cta" size="xl" className="h-12 rounded-md px-6 text-base" onClick={loading ? undefined : retry} aria-disabled={loading || undefined} data-testid="pricing-retry">
        {loading ? text("Retrying…") : text("Retry")}
      </Button>
    </div>
  );
}

/** Figma 1352:2366 — "Credit packs for every kind of reading". */
export function PackGrid() {
  const { text } = useI18n();
  const { summary, status, retry, rate } = usePricingView();
  const reference = summary?.reference ? summary.packs.find((pack) => pack.id === summary.reference?.pack_id) : undefined;
  const savings = Boolean(reference) && Boolean(summary?.packs.some((pack) => pack.badges.some((badge) => badge.type === "save")));

  return (
    <PricingSection aria-labelledby="packs-title" className="bg-cream-soft" innerClassName="flex flex-col items-center gap-6">
      <h2 id="packs-title" className="text-center font-display text-[clamp(1.5rem,3.5vw,2.25rem)] leading-tight text-ink">
        {text("Credit packs for every kind of reading")}
      </h2>
      {rate && (
        <p className="max-w-[850px] text-center font-serif text-[18px] leading-snug text-ink/80">
          {text("Advisors charge between {min} and {max} credits per minute. The total credits in your pack stay the same; your reading time depends on the advisor you choose.", { min: rate.min, max: rate.max })}
        </p>
      )}
      <div className="w-full" aria-busy={status === "loading"}>
        {summary ? (
          <ul className={GRID} data-testid="pack-grid">
            {summary.packs.map((pack) => (
              <li key={pack.id}>
                <PackCard pack={pack} />
              </li>
            ))}
          </ul>
        ) : (
          <div className="flex flex-col gap-6">
            <Unavailable loading={status === "loading"} retry={retry} />
            {status === "loading" && <PackSkeleton />}
          </div>
        )}
      </div>
      {summary && (
        <>
        <p className="max-w-[782px] text-center font-serif text-sm leading-snug text-ink/80" data-testid="pricing-footnote">
          {text("Minute estimates are approximate and depend on the advisor’s displayed rate and applicable billing increments. Credits are added after successful payment. Pack purchases are one-time payments.")}
          {savings && reference && <> {text("Savings compared with the {pack} pack rate.", { pack: reference.name })}</>}
        </p>
        {/* revisions 5.1: card acceptance marks next to the footnote (docs/platform-extensions.md §25.6). */}
        <AcceptedCards label={text("We accept")} className="text-ink/80" />
        </>
      )}
    </PricingSection>
  );
}
