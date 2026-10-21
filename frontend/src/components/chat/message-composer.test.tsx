// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import { composerKeyAction } from "@/components/chat/composer-keys";
import { MessageComposer, normalizeOutgoing } from "@/components/chat/message-composer";

const dictionary = { ...en, copy } as Dictionary;

function setup(opts: { touch?: boolean; disabled?: boolean } = {}) {
  vi.stubGlobal(
    "matchMedia",
    vi.fn((query: string) => ({
      matches: Boolean(opts.touch) && query.includes("pointer: coarse"),
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  );
  const onSend = vi.fn();
  const onTyping = vi.fn();
  render(
    <I18nProvider locale="en" dictionary={dictionary}>
      <MessageComposer onSend={onSend} onTyping={onTyping} note="note" disabled={opts.disabled} disabledReason="Start a live chat to send messages" />
    </I18nProvider>,
  );
  const box = screen.getByRole("textbox", { name: "Message" }) as HTMLTextAreaElement;
  return { onSend, onTyping, box };
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("composerKeyAction", () => {
  const key = (k: Partial<Parameters<typeof composerKeyAction>[0]>) => ({ key: "Enter", shiftKey: false, ...k });
  it("desktop: Enter sends, Shift/Alt+Enter insert a newline", () => {
    expect(composerKeyAction(key({}), false)).toBe("send");
    expect(composerKeyAction(key({ shiftKey: true }), false)).toBe("newline");
    expect(composerKeyAction(key({ altKey: true }), false)).toBe("newline");
  });
  it("touch: Enter inserts a newline", () => {
    expect(composerKeyAction(key({}), true)).toBe("newline");
  });
  it("Ctrl/Cmd+Enter always sends; IME composition never does", () => {
    expect(composerKeyAction(key({ ctrlKey: true }), true)).toBe("send");
    expect(composerKeyAction(key({ metaKey: true }), true)).toBe("send");
    expect(composerKeyAction(key({ isComposing: true }), false)).toBe("default");
    expect(composerKeyAction(key({ keyCode: 229 }), false)).toBe("default");
  });
  it("ignores other keys", () => {
    expect(composerKeyAction(key({ key: "a" }), false)).toBe("default");
  });
});

describe("MessageComposer", () => {
  it("sends on Enter and keeps inner spacing and newlines", () => {
    const { onSend, box } = setup();
    fireEvent.change(box, { target: { value: "\n  Hello\n   indented  line  \n\n" } });
    fireEvent.keyDown(box, { key: "Enter" });
    expect(onSend).toHaveBeenCalledWith({ body: "  Hello\n   indented  line" });
    expect(box.value).toBe("");
  });

  it("does not send on Shift+Enter", () => {
    const { onSend, box } = setup();
    fireEvent.change(box, { target: { value: "line one" } });
    const allowed = fireEvent.keyDown(box, { key: "Enter", shiftKey: true });
    expect(allowed).toBe(true); // default not prevented: the browser inserts the newline
    expect(onSend).not.toHaveBeenCalled();
  });

  it("on touch devices Enter is a newline and the button sends", () => {
    const { onSend, box } = setup({ touch: true });
    fireEvent.change(box, { target: { value: "hi" } });
    expect(fireEvent.keyDown(box, { key: "Enter" })).toBe(true);
    expect(onSend).not.toHaveBeenCalled();
    expect(box.getAttribute("enterkeyhint")).toBe("enter");
    fireEvent.click(screen.getByRole("button", { name: "Send message" }));
    expect(onSend).toHaveBeenCalledWith({ body: "hi" });
  });

  it("refuses empty messages", () => {
    const { onSend, box } = setup();
    fireEvent.change(box, { target: { value: "   \n " } });
    fireEvent.keyDown(box, { key: "Enter" });
    expect(onSend).not.toHaveBeenCalled();
    expect(screen.getByRole("alert").textContent).toBe("Write something before sending.");
  });

  it("shows a counter near the 1000 character limit", () => {
    const { box } = setup();
    fireEvent.change(box, { target: { value: "a".repeat(799) } });
    expect(screen.queryByText("799/1000")).toBeNull();
    fireEvent.change(box, { target: { value: "a".repeat(950) } });
    expect(screen.getByText("950/1000")).toBeTruthy();
  });

  it("announces typing at most every 2 seconds", () => {
    vi.useFakeTimers();
    const { onTyping, box } = setup();
    fireEvent.change(box, { target: { value: "a" } });
    fireEvent.change(box, { target: { value: "ab" } });
    expect(onTyping).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(3100);
    expect(onTyping).toHaveBeenLastCalledWith(false);
    vi.useRealTimers();
  });

  it("validates attachments and sends the text as the caption", () => {
    const { onSend, box } = setup();
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: vi.fn(() => "blob:1"), revokeObjectURL: vi.fn() }));
    const input = screen.getByTestId("attach-input") as HTMLInputElement;
    const pdf = new File(["%PDF"], "doc.pdf", { type: "application/pdf" });
    fireEvent.change(input, { target: { files: [pdf] } });
    expect(screen.getByRole("alert").textContent).toBe("Only PNG, JPEG, WebP or GIF images can be sent.");

    const png = new File([new Uint8Array([1, 2, 3])], "moon.png", { type: "image/png" });
    fireEvent.change(input, { target: { files: [png] } });
    expect(screen.getByTestId("composer-attachment")).toBeTruthy();
    fireEvent.change(box, { target: { value: "Full moon 🌕" } });
    fireEvent.keyDown(box, { key: "Enter" });
    expect(onSend).toHaveBeenCalledWith({ body: "Full moon 🌕", file: png });
  });

  it("accepts a pasted image", () => {
    const { box } = setup();
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: vi.fn(() => "blob:2"), revokeObjectURL: vi.fn() }));
    const png = new File([new Uint8Array([1])], "paste.png", { type: "image/png" });
    fireEvent.paste(box, { clipboardData: { files: [png] } });
    expect(screen.getByTestId("composer-attachment")).toBeTruthy();
  });

  it("accepts a dropped image and shows the drop zone while dragging", () => {
    const { box } = setup();
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: vi.fn(() => "blob:3"), revokeObjectURL: vi.fn() }));
    const form = box.closest("form")!;
    const png = new File([new Uint8Array([1])], "drop.png", { type: "image/png" });
    fireEvent.dragEnter(form, { dataTransfer: { types: ["Files"], files: [png] } });
    expect(screen.getByText("Drop the image to attach it")).toBeTruthy();
    fireEvent.drop(form, { dataTransfer: { types: ["Files"], files: [png] } });
    expect(screen.queryByText("Drop the image to attach it")).toBeNull();
    expect(screen.getByTestId("composer-attachment").textContent).toContain("drop.png");
  });

  it("rejects images over 8 MB", () => {
    setup();
    const big = new File([new Uint8Array([1])], "big.png", { type: "image/png" });
    Object.defineProperty(big, "size", { value: 8 * 1024 * 1024 + 1 });
    fireEvent.change(screen.getByTestId("attach-input"), { target: { files: [big] } });
    expect(screen.getByRole("alert").textContent).toBe("Images must be 8 MB or smaller.");
    expect(screen.queryByTestId("composer-attachment")).toBeNull();
  });

  it("is inert while disabled", () => {
    const { onSend, box } = setup({ disabled: true });
    expect(box.disabled).toBe(true);
    expect(box.placeholder).toBe("Start a live chat to send messages");
    fireEvent.keyDown(box, { key: "Enter" });
    expect(onSend).not.toHaveBeenCalled();
  });
});

describe("normalizeOutgoing", () => {
  it("drops leading blank lines and trailing whitespace only", () => {
    expect(normalizeOutgoing("\n\n  a  b\n\tc \n")).toBe("  a  b\n\tc");
  });
});
