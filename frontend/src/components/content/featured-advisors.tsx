"use client";
/* eslint-disable @next/next/no-img-element */
import { useCallback, useEffect, useId, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import type { Locale } from "@/i18n/config";
import { psychicsApi, waitlistApi } from "@/lib/api/endpoints";
import { toApiError } from "@/lib/api/errors";
import type { FeaturedAdvisor, FeaturedList, PresenceStatus } from "@/lib/api/types";
import { useRequireAdult } from "@/lib/auth/age-gate"; // revisions 5.0: 18+ age gate (§24.6)
import { useRealtimeEvent, useSession } from "@/lib/auth/session-provider";
import { useAsync } from "@/lib/use-async";
import { formatRating } from "@/lib/format";
import { loginUrl } from "@/lib/navigation";
import { track } from "@/lib/analytics";
import { FreeChatBadge } from "@/components/billing/free-chat-badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/feedback/error-state";
import { useToast } from "@/components/feedback/toast-provider";
import { announce } from "@/components/feedback/live-region";

/** How many advisors the home page shows, and how often it refreshes. */
export const FEATURED_COUNT = 3;
const REFRESH_MS = 30_000;
const imgStar = "https://assets.staryield.net/assets/figma/d9e90.svg";
const imgDivider = "https://assets.staryield.net/assets/figma/7bd7f.svg";
const imgPaper = "https://assets.staryield.net/assets/figma/fb9ee.jpg";

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

/** Statuses can change between refreshes: the call to action follows them. */
function withStatus(list: FeaturedList, slug: string, status: PresenceStatus): FeaturedList {
  return {
    ...list,
    items: list.items.map((a) =>
      a.slug === slug ? { ...a, status, cta: status === "online" ? "chat" : "waitlist" } : a,
    ),
  };
}

function CardSkeleton() {
  return (
    <div className="flex w-full max-w-[330px] flex-col gap-5 rounded-[16px] bg-[#e0d5c3] p-6" aria-hidden>
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

type CardProps = {
  advisor: FeaturedAdvisor;
  joined: boolean;
  pending: boolean;
  onJoin: (a: FeaturedAdvisor) => void;
  onLeave: (a: FeaturedAdvisor) => void;
};

function FeaturedCard({ advisor, joined, pending, onJoin, onLeave }: CardProps) {
  const { text, locale } = useI18n();
  const online = advisor.status === "online";
  // Buttons repeat on every card: they point at the advisor's name.
  const nameId = useId();
  return (
    <article
      className="relative flex w-full max-w-[330px] flex-col items-start gap-5 rounded-[16px] bg-[#e0d5c3] p-6"
      data-testid="featured-advisor"
    >
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
      <div className="flex w-full flex-col gap-1 text-black">
        <h3 id={nameId} className="font-serif text-[22px] font-semibold">
          {text(advisor.name)}
        </h3>
        <p className="font-serif text-[14px] font-light">{text(advisor.headline)}</p>
      </div>
      <img alt="" aria-hidden className="block h-px w-full" src={imgDivider} />
      <dl className="flex w-full flex-col gap-2 font-serif text-[13px] text-black">
        <div className="flex items-start justify-between gap-3">
          <dt className="font-light">{text("Consultation Rate ")}</dt>
          <dd className="font-semibold">
            {text("{value0} credits / min", { value0: advisor.rate_per_minute.toLocaleString(locale) })}
          </dd>
        </div>
        <div className="flex items-start justify-between gap-3">
          <dt className="font-light">{text("Rating ")}</dt>
          <dd className="flex items-center gap-1 font-semibold">
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
      {online ? (
        <Button asChild variant="cta" className="relative h-11 w-full overflow-hidden text-[14px]">
          <Link
            aria-describedby={nameId}
            href={`/${locale}/chatroom?reader=${encodeURIComponent(advisor.slug)}`}
            onClick={() => track("chat_cta_clicked", { slug: advisor.slug, status: advisor.status, source: "home_featured" })}
          >
            <img alt="" aria-hidden className="pointer-events-none absolute inset-0 size-full object-cover mix-blend-overlay" src={imgPaper} />
            <span className="relative">{text("Start 1-on-1 Chat ")}</span>
          </Link>
        </Button>
      ) : joined ? (
        <Button
          type="button"
          variant="glass"
          className="h-11 w-full text-[14px]"
          aria-describedby={nameId}
          disabled={pending}
          aria-busy={pending}
          onClick={() => onLeave(advisor)}
        >
          {text("✓ On the waiting list")}
          <span className="sr-only">{text("(select to leave)")}</span>
        </Button>
      ) : (
        <Button
          type="button"
          variant="glass"
          className="h-11 w-full text-[14px]"
          aria-describedby={nameId}
          disabled={pending}
          aria-busy={pending}
          onClick={() => onJoin(advisor)}
        >
          {text(pending ? "Joining…" : "Join Waiting List ")}
        </Button>
      )}
    </article>
  );
}

/**
 * The home page "Psychic readings built around your journey" cards: the
 * advisors who can talk now first, then the best rated by many reviews
 * (GET /psychics/featured). Statuses stay live through realtime events and a
 * refresh every 30 seconds while the page is visible. Busy or offline
 * advisors offer the waiting list; visitors are asked to sign in first.
 */
export function FeaturedAdvisors() {
  const { text, locale } = useI18n();
  const router = useRouter();
  const toast = useToast();
  const { status: sessionStatus, user } = useSession();
  const requireAdult = useRequireAdult();
  const isClient = sessionStatus === "authenticated" && user?.role === "client";
  const featured = useAsync((signal) => psychicsApi.featured(FEATURED_COUNT, signal), []);
  const mine = useAsync((signal) => waitlistApi.mine({ limit: 50 }, signal), [user?.id ?? ""], isClient);
  const [joinedNow, setJoinedNow] = useState<Record<string, boolean>>({});
  const [pending, setPending] = useState<string | null>(null);
  const { reload, setData } = featured;

  // Refresh while visible; pause in background tabs.
  useEffect(() => {
    const tick = () => {
      if (document.visibilityState === "visible") reload();
    };
    const id = window.setInterval(tick, REFRESH_MS);
    document.addEventListener("visibilitychange", tick);
    return () => {
      window.clearInterval(id);
      document.removeEventListener("visibilitychange", tick);
    };
  }, [reload]);

  useRealtimeEvent<{ slug: string; status: PresenceStatus }>("psychic.status_changed", (d) =>
    setData((list) => (list ? withStatus(list, d.slug, d.status) : list)),
  );

  const joined = useMemo(() => {
    const out = new Set((mine.data?.items ?? []).map((e) => e.slug));
    for (const [slug, on] of Object.entries(joinedNow)) {
      if (on) out.add(slug);
      else out.delete(slug);
    }
    return out;
  }, [mine.data, joinedNow]);

  const join = useCallback(
    async (advisor: FeaturedAdvisor) => {
      if (sessionStatus !== "authenticated") {
        router.push(loginUrl(locale));
        return;
      }
      if (user?.role !== "client") {
        router.push(`/${locale}/psychics/${advisor.slug}`);
        return;
      }
      if (!requireAdult()) return;
      setPending(advisor.slug);
      try {
        await waitlistApi.join(advisor.slug);
        setJoinedNow((m) => ({ ...m, [advisor.slug]: true }));
        const message = text("You're on {value0}'s waiting list. We'll let you know when they're online.", { value0: advisor.name });
        toast.show({ tone: "success", message });
        announce(message);
      } catch (err) {
        const e = toApiError(err);
        if (e.code === "advisor_available") {
          setData((list) => (list ? withStatus(list, advisor.slug, "online") : list));
          toast.show({ tone: "info", message: text("{value0} is online now. Start your chat.", { value0: advisor.name }) });
          reload();
        } else {
          toast.showError(e);
        }
      } finally {
        setPending(null);
      }
    },
    [locale, reload, requireAdult, router, sessionStatus, setData, text, toast, user?.role],
  );

  const leave = useCallback(
    async (advisor: FeaturedAdvisor) => {
      setPending(advisor.slug);
      try {
        await waitlistApi.leave(advisor.slug);
        setJoinedNow((m) => ({ ...m, [advisor.slug]: false }));
        const message = text("You left {value0}'s waiting list.", { value0: advisor.name });
        toast.show({ tone: "info", message });
        announce(message);
      } catch (err) {
        toast.showError(err);
      } finally {
        setPending(null);
      }
    },
    [text, toast],
  );

  const list = featured.data;
  return (
    <div
      className="relative flex w-full flex-col items-center gap-6 md:flex-row md:flex-wrap md:items-stretch md:justify-center lg:flex-nowrap lg:gap-[105px]"
      data-name="Layout / Our Psychics / 02"
      aria-busy={featured.loading}
    >
      {list === null ? (
        featured.error && !featured.loading ? (
          <ErrorState className="w-full max-w-md bg-[#e0d5c3]" error={featured.error} onRetry={reload} title={text("We couldn't load the advisors.")} />
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
        <p className="font-serif text-[16px] text-white">{text("No advisors are listed right now.")}</p>
      ) : (
        list.items.map((advisor) => (
          <FeaturedCard
            key={advisor.slug}
            advisor={advisor}
            joined={joined.has(advisor.slug)}
            pending={pending === advisor.slug}
            onJoin={join}
            onLeave={leave}
          />
        ))
      )}
    </div>
  );
}
