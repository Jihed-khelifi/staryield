"use client";

import * as React from "react";
import Link from "next/link";
import { CalendarClock, Moon, Sparkles, Sun, Zap } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import { astroApi, type MyHoroscope, type SkyEvents } from "@/lib/api/astrology";
import type { ApiError } from "@/lib/api/errors";
import type { User } from "@/lib/api/types";
import { useAsync, type AsyncState } from "@/lib/use-async";
import { EmptyState } from "@/components/feedback/empty-state";
import { ErrorState } from "@/components/feedback/error-state";
import { Button } from "@/components/ui/button";
import { Skeleton, SkeletonText } from "@/components/ui/skeleton";
import { ClientOrders } from "@/components/account/orders";

/**
 * The client's daily reading for their own sun sign (`GET /me/horoscope`,
 * "today" in the browser's zone). The account shell loads it once: the
 * affirmation banner and the overview share it.
 */
export function useMyHoroscope(user: User | null | undefined): AsyncState<MyHoroscope> {
  return useAsync((signal) => astroApi.myHoroscope({}, signal), [user?.id, user?.profile.birth_date, user?.profile.birth_time, user?.profile.birth_place_id], Boolean(user) && user?.role === "client");
}

function greeting(hour: number) {
  if (hour < 12) return "Good morning, ";
  if (hour < 18) return "Good afternoon, ";
  return "Good evening, ";
}

/** True when the API says the profile has no birth date yet. */
function missingBirthDate(error: ApiError | null): boolean {
  return Boolean(error && error.code === "validation_failed" && error.fields.birth_date);
}

type CosmicItem = {
  key: string;
  kind: "lunar" | "retrograde";
  title: string;
  /** When it happens (lunar) or starts (retrograde). */
  at: Date;
  end?: Date;
  ongoing: boolean;
};

const DAY = 86_400_000;

/** The next new and full moons and the nearest retrograde, soonest first (at most three). */
export function cosmicItems(events: SkyEvents, now: Date, text: ReturnType<typeof useI18n>["text"]): CosmicItem[] {
  const lunar = events.lunar
    .filter((e) => (e.kind === "full_moon" || e.kind === "new_moon") && new Date(e.at).getTime() >= now.getTime())
    .slice(0, 2)
    .map<CosmicItem>((e) => ({
      key: `${e.kind}-${e.at}`,
      kind: "lunar",
      title: text(e.kind === "full_moon" ? "Full Moon in {value0}" : "New Moon in {value0}", { value0: text(e.sign) }),
      at: new Date(e.at),
      ongoing: false,
    }));
  const retro = events.retrogrades
    .filter((r) => new Date(r.end).getTime() > now.getTime())
    .sort((a, b) => a.start.localeCompare(b.start))
    .slice(0, 1)
    .map<CosmicItem>((r) => ({
      key: `${r.body}-${r.start}`,
      kind: "retrograde",
      title: text("{value0} Retrograde", { value0: text(r.body) }),
      at: new Date(r.start),
      end: new Date(r.end),
      ongoing: new Date(r.start).getTime() <= now.getTime(),
    }));
  return [...retro.filter((r) => r.ongoing), ...[...lunar, ...retro.filter((r) => !r.ongoing)].sort((a, b) => a.at.getTime() - b.at.getTime())].slice(0, 3);
}

function daysUntil(when: Date, now: Date) {
  return Math.max(0, Math.ceil((when.getTime() - now.getTime()) / DAY));
}

/**
 * The client's Account Overview (Figma 384:1557 desktop, 867:16324 mobile):
 * greeting, compatibility score with its label, lucky number and mood, the
 * daily horoscope, upcoming sky events and the day's focus, then orders.
 */
export function Overview({ name, user, horoscope }: { name: string; user: User; horoscope: AsyncState<MyHoroscope> }) {
  const { text, locale } = useI18n();
  const [now] = React.useState(() => new Date());
  const h = horoscope.data;
  const tracked = React.useRef(false);
  // Sky events come with the reading; without one (no birth date yet, or an
  // error) the public calendar still fills the section.
  const fallbackEvents = useAsync((signal) => astroApi.events(6, signal), ["overview-events"], Boolean(horoscope.error));

  React.useEffect(() => {
    if (h && !tracked.current) {
      tracked.current = true;
      track("horoscope_viewed"); // no sign or birth data in analytics
    }
  }, [h]);

  const events: SkyEvents | null = h?.events ?? fallbackEvents.data;
  const eventsError = horoscope.error ? fallbackEvents.error : null;
  const items = React.useMemo(() => (events ? cosmicItems(events, now, text) : []), [events, now, text]);

  const longDate = new Intl.DateTimeFormat(locale, { dateStyle: "long" }).format(now);
  const weekday = new Intl.DateTimeFormat(locale, { weekday: "long" }).format(now);
  const needsBirthDate = missingBirthDate(horoscope.error);

  const stats = (
    <dl className="grid grid-cols-3 gap-2 md:flex md:gap-4" data-testid="overview-stats">
      <div className="flex min-w-0 flex-col gap-[3px] rounded-[6px] bg-[rgb(214_208_188/0.2)] p-3 md:w-[177px] md:gap-2 md:rounded-[4px] md:border md:border-gold md:bg-warm-gray md:p-4">
        <dt className="text-[9px] font-bold tracking-[0.45px] text-[var(--muted-ink)] uppercase md:text-xs md:font-light md:tracking-normal md:text-black">
          {text("Compatibility score")}
        </dt>
        <dd className="flex flex-col gap-[3px] md:flex-row md:items-center md:justify-between">
          {h ? (
            <>
              <span className="text-[21px] font-semibold md:text-2xl md:font-normal" data-testid="compatibility-score">
                {text("{value0}%", { value0: h.compatibility_score })}
              </span>
              <span className="text-[9px] text-[var(--muted-ink)] md:rounded-[4px] md:bg-[var(--badge-gold)] md:px-2 md:py-1 md:text-[10px] md:font-bold md:text-white md:uppercase" data-testid="compatibility-label">
                {h.compatibility_label}
              </span>
            </>
          ) : horoscope.loading ? (
            <Skeleton className="h-7 w-20" />
          ) : (
            <span className="text-[21px] md:text-2xl">—</span>
          )}
        </dd>
      </div>
      <div className="flex min-w-0 flex-col gap-[3px] rounded-[6px] bg-[rgb(214_208_188/0.2)] p-3 md:w-[152px] md:gap-2 md:rounded-[4px] md:border md:border-gold md:bg-warm-gray md:p-4">
        <dt className="text-[9px] font-bold tracking-[0.45px] text-[var(--muted-ink)] uppercase md:text-xs md:font-light md:tracking-normal md:text-black">
          {text("Lucky number")}
        </dt>
        <dd className="text-[21px] font-semibold md:text-2xl md:font-normal" data-testid="lucky-number">
          {h ? h.lucky_number : horoscope.loading ? <Skeleton className="h-7 w-10" /> : "—"}
        </dd>
      </div>
      <div className="flex min-w-0 flex-col gap-[3px] rounded-[6px] bg-[rgb(214_208_188/0.2)] p-3 md:w-[208px] md:gap-2 md:rounded-[4px] md:border md:border-gold md:bg-warm-gray md:p-4">
        <dt className="text-[9px] font-bold tracking-[0.45px] text-[var(--muted-ink)] uppercase md:text-xs md:font-light md:tracking-normal md:text-black">
          {text("Mood")}
        </dt>
        <dd className="text-[13px] font-semibold break-words md:text-2xl md:font-normal">{text(user.profile.mood || h?.mood || "—")}</dd>
      </div>
    </dl>
  );

  const horoscopeBody = needsBirthDate ? (
    <EmptyState
      title={text("Add your birth date to read your horoscope")}
      description={text("Your daily horoscope, lucky number and compatibility come from your sun sign.")}
      action={
        <Button asChild variant="cta" size="lg" className="px-4">
          <Link href={`/${locale}/profile/settings`}>{text("Open settings")}</Link>
        </Button>
      }
    />
  ) : horoscope.error ? (
    <ErrorState error={horoscope.error} onRetry={horoscope.reload} compact title={text("Your horoscope could not be loaded")} />
  ) : h ? (
    <p className="text-[13px] leading-[1.55] font-light text-[var(--muted-ink)] md:text-base md:leading-normal md:text-black" data-testid="horoscope-text">
      {h.text}
    </p>
  ) : (
    <div aria-busy="true">
      <p role="status" className="sr-only">
        {text("Reading the sky…")}
      </p>
      <SkeletonText lines={4} />
    </div>
  );

  const horoscopeDate = h ? new Intl.DateTimeFormat(locale, { dateStyle: "long", timeZone: "UTC" }).format(new Date(`${h.date}T12:00:00Z`)) : "";

  return (
    <div className="flex flex-col gap-5 md:gap-8" data-testid="client-overview">
      <section className="flex flex-col gap-5 md:flex-row md:items-start md:justify-between md:gap-6">
        <div className="flex items-start justify-between gap-3">
          <div className="flex flex-col gap-[3px] md:gap-1">
            <h1 className="text-[25px] leading-[1.05] font-semibold text-[#17150f] md:text-[28px] md:font-medium md:text-black">
              {text(greeting(now.getHours()))}
              {name}
            </h1>
            <p className="text-[11px] text-[var(--muted-ink)] md:hidden">
              {weekday} · {longDate}
            </p>
            <p className="hidden text-base font-light md:block">{text("Today is {value0}", { value0: longDate })}</p>
          </div>
          <Sun aria-hidden className="size-7 shrink-0 text-[#8a6a1f] md:hidden" strokeWidth={1.6} />
        </div>
        {stats}
      </section>

      {/* Phones read the horoscope before the calendar (867:16324). */}
      <section className="rounded-[12px] bg-[rgb(251_246_233/0.2)] p-[18px] shadow-[0_10px_28px_rgb(23_21_15/0.09)] md:hidden" aria-labelledby="horoscope-title-mobile">
        <div className="flex items-center justify-between gap-3 border-b border-[rgb(170_142_98/0.35)] pb-3">
          <h2 id="horoscope-title-mobile" className="text-xl font-semibold text-[#17150f]">
            {text("Your horoscope")}
          </h2>
          {h ? <span className="text-[13px] tracking-wide text-[#74571a] uppercase">{h.sign_name}</span> : null}
        </div>
        <div className="mt-3">{horoscopeBody}</div>
      </section>

      <section aria-labelledby="cosmic-title" className="flex flex-col gap-4">
        <div className="flex flex-col gap-2">
          <p className="text-[10px] font-medium tracking-[1.5px] text-[#74571a] uppercase md:hidden">{text("Celestial calendar")}</p>
          <h2 id="cosmic-title" className="text-[30px] leading-[1.08] font-medium text-[#17150f] md:text-xl md:leading-normal md:font-semibold md:text-black">
            {text("Upcoming Cosmic Events ")}
          </h2>
        </div>
        <div className="flex flex-col gap-6 md:flex-row md:items-stretch md:gap-[21px]">
          <div className="flex flex-col gap-2.5 md:w-[355px] md:shrink-0 md:justify-between md:gap-3" data-testid="cosmic-events">
            {eventsError ? (
              <ErrorState error={eventsError} onRetry={fallbackEvents.reload} compact />
            ) : !events ? (
              <>
                <p role="status" className="sr-only">
                  {text("Loading…")}
                </p>
                <Skeleton className="h-[74px] w-full rounded-[12px] md:rounded-[4px]" />
                <Skeleton className="h-[74px] w-full rounded-[12px] md:rounded-[4px]" />
              </>
            ) : items.length === 0 ? (
              <EmptyState icon={<CalendarClock />} title={text("No major sky events in the coming weeks.")} className="py-5" />
            ) : (
              <ul className="flex flex-col gap-2.5 md:gap-3">
                {items.map((item) => (
                  <CosmicEventCard key={item.key} item={item} now={now} />
                ))}
              </ul>
            )}
          </div>
          <aside
            className="calc-olive flex flex-col gap-2 rounded-[12px] border-gold p-[18px] text-white md:min-h-[160px] md:flex-1 md:gap-3 md:rounded-[4px] md:border md:p-5"
            data-tone="dark"
            style={{ "--calc-texture": 0.2 } as React.CSSProperties}
            aria-labelledby="focus-title"
          >
            <h2 id="focus-title" className="flex items-center gap-2 text-[10px] font-bold tracking-[0.8px] uppercase md:text-sm md:font-semibold md:tracking-normal md:normal-case">
              <Sparkles aria-hidden className="hidden size-4 text-[#e9cf86] md:block" />
              {text("What to focus on today")}
            </h2>
            {h ? (
              <p className="text-[15px] leading-[1.45] md:text-base md:leading-6" data-testid="focus-text">
                “{h.focus}”
              </p>
            ) : horoscope.error ? (
              <p className="text-[15px] leading-[1.45] md:text-base md:leading-6">
                {text("Take a moment to reflect on your intentions today.")}
              </p>
            ) : (
              <SkeletonText lines={2} className="opacity-60" />
            )}
          </aside>
        </div>
      </section>

      <section className="hidden border-t border-gold pt-6 md:block" aria-labelledby="horoscope-title">
        <div className="flex items-start gap-10">
          <div className="w-40 shrink-0">
            <h2 id="horoscope-title" className="text-xl font-semibold">
              {text("Your horoscope")}
            </h2>
            {h ? (
              <p className="mt-1 text-sm font-light">
                <span className="block">{horoscopeDate}</span>
                <span className="block text-xs tracking-wide text-[#74571a] uppercase">{h.sign_name}</span>
              </p>
            ) : null}
          </div>
          <div className="min-w-0 flex-1">{horoscopeBody}</div>
        </div>
      </section>

      <ClientOrders />
    </div>
  );
}

function CosmicEventCard({ item, now }: { item: CosmicItem; now: Date }) {
  const { text, locale } = useI18n();
  const Icon = item.kind === "lunar" ? Moon : Zap;
  const date = new Intl.DateTimeFormat(locale, { month: "long", day: "numeric", year: "numeric" });
  const time = new Intl.DateTimeFormat(locale, { hour: "2-digit", minute: "2-digit" });
  const when = item.ongoing && item.end ? text("Until {value0}", { value0: date.format(item.end) }) : `${date.format(item.at)} • ${time.format(item.at)}`;
  const days = item.ongoing && item.end ? daysUntil(item.end, now) : daysUntil(item.at, now);
  const left =
    item.ongoing
      ? days <= 0
        ? text("Ends today")
        : days === 1
          ? text("Ends tomorrow")
          : text("Ends in {count} days", { count: days })
      : days <= 0
        ? text("Today")
        : days === 1
          ? text("Tomorrow")
          : text("{count} days left", { count: days });
  return (
    <li className="flex items-center gap-3 rounded-[12px] border border-gold bg-[#d6d0bc] p-[14px] md:gap-4 md:rounded-[4px] md:bg-warm-gray md:p-4" data-kind={item.kind}>
      <span aria-hidden className="flex size-[38px] shrink-0 items-center justify-center rounded-full border border-gold bg-sage text-[#fbf6e9] md:size-10">
        <Icon className="size-5" strokeWidth={1.5} />
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-[3px] md:gap-0.5">
        <h3 className="text-[15px] font-semibold text-[#17150f] md:text-base md:text-black">{item.title}</h3>
        <p className="text-[10px] text-[var(--muted-ink)] md:text-[13px] md:text-black">{when}</p>
      </div>
      <span className="shrink-0 text-[9px] text-[var(--muted-ink)] md:text-xs md:font-light md:text-black">{left}</span>
    </li>
  );
}
