// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";

import { renderWithI18n } from "@/test/render";

const m = vi.hoisted(() => ({
  pushApi: { config: vi.fn() },
  settingsApi: { update: vi.fn() },
  enablePush: vi.fn(),
  supported: { current: true },
}));
vi.mock("@/lib/api/account", () => ({ pushApi: m.pushApi, settingsApi: m.settingsApi }));
vi.mock("@/lib/push", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/push")>()),
  enablePush: m.enablePush,
  pushSupported: () => m.supported.current,
  pushPermission: () => "default",
}));

import { ChatPushPrompt } from "@/components/chat/push-prompt";
import { PROMPT_KEY_PREFIX } from "@/lib/chat/chat-alerts";

beforeEach(() => {
  window.localStorage.clear();
  m.supported.current = true;
  m.pushApi.config.mockReset().mockResolvedValue({ enabled: true, public_key: "KEY" });
  m.settingsApi.update.mockReset().mockResolvedValue({});
  m.enablePush.mockReset().mockResolvedValue("subscribed");
});
afterEach(cleanup);

describe("ChatPushPrompt", () => {
  it("keeps the client copy and waits for the first message", async () => {
    const view = renderWithI18n(<ChatPushPrompt userId="u1" hasSent={false} />);
    await Promise.resolve();
    expect(screen.queryByTestId("chat-push-prompt")).toBeNull();
    view.rerender(<ChatPushPrompt userId="u1" hasSent />);
    expect(await screen.findByText("Don't miss a reply")).toBeTruthy();
    expect(screen.getByText("Turn on notifications to know when a new message arrives, even when this page is closed.")).toBeTruthy();
  });
  it("shows advisors the chat-request variant without hasSent", async () => {
    renderWithI18n(<ChatPushPrompt userId="p1" hasSent={false} role="psychic" />);
    expect(await screen.findByText("Never miss a chat request")).toBeTruthy();
    expect(screen.getByText("Turn on notifications to hear about new chat requests, even when this page is closed.")).toBeTruthy();
  });
  it("respects the shared dismissal key", async () => {
    window.localStorage.setItem(PROMPT_KEY_PREFIX + "p1", "1");
    renderWithI18n(<ChatPushPrompt userId="p1" role="psychic" />);
    await Promise.resolve();
    expect(screen.queryByTestId("chat-push-prompt")).toBeNull();
  });
  it("hides when push is unsupported", () => {
    m.supported.current = false;
    renderWithI18n(<ChatPushPrompt userId="p1" role="psychic" />);
    expect(screen.queryByTestId("chat-push-prompt")).toBeNull();
  });
  it("turns on, saves the account switch and confirms for advisors", async () => {
    renderWithI18n(<ChatPushPrompt userId="p1" role="psychic" />);
    await waitFor(() => expect(m.pushApi.config).toHaveBeenCalled());
    fireEvent.click(await screen.findByRole("button", { name: "Turn on notifications" }));
    expect(await screen.findByTestId("chat-push-done")).toBeTruthy();
    expect(m.enablePush).toHaveBeenCalledWith("KEY");
    expect(m.settingsApi.update).toHaveBeenCalledWith({ notifications: { push: true } });
  });
  it("explains Brave with a copyable address and retries", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    m.enablePush.mockResolvedValueOnce("brave_blocked");
    renderWithI18n(<ChatPushPrompt userId="p1" role="psychic" />);
    await waitFor(() => expect(m.pushApi.config).toHaveBeenCalled());
    fireEvent.click(await screen.findByRole("button", { name: "Turn on notifications" }));
    expect(await screen.findByTestId("push-brave-address")).toHaveProperty("textContent", "brave://settings/privacy");
    fireEvent.click(screen.getByTestId("push-brave-copy"));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith("brave://settings/privacy"));
    expect(await screen.findByText("Copied")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByTestId("chat-push-done")).toBeTruthy();
  });
  it("shows each failure outcome's message", async () => {
    m.enablePush.mockResolvedValue("server_rejected");
    renderWithI18n(<ChatPushPrompt userId="u1" hasSent />);
    await waitFor(() => expect(m.pushApi.config).toHaveBeenCalled());
    fireEvent.click(await screen.findByRole("button", { name: "Turn on notifications" }));
    expect(await screen.findByText(/isn't supported\. Try Chrome/)).toBeTruthy();
  });
  it("dismisses and remembers", async () => {
    renderWithI18n(<ChatPushPrompt userId="p1" role="psychic" />);
    fireEvent.click(await screen.findByRole("button", { name: "Not now" }));
    expect(screen.queryByTestId("chat-push-prompt")).toBeNull();
    expect(window.localStorage.getItem(PROMPT_KEY_PREFIX + "p1")).not.toBeNull();
  });
});
