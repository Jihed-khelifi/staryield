// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { MessageLikes } from "@/lib/api/types";
import type { UiMessage } from "@/components/chat/chat-model";

const like = vi.fn();
const unlike = vi.fn();
vi.mock("@/lib/api/endpoints", () => ({ chatApi: { like: (id: string) => like(id), unlike: (id: string) => unlike(id) } }));

const { useMessageLikes, withLikeOverrides } = await import("@/components/chat/use-message-likes");

const ME = "me";
const msg = (extra: Partial<UiMessage> = {}): UiMessage => ({ id: "m1", seq: 1, conversation_id: "c", session_id: "s", sender_id: "them", kind: "text", body: "hi", created_at: "2026-11-05T10:00:00Z", ...extra });
const answer = (liked_by: string[]): MessageLikes => ({ message_id: "m1", conversation_id: "c", seq: 1, liked_by });

beforeEach(() => {
  like.mockReset();
  unlike.mockReset();
});
afterEach(cleanup);

function setup(messages: UiMessage[] = [msg()]) {
  const applyLikes = vi.fn();
  const onFailed = vi.fn();
  const hook = renderHook((props: { messages: UiMessage[] }) => useMessageLikes({ meId: ME, messages: props.messages, applyLikes, onFailed }), { initialProps: { messages } });
  return { ...hook, applyLikes, onFailed };
}

describe("useMessageLikes (§23.3)", () => {
  it("likes optimistically, then the API's answer is applied and the override goes away", async () => {
    let finish!: (l: MessageLikes) => void;
    like.mockReturnValue(new Promise((r) => (finish = r)));
    const { result, applyLikes } = setup();
    act(() => result.current.toggle(msg()));
    expect(like).toHaveBeenCalledWith("m1");
    expect(result.current.override).toEqual({ m1: [ME] });
    await act(async () => finish(answer([ME, "them"])));
    expect(applyLikes).toHaveBeenCalledWith(answer([ME, "them"]));
    expect(result.current.override).toEqual({});
  });

  it("unlikes when the message is liked by the viewer, keeping the other person's like", async () => {
    unlike.mockResolvedValue(answer(["them"]));
    const liked = msg({ liked_by: [ME, "them"] });
    const { result } = setup([liked]);
    act(() => result.current.toggle(liked));
    expect(unlike).toHaveBeenCalledWith("m1");
    expect(result.current.override).toEqual({ m1: ["them"] });
    await waitFor(() => expect(result.current.override).toEqual({}));
  });

  it("rolls back and reports a failure with a way to try again", async () => {
    like.mockRejectedValueOnce(new ApiError({ code: "network_error", message: "offline", status: 0 })).mockResolvedValue(answer([ME]));
    const { result, onFailed, applyLikes } = setup();
    act(() => result.current.toggle(msg()));
    expect(result.current.override).toEqual({ m1: [ME] });
    await waitFor(() => expect(onFailed).toHaveBeenCalledTimes(1));
    expect(result.current.override).toEqual({});
    expect(applyLikes).not.toHaveBeenCalled();
    expect(onFailed.mock.calls[0][0].code).toBe("network_error");
    // "Try again" repeats the like.
    await act(async () => onFailed.mock.calls[0][1]());
    await waitFor(() => expect(applyLikes).toHaveBeenCalledWith(answer([ME])));
    expect(like).toHaveBeenCalledTimes(2);
  });

  it("ignores a second toggle while the first request runs", () => {
    like.mockReturnValue(new Promise(() => undefined));
    const { result } = setup();
    act(() => result.current.toggle(msg()));
    act(() => result.current.toggle(msg()));
    expect(like).toHaveBeenCalledTimes(1);
    expect(unlike).not.toHaveBeenCalled();
  });

  it("never acts on pending messages or system notices", () => {
    const { result } = setup();
    act(() => result.current.toggle(msg({ pending: "sending" })));
    act(() => result.current.toggle(msg({ kind: "system" })));
    expect(like).not.toHaveBeenCalled();
    expect(result.current.override).toEqual({});
  });

  it("reads the newest likes at retry time, not the ones of the render that failed", async () => {
    like.mockRejectedValueOnce(new ApiError({ code: "network_error", message: "x", status: 0 }));
    const { result, rerender, onFailed } = setup();
    act(() => result.current.toggle(msg()));
    await waitFor(() => expect(onFailed).toHaveBeenCalled());
    // Meanwhile the viewer's like arrived from another device.
    rerender({ messages: [msg({ liked_by: [ME] })] });
    unlike.mockResolvedValue(answer([]));
    await act(async () => onFailed.mock.calls[0][1]());
    expect(unlike).toHaveBeenCalledWith("m1");
  });
});

describe("withLikeOverrides", () => {
  it("lays overrides over the messages without touching the others", () => {
    const list = [msg(), msg({ id: "m2", seq: 2, liked_by: ["them"] })];
    const out = withLikeOverrides(list, { m1: [ME] });
    expect(out[0].liked_by).toEqual([ME]);
    expect(out[1]).toBe(list[1]);
    expect(withLikeOverrides(list, {})).toBe(list);
  });
});
