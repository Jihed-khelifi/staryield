// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { ClientProfile, Participant } from "@/lib/api/types";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const h = vi.hoisted(() => ({
  session: { status: "authenticated", user: { id: "adv", role: "psychic" } } as { status: string; user: { id: string; role: string } | null },
  clientProfile: vi.fn(),
  lookup: vi.fn(),
  set: vi.fn(),
  clear: vi.fn(),
  announce: vi.fn(),
}));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => h.session, useRealtimeEvent: () => undefined }));
vi.mock("@/lib/api/endpoints", () => ({
  chatApi: { clientProfile: (id: string) => h.clientProfile(id) },
  aliasApi: { lookup: (...a: unknown[]) => h.lookup(...a), set: (...a: unknown[]) => h.set(...a), clear: (...a: unknown[]) => h.clear(...a) },
}));
vi.mock("@/components/feedback/live-region", () => ({ announce: (...a: unknown[]) => h.announce(...a) }));

import { ChatHeader } from "@/components/chat/chat-header";
import { clearClientProfileCache } from "@/components/chat/use-client-profile";
import { ClientAliasProvider } from "@/lib/chat/client-alias-provider";

const client: Participant = { id: "c1", role: "client", name: "Paul", status: "online" };
const profile = (over: Partial<ClientProfile> = {}): ClientProfile =>
  ({ id: "c1", name: "Paul", online: true, locale: "en", member_since: "2025-01-01T00:00:00Z", birth_details_shared: false, history: { sessions: 0, minutes: 0 }, ...over }) as ClientProfile;

const noop = vi.fn();
function header(props: Partial<React.ComponentProps<typeof ChatHeader>> = {}) {
  return renderWithI18n(
    <ClientAliasProvider>
      <ChatHeader counterpart={client} conversationId="conv-1" viewerRole="psychic" session={null} elapsedSeconds={0} onOpenInbox={noop} onOpenDetails={noop} onEnd={noop} ending={false} {...props} />
    </ClientAliasProvider>,
  );
}
const ok = (name: string) => ({ client_id: "c1", alias: name, updated_at: "2026-11-05T10:00:00Z" });
const pen = () => screen.getByRole("button", { name: "Edit nickname" });
const input = () => screen.getByRole("textbox", { name: "Nickname for Paul" }) as HTMLInputElement;
const open = async () => {
  fireEvent.click(await screen.findByRole("button", { name: "Edit nickname" }));
  return input();
};

beforeAll(stubResizeObserver);
beforeEach(() => {
  h.session = { status: "authenticated", user: { id: "adv", role: "psychic" } };
  clearClientProfileCache();
  h.clientProfile.mockReset().mockResolvedValue(profile());
  h.lookup.mockReset().mockResolvedValue([]);
  h.set.mockReset().mockImplementation(async (_c: string, alias: string) => ok(alias));
  h.clear.mockReset().mockResolvedValue(undefined);
  h.announce.mockReset();
});
afterEach(cleanup);

describe("nickname editor in the advisor's chat header (§23.4)", () => {
  it("a pen next to the name opens an input in place, prefilled, with the real name as placeholder", async () => {
    h.clientProfile.mockResolvedValue(profile({ alias: "Moon Paul" }));
    header();
    expect(await screen.findByText("Moon Paul")).toBeTruthy();
    const field = await open();
    expect(field.value).toBe("Moon Paul");
    expect(field.placeholder).toBe("Paul");
    expect(field.maxLength).toBe(40);
    // Replaced in place: the name text is gone while editing.
    expect(screen.queryByTestId("chat-header-name")).toBeNull();
    expect(document.activeElement).toBe(field);
    expect(screen.getByRole("button", { name: "Save nickname" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeTruthy();
  });

  it("Enter saves: the header shows the nickname, the editor closes, focus returns to the pen and it is announced", async () => {
    header();
    const field = await open();
    fireEvent.change(field, { target: { value: "  Moon   Paul " } });
    fireEvent.submit(field.closest("form")!);
    await waitFor(() => expect(screen.queryByTestId("nickname-form")).toBeNull());
    expect(h.set).toHaveBeenCalledWith("conv-1", "Moon Paul");
    expect(screen.getByTestId("chat-header-name").textContent).toBe("Moon Paul");
    expect(h.announce).toHaveBeenCalledWith("Nickname saved");
    expect(document.activeElement).toBe(pen());
  });

  it("the confirm button saves too", async () => {
    header();
    const field = await open();
    fireEvent.change(field, { target: { value: "Sun" } });
    fireEvent.click(screen.getByRole("button", { name: "Save nickname" }));
    await waitFor(() => expect(h.set).toHaveBeenCalledWith("conv-1", "Sun"));
  });

  it("Esc cancels without a request and keeps the name", async () => {
    header();
    const field = await open();
    fireEvent.change(field, { target: { value: "Something else" } });
    fireEvent.keyDown(field, { key: "Escape" });
    expect(screen.queryByTestId("nickname-form")).toBeNull();
    expect(h.set).not.toHaveBeenCalled();
    expect(screen.getByTestId("chat-header-name").textContent).toBe("Paul");
    expect(document.activeElement).toBe(pen());
  });

  it("the cancel button closes it", async () => {
    header();
    await open();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByTestId("nickname-form")).toBeNull();
    expect(h.set).not.toHaveBeenCalled();
  });

  it("an empty value clears the nickname", async () => {
    h.clientProfile.mockResolvedValue(profile({ alias: "Moon Paul" }));
    header();
    await screen.findByText("Moon Paul");
    const field = await open();
    fireEvent.change(field, { target: { value: "   " } });
    fireEvent.submit(field.closest("form")!);
    await waitFor(() => expect(screen.queryByTestId("nickname-form")).toBeNull());
    expect(h.clear).toHaveBeenCalledWith("conv-1");
    expect(h.set).not.toHaveBeenCalled();
    expect(screen.getByTestId("chat-header-name").textContent).toBe("Paul");
    expect(h.announce).toHaveBeenCalledWith("Nickname removed");
  });

  it("an unchanged value makes no request", async () => {
    h.clientProfile.mockResolvedValue(profile({ alias: "Moon Paul" }));
    header();
    await screen.findByText("Moon Paul");
    const field = await open();
    fireEvent.submit(field.closest("form")!);
    expect(screen.queryByTestId("nickname-form")).toBeNull();
    expect(h.set).not.toHaveBeenCalled();
    expect(h.clear).not.toHaveBeenCalled();
  });

  it("while saving: the controls are disabled and a spinner shows", async () => {
    let finish!: (a: unknown) => void;
    h.set.mockReturnValue(new Promise((r) => (finish = r)));
    header();
    const field = await open();
    fireEvent.change(field, { target: { value: "Sun" } });
    fireEvent.submit(field.closest("form")!);
    await waitFor(() => expect(screen.getByTestId("nickname-saving")).toBeTruthy());
    expect(input().disabled).toBe(true);
    expect((screen.getByRole("button", { name: "Save nickname" }) as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByRole("button", { name: "Cancel" }) as HTMLButtonElement).disabled).toBe(true);
    expect(input().getAttribute("aria-busy")).toBe("true");
    // Esc is ignored while the request runs.
    fireEvent.keyDown(input(), { key: "Escape" });
    expect(screen.getByTestId("nickname-form")).toBeTruthy();
    await act(async () => finish(ok("Sun")));
    await waitFor(() => expect(screen.queryByTestId("nickname-form")).toBeNull());
  });

  it("an error shows under the field with the API's copy, keeps the editor open and allows a retry", async () => {
    h.set.mockRejectedValueOnce(new ApiError({ code: "validation_failed", message: "bad", status: 422, details: { fields: { alias: "Nicknames cannot contain control characters." } } }));
    header();
    const field = await open();
    fireEvent.change(field, { target: { value: "Bad\u0007" } });
    fireEvent.submit(field.closest("form")!);
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toBe("Nicknames cannot contain control characters.");
    expect(input().getAttribute("aria-invalid")).toBe("true");
    expect(input().getAttribute("aria-describedby")).toBe(alert.id);
    expect(input().disabled).toBe(false);
    // The editor stays open; typing clears the error and a retry works.
    fireEvent.change(input(), { target: { value: "Good" } });
    expect(screen.queryByRole("alert")).toBeNull();
    fireEvent.submit(input().closest("form")!);
    await waitFor(() => expect(screen.queryByTestId("nickname-form")).toBeNull());
    expect(screen.getByTestId("chat-header-name").textContent).toBe("Good");
  });

  it("falls back to the generic message for a network failure", async () => {
    h.set.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    header();
    const field = await open();
    fireEvent.change(field, { target: { value: "Sun" } });
    fireEvent.submit(field.closest("form")!);
    expect((await screen.findByRole("alert")).textContent).toMatch(/connect|network|reach/i);
    expect(screen.getByTestId("nickname-form")).toBeTruthy();
  });

  it("with a nickname, the real name is in the tooltip and the title", async () => {
    h.clientProfile.mockResolvedValue(profile({ alias: "Moon Paul" }));
    header();
    const name = await screen.findByText("Moon Paul");
    expect(name.getAttribute("title")).toBe("Name on the account: Paul");
  });

  it("a nickname replaces the name on the avatar button and the End chat label too", async () => {
    h.clientProfile.mockResolvedValue(profile({ alias: "Moon Paul" }));
    header({ session: { status: "active" } as never });
    await screen.findByText("Moon Paul");
    expect(screen.getByRole("button", { name: "View Moon Paul's details" })).toBeTruthy();
  });

  it("clients never get a pen, whoever the counterpart is", () => {
    h.session = { status: "authenticated", user: { id: "cl", role: "client" } };
    header({ viewerRole: "client", counterpart: { id: "p1", role: "psychic", name: "Theo", slug: "theo", status: "online" } });
    expect(screen.queryByRole("button", { name: "Edit nickname" })).toBeNull();
    expect(screen.getByText("Theo")).toBeTruthy();
    expect(h.lookup).not.toHaveBeenCalled();
  });

  it("is primed by the client profile's alias without a lookup", async () => {
    h.clientProfile.mockResolvedValue(profile({ alias: "From profile" }));
    header();
    expect(await screen.findByText("From profile")).toBeTruthy();
  });

  it("has no serious accessibility violations, closed or editing", async () => {
    h.clientProfile.mockResolvedValue(profile({ alias: "Moon Paul" }));
    header();
    await screen.findByText("Moon Paul");
    expect(await axeViolations()).toEqual([]);
    await open();
    expect(await axeViolations()).toEqual([]);
    within(screen.getByTestId("nickname-form"));
  });
});
