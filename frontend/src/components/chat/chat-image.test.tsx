// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";

const h = vi.hoisted(() => ({ acquire: vi.fn() }));
vi.mock("@/lib/chat/attachments", () => ({ acquireAttachment: (...a: unknown[]) => h.acquire(...a), releaseAttachment: vi.fn() }));
vi.mock("@/lib/use-media-query", () => ({ useMediaQuery: () => false }));

import { ChatImage, displaySize } from "@/components/chat/chat-image";

const dictionary = { ...en, copy } as Dictionary;
const attachment = { id: "a1", content_type: "image/png", width: 400, height: 200, bytes: 10 };
const ui = () => (
  <I18nProvider locale="en" dictionary={dictionary}>
    <ChatImage attachment={attachment} alt="A card" />
  </I18nProvider>
);

beforeEach(() => {
  h.acquire.mockReset();
});
afterEach(() => {
  cleanup();
});

describe("displaySize", () => {
  it("reserves the stored aspect ratio and caps the size", () => {
    expect(displaySize(400, 200, 260, 320)).toEqual({ width: 260, ratio: 2 });
    expect(displaySize(100, 400, 260, 320)).toEqual({ width: 80, ratio: 0.25 });
    expect(displaySize(0, 0, 260, 320).ratio).toBeCloseTo(4 / 3);
  });
});

describe("ChatImage", () => {
  it("shows a skeleton frame with the aspect ratio before the pixels load", () => {
    h.acquire.mockImplementation(() => new Promise(() => undefined));
    render(ui());
    const frame = screen.getByRole("button", { name: "Open image: A card" }) as HTMLButtonElement;
    expect(frame.disabled).toBe(true);
    expect(frame.className).toContain("chat-image-loading");
    expect(frame.style.aspectRatio).toMatch(/^2/);
  });

  it("opens the viewer when loaded", async () => {
    h.acquire.mockResolvedValue("blob:ok");
    render(ui());
    const frame = await waitFor(() => {
      const b = screen.getByRole("button", { name: "Open image: A card" }) as HTMLButtonElement;
      expect(b.disabled).toBe(false);
      return b;
    });
    fireEvent.click(frame);
    expect(screen.getByTestId("image-viewer")).toBeTruthy();
  });

  it("shows an error with retry when the download fails, and recovers", async () => {
    h.acquire.mockRejectedValueOnce(new Error("net")).mockResolvedValue("blob:ok");
    render(ui());
    expect(await screen.findByTestId("chat-image-error")).toBeTruthy();
    expect(screen.getByText("This image couldn't be loaded.")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.queryByTestId("chat-image-error")).toBeNull());
    expect(h.acquire).toHaveBeenCalledTimes(2);
  });

  it("shows the error state when the image cannot be decoded", async () => {
    h.acquire.mockResolvedValue("blob:bad");
    const { container } = render(ui());
    const img = await waitFor(() => {
      const el = container.querySelector("img");
      expect(el).toBeTruthy();
      return el!;
    });
    fireEvent.error(img);
    expect(await screen.findByTestId("chat-image-error")).toBeTruthy();
  });
});
