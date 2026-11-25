// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import { onAgeBlocked } from "@/lib/api/age-events";
import type { Page, PresenceStatus, WaitlistEntry } from "@/lib/api/types";
import { axeViolations, renderWithI18n } from "@/test/render";

const join = vi.fn<(slug: string) => Promise<WaitlistEntry>>();
const leave = vi.fn<(slug: string) => Promise<void>>();
const mine = vi.fn<() => Promise<Page<WaitlistEntry>>>();
const toastShow = vi.fn();
const toastError = vi.fn();
const track = vi.fn();
// Realtime subscribers, one slot per event type and call site (like the real client, every subscriber hears an event).
const handlers = new Map<string, (d: unknown) => void>();
const emit = (type: string, data: unknown) => handlers.forEach((h, key) => key.startsWith(`${type}|`) && h(data));
type Session = {
  status: "loading" | "anonymous" | "authenticated";
  user: { id: string; role: string; age_status?: string } | null;
  wallet: { balance: number; held: number; free_minutes: number } | null;
};
let session: Session = { status: "anonymous", user: null, wallet: null };
let site = { freeMode: false, offer: 0 };

vi.mock("@/lib/api/endpoints", () => ({
  waitlistApi: { join: (s: string) => join(s), leave: (s: string) => leave(s), mine: () => mine() },
}));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ ...session, realtime: null }),
  useRealtimeEvent: (type: string, handler: (d: unknown) => void) => {
    handlers.set(`${type}|${new Error().stack?.split("\n")[2] ?? "x"}`, handler);
  },
}));
vi.mock("@/lib/site-config/site-config-provider", () => ({
  useFreeMode: () => site.freeMode,
  useSignupFreeMinutes: () => site.offer,
}));
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => ({ show: toastShow, showError: toastError }) }));
vi.mock("@/lib/analytics", () => ({ track: (...args: unknown[]) => track(...args) }));

const { BookButton } = await import("@/components/booking/book-button");
const { BookingBar } = await import("@/components/booking/booking-bar");
const { AVAILABLE_HOLD_MS, WaitlistProvider } = await import("@/lib/booking/waitlist-provider");

type Advisor = { slug: string; name: string; status: PresenceStatus; rate_per_minute: number };
const theo = (status: PresenceStatus): Advisor => ({ slug: "theo", name: "Theo", status, rate_per_minute: 30 });
const entry = (slug: string): WaitlistEntry => ({ slug, name: slug, image_url: "", status: "busy", joined_at: "2026-11-05T10:00:00Z" });
const client = (over: Partial<Session> = {}): Session => ({
  status: "authenticated",
  user: { id: "u1", role: "client", age_status: "adult" },
  wallet: { balance: 0, held: 0, free_minutes: 0 },
  ...over,
});

function renderButton(advisor: Advisor, props: Partial<React.ComponentProps<typeof BookButton>> = {}) {
  return renderWithI18n(
    <WaitlistProvider>
      <BookButton advisor={advisor} placement="profile_hero" {...props} />
    </WaitlistProvider>,
  );
}

beforeEach(() => {
  for (const f of [join, leave, mine, toastShow, toastError, track]) f.mockReset();
  handlers.clear();
  mine.mockResolvedValue({ items: [] });
  session = { status: "anonymous", user: null, wallet: null };
  site = { freeMode: false, offer: 0 };
  window.history.replaceState(null, "", "/en/psychics/theo");
  sessionStorage.clear();
});
afterEach(cleanup);

describe("BookButton for visitors", () => {
  it("sends a visitor to sign up with the chat as the next step, naming the new-member offer", async () => {
    site.offer = 10;
    renderButton(theo("online"));
    const link = screen.getByRole("link", { name: /Chat now/ });
    expect(link.getAttribute("href")).toBe("/en/signup?next=%2Fen%2Fchatroom%3Freader%3Dtheo");
    expect(link.textContent).toContain("10 free minutes for new members");
    expect(link.className).toContain("paper-overlay"); // the home call-to-action look
    fireEvent.click(link);
    expect(track).toHaveBeenCalledWith("book_cta_clicked", { slug: "theo", status: "online", placement: "profile_hero", action: "signup" });
    expect(mine).not.toHaveBeenCalled();
    expect(await axeViolations()).toEqual([]);
  });

  it("brings an offline advisor's visitor back to the profile after sign-up, remembering the wish to wait", () => {
    renderButton(theo("offline"));
    const link = screen.getByRole("link", { name: /Notify me when online/ });
    expect(link.getAttribute("href")).toBe("/en/signup?next=%2Fen%2Fpsychics%2Ftheo");
    expect(link.getAttribute("data-variant")).toBe("glass");
    expect(mine).not.toHaveBeenCalled();
    fireEvent.click(link);
    expect(JSON.parse(sessionStorage.getItem("staryield.booking.intent") ?? "null")).toMatchObject({ slug: "theo", action: "join_waitlist" });
  });

  it("remembers nothing when the visitor goes to sign up for the chat", () => {
    renderButton(theo("online"));
    fireEvent.click(screen.getByRole("link", { name: /Chat now/ }));
    expect(sessionStorage.getItem("staryield.booking.intent")).toBeNull();
  });

  it("says a busy advisor is in a reading", () => {
    renderButton(theo("busy"));
    expect(screen.getByRole("link", { name: /Join the waiting list.*In a reading/ })).toBeTruthy();
  });

  it("shows free mode instead of the offer", () => {
    site = { freeMode: true, offer: 10 };
    renderButton(theo("online"));
    expect(screen.getByRole("link").textContent).toContain("Free for a limited time");
  });

  it("holds a disabled placeholder while the session loads, promising no offer yet", () => {
    session = { status: "loading", user: null, wallet: null };
    site.offer = 10;
    renderButton(theo("online"));
    const button = screen.getByRole("button", { name: /Chat now/ }) as HTMLButtonElement;
    expect(button.textContent).toContain("30 credits/min");
    expect(button.textContent).not.toContain("free minutes");
    expect(button.disabled).toBe(true);
    expect(button.getAttribute("aria-busy")).toBe("true");
    expect(screen.queryByRole("link")).toBeNull();
  });
});

describe("BookButton for clients", () => {
  it("links to the chatroom and offers the client's own free minutes", () => {
    session = client({ wallet: { balance: 0, held: 0, free_minutes: 5 } });
    renderButton(theo("online"), { placement: "home_featured", size: "full" });
    const link = screen.getByRole("link", { name: /Chat now/ });
    expect(link.getAttribute("href")).toBe("/en/chatroom?reader=theo");
    expect(link.textContent).toContain("Use your 5 free minutes");
    expect(link.className).toContain("w-full");
    fireEvent.click(link);
    expect(track).toHaveBeenCalledWith("book_cta_clicked", { slug: "theo", status: "online", placement: "home_featured", action: "chat" });
    expect(mine).not.toHaveBeenCalled(); // nothing to wait for
  });

  it("shows the rate when there is nothing free", () => {
    session = client();
    renderButton(theo("online"));
    expect(screen.getByRole("link").textContent).toContain("30 credits/min");
  });

  it("joins the waiting list, announces it, and shows the joined state", async () => {
    session = client();
    join.mockResolvedValue(entry("theo"));
    renderButton(theo("busy"));
    const button = await screen.findByRole("button", { name: /Join the waiting list/ });
    await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(button);
    const joined = await screen.findByRole("button", { name: /On the waiting list.*select to leave/ });
    expect(join).toHaveBeenCalledWith("theo");
    expect(joined.getAttribute("data-action")).toBe("leave_waitlist");
    expect(toastShow).toHaveBeenCalledWith({ tone: "success", message: "You're on Theo's waiting list. We'll let you know when they're online." });
    expect(track).toHaveBeenCalledWith("book_cta_clicked", { slug: "theo", status: "busy", placement: "profile_hero", action: "join_waitlist" });
    await waitFor(() => expect(document.querySelector('[data-slot="live-region"]')?.textContent).toContain("You're on Theo's waiting list"));
    expect(await axeViolations()).toEqual([]);
  });

  it("leaves the waiting list", async () => {
    session = client();
    mine.mockResolvedValue({ items: [entry("theo")] });
    leave.mockResolvedValue();
    renderButton(theo("offline"));
    fireEvent.click(await screen.findByRole("button", { name: /On the waiting list/ }));
    await screen.findByRole("button", { name: /Notify me when online/ });
    expect(leave).toHaveBeenCalledWith("theo");
    expect(toastShow).toHaveBeenCalledWith({ tone: "info", message: "You left Theo's waiting list." });
    expect(track).toHaveBeenCalledWith("book_cta_clicked", { slug: "theo", status: "offline", placement: "profile_hero", action: "leave_waitlist" });
  });

  it("is disabled and busy while the request runs", async () => {
    session = client();
    let done: (e: WaitlistEntry) => void = () => {};
    join.mockReturnValue(new Promise((resolve) => (done = resolve)));
    renderButton(theo("busy"));
    const button = (await screen.findByRole("button", { name: /Join the waiting list/ })) as HTMLButtonElement;
    await waitFor(() => expect(button.disabled).toBe(false));
    fireEvent.click(button);
    await waitFor(() => expect(button.disabled).toBe(true));
    expect(button.getAttribute("aria-busy")).toBe("true");
    expect(button.textContent).toContain("Joining…");
    fireEvent.click(button);
    expect(join).toHaveBeenCalledOnce();
    await act(async () => done(entry("theo")));
    await screen.findByRole("button", { name: /On the waiting list/ });
  });

  it("switches to the chat when the advisor turns out to be online (409)", async () => {
    session = client();
    join.mockRejectedValue(new ApiError({ code: "advisor_available", message: "online", status: 409 }));
    const onAvailable = vi.fn();
    renderButton(theo("busy"), { onAvailable });
    const button = await screen.findByRole("button", { name: /Join the waiting list/ });
    await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(button);
    const link = await screen.findByRole("link", { name: /Chat now/ });
    expect(link.getAttribute("href")).toBe("/en/chatroom?reader=theo");
    expect(onAvailable).toHaveBeenCalledOnce();
    expect(toastShow).toHaveBeenCalledWith({ tone: "info", message: "Theo is online now. Start your chat." });
  });

  it("reports an error a retry can fix with a retry that tries again", async () => {
    session = client();
    join.mockRejectedValueOnce(new ApiError({ code: "network_error", message: "offline", status: 0 })).mockResolvedValueOnce(entry("theo"));
    renderButton(theo("away"));
    const button = await screen.findByRole("button", { name: /Notify me when online/ });
    await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(button);
    await waitFor(() => expect(toastShow).toHaveBeenCalledOnce());
    const toast = toastShow.mock.calls[0][0];
    expect(toast).toMatchObject({ tone: "error", actionLabel: "Try again" });
    expect(screen.getByRole("button", { name: /Notify me when online/ })).toBeTruthy();
    await act(async () => toast.onAction());
    await screen.findByRole("button", { name: /On the waiting list/ });
    expect(join).toHaveBeenCalledTimes(2);
  });

  it.each([
    ["limit_reached", 409],
    ["forbidden", 403],
    ["not_found", 404],
  ] as const)("says why without a pointless retry for %s", async (code, status) => {
    session = client();
    const error = new ApiError({ code, message: code, status });
    join.mockRejectedValue(error);
    leave.mockRejectedValue(error);
    mine.mockResolvedValue({ items: [entry("solomon")] });
    renderWithI18n(
      <WaitlistProvider>
        <BookButton advisor={theo("away")} placement="directory" />
        <BookButton advisor={{ slug: "solomon", name: "Solomon", status: "offline", rate_per_minute: 30 }} placement="directory" />
      </WaitlistProvider>,
    );
    const button = await screen.findByRole("button", { name: /Notify me when online/ });
    await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(button);
    await waitFor(() => expect(toastError).toHaveBeenCalledWith(error));
    fireEvent.click(screen.getByRole("button", { name: /On the waiting list/ }));
    await waitFor(() => expect(toastError).toHaveBeenCalledTimes(2));
    expect(toastShow).not.toHaveBeenCalled();
    expect(join).toHaveBeenCalledOnce();
  });

  it("hands a client who has not finished the age step to the age flow", async () => {
    session = client({ user: { id: "u1", role: "client", age_status: "birth_date_required" } });
    const blocked = vi.fn();
    const off = onAgeBlocked(blocked);
    renderButton(theo("busy"));
    const button = await screen.findByRole("button", { name: /Join the waiting list/ });
    await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(button);
    expect(blocked).toHaveBeenCalledWith("birth_date_required");
    expect(join).not.toHaveBeenCalled();
    off();
  });

  it("follows the advisor's status, and a stale 'online now' ends with the real change", async () => {
    session = client();
    join.mockRejectedValue(new ApiError({ code: "advisor_available", message: "online", status: 409 }));
    const view = (status: PresenceStatus) => (
      <WaitlistProvider>
        <BookButton advisor={theo(status)} placement="profile_hero" />
      </WaitlistProvider>
    );
    const { rerender } = renderWithI18n(view("busy"));
    const button = await screen.findByRole("button", { name: /Join the waiting list/ });
    await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(button);
    await screen.findByRole("link", { name: /Chat now/ });
    rerender(view("busy")); // still shown as busy by the page: the chat stays
    expect(screen.getByRole("link", { name: /Chat now/ })).toBeTruthy();
    rerender(view("online"));
    rerender(view("away")); // went online and left again: the real status wins
    expect(screen.getByRole("button", { name: /Notify me when online/ })).toBeTruthy();
  });

  it("ends the 'online now' with the advisor's next live status change, even back to the status of the click", async () => {
    session = client();
    join.mockRejectedValue(new ApiError({ code: "advisor_available", message: "online", status: 409 }));
    // The profile: its status follows the 409 (onAvailable) and the live changes.
    const view = (status: PresenceStatus) => (
      <WaitlistProvider>
        <BookButton advisor={theo(status)} placement="profile_hero" />
        <BookButton advisor={theo(status)} placement="profile_bar" />
      </WaitlistProvider>
    );
    const { rerender } = renderWithI18n(view("busy"));
    const [button] = await screen.findAllByRole("button", { name: /Join the waiting list/ });
    await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(button);
    await waitFor(() => expect(screen.getAllByRole("link", { name: /Chat now/ })).toHaveLength(2));
    rerender(view("online"));
    // Theo takes another client: the live change brings "busy" back, and both buttons offer the list again.
    act(() => emit("psychic.status_changed", { slug: "theo", status: "busy" }));
    rerender(view("busy"));
    expect(await screen.findAllByRole("button", { name: /Join the waiting list/ })).toHaveLength(2);
    expect(screen.queryByRole("link", { name: /Chat now/ })).toBeNull();
  });

  it("holds the 'online now' over a stale reload, then trusts the page again", async () => {
    session = client();
    join.mockRejectedValue(new ApiError({ code: "advisor_available", message: "online", status: 409 }));
    renderButton(theo("busy"));
    const button = await screen.findByRole("button", { name: /Join the waiting list/ });
    await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false));
    vi.useFakeTimers();
    try {
      fireEvent.click(button);
      await act(async () => {});
      expect(screen.getByRole("link", { name: /Chat now/ })).toBeTruthy();
      // A list reloaded within the hold still says busy (the API's snapshot lags): the chat stays.
      act(() => vi.advanceTimersByTime(AVAILABLE_HOLD_MS - 1));
      expect(screen.getByRole("link", { name: /Chat now/ })).toBeTruthy();
      act(() => vi.advanceTimersByTime(1));
      expect(screen.getByRole("button", { name: /Join the waiting list/ })).toBeTruthy();
    } finally {
      vi.useRealTimers();
    }
  });

  it("takes a client in a reading with the advisor back to the chat, without reading the waiting lists", () => {
    session = client();
    renderButton(theo("busy"), { inReading: true });
    const link = screen.getByRole("link", { name: /Back to chat/ });
    expect(link.getAttribute("href")).toBe("/en/chatroom?reader=theo");
    expect(link.getAttribute("data-variant")).toBe("cta");
    expect(mine).not.toHaveBeenCalled();
    fireEvent.click(link);
    expect(track).toHaveBeenCalledWith("book_cta_clicked", { slug: "theo", status: "busy", placement: "profile_hero", action: "chat" });
  });
});

describe("BookButton after sign-up", () => {
  const remember = () => sessionStorage.setItem("staryield.booking.intent", JSON.stringify({ slug: "theo", action: "join_waitlist", at: Date.now() }));

  it("finishes the visitor's join once, even with two buttons for the advisor", async () => {
    remember();
    session = client();
    join.mockResolvedValue(entry("theo"));
    renderWithI18n(
      <WaitlistProvider>
        <BookButton advisor={theo("offline")} placement="profile_hero" />
        <BookingBar advisor={theo("offline")} />
      </WaitlistProvider>,
    );
    await waitFor(() => expect(screen.getAllByRole("button", { name: /On the waiting list/ })).toHaveLength(2));
    expect(join).toHaveBeenCalledOnce();
    expect(toastShow).toHaveBeenCalledWith({ tone: "success", message: "You're on Theo's waiting list. We'll let you know when they're online." });
    expect(sessionStorage.getItem("staryield.booking.intent")).toBeNull();
  });

  it("says the advisor is online now instead of joining", async () => {
    remember();
    session = client();
    renderButton(theo("online"));
    await waitFor(() => expect(toastShow).toHaveBeenCalledWith({ tone: "info", message: "Theo is online now. Start your chat." }));
    expect(join).not.toHaveBeenCalled();
    expect(sessionStorage.getItem("staryield.booking.intent")).toBeNull();
  });

  it("drops it when the client is already on the list, or must finish the age step", async () => {
    remember();
    session = client();
    mine.mockResolvedValue({ items: [entry("theo")] });
    renderButton(theo("busy"));
    await screen.findByRole("button", { name: /On the waiting list/ });
    // Taken by an effect of the settled button, which may run just after the DOM shows it.
    await waitFor(() => expect(sessionStorage.getItem("staryield.booking.intent")).toBeNull());
    cleanup();
    remember();
    session = client({ user: { id: "u1", role: "client", age_status: "birth_date_required" } });
    mine.mockResolvedValue({ items: [] });
    renderButton(theo("busy"));
    await waitFor(() => expect(sessionStorage.getItem("staryield.booking.intent")).toBeNull());
    expect(join).not.toHaveBeenCalled();
    expect(toastShow).not.toHaveBeenCalled();
  });

  it("leaves another advisor's wish, and waits while the session loads", () => {
    remember();
    session = { status: "loading", user: null, wallet: null };
    const { rerender } = renderButton(theo("busy"));
    expect(sessionStorage.getItem("staryield.booking.intent")).not.toBeNull();
    session = client();
    rerender(
      <WaitlistProvider>
        <BookButton advisor={{ ...theo("busy"), slug: "solomon", name: "Solomon" }} placement="profile_hero" />
      </WaitlistProvider>,
    );
    expect(sessionStorage.getItem("staryield.booking.intent")).not.toBeNull();
    expect(join).not.toHaveBeenCalled();
  });
});

describe("BookButton for advisor accounts", () => {
  it("shows an explanation and no action", () => {
    session = client({ user: { id: "p1", role: "psychic" } });
    renderButton(theo("online"));
    expect(screen.getByRole("note").textContent).toBe("Advisor accounts can't book readings");
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByRole("link")).toBeNull();
    expect(mine).not.toHaveBeenCalled();
  });
});

describe("BookButton on repeated cards", () => {
  it("is described by the advisor's name", () => {
    renderWithI18n(
      <WaitlistProvider>
        <h3 id="name-theo">Theo</h3>
        <BookButton advisor={theo("online")} placement="directory" describedBy="name-theo" />
      </WaitlistProvider>,
    );
    expect(screen.getByRole("link", { name: /Chat now/ }).getAttribute("aria-describedby")).toBe("name-theo");
  });
});

describe("BookingBar", () => {
  it("shows the advisor, the status and the standard button, on phones only", async () => {
    site.offer = 10;
    renderWithI18n(
      <WaitlistProvider>
        <BookingBar advisor={theo("online")} />
      </WaitlistProvider>,
    );
    const bar = screen.getByTestId("booking-bar");
    expect(bar.className).toContain("booking-bar"); // phones only, fixed above the tab bar (globals.css, checked below)
    expect(bar.textContent).toContain("Theo");
    expect(bar.textContent).toContain("Online");
    // Online with an offer on the button: the bar still says what a reading costs.
    expect(within(bar).getByTestId("advisor-price").textContent).toBe("30 credits/min");
    expect(within(bar).getByRole("link").textContent).toContain("10 free minutes for new members");
    const link = screen.getByRole("link", { name: /Chat now/ });
    expect(link.getAttribute("aria-describedby")).toBeTruthy();
    fireEvent.click(link);
    expect(track).toHaveBeenCalledWith("book_cta_clicked", { slug: "theo", status: "online", placement: "profile_bar", action: "signup" });
    expect(await axeViolations()).toEqual([]);
  });

  it("follows status changes of the page through its props", () => {
    session = client();
    const view = (status: PresenceStatus) => (
      <WaitlistProvider>
        <BookingBar advisor={theo(status)} />
      </WaitlistProvider>
    );
    const { rerender } = renderWithI18n(view("online"));
    expect(screen.getByRole("link", { name: /Chat now/ })).toBeTruthy();
    rerender(view("busy"));
    expect(screen.getByTestId("booking-bar").textContent).toContain("Busy");
    expect(screen.queryByRole("link", { name: /Chat now/ })).toBeNull();
    // Not online: the bar says what a reading costs.
    expect(screen.getByTestId("advisor-price").textContent).toBe("30 credits/min");
  });

  it("does not repeat the button's line when it is the rate or free mode", () => {
    session = client();
    const { unmount } = renderWithI18n(
      <WaitlistProvider>
        <BookingBar advisor={theo("online")} />
      </WaitlistProvider>,
    );
    expect(screen.getByRole("link").textContent).toContain("30 credits/min");
    expect(screen.queryByTestId("advisor-price")).toBeNull();
    unmount();
    site.freeMode = true;
    renderWithI18n(
      <WaitlistProvider>
        <BookingBar advisor={theo("online")} />
      </WaitlistProvider>,
    );
    expect(screen.getByRole("link").textContent).toContain("Free for a limited time");
    expect(screen.queryByTestId("advisor-price")).toBeNull();
  });

  it("says chat is free in free mode", () => {
    site.freeMode = true;
    renderWithI18n(
      <WaitlistProvider>
        <BookingBar advisor={theo("away")} />
      </WaitlistProvider>,
    );
    expect(screen.getByTestId("advisor-price").textContent).toBe("Free for a limited time");
  });

  it("is styled for phones only, above the tab bar and inside the safe area (globals.css)", () => {
    const css = readFileSync(`${process.cwd()}/src/app/globals.css`, "utf8");
    // Hidden by default (an unlayered rule: a Tailwind `md:hidden` would lose to it)...
    expect(css).toMatch(/\.booking-bar \{\s*display: none;\s*\}/);
    // ...shown, fixed and padded for the notch and home indicator below the tablet width only.
    const phone = css.slice(css.lastIndexOf("@media (max-width: 767px) {"));
    expect(phone).toMatch(/\.booking-bar \{[^}]*position: fixed;[^}]*env\(safe-area-inset-left\)[^}]*env\(safe-area-inset-right\)[^}]*bottom: calc\(76px \+ env\(safe-area-inset-bottom\)\)[^}]*display: flex;/);
    // The page reserves room under the bar.
    expect(phone).toMatch(/\.psychic-profile\.has-booking-bar \{\s*padding-bottom:/);
  });

  it("is not shown to advisor accounts", () => {
    session = client({ user: { id: "p1", role: "psychic" } });
    renderWithI18n(
      <WaitlistProvider>
        <BookingBar advisor={theo("online")} />
      </WaitlistProvider>,
    );
    expect(screen.queryByTestId("booking-bar")).toBeNull();
  });
});
