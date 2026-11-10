// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api/errors";

const api = vi.hoisted(() => ({ googleStart: vi.fn() }));
const attribution = vi.hoisted(() => ({ value: undefined as Record<string, string> | undefined }));
vi.mock("@/lib/api/endpoints", () => ({ authApi: { googleStart: api.googleStart } }));
vi.mock("@/lib/attribution", () => ({ attributionForSignup: () => attribution.value }));

import {
  authorizationTarget,
  browser,
  forgetGoogleIntent,
  localizedNext,
  nextForApi,
  recallGoogleIntent,
  rememberGoogleIntent,
  startGoogle,
} from "@/lib/auth/google";

beforeEach(() => {
  sessionStorage.clear();
  attribution.value = undefined;
});
afterEach(() => vi.restoreAllMocks());

describe("authorizationTarget", () => {
  it("accepts https URLs", () => {
    expect(authorizationTarget("https://accounts.google.com/o/oauth2/v2/auth?state=x", false)).toBe("https://accounts.google.com/o/oauth2/v2/auth?state=x");
  });

  it("accepts a loopback http URL only outside production", () => {
    expect(authorizationTarget("http://127.0.0.1:19090/authorize?state=x", true)).toBe("http://127.0.0.1:19090/authorize?state=x");
    expect(authorizationTarget("http://localhost:19090/authorize", true)).toBe("http://localhost:19090/authorize");
    expect(authorizationTarget("http://127.0.0.1:19090/authorize", false)).toBeNull();
  });

  it.each([
    ["http://accounts.google.com/auth", true],
    ["http://127.0.0.1.evil.example/auth", true],
    ["javascript:alert(1)", true],
    ["data:text/html,hi", true],
    ["//evil.example/auth", true],
    ["/relative", true],
    ["https://user:pass@accounts.google.com/auth", true],
    ["", true],
    [undefined, true],
    [42, true],
  ])("refuses %s", (value, development) => {
    expect(authorizationTarget(value, development)).toBeNull();
  });
});

describe("next handling", () => {
  it("sends the locale-less path of a safe ?next", () => {
    expect(nextForApi("?next=/en/chatroom", "en")).toBe("/chatroom");
    expect(nextForApi("?next=%2Fen%2Fpsychics%2Ftheo%3Ftab%3Dabout", "en")).toBe("/psychics/theo?tab=about");
    expect(nextForApi("?next=/en", "en")).toBe("/");
  });

  it("drops unsafe or missing values", () => {
    expect(nextForApi("", "en")).toBeUndefined();
    expect(nextForApi("?next=//evil.example", "en")).toBeUndefined();
    expect(nextForApi("?next=https://evil.example", "en")).toBeUndefined();
    expect(nextForApi("?next=/fr/chatroom", "en")).toBeUndefined();
  });

  it("turns the API's next into a locale-prefixed router path", () => {
    expect(localizedNext("/chatroom", "en")).toBe("/en/chatroom");
    expect(localizedNext("/en/chatroom?x=1", "en")).toBe("/en/chatroom?x=1");
    expect(localizedNext("/", "fr")).toBe("/fr");
    expect(localizedNext(null, "en")).toBeNull();
    expect(localizedNext("//evil.example", "en")).toBeNull();
    expect(localizedNext("https://evil.example", "en")).toBeNull();
    expect(localizedNext("/\\evil.example", "en")).toBeNull();
  });
});

describe("remembered intent", () => {
  it("round-trips and expires", () => {
    rememberGoogleIntent("link", "/profile/settings");
    expect(recallGoogleIntent()).toEqual({ intent: "link", next: "/profile/settings" });
    expect(recallGoogleIntent(Date.now() + 16 * 60_000)).toBeNull();
    forgetGoogleIntent();
    expect(recallGoogleIntent()).toBeNull();
  });

  it("ignores garbage", () => {
    sessionStorage.setItem("staryield.google.intent", "{not json");
    expect(recallGoogleIntent()).toBeNull();
    sessionStorage.setItem("staryield.google.intent", JSON.stringify({ intent: "admin", at: Date.now() }));
    expect(recallGoogleIntent()).toBeNull();
  });
});

describe("startGoogle", () => {
  it("starts the intent, remembers it and navigates to Google", async () => {
    const assign = vi.spyOn(browser, "assign").mockImplementation(() => {});
    api.googleStart.mockResolvedValue({ authorization_url: "https://accounts.google.com/o/oauth2/v2/auth?state=s" });
    await startGoogle({ intent: "signup", next: "/profile", locale: "fr" });
    expect(api.googleStart).toHaveBeenCalledWith({ intent: "signup", accept_terms: true, next: "/profile", locale: "fr", attribution: undefined });
    expect(assign).toHaveBeenCalledWith("https://accounts.google.com/o/oauth2/v2/auth?state=s");
    expect(recallGoogleIntent()).toEqual({ intent: "signup", next: "/profile" });
  });

  it("never sends terms for sign-in or attribution for linking", async () => {
    vi.spyOn(browser, "assign").mockImplementation(() => {});
    attribution.value = { utm_source: "news" };
    api.googleStart.mockResolvedValue({ authorization_url: "https://accounts.google.com/x" });
    await startGoogle({ intent: "signin", locale: "en" });
    expect(api.googleStart.mock.calls[0][0]).toMatchObject({ intent: "signin", accept_terms: undefined, attribution: { utm_source: "news" } });
    await startGoogle({ intent: "link", next: "/profile/settings", locale: "en" });
    expect(api.googleStart.mock.calls[1][0]).toMatchObject({ intent: "link", attribution: undefined });
  });

  it("retries without attribution when the API rejects it", async () => {
    vi.spyOn(browser, "assign").mockImplementation(() => {});
    attribution.value = { utm_source: "news" };
    api.googleStart
      .mockRejectedValueOnce(new ApiError({ code: "validation_failed", message: "x", status: 422, details: { fields: { "attribution.utm_source": "bad" } } }))
      .mockResolvedValueOnce({ authorization_url: "https://accounts.google.com/x" });
    await startGoogle({ intent: "signin", locale: "en" });
    expect(api.googleStart).toHaveBeenCalledTimes(2);
    expect(api.googleStart.mock.calls[1][0].attribution).toBeUndefined();
  });

  it("refuses an unsafe authorization URL without navigating", async () => {
    const assign = vi.spyOn(browser, "assign").mockImplementation(() => {});
    api.googleStart.mockResolvedValue({ authorization_url: "http://evil.example/phish" });
    await expect(startGoogle({ intent: "signin", locale: "en" })).rejects.toMatchObject({ code: "google_unavailable" });
    expect(assign).not.toHaveBeenCalled();
    expect(recallGoogleIntent()).toBeNull();
  });

  it("passes API errors through", async () => {
    api.googleStart.mockRejectedValue(new ApiError({ code: "google_unavailable", message: "x", status: 503 }));
    await expect(startGoogle({ intent: "signin", locale: "en" })).rejects.toMatchObject({ code: "google_unavailable" });
  });
});
