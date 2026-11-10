// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";

import type { ReplyPreview } from "@/lib/api/types";
import { JUMP_BUSY_TRIES, JUMP_MAX_PAGES, useJumpToMessage } from "@/components/chat/use-jump-to-message";
import type { LoadOlderResult } from "@/components/chat/use-conversation-timeline";

beforeEach(() => vi.useFakeTimers());
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

const QUOTE: ReplyPreview = { id: "m3", seq: 3, sender_id: "them", kind: "text", excerpt: "old" };
const page: LoadOlderResult = { ok: true, items: [] };

function setup(opts: { onScreen?: () => boolean; loads?: LoadOlderResult[]; conversationId?: string | null } = {}) {
  const loads = [...(opts.loads ?? [])];
  const scrollTo = vi.fn(() => opts.onScreen?.() ?? false);
  const loadOlder = vi.fn(async () => loads.shift() ?? ({ ok: false, reason: "none" } as LoadOlderResult));
  const onFailed = vi.fn();
  const onSearching = vi.fn();
  const hook = renderHook((props: { conversationId: string | null }) => useJumpToMessage({ scrollTo, loadOlder, conversationId: props.conversationId, onFailed, onSearching }), {
    initialProps: { conversationId: opts.conversationId === undefined ? "c1" : opts.conversationId },
  });
  const jump = async () => {
    let done!: Promise<void>;
    await act(async () => {
      done = hook.result.current.jumpTo(QUOTE);
      await vi.advanceTimersByTimeAsync(0);
    });
    // Boxed: returning the promise itself would make `await jump()` wait for the whole search.
    return { done };
  };
  const finish = async (run: { done: Promise<void> }) =>
    act(async () => {
      await vi.advanceTimersByTimeAsync(30000);
      await run.done;
    });
  return { ...hook, scrollTo, loadOlder, onFailed, onSearching, jump, finish };
}

describe("useJumpToMessage (§23.2)", () => {
  it("scrolls straight to a message that is on screen, loading nothing", async () => {
    const s = setup({ onScreen: () => true });
    await s.finish(await s.jump());
    expect(s.scrollTo).toHaveBeenCalledWith("m3");
    expect(s.loadOlder).not.toHaveBeenCalled();
    expect(s.onSearching).not.toHaveBeenCalled();
  });

  it("loads older pages until the message appears, with a loading state meanwhile", async () => {
    let shown = false;
    const s = setup({ onScreen: () => shown, loads: [page, page, page] });
    s.loadOlder.mockImplementation(async () => {
      if (s.loadOlder.mock.calls.length === 2) shown = true;
      return page;
    });
    const run = await s.jump();
    expect(s.result.current.jumpingTo).toBe("m3");
    expect(s.onSearching).toHaveBeenCalledTimes(1);
    await s.finish(run);
    expect(s.loadOlder).toHaveBeenCalledTimes(2);
    expect(s.result.current.jumpingTo).toBeNull();
    expect(s.onFailed).not.toHaveBeenCalled();
  });

  it("stops after a bounded number of pages and offers a retry", async () => {
    const s = setup({ loads: Array.from({ length: JUMP_MAX_PAGES + 5 }, () => page) });
    await s.finish(await s.jump());
    expect(s.loadOlder).toHaveBeenCalledTimes(JUMP_MAX_PAGES);
    expect(s.onFailed).toHaveBeenCalledWith("failed", expect.any(Function));
  });

  it("a failed page load reports it with a retry that tries again", async () => {
    const s = setup({ loads: [{ ok: false, reason: "error" }] });
    await s.finish(await s.jump());
    expect(s.onFailed).toHaveBeenCalledWith("failed", expect.any(Function));
    const retry = s.onFailed.mock.calls[0][1] as () => void;
    s.loadOlder.mockClear();
    await act(async () => {
      retry();
      await vi.advanceTimersByTimeAsync(30000);
    });
    expect(s.loadOlder).toHaveBeenCalled();
    expect(s.result.current.jumpingTo).toBeNull();
  });

  it("reaching the start of the conversation without finding it says it is gone, without a retry", async () => {
    const s = setup({ loads: [page, { ok: false, reason: "none" }] });
    await s.finish(await s.jump());
    expect(s.onFailed).toHaveBeenCalledWith("unavailable", null);
  });

  it("waits for another load to finish (busy) instead of failing, but not forever", async () => {
    let shown = false;
    const s = setup({ onScreen: () => shown, loads: [{ ok: false, reason: "busy" }, { ok: false, reason: "busy" }] });
    s.loadOlder.mockImplementationOnce(async () => ({ ok: false, reason: "busy" }));
    s.loadOlder.mockImplementationOnce(async () => {
      shown = true;
      return { ok: false, reason: "busy" };
    });
    await s.finish(await s.jump());
    expect(s.onFailed).not.toHaveBeenCalled();

    const stuck = setup({});
    stuck.loadOlder.mockImplementation(async () => ({ ok: false, reason: "busy" }));
    await stuck.finish(await stuck.jump());
    expect(stuck.loadOlder).toHaveBeenCalledTimes(JUMP_BUSY_TRIES + 1);
    expect(stuck.onFailed).toHaveBeenCalledWith("failed", expect.any(Function));
  });

  it("is abandoned silently when another conversation is opened", async () => {
    const s = setup({ loads: [page, page, page] });
    const run = await s.jump();
    s.rerender({ conversationId: "c2" });
    await s.finish(run);
    expect(s.onFailed).not.toHaveBeenCalled();
    expect(s.result.current.jumpingTo).toBeNull();
  });

  it("a second jump while one runs is ignored", async () => {
    const s = setup({ loads: [page] });
    const first = await s.jump();
    await s.jump();
    await s.finish(first);
    expect(s.loadOlder.mock.calls.length).toBeLessThanOrEqual(JUMP_MAX_PAGES);
    expect(s.onSearching).toHaveBeenCalledTimes(1);
  });
});
