// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const attachment = vi.fn();
vi.mock("@/lib/api/endpoints", () => ({
  ATTACHMENT_MAX_BYTES: 8 * 1024 * 1024,
  chatApi: { attachment: (id: string) => attachment(id) },
}));

import {
  ACCEPT_ATTRIBUTE,
  acquireAttachment,
  clearAttachmentCache,
  firstFile,
  releaseAttachment,
  seedAttachment,
  validateAttachment,
} from "@/lib/chat/attachments";

const file = (type: string, size: number, name = "x") => new File([new Uint8Array(Math.min(size, 16))], name, { type }) as File & { size: number };
const sized = (type: string, size: number) => Object.defineProperty(file(type, size), "size", { value: size });

describe("validateAttachment", () => {
  it.each(["image/png", "image/jpeg", "image/webp", "image/gif"])("accepts %s", (type) => {
    expect(validateAttachment(sized(type, 1024))).toBeNull();
  });

  it.each(["image/svg+xml", "image/heic", "application/pdf", "text/html", ""])("rejects %s", (type) => {
    expect(validateAttachment(sized(type, 1024))).toBe("type");
  });

  it("enforces the 8 MB limit and rejects empty files", () => {
    expect(validateAttachment(sized("image/png", 8 * 1024 * 1024))).toBeNull();
    expect(validateAttachment(sized("image/png", 8 * 1024 * 1024 + 1))).toBe("size");
    expect(validateAttachment(sized("image/png", 0))).toBe("empty");
  });

  it("advertises only accepted types to the file picker", () => {
    expect(ACCEPT_ATTRIBUTE).toBe("image/png,image/jpeg,image/webp,image/gif");
  });

  it("firstFile prefers images", () => {
    const pdf = file("application/pdf", 10, "a.pdf");
    const png = file("image/png", 10, "b.png");
    expect(firstFile([pdf, png])).toBe(png);
    expect(firstFile([pdf])).toBe(pdf);
    expect(firstFile([])).toBeNull();
    expect(firstFile(null)).toBeNull();
  });
});

describe("attachment cache", () => {
  let n = 0;
  beforeEach(() => {
    vi.useFakeTimers();
    attachment.mockReset();
    attachment.mockImplementation(async () => new Blob(["img"], { type: "image/png" }));
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: vi.fn(() => `blob:${++n}`), revokeObjectURL: vi.fn() }));
  });
  afterEach(() => {
    clearAttachmentCache();
    vi.useRealTimers();
  });

  it("downloads once for concurrent users and revokes after the last release", async () => {
    const [a, b] = await Promise.all([acquireAttachment("att-1"), acquireAttachment("att-1")]);
    expect(a).toBe(b);
    expect(attachment).toHaveBeenCalledTimes(1);
    releaseAttachment("att-1");
    await vi.advanceTimersByTimeAsync(60_000);
    expect(URL.revokeObjectURL).not.toHaveBeenCalled();
    releaseAttachment("att-1");
    await vi.advanceTimersByTimeAsync(60_000);
    expect(URL.revokeObjectURL).toHaveBeenCalledWith(a);
  });

  it("re-acquiring before the release delay reuses the URL", async () => {
    const a = await acquireAttachment("att-2");
    releaseAttachment("att-2");
    await vi.advanceTimersByTimeAsync(5_000);
    const b = await acquireAttachment("att-2");
    expect(b).toBe(a);
    expect(attachment).toHaveBeenCalledTimes(1);
  });

  it("uses seeded bytes without downloading", async () => {
    seedAttachment("att-3", new Blob(["mine"]));
    await acquireAttachment("att-3");
    expect(attachment).not.toHaveBeenCalled();
  });

  it("propagates failures and allows a retry", async () => {
    attachment.mockRejectedValueOnce(new Error("404"));
    await expect(acquireAttachment("att-4")).rejects.toThrow("404");
    releaseAttachment("att-4");
    await expect(acquireAttachment("att-4")).resolves.toMatch(/^blob:/);
    expect(attachment).toHaveBeenCalledTimes(2);
  });
});
