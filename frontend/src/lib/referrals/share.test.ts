// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";

import { canWebShare, copyText, shareInvite } from "@/lib/referrals/share";

const URL_ = "https://staryield.net/en?ref=ABCD2345";
const data = { url: URL_, title: "Join me", text: "Join with my link:" };

function execCommand(result: boolean | "throw") {
  const fn = vi.fn(() => {
    if (result === "throw") throw new Error("denied");
    return result;
  });
  Object.defineProperty(document, "execCommand", { value: fn, configurable: true });
  return fn;
}

afterEach(() => {
  document.body.innerHTML = "";
});

describe("copyText", () => {
  it("uses the Clipboard API when it works", async () => {
    const writeText = vi.fn(async () => {});
    const fallback = execCommand(true);
    expect(await copyText(URL_, { clipboard: { writeText } }, document)).toBe("clipboard");
    expect(writeText).toHaveBeenCalledWith(URL_);
    expect(fallback).not.toHaveBeenCalled();
  });

  it("falls back to a hidden textarea when the Clipboard API is refused or missing, and cleans up", async () => {
    const fallback = execCommand(true);
    const refused = vi.fn(async () => {
      throw new Error("NotAllowedError");
    });
    expect(await copyText(URL_, { clipboard: { writeText: refused } }, document)).toBe("fallback");
    expect(await copyText(URL_, {}, document)).toBe("fallback");
    expect(fallback).toHaveBeenCalledWith("copy");
    expect(document.querySelector("textarea")).toBeNull();
  });

  it("gives the focus back to the button that asked", async () => {
    execCommand(true);
    const button = document.createElement("button");
    document.body.appendChild(button);
    button.focus();
    await copyText(URL_, {}, document);
    expect(document.activeElement).toBe(button);
  });

  it("reports failure when nothing can copy", async () => {
    execCommand(false);
    expect(await copyText(URL_, {}, document)).toBeNull();
    execCommand("throw");
    expect(await copyText(URL_, {}, document)).toBeNull();
    expect(await copyText(URL_, {}, undefined)).toBeNull();
  });
});

describe("canWebShare", () => {
  it("needs navigator.share and respects canShare", () => {
    expect(canWebShare(undefined)).toBe(false);
    expect(canWebShare({})).toBe(false);
    expect(canWebShare({ share: vi.fn() })).toBe(true);
    expect(canWebShare({ share: vi.fn(), canShare: () => false })).toBe(false);
    expect(
      canWebShare({
        share: vi.fn(),
        canShare: () => {
          throw new Error("bad");
        },
      }),
    ).toBe(false);
  });
});

describe("shareInvite", () => {
  it("opens the native share sheet", async () => {
    const share = vi.fn(async () => {});
    expect(await shareInvite(data, { share }, document)).toEqual({ method: "web_share", result: "shared" });
    expect(share).toHaveBeenCalledWith(data);
  });

  it("treats a dismissed sheet as a cancellation, not an error", async () => {
    const share = vi.fn(async () => {
      throw Object.assign(new Error("dismissed"), { name: "AbortError" });
    });
    expect(await shareInvite(data, { share }, document)).toEqual({ method: "web_share", result: "cancelled" });
  });

  it("copies when the sheet fails or does not exist", async () => {
    const writeText = vi.fn(async () => {});
    const share = vi.fn(async () => {
      throw Object.assign(new Error("denied"), { name: "NotAllowedError" });
    });
    expect(await shareInvite(data, { share, clipboard: { writeText } }, document)).toEqual({ method: "copy", result: "copied" });
    expect(await shareInvite(data, { clipboard: { writeText } }, document)).toEqual({ method: "copy", result: "copied" });
    execCommand(false);
    expect(await shareInvite(data, {}, document)).toEqual({ method: "copy", result: "failed" });
  });
});
