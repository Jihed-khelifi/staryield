/**
 * Test helpers for the public site configuration (`GET /site/config`): a
 * complete config fixture and a wrapper that provides it, so components that
 * read the configuration (free mode, memberships, …) are rendered the way the
 * `[lang]` layout renders them.
 */
import * as React from "react";

import type { SiteConfig } from "@/lib/api/types";
import { SiteConfigProvider } from "@/lib/site-config/site-config-provider";

export function siteConfigFixture(over: Partial<SiteConfig> = {}): SiteConfig {
  return {
    banners: { rotation_seconds: 6, items: [] },
    signup_offer: { enabled: false, free_minutes: 0 },
    free_mode: { active: false },
    generated_at: "2026-11-12T00:00:00Z",
    ...over,
  };
}

/**
 * `memberships`: true / false as the API sends it, or `undefined` for an older
 * API that has no `features` at all (§25.1: absent means off).
 */
export function WithSiteConfig({ memberships, children }: { memberships?: boolean; children: React.ReactNode }) {
  const config = siteConfigFixture(memberships === undefined ? {} : { features: { memberships } });
  return <SiteConfigProvider initial={config}>{children}</SiteConfigProvider>;
}
