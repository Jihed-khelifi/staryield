"use client";
/* eslint-disable @next/next/no-img-element */
import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useI18n } from "@/i18n/i18n-provider";
import { formatRating } from "@/lib/format";
import { SiteHeader, MobileNavigation } from "@/components/layout/site-header";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { psychicsApi, requestsApi } from "@/lib/api/endpoints";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import { newId } from "@/lib/api/client";
import type { PresenceStatus, Psychic, Service } from "@/lib/api/types";
import { track } from "@/lib/analytics";
import { useRealtimeEvent, useSession } from "@/lib/auth/session-provider";
import { useToast } from "@/components/feedback/toast-provider";
import { PsychicReviews } from "@/components/reviews/psychic-reviews";

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

export function PsychicsBrowse() {
  const { text, locale } = useI18n();
  const [advisors, setAdvisors] = useState<Psychic[] | null>(null);
  const [error, setError] = useState<ApiError | null>(null);

  const load = useCallback(() => {
    psychicsApi
      .list()
      .then((list) => {
        setAdvisors(list);
        setError(null);
      })
      .catch((err) => setError(toApiError(err)));
  }, []);

  useEffect(() => {
    load();
    track("psychic_list_viewed");
  }, [load]);

  useLiveStatuses(
    (slug, status) => setAdvisors((list) => list?.map((a) => (a.slug === slug ? { ...a, status } : a)) ?? list),
    load,
  );

  return (
    <div className="dashboard-page">
      <SiteHeader />
      <main className="psychics-browse">
        <div className="text-center">
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
        {advisors === null ? (
          <LoadState
            error={error}
            onRetry={() => {
              setError(null);
              load();
            }}
          />
        ) : advisors.length === 0 ? (
          <p className="py-16 text-center text-sm">{text("No advisors are listed right now.")}</p>
        ) : (
          <div className="advisor-grid">
            {advisors.map((advisor) => (
              <article key={advisor.slug} className="advisor-card">
                <img className="advisor-card-image" src={advisor.image_url} alt={text(advisor.name)} />
                <div className="advisor-card-copy">
                  <div className="flex items-center justify-between">
                    <h2 className="text-[26px] md:font-display">
                      <span className="hidden text-amber md:inline">• </span>
                      {text(advisor.name)}
                    </h2>
                    <span className={`status-pill ${advisor.status === "online" ? "online" : ""}`}>
                      {text(statusCopy[advisor.status])}
                    </span>
                  </div>
                  <p className="mt-2 text-xs text-sage">
                    {advisor.skills.map((skill) => text(skill)).join(" · ")}
                  </p>
                  <div className="mt-3 flex items-center justify-between gap-2 text-[11px]">
                    <span>
                      ★{" "}
                      <strong>
                        {formatRating(advisor.rating, locale)} · {advisor.reviews_count.toLocaleString(locale)} {text("reviews ")}
                      </strong>
                    </span>
                    <span className="font-extralight">
                      {advisor.rate_per_minute} {text("credits/min ")}
                    </span>
                  </div>
                  <p className="mt-3 text-[13px] leading-snug">{text(advisor.headline)}</p>
                  <Link className="advisor-profile-link" href={`/${locale}/psychics/${advisor.slug}`}>
                    {text("View full profile ")}
                    <span className="md:hidden">&gt;</span>
                  </Link>
                </div>
              </article>
            ))}
          </div>
        )}
      </main>
      <MobileNavigation />
    </div>
  );
}

export function PsychicProfile({ slug }: { slug: string }) {
  const { text, locale } = useI18n();
  const { status: sessionStatus, user, wallet, setWallet } = useSession();
  const toast = useToast();
  const [advisor, setAdvisor] = useState<Psychic | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [service, setService] = useState<Service | null>(null);
  const [notice, setNotice] = useState("");
  const [orderError, setOrderError] = useState<ApiError | null>(null);
  const [ordering, setOrdering] = useState(false);
  const orderKey = useRef(newId());

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

  async function order(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!service) return;
    const data = new FormData(event.currentTarget);
    const date = String(data.get("date") ?? "");
    const time = String(data.get("time") ?? "");
    const preferred = date ? new Date(`${date}T${time || "12:00"}`).toISOString() : undefined;
    setOrdering(true);
    setOrderError(null);
    setNotice("");
    track("service_order_started", { slug, credits: service.price_credits });
    try {
      const req = await requestsApi.order({
        service_id: service.id,
        note: String(data.get("note") ?? ""),
        preferred_at: preferred,
        idempotency_key: orderKey.current,
      });
      if (req.wallet) setWallet(req.wallet);
      orderKey.current = newId();
      setNotice(text("Your request was sent. {value0} credits are held until {value1} delivers it.", {
        value0: req.price_credits.toLocaleString(locale),
        value1: advisor?.name ?? "",
      }));
      toast.show({ tone: "success", message: text("Request sent to {value0}.", { value0: advisor?.name ?? "" }) });
    } catch (err) {
      setOrderError(toApiError(err));
    } finally {
      setOrdering(false);
    }
  }

  if (!advisor) {
    return (
      <div className="dashboard-page">
        <SiteHeader />
        <main className="psychic-profile">
          {error?.code === "not_found" ? (
            <div className="py-16 text-center">
              <h1 className="font-display text-3xl">{text("This advisor could not be found.")}</h1>
              <Link className="mt-4 inline-block underline" href={`/${locale}/psychics`}>
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
            <Link href={`/${locale}/psychics`}>{text("Psychics")}</Link>{" "}
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
              {advisor.zodiac ? (
                <img
                  src={`https://assets.staryield.net/assets/zodiac/${advisor.zodiac.toLowerCase()}.png`}
                  alt={text(advisor.zodiac)}
                  className="h-12 w-12 object-contain"
                />
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
                  <p>
                    <span className="font-display text-[32px]">{advisor.rate_per_minute}</span>{" "}
                    <strong className="text-sm">{text("Credits / minute")}</strong>
                  </p>
                </div>
                <p className="self-center text-right text-xs">
                  {text(canChat ? "● Instant Connection " : statusCopy[advisor.status])}
                  <br />
                  {text(canChat ? "No waiting queue " : "Live chat opens when this advisor is online.")}
                </p>
              </div>
              <Button asChild className="gold-button mt-5 h-14 w-full">
                <Link href={`/${locale}/chatroom?reader=${slug}`} onClick={() => track("chat_cta_clicked", { slug, status: advisor.status })}>
                  {text("♧ Start live chat ")}
                </Link>
              </Button>
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
                      setNotice("");
                      setOrderError(null);
                      orderKey.current = newId();
                      track("service_viewed", { slug, credits: item.price_credits });
                    }}
                  >
                    {text("Book Session › ")}
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
          ) : (
            <form className="space-y-5 p-6" onSubmit={order}>
              <p>{text(service?.description)}</p>
              <label className="field">
                {text("What would you like to explore?")}
                <textarea name="note" rows={4} maxLength={2000} />
              </label>
              <label className="field">
                {text("Preferred date ")}
                <input type="date" name="date" min={new Date().toISOString().slice(0, 10)} />
              </label>
              <label className="field">
                {text("Preferred time ")}
                <input type="time" name="time" />
              </label>
              <p>
                {text("{value0} Credits", { value0: (service?.price_credits ?? 0).toLocaleString(locale) })} ·{" "}
                {text("Balance: {value0} credits", { value0: (wallet?.balance ?? 0).toLocaleString(locale) })}
              </p>
              <p className="text-xs font-light">
                {text("Credits are held when you order and released to the advisor only when they deliver. Declined or expired requests are refunded.")}
              </p>
              <Button className="gold-button w-full" type="submit" disabled={ordering} aria-busy={ordering}>
                {text(ordering ? "Sending request…" : "Request a session ")}
              </Button>
              {orderError ? (
                <div role="alert" className="text-sm text-red-800">
                  <p>{text(errorMessage(orderError))}</p>
                  {orderError.code === "insufficient_credits" ? (
                    <Link className="underline" href={`/${locale}/credits?next=${returnHere}`}>
                      {text("Top up credits")}
                    </Link>
                  ) : null}
                </div>
              ) : null}
              <p role="status">{notice}</p>
            </form>
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}
