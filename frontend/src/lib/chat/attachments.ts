/**
 * Chat image attachments: client-side validation (the server re-validates
 * and re-encodes every image) and a reference-counted cache of blob URLs.
 *
 * Images are private: the bytes are fetched through the API client with the
 * session cookies (refreshing an expired session when needed), never via a
 * public URL. Each attachment is downloaded once per page and shared by
 * every component showing it; blob URLs are revoked shortly after the last
 * user releases them.
 */
import { chatApi, ATTACHMENT_MAX_BYTES } from "@/lib/api/endpoints";

export const ACCEPTED_IMAGE_TYPES = ["image/png", "image/jpeg", "image/webp", "image/gif"] as const;
export const ACCEPT_ATTRIBUTE = ACCEPTED_IMAGE_TYPES.join(",");
export { ATTACHMENT_MAX_BYTES };

export type AttachmentProblem = "type" | "size" | "empty";

/** Validate a picked, pasted or dropped file. Returns null when it can be sent. */
export function validateAttachment(file: Pick<File, "type" | "size">): AttachmentProblem | null {
  if (!(ACCEPTED_IMAGE_TYPES as readonly string[]).includes(file.type)) return "type";
  if (file.size <= 0) return "empty";
  if (file.size > ATTACHMENT_MAX_BYTES) return "size";
  return null;
}

/** English copy per problem (translated with text()). */
export const attachmentProblemCopy: Record<AttachmentProblem, string> = {
  type: "Only PNG, JPEG, WebP or GIF images can be sent.",
  size: "Images must be 8 MB or smaller.",
  empty: "This image could not be read.",
};

/** The first file of a DataTransfer/FileList-like collection that looks like an image (or any file). */
export function firstFile(files: ArrayLike<File> | null | undefined): File | null {
  if (!files || files.length === 0) return null;
  for (let i = 0; i < files.length; i++) if (files[i].type.startsWith("image/")) return files[i];
  return files[0];
}

type Entry = { url?: string; promise?: Promise<string>; refs: number; revokeTimer?: ReturnType<typeof setTimeout> };

const RELEASE_DELAY_MS = 30_000;
const entries = new Map<string, Entry>();

function entry(id: string): Entry {
  let e = entries.get(id);
  if (!e) {
    e = { refs: 0 };
    entries.set(id, e);
  }
  return e;
}

function revoke(id: string) {
  const e = entries.get(id);
  if (!e || e.refs > 0) return;
  e.revokeTimer = undefined;
  // A download in flight schedules its own release when it lands.
  if (e.promise) return;
  if (e.url) URL.revokeObjectURL(e.url);
  entries.delete(id);
}

/** Store bytes we already have (an image this tab just uploaded). */
export function seedAttachment(id: string, blob: Blob) {
  const e = entry(id);
  if (e.url || e.promise) return;
  e.url = URL.createObjectURL(blob);
  if (e.refs === 0) e.revokeTimer = setTimeout(() => revoke(id), RELEASE_DELAY_MS);
}

/** Get (downloading once) a blob URL for an attachment; pair with releaseAttachment. */
export function acquireAttachment(id: string): Promise<string> {
  const e = entry(id);
  e.refs++;
  if (e.revokeTimer) {
    clearTimeout(e.revokeTimer);
    e.revokeTimer = undefined;
  }
  if (e.url) return Promise.resolve(e.url);
  if (!e.promise) {
    e.promise = chatApi.attachment(id).then(
      (blob) => {
        e.url = URL.createObjectURL(blob);
        e.promise = undefined;
        if (entries.get(id) !== e) entries.set(id, e); // cache cleared meanwhile
        if (e.revokeTimer) clearTimeout(e.revokeTimer);
        e.revokeTimer = e.refs === 0 ? setTimeout(() => revoke(id), RELEASE_DELAY_MS) : undefined;
        return e.url;
      },
      (err) => {
        e.promise = undefined;
        throw err;
      },
    );
  }
  return e.promise;
}

/** Release a URL obtained with acquireAttachment (revoked once unused for a while). */
export function releaseAttachment(id: string) {
  const e = entries.get(id);
  if (!e) return;
  e.refs = Math.max(0, e.refs - 1);
  if (e.refs === 0 && !e.revokeTimer) e.revokeTimer = setTimeout(() => revoke(id), RELEASE_DELAY_MS);
}

/** Revoke everything (sign-out). */
export function clearAttachmentCache() {
  for (const [, e] of entries) {
    if (e.revokeTimer) clearTimeout(e.revokeTimer);
    if (e.url) URL.revokeObjectURL(e.url);
  }
  entries.clear();
}

/** Natural size of a local image file (for the optimistic placeholder). */
export function imageSize(file: Blob): Promise<{ width: number; height: number } | null> {
  if (typeof window === "undefined") return Promise.resolve(null);
  return new Promise((resolve) => {
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      resolve(img.naturalWidth && img.naturalHeight ? { width: img.naturalWidth, height: img.naturalHeight } : null);
      URL.revokeObjectURL(url);
    };
    img.onerror = () => {
      resolve(null);
      URL.revokeObjectURL(url);
    };
    img.src = url;
  });
}
