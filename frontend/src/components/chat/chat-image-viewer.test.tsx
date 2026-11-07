// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import { ChatImageViewer, MAX_ZOOM, MIN_ZOOM, clampZoom, pinchZoom } from "@/components/chat/chat-image-viewer";

const dictionary = { ...en, copy } as Dictionary;
function setup(props: Partial<React.ComponentProps<typeof ChatImageViewer>> = {}) {
  const onOpenChange = vi.fn();
  render(
    <I18nProvider locale="en" dictionary={dictionary}>
      <ChatImageViewer open onOpenChange={onOpenChange} src="blob:x" alt="A sunrise" {...props} />
    </I18nProvider>,
  );
  return { onOpenChange };
}
const zoom = () => Number(screen.getByTestId("lightbox-stage").getAttribute("data-zoom"));

afterEach(() => cleanup());

describe("zoom maths", () => {
  it("clamps", () => {
    expect(clampZoom(0.2)).toBe(MIN_ZOOM);
    expect(clampZoom(99)).toBe(MAX_ZOOM);
    expect(clampZoom(2)).toBe(2);
  });
  it("pinch scales by the finger distance and ignores a zero start", () => {
    expect(pinchZoom(1, 100, 200)).toBe(2);
    expect(pinchZoom(2, 100, 50)).toBe(1);
    expect(pinchZoom(1, 100, 100000)).toBe(MAX_ZOOM);
    expect(pinchZoom(1.5, 0, 50)).toBe(1.5);
  });
});

describe("ChatImageViewer", () => {
  it("is a labelled dialog with the image and alt text", () => {
    setup();
    expect(screen.getByRole("dialog")).toBeTruthy();
    expect(screen.getByRole("img", { name: "A sunrise" }).getAttribute("src")).toBe("blob:x");
  });

  it("closes with the close button and Escape", () => {
    const { onOpenChange } = setup();
    fireEvent.click(screen.getByRole("button", { name: "Close image" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
    onOpenChange.mockClear();
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("shows the caption when there is one", () => {
    setup({ caption: "Look at this" });
    expect(screen.getByText("Look at this")).toBeTruthy();
  });

  it("renders nothing inside without a source", () => {
    setup({ src: null });
    expect(screen.queryByTestId("lightbox-stage")).toBeNull();
  });

  it("zooms with the buttons and resets", () => {
    setup();
    expect(zoom()).toBe(1);
    expect((screen.getByRole("button", { name: "Zoom out" }) as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "Zoom in" }));
    expect(zoom()).toBeCloseTo(1.5);
    fireEvent.click(screen.getByRole("button", { name: "Zoom in" }));
    expect(zoom()).toBeCloseTo(2.25);
    fireEvent.click(screen.getByRole("button", { name: "Reset zoom" }));
    expect(zoom()).toBe(1);
  });

  it("zooms with the keyboard, the wheel and a double tap", () => {
    setup();
    const stage = screen.getByTestId("lightbox-stage");
    fireEvent.keyDown(stage, { key: "+" });
    expect(zoom()).toBeCloseTo(1.5);
    fireEvent.keyDown(stage, { key: "-" });
    expect(zoom()).toBe(1);
    fireEvent.wheel(stage, { deltaY: -200 });
    expect(zoom()).toBeGreaterThan(1);
    fireEvent.keyDown(stage, { key: "0" });
    expect(zoom()).toBe(1);
    fireEvent.doubleClick(stage);
    expect(zoom()).toBeCloseTo(2.5);
    fireEvent.doubleClick(stage);
    expect(zoom()).toBe(1);
  });

  it("never zooms past the limits", () => {
    setup();
    const stage = screen.getByTestId("lightbox-stage");
    for (let i = 0; i < 12; i++) fireEvent.keyDown(stage, { key: "+" });
    expect(zoom()).toBe(MAX_ZOOM);
    for (let i = 0; i < 20; i++) fireEvent.keyDown(stage, { key: "-" });
    expect(zoom()).toBe(MIN_ZOOM);
  });
});
