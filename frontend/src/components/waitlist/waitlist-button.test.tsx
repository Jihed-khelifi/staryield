// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { Page, WaitlistEntry } from "@/lib/api/types";
import { axeViolations, renderWithI18n } from "@/test/render";

const join = vi.fn<(slug: string) => Promise<WaitlistEntry>>();
const leave = vi.fn<(slug: string) => Promise<void>>();
const mine = vi.fn<() => Promise<Page<WaitlistEntry>>>();
const push = vi.fn();
const toastShow = vi.fn();
const toastError = vi.fn();
let session: { status: string; user: { id: string; role: string } | null } = { status: "anonymous", user: null };

vi.mock("@/lib/api/endpoints", () => ({
  waitlistApi: { join: (s: string) => join(s), leave: (s: string) => leave(s), mine: () => mine() },
}));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ ...session, realtime: null }) }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => ({ show: toastShow, showError: toastError }) }));

const { WaitlistButton } = await import("@/components/waitlist/waitlist-button");

const entry = (slug: string): WaitlistEntry => ({ slug, name: "Theo", image_url: "", status: "busy", joined_at: "2026-11-05T10:00:00Z" });

beforeEach(() => {
  for (const f of [join, leave, mine, push, toastShow, toastError]) f.mockReset();
  mine.mockResolvedValue({ items: [] });
  window.history.replaceState(null, "", "/en/psychics/theo");
});
afterEach(cleanup);

describe("WaitlistButton", () => {
  it("sends visitors to sign in first, coming back to this page", async () => {
    session = { status: "anonymous", user: null };
    renderWithI18n(<WaitlistButton slug="theo" name="Theo" />);
    const button = screen.getByRole("button", { name: "Join Waiting List" });
    expect(button.className).toContain("glass");
    fireEvent.click(button);
    expect(push).toHaveBeenCalledWith("/en/login?next=%2Fen%2Fpsychics%2Ftheo");
    expect(join).not.toHaveBeenCalled();
    expect(mine).not.toHaveBeenCalled();
    expect(await axeViolations()).toEqual([]);
  });

  it("lets a client join and leave, announcing each change", async () => {
    session = { status: "authenticated", user: { id: "u1", role: "client" } };
    join.mockResolvedValue(entry("theo"));
    leave.mockResolvedValue();
    renderWithI18n(<WaitlistButton slug="theo" name="Theo" />);
    await waitFor(() => expect((screen.getByTestId("waitlist-button") as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(screen.getByRole("button", { name: "Join Waiting List" }));
    const joined = await screen.findByRole("button", { name: /On the waiting list.*select to leave/ });
    expect(join).toHaveBeenCalledWith("theo");
    expect(toastShow).toHaveBeenCalledWith(expect.objectContaining({ tone: "success" }));
    expect(joined.getAttribute("data-joined")).toBe("true");
    expect(await axeViolations()).toEqual([]);
    fireEvent.click(joined);
    await screen.findByRole("button", { name: "Join Waiting List" });
    expect(leave).toHaveBeenCalledWith("theo");
  });

  it("reads the joined state from GET /me/waitlist", async () => {
    session = { status: "authenticated", user: { id: "u1", role: "client" } };
    mine.mockResolvedValue({ items: [entry("ramone"), entry("theo")] });
    renderWithI18n(<WaitlistButton slug="theo" name="Theo" />);
    expect(await screen.findByRole("button", { name: /On the waiting list/ })).toBeTruthy();
    expect(mine).toHaveBeenCalledOnce();
  });

  it("switches to the chat when the advisor is available (409 advisor_available)", async () => {
    session = { status: "authenticated", user: { id: "u1", role: "client" } };
    join.mockRejectedValue(new ApiError({ code: "advisor_available", message: "online", status: 409 }));
    const onAvailable = vi.fn();
    renderWithI18n(<WaitlistButton slug="theo" name="Theo" onAvailable={onAvailable} />);
    await waitFor(() => expect((screen.getByTestId("waitlist-button") as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(screen.getByRole("button", { name: "Join Waiting List" }));
    await waitFor(() => expect(onAvailable).toHaveBeenCalledOnce());
    expect(toastShow).toHaveBeenCalledWith({ tone: "info", message: "Theo is online now. Start your chat." });
    expect(toastError).not.toHaveBeenCalled();
  });

  it("shows any other error as a toast and stays joinable", async () => {
    session = { status: "authenticated", user: { id: "u1", role: "client" } };
    const limit = new ApiError({ code: "limit_reached", message: "20 lists", status: 409 });
    join.mockRejectedValue(limit);
    renderWithI18n(<WaitlistButton slug="theo" name="Theo" />);
    await waitFor(() => expect((screen.getByTestId("waitlist-button") as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(screen.getByRole("button", { name: "Join Waiting List" }));
    await waitFor(() => expect(toastError).toHaveBeenCalledWith(limit));
    expect(screen.getByRole("button", { name: "Join Waiting List" })).toBeTruthy();
  });

  it("renders nothing for advisors and admins", () => {
    session = { status: "authenticated", user: { id: "p1", role: "psychic" } };
    const { container } = renderWithI18n(<WaitlistButton slug="theo" name="Theo" />);
    expect(container.querySelector("button")).toBeNull();
    expect(mine).not.toHaveBeenCalled();
  });
});
