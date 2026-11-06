// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";

import type { ChatMessage, ClientProfile, Participant } from "@/lib/api/types";
import type { PagedList } from "@/lib/use-paged-list";
import { ApiError } from "@/lib/api/errors";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const mocks = vi.hoisted(() => ({
  chatApi: { clientProfile: vi.fn() },
  notesApi: { list: vi.fn(), create: vi.fn(), update: vi.fn(), remove: vi.fn(), shared: vi.fn() },
}));
vi.mock("@/lib/api/endpoints", () => mocks);
vi.mock("@/lib/auth/session-provider", () => ({ useRealtimeEvent: () => undefined }));

import { ClientPanel } from "@/components/chat/client-panel";

beforeAll(stubResizeObserver);
afterEach(cleanup);
beforeEach(() => {
  mocks.chatApi.clientProfile.mockReset();
  mocks.notesApi.list.mockReset().mockResolvedValue({ items: [] });
});

const counterpart = { id: "c1", name: "Paul", status: "online" } as Participant;
const profile = (over: Partial<ClientProfile> = {}): ClientProfile => ({
  id: "c1",
  name: "Paul",
  online: true,
  locale: "en",
  country_code: "US",
  age: 27,
  member_since: "2025-01-01T00:00:00Z",
  astro: { sun_sign: "Aries", rising_sign: "Gemini", moon_sign: "Pisces", time_known: true },
  birth_details_shared: false,
  history: { sessions: 3, minutes: 40 },
  ...over,
});
const saved = (n: number): ChatMessage =>
  ({ id: `m${n}`, kind: "text", body: `Saved ${n}`, created_at: new Date(Date.now() - n * 60_000).toISOString() }) as ChatMessage;

/** One page of saved messages as `usePagedList` exposes it. */
function pageOf(items: ChatMessage[], over: Partial<PagedList<ChatMessage>> = {}): PagedList<ChatMessage> {
  return {
    items,
    page: 1,
    hasPrev: false,
    hasNext: false,
    loading: false,
    error: null,
    next: vi.fn(),
    prev: vi.fn(),
    reset: vi.fn(),
    reload: vi.fn(),
    replaceItem: vi.fn(),
    removeItem: vi.fn(),
    prependItem: vi.fn(),
    ...over,
  };
}

function renderPanel(variant: "column" | "tabs" = "column", list: PagedList<ChatMessage> = pageOf([saved(2), saved(5), saved(10)])) {
  return renderWithI18n(
    <ToastProvider>
      <ClientPanel conversationId="conv-1" counterpart={counterpart} saved={list} onRemove={vi.fn()} variant={variant} />
    </ToastProvider>,
  );
}

describe("ClientPanel", () => {
  it("shows who the client is, falling back to country and age without consent", async () => {
    mocks.chatApi.clientProfile.mockResolvedValue(profile());
    renderPanel();
    expect(await screen.findByRole("heading", { name: "Paul" })).toBeTruthy();
    await waitFor(() => expect(screen.getByTestId("client-location").textContent).toContain("United States"));
    expect(screen.getByText("27 years old")).toBeTruthy();
    expect(screen.getByTestId("client-presence").textContent).toBe("Online");
    expect(screen.getByTestId("client-sessions").textContent).toBe("3 sessions together");
    expect(await axeViolations()).toEqual([]);
  });

  it("shows the shared location and birth date when the client consents", async () => {
    mocks.chatApi.clientProfile.mockResolvedValue(
      profile({
        birth_details_shared: true,
        birth_details: { place: "US, Tennessee, Memphis", date_of_birth: "1999-06-18" },
        history: { sessions: 0, minutes: 0 },
      }),
    );
    renderPanel();
    await waitFor(() => expect(screen.getByTestId("client-location").textContent).toBe("US, Tennessee, Memphis"));
    expect(screen.getByTestId("client-birth-date").textContent).toContain("Jun 18, 1999");
    expect(screen.getByTestId("client-sessions").textContent).toBe("First session together");
  });

  it("shows three placements, then all of them on expand", async () => {
    mocks.chatApi.clientProfile.mockResolvedValue(
      profile({
        astro: {
          sun_sign: "Aries",
          rising_sign: "Gemini",
          time_known: true,
          placements: ["Sun:Aries", "Mercury:Pisces", "Jupiter:Aries", "Mars:Taurus"].map((p) => {
            const [body, sign] = p.split(":");
            return { body, sign, retrograde: false };
          }),
        },
      }),
    );
    renderPanel();
    const section = await screen.findByTestId("client-placements");
    await waitFor(() => expect(within(section).getAllByRole("listitem")).toHaveLength(3));
    expect(within(section).getByText("Sun in")).toBeTruthy();
    const toggle = within(section).getByRole("button", { name: "+ expand" });
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    fireEvent.click(toggle);
    expect(within(section).getAllByRole("listitem")).toHaveLength(5);
    expect(within(section).getByRole("button", { name: "- collapse" }).getAttribute("aria-expanded")).toBe("true");
  });

  it("shows two saved messages, then the rest on expand", async () => {
    mocks.chatApi.clientProfile.mockResolvedValue(profile());
    renderPanel();
    const section = screen.getByTestId("client-saved");
    expect(within(section).getAllByTestId("saved-message")).toHaveLength(2);
    expect(within(section).getByText("Sent 2 min. ago")).toBeTruthy();
    fireEvent.click(within(section).getByRole("button", { name: "+ expand" }));
    expect(within(section).getAllByTestId("saved-message")).toHaveLength(3);
    // A single page: no pager.
    expect(within(section).queryByRole("navigation")).toBeNull();
  });

  it("pages saved messages past five, with the pager once expanded", async () => {
    mocks.chatApi.clientProfile.mockResolvedValue(profile());
    const next = vi.fn();
    renderPanel("column", pageOf([1, 2, 3, 4, 5].map(saved), { hasNext: true, next }));
    const section = screen.getByTestId("client-saved");
    expect(within(section).queryByRole("navigation")).toBeNull();
    fireEvent.click(within(section).getByRole("button", { name: "+ expand" }));
    expect(within(section).getAllByTestId("saved-message")).toHaveLength(5);
    const pager = within(section).getByRole("navigation", { name: "Saved messages pages" });
    expect(within(pager).getByText("Page 1")).toBeTruthy();
    expect((within(pager).getByRole("button", { name: "Previous" }) as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(within(pager).getByRole("button", { name: "Next" }));
    expect(next).toHaveBeenCalled();
    expect(await axeViolations()).toEqual([]);
  });

  it("shows loading, error and empty states for saved messages", async () => {
    mocks.chatApi.clientProfile.mockResolvedValue(profile());
    const { unmount } = renderPanel("column", pageOf([], { loading: true }));
    expect(within(screen.getByTestId("client-saved")).getByTestId("saved-loading")).toBeTruthy();
    unmount();
    const reload = vi.fn();
    const failed = renderPanel("column", pageOf([], { error: new ApiError({ code: "network_error", message: "offline" }), reload }));
    fireEvent.click(within(screen.getByTestId("client-saved")).getByRole("button", { name: "Try again" }));
    expect(reload).toHaveBeenCalled();
    failed.unmount();
    renderPanel("column", pageOf([]));
    expect(within(screen.getByTestId("client-saved")).getByText(/No saved messages yet/)).toBeTruthy();
  });

  it("uses tabs in the details sheet", async () => {
    mocks.chatApi.clientProfile.mockResolvedValue(profile());
    renderPanel("tabs");
    const tabs = screen.getAllByRole("tab").map((t) => t.textContent);
    expect(tabs).toEqual(["Overview", "Saved", "Notes"]);
    expect(await screen.findByTestId("client-placements")).toBeTruthy();
    expect(screen.queryByTestId("client-notes")).toBeNull();
    // Radix tabs activate on mouse down.
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Notes" }));
    expect(await screen.findByTestId("client-notes")).toBeTruthy();
    expect(screen.getByPlaceholderText("Write notes about this client…")).toBeTruthy();
  });
});
