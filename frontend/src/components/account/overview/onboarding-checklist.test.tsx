// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import type * as React from "react";

import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { ApiError } from "@/lib/api/errors";
import type { Onboarding } from "@/lib/api/onboarding";
import { OnboardingChecklist, ONBOARDING_REFRESH_EVENTS } from "@/components/account/overview/onboarding-checklist";

// growth: the client's "Get started" checklist on the Account Overview.
const api = vi.hoisted(() => ({ get: vi.fn(), dismiss: vi.fn(), restore: vi.fn() }));
const session = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));
const realtime = vi.hoisted(() => ({ handler: null as null | ((type: string, data: unknown) => void), types: [] as readonly string[] }));
const track = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api/onboarding", async (importOriginal) => ({ ...(await importOriginal<typeof import("@/lib/api/onboarding")>()), onboardingApi: api }));
vi.mock("@/lib/analytics", () => ({ track }));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => session.current,
  useRealtimeEvents: (types: readonly string[], handler: (type: string, data: unknown) => void) => {
    realtime.types = types;
    realtime.handler = handler;
  },
}));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

const client = { id: "u1", role: "client", profile: { birth_date: "1990-04-02", birth_place_id: "" } };

function checklist(overrides: Partial<Onboarding> = {}): Onboarding {
  const steps: Onboarding["steps"] = overrides.steps ?? [
    { key: "verify_email", done: true },
    { key: "birth_details", done: false },
    { key: "first_reading", done: false },
    { key: "invite_friend", done: false },
  ];
  return { steps, total: steps.length, completed: steps.filter((s) => s.done).length, dismissed: false, ...overrides };
}

const render = () => renderWithI18n(<ToastProvider>{<OnboardingChecklist />}</ToastProvider>);

/** The card once it reached a state (it starts as a loading skeleton). */
async function card(state: "ready" | "error") {
  await waitFor(() => expect(screen.getByTestId("onboarding-checklist").getAttribute("data-state")).toBe(state));
  return screen.getByTestId("onboarding-checklist");
}

beforeAll(stubResizeObserver);
beforeEach(() => {
  session.current = { status: "authenticated", user: client };
  realtime.handler = null;
  api.get.mockResolvedValue(checklist());
  api.dismiss.mockResolvedValue(checklist({ dismissed: true }));
  api.restore.mockResolvedValue(checklist());
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("OnboardingChecklist", () => {
  it("shows the progress and each open step with the place where it is done", async () => {
    render();
    const ready = await card("ready");
    expect(within(ready).getByRole("heading", { name: "Get started" })).toBeTruthy();
    expect(screen.getByTestId("onboarding-progress-text").textContent).toBe("1 of 4 done");
    const bar = screen.getByRole("progressbar");
    expect([bar.getAttribute("aria-valuenow"), bar.getAttribute("aria-valuemax"), bar.getAttribute("aria-valuetext")]).toEqual(["1", "4", "1 of 4 done"]);

    const steps = screen.getAllByTestId("onboarding-step");
    expect(steps.map((s) => s.getAttribute("data-step"))).toEqual(["verify_email", "birth_details", "first_reading", "invite_friend"]);
    // A done step is checked off: no call to action, its done label instead.
    expect(steps[0].getAttribute("data-done")).toBe("true");
    expect(within(steps[0]).queryByRole("link")).toBeNull();
    expect(within(steps[0]).getByText("Email confirmed")).toBeTruthy();
    expect(within(steps[0]).getByText("Completed")).toBeTruthy();

    expect(screen.getByTestId("onboarding-cta-birth_details").getAttribute("href")).toBe("/en/profile/settings#account");
    // The next open step is the one that stands out.
    expect(screen.getByTestId("onboarding-cta-birth_details").getAttribute("data-next")).toBe("true");
    expect(screen.getByTestId("onboarding-cta-first_reading").getAttribute("data-next")).toBeNull();
    expect(screen.getByRole("link", { name: "Choose an advisor" }).getAttribute("href")).toBe("/en/psychics");
    expect(screen.getByRole("link", { name: "Get your invite link" }).getAttribute("href")).toBe("/en/profile/invite");
    expect(await axeViolations()).toEqual([]);
  });

  it("links the e-mail step to the account settings and records which step was clicked", async () => {
    api.get.mockResolvedValue(checklist({ steps: [{ key: "verify_email", done: false }, { key: "first_reading", done: false }] }));
    render();
    const cta = await screen.findByTestId("onboarding-cta-verify_email");
    expect(cta.getAttribute("href")).toBe("/en/profile/settings#account");
    fireEvent.click(screen.getByTestId("onboarding-cta-first_reading"));
    expect(track).toHaveBeenCalledWith("onboarding_step_clicked", { step: "first_reading" });
  });

  it("shows a skeleton while loading, then the card", async () => {
    let resolve: (value: Onboarding) => void = () => undefined;
    api.get.mockReturnValue(new Promise<Onboarding>((r) => (resolve = r)));
    render();
    const loading = screen.getByTestId("onboarding-checklist");
    expect(loading.getAttribute("data-state")).toBe("loading");
    expect(loading.getAttribute("aria-busy")).toBe("true");
    expect(within(loading).getByRole("status").textContent).toBe("Loading your checklist…");
    await act(async () => resolve(checklist()));
    expect(screen.getByTestId("onboarding-checklist").getAttribute("data-state")).toBe("ready");
  });

  it("explains a failed load and retries", async () => {
    api.get.mockRejectedValueOnce(new ApiError({ status: 503, code: "service_unavailable", message: "Service unavailable." }));
    render();
    await card("error");
    expect(screen.getByText("Your checklist could not be loaded")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: /try again/i }));
    await card("ready");
    expect(api.get).toHaveBeenCalledTimes(2);
  });

  it("hides with an undo that brings it back", async () => {
    render();
    fireEvent.click(await screen.findByRole("button", { name: "Hide the checklist" }));
    await waitFor(() => expect(screen.queryByTestId("onboarding-checklist")).toBeNull());
    expect(api.dismiss).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith("onboarding_dismissed");
    expect(screen.getByText("Checklist hidden.")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Undo" }));
    await card("ready");
    expect(api.restore).toHaveBeenCalledTimes(1);
    expect(await screen.findByText("Your checklist is back.")).toBeTruthy();
  });

  it("stays visible and says why when hiding fails", async () => {
    api.dismiss.mockRejectedValue(new ApiError({ status: 0, code: "network_error", message: "network" }));
    render();
    const hide = await screen.findByTestId("onboarding-dismiss");
    fireEvent.click(hide);
    await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
    expect(screen.getByTestId("onboarding-checklist").getAttribute("data-state")).toBe("ready");
    expect((hide as HTMLButtonElement).disabled).toBe(false);
    expect(track).not.toHaveBeenCalledWith("onboarding_dismissed");
  });

  it.each([
    ["an advisor", () => (session.current = { status: "authenticated", user: { ...client, role: "psychic" } }), false],
    ["an admin", () => (session.current = { status: "authenticated", user: { ...client, role: "admin" } }), false],
    ["a hidden checklist", () => api.get.mockResolvedValue(checklist({ dismissed: true })), true],
    ["a finished checklist", () => api.get.mockResolvedValue(checklist({ steps: [{ key: "birth_details", done: true }, { key: "first_reading", done: true }] })), true],
  ])("renders nothing for %s", async (_label, setup, fetches) => {
    setup();
    const { container } = render();
    await waitFor(() => expect(api.get).toHaveBeenCalledTimes(fetches ? 1 : 0));
    await act(async () => undefined);
    expect(container.querySelector("[data-testid=onboarding-checklist]")).toBeNull();
  });

  it("follows other tabs and refetches when an account fact changes", async () => {
    render();
    await card("ready");
    expect(realtime.types).toEqual(["onboarding.updated", ...ONBOARDING_REFRESH_EVENTS]);
    // Hidden in another tab: the event carries the checklist, no request needed.
    act(() => realtime.handler?.("onboarding.updated", checklist({ dismissed: true })));
    expect(screen.queryByTestId("onboarding-checklist")).toBeNull();
    act(() => realtime.handler?.("onboarding.updated", checklist()));
    expect(screen.getByTestId("onboarding-checklist")).toBeTruthy();
    expect(api.get).toHaveBeenCalledTimes(1);

    // A reading ended (several events at once): one refetch, with the new progress.
    vi.useFakeTimers();
    api.get.mockResolvedValue(checklist({ steps: [{ key: "verify_email", done: true }, { key: "birth_details", done: false }, { key: "first_reading", done: true }] }));
    act(() => {
      realtime.handler?.("session.ended", {});
      realtime.handler?.("wallet.updated", {});
      realtime.handler?.("notification.created", {});
    });
    await act(async () => {
      vi.advanceTimersByTime(450);
    });
    vi.useRealTimers();
    await waitFor(() => expect(screen.getByTestId("onboarding-progress-text").textContent).toBe("2 of 3 done"));
    expect(api.get).toHaveBeenCalledTimes(2);
  });
});
