// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import { composerKeyAction } from "@/components/chat/composer-keys";
import { PrepareImageError } from "@/lib/chat/prepare-image";
import { MessageComposer, normalizeOutgoing, type ComposerHandle } from "@/components/chat/message-composer";

const prepare = vi.hoisted(() => ({ prepareImage: vi.fn() }));
vi.mock("@/lib/chat/prepare-image", async (original) => ({ ...(await original<typeof import("@/lib/chat/prepare-image")>()), prepareImage: prepare.prepareImage }));

const dictionary = { ...en, copy } as Dictionary;

function setup(opts: { touch?: boolean; disabled?: boolean; ref?: React.Ref<ComposerHandle>; userId?: string; draftId?: string } = {}) {
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
      <MessageComposer
        ref={opts.ref}
        onSend={onSend}
        onTyping={onTyping}
        note="note"
        disabled={opts.disabled}
        disabledReason="Start a live chat to send messages"
        userId={opts.userId}
        draftId={opts.draftId}
      />
    </I18nProvider>,
  );
  const box = screen.getByRole("textbox", { name: "Message" }) as HTMLTextAreaElement;
  return { onSend, onTyping, box };
}

beforeEach(() => {
  prepare.prepareImage.mockReset();
  prepare.prepareImage.mockImplementation(async (file: File) => ({ file, width: 0, height: 0, reencoded: false }));
});
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

  it("prepares attachments, explains what cannot be sent and sends the text as the caption", async () => {
    const { onSend, box } = setup();
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: vi.fn(() => "blob:1"), revokeObjectURL: vi.fn() }));
    const input = screen.getByTestId("attach-input") as HTMLInputElement;
    expect(input.accept).toBe("image/*,.heic,.heif");
    prepare.prepareImage.mockRejectedValueOnce(new PrepareImageError("not_image"));
    fireEvent.change(input, { target: { files: [new File(["%PDF"], "doc.pdf", { type: "application/pdf" })] } });
    expect((await screen.findByRole("alert")).textContent).toBe("This file isn't an image we can send. Choose a PNG, JPEG, WebP or GIF image.");

    const png = new File([new Uint8Array([1, 2, 3])], "moon.png", { type: "image/png" });
    prepare.prepareImage.mockResolvedValueOnce({ file: png, width: 30, height: 20, reencoded: false });
    fireEvent.change(input, { target: { files: [png] } });
    expect(await screen.findByTestId("composer-attachment")).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();
    fireEvent.change(box, { target: { value: "Full moon 🌕" } });
    fireEvent.keyDown(box, { key: "Enter" });
    expect(onSend).toHaveBeenCalledWith({ body: "Full moon 🌕", file: png, size: { width: 30, height: 20 } });
  });

  it("shows a Preparing image state, blocks sending meanwhile, and can be cancelled", async () => {
    const { onSend, box } = setup();
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: vi.fn(() => "blob:p"), revokeObjectURL: vi.fn() }));
    let finish: (v: { file: File; width: number; height: number; reencoded: boolean }) => void = () => undefined;
    prepare.prepareImage.mockReturnValueOnce(new Promise((resolve) => (finish = resolve)));
    const photo = new File([new Uint8Array([1])], "photo.jpg", { type: "image/jpeg" });
    fireEvent.change(screen.getByTestId("attach-input"), { target: { files: [photo] } });
    expect((await screen.findByTestId("composer-preparing")).textContent).toContain("Preparing image…");
    fireEvent.change(box, { target: { value: "hi" } });
    fireEvent.keyDown(box, { key: "Enter" });
    expect(onSend).not.toHaveBeenCalled();
    // Cancel: the late result is ignored.
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await act(async () => finish({ file: photo, width: 1, height: 1, reencoded: false }));
    expect(screen.queryByTestId("composer-attachment")).toBeNull();
    expect(screen.queryByTestId("composer-preparing")).toBeNull();
  });

  it("accepts a pasted image", async () => {
    const { box } = setup();
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: vi.fn(() => "blob:2"), revokeObjectURL: vi.fn() }));
    const png = new File([new Uint8Array([1])], "paste.png", { type: "image/png" });
    fireEvent.paste(box, { clipboardData: { files: [png] } });
    expect(await screen.findByTestId("composer-attachment")).toBeTruthy();
  });

  it("attaches through the imperative handle (the conversation column is the drop target)", async () => {
    const ref = React.createRef<ComposerHandle>();
    setup({ ref });
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: vi.fn(() => "blob:3"), revokeObjectURL: vi.fn() }));
    const png = new File([new Uint8Array([1])], "drop.png", { type: "image/png" });
    await act(async () => ref.current!.attach(png));
    expect(screen.getByTestId("composer-attachment").textContent).toContain("drop.png");
    expect(document.activeElement).toBe(screen.getByRole("textbox", { name: "Message" }));
  });

  it("keeps the composer's own drag handlers out: the form is not a drop target any more", () => {
    const { box } = setup();
    const form = box.closest("form")!;
    const png = new File([new Uint8Array([1])], "drop.png", { type: "image/png" });
    fireEvent.dragEnter(form, { dataTransfer: { types: ["Files"], files: [png] } });
    expect(screen.queryByText("Drop the image to attach it")).toBeNull();
    expect(prepare.prepareImage).not.toHaveBeenCalled();
  });

  it("rejects images over 8 MB after preparation", async () => {
    setup();
    const big = new File([new Uint8Array([1])], "big.png", { type: "image/png" });
    Object.defineProperty(big, "size", { value: 8 * 1024 * 1024 + 1 });
    prepare.prepareImage.mockResolvedValueOnce({ file: big, width: 0, height: 0, reencoded: false });
    fireEvent.change(screen.getByTestId("attach-input"), { target: { files: [big] } });
    expect((await screen.findByRole("alert")).textContent).toBe("Images must be 8 MB or smaller.");
    expect(screen.queryByTestId("composer-attachment")).toBeNull();
  });

  it("shows the preparation error (HEIC the browser cannot decode)", async () => {
    setup();
    prepare.prepareImage.mockRejectedValueOnce(new PrepareImageError("undecodable"));
    fireEvent.change(screen.getByTestId("attach-input"), { target: { files: [new File([new Uint8Array([1])], "IMG_1.HEIC", { type: "" })] } });
    expect((await screen.findByRole("alert")).textContent).toBe("This photo format can't be sent from this browser. Choose JPEG or PNG, or send a screenshot.");
  });

  it("is inert while disabled, says why, and an attach while disabled shows the reason instead of dropping the file", async () => {
    const ref = React.createRef<ComposerHandle>();
    const { onSend, box } = setup({ disabled: true, ref });
    expect(box.disabled).toBe(true);
    expect(screen.getByTestId("composer-reason").textContent).toBe("Start a live chat to send messages");
    fireEvent.keyDown(box, { key: "Enter" });
    expect(onSend).not.toHaveBeenCalled();
    await act(async () => ref.current!.attach(new File([new Uint8Array([1])], "a.png", { type: "image/png" })));
    expect(screen.getByRole("alert").textContent).toBe("Start a live chat to send messages");
    expect(prepare.prepareImage).not.toHaveBeenCalled();
  });
});

describe("MessageComposer layout stability (§22.5)", () => {
  function mockMetrics(scrollHeight: () => number) {
    vi.spyOn(HTMLElement.prototype, "scrollHeight", "get").mockImplementation(scrollHeight);
    vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockReturnValue(300);
  }
  afterEach(() => vi.restoreAllMocks());

  it("measures with the hidden mirror and writes the height only when it changes", async () => {
    let content = 20;
    mockMetrics(() => content);
    const { box } = setup();
    const writes: string[] = [];
    const heightWrites = new MutationObserver(() => writes.push(box.style.height));
    heightWrites.observe(box, { attributes: true, attributeFilter: ["style"] });
    await waitFor(() => expect(box.style.height).toBe("20px"));
    await act(async () => undefined);
    writes.length = 0;
    // Typing that does not change the height writes nothing at all.
    for (const v of ["a", "ab", "abc"]) {
      fireEvent.change(box, { target: { value: v } });
      await act(async () => undefined);
    }
    expect(writes).toEqual([]);
    // The live textarea is never collapsed with height:auto to measure it.
    expect(box.style.height).not.toBe("auto");
    content = 60;
    fireEvent.change(box, { target: { value: "a\nb\nc" } });
    await waitFor(() => expect(box.style.height).toBe("60px"));
    // Capped at six lines: then it scrolls.
    content = 500;
    fireEvent.change(box, { target: { value: "x\n".repeat(30) } });
    await waitFor(() => expect(box.style.height).toBe("120px"));
    expect(box.style.overflowY).toBe("auto");
    heightWrites.disconnect();
    expect(document.querySelector(".composer-mirror")?.textContent).toContain("x");
  });

  it("puts the draft hint and the counter in a fixed slot beside the note, never in the layout flow of the field", async () => {
    vi.useFakeTimers();
    setup({ userId: "u1", draftId: "c1" });
    const box = screen.getByRole("textbox", { name: "Message" });
    const aside = screen.getByTestId("composer-aside");
    expect(aside.className).toBe("composer-meta-aside");
    expect(aside.previousElementSibling).toBe(screen.getByTestId("composer-note"));
    expect(aside.parentElement?.className).toBe("composer-meta");
    expect(aside.children).toHaveLength(0);
    fireEvent.change(box, { target: { value: "a".repeat(900) } });
    await act(async () => void vi.advanceTimersByTime(500));
    // Both come and go inside the same slot: the form's children and the note are untouched.
    expect(aside.contains(screen.getByTestId("draft-saved"))).toBe(true);
    expect(aside.contains(screen.getByText("900/1000"))).toBe(true);
    expect(screen.getByTestId("composer-note").parentElement).toBe(aside.parentElement);
    await act(async () => void vi.advanceTimersByTime(3000));
    expect(screen.queryByTestId("draft-saved")).toBeNull();
    expect(screen.getByTestId("composer-aside")).toBe(aside);
    vi.useRealTimers();
  });
});

describe("normalizeOutgoing", () => {
  it("drops leading blank lines and trailing whitespace only", () => {
    expect(normalizeOutgoing("\n\n  a  b\n\tc \n")).toBe("  a  b\n\tc");
  });
});
