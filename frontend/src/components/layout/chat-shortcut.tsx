"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import * as React from "react";
import { MessageCircle } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { useSession } from "@/lib/auth/session-provider";
import { useChatUnread } from "@/lib/chat/chat-unread-provider";
import { cn } from "@/lib/utils";

/**
 * Header shortcut to the chatroom for signed-in clients and advisors, with
 * a badge for unread messages (1-99, then "99+") that follows the server
 * count live. The badge pops when the count grows (not under reduced motion).
 */
export function ChatShortcut({ className }: { className?: string }) {
  const { text, locale } = useI18n();
  const { status, user } = useSession();
  const pathname = usePathname();
  const { count } = useChatUnread();
  // Remount the badge (pop animation) when the count grows; adjusted during render.
  const [seen, setSeen] = React.useState({ count, pop: 0 });
  if (count !== seen.count) setSeen({ count, pop: count > seen.count ? seen.pop + 1 : seen.pop });

  if (status !== "authenticated" || !user) return null;
  const label =
    count > 0
      ? text(count === 1 ? "Chat, {count} unread message" : "Chat, {count} unread messages", { count: count > 99 ? "99+" : count })
      : text("Chat");
  const current = pathname?.startsWith(`/${locale}/chatroom`);
  return (
    <Link
      href={`/${locale}/chatroom`}
      aria-label={label}
      aria-current={current ? "page" : undefined}
      className={cn("notification-bell chat-shortcut", className)}
      data-testid="chat-shortcut"
    >
      <MessageCircle size={20} strokeWidth={1.5} aria-hidden />
      {count > 0 ? (
        <span key={seen.pop} className="notification-badge chat-shortcut-badge" aria-hidden data-testid="chat-unread-badge" data-pop={seen.pop > 0 || undefined}>
          {count > 99 ? "99+" : count}
        </span>
      ) : null}
    </Link>
  );
}
