"use client";
/* eslint-disable @next/next/no-img-element */
import { useState } from "react";
import Link from "next/link";
import { useI18n } from "@/i18n/i18n-provider";
import { formatRating } from "@/lib/format";
import advisors from "@/data/advisors.json";
import { SiteHeader, MobileNavigation } from "@/components/layout/site-header";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";

const previews = [
  "I help you see your path more clearly through Tarot and cosmic guidance.",
  "Channeling ancestral wisdom through crystal energy and dream interpretation.",
  "Ancient wisdom meets modern intuition for life's biggest questions.",
];

export function PsychicsBrowse() {
  const { text, locale } = useI18n();
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
        <div className="advisor-grid">
          {advisors.map((advisor, index) => (
            <article key={advisor.slug} className="advisor-card">
              <img
                className="advisor-card-image"
                src={advisor.image}
                alt={text(advisor.name)}
              />
              <div className="advisor-card-copy">
                <div className="flex items-center justify-between">
                  <h2 className="text-[26px] md:font-display">
                    <span className="hidden text-amber md:inline">• </span>
                    {text(advisor.name)}
                  </h2>
                  <span
                    className={`status-pill md:hidden ${index === 1 ? "" : "online"}`}
                  >
                    {text(index === 1 ? "● Busy" : "● Online")}
                  </span>
                </div>
                <p className="mt-2 text-xs text-sage">
                  {advisor.skills.map((skill) => text(skill)).join(" · ")}
                </p>
                <div className="mt-3 flex items-center justify-between gap-2 text-[11px]">
                  <span>
                    ★{" "}
                    <strong>
                      {text(
                        index === 0
                          ? "4.9 · 350"
                          : index === 1
                            ? "4.8 · 280"
                            : "4.8 · 671",
                      )}{" "}
                      {text("reviews ")}
                    </strong>
                  </span>
                  <span className="font-extralight">
                    {[40, 55, 45][index]} {text("credits/min ")}
                  </span>
                </div>
                <p className="mt-3 text-[13px] leading-snug">
                  {text(previews[index])}
                </p>
                <Link
                  className="advisor-profile-link"
                  href={`/${locale}/psychics/${advisor.slug}`}
                >
                  {text("View full profile ")}
                  <span className="md:hidden">&gt;</span>
                </Link>
              </div>
            </article>
          ))}
        </div>
      </main>
      <MobileNavigation />
    </div>
  );
}

export function PsychicProfile({ slug }: { slug: string }) {
  const { text, locale } = useI18n();
  const advisor = advisors.find((a) => a.slug === slug)!;
  const [service, setService] = useState<
    (typeof advisor.services)[number] | null
  >(null);
  const [notice, setNotice] = useState("");
  return (
    <div className="dashboard-page">
      <SiteHeader />
      <main className="psychic-profile">
        <div className="mb-8 flex items-center justify-between text-sm">
          <div>
            <Link href={`/${locale}/psychics`}>{text("Psychics")}</Link>{" "}
            <span className="mx-2">›</span>{" "}
            <strong>{text(advisor.name)}</strong>
          </div>
          <span className="status-pill">{text("● Online now")}</span>
        </div>
        <section className="profile-hero">
          <div className="relative overflow-hidden rounded-2xl">
            <img
              className="h-full w-full object-cover"
              src={advisor.image}
              alt={text(advisor.name)}
            />
            <span className="absolute bottom-6 left-6 rounded-full bg-black px-4 py-2 text-xs text-white">
              {text("♙ Verified Master Advisor ")}
            </span>
          </div>
          <div className="flex flex-col justify-center gap-6">
            <h1 className="flex items-center gap-4 font-display text-4xl lg:text-[56px]">
              <img
                src={`https://assets.staryield.net/assets/zodiac/${advisor.zodiac.toLowerCase()}.png`}
                alt={text(advisor.zodiac)}
                className="h-12 w-12 object-contain"
              />
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
                [
                  `${formatRating(advisor.rating, locale)} ★`,
                  "Rating",
                  text("{count} reviews", { count: advisor.reviews }),
                ],
                [advisor.readings, "Readings", "Completed"],
                [
                  text("{count} years", { count: advisor.years }),
                  "Experience",
                  "Active practice",
                ],
              ].map(([value, label, detail]) => (
                <div className="stat-tile" key={label}>
                  <p className="font-display text-lg lg:text-[28px]">
                    {text(value)}
                  </p>
                  <p className="mt-2 text-xs uppercase">{text(label)}</p>
                  <p className="text-[11px]">{text(detail)}</p>
                </div>
              ))}
            </div>
            <div className="rounded-2xl border border-gold p-6">
              <div className="flex justify-between gap-3">
                <div>
                  <p className="text-sm uppercase">
                    {text("Consultation rate")}
                  </p>
                  <p>
                    <span className="font-display text-[32px]">
                      {advisor.rate}
                    </span>{" "}
                    <strong className="text-sm">
                      {text("Credits / minute")}
                    </strong>
                  </p>
                </div>
                <p className="self-center text-right text-xs">
                  {text("● Instant Connection ")}
                  <br />
                  {text("No waiting queue ")}
                </p>
              </div>
              <Button asChild className="gold-button mt-5 h-14 w-full">
                <Link href={`/${locale}/chatroom?reader=${slug}`}>
                  {text("♧ Start live chat ")}
                </Link>
              </Button>
            </div>
          </div>
        </section>
        <section className="profile-details">
          <div>
            <h2 className="font-display text-[32px]">{text("About")}</h2>
            {advisor.about.slice(0, -1).map((paragraph, index) => (
              <p className="mt-6 font-extralight leading-[1.8]" key={index}>
                {text(paragraph)}
              </p>
            ))}
            <div className="mt-6 rounded-2xl bg-warm-gray/55 p-6">
              <h3 className="font-display text-xl">{text("My philosophy")}</h3>
              <p className="mt-4 text-sm font-light">
                {text(advisor.about.at(-1))}
              </p>
            </div>
          </div>
          <div>
            <h2 className="mb-6 text-[32px]">{text("Orders / Services")}</h2>
            <div className="space-y-4">
              {advisor.services.map((item) => (
                <article
                  className="rounded-2xl border border-gold p-6"
                  key={item.name}
                >
                  <div className="flex justify-between gap-3">
                    <div>
                      <h3 className="text-xl">{text(item.name)}</h3>
                      <p className="text-xs font-light">
                        {text(item.subtitle)}
                      </p>
                    </div>
                    <span className="status-pill self-start">
                      {text(item.price)}
                    </span>
                  </div>
                  <p className="my-4 text-sm font-extralight leading-relaxed">
                    {text(item.description)}
                  </p>
                  <Button
                    className="h-10 w-full bg-gold"
                    onClick={() => {
                      setService(item);
                      setNotice("");
                    }}
                  >
                    {text("Book Session › ")}
                  </Button>
                </article>
              ))}
            </div>
          </div>
        </section>
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
          <form
            className="space-y-5 p-6"
            onSubmit={(event) => {
              event.preventDefault();
              const data = new FormData(event.currentTarget);
              try {
                localStorage.setItem(
                  "staryield.session-preference",
                  JSON.stringify({
                    advisor: slug,
                    service: service?.name,
                    date: data.get("date"),
                    time: data.get("time"),
                  }),
                );
              } catch {
                setNotice(
                  "Your browser could not save this preference. Please allow local storage and try again.",
                );
                return;
              }
              setNotice(
                "Your session preference is saved locally. Booking will be available when the reading service is connected.",
              );
            }}
          >
            <p>{text(service?.description)}</p>
            <label className="field">
              {text("Preferred date ")}
              <input type="date" name="date" required />
            </label>
            <label className="field">
              {text("Preferred time ")}
              <input type="time" name="time" required />
            </label>
            <p>{text(service?.price)}</p>
            <Button className="gold-button w-full" type="submit">
              {text("Request a session ")}
            </Button>
            <p role="status">{text(notice)}</p>
          </form>
        </SheetContent>
      </Sheet>
    </div>
  );
}
