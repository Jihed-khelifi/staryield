// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, renderHook, screen, waitFor } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { ClientProfile, Participant } from "@/lib/api/types";
import { renderWithI18n } from "@/test/render";

const clientProfile = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api/endpoints", () => ({ chatApi: { clientProfile: (id: string) => clientProfile(id) } }));

import { ChatHeader } from "@/components/chat/chat-header";
import { CLIENT_PROFILE_TTL_MS, clearClientProfileCache, useClientProfile } from "@/components/chat/use-client-profile";

const client: Participant = { id: "c1", role: "client", name: "Paul", status: "online" };
const profile = (over: Partial<ClientProfile> = {}): ClientProfile =>
  ({ id: "c1", name: "Paul", online: true, locale: "en", member_since: "2025-01-01T00:00:00Z", birth_details_shared: false, history: { sessions: 0, minutes: 0 }, ...over }) as ClientProfile;

const noop = vi.fn();
const header = (props: Partial<React.ComponentProps<typeof ChatHeader>> = {}) =>
  renderWithI18n(
    <ChatHeader counterpart={client} conversationId="conv-1" viewerRole="psychic" session={null} elapsedSeconds={0} onOpenInbox={noop} onOpenDetails={noop} onEnd={noop} ending={false} {...props} />,
  );

beforeEach(() => {
  clearClientProfileCache();
  clientProfile.mockReset();
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("advisor chat header: the client's date of birth (§22.6)", () => {
  it("shows the long date under the name, even when the birth city is not shared", async () => {
    clientProfile.mockResolvedValue(profile({ date_of_birth: "1999-06-18", birth_details_shared: false }));
    header();
    const dob = await screen.findByTestId("chat-header-dob");
    expect(dob.textContent).toContain("June 18, 1999");
    expect(dob.querySelector("time")?.getAttribute("datetime")).toBe("1999-06-18");
    // Between the name and the status line.
    const name = screen.getByText("Paul");
    expect(name.nextElementSibling).toBe(dob);
  });

  it("shows a skeleton while loading and nothing on error", async () => {
    let fail: (e: unknown) => void = () => undefined;
    clientProfile.mockReturnValue(new Promise((_, reject) => (fail = reject)));
    header();
    expect(screen.getByTestId("chat-header-dob-loading").getAttribute("aria-hidden")).toBe("true");
    fail(new ApiError({ code: "internal_error", message: "x", status: 500 }));
    await waitFor(() => expect(screen.queryByTestId("chat-header-dob-loading")).toBeNull());
    expect(screen.queryByTestId("chat-header-dob")).toBeNull();
  });

  it("nothing for clients (the header never requests the profile for them)", () => {
    header({ viewerRole: "client", counterpart: { id: "p1", role: "psychic", name: "Theo", slug: "theo" } });
    expect(clientProfile).not.toHaveBeenCalled();
    expect(screen.queryByTestId("chat-header-dob")).toBeNull();
    expect(screen.queryByTestId("chat-header-dob-loading")).toBeNull();
  });

  it("formats for the viewer's locale", async () => {
    clientProfile.mockResolvedValue(profile({ date_of_birth: "1999-06-18" }));
    const fr = await import("@/i18n/dictionaries/fr.json");
    const copy = await import("@/i18n/dictionaries/copy-fr.json");
    const { I18nProvider } = await import("@/i18n/i18n-provider");
    const { render } = await import("@testing-library/react");
    render(
      <I18nProvider locale="fr" dictionary={{ ...fr.default, copy: copy.default } as never}>
        <ChatHeader counterpart={client} conversationId="conv-1" viewerRole="psychic" session={null} elapsedSeconds={0} onOpenInbox={noop} onOpenDetails={noop} onEnd={noop} ending={false} />
      </I18nProvider>,
    );
    expect((await screen.findByTestId("chat-header-dob")).textContent).toContain("18 juin 1999");
  });
});

describe("useClientProfile (shared cached fetch)", () => {
  it("dedupes concurrent callers: the header and the panel share one request", async () => {
    clientProfile.mockResolvedValue(profile({ date_of_birth: "1999-06-18" }));
    const a = renderHook(() => useClientProfile("conv-1"));
    const b = renderHook(() => useClientProfile("conv-1"));
    expect(a.result.current.loading).toBe(true);
    await waitFor(() => expect(a.result.current.profile?.date_of_birth).toBe("1999-06-18"));
    await waitFor(() => expect(b.result.current.profile?.date_of_birth).toBe("1999-06-18"));
    expect(clientProfile).toHaveBeenCalledTimes(1);
  });

  it("serves a later mount from the cache, per conversation, until the cache expires", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    clientProfile.mockResolvedValue(profile());
    const first = renderHook(() => useClientProfile("conv-1"));
    await waitFor(() => expect(first.result.current.profile).not.toBeNull());
    first.unmount();
    const again = renderHook(() => useClientProfile("conv-1"));
    expect(again.result.current.profile).not.toBeNull(); // instantly
    expect(again.result.current.loading).toBe(false);
    expect(clientProfile).toHaveBeenCalledTimes(1);
    again.unmount();
    renderHook(() => useClientProfile("conv-2"));
    await waitFor(() => expect(clientProfile).toHaveBeenCalledTimes(2));
    expect(clientProfile).toHaveBeenLastCalledWith("conv-2");
    vi.setSystemTime(Date.now() + CLIENT_PROFILE_TTL_MS + 1);
    renderHook(() => useClientProfile("conv-1"));
    await waitFor(() => expect(clientProfile).toHaveBeenCalledTimes(3));
  });

  it("reports an error and retries with reload", async () => {
    clientProfile.mockRejectedValueOnce(new ApiError({ code: "network_error", message: "offline" }));
    const { result } = renderHook(() => useClientProfile("conv-1"));
    await waitFor(() => expect(result.current.error?.code).toBe("network_error"));
    expect(result.current.loading).toBe(false);
    clientProfile.mockResolvedValue(profile({ date_of_birth: "2000-01-02" }));
    result.current.reload();
    await waitFor(() => expect(result.current.profile?.date_of_birth).toBe("2000-01-02"));
    expect(result.current.error).toBeNull();
  });

  it("does nothing without a conversation", () => {
    const { result } = renderHook(() => useClientProfile(null));
    expect(result.current).toMatchObject({ profile: null, loading: false, error: null });
    expect(clientProfile).not.toHaveBeenCalled();
  });
});
