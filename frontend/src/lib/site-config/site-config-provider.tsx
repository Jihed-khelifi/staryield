"use client";

/**
 * Public runtime settings (docs/platform-extensions.md §15.1): promo
 * banners, the sign-up offer and free mode, controlled from the admin
 * console.
 *
 * Loaded once per page load and per locale, revalidated when the tab becomes
 * visible again (at most once a minute) and every five minutes. A failed
 * refresh keeps the last good copy; before the first answer `config` is
 * null, so consumers render their neutral state (no banner, no free-mode
 * copy). The server stays the authority for billing: this is display only.
 */
import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { siteApi } from "@/lib/api/endpoints";
import { toApiError } from "@/lib/api/errors";
import type { SiteConfig } from "@/lib/api/types";
import { logger } from "@/lib/logger";

export const SITE_CONFIG_REFRESH_MS = 5 * 60_000;
export const SITE_CONFIG_STALE_MS = 60_000;

export type SiteConfigStatus = "loading" | "ready" | "error";

type SiteConfigValue = {
  config: SiteConfig | null;
  status: SiteConfigStatus;
  refresh: () => void;
};

const SiteConfigContext = React.createContext<SiteConfigValue>({ config: null, status: "loading", refresh: () => {} });

export function SiteConfigProvider({ children, initial = null }: { children: React.ReactNode; initial?: SiteConfig | null }) {
  const { locale } = useI18n();
  const [state, setState] = React.useState<{ config: SiteConfig | null; status: SiteConfigStatus; locale: string | null }>({
    config: initial,
    status: initial ? "ready" : "loading",
    locale: initial ? locale : null,
  });
  const loadedAt = React.useRef(0);
  const inFlight = React.useRef<AbortController | null>(null);

  const load = React.useCallback(() => {
    inFlight.current?.abort();
    const ctrl = new AbortController();
    inFlight.current = ctrl;
    siteApi.config(ctrl.signal).then(
      (config) => {
        if (ctrl.signal.aborted) return;
        loadedAt.current = Date.now();
        setState({ config, status: "ready", locale });
      },
      (err) => {
        const e = toApiError(err);
        if (ctrl.signal.aborted || e.code === "aborted") return;
        logger.debug("site config failed", { code: String(e.code) });
        // Keep the last good copy; only report an error when there is none.
        setState((cur) => (cur.config ? cur : { ...cur, status: "error" }));
      },
    );
  }, [locale]);

  React.useEffect(() => {
    if (initial) loadedAt.current = Date.now();
    // Only the first render's initial copy counts as "just loaded".
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // First load, and again when the locale changes (banner copy is localized).
  React.useEffect(() => {
    if (initial && !loadedAt.current) loadedAt.current = Date.now();
    if (state.locale !== locale || !state.config) load();
    return () => inFlight.current?.abort();
    // Only the locale should trigger this effect.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [locale]);

  React.useEffect(() => {
    const onVisible = () => {
      if (document.visibilityState === "visible" && Date.now() - loadedAt.current > SITE_CONFIG_STALE_MS) load();
    };
    document.addEventListener("visibilitychange", onVisible);
    const timer = window.setInterval(load, SITE_CONFIG_REFRESH_MS);
    return () => {
      document.removeEventListener("visibilitychange", onVisible);
      window.clearInterval(timer);
    };
  }, [load]);

  const value = React.useMemo(() => ({ config: state.config, status: state.status, refresh: load }), [state.config, state.status, load]);
  return <SiteConfigContext.Provider value={value}>{children}</SiteConfigContext.Provider>;
}

export function useSiteConfig(): SiteConfigValue {
  return React.useContext(SiteConfigContext);
}

/** True while free mode applies (and its end time, when set, is still ahead). */
export function freeModeActive(config: SiteConfig | null, now: number = Date.now()): boolean {
  if (!config?.free_mode.active) return false;
  const end = config.free_mode.ends_at ? Date.parse(config.free_mode.ends_at) : NaN;
  return Number.isNaN(end) || end > now;
}

/** Free mode for display (the chat paywall is off for everyone). */
export function useFreeMode(): boolean {
  const { config } = useSiteConfig();
  return freeModeActive(config);
}

/** Free minutes a new account receives now (0 when the offer is off or unknown). */
export function useSignupFreeMinutes(): number {
  const { config } = useSiteConfig();
  return config?.signup_offer.enabled ? config.signup_offer.free_minutes : 0;
}

/**
 * Memberships are built but switched off (docs/platform-extensions.md §25.1):
 * every membership surface (plans, member discounts, "Premium", member-only
 * perks) renders only when the API says so. Unknown or absent means false.
 */
export function membershipsEnabled(config: SiteConfig | null): boolean {
  return config?.features?.memberships === true;
}

export function useMembershipsEnabled(): boolean {
  const { config } = useSiteConfig();
  return membershipsEnabled(config);
}
