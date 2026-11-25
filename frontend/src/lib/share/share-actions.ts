/**
 * What happens to a rendered share image: the native share sheet (Web Share
 * API level 2, with the PNG as a file), a download and a copied link. Each
 * function reports what actually happened so the dialog can say so.
 */
import type { CalculatorKind } from "@/lib/schemas/calculator";

export type ShareMethod = "native" | "download" | "copy_link";

/** File name of the downloaded image ("staryield-tarot-birth-card.png"). */
export function shareFileName(kind: CalculatorKind): string {
  return `staryield-${kind}.png`;
}

export function shareFile(blob: Blob, kind: CalculatorKind): File {
  return new File([blob], shareFileName(kind), { type: "image/png" });
}

/** The browser can hand this image to the share sheet (Web Share API level 2). */
export function canShareFile(file: File): boolean {
  if (typeof navigator === "undefined" || typeof navigator.share !== "function" || typeof navigator.canShare !== "function") return false;
  try {
    return navigator.canShare({ files: [file] });
  } catch {
    return false;
  }
}

export type NativeShareOutcome = "shared" | "cancelled";

/**
 * Open the share sheet with the image and the text. A dismissed sheet
 * (AbortError) is not an error: it reports "cancelled". Anything else is
 * thrown for the caller to show.
 */
export async function shareNative(input: { file: File; text: string; url: string; title: string }): Promise<NativeShareOutcome> {
  try {
    await navigator.share({ files: [input.file], text: input.text, url: input.url, title: input.title });
    return "shared";
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") return "cancelled";
    throw error;
  }
}

/** Save the image through a temporary link (the object URL is revoked right after the click). */
export function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  try {
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    link.rel = "noopener";
    document.body.appendChild(link);
    link.click();
    link.remove();
  } finally {
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
}

/** Copy the link; false when the browser refuses (insecure page, permission). */
export async function copyText(value: string): Promise<boolean> {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(value);
      return true;
    }
  } catch {
    // fall through to the legacy path
  }
  try {
    const field = document.createElement("textarea");
    field.value = value;
    field.setAttribute("readonly", "");
    field.style.position = "fixed";
    field.style.opacity = "0";
    document.body.appendChild(field);
    field.select();
    const ok = document.execCommand("copy");
    field.remove();
    return ok;
  } catch {
    return false;
  }
}
