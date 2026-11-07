// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  PROMPT_KEY_PREFIX,
  chatNotificationTag,
  chatNotificationUrl,
  dismissPrompt,
  promptDismissed,
  shouldShowPrompt,
  shouldShowSystemNotification,
  shouldToastOtherConversation,
  showChatNotification,
  type AlertInput,
} from "@/lib/chat/chat-alerts";

const base: AlertInput = {
  message: { conversation_id: "c1", sender_id: "other", kind: "text" },
  meId: "me",
  activeConversationId: "c1",
  hidden: true,
  permission: "granted",
};
const with_ = (o: Partial<AlertInput>): AlertInput => ({ ...base, ...o });

describe("system notification decision", () => {
  it("fires for a counterpart text or image message in a hidden tab with permission", () => {
    expect(shouldShowSystemNotification(base)).toBe(true);
    expect(shouldShowSystemNotification(with_({ message: { ...base.message, kind: "image" } }))).toBe(true);
  });
  it("never fires for own, system or sender-less messages", () => {
    expect(shouldShowSystemNotification(with_({ message: { ...base.message, sender_id: "me" } }))).toBe(false);
    expect(shouldShowSystemNotification(with_({ message: { ...base.message, kind: "system" } }))).toBe(false);
    expect(shouldShowSystemNotification(with_({ message: { ...base.message, sender_id: undefined } }))).toBe(false);
  });
  it("needs a hidden tab and a granted permission", () => {
    expect(shouldShowSystemNotification(with_({ hidden: false }))).toBe(false);
    for (const permission of ["default", "denied", "unsupported"] as const) expect(shouldShowSystemNotification(with_({ permission }))).toBe(false);
  });
});

describe("toast for another conversation", () => {
  it("fires only for another conversation than the one on screen while visible", () => {
    const other = with_({ hidden: false, message: { ...base.message, conversation_id: "c2" } });
    expect(shouldToastOtherConversation(other)).toBe(true);
    expect(shouldToastOtherConversation(with_({ hidden: false }))).toBe(false);
    expect(shouldToastOtherConversation({ ...other, hidden: true })).toBe(false);
    expect(shouldToastOtherConversation({ ...other, message: { ...other.message, sender_id: "me" } })).toBe(false);
    expect(shouldToastOtherConversation({ ...other, activeConversationId: null })).toBe(false);
  });
});

describe("notification details", () => {
  it("uses the server push tag and a safe URL", () => {
    expect(chatNotificationTag("abc")).toBe("chat:abc");
    expect(chatNotificationUrl("fr", "ramone")).toBe("/fr/chatroom?reader=ramone");
    expect(chatNotificationUrl("en")).toBe("/en/chatroom");
    expect(chatNotificationUrl("en", "../evil?x=<")).toBe("/en/chatroom");
  });

  it("shows through the service worker registration with tag, title and body", async () => {
    const showNotification = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("isSecureContext", true);
    vi.stubGlobal("Notification", { permission: "granted" });
    vi.stubGlobal("PushManager", class {});
    Object.defineProperty(navigator, "serviceWorker", { configurable: true, value: { getRegistration: vi.fn().mockResolvedValue({ showNotification }) } });
    const ok = await showChatNotification({ conversationId: "c1", title: "New message from Theo", body: "Open the chatroom to read it.", url: "/en/chatroom" });
    expect(ok).toBe(true);
    const [title, options] = showNotification.mock.calls[0];
    expect(title).toBe("New message from Theo");
    expect(options).toMatchObject({ body: "Open the chatroom to read it.", tag: "chat:c1", renotify: true });
    expect(options.data.url).toMatch(/\/en\/chatroom$/);
    vi.unstubAllGlobals();
  });

  it("reports false when push is unsupported or no worker is registered", async () => {
    expect(await showChatNotification({ conversationId: "c1", title: "t", body: "b", url: "/en/chatroom" })).toBe(false);
    vi.stubGlobal("isSecureContext", true);
    vi.stubGlobal("Notification", { permission: "granted" });
    vi.stubGlobal("PushManager", class {});
    Object.defineProperty(navigator, "serviceWorker", { configurable: true, value: { getRegistration: vi.fn().mockResolvedValue(undefined) } });
    expect(await showChatNotification({ conversationId: "c1", title: "t", body: "b", url: "/en/chatroom" })).toBe(false);
    vi.unstubAllGlobals();
  });
});

describe("push prompt", () => {
  beforeEach(() => window.localStorage.clear());
  afterEach(() => vi.restoreAllMocks());
  const ok = { supported: true, permission: "default" as const, dismissed: false, hasSent: true, serverEnabled: true };
  it("shows once the user has sent a message, when push can be enabled", () => {
    expect(shouldShowPrompt(ok)).toBe(true);
    expect(shouldShowPrompt({ ...ok, hasSent: false })).toBe(false);
    expect(shouldShowPrompt({ ...ok, supported: false })).toBe(false);
    expect(shouldShowPrompt({ ...ok, serverEnabled: false })).toBe(false);
    expect(shouldShowPrompt({ ...ok, dismissed: true })).toBe(false);
    for (const permission of ["granted", "denied", "unsupported"] as const) expect(shouldShowPrompt({ ...ok, permission })).toBe(false);
  });
  it("remembers the dismissal per user", () => {
    expect(promptDismissed("u1")).toBe(false);
    dismissPrompt("u1");
    expect(promptDismissed("u1")).toBe(true);
    expect(promptDismissed("u2")).toBe(false);
    expect(window.localStorage.getItem(PROMPT_KEY_PREFIX + "u1")).not.toBeNull();
  });
  it("survives blocked storage", () => {
    vi.spyOn(window, "localStorage", "get").mockImplementation(() => {
      throw new Error("denied");
    });
    expect(promptDismissed("u1")).toBe(false);
    expect(() => dismissPrompt("u1")).not.toThrow();
  });
});
