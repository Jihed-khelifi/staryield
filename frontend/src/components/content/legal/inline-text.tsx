import { Fragment } from "react";
import { locales, type Locale } from "@/i18n/config";

/**
 * The only inline markup content modules may use: `**bold**` and
 * `[label](href)`.
 */
const INLINE = /\*\*([^*]+)\*\*|\[([^\]]+)\]\(([^)\s]+)\)/g;

export type InlinePart =
  | { kind: "text"; value: string }
  | { kind: "bold"; value: string }
  | { kind: "link"; label: string; href: string };

export function parseInline(text: string): InlinePart[] {
  const parts: InlinePart[] = [];
  let last = 0;
  for (const match of text.matchAll(INLINE)) {
    const start = match.index ?? 0;
    if (start > last) parts.push({ kind: "text", value: text.slice(last, start) });
    if (match[1] !== undefined) parts.push({ kind: "bold", value: match[1] });
    else parts.push({ kind: "link", label: match[2], href: match[3] });
    last = start + match[0].length;
  }
  if (last < text.length) parts.push({ kind: "text", value: text.slice(last) });
  return parts;
}

/**
 * Internal paths get the reader's locale prefix unless they already carry
 * one; in-page anchors and mailto: are kept as-is. Anything else (absolute
 * URLs, protocol-relative URLs, javascript:) is refused and rendered as text.
 */
export function resolveHref(href: string, locale: Locale): string | null {
  if (/^#[a-z0-9-]+$/.test(href)) return href;
  if (/^mailto:[^\s@]+@[^\s@]+$/.test(href)) return href;
  if (href.startsWith("/") && !href.startsWith("//")) {
    const first = href.split(/[/?#]/)[1] ?? "";
    return (locales as readonly string[]).includes(first)
      ? href
      : `/${locale}${href}`;
  }
  return null;
}

export function InlineText({ text, locale }: { text: string; locale: Locale }) {
  return (
    <>
      {parseInline(text).map((part, index) => {
        if (part.kind === "text") return <Fragment key={index}>{part.value}</Fragment>;
        if (part.kind === "bold")
          return (
            <strong key={index} className="font-semibold">
              {part.value}
            </strong>
          );
        const href = resolveHref(part.href, locale);
        if (!href) return <Fragment key={index}>{part.label}</Fragment>;
        return (
          <a
            key={index}
            href={href}
            className="font-semibold text-[#6b4f1d] underline decoration-[#c29a3d] underline-offset-2 hover:text-ink focus-visible:rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#c29a3d]"
          >
            {part.label}
          </a>
        );
      })}
    </>
  );
}
