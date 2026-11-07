// @vitest-environment jsdom
import { cleanup, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const session = vi.hoisted(() => ({ status: "anonymous" as string }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: session.status }) }));

import { psychicsHref, psychicsPath, usePsychicsHref, usePsychicsPath } from "@/lib/psychics-link";
import { contentDestination } from "@/components/content/content-link";
import { I18nProvider } from "@/i18n/i18n-provider";
import { dictionary } from "@/test/render";
import type * as React from "react";

afterEach(cleanup);

describe("psychics link helper", () => {
  it("sends visitors to the home section and members to the directory", () => {
    expect(psychicsPath(false)).toBe("#psychics");
    expect(psychicsPath(true)).toBe("/psychics");
    expect(psychicsHref("fr", false)).toBe("/fr#psychics");
    expect(psychicsHref("es", true)).toBe("/es/psychics");
  });

  it("follows the session state", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <I18nProvider locale="fr" dictionary={dictionary}>
        {children}
      </I18nProvider>
    );
    for (const [status, href, path] of [
      ["anonymous", "/fr#psychics", "#psychics"],
      ["loading", "/fr#psychics", "#psychics"],
      ["authenticated", "/fr/psychics", "/psychics"],
    ] as const) {
      session.status = status;
      expect(renderHook(() => usePsychicsHref(), { wrapper }).result.current).toBe(href);
      expect(renderHook(() => usePsychicsPath(), { wrapper }).result.current).toBe(path);
    }
  });
});

describe("contentDestination", () => {
  it("defaults to the supplied psychics entry point", () => {
    expect(contentDestination("Explore Advisors")).toBe("/psychics");
    expect(contentDestination("Explore Advisors", "#psychics")).toBe("#psychics");
    expect(contentDestination("Browse Psychics", "#psychics")).toBe("#psychics");
  });
  it("keeps the other destinations", () => {
    expect(contentDestination("Contact Us", "#psychics")).toBe("/contact");
    expect(contentDestination("get your astrological calculation", "#psychics")).toBe("/calculators/astrology");
    expect(contentDestination("Sign Up", "#psychics")).toBe("/signup");
    expect(contentDestination("Start 1-on-1 chat", "#psychics")).toBe("/chatroom");
  });
});
