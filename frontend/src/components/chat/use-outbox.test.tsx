// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";

import type { ChatMessage } from "@/lib/api/types";

const sendToConversation = vi.fn();
vi.mock("@/lib/api/endpoints", () => ({ chatApi: { sendToConversation: (...a: unknown[]) => sendToConversation(...a) } }));

const { useOutbox } = await import("@/components/chat/use-outbox");

const CONV = "11111111-1111-4111-8111-111111111111";
const TARGET = { id: "55555555-5555-4555-8555-555555555555", seq: 4, sender_id: "peer", kind: "text" as const, excerpt: "quoted" };
const stored = (cmid: string): ChatMessage => ({ id: `srv-${cmid}`, seq: 9, client_msg_id: cmid, conversation_id: CONV, sender_id: "me", kind: "text", body: "x", created_at: "2026-11-05T10:00:00Z" });

beforeEach(() => {
  sessionStorage.clear();
  sendToConversation.mockReset();
});
afterEach(cleanup);

function setup(realtime: { request: ReturnType<typeof vi.fn> } | null) {
  const onDelivered = vi.fn();
  const hook = renderHook(() =>
    useOutbox({ userId: "me", conversationId: CONV, liveSessionId: null, realtime: realtime as never, connection: "open", onDelivered }),
  );
  return { ...hook, onDelivered };
}

describe("useOutbox sends replies (§23.2)", () => {
  it("over the socket: reply_to_id only when replying", async () => {
    const request = vi.fn(async (_type: string, payload: { client_msg_id: string }) => ({ message: stored(payload.client_msg_id) }));
    const { result } = setup({ request });
    act(() => void result.current.send("plain"));
    await waitFor(() => expect(request).toHaveBeenCalledTimes(1));
    expect(request.mock.calls[0][0]).toBe("message.send");
    expect("reply_to_id" in (request.mock.calls[0][1] as object)).toBe(false);
    act(() => void result.current.send("answer", TARGET));
    await waitFor(() => expect(request).toHaveBeenCalledTimes(2));
    expect(request.mock.calls[1][1]).toMatchObject({ conversation_id: CONV, body: "answer", reply_to_id: TARGET.id });
  });

  it("over REST (no socket): the reply id is the fourth argument", async () => {
    sendToConversation.mockImplementation(async (_c: string, _b: string, cmid: string) => ({ message: stored(cmid) }));
    const { result, onDelivered } = setup(null);
    act(() => void result.current.send("answer", TARGET));
    await waitFor(() => expect(onDelivered).toHaveBeenCalled());
    expect(sendToConversation).toHaveBeenCalledWith(CONV, "answer", expect.any(String), TARGET.id);
  });

  it("the queued entry shows the quote while it waits", () => {
    const request = vi.fn(() => new Promise<never>(() => undefined));
    const { result } = setup({ request });
    act(() => void result.current.send("answer", TARGET));
    expect(result.current.entries[0].reply_to).toEqual(TARGET);
  });
});
