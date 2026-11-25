/**
 * Copy and share an invite link (growth: referrals). The Clipboard API needs
 * a secure context and may be refused; the fallback copies through a hidden
 * textarea. Web Share is used where the browser offers it (phones, Safari);
 * elsewhere sharing copies the link. Pure functions with injectable browser
 * objects, so every path is unit-tested.
 */

export type CopyMethod = "clipboard" | "fallback";

type ClipboardNavigator = { clipboard?: { writeText?: (value: string) => Promise<void> } };
type ShareNavigator = {
  share?: (data: ShareData) => Promise<void>;
  canShare?: (data: ShareData) => boolean;
};

/** Copy `value`; resolves with how it was copied, or null when nothing worked. */
export async function copyText(
  value: string,
  nav: ClipboardNavigator | undefined = typeof navigator !== "undefined" ? navigator : undefined,
  doc: Document | undefined = typeof document !== "undefined" ? document : undefined,
): Promise<CopyMethod | null> {
  if (nav?.clipboard?.writeText) {
    try {
      await nav.clipboard.writeText(value);
      return "clipboard";
    } catch {
      // Refused (no permission, insecure context): try the fallback.
    }
  }
  if (!doc?.body) return null;
  const area = doc.createElement("textarea");
  area.value = value;
  area.setAttribute("readonly", "");
  area.setAttribute("aria-hidden", "true");
  Object.assign(area.style, { position: "fixed", top: "0", left: "0", opacity: "0", pointerEvents: "none" });
  doc.body.appendChild(area);
  const active = doc.activeElement as HTMLElement | null;
  try {
    area.select();
    area.setSelectionRange(0, value.length);
    return typeof doc.execCommand === "function" && doc.execCommand("copy") ? "fallback" : null;
  } catch {
    return null;
  } finally {
    area.remove();
    // Give the focus back to the button that asked.
    active?.focus?.();
  }
}

/** Whether the browser can share this link natively. */
export function canWebShare(nav: ShareNavigator | undefined = typeof navigator !== "undefined" ? navigator : undefined, url = "https://staryield.net"): boolean {
  if (typeof nav?.share !== "function") return false;
  try {
    return typeof nav.canShare === "function" ? nav.canShare({ url }) : true;
  } catch {
    return false;
  }
}

export type ShareOutcome = { method: "web_share" | "copy"; result: "shared" | "cancelled" | "copied" | "failed" };

/**
 * Share with the native sheet when available; a dismissed sheet is not an
 * error. Without Web Share, or when it fails for another reason, the link is
 * copied instead.
 */
export async function shareInvite(
  data: { url: string; title: string; text: string },
  nav: (ShareNavigator & ClipboardNavigator) | undefined = typeof navigator !== "undefined" ? navigator : undefined,
  doc?: Document,
): Promise<ShareOutcome> {
  if (canWebShare(nav, data.url)) {
    try {
      await nav!.share!(data);
      return { method: "web_share", result: "shared" };
    } catch (err) {
      if (err instanceof Error && err.name === "AbortError") return { method: "web_share", result: "cancelled" };
      // NotAllowedError and the like: copy instead.
    }
  }
  const copied = await copyText(data.url, nav, doc ?? (typeof document !== "undefined" ? document : undefined));
  return { method: "copy", result: copied ? "copied" : "failed" };
}
