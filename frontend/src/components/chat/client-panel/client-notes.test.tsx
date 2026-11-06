// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { Note, Page, SharedNote } from "@/lib/api/types";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const { notesApi, realtime } = vi.hoisted(() => ({
  notesApi: { list: vi.fn(), create: vi.fn(), update: vi.fn(), remove: vi.fn(), shared: vi.fn() },
  realtime: new Map<string, (data: unknown) => void>(),
}));
vi.mock("@/lib/api/endpoints", () => ({ notesApi }));
vi.mock("@/lib/auth/session-provider", () => ({
  useRealtimeEvent: (type: string, handler: (data: unknown) => void) => {
    realtime.set(type, handler);
  },
}));

import { ClientNotes } from "@/components/chat/client-panel/client-notes";
import { noteErrorCopy } from "@/components/chat/client-panel/panel-helpers";

beforeAll(stubResizeObserver);
afterEach(cleanup);
beforeEach(() => {
  for (const fn of Object.values(notesApi)) fn.mockReset();
  realtime.clear();
});

const AT = new Date(Date.now() - 5 * 60_000).toISOString();
const note = (n: number, over: Partial<Note> = {}): Note => ({ id: `note-${n}`, body: `Note ${n}`, shared: false, created_at: AT, updated_at: AT, ...over });
const page = <T,>(items: T[], next_cursor?: string): Page<T> => ({ items, next_cursor });

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function renderNotes() {
  return renderWithI18n(
    <ToastProvider>
      <ClientNotes conversationId="conv-1" clientName="Paul" />
    </ToastProvider>,
  );
}

const composer = () => screen.getByPlaceholderText("Write notes about this client…") as HTMLTextAreaElement;

describe("ClientNotes", () => {
  it("shows five notes per page and pages with the cursor", async () => {
    notesApi.list.mockImplementation(async (_id: string, cursor?: string) =>
      cursor === "c2" ? page([note(6), note(7)]) : page([1, 2, 3, 4, 5].map((n) => note(n)), "c2"),
    );
    renderNotes();
    await waitFor(() => expect(screen.getAllByTestId("note-card")).toHaveLength(5));
    expect(notesApi.list).toHaveBeenCalledWith("conv-1", undefined, 5, expect.any(AbortSignal));
    expect(screen.getAllByText("Saved 5 min. ago").length).toBe(5);

    fireEvent.click(screen.getByRole("button", { name: /Next/ }));
    await waitFor(() => expect(screen.getAllByTestId("note-card")).toHaveLength(2));
    expect(notesApi.list).toHaveBeenLastCalledWith("conv-1", "c2", 5, expect.any(AbortSignal));
    expect(screen.getByText("Page 2")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: /Previous/ }));
    expect(screen.getAllByTestId("note-card")).toHaveLength(5);
    expect(await axeViolations()).toEqual([]);
  });

  it("adds a note at once and swaps in the saved one", async () => {
    notesApi.list.mockResolvedValue(page([note(1)]));
    const create = deferred<Note>();
    notesApi.create.mockReturnValue(create.promise);
    renderNotes();
    await screen.findByText("Note 1");

    fireEvent.change(composer(), { target: { value: "  Lost her dog yesterday  " } });
    fireEvent.click(screen.getByRole("button", { name: "Save note" }));

    // Optimistic: on top, marked as saving, composer cleared.
    await waitFor(() => expect(screen.getAllByTestId("note-card")[0].textContent).toContain("Lost her dog yesterday"));
    expect(within(screen.getAllByTestId("note-card")[0]).getByText("Saving…")).toBeTruthy();
    expect(composer().value).toBe("");
    expect(notesApi.create).toHaveBeenCalledWith("conv-1", { body: "Lost her dog yesterday", shared: false });

    const now = new Date().toISOString();
    await act(async () => create.resolve({ id: "note-9", body: "Lost her dog yesterday", shared: false, created_at: now, updated_at: now }));
    const first = screen.getAllByTestId("note-card")[0];
    expect(within(first).getByText("Saved just now")).toBeTruthy();
    expect(within(first).getByRole("button", { name: "Edit" })).toBeTruthy();
    expect(screen.getAllByTestId("note-card")).toHaveLength(2);
  });

  it("rolls back and gives the text back when the limit is reached", async () => {
    notesApi.list.mockResolvedValue(page([]));
    notesApi.create.mockRejectedValue(new ApiError({ code: "limit_reached", message: "server", status: 409 }));
    renderNotes();
    await screen.findByText(/No notes yet/);

    fireEvent.change(composer(), { target: { value: "One more note" } });
    fireEvent.click(screen.getByRole("button", { name: "Save note" }));

    await waitFor(() => expect(screen.getByRole("alert").textContent).toContain(noteErrorCopy.limit_reached));
    expect(screen.queryAllByTestId("note-card")).toHaveLength(0);
    expect(composer().value).toBe("One more note");
  });

  it("sends shared: true only when the advisor ticks the box", async () => {
    notesApi.list.mockResolvedValue(page([]));
    notesApi.create.mockImplementation(async (_id: string, input: { body: string; shared: boolean }) => ({ ...note(3), ...input }));
    renderNotes();
    await screen.findByText(/No notes yet/);
    const share = screen.getByRole("checkbox", { name: "Share with other advisors" });
    expect(share.getAttribute("aria-checked")).toBe("false");
    fireEvent.click(share);
    fireEvent.change(composer(), { target: { value: "Shared insight" } });
    // Cmd/Ctrl+Enter saves.
    fireEvent.keyDown(composer(), { key: "Enter", ctrlKey: true });
    await waitFor(() => expect(notesApi.create).toHaveBeenCalledWith("conv-1", { body: "Shared insight", shared: true }));
    expect(await screen.findByTestId("note-shared-badge")).toBeTruthy();
    // The box is unticked again for the next note.
    expect(screen.getByRole("checkbox", { name: "Share with other advisors" }).getAttribute("aria-checked")).toBe("false");
  });

  it("validates the note before saving", async () => {
    notesApi.list.mockResolvedValue(page([]));
    renderNotes();
    await screen.findByText(/No notes yet/);
    fireEvent.click(screen.getByRole("button", { name: "Save note" }));
    expect(await screen.findByText("Write a note first.")).toBeTruthy();
    fireEvent.change(composer(), { target: { value: "x".repeat(2001) } });
    fireEvent.click(screen.getByRole("button", { name: "Save note" }));
    expect(await screen.findByText("Use at most 2000 characters.")).toBeTruthy();
    expect(notesApi.create).not.toHaveBeenCalled();
  });

  it("toggles sharing optimistically and rolls back on failure", async () => {
    notesApi.list.mockResolvedValue(page([note(1)]));
    notesApi.update.mockResolvedValueOnce({ ...note(1), shared: true });
    renderNotes();
    await screen.findByText("Note 1");

    fireEvent.click(screen.getByRole("button", { name: "Share" }));
    expect(screen.getByTestId("note-shared-badge").textContent).toContain("Shared");
    await screen.findByText("Note shared with other advisors.");
    expect(notesApi.update).toHaveBeenCalledWith("note-1", { shared: true });

    notesApi.update.mockRejectedValueOnce(new ApiError({ code: "internal_error", message: "x", status: 500 }));
    fireEvent.click(screen.getByRole("button", { name: "Stop sharing" }));
    expect(screen.queryByTestId("note-shared-badge")).toBeNull();
    // Rolled back to shared.
    await waitFor(() => expect(screen.getByTestId("note-shared-badge")).toBeTruthy());
    expect(screen.getByRole("button", { name: "Stop sharing" })).toBeTruthy();
  });

  it("edits inline and deletes after confirmation", async () => {
    notesApi.list.mockResolvedValue(page([note(1)]));
    notesApi.update.mockImplementation(async (_id: string, patch: { body: string }) => ({ ...note(1), ...patch, updated_at: new Date().toISOString() }));
    notesApi.remove.mockResolvedValue(undefined);
    renderNotes();
    await screen.findByText("Note 1");

    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    const edit = screen.getByRole("textbox", { name: "Edit note" }) as HTMLTextAreaElement;
    fireEvent.change(edit, { target: { value: "Note 1, revised" } });
    fireEvent.keyDown(edit, { key: "Enter", metaKey: true });
    await screen.findByText("Note 1, revised");
    expect(notesApi.update).toHaveBeenCalledWith("note-1", { body: "Note 1, revised" });
    expect(screen.getByText("Edited just now")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    const confirm = await screen.findByRole("alertdialog", { name: "Delete this note?" });
    expect(notesApi.remove).not.toHaveBeenCalled();
    fireEvent.click(within(confirm).getByRole("button", { name: "Delete note" }));
    await waitFor(() => expect(notesApi.remove).toHaveBeenCalledWith("note-1"));
    await screen.findByText("Note deleted.");
  });

  it("explains that notes start after the first session, and opens up once it starts", async () => {
    notesApi.list.mockRejectedValueOnce(new ApiError({ code: "forbidden", message: "x", status: 403 })).mockResolvedValue(page([]));
    renderNotes();
    expect((await screen.findByTestId("notes-blocked")).textContent).toBe(noteErrorCopy.forbidden);
    expect(screen.queryByTestId("note-composer")).toBeNull();

    // Another conversation's chat does not matter; this one's does.
    act(() => realtime.get("session.started")?.({ session: { conversation_id: "conv-2" } }));
    expect(notesApi.list).toHaveBeenCalledTimes(1);
    act(() => realtime.get("session.started")?.({ session: { conversation_id: "conv-1" } }));
    expect(await screen.findByTestId("note-composer")).toBeTruthy();
    expect(notesApi.list).toHaveBeenCalledTimes(2);
  });
});

describe("Notes from other psychics", () => {
  const shared = (n: number): SharedNote => ({
    id: `s-${n}`,
    body: `Shared ${n}`,
    author: { name: n % 2 ? "Ramone" : "Theo", slug: "x" },
    created_at: "2026-10-13T10:00:00Z",
    updated_at: "2026-10-13T10:00:00Z",
  });

  it("loads only when opened, then pages through shared notes", async () => {
    notesApi.list.mockResolvedValue(page([]));
    notesApi.shared.mockImplementation(async (_id: string, cursor?: string) =>
      cursor ? page([shared(6)]) : page([1, 2, 3, 4, 5].map(shared), "n2"),
    );
    renderNotes();
    await screen.findByText(/No notes yet/);
    expect(notesApi.shared).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "+ Check other advisor's notes on this user" }));
    const dialog = await screen.findByRole("dialog", { name: "Notes from other psychics" });
    await waitFor(() => expect(within(dialog).getAllByTestId("other-note")).toHaveLength(5));
    expect(within(dialog).getAllByText("Ramone").length).toBe(3);
    expect(within(dialog).getAllByText("Oct 13, 2026").length).toBe(5);
    expect(await axeViolations(dialog)).toEqual([]);

    fireEvent.click(within(dialog).getByRole("button", { name: /Next/ }));
    await waitFor(() => expect(within(dialog).getAllByTestId("other-note")).toHaveLength(1));
    expect(notesApi.shared).toHaveBeenLastCalledWith("conv-1", "n2", 5, expect.any(AbortSignal));
  });

  it("has an empty state", async () => {
    notesApi.list.mockResolvedValue(page([]));
    notesApi.shared.mockResolvedValue(page([]));
    renderNotes();
    await screen.findByText(/No notes yet/);
    fireEvent.click(screen.getByRole("button", { name: "+ Check other advisor's notes on this user" }));
    expect(await screen.findByText("No shared notes yet")).toBeTruthy();
  });
});
