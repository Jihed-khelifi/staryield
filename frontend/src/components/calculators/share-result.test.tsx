// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { tarotBirthResult } from "@/lib/astro/tarot-birth-card";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const m = vi.hoisted(() => ({ render: vi.fn(), track: vi.fn() }));
vi.mock("@/lib/share/render-card", () => ({ renderShareCard: m.render }));
vi.mock("@/lib/analytics", () => ({ track: m.track }));

import type { CalculatorResult } from "@/components/calculators/calculate";
import { ShareResultButton, ShareResultDialog } from "@/components/calculators/share-result";

const RESULT: CalculatorResult = { kind: "tarot-birth-card", tarot: tarotBirthResult("1987-06-15")! };
const PNG = new Blob(["png"], { type: "image/png" });

const objectUrls = { created: [] as string[], revoked: [] as string[] };

beforeAll(stubResizeObserver);
beforeEach(() => {
  m.render.mockReset().mockResolvedValue(PNG);
  m.track.mockReset();
  objectUrls.created = [];
  objectUrls.revoked = [];
  let n = 0;
  URL.createObjectURL = vi.fn(() => {
    const url = `blob:share-${++n}`;
    objectUrls.created.push(url);
    return url;
  });
  URL.revokeObjectURL = vi.fn((url: string) => void objectUrls.revoked.push(url));
  window.history.replaceState(null, "", "/en/calculators/tarot-birth-card#v=1&d=1987-06-15");
});
afterEach(() => {
  cleanup();
  Reflect.deleteProperty(navigator, "share");
  Reflect.deleteProperty(navigator, "canShare");
  Reflect.deleteProperty(navigator, "clipboard");
});

function setShare(share: (data: ShareData) => Promise<void>, canShare = true) {
  Object.defineProperty(navigator, "share", { configurable: true, value: vi.fn(share) });
  Object.defineProperty(navigator, "canShare", { configurable: true, value: vi.fn(() => canShare) });
  return navigator.share as unknown as ReturnType<typeof vi.fn>;
}

function openDialog() {
  const onOpenChange = vi.fn();
  const view = renderWithI18n(<ShareResultDialog result={RESULT} open onOpenChange={onOpenChange} />);
  return { ...view, onOpenChange };
}

describe("ShareResultButton", () => {
  it("is a labelled button", async () => {
    const onClick = vi.fn();
    renderWithI18n(<ShareResultButton onClick={onClick} />);
    await userEvent.setup().click(screen.getByRole("button", { name: "Share your result" }));
    expect(onClick).toHaveBeenCalled();
  });
});

describe("ShareResultDialog", () => {
  it("renders the image when it opens: skeleton, then the preview; counts the opening", async () => {
    let finish: (blob: Blob) => void = () => undefined;
    m.render.mockReturnValue(new Promise<Blob>((resolve) => (finish = resolve)));
    openDialog();
    expect(screen.getByTestId("share-loading")).toBeTruthy();
    expect(within(screen.getByTestId("share-loading")).getByRole("status").textContent).toBe("Preparing your image…");
    expect(m.track).toHaveBeenCalledWith("share_opened", { kind: "tarot-birth-card" });
    expect(m.render.mock.calls[0][0]).toMatchObject({ headline: "My tarot birth card is Wheel Of Fortune" });
    finish(PNG);
    const preview = await screen.findByTestId("share-preview");
    expect(preview.getAttribute("src")).toBe("blob:share-1");
    expect(preview.getAttribute("alt")).toContain("My tarot birth card is Wheel Of Fortune");
    expect(await axeViolations()).toEqual([]);
  });

  it("shares the PNG with the text and the calculator's link (no fragment) when the browser can", async () => {
    const share = setShare(async () => undefined);
    const u = userEvent.setup();
    openDialog();
    await u.click(await screen.findByTestId("share-native"));
    await screen.findByText("Shared. Thank you!");
    const data = share.mock.calls[0][0] as ShareData;
    expect(data.files?.[0].name).toBe("staryield-tarot-birth-card.png");
    expect(data.files?.[0].type).toBe("image/png");
    expect(data.text).toBe("My tarot birth card is Wheel Of Fortune — find yours at staryield.net");
    expect(data.url).toBe("http://localhost:3000/en/calculators/tarot-birth-card");
    expect(m.track).toHaveBeenCalledWith("share_completed", { kind: "tarot-birth-card", method: "native" });
  });

  it("treats a dismissed share sheet as the visitor's choice: no error, nothing counted", async () => {
    setShare(async () => {
      throw new DOMException("Share canceled", "AbortError");
    });
    const u = userEvent.setup();
    openDialog();
    await u.click(await screen.findByTestId("share-native"));
    await waitFor(() => expect(screen.getByTestId("share-native").hasAttribute("disabled")).toBe(false));
    expect(screen.getByTestId("share-feedback").textContent).toBe("");
    expect(m.track).not.toHaveBeenCalledWith("share_completed", expect.anything());
  });

  it("explains a failed share and keeps the other ways open", async () => {
    setShare(async () => {
      throw new DOMException("Permission denied", "NotAllowedError");
    });
    const u = userEvent.setup();
    openDialog();
    await u.click(await screen.findByTestId("share-native"));
    expect(await screen.findByText("Sharing did not work. You can download the image or copy the link instead.")).toBeTruthy();
    expect(screen.getByTestId("share-download")).toBeTruthy();
  });

  it("without file sharing: download is the main action and the dialog says why", async () => {
    setShare(async () => undefined, false);
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
    const u = userEvent.setup();
    openDialog();
    await u.click(await screen.findByTestId("share-download"));
    expect(screen.queryByTestId("share-native")).toBeNull();
    expect(screen.getByText("This browser cannot open the share menu with an image. Download it or copy the link.")).toBeTruthy();
    expect(click).toHaveBeenCalledTimes(1);
    expect((click.mock.contexts[0] as HTMLAnchorElement).download).toBe("staryield-tarot-birth-card.png");
    expect(screen.getByText("Image saved. Post it anywhere you like.")).toBeTruthy();
    expect(m.track).toHaveBeenCalledWith("share_completed", { kind: "tarot-birth-card", method: "download" });
  });

  it("copies the link, or shows it to copy by hand when the clipboard is refused", async () => {
    const u = userEvent.setup(); // installs its own clipboard: replace it after
    const writeText = vi.fn(async (value: string) => void value);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    openDialog();
    await u.click(await screen.findByTestId("share-copy"));
    expect(writeText).toHaveBeenCalledWith("http://localhost:3000/en/calculators/tarot-birth-card");
    expect(await screen.findByText("Link copied.")).toBeTruthy();
    expect(m.track).toHaveBeenCalledWith("share_completed", { kind: "tarot-birth-card", method: "copy_link" });

    writeText.mockRejectedValueOnce(new Error("denied"));
    document.execCommand = vi.fn(() => false);
    await u.click(screen.getByTestId("share-copy"));
    expect(await screen.findByText("Copy the link below.")).toBeTruthy();
    expect((screen.getByTestId("share-link-field") as HTMLInputElement).value).toBe("http://localhost:3000/en/calculators/tarot-birth-card");
  });

  it("offers a retry when the image cannot be created", async () => {
    m.render.mockRejectedValueOnce(new Error("canvas")).mockResolvedValueOnce(PNG);
    vi.spyOn(console, "warn").mockImplementation(() => undefined);
    const u = userEvent.setup();
    openDialog();
    expect((await screen.findByTestId("share-error")).textContent).toContain("We could not create your image.");
    expect(screen.queryByTestId("share-download")).toBeNull();
    await u.click(screen.getByTestId("share-retry"));
    expect(await screen.findByTestId("share-preview")).toBeTruthy();
    expect(m.render).toHaveBeenCalledTimes(2);
  });

  it("revokes the image's object URL when the dialog closes, and starts clean when it opens again", async () => {
    setShare(async () => {
      throw new DOMException("Permission denied", "NotAllowedError");
    });
    const u = userEvent.setup();
    const { rerender } = openDialog();
    await u.click(await screen.findByTestId("share-native"));
    await screen.findByText(/Sharing did not work/);
    rerender(<ShareResultDialog result={RESULT} open={false} onOpenChange={vi.fn()} />);
    await waitFor(() => expect(objectUrls.revoked).toEqual(["blob:share-1"]));
    rerender(<ShareResultDialog result={RESULT} open onOpenChange={vi.fn()} />);
    expect(await screen.findByTestId("share-preview")).toBeTruthy();
    expect(screen.queryByText(/Sharing did not work/)).toBeNull();
    expect(m.track.mock.calls.filter(([name]) => name === "share_opened")).toHaveLength(2);
  });
});
