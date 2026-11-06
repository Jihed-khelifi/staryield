// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, renderHook, screen, waitFor } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { ChatMessage, Conversation, InboxPage } from "@/lib/api/types";
import { axeViolations, renderWithI18n } from "@/test/render";

const inbox = vi.fn<(cursor?: string, limit?: number) => Promise<InboxPage>>();
vi.mock("@/lib/api/endpoints", () => ({ chatApi: { inbox: (cursor?: string, limit?: number) => inbox(cursor, limit) } }));

const { mergeInbox, sortInbox, useInbox, withMessage, INBOX_PAGE_SIZE } = await import("@/components/chat/use-inbox");
const { ConversationList } = await import("@/components/chat/conversation-list");

const conv = (id: string, at: string, over: Partial<Conversation> = {}): Conversation => ({
  id,
  counterpart: { id: `p-${id}`, role: "psychic", name: `Advisor ${id}`, slug: id, status: "online" },
  unread: 0,
  created_at: "2026-01-01T00:00:00Z",
  last_activity_at: at,
  ...over,
});

beforeEach(() => inbox.mockReset());
afterEach(cleanup);

describe("inbox helpers", () => {
  it("orders by last activity, newest first", () => {
    const out = sortInbox([conv("a", "2026-11-05T10:00:00Z"), conv("b", "2026-11-05T12:00:00Z"), conv("c", "2026-11-05T11:00:00Z")]);
    expect(out.map((c) => c.id)).toEqual(["b", "c", "a"]);
  });

  it("merges a page without duplicates (the page's copy wins)", () => {
    const current = [conv("a", "2026-11-05T10:00:00Z"), conv("b", "2026-11-05T09:00:00Z")];
    const out = mergeInbox(current, [conv("b", "2026-11-05T12:00:00Z", { unread: 2 }), conv("c", "2026-11-05T11:00:00Z")]);
    expect(out.map((c) => c.id)).toEqual(["b", "c", "a"]);
    expect(out[0].unread).toBe(2);
  });

  it("moves a conversation up on a new message and counts it unread unless it is open", () => {
    const items = [conv("a", "2026-11-05T10:00:00Z"), conv("b", "2026-11-05T09:00:00Z")];
    const message = { id: "m9", seq: 9, conversation_id: "b", sender_id: "them", kind: "text", body: "hi", created_at: "2026-11-05T13:00:00Z" } as ChatMessage;
    const moved = withMessage(items, message, { meId: "me", activeId: null })!;
    expect(moved.map((c) => c.id)).toEqual(["b", "a"]);
    expect(moved[0]).toMatchObject({ unread: 1, last_seq: 9, last_message: message });
    expect(withMessage(items, message, { meId: "me", activeId: "b" })![0].unread).toBe(0);
    expect(withMessage(items, { ...message, sender_id: "me" }, { meId: "me", activeId: null })![0].unread).toBe(0);
    expect(withMessage(items, { ...message, conversation_id: "zzz" }, { meId: "me", activeId: null })).toBeNull();
  });
});

describe("useInbox (cursor pages)", () => {
  it("loads the first page, then the next pages on demand", async () => {
    inbox.mockImplementation(async (cursor) =>
      cursor ? { items: [conv("c", "2026-11-05T08:00:00Z")], has_more: false } : { items: [conv("a", "2026-11-05T10:00:00Z"), conv("b", "2026-11-05T09:00:00Z")], next_cursor: "cur-1", has_more: true },
    );
    const onFirstPage = vi.fn();
    const { result } = renderHook(() => useInbox(true, { onFirstPage }));
    expect(result.current.status).toBe("loading");
    await waitFor(() => expect(result.current.status).toBe("ready"));
    expect(inbox).toHaveBeenCalledWith(undefined, INBOX_PAGE_SIZE);
    expect(onFirstPage).toHaveBeenCalledOnce();
    expect(result.current.hasMore).toBe(true);
    act(() => result.current.loadMore());
    await waitFor(() => expect(result.current.items).toHaveLength(3));
    expect(inbox).toHaveBeenLastCalledWith("cur-1", INBOX_PAGE_SIZE);
    expect(result.current.hasMore).toBe(false);
  });

  it("refreshes the first page without dropping later pages", async () => {
    let first = [conv("a", "2026-11-05T10:00:00Z")];
    inbox.mockImplementation(async (cursor) => (cursor ? { items: [conv("z", "2026-11-04T10:00:00Z")], has_more: false } : { items: first, next_cursor: "cur-1", has_more: true }));
    const { result } = renderHook(() => useInbox(true));
    await waitFor(() => expect(result.current.status).toBe("ready"));
    act(() => result.current.loadMore());
    await waitFor(() => expect(result.current.items).toHaveLength(2));
    first = [conv("new", "2026-11-05T12:00:00Z"), conv("a", "2026-11-05T10:00:00Z")];
    act(() => result.current.refresh());
    await waitFor(() => expect(result.current.items.map((c) => c.id)).toEqual(["new", "a", "z"]));
    expect(result.current.hasMore).toBe(false);
  });

  it("reports a failed first page, and a failed next page separately", async () => {
    inbox.mockRejectedValueOnce(new ApiError({ code: "network_error", message: "offline" }));
    const { result } = renderHook(() => useInbox(true));
    await waitFor(() => expect(result.current.status).toBe("error"));
    inbox.mockResolvedValueOnce({ items: [conv("a", "2026-11-05T10:00:00Z")], next_cursor: "cur-1", has_more: true });
    act(() => result.current.refresh());
    await waitFor(() => expect(result.current.status).toBe("ready"));
    inbox.mockRejectedValueOnce(new ApiError({ code: "timeout", message: "slow" }));
    act(() => result.current.loadMore());
    await waitFor(() => expect(result.current.moreError?.code).toBe("timeout"));
    expect(result.current.items).toHaveLength(1);
  });
});

describe("ConversationList states", () => {
  const list = (props: Partial<React.ComponentProps<typeof ConversationList>>) =>
    renderWithI18n(<ConversationList conversations={[]} activeId={null} onSelect={vi.fn()} {...props} />);

  it("shows a skeleton, an error with retry, an empty state, and a load-more button", async () => {
    const { unmount } = list({ status: "loading" });
    expect(screen.getByTestId("inbox-loading")).toBeTruthy();
    unmount();
    const onRetry = vi.fn();
    const failed = list({ status: "error", error: new ApiError({ code: "network_error", message: "offline" }), onRetry });
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(onRetry).toHaveBeenCalled();
    failed.unmount();
    const empty = list({ status: "ready" });
    expect(screen.getByTestId("inbox-empty")).toBeTruthy();
    empty.unmount();
    const onLoadMore = vi.fn();
    list({ status: "ready", conversations: [conv("a", "2026-11-05T10:00:00Z", { unread: 3 })], hasMore: true, onLoadMore });
    fireEvent.click(screen.getByRole("button", { name: "Load more conversations" }));
    expect(onLoadMore).toHaveBeenCalled();
    expect(await axeViolations()).toEqual([]);
  });

  it("names the advisor's availability on the switcher cards", () => {
    list({
      status: "ready",
      variant: "cards",
      excludeId: "a",
      conversations: [conv("a", "2026-11-05T10:00:00Z"), conv("b", "2026-11-05T09:00:00Z", { counterpart: { id: "p-b", role: "psychic", name: "Theo", slug: "theo", status: "busy" } })],
    });
    const rows = screen.getAllByTestId("inbox-row");
    expect(rows).toHaveLength(1);
    expect(rows[0].textContent).toContain("Theo · Busy");
  });
});
