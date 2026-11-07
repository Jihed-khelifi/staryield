"use client";
/* eslint-disable @next/next/no-img-element */
import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useI18n } from "@/i18n/i18n-provider";
import { zodiacIconUrl } from "@/lib/astro-icons";
import { formatRating } from "@/lib/format";
import { SiteHeader, MobileNavigation } from "@/components/layout/site-header";
import { Button } from "@/components/ui/button";
import { FreeChatBadge } from "@/components/billing/free-chat-badge";
import { useFreeMode } from "@/lib/site-config/site-config-provider";
import { psychicsHref as buildPsychicsHref, usePsychicsHref } from "@/lib/psychics-link";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { psychicsApi } from "@/lib/api/endpoints";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import type { AdvisorCard, PresenceStatus, Psychic, Service } from "@/lib/api/types";
import { track } from "@/lib/analytics";
import { useRealtimeEvent, useSession } from "@/lib/auth/session-provider";
import { PsychicReviews } from "@/components/reviews/psychic-reviews";
import { NumberedPager } from "@/components/ui/pagination";
import { Skeleton, SkeletonText } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/feedback/empty-state";
import { ErrorState } from "@/components/feedback/error-state";
import { announce } from "@/components/feedback/live-region";
import { useAsync } from "@/lib/use-async";
import { ServiceOrderForm } from "@/components/psychics/service-order-form";
import { WaitlistButton } from "@/components/waitlist/waitlist-button";

const statusCopy: Record<PresenceStatus, string> = {
  online: "● Online",
  busy: "● Busy",
  away: "● Away",
  offline: "● Offline",
};

/** Keeps advisor statuses live (realtime for members, polling otherwise). */
function useLiveStatuses(update: (slug: string, status: PresenceStatus) => void, refetch: () => void) {
  const { realtime } = useSession();
  useRealtimeEvent<{ slug: string; status: PresenceStatus }>("psychic.status_changed", (d) => update(d.slug, d.status));
  useEffect(() => {
    if (realtime) return;
    const id = window.setInterval(refetch, 60_000);
    return () => window.clearInterval(id);
  }, [realtime, refetch]);
}

function LoadState({ error, onRetry }: { error: ApiError | null; onRetry: () => void }) {
  const { text } = useI18n();
  if (!error)
    return (
      <p className="py-16 text-center text-sm" role="status">
        {text("Loading advisors…")}
      </p>
    );
  return (
    <div className="py-16 text-center" role="alert">
      <p className="text-sm">{text(errorMessage(error))}</p>
      <Button className="gold-button mt-4" onClick={onRetry}>
        {text("Try again")}
      </Button>
    </div>
  );
}

/** Advisors per directory page (the API allows up to 48). */
export const DIRECTORY_PAGE_SIZE = 12;

/** The page number in the address bar (`?page=`), 1 when absent or invalid. */
export function readDirectoryPage(search: string): number {
  const raw = new URLSearchParams(search).get("page");
  if (!raw || !/^\d{1,5}$/.test(raw)) return 1;
  const n = Number(raw);
  return n >= 1 && n <= 10_000 ? n : 1;
}

/** Records the page in the address bar without a navigation or new history entry. */
function writeDirectoryPage(page: number) {
  const url = new URL(window.location.href);
  if (page > 1) url.searchParams.set("page", String(page));
  else url.searchParams.delete("page");
  if (url.href !== window.location.href) window.history.replaceState(window.history.state, "", url);
}

function AdvisorGridSkeleton() {
  return (
    <div className="advisor-grid" aria-hidden data-testid="advisor-grid-skeleton">
      {Array.from({ length: 6 }, (_, i) => (
        <div key={i} className="advisor-card">
          <Skeleton className="advisor-card-image rounded-none" />
          <div className="advisor-card-copy space-y-3">
            <Skeleton className="h-7 w-2/3" />
            <Skeleton className="h-3 w-1/2" />
            <Skeleton className="h-3 w-full" />
            <SkeletonText lines={2} />
          </div>
        </div>
      ))}
    </div>
  );
}

/** Accessible status names (the desktop card shows a dot, mobile a pill). */
const statusName: Record<PresenceStatus, string> = { online: "Online", busy: "Busy", away: "Away", offline: "Offline" };

function AdvisorCardView({ advisor }: { advisor: AdvisorCard }) {
  const { text, locale } = useI18n();
  return (
    <article className="advisor-card texture-24" data-status={advisor.status}>
      <img className="advisor-card-image" src={advisor.image_url} alt={text(advisor.name)} loading="lazy" decoding="async" />
      <div className="advisor-card-copy">
        <div className="advisor-card-head">
          <h2 className="text-[26px] md:font-display">
            {/* Figma desktop: a status dot before the name. Its shape also
                changes (filled when available, hollow when offline), so the
                status is not told by colour alone. */}
            <span className={`advisor-status-dot ${advisor.status}`} aria-hidden title={text(statusName[advisor.status])} />
            {text(advisor.name)}
            <span className="sr-only">, {text(statusName[advisor.status])}</span>
          </h2>
          <span className={`status-pill ${advisor.status === "online" ? "online" : ""}`} aria-hidden>
            {text(statusCopy[advisor.status])}
          </span>
        </div>
        <FreeChatBadge />
        {advisor.skills.length ? (
          <ul className="advisor-skills" aria-label={text("Specialties")}>
            {advisor.skills.slice(0, 4).map((skill) => (
              <li key={skill}>{text(skill)}</li>
            ))}
          </ul>
        ) : null}
        <div className="advisor-card-meta">
          <span>
            <img className="advisor-card-star" src="https://assets.staryield.net/assets/figma/d9e90.svg" alt="" aria-hidden />
            <strong>{formatRating(advisor.rating, locale)}</strong>{" "}
            <span className="advisor-card-reviews">
              {text("({value0} reviews)", { value0: advisor.reviews_count.toLocaleString(locale) })}
            </span>
          </span>
          <span className="font-extralight">
            {advisor.rate_per_minute} {text("credits/min ")}
          </span>
        </div>
        <p className="advisor-card-headline">{text(advisor.headline)}</p>
        <Link className="advisor-profile-link" href={`/${locale}/psychics/${advisor.slug}`}>
          {text("View full profile ")}
          <span className="md:hidden" aria-hidden>
            &gt;
          </span>
          <span className="sr-only">: {text(advisor.name)}</span>
        </Link>
      </div>
    </article>
  );
}

function PsychicsDirectory() {
  const { text } = useI18n();
  // Read once on the client; the server renders the loading state, which
  // does not depend on the page, so hydration matches.
  const [page, setPage] = useState(() => (typeof window === "undefined" ? 1 : readDirectoryPage(window.location.search)));
  const top = useRef<HTMLDivElement>(null);
  const { data, error, loading, reload, setData } = useAsync(async (signal) => {
    const res = await psychicsApi.browse({ page, page_size: DIRECTORY_PAGE_SIZE }, signal);
    // A deep link past the end (advisors were hidden since): show the last page.
    if (res.items.length === 0 && res.total_pages > 0 && page > res.total_pages)
      return psychicsApi.browse({ page: res.total_pages, page_size: DIRECTORY_PAGE_SIZE }, signal);
    return res;
  }, [page]);

  useEffect(() => {
    track("psychic_list_viewed");
  }, []);

  const shown = data?.page;
  useEffect(() => {
    if (shown) writeDirectoryPage(shown);
  }, [shown]);

  useLiveStatuses(
    (slug, status) =>
      setData((d) => (d ? { ...d, items: d.items.map((a) => (a.slug === slug ? { ...a, status } : a)) } : d)),
    reload,
  );

  function goTo(next: number) {
    setPage(next);
    const reduce = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
    top.current?.scrollIntoView({ behavior: reduce ? "auto" : "smooth", block: "start" });
    announce(text("Page {value0} of {value1}", { value0: next, value1: data?.total_pages ?? next }));
  }

  return (
    <div className="dashboard-page">
      <SiteHeader />
      <main className="psychics-browse">
        <div className="text-center" ref={top}>
          <p className="eyebrow md:hidden">{text("Trusted guidance")}</p>
          <h1 className="mt-3 text-3xl md:font-display md:text-[40px]">
            <span className="md:hidden">{text("Our psychic advisors")}</span>
            <span className="hidden md:inline">{text("Our advisors")}</span>
          </h1>
          <p className="mx-auto mt-3 hidden max-w-[760px] text-base font-extralight leading-relaxed md:block">
            {text(
              "Our advisors are widely acknowledged and recognized as being profoundly accurate and intuitive. Through consistent excellence, they have successfully cemented their already-established unwavering pillars as cornerstones within the guidance industry. ",
            )}
          </p>
          <p className="mt-2 text-sm font-light md:hidden">
            {text(
              "Accurate, intuitive specialists—available for private live readings. ",
            )}
          </p>
        </div>
        {data === null ? (
          error && !loading ? (
            <ErrorState className="mx-auto mt-11 max-w-md" error={error} onRetry={reload} title={text("We couldn't load the advisors.")} />
          ) : (
            <div aria-busy="true">
              <p role="status" className="sr-only">
                {text("Loading advisors…")}
              </p>
              <AdvisorGridSkeleton />
            </div>
          )
        ) : data.total === 0 ? (
          <EmptyState
            className="mx-auto mt-11 max-w-md"
            title={text("No advisors are listed right now.")}
            description={text("New advisors join regularly. Please check back soon.")}
          />
        ) : (
          <>
            {error && !loading ? <ErrorState compact className="mx-auto mt-6 max-w-md" error={error} onRetry={reload} /> : null}
            <div aria-busy={loading} className={loading ? "opacity-70 transition-opacity" : "transition-opacity"}>
              <div className="advisor-grid">
                {data.items.map((advisor) => (
                  <AdvisorCardView key={advisor.slug} advisor={advisor} />
                ))}
              </div>
            </div>
            <NumberedPager
              className="mt-10"
              page={data.page}
              pageCount={data.total_pages}
              loading={loading}
              label={text("Advisor pages")}
              onPageChange={goTo}
            />
          </>
        )}
      </main>
      <MobileNavigation />
    </div>
  );
}

/**
 * The directory is for signed-in customers (revisions 3.0). Visitors are sent
 * to the home section once the session answers "anonymous"; until then (and
 * while redirecting) only a neutral loading state shows, never the directory.
 */
export function PsychicsBrowse() {
  const { text, locale } = useI18n();
  const router = useRouter();
  const { status } = useSession();
  useEffect(() => {
    if (status === "anonymous") router.replace(buildPsychicsHref(locale, false));
  }, [status, locale, router]);
  if (status !== "authenticated")
    return (
      <div className="dashboard-page">
        <SiteHeader />
        <main className="psychics-browse" aria-busy="true" data-testid="psychics-gate">
          <p role="status" className="sr-only">
            {text("Loading advisors…")}
          </p>
          <AdvisorGridSkeleton />
        </main>
        <MobileNavigation />
      </div>
    );
  return <PsychicsDirectory />;
}

export function PsychicProfile({ slug }: { slug: string }) {
  const { text, locale } = useI18n();
  const { status: sessionStatus, user } = useSession();
  const psychicsHref = usePsychicsHref();
  const freeMode = useFreeMode();
  const [advisor, setAdvisor] = useState<Psychic | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [service, setService] = useState<Service | null>(null);

  const load = useCallback(() => {
    psychicsApi
      .get(slug)
      .then((p) => {
        setAdvisor(p);
        setError(null);
        track("psychic_profile_viewed", { slug });
      })
      .catch((err) => setError(toApiError(err)));
  }, [slug]);

  useEffect(load, [load]);
  useLiveStatuses((s, st) => setAdvisor((a) => (a && a.slug === s ? { ...a, status: st } : a)), load);

  if (!advisor) {
    return (
      <div className="dashboard-page">
        <SiteHeader />
        <main className="psychic-profile">
          {error?.code === "not_found" ? (
            <div className="py-16 text-center">
              <h1 className="font-display text-3xl">{text("This advisor could not be found.")}</h1>
              <Link className="mt-4 inline-block underline" href={psychicsHref}>
                {text("Browse all advisors")}
              </Link>
            </div>
          ) : (
            <LoadState
              error={error}
              onRetry={() => {
                setError(null);
                load();
              }}
            />
          )}
        </main>
        <MobileNavigation />
      </div>
    );
  }

  const canChat = advisor.status === "online";
  const returnHere = encodeURIComponent(`/${locale}/psychics/${slug}`);

  return (
    <div className="dashboard-page">
      <SiteHeader />
      <main className="psychic-profile">
        <div className="mb-8 flex items-center justify-between text-sm">
          <div>
            <Link href={psychicsHref}>{text("Psychics")}</Link>{" "}
            <span className="mx-2">›</span> <strong>{text(advisor.name)}</strong>
          </div>
          <span className={`status-pill ${canChat ? "online" : ""}`}>
            {text(canChat ? "● Online now" : statusCopy[advisor.status])}
          </span>
        </div>
        <section className="profile-hero">
          <div className="relative overflow-hidden rounded-2xl">
            <img className="h-full w-full object-cover" src={advisor.image_url} alt={text(advisor.name)} />
            <span className="absolute bottom-6 left-6 rounded-full bg-black px-4 py-2 text-xs text-white">
              {text("♙ Verified Master Advisor ")}
            </span>
          </div>
          <div className="flex flex-col justify-center gap-6">
            <h1 className="flex items-center gap-4 font-display text-4xl lg:text-[56px]">
              {zodiacIconUrl(advisor.zodiac) ? (
                <img src={zodiacIconUrl(advisor.zodiac)} alt={text(advisor.zodiac)} className="h-12 w-12 object-contain" />
              ) : null}
              {text(advisor.name)}
            </h1>
            <div className="flex flex-wrap gap-2">
              {advisor.skills.map((skill) => (
                <span className="status-pill" key={skill}>
                  {text(skill)}
                </span>
              ))}
            </div>
            <div className="grid grid-cols-3 gap-3">
              {[
                [`${formatRating(advisor.rating, locale)} ★`, "Rating", text("{count} reviews", { count: advisor.reviews_count })],
                [`${advisor.readings_count.toLocaleString(locale)}+`, "Readings", "Completed"],
                [text("{count} years", { count: advisor.years_experience }), "Experience", "Active practice"],
              ].map(([value, label, detail]) => (
                <div className="stat-tile" key={label}>
                  <p className="font-display text-lg lg:text-[28px]">{text(value)}</p>
                  <p className="mt-2 text-xs uppercase">{text(label)}</p>
                  <p className="text-[11px]">{text(detail)}</p>
                </div>
              ))}
            </div>
            <div className="rounded-2xl border border-gold p-6">
              <div className="flex justify-between gap-3">
                <div>
                  <p className="text-sm uppercase">{text("Consultation rate")}</p>
                  {freeMode ? (
                    <p data-testid="profile-free-rate">
                      <span className="font-display text-[32px]">{text("Free for a limited time")}</span>{" "}
                      <s className="text-sm" aria-label={text("List rate: {value0} credits per minute", { value0: advisor.rate_per_minute })}>
                        {advisor.rate_per_minute} {text("Credits / minute")}
                      </s>
                    </p>
                  ) : (
                    <p>
                      <span className="font-display text-[32px]">{advisor.rate_per_minute}</span>{" "}
                      <strong className="text-sm">{text("Credits / minute")}</strong>
                    </p>
                  )}
                </div>
                <p className="self-center text-right text-xs">
                  {text(canChat ? "● Instant Connection " : statusCopy[advisor.status])}
                  <br />
                  {text(canChat ? "No waiting queue " : "Live chat opens when this advisor is online.")}
                </p>
              </div>
              <FreeChatBadge className="mt-4" />
              {/* Busy, away or offline: wait for the advisor next to the chat CTA (same flow as the home cards). */}
              <div className="mt-5 flex flex-col gap-3 sm:flex-row" data-testid="profile-ctas">
                <Button asChild className="gold-button h-14 w-full sm:flex-1">
                  <Link href={`/${locale}/chatroom?reader=${slug}`} onClick={() => track("chat_cta_clicked", { slug, status: advisor.status })}>
                    {text("♧ Start live chat ")}
                  </Link>
                </Button>
                {!canChat ? (
                  <WaitlistButton
                    slug={slug}
                    name={advisor.name}
                    className="h-14 w-full sm:flex-1"
                    onAvailable={() => setAdvisor((a) => (a ? { ...a, status: "online" } : a))}
                  />
                ) : null}
              </div>
            </div>
          </div>
        </section>
        <section className="profile-details">
          <div>
            <h2 className="font-display text-[32px]">{text("About")}</h2>
            {advisor.about.map((paragraph, index) => (
              <p className="mt-6 font-extralight leading-[1.8]" key={index}>
                {text(paragraph)}
              </p>
            ))}
            {advisor.philosophy ? (
              <div className="mt-6 rounded-2xl bg-warm-gray/55 p-6">
                <h3 className="font-display text-xl">{text("My philosophy")}</h3>
                <p className="mt-4 text-sm font-light">{text(advisor.philosophy)}</p>
              </div>
            ) : null}
          </div>
          <div>
            <h2 className="mb-6 text-[32px]">{text("Orders / Services")}</h2>
            <div className="space-y-4">
              {advisor.services.map((item) => (
                <article className="rounded-2xl border border-gold p-6" key={item.id}>
                  <div className="flex justify-between gap-3">
                    <div>
                      <h3 className="text-xl">{text(item.name)}</h3>
                      <p className="text-xs font-light">{text(item.subtitle)}</p>
                      {item.kind === "custom" && item.delivery_days ? (
                        <p className="text-xs font-light" data-testid="service-delivery">
                          {text("Custom request · delivered in about {value0} days", { value0: item.delivery_days })}
                        </p>
                      ) : null}
                    </div>
                    <span className="status-pill self-start">
                      {text("{value0} Credits", { value0: item.price_credits.toLocaleString(locale) })}
                    </span>
                  </div>
                  <p className="my-4 text-sm font-extralight leading-relaxed">{text(item.description)}</p>
                  <Button
                    className="h-10 w-full bg-gold"
                    onClick={() => {
                      setService(item);
                      track("service_viewed", { slug, credits: item.price_credits });
                    }}
                  >
                    {text("Order service › ")}
                  </Button>
                </article>
              ))}
            </div>
          </div>
        </section>
        <PsychicReviews slug={slug} total={advisor.reviews_count} />
      </main>
      <MobileNavigation />
      <Sheet
        open={Boolean(service)}
        onOpenChange={(open) => {
          if (!open) setService(null);
        }}
      >
        <SheetContent className="overflow-y-auto">
          <SheetHeader>
            <SheetTitle>{text(service?.name)}</SheetTitle>
          </SheetHeader>
          {sessionStatus !== "authenticated" ? (
            <div className="space-y-4 p-6">
              <p>{text("Sign in to place a request with this advisor.")}</p>
              <Button asChild className="gold-button w-full">
                <Link href={`/${locale}/login?next=${returnHere}`}>{text("Log In")}</Link>
              </Button>
            </div>
          ) : user?.role !== "client" ? (
            <p className="p-6">{text("Only client accounts can place requests.")}</p>
          ) : service ? (
            <ServiceOrderForm key={service.id} service={service} advisorName={advisor.name} slug={slug} returnHere={returnHere} />
          ) : null}
        </SheetContent>
      </Sheet>
    </div>
  );
}
