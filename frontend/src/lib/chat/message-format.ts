/**
 * Chat message text helpers. Message bodies are always rendered as React
 * text nodes; linkification only splits them into text and link tokens, so a
 * message can never inject markup. Only http(s) URLs (and `www.` hosts, which
 * become https) are linked.
 */

export type MessageToken = { type: "text"; value: string } | { type: "link"; value: string; href: string };

// A URL runs until whitespace or a character that cannot appear unescaped.
const URL_CANDIDATE = /(?:https?:\/\/|www\.)[^\s<>"`\u0000-\u001f\u007f]+/giu;
// A URL must not continue a word, an e-mail address or a path.
const WORD_BEFORE = /[\p{L}\p{N}@._\-/]/u;
const TRAILING = /[.,;:!?…'"*]$/u;
const PAIRS: Record<string, string> = { ")": "(", "]": "[", "}": "{" };

function count(text: string, ch: string): number {
  let n = 0;
  for (const c of text) if (c === ch) n++;
  return n;
}

/** Remove trailing punctuation that belongs to the sentence, not the URL. */
export function trimUrl(candidate: string): string {
  let url = candidate;
  for (;;) {
    const last = url.slice(-1);
    if (TRAILING.test(url)) {
      url = url.slice(0, -1);
      continue;
    }
    const open = PAIRS[last];
    if (open && count(url, last) > count(url, open)) {
      url = url.slice(0, -1);
      continue;
    }
    return url;
  }
}

/** The safe href for a URL candidate, or null when it is not a usable http(s) link. */
export function safeHref(url: string): string | null {
  const href = /^www\./i.test(url) ? `https://${url}` : url;
  try {
    const parsed = new URL(href);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return null;
    if (parsed.username || parsed.password) return null;
    const host = parsed.hostname;
    if (!host || (!host.includes(".") && host !== "localhost")) return null;
    if (host.startsWith(".") || host.endsWith(".")) return null;
    return parsed.href;
  } catch {
    return null;
  }
}

/** Split text into plain text and link tokens. Concatenating `value`s returns the input. */
export function linkify(text: string): MessageToken[] {
  const tokens: MessageToken[] = [];
  let last = 0;
  const push = (value: string) => {
    if (!value) return;
    const prev = tokens.at(-1);
    if (prev?.type === "text") prev.value += value;
    else tokens.push({ type: "text", value });
  };
  for (const match of text.matchAll(URL_CANDIDATE)) {
    const start = match.index ?? 0;
    if (start > 0 && WORD_BEFORE.test(text[start - 1])) continue;
    const url = trimUrl(match[0]);
    const href = safeHref(url);
    if (!href) continue;
    push(text.slice(last, start));
    tokens.push({ type: "link", value: url, href });
    last = start + url.length;
  }
  push(text.slice(last));
  return tokens;
}

// One emoji: a flag, a keycap, or a pictograph with modifiers/variation
// selectors/tags, optionally joined to others with ZWJ.
const EMOJI =
  /\p{Regional_Indicator}{2}|[0-9#*]️?⃣|\p{Extended_Pictographic}(?:️|\p{Emoji_Modifier}|[\u{E0020}-\u{E007F}])*(?:‍\p{Extended_Pictographic}(?:️|\p{Emoji_Modifier}|[\u{E0020}-\u{E007F}])*)*/gu;

/** Number of emoji when the text consists only of emoji (and whitespace), else 0. */
export function emojiOnlyCount(text: string): number {
  const compact = text.replace(/\s+/gu, "");
  if (!compact) return 0;
  let n = 0;
  const rest = compact.replace(EMOJI, () => {
    n++;
    return "";
  });
  return rest === "" ? n : 0;
}

/** Short emoji-only messages are shown slightly larger. */
export function isEmojiOnly(text: string, max = 8): boolean {
  const n = emojiOnlyCount(text);
  return n > 0 && n <= max;
}

/** Insert text at a selection, returning the new value and caret position. */
export function insertAtCaret(value: string, start: number, end: number, insert: string): { value: string; caret: number } {
  const s = Math.max(0, Math.min(start, value.length));
  const e = Math.max(s, Math.min(end, value.length));
  return { value: value.slice(0, s) + insert + value.slice(e), caret: s + insert.length };
}
