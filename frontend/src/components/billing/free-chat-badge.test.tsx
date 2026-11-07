// @vitest-environment jsdom
import { cleanup, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { renderWithI18n } from "@/test/render";
import type { SiteConfig } from "@/lib/api/types";

vi.mock("@/lib/api/endpoints", () => ({ siteApi: { config: vi.fn(() => new Promise(() => {})) } }));

import { FreeChatBadge, FreeModeCreditsNotice } from "@/components/billing/free-chat-badge";
import { SiteConfigProvider } from "@/lib/site-config/site-config-provider";

const cfg = (active: boolean, ends_at?: string): SiteConfig => ({
  banners: { rotation_seconds: 6, items: [] },
  signup_offer: { enabled: true, free_minutes: 5 },
  free_mode: { active, ends_at },
  generated_at: "2026-11-07T00:00:00Z",
});
afterEach(cleanup);

describe("FreeChatBadge", () => {
  it("shows while free mode is active", () => {
    renderWithI18n(<SiteConfigProvider initial={cfg(true)}><FreeChatBadge /></SiteConfigProvider>);
    expect(screen.getByTestId("free-chat-badge").textContent).toBe("Free live chat");
  });
  it("is absent when free mode is off, ended, or unknown", () => {
    renderWithI18n(<SiteConfigProvider initial={cfg(false)}><FreeChatBadge /></SiteConfigProvider>);
    expect(screen.queryByTestId("free-chat-badge")).toBeNull();
    cleanup();
    renderWithI18n(<SiteConfigProvider initial={cfg(true, "2020-01-01T00:00:00Z")}><FreeChatBadge /></SiteConfigProvider>);
    expect(screen.queryByTestId("free-chat-badge")).toBeNull();
    cleanup();
    renderWithI18n(<FreeChatBadge />);
    expect(screen.queryByTestId("free-chat-badge")).toBeNull();
  });
});

describe("FreeModeCreditsNotice", () => {
  it("explains that chat is free and credits still pay for custom requests", () => {
    renderWithI18n(<SiteConfigProvider initial={cfg(true)}><FreeModeCreditsNotice /></SiteConfigProvider>);
    const note = screen.getByTestId("credits-free-notice");
    expect(note.textContent).toContain("Live chat is free right now.");
    expect(note.textContent).toContain("custom requests");
  });
  it("is absent outside free mode", () => {
    renderWithI18n(<SiteConfigProvider initial={cfg(false)}><FreeModeCreditsNotice /></SiteConfigProvider>);
    expect(screen.queryByTestId("credits-free-notice")).toBeNull();
  });
});
