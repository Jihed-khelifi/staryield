// @vitest-environment jsdom
import * as React from "react";
import { act, cleanup, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithI18n } from "@/test/render";
import type { SiteConfig } from "@/lib/api/types";

const config = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api/endpoints", () => ({ siteApi: { config } }));

import {
  freeModeActive,
  membershipsEnabled,
  useMembershipsEnabled,
  SiteConfigProvider,
  SITE_CONFIG_REFRESH_MS,
  useFreeMode,
  useSignupFreeMinutes,
  useSiteConfig,
} from "@/lib/site-config/site-config-provider";

function site(overrides: Partial<SiteConfig> = {}): SiteConfig {
  return {
    banners: { rotation_seconds: 6, items: [] },
    signup_offer: { enabled: true, free_minutes: 5 },
    free_mode: { active: false },
    generated_at: "2026-11-07T00:00:00Z",
    ...overrides,
  };
}

function Probe() {
  const { status, config } = useSiteConfig();
  const free = useFreeMode();
  const minutes = useSignupFreeMinutes();
  return (
    <p data-testid="probe">
      {status}|{config ? "loaded" : "none"}|{String(free)}|{minutes}
    </p>
  );
}

describe("SiteConfigProvider", () => {
  beforeEach(() => config.mockReset());
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("loads the configuration and exposes the helpers", async () => {
    config.mockResolvedValue(site({ free_mode: { active: true } }));
    renderWithI18n(
      <SiteConfigProvider>
        <Probe />
      </SiteConfigProvider>,
    );
    expect(screen.getByTestId("probe").textContent).toBe("loading|none|false|0");
    await waitFor(() => expect(screen.getByTestId("probe").textContent).toBe("ready|loaded|true|5"));
  });

  it("reports an error only when nothing was ever loaded, and keeps the last good copy", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    config.mockRejectedValueOnce(Object.assign(new Error("down"), { code: "network_error" }));
    renderWithI18n(
      <SiteConfigProvider>
        <Probe />
      </SiteConfigProvider>,
    );
    await waitFor(() => expect(screen.getByTestId("probe").textContent).toBe("error|none|false|0"));
    config.mockResolvedValueOnce(site({ signup_offer: { enabled: false, free_minutes: 5 } }));
    await act(async () => {
      vi.advanceTimersByTime(SITE_CONFIG_REFRESH_MS);
    });
    await waitFor(() => expect(screen.getByTestId("probe").textContent).toBe("ready|loaded|false|0"));
    config.mockRejectedValueOnce(Object.assign(new Error("down"), { code: "network_error" }));
    await act(async () => {
      vi.advanceTimersByTime(SITE_CONFIG_REFRESH_MS);
    });
    expect(screen.getByTestId("probe").textContent).toBe("ready|loaded|false|0");
  });

  it("treats a free-mode period whose end has passed as inactive", () => {
    const now = Date.parse("2026-11-07T12:00:00Z");
    expect(freeModeActive(site({ free_mode: { active: true, ends_at: "2026-11-07T11:00:00Z" } }), now)).toBe(false);
    expect(freeModeActive(site({ free_mode: { active: true, ends_at: "2026-11-07T13:00:00Z" } }), now)).toBe(true);
    expect(freeModeActive(site({ free_mode: { active: true } }), now)).toBe(true);
    expect(freeModeActive(null, now)).toBe(false);
  });
});

describe("memberships switch (§25.1)", () => {
  afterEach(cleanup);

  it("is off unless the API says true: absent, false and unknown config all mean off", () => {
    expect(membershipsEnabled(null)).toBe(false);
    expect(membershipsEnabled(site())).toBe(false);
    expect(membershipsEnabled(site({ features: { memberships: false } }))).toBe(false);
    expect(membershipsEnabled(site({ features: { memberships: "yes" as unknown as boolean } }))).toBe(false);
    expect(membershipsEnabled(site({ features: { memberships: true } }))).toBe(true);
  });

  function Flag() {
    return <p data-testid="flag">{String(useMembershipsEnabled())}</p>;
  }

  it("the hook is false outside a provider and before the first answer, and follows the config", () => {
    renderWithI18n(<Flag />);
    expect(screen.getByTestId("flag").textContent).toBe("false");
    cleanup();
    renderWithI18n(
      <SiteConfigProvider initial={site({ features: { memberships: true } })}>
        <Flag />
      </SiteConfigProvider>,
    );
    expect(screen.getByTestId("flag").textContent).toBe("true");
    cleanup();
    renderWithI18n(
      <SiteConfigProvider initial={site()}>
        <Flag />
      </SiteConfigProvider>,
    );
    expect(screen.getByTestId("flag").textContent).toBe("false");
  });
});

