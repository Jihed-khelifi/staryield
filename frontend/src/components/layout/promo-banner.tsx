"use client";

/**
 * The promo bar above the header (revisions 3.0, §15): admin-controlled,
 * several banners that rotate, each with its own text and action.
 *
 * - Banners are filtered by audience against the session state (the auth
 *   hint decides on the first paint, before the session answers).
 * - All banners share one grid cell, so the bar never changes height while
 *   rotating or when the configuration arrives from the server render.
 * - WCAG 2.2.2: rotation pauses on hover and focus and has its own
 *   pause/play button. `prefers-reduced-motion` removes the fade only.
 */
import * as React from "react";
import Link from "next/link";
import { Pause, Play } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { readAuthHint } from "@/lib/api/client";
import type { SiteBanner } from "@/lib/api/types";
import { useSession } from "@/lib/auth/session-provider";
import { useSiteConfig } from "@/lib/site-config/site-config-provider";
import { cn } from "@/lib/utils";

/** Banners visible to this audience (`signedIn` false = anonymous). */
export function filterBanners(items: readonly SiteBanner[], signedIn: boolean): SiteBanner[] {
  return items.filter((b) => b.audience === "all" || b.audience === (signedIn ? "signed_in" : "anonymous"));
}

export type BannerLink = { href: string; external: boolean };

/**
 * The link of a banner: an internal path gets the locale prefix, an https
 * URL is external, anything else (none, empty, other schemes) is text only.
 */
export function bannerLink(banner: SiteBanner, locale: string): BannerLink | null {
  if (banner.action.type !== "link") return null;
  const href = banner.action.href.trim();
  if (/^https:\/\/[^\s/]+/i.test(href)) return { href, external: true };
  if (!href.startsWith("/") || href.startsWith("//") || href.includes("\\")) return null;
  const rest = href === "/" ? "" : href.startsWith("/?") || href.startsWith("/#") ? href.slice(1) : href;
  return { href: `/${locale}${rest}`, external: false };
}

const subscribeNone = () => () => {};

/** Auth hint for the first paint: false on the server, the stored hint in the browser (no hydration warning). */
function useAuthHint(): boolean {
  return React.useSyncExternalStore(subscribeNone, readAuthHint, () => false);
}

const SLIDE = "flex w-full flex-wrap items-center justify-center gap-x-6 gap-y-0.5 text-center sm:gap-x-[78px]";

function Slide({ banner, active, locale }: { banner: SiteBanner; active: boolean; locale: string }) {
  const { text } = useI18n();
  const link = bannerLink(banner, locale);
  const body = (
    <>
      <span className="[overflow-wrap:anywhere]">{banner.text}</span>
      {banner.cta_label ? <span className="hidden sm:inline">{banner.cta_label}</span> : null}
    </>
  );
  return (
    <div
      className={cn(
        "col-start-1 row-start-1 flex w-full justify-center transition-opacity duration-500 motion-reduce:transition-none",
        active ? "opacity-100" : "invisible opacity-0",
      )}
      aria-hidden={active ? undefined : true}
      data-testid="promo-slide"
      data-active={active}
    >
      {link ? (
        link.external ? (
          <a href={link.href} target="_blank" rel="noopener noreferrer" className={cn(SLIDE, "hover:underline")}>
            {body}
            <span className="sr-only">{text("(opens in a new tab)")}</span>
          </a>
        ) : (
          <Link href={link.href} className={cn(SLIDE, "hover:underline")}>
            {body}
          </Link>
        )
      ) : (
        <p className={SLIDE}>{body}</p>
      )}
    </div>
  );
}

export function PromoBanner() {
  const { text, locale } = useI18n();
  const { status } = useSession();
  const { config } = useSiteConfig();
  const hint = useAuthHint();
  const signedIn = status === "loading" ? hint : status === "authenticated";
  const items = React.useMemo(() => filterBanners(config?.banners.items ?? [], signedIn), [config, signedIn]);
  const seconds = Math.min(30, Math.max(4, config?.banners.rotation_seconds ?? 6));

  const ids = items.map((b) => b.id).join("|");
  // A new set of banners (admin change, audience change) starts from the first.
  const [pos, setPos] = React.useState({ ids, index: 0 });
  const index = pos.ids === ids ? pos.index : 0;
  const [userPaused, setUserPaused] = React.useState(false);
  const [hover, setHover] = React.useState(false);
  const [focus, setFocus] = React.useState(false);
  const count = items.length;
  const current = count ? index % count : 0;
  const rotating = count > 1 && !userPaused && !hover && !focus;

  // One timeout per slide: hovering or focusing restarts the full interval.
  React.useEffect(() => {
    if (!rotating) return;
    const timer = window.setTimeout(() => setPos({ ids, index: (current + 1) % count }), seconds * 1000);
    return () => window.clearTimeout(timer);
  }, [rotating, current, count, seconds, ids]);

  if (count === 0) return null;
  return (
    <div
      className="promo-bar paper-page relative"
      role="region"
      aria-roledescription="carousel"
      aria-label={text("Promotions")}
      data-testid="promo-bar"
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
    >
      {/* Keyboard focus on a banner pauses it; focus on the play/pause button
          does not, so an explicit "Play" resumes the rotation at once. */}
      <div
        className={cn("grid min-w-0 flex-1 justify-items-center", count > 1 && "px-8")}
        aria-live={rotating ? "off" : "polite"}
        onFocus={() => setFocus(true)}
        onBlur={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setFocus(false);
        }}
      >
        {items.map((banner, i) => (
          <Slide key={banner.id} banner={banner} active={i === current} locale={locale} />
        ))}
      </div>
      {count > 1 ? (
        <button
          type="button"
          onClick={() => setUserPaused((p) => !p)}
          aria-pressed={userPaused}
          aria-label={text(userPaused ? "Play promotions" : "Pause promotions")}
          data-testid="promo-toggle"
          className="absolute top-1/2 right-2 flex size-7 -translate-y-1/2 items-center justify-center rounded-full border border-ink/30 hover:bg-gold/20 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ink"
        >
          {userPaused ? <Play size={12} aria-hidden /> : <Pause size={12} aria-hidden />}
        </button>
      ) : null}
    </div>
  );
}
