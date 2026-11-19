// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import { ApiError } from "@/lib/api/errors";
import type { ChatHold } from "@/lib/api/holds";
import type { ChatMessage, Wallet } from "@/lib/api/types";

const api = vi.hoisted(() => ({ get: vi.fn(), reveal: vi.fn(), continueReading: vi.fn() }));
const toast = vi.hoisted(() => ({ show: vi.fn(), showError: vi.fn() }));
vi.mock("@/lib/api/holds", () => ({ holdsApi: api }));
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => toast }));

import { CONTINUE_OFFER_DELAY_MS, useHoldActions } from "@/components/chat/use-hold-actions";

const dictionary = { ...en, copy } as Dictionary;
const wrapper = ({ children }: { children: React.ReactNode }) => (
  <I18nProvider locale="en" dictionary={dictionary}>
    {children}
  </I18nProvider>
);

const hold = (over: Partial<ChatHold> = {}): ChatHold => ({
  id: "h1",
  session_id: "s1",
  conversation_id: "c1",
  reason: "trial_ended",
  status: "expired",
  started_at: "2026-11-19T10:00:00Z",
  expires_at: "2026-11-19T10:01:00Z",
  price_credits: 40,
  held_messages: 3,
  ...over,
});
const wallet = (balance: number): Wallet => ({ balance, held: 0, free_minutes: 0 });
const revealed: ChatMessage = { id: "m1", conversation_id: "c1", kind: "text", body: "Hello", created_at: "2026-11-19T10:00:00Z" };

type Props = Parameters<typeof useHoldActions>[0];
function setup(over: Partial<Props> = {}) {
  const calls = {
    openTopUp: vi.fn(),
    applyHold: vi.fn(),
    setWallet: vi.fn(),
    onRevealed: vi.fn(),
    onContinued: vi.fn(),
    onStale: vi.fn(),
    holds: new Map<string, ChatHold>(),
  };
  const initial: Props = {
    role: "client",
    hold: null,
    sessionId: undefined,
    wallet: wallet(100),
    holdById: (id) => calls.holds.get(id),
    applyHold: (h) => {
      calls.holds.set(h.id, h);
      calls.applyHold(h);
    },
    setWallet: calls.setWallet,
    openTopUp: calls.openTopUp,
    topUpOpen: false,
    onRevealed: calls.onRevealed,
    onContinued: calls.onContinued,
    onStale: calls.onStale,
    ...over,
  };
  const view = renderHook((p: Props) => useHoldActions(p), { wrapper, initialProps: initial });
  return { ...view, calls, props: initial };
}

beforeEach(() => {
  api.get.mockReset();
  api.reveal.mockReset();
  api.continueReading.mockReset();
  toast.show.mockReset();
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("locked message while the hold is open", () => {
  it("opens the top-up dialog", async () => {
    const { result, calls } = setup({ holdById: () => hold({ status: "open" }) });
    act(() => result.current.openLocked("h1"));
    await waitFor(() => expect(calls.openTopUp).toHaveBeenCalledTimes(1));
    expect(result.current.confirm).toBeNull();
    expect(api.reveal).not.toHaveBeenCalled();
  });

  it("does nothing for an advisor or without a hold id", () => {
    const advisor = setup({ role: "psychic", holdById: () => hold({ status: "open" }) });
    act(() => advisor.result.current.openLocked("h1"));
    const client = setup();
    act(() => client.result.current.openLocked(undefined));
    expect(advisor.calls.openTopUp).not.toHaveBeenCalled();
    expect(client.calls.openTopUp).not.toHaveBeenCalled();
  });
});

describe("reveal after expiry", () => {
  it("balance >= price: confirm first, then reveal once and apply the messages", async () => {
    api.reveal.mockResolvedValue({ hold: hold({ status: "revealed" }), messages: [revealed], wallet: wallet(60) });
    const { result, calls } = setup({ holdById: () => hold() });
    act(() => result.current.openLocked("h1"));
    await waitFor(() => expect(result.current.confirm).toEqual({ holdId: "h1", price: 40, count: 3, error: null }));
    expect(api.reveal).not.toHaveBeenCalled(); // nothing is charged by opening the dialog

    await act(async () => result.current.confirmReveal());
    expect(api.reveal).toHaveBeenCalledTimes(1);
    expect(api.reveal).toHaveBeenCalledWith("h1", expect.any(String));
    expect(calls.onRevealed).toHaveBeenCalledWith(expect.objectContaining({ messages: [revealed] }));
    expect(calls.setWallet).toHaveBeenCalledWith(wallet(60));
    expect(calls.applyHold).toHaveBeenCalledWith(expect.objectContaining({ status: "revealed" }));
    expect(result.current.confirm).toBeNull();
  });

  it("cancel closes the dialog without a request", async () => {
    const { result } = setup({ holdById: () => hold() });
    act(() => result.current.openLocked("h1"));
    await waitFor(() => expect(result.current.confirm).not.toBeNull());
    act(() => result.current.cancelReveal());
    expect(result.current.confirm).toBeNull();
    expect(api.reveal).not.toHaveBeenCalled();
  });

  it("fetches the hold when only the server knows it (after a reload), then confirms", async () => {
    api.get.mockResolvedValue(hold({ id: "h9", held_messages: 1 }));
    const { result, calls } = setup();
    act(() => result.current.openLocked("h9"));
    await waitFor(() => expect(result.current.confirm).toEqual({ holdId: "h9", price: 40, count: 1, error: null }));
    expect(api.get).toHaveBeenCalledWith("h9");
    expect(calls.applyHold).toHaveBeenCalled();
  });

  it("balance < price: opens the top-up, then confirms by itself once the wallet covers the price", async () => {
    const { result, calls, rerender, props } = setup({ wallet: wallet(10), holdById: () => hold() });
    act(() => result.current.openLocked("h1"));
    await waitFor(() => expect(calls.openTopUp).toHaveBeenCalledTimes(1));
    expect(result.current.confirm).toBeNull();

    // The dialog is open, the purchase succeeds: the wallet now covers the price.
    rerender({ ...props, wallet: wallet(10), topUpOpen: true });
    rerender({ ...props, wallet: wallet(50), topUpOpen: false });
    await waitFor(() => expect(result.current.confirm).toEqual({ holdId: "h1", price: 40, count: 3, error: null }));
  });

  it("closing the top-up without paying ends the wait: no confirm appears later", async () => {
    vi.useFakeTimers();
    const { result, calls, rerender, props } = setup({ wallet: wallet(10), holdById: () => hold() });
    await act(async () => result.current.openLocked("h1"));
    expect(calls.openTopUp).toHaveBeenCalledTimes(1);
    rerender({ ...props, wallet: wallet(10), topUpOpen: true });
    rerender({ ...props, wallet: wallet(10), topUpOpen: false });
    await act(async () => {
      vi.advanceTimersByTime(3000);
    });
    // Money arrives later from somewhere else: the reveal is not asked.
    rerender({ ...props, wallet: wallet(500), topUpOpen: false });
    expect(result.current.confirm).toBeNull();
  });

  it("402 insufficient_credits at the server opens the top-up", async () => {
    api.reveal.mockRejectedValue(new ApiError({ code: "insufficient_credits", message: "no", status: 402, details: { required: 40, balance: 5 } }));
    const { result, calls, rerender, props } = setup({ holdById: () => hold() });
    act(() => result.current.openLocked("h1"));
    await waitFor(() => expect(result.current.confirm).not.toBeNull());
    await act(async () => result.current.confirmReveal());
    expect(calls.openTopUp).toHaveBeenCalledTimes(1);
    expect(calls.onRevealed).not.toHaveBeenCalled();
    // The server's balance replaces the wallet the page believed in (it shows the short wallet, not a confirm loop).
    expect(calls.setWallet).toHaveBeenCalledWith({ balance: 5, held: 0, free_minutes: 0 });
    rerender({ ...props, wallet: wallet(5) });
    expect(result.current.confirm).toBeNull();
  });

  it("a network error keeps the dialog with the reason; retrying repeats the same key and succeeds", async () => {
    api.reveal
      .mockRejectedValueOnce(new ApiError({ code: "network_error", message: "offline" }))
      .mockResolvedValueOnce({ hold: hold({ status: "revealed" }), messages: [revealed], wallet: wallet(60) });
    const { result, calls } = setup({ holdById: () => hold() });
    act(() => result.current.openLocked("h1"));
    await waitFor(() => expect(result.current.confirm).not.toBeNull());
    await act(async () => result.current.confirmReveal());
    expect(result.current.confirm?.error).toBeTruthy();
    expect(calls.onRevealed).not.toHaveBeenCalled();

    await act(async () => result.current.confirmReveal());
    expect(api.reveal).toHaveBeenCalledTimes(2);
    expect(api.reveal.mock.calls[1][1]).toBe(api.reveal.mock.calls[0][1]);
    expect(calls.onRevealed).toHaveBeenCalledTimes(1);
    expect(result.current.confirm).toBeNull();
  });

  it("409 invalid_state refreshes the hold: an open one goes to the top-up", async () => {
    api.reveal.mockRejectedValue(new ApiError({ code: "invalid_state", message: "open", status: 409 }));
    api.get.mockResolvedValue(hold({ status: "open" }));
    const { result, calls } = setup({ holdById: () => hold() });
    act(() => result.current.openLocked("h1"));
    await waitFor(() => expect(result.current.confirm).not.toBeNull());
    await act(async () => result.current.confirmReveal());
    await waitFor(() => expect(calls.openTopUp).toHaveBeenCalledTimes(1));
    expect(api.get).toHaveBeenCalledWith("h1");
    expect(result.current.confirm).toBeNull();
  });

  it("409 invalid_state with a settled hold tells the client and re-reads the reading", async () => {
    api.reveal.mockRejectedValue(new ApiError({ code: "invalid_state", message: "x", status: 409 }));
    api.get.mockResolvedValue(hold({ status: "converted" }));
    const { result, calls } = setup({ holdById: () => hold() });
    act(() => result.current.openLocked("h1"));
    await waitFor(() => expect(result.current.confirm).not.toBeNull());
    await act(async () => result.current.confirmReveal());
    await waitFor(() => expect(calls.onStale).toHaveBeenCalledWith("s1"));
    expect(toast.show).toHaveBeenCalledWith(expect.objectContaining({ tone: "info" }));
  });

  it("an already revealed hold asks the server for the messages without a confirm", async () => {
    api.reveal.mockResolvedValue({ hold: hold({ status: "revealed" }), messages: [revealed], wallet: wallet(100) });
    const { result, calls } = setup({ holdById: () => hold({ status: "revealed" }) });
    act(() => result.current.openLocked("h1"));
    await waitFor(() => expect(calls.onRevealed).toHaveBeenCalledTimes(1));
    expect(result.current.confirm).toBeNull();
  });

  it("a converted hold only refreshes (nothing is hidden any more)", async () => {
    const { result, calls } = setup({ holdById: () => hold({ status: "converted" }) });
    act(() => result.current.openLocked("h1"));
    await waitFor(() => expect(calls.onStale).toHaveBeenCalledWith("s1"));
    expect(api.reveal).not.toHaveBeenCalled();
  });

  it("a failed hold lookup is explained with a retry", async () => {
    api.get.mockRejectedValue(new ApiError({ code: "network_error", message: "offline" }));
    const { result } = setup();
    act(() => result.current.openLocked("h9"));
    await waitFor(() => expect(toast.show).toHaveBeenCalled());
    expect(toast.show.mock.calls[0][0]).toMatchObject({ tone: "error", actionLabel: "Try again" });
    expect(result.current.confirm).toBeNull();
  });
});

describe("continue reading (fallback)", () => {
  const open = hold({ status: "open" });

  it("is offered after 5 s once the wallet covers the price and the hold is still open", async () => {
    vi.useFakeTimers();
    const { result } = setup({ hold: open, sessionId: "s1", wallet: wallet(100) });
    expect(result.current.continueOffered).toBe(false);
    await act(async () => {
      vi.advanceTimersByTime(CONTINUE_OFFER_DELAY_MS - 100);
    });
    expect(result.current.continueOffered).toBe(false);
    await act(async () => {
      vi.advanceTimersByTime(200);
    });
    expect(result.current.continueOffered).toBe(true);
  });

  it("is never offered while the wallet is short, for an advisor or after the hold ended", async () => {
    vi.useFakeTimers();
    const short = setup({ hold: open, sessionId: "s1", wallet: wallet(10) });
    const advisor = setup({ hold: open, sessionId: "s1", wallet: wallet(100), role: "psychic" });
    const ended = setup({ hold: hold({ status: "converted" }), sessionId: "s1", wallet: wallet(100) });
    await act(async () => {
      vi.advanceTimersByTime(CONTINUE_OFFER_DELAY_MS * 2);
    });
    expect(short.result.current.continueOffered).toBe(false);
    expect(advisor.result.current.continueOffered).toBe(false);
    expect(ended.result.current.continueOffered).toBe(false);
  });

  it("withdraws the offer when the hold converts by itself", async () => {
    vi.useFakeTimers();
    const { result, rerender, props } = setup({ hold: open, sessionId: "s1", wallet: wallet(100) });
    await act(async () => {
      vi.advanceTimersByTime(CONTINUE_OFFER_DELAY_MS + 100);
    });
    expect(result.current.continueOffered).toBe(true);
    rerender({ ...props, hold: hold({ status: "converted" }) });
    expect(result.current.continueOffered).toBe(false);
  });

  it("calls the endpoint, applies the answer and keeps the key stable across a retry", async () => {
    api.continueReading
      .mockRejectedValueOnce(new ApiError({ code: "network_error", message: "offline" }))
      .mockResolvedValueOnce({ session: { id: "s1" }, hold: hold({ status: "converted" }), wallet: wallet(60) });
    const { result, calls } = setup({ hold: open, sessionId: "s1" });
    await act(async () => result.current.continueReading());
    expect(toast.show).toHaveBeenCalledWith(expect.objectContaining({ tone: "error", actionLabel: "Try again" }));
    await act(async () => result.current.continueReading());
    expect(api.continueReading.mock.calls[1][1]).toBe(api.continueReading.mock.calls[0][1]);
    expect(calls.onContinued).toHaveBeenCalledWith(expect.objectContaining({ session: { id: "s1" } }));
    expect(calls.setWallet).toHaveBeenCalledWith(wallet(60));
    expect(calls.applyHold).toHaveBeenCalledWith(expect.objectContaining({ status: "converted" }));
  });

  it("402 opens the top-up and 409 refreshes the reading", async () => {
    api.continueReading.mockRejectedValueOnce(new ApiError({ code: "insufficient_credits", message: "no", status: 402 }));
    const a = setup({ hold: open, sessionId: "s1" });
    await act(async () => a.result.current.continueReading());
    expect(a.calls.openTopUp).toHaveBeenCalledTimes(1);

    api.continueReading.mockRejectedValueOnce(new ApiError({ code: "invalid_state", message: "x", status: 409 }));
    const b = setup({ hold: open, sessionId: "s1" });
    await act(async () => b.result.current.continueReading());
    expect(b.calls.onStale).toHaveBeenCalledWith("s1");
  });

  it("does nothing without a held session", async () => {
    const { result } = setup({ hold: open, sessionId: undefined });
    await act(async () => result.current.continueReading());
    expect(api.continueReading).not.toHaveBeenCalled();
  });
});
