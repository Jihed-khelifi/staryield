// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";

import type { ChatMessage, MessagePage, SincePage } from "@/lib/api/types";

const history = vi.fn<(conv: string, beforeSeq?: number, limit?: number) => Promise<MessagePage>>();
const since = vi.fn<(conv: string, afterSeq: number, limit?: number) => Promise<SincePage>>();
vi.mock("@/lib/api/endpoints", () => ({
  chatApi: {
    history: (conv: string, beforeSeq?: number, limit?: number) => history(conv, beforeSeq, limit),
    since: (conv: string, afterSeq: number, limit?: number) => since(conv, afterSeq, limit),
  },
}));

const { useConversationTimeline } = await import("@/components/chat/use-conversation-timeline");

const msg = (seq: number, sender = "them"): ChatMessage => ({
  id: `m${seq}`,
  seq,
  conversation_id: "conv",
  session_id: "s",
  sender_id: sender,
  kind: "text",
  body: `Message ${seq}`,
  created_at: new Date(Date.UTC(2026, 10, 5, 10, 0, seq)).toISOString(),
});
const range = (from: number, to: number) => Array.from({ length: to - from + 1 }, (_, i) => msg(from + i));
const seqs = (items: { seq?: number }[]) => items.map((m) => m.seq);

beforeEach(() => {
  history.mockReset();
  since.mockReset();
});
afterEach(cleanup);

describe("useConversationTimeline", () => {
  it("loads the newest page and pages back with before_seq", async () => {
    history.mockImplementation(async (_c, before) => (before ? { items: range(41, 50), has_more: false } : { items: range(51, 60), has_more: true, next_before_seq: 51 }));
    const { result } = renderHook(() => useConversationTimeline("conv"));
    expect(result.current.status).toBe("loading");
    await waitFor(() => expect(result.current.status).toBe("ready"));
    expect(seqs(result.current.timeline.messages)).toEqual(range(51, 60).map((m) => m.seq));
    expect(result.current.timeline.hasOlder).toBe(true);
    await act(() => result.current.loadOlder());
    expect(history).toHaveBeenLastCalledWith("conv", 51, 50);
    expect(result.current.timeline.messages).toHaveLength(20);
    expect(result.current.timeline.hasOlder).toBe(false);
  });

  it("fills a gap once over REST, following has_more, with no duplicates", async () => {
    history.mockResolvedValue({ items: range(1, 5), has_more: false });
    let release!: () => void;
    const gate = new Promise<void>((r) => (release = r));
    since.mockImplementation(async (_c, after) => {
      if (after === 5) {
        await gate;
        return { items: range(6, 7), has_more: true, last_seq: 9 };
      }
      return { items: range(8, 9), has_more: false, last_seq: 9 };
    });
    const onBackfilled = vi.fn();
    const { result } = renderHook(() => useConversationTimeline("conv", { onBackfilled }));
    await waitFor(() => expect(result.current.status).toBe("ready"));
    // Events 8 and 9 arrive, but 6 and 7 were missed.
    act(() => result.current.receive([msg(8)]));
    act(() => result.current.receive([msg(9)]));
    act(() => result.current.receive([msg(9)]));
    expect(result.current.timeline.headSeq).toBe(5);
    release();
    await waitFor(() => expect(result.current.timeline.headSeq).toBe(9));
    // One walk (after_seq=5, then 7) despite several gap events.
    expect(since.mock.calls.map((c) => c[1])).toEqual([5, 7]);
    expect(seqs(result.current.timeline.messages)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9]);
    // Only the messages that were not on screen are reported.
    expect(onBackfilled.mock.calls.flatMap((c) => seqs(c[0]))).toEqual([6, 7]);
  });

  it("fills after a reconnect even without a visible gap", async () => {
    history.mockResolvedValue({ items: range(1, 3), has_more: false });
    since.mockResolvedValue({ items: range(4, 4), has_more: false, last_seq: 4 });
    const { result } = renderHook(() => useConversationTimeline("conv"));
    await waitFor(() => expect(result.current.status).toBe("ready"));
    act(() => result.current.fillGaps());
    await waitFor(() => expect(result.current.timeline.headSeq).toBe(4));
    expect(since).toHaveBeenCalledWith("conv", 3, 200);
  });

  it("keeps events that arrive while the first page loads", async () => {
    let resolve!: (p: MessagePage) => void;
    history.mockReturnValue(new Promise((r) => (resolve = r)));
    const { result } = renderHook(() => useConversationTimeline("conv"));
    act(() => result.current.receive([msg(11)]));
    expect(since).not.toHaveBeenCalled();
    await act(async () => resolve({ items: range(1, 10), has_more: false }));
    expect(seqs(result.current.timeline.messages)).toEqual(range(1, 11).map((m) => m.seq));
    expect(result.current.timeline.headSeq).toBe(11);
  });

  it("reports a failed load and recovers on reload", async () => {
    history.mockRejectedValueOnce(new Error("offline")).mockResolvedValue({ items: range(1, 2), has_more: false });
    const { result } = renderHook(() => useConversationTimeline("conv"));
    await waitFor(() => expect(result.current.status).toBe("error"));
    act(() => result.current.reload());
    await waitFor(() => expect(result.current.status).toBe("ready"));
    expect(result.current.timeline.messages).toHaveLength(2);
  });

  it("applies message.liked as authoritative, ignores messages that are not loaded and other conversations", async () => {
    history.mockResolvedValue({ items: range(5, 8), has_more: true, next_before_seq: 5 });
    const { result } = renderHook(() => useConversationTimeline("conv"));
    await waitFor(() => expect(result.current.status).toBe("ready"));
    const likedBy = () => result.current.timeline.messages.map((m) => m.liked_by ?? []);
    act(() => result.current.applyLikes({ message_id: "m6", conversation_id: "conv", liked_by: ["a", "b"] }));
    expect(likedBy()).toEqual([[], ["a", "b"], [], []]);
    // Older than the loaded page, or another conversation: nothing happens.
    act(() => result.current.applyLikes({ message_id: "m1", conversation_id: "conv", liked_by: ["a"] }));
    act(() => result.current.applyLikes({ message_id: "m7", conversation_id: "other", liked_by: ["a"] }));
    expect(likedBy()).toEqual([[], ["a", "b"], [], []]);
    act(() => result.current.applyLikes({ message_id: "m6", conversation_id: "conv", liked_by: [] }));
    expect(likedBy()).toEqual([[], [], [], []]);
  });

  it("an event that omits likes keeps them; the refetch of the newest page makes likes right again", async () => {
    history.mockResolvedValueOnce({ items: [msg(1), { ...msg(2), liked_by: ["a"] }], has_more: false });
    const { result } = renderHook(() => useConversationTimeline("conv"));
    await waitFor(() => expect(result.current.status).toBe("ready"));
    act(() => result.current.receive([msg(2)]));
    expect(result.current.timeline.messages[1].liked_by).toEqual(["a"]);
    // The like was removed while the socket was down: the page no longer lists it.
    history.mockResolvedValueOnce({ items: [msg(1), msg(2)], has_more: false });
    act(() => result.current.refreshNewest());
    await waitFor(() => expect(result.current.timeline.messages[1].liked_by).toEqual([]));
    expect(history).toHaveBeenLastCalledWith("conv", undefined, 50);
    expect(result.current.timeline.messages).toHaveLength(2);
  });

  it("loadOlder says what it did: the page, busy while another load runs, none at the start, error on failure", async () => {
    history.mockImplementation(async (_c, before) => (before ? { items: range(1, 4), has_more: false } : { items: range(5, 8), has_more: true, next_before_seq: 5 }));
    const { result } = renderHook(() => useConversationTimeline("conv"));
    await waitFor(() => expect(result.current.status).toBe("ready"));
    const first = result.current.loadOlder();
    const second = result.current.loadOlder();
    await expect(second).resolves.toEqual({ ok: false, reason: "busy" });
    const done = await act(() => first);
    expect(done).toMatchObject({ ok: true });
    expect((done as { items: ChatMessage[] }).items.map((m) => m.seq)).toEqual([1, 2, 3, 4]);
    await expect(result.current.loadOlder()).resolves.toEqual({ ok: false, reason: "none" });

    history.mockReset();
    history.mockResolvedValueOnce({ items: range(5, 8), has_more: true, next_before_seq: 5 }).mockRejectedValueOnce(new Error("offline"));
    const again = renderHook(() => useConversationTimeline("conv2"));
    await waitFor(() => expect(again.result.current.status).toBe("ready"));
    await expect(again.result.current.loadOlder()).resolves.toEqual({ ok: false, reason: "error" });
  });
});

