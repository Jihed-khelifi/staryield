// @vitest-environment jsdom
import type * as React from "react";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
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

import enDict from "@/i18n/dictionaries/en.json";
import esDict from "@/i18n/dictionaries/es.json";
import frDict from "@/i18n/dictionaries/fr.json";
import copyEn from "@/i18n/dictionaries/copy-en.json";
import copyEs from "@/i18n/dictionaries/copy-es.json";
import copyFr from "@/i18n/dictionaries/copy-fr.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import { TooltipProvider } from "@/components/ui/tooltip";
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

function config(referrals: { enabled: boolean; reward_minutes?: number } | null, signupMinutes = 5) {
  return siteConfigFixture({
    signup_offer: { enabled: signupMinutes > 0, free_minutes: signupMinutes },
    ...(referrals ? { referrals: { reward_minutes: 5, ...referrals } } : {}),
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
    expect(within(card).getByText(/you get 5 free minutes\./)).toBeTruthy();
    expect(within(card).getByTestId("referral-progress").textContent).toBe("2 friends joined · 1 completed a reading · 5 minutes on their way");
    expect(within(card).getByRole("link", { name: "How it works" }).getAttribute("href")).toBe("/en/profile/invite");
    expect(within(card).queryByText(/beta/i)).toBeNull();
    expect(await axeViolations(card)).toEqual([]);
  });

  it("has no \"Beta invite\" line above its title", async () => {
    mine.mockResolvedValue(data());
    renderWith(<ReferralCard />);
    const card = await screen.findByTestId("referral-card");
    await waitFor(() => expect(within(card).getByTestId("referral-link")).toBeTruthy());
    expect(within(card).queryByText("Beta invite")).toBeNull();
    expect(card.textContent).not.toMatch(/beta|launch/i);
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
  it("explains the program with counters and rules, and shows no notice while it runs", async () => {
    mine.mockResolvedValue(data({ stats: { joined: 3, qualified: 2, pending_minutes: 10, granted_minutes: 5, remaining_rewards: 18 } }));
    renderWith(<InvitePage />);
    await screen.findByTestId("referral-stats");
    expect(screen.queryByTestId("referral-program-notice")).toBeNull();
    expect(screen.getByTestId("stat-joined").textContent).toContain("3");
    expect(screen.getByTestId("stat-granted").textContent).toContain("5");
    expect(screen.getByTestId("stat-granted").textContent).toContain("Minutes added");
    expect(screen.queryByTestId("stat-pending")).toBeNull();
    expect(screen.getByText("10 more minutes are on their way to your wallet.")).toBeTruthy();
    expect(screen.getByTestId("stat-remaining").textContent).toContain("18 of 20");
    // The rules: the friend gets the standard offer only, and nobody is named.
    fireEvent.click(screen.getByRole("button", { name: "What does my friend get?" }));
    expect(await screen.findByText(/their first 5 minutes of live chat are free\. There is no extra reward for being invited\./)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Can free minutes be exchanged for money?" }));
    expect(await screen.findByText(/no cash value/)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "When do I get my free minutes?" }));
    expect(
      await screen.findByText(
        "As soon as your friend has verified their e-mail address and completed a reading. The minutes are usually in your wallet within a minute, and we let you know.",
      ),
    ).toBeTruthy();
    expect(screen.getByRole("link", { name: "Terms of Use" }).getAttribute("href")).toBe("/en/terms");
    expect(await axeViolations(screen.getByTestId("invite-page"))).toEqual([]);
  });

  it("shows the minutes added and the cap, and no on-their-way line when nothing is pending", async () => {
    mine.mockResolvedValue(data({ program: { max_rewards: 2 }, stats: { joined: 2, qualified: 2, granted_minutes: 10, remaining_rewards: 0 } }));
    renderWith(<InvitePage />);
    await screen.findByTestId("referral-stats");
    expect(screen.queryByTestId("referral-program-notice")).toBeNull();
    expect(screen.getByTestId("stat-granted").textContent).toContain("10");
    expect(screen.getByTestId("referral-cap-reached")).toBeTruthy();
    expect(screen.queryByText(/on their way to your wallet/)).toBeNull();
  });

  it("says 1 more minute is on its way in the singular", async () => {
    mine.mockResolvedValue(data({ stats: { joined: 1, qualified: 1, pending_minutes: 1, remaining_rewards: 19 } }));
    renderWith(<InvitePage />);
    await screen.findByTestId("referral-stats");
    expect(screen.getByText("1 more minute is on its way to your wallet.")).toBeTruthy();
  });

  it("hides the link while invitations are paused but keeps the counters", async () => {
    mine.mockResolvedValue(data({ program: { enabled: false }, stats: { joined: 1, qualified: 1, pending_minutes: 5 } }));
    renderWith(<InvitePage />, config({ enabled: false }));
    await screen.findByTestId("referral-stats");
    expect(screen.getByTestId("referral-program-notice").getAttribute("data-state")).toBe("paused");
    expect(screen.getByTestId("referral-program-notice").textContent).toContain("Invitations are paused");
    expect(screen.getByTestId("stat-granted")).toBeTruthy();
    expect(screen.queryByTestId("referral-link")).toBeNull();
    // The hero promises nothing while paused (new sign-ups and readings earn nothing).
    expect(screen.getByText("Rewards for new invitations are paused for now.")).toBeTruthy();
    expect(screen.queryByText(/For every friend who joins/)).toBeNull();
  });

  it("offers a retry when the invite cannot be loaded", async () => {
    mine.mockRejectedValue(new ApiError({ code: "internal_error", message: "boom", status: 500 }));
    renderWith(<InvitePage />);
    expect(await screen.findByText("Your invite link could not be loaded")).toBeTruthy();
  });
});

describe("ReferralPrompt (finished reading)", () => {
  it("invites the client to share after a reading", async () => {
    mine.mockResolvedValue(data());
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

const dictionaries = { en: enDict, fr: frDict, es: esDict };
const copies = { en: copyEn, fr: copyFr, es: copyEs };

describe("the invite surfaces never mention a beta or a launch", () => {
  const OLD_WORDING = /\bbeta\b|bêta|launch|lancement|lanzamiento/i;

  it.each(["en", "fr", "es"] as const)("%s: the card, the page (running and paused) and the prompt", async (locale) => {
    const dict = { ...dictionaries[locale], copy: copies[locale] } as Dictionary;
    const view = (ui: React.ReactElement) =>
      render(ui, {
        wrapper: ({ children }) => (
          <I18nProvider locale={locale} dictionary={dict}>
            <TooltipProvider>{children}</TooltipProvider>
          </I18nProvider>
        ),
      });
    const pages: [string, MyReferrals, ReturnType<typeof config>][] = [
      ["running", data({ stats: { joined: 3, qualified: 3, pending_minutes: 5, granted_minutes: 10, remaining_rewards: 17 } }), config({ enabled: true })],
      ["paused", data({ program: { enabled: false } }), config({ enabled: false })],
    ];
    for (const [label, payload, cfg] of pages) {
      mine.mockResolvedValue(payload);
      const { container, unmount } = view(
        <SiteConfigProvider initial={cfg}>
          <ToastProvider>
            <ReferralProvider>
              <InvitePage />
              <ReferralCard />
            </ReferralProvider>
          </ToastProvider>
        </SiteConfigProvider>,
      );
      await screen.findByTestId("referral-stats");
      for (const trigger of container.querySelectorAll("[data-testid=referral-faq] button")) fireEvent.click(trigger);
      expect(container.textContent, `${locale} ${label}`).not.toMatch(OLD_WORDING);
      unmount();
    }
  });
});
