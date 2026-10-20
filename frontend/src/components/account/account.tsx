"use client";
/* eslint-disable @next/next/no-img-element */
import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Moon, Zap, Sun, CircleX } from "lucide-react";
import { Dialog } from "radix-ui";
import { useI18n } from "@/i18n/i18n-provider";
import { SiteHeader, MobileNavigation } from "@/components/layout/site-header";
import { AstrologicalChart } from "@/components/astrology/AstrologicalChart";
import { Button } from "@/components/ui/button";
import { LocaleSwitcher } from "@/components/layout/locale-switcher";
import { useSession } from "@/lib/auth/session-provider";
import { astrologyApi, meApi } from "@/lib/api/endpoints";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import type { ChartPosition, Horoscope, Place } from "@/lib/api/types";
import { track } from "@/lib/analytics";
import { loginUrl } from "@/lib/navigation";
import { useAsync, type AsyncState } from "@/lib/use-async";
import { useToast } from "@/components/feedback/toast-provider";
import { houseTheme, planetMeaning, planetSymbol, signFlavor } from "@/components/account/chart-copy";
import { ClientOrders, PsychicTools } from "@/components/account/orders";

export type AccountView = "overview" | "chart" | "settings";

function planetAsset(name: string) {
  const file = name === "Ascendant" ? "ascending" : name.toLowerCase();
  return `https://assets.staryield.net/assets/zodiac/${file}-1.${["Sun", "Uranus", "Venus"].includes(name) ? "svg" : "png"}`;
}

function PlanetControl({ planet, onSelect }: { planet: ChartPosition; onSelect: () => void }) {
  const { text } = useI18n();
  return (
    <button className="planet-control" onClick={onSelect}>
      <img src={planetAsset(planet.body)} className="mb-2 h-[22px] w-auto object-contain" alt="" />
      <span className="font-display text-[9px] tracking-[1.5px] uppercase">{text(planet.body)}</span>
      <span className="text-[9px] tracking-wider">{text(planet.formatted)}</span>
      {planet.house_label ? <strong className="text-[9px] tracking-wider uppercase">{text(planet.house_label)}</strong> : null}
    </button>
  );
}

function Notice({ error, onRetry }: { error: ApiError; onRetry?: () => void }) {
  const { text, locale } = useI18n();
  const needsBirthDate = error.code === "validation_failed" && Boolean(error.fields.birth_date);
  return (
    <div className="rounded-xl bg-warm-gray/40 p-6 text-sm" role="alert">
      <p>{text(needsBirthDate ? "Add your birth date in settings to see this." : errorMessage(error))}</p>
      {needsBirthDate ? (
        <Link className="mt-2 inline-block underline" href={`/${locale}/profile/settings`}>
          {text("Open settings")}
        </Link>
      ) : onRetry ? (
        <button type="button" className="mt-2 underline" onClick={onRetry}>
          {text("Try again")}
        </button>
      ) : null}
    </div>
  );
}

export function Account({ view }: { view: AccountView }) {
  const { text, locale } = useI18n();
  const router = useRouter();
  const { status, user, wallet } = useSession();
  const [planet, setPlanet] = useState<ChartPosition | null>(null);

  useEffect(() => {
    if (status === "anonymous") router.replace(loginUrl(locale));
  }, [status, locale, router]);

  const horoscope = useAsync((signal) => meApi.horoscope(undefined, signal), [user?.id, user?.profile.birth_date], Boolean(user) && user?.role === "client");
  const chart = useAsync((signal) => meApi.chart(signal), [user?.id, user?.profile.birth_date, user?.profile.birth_time, user?.profile.birth_place_id], Boolean(user) && view === "chart");

  useEffect(() => {
    if (view === "chart" && chart.data) track("chart_viewed", { has_location: chart.data.has_location });
  }, [view, chart.data]);

  if (!user) {
    return (
      <div className="dashboard-page">
        <SiteHeader />
        <p className="py-24 text-center text-sm" role="status">
          {text("Loading your profile…")}
        </p>
        <MobileNavigation />
      </div>
    );
  }

  const name = user.profile.nickname || user.display_name;
  const affirmation = horoscope.data?.affirmation ?? "I grow spiritually when I accept responsibility for my life. ";
  const positions = (chart.data?.positions ?? []).filter((p) => p.body !== "Midheaven");
  const split = Math.ceil(positions.length / 2);
  const theme = user.profile.theme === "Light Cream" ? "theme-light" : "";

  return (
    <div className={`dashboard-page ${theme}`}>
      <SiteHeader />
      <div className="account-wrap">
        <div className={`affirmation ${view !== "overview" ? "account-affirmation-secondary" : ""}`}>
          <div>
            <strong>{text("Affirmation of the day")}</strong>
            <p>{text(affirmation)}</p>
          </div>
          {user.role === "client" ? (
            <div className="hidden items-center gap-4 md:flex">
              <p className="text-xs">
                {text("balance: ")}
                <br />
                <strong>{text("{value0} credits", { value0: (wallet?.balance ?? 0).toLocaleString(locale) })}</strong>
              </p>
              <Button asChild className="gold-button">
                <Link href={`/${locale}/credits`}>{text("Top up credits")}</Link>
              </Button>
            </div>
          ) : null}
        </div>
        <div className="account-body">
          <aside className="account-sidebar">
            <div className="py-6 text-center">
              <div className="mx-auto flex size-10 items-center justify-center overflow-hidden rounded-full bg-gold text-white">
                {user.profile.avatar_url ? (
                  <img src={user.profile.avatar_url} alt={text("Your avatar")} className="size-full object-cover" />
                ) : (
                  name.charAt(0).toUpperCase()
                )}
              </div>
              <p className="mt-2 text-sm">{name}</p>
            </div>
            {[
              ["Overview", "/profile", "overview"],
              ["Chart", "/profile/chart", "chart"],
              ["Settings", "/profile/settings", "settings"],
            ].map(([label, href, id]) => (
              <Link key={id} className={view === id ? "active" : ""} href={`/${locale}${href}`}>
                {text(label)}
              </Link>
            ))}
          </aside>
          <main className={`account-main account-${view}`}>
            {view === "overview" ? (
              user.role === "psychic" ? (
                <PsychicTools />
              ) : (
                <Overview name={name} horoscope={horoscope} />
              )
            ) : view === "settings" ? (
              <>
                <ProfileTabs view={view} />
                <Settings />
              </>
            ) : (
              <>
                <ProfileTabs view={view} />
                <div className="chart-mobile-intro">
                  <p className="eyebrow">{name}</p>
                  {chart.data ? (
                    <h2 className="mt-3 text-[28px]">
                      {text("{value0} sun", { value0: text(chart.data.sun_sign) })}
                      {chart.data.rising_sign ? ` · ${text("{value0} rising", { value0: text(chart.data.rising_sign) })}` : ""}
                    </h2>
                  ) : null}
                  <p className="mt-2 text-sm font-light">
                    {text("Your natal chart is a map of the sky at the precise moment you arrived. ")}
                  </p>
                </div>
                {chart.error ? (
                  <Notice error={chart.error} onRetry={chart.reload} />
                ) : chart.loading ? (
                  <p className="py-12 text-center text-sm" role="status">{text("Calculating your chart…")}</p>
                ) : (
                  <div className="account-chart-grid">
                    <div className="planet-column">
                      {positions.slice(0, split).map((p) => (
                        <PlanetControl key={p.body} planet={p} onSelect={() => setPlanet(p)} />
                      ))}
                    </div>
                    <div className="account-chart-art">
                      <AstrologicalChart />
                    </div>
                    <div className="planet-column right">
                      {positions.slice(split).map((p) => (
                        <PlanetControl key={p.body} planet={p} onSelect={() => setPlanet(p)} />
                      ))}
                    </div>
                  </div>
                )}
                {chart.data ? (
                  <p className="mt-4 text-center text-xs text-sage">
                    {text(chart.data.time_known ? "Positions use your birth time and place (equal houses)." : "Add your birth time and place in settings for your rising sign and houses.")}{" "}
                    {text(chart.data.disclaimer)}
                  </p>
                ) : null}
              </>
            )}
          </main>
        </div>
      </div>
      <MobileNavigation />
      <Dialog.Root open={Boolean(planet)} onOpenChange={(open) => !open && setPlanet(null)}>
        <Dialog.Portal>
          <Dialog.Overlay className="planet-overlay" />
          <Dialog.Content className="planet-popup">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-3">
                <span className="text-[22px]" aria-hidden="true">
                  {planet ? planetSymbol[planet.body] : ""}
                </span>
                <div>
                  <Dialog.Title className="font-display text-[9px] tracking-[1.5px] uppercase">{text(planet?.body)}</Dialog.Title>
                  <p className="mt-0.5 text-[9px] tracking-wider uppercase">
                    {text(planet?.formatted)}
                    {planet?.retrograde ? ` · ${text("Retrograde")}` : ""}
                  </p>
                </div>
              </div>
              <Dialog.Close asChild>
                <button type="button" aria-label={text("Close planetary insight")}>
                  <CircleX size={20} />
                </button>
              </Dialog.Close>
            </div>
            <Dialog.Description className="min-h-[156px] text-[13px] leading-5">
              {planet ? (
                <>
                  {text(planetMeaning[planet.body] ?? "")} {text(signFlavor[planet.sign] ?? "")}{" "}
                  {planet.house && planet.house_label
                    ? text("In the {value0}, it is often expressed through {value1}.", {
                        value0: text(planet.house_label).toLowerCase(),
                        value1: text(houseTheme[planet.house]),
                      })
                    : ""}
                </>
              ) : null}
            </Dialog.Description>
            <Button asChild className="h-11 w-full rounded-none bg-amber font-display text-xs leading-tight whitespace-normal">
              <Link href={`/${locale}/psychics`}>{text("Get more insight from one of our advisors ")}</Link>
            </Button>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </div>
  );
}

function ProfileTabs({ view }: { view: AccountView }) {
  const { text, locale } = useI18n();
  return (
    <div className="profile-tabs">
      <h1>{text("Your cosmic profile")}</h1>
      <nav>
        <Link href={`/${locale}/profile/settings`} className={view === "settings" ? "active" : ""}>
          {text("Settings ")}
        </Link>
        <Link href={`/${locale}/profile/chart`} className={view === "chart" ? "active" : ""}>
          {text("Astrological chart ")}
        </Link>
      </nav>
    </div>
  );
}

function greeting(hour: number) {
  if (hour < 12) return "Good morning, ";
  if (hour < 18) return "Good afternoon, ";
  return "Good evening, ";
}

function daysUntil(iso: string) {
  return Math.max(0, Math.ceil((new Date(iso).getTime() - Date.now()) / 86_400_000));
}

function Overview({ name, horoscope }: { name: string; horoscope: AsyncState<Horoscope> }) {
  const { text, locale } = useI18n();
  const { user } = useSession();
  const events = useAsync((signal) => astrologyApi.events(signal), []);
  const [now] = useState(() => new Date());
  const h = horoscope.data;

  useEffect(() => {
    if (h) track("horoscope_viewed", { sign: h.sign });
  }, [h]);

  const dateLong = (iso: string) =>
    new Intl.DateTimeFormat(locale, { month: "long", day: "numeric", year: "numeric", hour: "2-digit", minute: "2-digit" }).format(new Date(iso));

  const cosmic = useMemo(() => {
    const lunar = (events.data?.lunar ?? [])
      .filter((e) => e.kind === "full_moon" || e.kind === "new_moon")
      .slice(0, 2)
      .map((e) => ({
        icon: Moon,
        title: text(e.kind === "full_moon" ? "Full Moon in {value0}" : "New Moon in {value0}", { value0: text(e.sign) }),
        at: e.at,
      }));
    const retro = (events.data?.retrogrades ?? []).slice(0, 1).map((r) => ({
      icon: Zap,
      title: text("{value0} Retrograde", { value0: text(r.body) }),
      at: r.start,
    }));
    return [...lunar, ...retro].sort((a, b) => a.at.localeCompare(b.at));
  }, [events.data, text]);

  return (
    <>
      <div className="overview-top">
        <div>
          <h1 className="text-[28px] font-semibold">
            {text(greeting(now.getHours()))}
            {name}
          </h1>
          <p className="text-sm font-light">
            {text("Today is {value0}", { value0: new Intl.DateTimeFormat(locale, { dateStyle: "long" }).format(now) })}
          </p>
        </div>
        <div className="overview-stats">
          {[
            ["Compatibility score", h ? `${h.compatibility}%` : "—", h?.compatibility_label ?? ""],
            ["Lucky number", h ? String(h.lucky_number) : "—", ""],
            ["Mood", user?.profile.mood ?? h?.mood ?? "", ""],
          ].map(([label, value, note]) => (
            <div className="stat-tile" key={label}>
              <p className="text-[10px] uppercase">{text(label)}</p>
              <p className="mt-2 text-2xl">{text(value)}</p>
              {note && <span className="text-xs">{text(note)}</span>}
            </div>
          ))}
        </div>
        <Sun className="overview-sun" />
      </div>
      <div className="horoscope">
        <div>
          <h2 className="text-xl font-semibold">{text("Your horoscope")}</h2>
          {h ? <p className="text-xs">{new Intl.DateTimeFormat(locale, { dateStyle: "long" }).format(new Date(`${h.date}T12:00:00`))}</p> : null}
        </div>
        {horoscope.error ? (
          <Notice error={horoscope.error} onRetry={horoscope.reload} />
        ) : h ? (
          <p className="font-light leading-relaxed">{text(h.text)}</p>
        ) : (
          <p className="text-sm" role="status">{text("Reading the sky…")}</p>
        )}
      </div>
      <p className="eyebrow mb-2 md:hidden">{text("Celestial calendar")}</p>
      <h2 className="mb-4 text-xl font-semibold max-md:text-[32px]">{text("Upcoming Cosmic Events ")}</h2>
      <div className="cosmic-events">
        <div className="space-y-3">
          {events.error ? <Notice error={events.error} onRetry={events.reload} /> : null}
          {cosmic.map(({ icon: EventIcon, title, at }) => (
            <div className="event-card" key={title + at}>
              <span className="event-icon">
                <EventIcon size={23} strokeWidth={1.5} />
              </span>
              <div>
                <h3 className="font-semibold">{title}</h3>
                <p className="text-xs">{dateLong(at)}</p>
              </div>
              <span className="ml-auto text-[10px] font-light">{text("{count} days left", { count: daysUntil(at) })}</span>
            </div>
          ))}
        </div>
        <div className="focus-card">
          <h3 className="text-sm font-semibold">{text("✦ What to focus on today")}</h3>
          <p className="mt-4 text-lg font-light">{text(h?.focus ?? "Take a moment to reflect on your intentions today.")}</p>
        </div>
      </div>
      <ClientOrders />
    </>
  );
}

function Settings() {
  const { text, locale } = useI18n();
  const { user, setUser } = useSession();
  const toast = useToast();
  const [error, setError] = useState<ApiError | null>(null);
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [placeQuery, setPlaceQuery] = useState(user?.profile.birth_place ?? "");
  const [placeId, setPlaceId] = useState(user?.profile.birth_place_id ?? "");
  const [places, setPlaces] = useState<Place[]>([]);

  const searchable = placeQuery.trim().length >= 2 && placeQuery !== user?.profile.birth_place && !placeId;
  useEffect(() => {
    if (!searchable) return;
    const ctrl = new AbortController();
    const t = window.setTimeout(() => {
      astrologyApi
        .places(placeQuery, ctrl.signal)
        .then(setPlaces)
        .catch(() => setPlaces([]));
    }, 250);
    return () => {
      window.clearTimeout(t);
      ctrl.abort();
    };
  }, [placeQuery, searchable]);
  const visiblePlaces = searchable ? places : [];

  if (!user) return null;
  const p = user.profile;
  const fieldError = (name: string) =>
    error?.fields[name] ? (
      <span className="text-xs text-red-800" role="alert">
        {text(error.fields[name])}
      </span>
    ) : null;

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    setSaving(true);
    setError(null);
    try {
      const updated = await meApi.update({
        display_name: String(data.get("name") ?? "").trim(),
        nickname: String(data.get("nickname") ?? ""),
        birth_date: String(data.get("birth") ?? ""),
        birth_time: String(data.get("birth_time") ?? ""),
        birth_place_id: placeQuery.trim() ? placeId : "",
        location: String(data.get("location") ?? ""),
        mood: String(data.get("mood")),
        theme: String(data.get("theme")),
        notify_horoscope: data.get("horoscope") === "on",
        notify_events: data.get("events") === "on",
        notify_chart: data.get("chart") === "on",
      });
      setUser(updated);
      toast.show({ tone: "success", message: text("Changes saved.") });
    } catch (err) {
      const e = toApiError(err);
      setError(e);
      toast.showError(e);
    } finally {
      setSaving(false);
    }
  }

  async function upload(file: File) {
    if (file.size > 1024 * 1024) {
      toast.show({ tone: "error", message: text("Choose an image smaller than 1 MB.") });
      return;
    }
    setUploading(true);
    try {
      setUser(await meApi.uploadAvatar(file));
      toast.show({ tone: "success", message: text("Profile picture updated.") });
    } catch (err) {
      toast.showError(err);
    } finally {
      setUploading(false);
    }
  }

  return (
    <form onSubmit={submit} className="settings-form" key={user.id}>
      <h2 className="mb-4 text-xl font-semibold">{text("Account Settings")}</h2>
      <div className="settings-fields">
        <label className="field">
          {text("Full Name")}
          <input type="text" name="name" defaultValue={user.display_name} required maxLength={80} />
          {fieldError("display_name")}
        </label>
        <label className="field">
          {text("Date of Birth")}
          <input type="date" name="birth" defaultValue={p.birth_date} />
          {fieldError("birth_date")}
        </label>
        <label className="field">
          {text("Email Address")}
          <input type="email" name="email" defaultValue={user.email} readOnly aria-readonly />
        </label>
        <label className="field">
          {text("Birth time")}
          <input type="time" name="birth_time" defaultValue={p.birth_time} />
          {fieldError("birth_time")}
        </label>
        <div className="field relative">
          <label htmlFor="birth-place">{text("Birth place")}</label>
          <input
            id="birth-place"
            autoComplete="off"
            value={placeQuery}
            placeholder={text("Search a city")}
            onChange={(e) => {
              setPlaceQuery(e.target.value);
              setPlaceId("");
            }}
            aria-autocomplete="list"
            aria-controls="birth-place-options"
          />
          {visiblePlaces.length > 0 ? (
            <ul id="birth-place-options" role="listbox" className="place-options">
              {visiblePlaces.map((pl) => (
                <li key={pl.id} role="option" aria-selected={pl.id === placeId}>
                  <button
                    type="button"
                    onClick={() => {
                      setPlaceId(pl.id);
                      setPlaceQuery(`${pl.name}, ${pl.country}`);
                      setPlaces([]);
                    }}
                  >
                    {pl.name}, {pl.country}
                  </button>
                </li>
              ))}
            </ul>
          ) : null}
          {fieldError("birth_place_id")}
        </div>
        <label className="field">
          {text("Profile Picture ")}
          <input
            type="file"
            accept="image/png,image/jpeg,image/webp"
            disabled={uploading}
            onChange={(event) => {
              const file = event.target.files?.[0];
              if (file) void upload(file);
              event.target.value = "";
            }}
          />
        </label>
        <div className="field">
          {text("Password ")}
          <p className="rounded-lg border border-gold bg-cream px-3 py-2 text-sm">
            ••••••••••{" "}
            <Link href={`/${locale}/forgot-password`} className="float-right underline">
              {text("Change ")}
            </Link>
          </p>
        </div>
        <label className="field">
          {text("Location ")}
          <select name="location" defaultValue={p.location}>
            <option value="">{text("Select Location")}</option>
            {["Brazil", "France", "United Kingdom", "United States", "Other"].map((country) => (
              <option key={country} value={country}>
                {text(country)}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          {text("Nickname ")}
          <input name="nickname" placeholder={text("Nickname")} defaultValue={p.nickname} maxLength={40} />
          {fieldError("nickname")}
        </label>
        <label className="field">
          {text("Set mood ")}
          <select name="mood" defaultValue={p.mood}>
            {["Empowered", "Hopeful", "Reflective", "Calm", "Curious"].map((mood) => (
              <option key={mood} value={mood}>
                {text(mood)}
              </option>
            ))}
          </select>
        </label>
      </div>
      <div className="settings-preferences">
        <div>
          <h2 className="mb-4 text-xl font-semibold">{text("Notification Preferences ")}</h2>
          <div className="preference-card">
            {[
              ["Daily Horoscope", "horoscope", p.notify_horoscope],
              ["Cosmic events", "events", p.notify_events],
              ["Changes in your chart", "chart", p.notify_chart],
            ].map(([label, name, value]) => (
              <label className="notification-row" key={String(name)}>
                <span>{text(String(label))}</span>
                <input className="notification-switch" type="checkbox" name={String(name)} defaultChecked={Boolean(value)} />
              </label>
            ))}
          </div>
        </div>
        <div>
          <h2 className="mb-4 text-xl font-semibold">{text("Display Settings")}</h2>
          <div className="preference-card flex gap-6">
            <label className="field flex-1">
              {text("Theme Preference ")}
              <select name="theme" defaultValue={p.theme}>
                <option value="Warm Beige">{text("Warm Beige")}</option>
                <option value="Light Cream">{text("Light Cream")}</option>
              </select>
            </label>
            <div className="field flex-1">
              {text("Language ")}
              <LocaleSwitcher />
            </div>
          </div>
        </div>
      </div>
      <p role="alert" className="mt-6 text-sm text-red-800">
        {error && Object.keys(error.fields).length === 0 ? text(errorMessage(error)) : ""}
      </p>
      <div className="mt-6 text-right">
        <Button type="submit" className="gold-button h-11 px-8" disabled={saving} aria-busy={saving}>
          {text(saving ? "Saving…" : "Save changes ")}
        </Button>
      </div>
    </form>
  );
}
