// @vitest-environment jsdom
import type * as React from "react";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mine = vi.hoisted(() => vi.fn());
const session = vi.hoisted(() => ({ status: "anonymous" as string }));
vi.mock("@/lib/api/referrals", () => ({ referralsApi: { mine } }));
vi.mock("@/lib/analytics", () => ({ track: vi.fn() }));
vi.mock("@/lib/api/endpoints", () => ({ siteApi: { config: vi.fn(() => new Promise(() => {})) } }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: session.status }), useRealtimeEvents: () => {} }));

import { ApiError } from "@/lib/api/errors";
import type { MyReferrals } from "@/lib/api/referrals";
import { REFERRAL_KEY } from "@/lib/referral";
import { ReferralProvider } from "@/lib/referrals/referral-provider";
import { SiteConfigProvider } from "@/lib/site-config/site-config-provider";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { InvitePage } from "@/components/referrals/invite-page";
import { ReferralCard } from "@/components/referrals/referral-card";
import { ReferralPrompt } from "@/components/referrals/referral-prompt";
import { ReferralWelcome } from "@/components/referrals/referral-welcome";
import { myReferralsFixture } from "@/test/referral-fixtures";
import { axeViolations, renderWithI18n } from "@/test/render";
import { siteConfigFixture } from "@/test/site-config";

function config(referrals: { enabled: boolean; reward_minutes?: number; launched?: boolean } | null, signupMinutes = 5) {
  return siteConfigFixture({
    signup_offer: { enabled: signupMinutes > 0, free_minutes: signupMinutes },
    ...(referrals ? { referrals: { reward_minutes: 5, launched: false, ...referrals } } : {}),
  });
}

function renderWith(ui: React.ReactElement, cfg = config({ enabled: true }), provider = true) {
  return renderWithI18n(
    <SiteConfigProvider initial={cfg}>
      <ToastProvider>{provider ? <ReferralProvider>{ui}</ReferralProvider> : ui}</ToastProvider>
    </SiteConfigProvider>,
  );
}

const data = (over: Parameters<typeof myReferralsFixture>[0] = {}): MyReferrals => myReferralsFixture(over);

beforeEach(() => {
  mine.mockReset();
  localStorage.clear();
  sessionStorage.clear();
  session.status = "anonymous";
});
afterEach(cleanup);

describe("ReferralCard (overview)", () => {
  it("shows the reward, the link and the progress", async () => {
    mine.mockResolvedValue(data({ stats: { joined: 2, qualified: 1, pending_minutes: 5, remaining_rewards: 19 } }));
    renderWith(<ReferralCard />);
    const card = await screen.findByTestId("referral-card");
    await waitFor(() => expect(within(card).getByTestId("referral-link")).toBeTruthy());
    expect((within(card).getByTestId("referral-link") as HTMLInputElement).value).toBe("https://staryield.net/en?ref=ABCD2345");
    expect(within(card).getByText(/you get 5 free minutes, added when Staryield launches\./)).toBeTruthy();
    expect(within(card).getByTestId("referral-progress").textContent).toBe("2 friends joined · 1 completed a reading · 5 minutes pending until launch");
    expect(within(card).getByRole("link", { name: "How it works" }).getAttribute("href")).toBe("/en/profile/invite");
    expect(await axeViolations(card)).toEqual([]);
  });

  it("holds a place while loading, then offers a retry on failure", async () => {
    mine.mockRejectedValueOnce(new ApiError({ code: "network_error", message: "offline", status: 0 }));
    mine.mockResolvedValueOnce(data());
    renderWith(<ReferralCard />);
    expect(screen.getByText("Loading your invite link…")).toBeTruthy();
    fireEvent.click(await screen.findByRole("button", { name: "Try again" }));
    await waitFor(() => expect(screen.getByTestId("referral-progress").textContent).toBe("No friend has joined with your link yet."));
  });

  it("is hidden while the program is off", async () => {
    mine.mockResolvedValue(data({ program: { enabled: false } }));
    renderWith(<ReferralCard />, config({ enabled: false }));
    await waitFor(() => expect(mine).toHaveBeenCalled());
    await new Promise((r) => setTimeout(r, 0));
    expect(screen.queryByTestId("referral-card")).toBeNull();
  });
});

describe("InvitePage", () => {
  it("explains the program before the launch, with counters and rules", async () => {
    mine.mockResolvedValue(data({ stats: { joined: 3, qualified: 2, pending_minutes: 10, remaining_rewards: 18 } }));
    renderWith(<InvitePage />);
    await screen.findByTestId("referral-stats");
    expect(screen.getByTestId("referral-program-notice").getAttribute("data-state")).toBe("beta");
    expect(screen.getByTestId("stat-joined").textContent).toContain("3");
    expect(screen.getByTestId("stat-pending").textContent).toContain("10");
    expect(screen.getByTestId("stat-remaining").textContent).toContain("18 of 20");
    expect(screen.queryByTestId("stat-granted")).toBeNull();
    // The rules: the friend gets the standard offer only, and nobody is named.
    fireEvent.click(screen.getByRole("button", { name: "What does my friend get?" }));
    expect(await screen.findByText(/their first 5 minutes of live chat are free\. There is no extra reward for being invited\./)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Can free minutes be exchanged for money?" }));
    expect(await screen.findByText(/no cash value/)).toBeTruthy();
    expect(screen.getByRole("link", { name: "Terms of Use" }).getAttribute("href")).toBe("/en/terms");
    expect(await axeViolations(screen.getByTestId("invite-page"))).toEqual([]);
  });

  it("shows the minutes added after the launch and the cap", async () => {
    mine.mockResolvedValue(
      data({ program: { launched: true, launched_at: "2026-12-01T10:00:00Z", max_rewards: 2 }, stats: { joined: 2, qualified: 2, granted_minutes: 10, remaining_rewards: 0 } }),
    );
    renderWith(<InvitePage />);
    await screen.findByTestId("referral-stats");
    expect(screen.getByTestId("referral-program-notice").getAttribute("data-state")).toBe("launched");
    expect(screen.getByTestId("stat-granted").textContent).toContain("10");
    expect(screen.getByTestId("referral-cap-reached")).toBeTruthy();
    expect(screen.getByText(/December 1, 2026/)).toBeTruthy();
  });

  it("hides the link while invitations are paused but keeps the counters", async () => {
    mine.mockResolvedValue(data({ program: { enabled: false }, stats: { joined: 1, qualified: 1, pending_minutes: 5 } }));
    renderWith(<InvitePage />, config({ enabled: false }));
    await screen.findByTestId("referral-stats");
    expect(screen.getByTestId("referral-program-notice").getAttribute("data-state")).toBe("paused");
    expect(screen.queryByTestId("referral-link")).toBeNull();
  });

  it("offers a retry when the invite cannot be loaded", async () => {
    mine.mockRejectedValue(new ApiError({ code: "internal_error", message: "boom", status: 500 }));
    renderWith(<InvitePage />);
    expect(await screen.findByText("Your invite link could not be loaded")).toBeTruthy();
  });
});

describe("ReferralPrompt (finished reading)", () => {
  it("invites the client to share after a reading", async () => {
    mine.mockResolvedValue(data({ program: { launched: true } }));
    renderWith(<ReferralPrompt />, config({ enabled: true }), false);
    const prompt = await screen.findByTestId("referral-prompt");
    expect(within(prompt).getByText("Enjoyed your reading? Invite a friend")).toBeTruthy();
    await waitFor(() => expect(within(prompt).getByTestId("referral-copy")).toBeTruthy());
    expect(within(prompt).getByText(/you get 5 free minutes\./)).toBeTruthy();
  });

  it("asks nothing while the program is off", () => {
    renderWith(<ReferralPrompt />, config({ enabled: false }), false);
    expect(screen.queryByTestId("referral-prompt")).toBeNull();
    expect(mine).not.toHaveBeenCalled();
  });
});

describe("ReferralWelcome (invited visitor)", () => {
  const keep = () => localStorage.setItem(REFERRAL_KEY, JSON.stringify({ code: "ABCD2345", first_seen_at: new Date().toISOString() }));

  it("greets an invited visitor with the standard welcome offer and discloses the friend's reward", () => {
    keep();
    renderWith(<ReferralWelcome placement="home" />, config({ enabled: true }, 8), false);
    const banner = screen.getByTestId("referral-welcome");
    expect(within(banner).getByText("A friend invited you to Staryield")).toBeTruthy();
    expect(within(banner).getByText("Create your free account: like every new client, your first 8 minutes of live chat are free.")).toBeTruthy();
    expect(within(banner).getByText("Your friend earns free minutes when you complete your first reading.")).toBeTruthy();
    expect(within(banner).getByRole("link", { name: "Create my account" }).getAttribute("href")).toBe("/en/signup");
    fireEvent.click(within(banner).getByRole("button", { name: "Dismiss" }));
    expect(screen.queryByTestId("referral-welcome")).toBeNull();
    expect(sessionStorage.getItem("staryield.referral.welcome-dismissed")).toBe("1");
  });

  it("only informs on the sign-up steps, without the offer when it is off", () => {
    keep();
    renderWith(<ReferralWelcome placement="signup" />, config({ enabled: true }, 0), false);
    const banner = screen.getByTestId("referral-welcome");
    expect(within(banner).getByText("Create your free account to chat live with our psychic advisors.")).toBeTruthy();
    expect(within(banner).queryByRole("link")).toBeNull();
    expect(within(banner).queryByRole("button")).toBeNull();
  });

  it.each([
    ["no invite kept", () => {}, config({ enabled: true }), "anonymous"],
    ["the program off", keep, config({ enabled: false }), "anonymous"],
    ["an older API", keep, config(null), "anonymous"],
    ["a signed-in visitor", keep, config({ enabled: true }), "authenticated"],
  ])("stays hidden with %s", (_, setup, cfg, status) => {
    setup();
    session.status = status;
    renderWith(<ReferralWelcome placement="home" />, cfg, false);
    expect(screen.queryByTestId("referral-welcome")).toBeNull();
  });
});
