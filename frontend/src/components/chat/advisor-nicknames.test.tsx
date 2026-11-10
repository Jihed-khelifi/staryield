// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";

import type { ChatMessage, ChatSession, ClientProfile, Conversation, Participant } from "@/lib/api/types";
import type { PagedList } from "@/lib/use-paged-list";
import { renderWithI18n, stubResizeObserver } from "@/test/render";
import { ToastProvider } from "@/components/feedback/toast-provider";

const h = vi.hoisted(() => ({
  session: { status: "authenticated", user: { id: "adv", role: "psychic" } } as { status: string; user: { id: string; role: string } | null },
  clientProfile: vi.fn(),
  lookup: vi.fn(),
}));
vi.mock("next/link", () => ({ default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => <a href={href} {...rest}>{children}</a> }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => h.session, useRealtimeEvent: () => undefined }));
vi.mock("@/lib/api/endpoints", () => ({
  chatApi: { clientProfile: (id: string) => h.clientProfile(id) },
  aliasApi: { lookup: (...a: unknown[]) => h.lookup(...a), set: vi.fn(), clear: vi.fn() },
  notesApi: { list: vi.fn().mockResolvedValue({ items: [] }), create: vi.fn(), update: vi.fn(), remove: vi.fn(), shared: vi.fn() },
}));
vi.mock("@/components/reviews/review-form", () => ({ ReviewForm: () => null, ReviewThanks: () => null }));
vi.mock("@/components/waitlist/waitlist-button", () => ({ WaitlistButton: () => null }));
vi.mock("@/components/layout/site-header", () => ({ MOBILE_NAV_LINKS: [], isMobileNavActive: () => false }));
vi.mock("@/components/layout/user-menu", () => ({ UserAvatar: () => null }));
vi.mock("@/components/notifications/notification-bell", () => ({ NotificationBell: () => null }));
vi.mock("@/components/psychic/header-actions", () => ({ AvailabilityMenu: () => null, WalletPill: () => null }));
vi.mock("@/components/psychic/use-own-status", () => ({ useOwnStatus: () => null }));
vi.mock("next/navigation", () => ({ usePathname: () => "/en/chatroom" }));

import { ClientAliasProvider } from "@/lib/chat/client-alias-provider";
import { ClientPanel } from "@/components/chat/client-panel";
import { clearClientProfileCache } from "@/components/chat/use-client-profile";
import { ConversationList } from "@/components/chat/conversation-list";
import { SessionPanel } from "@/components/chat/session-panel";
import { SwitcherHeader } from "@/components/chat/chat-mobile";

beforeAll(stubResizeObserver);
beforeEach(() => {
  h.session = { status: "authenticated", user: { id: "adv", role: "psychic" } };
  clearClientProfileCache();
  h.clientProfile.mockReset().mockResolvedValue({ id: "c1", name: "Paul", online: true, locale: "en", member_since: "2025-01-01T00:00:00Z", birth_details_shared: false, history: { sessions: 2, minutes: 10 }, alias: "Moon Paul" } as ClientProfile);
  h.lookup.mockReset().mockResolvedValue([{ client_id: "c1", alias: "Moon Paul", updated_at: "x" }]);
});
afterEach(cleanup);

const client: Participant = { id: "c1", role: "client", name: "Paul", status: "online" };
const withAliases = (ui: React.ReactElement) =>
  renderWithI18n(
    <ToastProvider>
      <ClientAliasProvider>{ui}</ClientAliasProvider>
    </ToastProvider>,
  );

const paged = (): PagedList<ChatMessage> => ({ items: [], page: 1, hasPrev: false, hasNext: false, loading: false, error: null, next: vi.fn(), prev: vi.fn(), reset: vi.fn(), reload: vi.fn(), replaceItem: vi.fn(), removeItem: vi.fn(), prependItem: vi.fn() });

describe("advisors see their nickname for a client everywhere (§23.4)", () => {
  it("client panel: the nickname as the name, the real name underneath", async () => {
    withAliases(<ClientPanel conversationId="conv-1" counterpart={client} saved={paged()} onRemove={vi.fn()} />);
    expect(await screen.findByRole("heading", { name: "Moon Paul" })).toBeTruthy();
    expect(screen.getByTestId("client-account-name").textContent).toBe("Name on the account: Paul");
  });

  it("client panel without a nickname shows the real name and no 'account' line", async () => {
    h.clientProfile.mockResolvedValue({ id: "c1", name: "Paul", online: true, locale: "en", member_since: "2025-01-01T00:00:00Z", birth_details_shared: false, history: { sessions: 0, minutes: 0 } });
    h.lookup.mockResolvedValue([]);
    withAliases(<ClientPanel conversationId="conv-1" counterpart={client} saved={paged()} onRemove={vi.fn()} />);
    expect(await screen.findByRole("heading", { name: "Paul" })).toBeTruthy();
    expect(screen.queryByTestId("client-account-name")).toBeNull();
  });

  const row = (over: Partial<Conversation> = {}): Conversation => ({ id: "conv-1", counterpart: { ...client }, unread: 0, created_at: "2026-11-05T10:00:00Z", ...over }) as Conversation;

  it("conversation list: rows use the nickname (resolved lazily, or primed by the inbox)", async () => {
    withAliases(<ConversationList conversations={[row(), row({ id: "conv-2", counterpart: { ...client, id: "c2", name: "Anna", alias: undefined } })]} viewerRole="psychic" activeId={null} onSelect={vi.fn()} />);
    expect(await screen.findByText("Moon Paul")).toBeTruthy();
    expect(screen.getByText("Anna")).toBeTruthy();
    expect(screen.queryByText("Paul")).toBeNull();
  });

  it("conversation list for a client shows the advisor's real name and asks for nothing", () => {
    h.session = { status: "authenticated", user: { id: "cl", role: "client" } };
    withAliases(<ConversationList conversations={[row({ counterpart: { id: "p1", role: "psychic", name: "Theo", status: "online" } })]} viewerRole="client" activeId={null} onSelect={vi.fn()} />);
    expect(screen.getByText("Theo")).toBeTruthy();
    expect(h.lookup).not.toHaveBeenCalled();
  });

  it("phone drawer: the current client card uses the nickname", async () => {
    withAliases(<SwitcherHeader role="psychic" counterpart={client} />);
    const card = screen.getByTestId("switcher-current");
    await waitFor(() => expect(within(card).getByText("Moon Paul")).toBeTruthy());
  });

  it("session card: the request names the client by nickname", async () => {
    const session = { id: "s1", conversation_id: "conv-1", status: "requested", client, psychic: { id: "p", role: "psychic", name: "Theo" }, rate_per_minute: 4 } as unknown as ChatSession;
    withAliases(
      <SessionPanel
        role="psychic"
        counterpart={client}
        session={session}
        summary={null}
        wallet={null}
        rate={4}
        secondsLeft={30}
        busy={false}
        error={null}
        returnPath="/en/chatroom"
        onStart={vi.fn()}
        onCancel={vi.fn()}
        onAccept={vi.fn()}
        onDecline={vi.fn()}
        onDismissSummary={vi.fn()}
      />,
    );
    expect(await screen.findByText("Moon Paul would like to start a live chat.")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Accept" }));
  });
});
