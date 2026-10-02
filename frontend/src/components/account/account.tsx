"use client";
/* eslint-disable @next/next/no-img-element */
import { useState } from "react";
import Link from "next/link";
import { Moon, Zap, Sun } from "lucide-react";
import { useI18n } from "@/i18n/i18n-provider";
import { useAccountPreferences } from "@/lib/account-preferences";
import { SiteHeader, MobileNavigation } from "@/components/layout/site-header";
import { AstrologicalChart } from "@/components/astrology/AstrologicalChart";
import { Button } from "@/components/ui/button";
import { LocaleSwitcher } from "@/components/layout/locale-switcher";
import { Dialog } from "radix-ui";
import { CircleX } from "lucide-react";

export type AccountView = "overview" | "chart" | "settings";
const planets = [
  {
    name: "Ascendant",
    symbol: "↑",
    position: "Cancer, 8°22′47″",
    house: "First house",
    description:
      "Your ascendant is the “mask” you present when meeting others. With Cancer rising, you appear gentle, intuitive, and warm, though some may see you as sensitive or emotional. Because the ascendant changes every two hours, confirm your birth time if this does not resonate.",
  },
  {
    name: "Sun",
    symbol: "☉",
    position: "Virgo, 3°14′22″",
    house: "Third house",
    description:
      "The Sun reflects your core identity, vitality, and conscious self. In Virgo, you are analytical, detail-oriented, and motivated to improve yourself and support others. In the third house, you express this identity through communication, learning, and your immediate environment.",
  },
  {
    name: "Mercury",
    symbol: "☿",
    position: "Virgo, 18°42′5″",
    house: "Third house",
    description:
      "Mercury governs how you think, communicate, and process information. In Virgo, your mind is precise, practical, and highly analytical; you notice details others miss, though perfectionism can surface. In the third house, this energy thrives through conversation, local connections, travel, and everyday learning.",
  },
  {
    name: "Mars",
    symbol: "♂",
    position: "Leo, 25°4′2″",
    house: "Third house",
    description:
      "Mars shapes how you assert yourself, take action, pursue ambition, and express anger or desire. In Leo, you act with confidence, authority, and persuasive energy. In the third house, you direct that drive toward communication, familiar subjects, and your immediate environment.",
  },
  {
    name: "Jupiter",
    symbol: "♃",
    position: "Gemini, 18°33′15″",
    house: "Twelfth house",
    description:
      "Jupiter rules luck, growth, and opportunity. In Gemini, abundance comes through curiosity, communication, learning, and sharing ideas. In the twelfth house, much of this expansion happens privately through solitude, reflection, and inner exploration.",
  },
  {
    name: "Venus",
    symbol: "♀",
    position: "Libra, 14°55′9″",
    house: "Fourth house",
    description:
      "Venus rules love, beauty, and attraction. In Libra, you are romantic, idealistic, and willing to compromise for an equal partnership, though realism and consistency may be challenges. In the fourth house, you most naturally express love through home and family.",
  },
  {
    name: "Saturn",
    symbol: "♄",
    position: "Taurus, 7°29′41″",
    house: "Eleventh house",
    description:
      "Saturn represents discipline, structure, and the challenges that lead to mastery. In Taurus, your lessons center on security, resources, and self-worth; overcoming scarcity thinking or stubbornness helps you build lasting stability. In the eleventh house, this growth unfolds through friendships, community, and long-term goals while maintaining healthy boundaries.",
  },
  {
    name: "Uranus",
    symbol: "♅",
    position: "Aquarius, 27°45′7″",
    house: "Eighth house",
    description:
      "Uranus is a generational influence tied to innovation, rebellion, and progress. In Aquarius, your generation challenges convention through intellectual independence and detachment. In the eighth house, you personally question outdated expectations around taboo subjects, intimacy, rebirth, and transformation.",
  },
  {
    name: "Neptune",
    symbol: "♆",
    position: "Aquarius, 4°8′23″",
    house: "Eighth house",
    description:
      "Neptune is a generational influence tied to dreams, imagination, and the unconscious. In Aquarius, your generation seeks inspiration through ideas and detached analysis. In the eighth house, this can create idealism—or impractical expectations—around intimacy, taboos, rebirth, and transformation.",
  },
  {
    name: "Pluto",
    symbol: "♇",
    position: "Sagittarius, 12°17′44″",
    house: "Sixth house",
    description:
      "Pluto is a generational force of power, rebirth, and transformation. In Sagittarius, your generation reshapes belief systems, philosophy, and the search for truth. In the sixth house, you experience deep personal change through daily routines, health, work, service, and discipline.",
  },
  {
    name: "Moon",
    symbol: "☽",
    position: "Scorpio, 12°45′0″",
    house: "Fifth house",
    description:
      "The Moon in Capricorn guards its tender side with composure. You feel safest with consistency, practical care, and promises that are kept. Though you reveal emotion slowly, devotion runs deep; intimacy grows when you let a partner support you as capably as you support them.",
  },
];
function PlanetControl({
  planet,
  onSelect,
}: {
  planet: (typeof planets)[number];
  onSelect: () => void;
}) {
  return (
    <button className="planet-control" onClick={onSelect}>
      <img
        src={`https://assets.staryield.net/assets/zodiac/${planet.name === "Ascendant" ? "ascending" : planet.name.toLowerCase()}-1.${["Sun", "Uranus", "Venus"].includes(planet.name) ? "svg" : "png"}`}
        className="mb-2 h-[22px] w-auto object-contain"
        alt=""
      />
      <span className="font-display text-[9px] tracking-[1.5px] uppercase">
        {planet.name}
      </span>
      <span className="text-[9px] tracking-wider">{planet.position}</span>
      <strong className="text-[9px] tracking-wider uppercase">
        {planet.house}
      </strong>
    </button>
  );
}
export function Account({ view }: { view: AccountView }) {
  const { locale } = useI18n();
  const { preferences } = useAccountPreferences();
  const [planet, setPlanet] = useState<(typeof planets)[number] | null>(null);
  return (
    <div
      className={`dashboard-page ${preferences.theme === "Light Cream" ? "theme-light" : ""}`}
    >
      <SiteHeader />
      <div className="account-wrap">
        {view === "overview" && (
          <div className="affirmation">
            <div>
              <strong>Affirmation of the day</strong>
              <p>
                I grow spiritually when I accept responsibility for my life.
              </p>
            </div>
            <div className="hidden items-center gap-4 md:flex">
              <p className="text-xs">
                balance:
                <br />
                <strong>150 credits</strong>
              </p>
              <Button asChild className="gold-button">
                <Link href={`/${locale}/credits`}>Top up credits</Link>
              </Button>
            </div>
          </div>
        )}
        {view !== "overview" && (
          <div className="affirmation account-affirmation-secondary">
            <div>
              <strong>Affirmation of the day</strong>
              <p>
                I grow spiritually when I accept responsibility for my life.
              </p>
            </div>
            <Button asChild className="gold-button">
              <Link href={`/${locale}/credits`}>Top up credits</Link>
            </Button>
          </div>
        )}
        <div className="account-body">
          <aside className="account-sidebar">
            <div className="py-6 text-center">
              <div className="mx-auto flex size-10 items-center justify-center overflow-hidden rounded-full bg-gold text-white">
                {preferences.avatar ? (
                  <img src={preferences.avatar} alt="Your avatar" />
                ) : (
                  preferences.name.charAt(0)
                )}
              </div>
              <p className="mt-2 text-sm">{preferences.name}</p>
            </div>
            {[
              ["Overview", "/profile", "overview"],
              ["Chart", "/profile/chart", "chart"],
              ["Settings", "/profile/settings", "settings"],
            ].map(([label, href, id]) => (
              <Link
                key={id}
                className={view === id ? "active" : ""}
                href={`/${locale}${href}`}
              >
                {label}
              </Link>
            ))}
          </aside>
          <main className={`account-main account-${view}`}>
            {view === "overview" ? (
              <Overview />
            ) : view === "settings" ? (
              <>
                <ProfileTabs view={view} />
                <Settings />
              </>
            ) : (
              <>
                <ProfileTabs view={view} />
                <div className="chart-mobile-intro">
                  <p className="eyebrow">Cassandra Moon</p>
                  <h2 className="mt-3 text-[28px]">
                    Aries sun · Cancer rising
                  </h2>
                  <p className="mt-2 text-sm font-light">
                    Your natal chart is a map of the sky at the precise moment
                    you arrived.
                  </p>
                </div>
                <div className="account-chart-grid">
                  <div className="planet-column">
                    {planets.slice(0, 6).map((p) => (
                      <PlanetControl
                        key={p.name}
                        planet={p}
                        onSelect={() => setPlanet(p)}
                      />
                    ))}
                  </div>
                  <div className="account-chart-art">
                    <AstrologicalChart />
                  </div>
                  <div className="planet-column right">
                    {planets.slice(6).map((p) => (
                      <PlanetControl
                        key={p.name}
                        planet={p}
                        onSelect={() => setPlanet(p)}
                      />
                    ))}
                  </div>
                </div>
                <p className="mt-4 text-center text-xs text-sage">
                  Sample birth chart. Your personalized chart will appear when
                  the astrology service is connected.
                </p>
              </>
            )}
          </main>
        </div>
      </div>
      <MobileNavigation />
      <Dialog.Root
        open={Boolean(planet)}
        onOpenChange={(open) => {
          if (!open) setPlanet(null);
        }}
      >
        <Dialog.Portal>
          <Dialog.Overlay className="planet-overlay" />
          <Dialog.Content className="planet-popup">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-3">
                <span className="text-[22px]" aria-hidden="true">
                  {planet?.symbol}
                </span>
                <div>
                  <Dialog.Title className="font-display text-[9px] tracking-[1.5px] uppercase">
                    {planet?.name}
                  </Dialog.Title>
                  <p className="mt-0.5 text-[9px] tracking-wider uppercase">
                    {planet?.position}
                  </p>
                </div>
              </div>
              <Dialog.Close asChild>
                <button type="button" aria-label="Close planetary insight">
                  <CircleX size={20} />
                </button>
              </Dialog.Close>
            </div>
            <Dialog.Description className="min-h-[156px] text-[13px] leading-5">
              {planet?.description}
            </Dialog.Description>
            <Button
              asChild
              className="h-11 w-full rounded-none bg-amber font-display text-xs leading-tight whitespace-normal"
            >
              <Link href={`/${locale}/psychics`}>
                Get more insight from one of our advisors
              </Link>
            </Button>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </div>
  );
}
function ProfileTabs({ view }: { view: AccountView }) {
  const { locale } = useI18n();
  return (
    <div className="profile-tabs">
      <h1>Your cosmic profile</h1>
      <nav>
        <Link
          href={`/${locale}/profile/settings`}
          className={view === "settings" ? "active" : ""}
        >
          Settings
        </Link>
        <Link
          href={`/${locale}/profile/chart`}
          className={view === "chart" ? "active" : ""}
        >
          Astrological chart
        </Link>
      </nav>
    </div>
  );
}
function Overview() {
  const { preferences } = useAccountPreferences();
  return (
    <>
      <div className="overview-top">
        <div>
          <h1 className="text-[28px] font-semibold">
            Good morning, {preferences.nickname || preferences.name}
          </h1>
          <p className="text-sm font-light">Today is October 1, 2026</p>
        </div>
        <div className="overview-stats">
          {[
            ["Compatibility score", "88%", "High"],
            ["Lucky number", "17", ""],
            ["Mood", preferences.mood, ""],
          ].map(([label, value, note]) => (
            <div className="stat-tile" key={label}>
              <p className="text-[10px] uppercase">{label}</p>
              <p className="mt-2 text-2xl">{value}</p>
              {note && <span className="text-xs">{note}</span>}
            </div>
          ))}
        </div>
        <Sun className="overview-sun" />
      </div>
      <div className="horoscope">
        <div>
          <h2 className="text-xl font-semibold">Your horoscope</h2>
          <p className="text-xs">July 30, 2026</p>
        </div>
        <p className="font-light leading-relaxed">
          Today, Aries, the energy of the Aquarius moon ignites your adventurous
          spirit. You may feel an irresistible urge to break free from routine
          and explore new ideas or experiences. Embrace this innovative energy
          and let your creative juices flow. This day is perfect for
          brainstorming, collaborating with others, or engaging in activities
          that spark your curiosity. The Aquarius moon encourages you to think
          outside the box, making it an ideal time for group projects or
          community involvement.
        </p>
      </div>
      <p className="eyebrow mb-2 md:hidden">Celestial calendar</p>
      <h2 className="mb-4 text-xl font-semibold max-md:text-[32px]">
        Upcoming Cosmic Events
      </h2>
      <div className="cosmic-events">
        <div className="space-y-3">
          {[
            [
              Moon,
              "Full Moon in Aquarius",
              "August 1, 2026 · 02:30 AM",
              "2 days left",
            ],
            [
              Zap,
              "Mercury Retrograde",
              "August 10, 2026 · 09:00 AM",
              "11 days left",
            ],
          ].map(([Icon, title, date, remaining]) => {
            const EventIcon = Icon as typeof Moon;
            return (
              <div className="event-card" key={String(title)}>
                <span className="event-icon">
                  <EventIcon size={23} strokeWidth={1.5} />
                </span>
                <div>
                  <h3 className="font-semibold">{String(title)}</h3>
                  <p className="text-xs">{String(date)}</p>
                </div>
                <span className="ml-auto text-[10px] font-light">
                  {String(remaining)}
                </span>
              </div>
            );
          })}
        </div>
        <div className="focus-card">
          <h3 className="text-sm font-semibold">✦ What to focus on today</h3>
          <p className="mt-4 text-lg font-light">
            &quot;The stars are aligning in your favor today. Take a moment to
            reflect on your intentions and visualize your success. The universe
            is listening.&quot;
          </p>
        </div>
      </div>
      <p className="mt-4 text-xs text-sage">
        Horoscope and celestial calendar shown with sample content from the
        design.
      </p>
    </>
  );
}
function Settings() {
  const { locale } = useI18n();
  const { preferences, save } = useAccountPreferences();
  const [notice, setNotice] = useState("");
  const [avatar, setAvatar] = useState<string | null>(null);
  function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    try {
      save({
        name: String(data.get("name")),
        email: String(data.get("email")),
        nickname: String(data.get("nickname")),
        birth: String(data.get("birth")),
        location: String(data.get("location")),
        mood: String(data.get("mood")),
        theme: String(data.get("theme")),
        horoscope: data.get("horoscope") === "on",
        events: data.get("events") === "on",
        chart: data.get("chart") === "on",
        avatar: avatar ?? preferences.avatar,
      });
      setNotice("Changes saved on this device.");
    } catch {
      setNotice(
        "Your browser could not save the changes. Please allow local storage and try again.",
      );
    }
  }
  return (
    <form
      onSubmit={submit}
      className="settings-form"
      key={preferences.name + preferences.email}
    >
      <h2 className="mb-4 text-xl font-semibold">Account Settings</h2>
      <div className="settings-fields">
        {[
          ["Full Name", "name", "text", preferences.name],
          ["Date of Birth", "birth", "date", preferences.birth],
          ["Email Address", "email", "email", preferences.email],
        ].map(([label, name, type, value]) => (
          <label className="field" key={name}>
            {label}
            <input
              type={type}
              name={name}
              defaultValue={value}
              required={name === "name"}
            />
          </label>
        ))}
        <label className="field">
          Profile Picture
          <input
            type="file"
            accept="image/png,image/jpeg,image/webp"
            onChange={(event) => {
              const file = event.target.files?.[0];
              if (!file) return;
              if (file.size > 1024 * 1024) {
                setNotice("Choose an image smaller than 1 MB.");
                return;
              }
              const reader = new FileReader();
              reader.onload = () => setAvatar(String(reader.result));
              reader.readAsDataURL(file);
            }}
          />
        </label>
        <div className="field">
          Password
          <p className="rounded-lg border border-gold bg-cream px-3 py-2 text-sm">
            ••••••••••{" "}
            <Link
              href={`/${locale}/forgot-password`}
              className="float-right underline"
            >
              Change
            </Link>
          </p>
        </div>
        <label className="field">
          Location
          <select name="location" defaultValue={preferences.location}>
            <option value="">Select Location</option>
            {[
              "Brazil",
              "France",
              "United Kingdom",
              "United States",
              "Other",
            ].map((country) => (
              <option key={country}>{country}</option>
            ))}
          </select>
        </label>
        <label className="field">
          Nickname
          <input
            name="nickname"
            placeholder="Nickname"
            defaultValue={preferences.nickname}
          />
        </label>
        <label className="field">
          Set mood
          <select name="mood" defaultValue={preferences.mood}>
            {["Empowered", "Hopeful", "Reflective", "Calm", "Curious"].map(
              (mood) => (
                <option key={mood}>{mood}</option>
              ),
            )}
          </select>
        </label>
      </div>
      <div className="settings-preferences">
        <div>
          <h2 className="mb-4 text-xl font-semibold">
            Notification Preferences
          </h2>
          <div className="preference-card">
            {[
              ["Daily Horoscope", "horoscope", preferences.horoscope],
              ["Cosmic events", "events", preferences.events],
              ["Changes in your chart", "chart", preferences.chart],
            ].map(([label, name, value]) => (
              <label className="notification-row" key={String(name)}>
                <span>{String(label)}</span>
                <input
                  className="notification-switch"
                  type="checkbox"
                  name={String(name)}
                  defaultChecked={Boolean(value)}
                />
              </label>
            ))}
          </div>
        </div>
        <div>
          <h2 className="mb-4 text-xl font-semibold">Display Settings</h2>
          <div className="preference-card flex gap-6">
            <label className="field flex-1">
              Theme Preference
              <select name="theme" defaultValue={preferences.theme}>
                <option>Warm Beige</option>
                <option>Light Cream</option>
              </select>
            </label>
            <div className="field flex-1">
              Language
              <LocaleSwitcher />
            </div>
          </div>
        </div>
      </div>
      <p role="status" className="mt-6 text-sm">
        {notice}
      </p>
      <div className="mt-6 text-right">
        <Button type="submit" className="gold-button h-11 px-8">
          Save changes
        </Button>
      </div>
    </form>
  );
}
