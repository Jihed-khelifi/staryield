// @vitest-environment jsdom
import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mine = vi.hoisted(() => vi.fn());
const track = vi.hoisted(() => vi.fn());
const realtime = vi.hoisted(() => ({ handler: null as null | ((type: string, data: unknown) => void) }));
vi.mock("@/lib/api/referrals", () => ({ referralsApi: { mine } }));
vi.mock("@/lib/analytics", () => ({ track }));
vi.mock("@/lib/auth/session-provider", () => ({
  useRealtimeEvents: (_types: readonly string[], handler: (type: string, data: unknown) => void) => {
    realtime.handler = handler;
  },
}));

import { ApiError } from "@/lib/api/errors";
import { REFRESH_COALESCE_MS, ReferralProvider, useReferrals } from "@/lib/referrals/referral-provider";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { myReferralsFixture } from "@/test/referral-fixtures";
import { renderWithI18n } from "@/test/render";

function Probe() {
  const r = useReferrals();
  return (
    <div>
      <p data-testid="state">{r.data ? `ready:${r.data.stats.joined}` : r.error ? `error:${r.error.code}` : r.loading ? "loading" : "idle"}</p>
      <p data-testid="copied">{String(r.copied)}</p>
      <button onClick={() => void r.copyLink()}>copy</button>
      <button onClick={() => void r.shareLink()}>share</button>
      <button onClick={r.reload}>reload</button>
    </div>
  );
}

function renderProvider(enabled = true) {
  return renderWithI18n(
    <ToastProvider>
      <ReferralProvider enabled={enabled}>
        <Probe />
      </ReferralProvider>
    </ToastProvider>,
  );
}

const writeText = vi.fn(async () => {});

beforeEach(() => {
  mine.mockReset();
  track.mockReset();
  writeText.mockReset();
  writeText.mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  Object.defineProperty(navigator, "share", { value: undefined, configurable: true });
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("ReferralProvider", () => {
  it("loads the invite once and exposes it", async () => {
    mine.mockResolvedValue(myReferralsFixture({ stats: { joined: 2 } }));
    renderProvider();
    expect(screen.getByTestId("state").textContent).toBe("loading");
    await waitFor(() => expect(screen.getByTestId("state").textContent).toBe("ready:2"));
    expect(mine).toHaveBeenCalledTimes(1);
  });

  it("asks nothing when disabled (advisors)", () => {
    renderProvider(false);
    expect(screen.getByTestId("state").textContent).toBe("idle");
    expect(mine).not.toHaveBeenCalled();
  });

  it("reports an error and retries", async () => {
    mine.mockRejectedValueOnce(new ApiError({ code: "internal_error", message: "boom", status: 500 }));
    mine.mockResolvedValueOnce(myReferralsFixture({ stats: { joined: 1 } }));
    renderProvider();
    await waitFor(() => expect(screen.getByTestId("state").textContent).toBe("error:internal_error"));
    fireEvent.click(screen.getByText("reload"));
    await waitFor(() => expect(screen.getByTestId("state").textContent).toBe("ready:1"));
  });

  it("copies the link, shows Copied for a moment and records the copy", async () => {
    mine.mockResolvedValue(myReferralsFixture());
    renderProvider();
    await waitFor(() => expect(screen.getByTestId("state").textContent).toBe("ready:0"));
    vi.useFakeTimers();
    await act(async () => {
      fireEvent.click(screen.getByText("copy"));
    });
    expect(writeText).toHaveBeenCalledWith("https://staryield.net/en?ref=ABCD2345");
    expect(screen.getByTestId("copied").textContent).toBe("true");
    expect(track).toHaveBeenCalledWith("referral_link_copied");
    await act(async () => {
      vi.advanceTimersByTime(3000);
    });
    expect(screen.getByTestId("copied").textContent).toBe("false");
  });

  it("explains a refused copy with a toast", async () => {
    mine.mockResolvedValue(myReferralsFixture());
    writeText.mockRejectedValue(new Error("NotAllowedError"));
    Object.defineProperty(document, "execCommand", { value: () => false, configurable: true });
    renderProvider();
    await waitFor(() => expect(screen.getByTestId("state").textContent).toBe("ready:0"));
    await act(async () => {
      fireEvent.click(screen.getByText("copy"));
    });
    expect(await screen.findByText("Couldn't copy the link. Select it and copy it yourself.")).toBeTruthy();
    expect(track).not.toHaveBeenCalled();
  });

  it("shares with the native sheet when there is one, else copies, recording the method only", async () => {
    mine.mockResolvedValue(myReferralsFixture());
    const share = vi.fn(async () => {});
    Object.defineProperty(navigator, "share", { value: share, configurable: true });
    renderProvider();
    await waitFor(() => expect(screen.getByTestId("state").textContent).toBe("ready:0"));
    await act(async () => {
      fireEvent.click(screen.getByText("share"));
    });
    expect(share).toHaveBeenCalledWith(expect.objectContaining({ url: "https://staryield.net/en?ref=ABCD2345", title: "Join me on Staryield" }));
    expect(track).toHaveBeenCalledWith("referral_link_shared", { method: "web_share" });
    cleanup();

    Object.defineProperty(navigator, "share", { value: undefined, configurable: true });
    renderProvider();
    await waitFor(() => expect(screen.getByTestId("state").textContent).toBe("ready:0"));
    await act(async () => {
      fireEvent.click(screen.getByText("share"));
    });
    expect(writeText).toHaveBeenCalled();
    expect(track).toHaveBeenCalledWith("referral_link_shared", { method: "copy" });
  });

  it("refreshes on referral notifications and on wallet updates while minutes are pending, coalesced", async () => {
    mine.mockResolvedValue(myReferralsFixture({ stats: { joined: 1, qualified: 1, pending_minutes: 5 } }));
    renderProvider();
    await waitFor(() => expect(screen.getByTestId("state").textContent).toBe("ready:1"));
    vi.useFakeTimers();
    mine.mockResolvedValue(myReferralsFixture({ stats: { joined: 2, qualified: 1, pending_minutes: 5 } }));
    await act(async () => {
      realtime.handler!("notification.created", { notification: { kind: "chat.missed" } });
      vi.advanceTimersByTime(REFRESH_COALESCE_MS);
    });
    expect(mine).toHaveBeenCalledTimes(1); // not a referral kind
    await act(async () => {
      realtime.handler!("notification.created", { notification: { kind: "referral.joined" } });
      realtime.handler!("wallet.updated", { balance: 10 });
      vi.advanceTimersByTime(REFRESH_COALESCE_MS);
    });
    expect(mine).toHaveBeenCalledTimes(2); // two events, one request
    vi.useRealTimers();
    await waitFor(() => expect(screen.getByTestId("state").textContent).toBe("ready:2"));
  });

  it("ignores wallet updates when nothing is pending", async () => {
    mine.mockResolvedValue(myReferralsFixture());
    renderProvider();
    await waitFor(() => expect(screen.getByTestId("state").textContent).toBe("ready:0"));
    vi.useFakeTimers();
    await act(async () => {
      realtime.handler!("wallet.updated", { balance: 10 });
      vi.advanceTimersByTime(REFRESH_COALESCE_MS * 2);
    });
    expect(mine).toHaveBeenCalledTimes(1);
  });
});
