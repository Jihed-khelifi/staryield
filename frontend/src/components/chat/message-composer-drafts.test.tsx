// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import { draftKey, loadDraft, saveDraft } from "@/lib/chat/drafts";
import { DRAFT_DEBOUNCE_MS, MessageComposer } from "@/components/chat/message-composer";

const dictionary = { ...en, copy } as Dictionary;

type Props = Partial<React.ComponentProps<typeof MessageComposer>>;
function ui(props: Props = {}) {
  return (
    <I18nProvider locale="en" dictionary={dictionary}>
      <MessageComposer onSend={props.onSend ?? vi.fn()} note="note" userId="u1" draftId="c1" {...props} />
    </I18nProvider>
  );
}
const box = () => screen.getByRole("textbox", { name: "Message" }) as HTMLTextAreaElement;
const type = (value: string) => fireEvent.change(box(), { target: { value } });
const settle = () => act(() => void vi.advanceTimersByTime(DRAFT_DEBOUNCE_MS + 10));

beforeEach(() => {
  vi.useFakeTimers();
  window.localStorage.clear();
  vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: false, media: "", addEventListener: vi.fn(), removeEventListener: vi.fn() })));
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("composer drafts", () => {
  it("saves the text after a pause and shows a Draft saved hint", () => {
    render(ui());
    type("half a thought");
    expect(loadDraft("u1", "c1")).toBe("");
    settle();
    expect(loadDraft("u1", "c1")).toBe("half a thought");
    expect(screen.getByTestId("draft-saved").textContent).toBe("Draft saved");
    act(() => void vi.advanceTimersByTime(3000));
    expect(screen.queryByTestId("draft-saved")).toBeNull();
  });

  it("restores the draft after a reload (new mount)", () => {
    saveDraft("u1", "c1", "survived the reload");
    render(ui());
    expect(box().value).toBe("survived the reload");
  });

  it("restores per conversation when switching", () => {
    saveDraft("u1", "c1", "for one");
    saveDraft("u1", "c2", "for two");
    const { rerender } = render(ui({ draftId: "c1" }));
    expect(box().value).toBe("for one");
    rerender(ui({ draftId: "c2" }));
    expect(box().value).toBe("for two");
    rerender(ui({ draftId: "c3" }));
    expect(box().value).toBe("");
    // Typing in c3 never overwrites c1/c2.
    type("three");
    settle();
    expect(loadDraft("u1", "c3")).toBe("three");
    expect(loadDraft("u1", "c1")).toBe("for one");
  });

  it("falls back to the advisor-slug draft until a conversation exists, then moves it", () => {
    saveDraft("u1", "reader:theo", "typed before the chat existed");
    render(ui({ draftId: "c9", draftFallbackId: "reader:theo" }));
    expect(box().value).toBe("typed before the chat existed");
    type("typed before the chat existed!");
    settle();
    expect(loadDraft("u1", "c9")).toBe("typed before the chat existed!");
    expect(loadDraft("u1", "reader:theo")).toBe("");
  });

  it("clears the draft on send", () => {
    const onSend = vi.fn();
    saveDraft("u1", "c1", "hello");
    render(ui({ onSend }));
    fireEvent.submit(box().closest("form")!);
    expect(onSend).toHaveBeenCalledWith({ body: "hello" });
    expect(box().value).toBe("");
    expect(window.localStorage.getItem(draftKey("u1", "c1"))).toBeNull();
    settle();
    expect(loadDraft("u1", "c1")).toBe("");
  });

  it("removes the draft when the text is erased", () => {
    saveDraft("u1", "c1", "x");
    render(ui());
    type("");
    settle();
    expect(window.localStorage.getItem(draftKey("u1", "c1"))).toBeNull();
  });

  it("keeps the draft while the composer is disabled and shows it when enabled", () => {
    saveDraft("u1", "c1", "waiting for a session");
    const { rerender } = render(ui({ disabled: true, disabledReason: "Start a live chat to send messages" }));
    expect(box().value).toBe("waiting for a session");
    expect(box().disabled).toBe(true);
    expect(loadDraft("u1", "c1")).toBe("waiting for a session");
    rerender(ui({ disabled: false }));
    expect(box().value).toBe("waiting for a session");
    expect(box().disabled).toBe(false);
  });

  it("flushes immediately when the page is hidden or unloaded", () => {
    render(ui());
    type("quick reload");
    expect(loadDraft("u1", "c1")).toBe("");
    window.dispatchEvent(new Event("pagehide"));
    expect(loadDraft("u1", "c1")).toBe("quick reload");
  });

  it("follows another tab's edit (storage event) unless the user is typing here", () => {
    render(ui());
    saveDraft("u1", "c1", "from tab two");
    act(() => void window.dispatchEvent(new StorageEvent("storage", { key: draftKey("u1", "c1") })));
    expect(box().value).toBe("from tab two");
    box().focus();
    saveDraft("u1", "c1", "ignored while typing");
    act(() => void window.dispatchEvent(new StorageEvent("storage", { key: draftKey("u1", "c1") })));
    expect(box().value).toBe("from tab two");
  });

  it("works (without persisting) when storage is blocked", () => {
    vi.spyOn(window, "localStorage", "get").mockImplementation(() => {
      throw new Error("denied");
    });
    render(ui());
    type("still typing");
    settle();
    expect(box().value).toBe("still typing");
    expect(screen.queryByTestId("draft-saved")).toBeNull();
  });

  it("does nothing without a user or key", () => {
    render(ui({ userId: undefined, draftId: null }));
    type("anon");
    settle();
    expect(window.localStorage.length).toBe(0);
  });
});
