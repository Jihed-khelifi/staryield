// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";

import { browserPrivacySignal, thirdPartyEmbedsAllowed } from "@/lib/consent";

function browser(values: { gpc?: boolean; dnt?: string | null }) {
  Object.defineProperty(navigator, "doNotTrack", { value: values.dnt ?? null, configurable: true });
  Object.defineProperty(navigator, "globalPrivacyControl", { value: values.gpc, configurable: true });
}

afterEach(() => {
  vi.unstubAllGlobals();
  browser({});
});

describe("thirdPartyEmbedsAllowed", () => {
  it("allows embeds when the browser sends no privacy signal", () => {
    browser({});
    expect(browserPrivacySignal()).toBe(false);
    expect(thirdPartyEmbedsAllowed()).toBe(true);
  });

  it("refuses them for Global Privacy Control", () => {
    browser({ gpc: true });
    expect(thirdPartyEmbedsAllowed()).toBe(false);
  });

  it("refuses them for Do Not Track", () => {
    browser({ dnt: "1" });
    expect(thirdPartyEmbedsAllowed()).toBe(false);
  });

  it("treats DNT 0 and GPC false as no signal", () => {
    browser({ dnt: "0", gpc: false });
    expect(thirdPartyEmbedsAllowed()).toBe(true);
  });

  it("never allows an embed without a browser (server rendering)", () => {
    vi.stubGlobal("navigator", undefined);
    expect(thirdPartyEmbedsAllowed()).toBe(false);
  });
});
