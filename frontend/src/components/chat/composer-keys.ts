/**
 * Composer keyboard behaviour.
 *
 * Desktop: Enter sends, Shift+Enter (or Alt+Enter) inserts a newline.
 * Touch devices: Enter inserts a newline and the send button sends, because
 * soft keyboards make an accidental "send" too easy. Ctrl/Cmd+Enter always
 * sends. Keys pressed while an IME composition is active never send.
 */
export type ComposerKey = {
  key: string;
  shiftKey: boolean;
  altKey?: boolean;
  ctrlKey?: boolean;
  metaKey?: boolean;
  isComposing?: boolean;
  keyCode?: number;
};

/** "send": submit and prevent the default; "newline"/"default": let the textarea handle it. */
export type ComposerKeyAction = "send" | "newline" | "default";

export function composerKeyAction(event: ComposerKey, touch: boolean): ComposerKeyAction {
  if (event.key !== "Enter") return "default";
  if (event.isComposing || event.keyCode === 229) return "default";
  if (event.ctrlKey || event.metaKey) return "send";
  if (event.shiftKey || event.altKey) return "newline";
  return touch ? "newline" : "send";
}

/** True on devices whose primary input is touch (no hover, coarse pointer). */
export function isTouchDevice(): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return false;
  return window.matchMedia("(hover: none) and (pointer: coarse)").matches;
}
