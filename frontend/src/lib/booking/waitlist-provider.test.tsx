// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { Page, WaitlistEntry } from "@/lib/api/types";
import { renderWithI18n } from "@/test/render";

const join = vi.fn<(slug: string) => Promise<WaitlistEntry>>();
const leave = vi.fn<(slug: string) => Promise<void>>();
const mine = vi.fn<() => Promise<Page<WaitlistEntry>>>();
let session: { status: string; user: { id: string; role: string } | null } = { status: "authenticated", user: { id: "u1", role: "client" } };
const handlers = new Map<string, (d: unknown) => void>();
/** Every subscriber of the event type hears it, like the real realtime client. */
const emit = (type: string, d: unknown) => handlers.forEach((h, key) => key.startsWith(`${type}|`) && h(d));
const waitlistNotification = (slug: string) => ({
  notification: { id: `n-${slug}`, kind: "waitlist.available", data: { psychic_name: slug, psychic_slug: slug }, link: `/chatroom?reader=${slug}`, created_at: "2026-11-05T12:00:00Z" },
});

vi.mock("@/lib/api/endpoints", () => ({
  waitlistApi: { join: (s: string) => join(s), leave: (s: string) => leave(s), mine: () => mine() },
}));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ ...session, realtime: null }),
  useRealtimeEvent: (type: string, handler: (d: unknown) => void) => {
    // One slot per event type and subscriber (the call site), refreshed on every render.
    handlers.set(`${type}|${new Error().stack?.split("\n")[2] ?? "x"}`, handler);
  },
}));

const { AVAILABLE_HOLD_MS, WaitlistProvider, WaitlistScope, useWaitlist } = await import("@/lib/booking/waitlist-provider");

const entry = (slug: string): WaitlistEntry => ({ slug, name: slug, image_url: "", status: "busy", joined_at: "2026-11-05T10:00:00Z" });

/** A probe that needs the list for the given advisors and prints what the provider says. */
function Probe({ slugs, needs = true }: { slugs: string[]; needs?: boolean }) {
  const w = useWaitlist();
  const { demand } = w;
  // Like BookButton: demand the list only when a button needs it.
  useDemand(needs, demand);
  return (
    <ul>
      {slugs.map((slug) => (
        <li key={slug} data-testid={slug}>
          {w.joined(slug)}|{w.pending(slug) ?? "idle"}|{w.availableFrom(slug) ?? "none"}
          <button onClick={() => void w.join(slug)}>join-{slug}</button>
          <button onClick={() => w.markAvailable(slug, "busy")}>available-{slug}</button>
          <button onClick={() => void w.leave(slug)}>leave-{slug}</button>
        </li>
      ))}
    </ul>
  );
}

function useDemand(want: boolean, demand: () => void) {
  React.useEffect(() => {
    if (want) demand();
  }, [want, demand]);
}

beforeEach(() => {
  for (const f of [join, leave, mine]) f.mockReset();
  mine.mockResolvedValue({ items: [entry("ramone")] });
  session = { status: "authenticated", user: { id: "u1", role: "client" } };
  handlers.clear();
});
afterEach(cleanup);

describe("WaitlistProvider", () => {
  it("reads the waiting lists once for every button on the page", async () => {
    renderWithI18n(
      <WaitlistProvider>
        <Probe slugs={["ramone", "theo"]} />
        <Probe slugs={["ramone"]} />
        <Probe slugs={["solomon"]} />
      </WaitlistProvider>,
    );
    expect(screen.getAllByTestId("ramone")[0].textContent).toContain("unknown");
    await waitFor(() => expect(screen.getAllByTestId("ramone")[0].textContent).toContain("joined|idle"));
    expect(screen.getAllByTestId("ramone").every((e) => e.textContent?.startsWith("joined|idle"))).toBe(true);
    expect(screen.getByTestId("theo").textContent).toMatch(/^not_joined/);
    expect(mine).toHaveBeenCalledOnce();
  });

  it("does not read them when no button needs them (everyone online)", () => {
    renderWithI18n(
      <WaitlistProvider>
        <Probe slugs={["ramone"]} needs={false} />
      </WaitlistProvider>,
    );
    expect(mine).not.toHaveBeenCalled();
    expect(screen.getByTestId("ramone").textContent).toMatch(/^unknown/);
  });

  it("reads them only once, even when the need goes away and comes back", async () => {
    const view = (needs: boolean) => (
      <WaitlistProvider>
        <Probe slugs={["ramone"]} needs={needs} />
      </WaitlistProvider>
    );
    const { rerender } = renderWithI18n(view(true));
    await waitFor(() => expect(screen.getByTestId("ramone").textContent).toMatch(/^joined/));
    rerender(view(false)); // the advisor came online
    rerender(view(true)); // and left again
    expect(screen.getByTestId("ramone").textContent).toMatch(/^joined/);
    expect(mine).toHaveBeenCalledOnce();
  });

  it("reads nothing for visitors and advisor accounts", () => {
    for (const s of [{ status: "anonymous", user: null }, { status: "authenticated", user: { id: "p1", role: "psychic" } }]) {
      session = s;
      const { unmount } = renderWithI18n(
        <WaitlistProvider>
          <Probe slugs={["ramone"]} />
        </WaitlistProvider>,
      );
      expect(mine).not.toHaveBeenCalled();
      expect(screen.getByTestId("ramone").textContent).toMatch(/^not_joined/);
      unmount();
    }
  });

  it("shares joined and pending state between buttons of the same advisor", async () => {
    let done: (e: WaitlistEntry) => void = () => {};
    join.mockReturnValue(new Promise((resolve) => (done = resolve)));
    renderWithI18n(
      <WaitlistProvider>
        <Probe slugs={["theo"]} />
        <Probe slugs={["theo"]} />
      </WaitlistProvider>,
    );
    await waitFor(() => expect(screen.getAllByTestId("theo")[0].textContent).toMatch(/^not_joined/));
    fireEvent.click(screen.getAllByText("join-theo")[0]);
    await waitFor(() => expect(screen.getAllByTestId("theo").every((e) => e.textContent?.includes("|join"))).toBe(true));
    await act(async () => done(entry("theo")));
    await waitFor(() => expect(screen.getAllByTestId("theo").every((e) => e.textContent?.startsWith("joined|idle"))).toBe(true));
    expect(mine).toHaveBeenCalledOnce();
  });

  it("returns the failure of a request and leaves the state alone", async () => {
    const failure = new ApiError({ code: "limit_reached", message: "limit", status: 409 });
    join.mockRejectedValue(failure);
    let result: unknown;
    function Caller() {
      const w = useWaitlist();
      return <button onClick={() => void w.join("theo").then((r) => (result = r))}>go</button>;
    }
    renderWithI18n(
      <WaitlistProvider>
        <Caller />
      </WaitlistProvider>,
    );
    fireEvent.click(screen.getByText("go"));
    await waitFor(() => expect(result).toEqual({ ok: false, error: failure }));
  });

  it("treats a failed read as not joined, so the button stays usable", async () => {
    mine.mockRejectedValue(new ApiError({ code: "network_error", message: "offline", status: 0 }));
    renderWithI18n(
      <WaitlistProvider>
        <Probe slugs={["ramone"]} />
      </WaitlistProvider>,
    );
    await waitFor(() => expect(screen.getByTestId("ramone").textContent).toMatch(/^not_joined/));
  });

  it("forgets an entry when the client is told the advisor is online (the server closes it then)", async () => {
    renderWithI18n(
      <WaitlistProvider>
        <Probe slugs={["ramone", "theo"]} />
      </WaitlistProvider>,
    );
    await waitFor(() => expect(screen.getByTestId("ramone").textContent).toMatch(/^joined/));
    // Coming online alone does not close the entry (the advisor may leave before the notice).
    act(() => emit("psychic.status_changed", { slug: "ramone", status: "online" }));
    expect(screen.getByTestId("ramone").textContent).toMatch(/^joined/);
    // Other notifications change nothing; the waiting-list one closes that advisor's entry only.
    act(() => emit("notification.created", { notification: { id: "n1", kind: "chat.missed", data: { psychic_slug: "ramone" }, link: "/", created_at: "" } }));
    expect(screen.getByTestId("ramone").textContent).toMatch(/^joined/);
    act(() => emit("notification.created", waitlistNotification("ramone")));
    expect(screen.getByTestId("ramone").textContent).toMatch(/^not_joined/);
    expect(screen.getByTestId("theo").textContent).toMatch(/^not_joined/);
    expect(mine).toHaveBeenCalledOnce();
  });

  it("starts over for another account", async () => {
    join.mockResolvedValue(entry("theo"));
    const { rerender } = renderWithI18n(
      <WaitlistProvider>
        <Probe slugs={["theo"]} />
      </WaitlistProvider>,
    );
    await waitFor(() => expect(screen.getByTestId("theo").textContent).toMatch(/^not_joined/));
    fireEvent.click(screen.getByText("join-theo"));
    await waitFor(() => expect(screen.getByTestId("theo").textContent).toMatch(/^joined/));
    session = { status: "authenticated", user: { id: "u2", role: "client" } };
    mine.mockResolvedValue({ items: [] });
    rerender(
      <WaitlistProvider>
        <Probe slugs={["theo"]} />
      </WaitlistProvider>,
    );
    await waitFor(() => expect(mine).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.getByTestId("theo").textContent).toMatch(/^not_joined/));
  });

  it("keeps an 'online now' (409) until that advisor's next live status change", async () => {
    renderWithI18n(
      <WaitlistProvider>
        <Probe slugs={["ramone", "theo"]} />
      </WaitlistProvider>,
    );
    fireEvent.click(screen.getByText("available-ramone"));
    fireEvent.click(screen.getByText("available-theo"));
    expect(screen.getByTestId("ramone").textContent).toMatch(/^\w+\|\w+\|busy/);
    // Another advisor's change leaves it alone; Ramone's own ends it, whatever the new status.
    act(() => emit("psychic.status_changed", { slug: "theo", status: "away" }));
    expect(screen.getByTestId("ramone").textContent).toMatch(/^\w+\|\w+\|busy/);
    expect(screen.getByTestId("theo").textContent).toMatch(/^\w+\|\w+\|none/);
    act(() => emit("psychic.status_changed", { slug: "ramone", status: "busy" }));
    expect(screen.getByTestId("ramone").textContent).toMatch(/^\w+\|\w+\|none/);
  });

  it("drops an 'online now' after the hold, and for another account", async () => {
    vi.useFakeTimers();
    try {
      const view = () => (
        <WaitlistProvider>
          <Probe slugs={["ramone"]} needs={false} />
        </WaitlistProvider>
      );
      const { rerender } = renderWithI18n(view());
      fireEvent.click(screen.getByText("available-ramone"));
      act(() => vi.advanceTimersByTime(AVAILABLE_HOLD_MS - 1));
      expect(screen.getByTestId("ramone").textContent).toMatch(/^\w+\|\w+\|busy/);
      act(() => vi.advanceTimersByTime(1));
      expect(screen.getByTestId("ramone").textContent).toMatch(/^\w+\|\w+\|none/);

      fireEvent.click(screen.getByText("available-ramone"));
      session = { status: "authenticated", user: { id: "u2", role: "client" } };
      rerender(view());
      expect(screen.getByTestId("ramone").textContent).toMatch(/^\w+\|\w+\|none/);
    } finally {
      vi.useRealTimers();
    }
  });

  it("requires a provider, and WaitlistScope reuses the parent's", async () => {
    const quiet = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => renderWithI18n(<Probe slugs={["a"]} />)).toThrow(/WaitlistProvider/);
    quiet.mockRestore();
    renderWithI18n(
      <WaitlistProvider>
        <Probe slugs={["ramone"]} />
        <WaitlistScope>
          <Probe slugs={["ramone"]} />
        </WaitlistScope>
      </WaitlistProvider>,
    );
    await waitFor(() => expect(screen.getAllByTestId("ramone")[1].textContent).toMatch(/^joined/));
    expect(mine).toHaveBeenCalledOnce();
  });
});
