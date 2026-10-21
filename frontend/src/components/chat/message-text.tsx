"use client";

import { cn } from "@/lib/utils";
import { isEmojiOnly, linkify } from "@/lib/chat/message-format";

/**
 * A message body rendered as text: spaces and newlines are preserved
 * (`white-space: pre-wrap`), http(s) and `www.` URLs become links, and short
 * emoji-only messages are shown slightly larger. Content is never parsed as
 * HTML: tokens are React text nodes and anchors with a validated href.
 */
export function MessageText({ body, className }: { body: string; className?: string }) {
  const emoji = isEmojiOnly(body);
  return (
    <p className={cn("message-text", emoji && "message-text-emoji", className)} data-emoji-only={emoji || undefined}>
      {linkify(body).map((token, i) =>
        token.type === "link" ? (
          <a key={i} href={token.href} target="_blank" rel="noopener noreferrer nofollow ugc" className="message-link">
            {token.value}
          </a>
        ) : (
          token.value
        ),
      )}
    </p>
  );
}
