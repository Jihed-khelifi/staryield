"use client";
/* eslint-disable @next/next/no-img-element */
import { useEffect } from "react";
import Link from "next/link";
import dynamic from "next/dynamic";
import { useRouter } from "next/navigation";
import { LogOut } from "lucide-react";
import { useI18n } from "@/i18n/i18n-provider";
import { SiteHeader, MobileNavigation } from "@/components/layout/site-header";
import { Button } from "@/components/ui/button";
import { useSession } from "@/lib/auth/session-provider";
import { loginUrl } from "@/lib/navigation";
import { PsychicOverview } from "@/components/psychic/psychic-overview";
// FE-CALC: the client overview and chart tab live in their own modules.
import { Overview, useMyHoroscope } from "@/components/account/overview/overview";
import { ChartTab } from "@/components/account/chart/chart-tab";
import { AccountSettings, SharedAccountSettings } from "@/components/account/settings/account-settings";
import { PsychicSettings } from "@/components/psychic/settings/psychic-settings"; // advisor settings

export type AccountView = "overview" | "chart" | "settings" | "analytics";

// Chart.js is only needed on the advisor analytics view.
const PsychicAnalyticsView = dynamic(() => import("@/components/analytics/psychic-analytics").then((m) => m.PsychicAnalyticsView), {
  ssr: false,
  loading: () => <AnalyticsLoading />,
});

function AnalyticsLoading() {
  const { text } = useI18n();
  return (
    <p className="py-12 text-center text-sm" role="status">
      {text("Loading your analytics…")}
    </p>
  );
}

export function Account({ view }: { view: AccountView }) {
  const { text, locale } = useI18n();
  const router = useRouter();
  const { status, user, wallet, signOut } = useSession();

  useEffect(() => {
    if (status === "anonymous") router.replace(loginUrl(locale));
  }, [status, locale, router]);

  // Analytics are for advisors only.
  useEffect(() => {
    if (view === "analytics" && user && user.role !== "psychic") router.replace(`/${locale}/profile`);
  }, [view, user, locale, router]);

  const horoscope = useMyHoroscope(user);

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
  const theme = user.profile.theme === "Light Cream" ? "theme-light" : "";

  return (
    <div className={`dashboard-page ${theme}`}>
      <SiteHeader />
      <div className="account-wrap">
        {/* Advisors go straight to their stats (Figma 1174:2294). */}
        <div className={`affirmation ${view !== "overview" ? "account-affirmation-secondary" : ""}`} hidden={user.role === "psychic"}>
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
            {(user.role === "psychic"
              ? [
                  ["Overview", "/profile", "overview"],
                  ["Analytics", "/profile/analytics", "analytics"],
                  ["Settings", "/profile/settings", "settings"],
                ]
              : [
                  ["Overview", "/profile", "overview"],
                  ["Chart", "/profile/chart", "chart"],
                  ["Settings", "/profile/settings", "settings"],
                ]
            ).map(([label, href, id]) => (
              <Link key={id} className={view === id ? "active" : ""} aria-current={view === id ? "page" : undefined} href={`/${locale}${href}`}>
                {text(label)}
              </Link>
            ))}
            <button type="button" className="account-signout" onClick={() => void signOut()}>
              <LogOut size={15} aria-hidden />
              {text("Sign out")}
            </button>
          </aside>
          <main className={`account-main account-${view}`}>
            {view === "overview" ? (
              user.role === "psychic" ? (
                <PsychicOverview />
              ) : (
                <Overview name={name} user={user} horoscope={horoscope} />
              )
            ) : view === "analytics" ? (
              <>
                <ProfileTabs view={view} />
                {user.role === "psychic" ? <PsychicAnalyticsView /> : null}
              </>
            ) : view === "settings" ? (
              // Advisors get their own page (Figma 1219:1284) hosting the shared account sections.
              // (its header actions are hidden on phones, so the tabs carry the navigation there).
              user.role === "psychic" ? (
                <>
                  <ProfileTabs view={view} />
                  <PsychicSettings SharedProvider={SharedAccountSettings} />
                </>
              ) : (
                <AccountSettings />
              )
            ) : (
              <>
                <ProfileTabs view={view} />
                <ChartTab name={name} user={user} />
              </>
            )}
          </main>
        </div>
      </div>
      <MobileNavigation />
    </div>
  );
}

function ProfileTabs({ view }: { view: AccountView }) {
  const { text, locale } = useI18n();
  const { user } = useSession();
  return (
    <div className="profile-tabs">
      <h1>{text("Your cosmic profile")}</h1>
      <nav>
        {user?.role === "psychic" ? (
          <Link href={`/${locale}/profile/analytics`} className={view === "analytics" ? "active" : ""}>
            {text("Analytics")}
          </Link>
        ) : null}
        <Link href={`/${locale}/profile/settings`} className={view === "settings" ? "active" : ""}>
          {text("Settings ")}
        </Link>
        {user?.role === "psychic" ? null : (
          <Link href={`/${locale}/profile/chart`} className={view === "chart" ? "active" : ""}>
            {text("Astrological chart ")}
          </Link>
        )}
      </nav>
    </div>
  );
}
