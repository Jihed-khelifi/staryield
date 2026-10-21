"use client";

import * as React from "react";
import { Popover } from "radix-ui";
import { Bell, CheckCheck, Clock, Coins, Megaphone, MessageCircle, MessageCircleX, ScrollText, Star, Wallet, type LucideIcon } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { errorMessage } from "@/lib/api/errors";
import type { AppNotification } from "@/lib/api/types";
import { notificationText, relativeTime } from "@/lib/notifications/notification-copy";
import { useNotifications } from "@/lib/notifications/notifications-provider";
import { useMediaQuery } from "@/lib/use-media-query";
import { cn } from "@/lib/utils";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";

const icons: Record<string, LucideIcon> = {
  "chat.requested": MessageCircle,
  "chat.missed": MessageCircleX,
  "chat.review_prompt": Star,
  "review.received": Star,
  "request.created": ScrollText,
  "request.updated": ScrollText,
  "purchase.succeeded": Coins,
  "credits.adjusted": Coins,
  "withdrawal.updated": Wallet,
  broadcast: Megaphone,
};

function useMinuteTick(active: boolean) {
  const [now, setNow] = React.useState(() => Date.now());
  React.useEffect(() => {
    if (!active) return;
    const id = window.setInterval(() => setNow(Date.now()), 60_000);
    return () => window.clearInterval(id);
  }, [active]);
  return now;
}

function NotificationRow({ notification, now, onOpen }: { notification: AppNotification; now: number; onOpen: () => void }) {
  const { text, locale } = useI18n();
  const copy = notificationText(notification, locale, text);
  const Icon = icons[notification.kind] ?? Bell;
  const unread = !notification.read_at;
  return (
    <li>
      <button type="button" className="notification-item" data-unread={unread || undefined} onClick={onOpen}>
        <span className="notification-icon" aria-hidden>
          <Icon size={16} strokeWidth={1.6} />
        </span>
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="notification-title">
            {unread ? <span className="sr-only">{text("Unread: ")}</span> : null}
            {copy.title}
          </span>
          {copy.body ? <span className="notification-body">{copy.body}</span> : null}
          <time className="notification-time" dateTime={notification.created_at}>
            {relativeTime(notification.created_at, locale, now)}
          </time>
        </span>
        {unread ? <span className="notification-dot" aria-hidden /> : null}
      </button>
    </li>
  );
}

/** The list shared by the desktop popover and the mobile sheet. */
function NotificationList({ onNavigate, active, showTitle = true }: { onNavigate: () => void; active: boolean; showTitle?: boolean }) {
  const { text } = useI18n();
  const n = useNotifications();
  const now = useMinuteTick(active);
  const sentinel = React.useRef<HTMLLIElement>(null);
  const { hasMore, loadingMore, loadMore } = n;

  // Infinite scroll: load the next page when the end of the list is visible.
  React.useEffect(() => {
    const el = sentinel.current;
    if (!el || !hasMore || typeof IntersectionObserver === "undefined") return;
    const io = new IntersectionObserver((entries) => {
      if (entries.some((e) => e.isIntersecting) && !loadingMore) void loadMore();
    });
    io.observe(el);
    return () => io.disconnect();
  }, [hasMore, loadingMore, loadMore]);

  return (
    <div className="notification-panel">
      <div className="notification-panel-header">
        {showTitle ? (
          <p className="font-display text-sm tracking-wide" aria-hidden>
            {text("Notifications")}
          </p>
        ) : (
          <span />
        )}
        <button type="button" className="notification-mark-all" onClick={() => void n.markAllRead()} disabled={n.unread === 0}>
          <CheckCheck size={14} aria-hidden />
          {text("Mark all as read")}
        </button>
      </div>
      <div className="notification-scroll">
        {n.error && n.items.length === 0 ? (
          <div className="p-4 text-sm" role="alert">
            <p>{text(errorMessage(n.error))}</p>
            <button type="button" className="mt-2 underline" onClick={n.refresh}>
              {text("Try again")}
            </button>
          </div>
        ) : n.loading && n.items.length === 0 ? (
          <p className="flex items-center gap-2 p-4 text-sm" role="status">
            <Clock size={14} aria-hidden /> {text("Loading…")}
          </p>
        ) : n.loaded && n.items.length === 0 ? (
          <p className="p-6 text-center text-sm font-light">{text("You're all caught up.")}</p>
        ) : (
          <ul aria-label={text("Notifications")} className="flex flex-col">
            {n.items.map((item) => (
              <NotificationRow
                key={item.id}
                notification={item}
                now={now}
                onOpen={() => {
                  n.open(item);
                  onNavigate();
                }}
              />
            ))}
            {n.hasMore ? (
              <li ref={sentinel} className="p-3 text-center">
                <button type="button" className="font-ui text-xs underline" onClick={() => void n.loadMore()} disabled={n.loadingMore}>
                  {text(n.loadingMore ? "Loading…" : "Load more")}
                </button>
              </li>
            ) : null}
          </ul>
        )}
      </div>
    </div>
  );
}

/** Bell with unread badge: popover on desktop, sheet on phones. */
export function NotificationBell({ className }: { className?: string }) {
  const { text } = useI18n();
  const n = useNotifications();
  const desktop = useMediaQuery("(min-width: 768px)");
  const [open, setOpen] = React.useState(false);
  const { ensureLoaded } = n;

  const onOpenChange = React.useCallback(
    (next: boolean) => {
      setOpen(next);
      if (next) ensureLoaded();
    },
    [ensureLoaded],
  );

  if (!n.enabled) return null;
  const count = n.unread;
  const label = count > 0 ? text("Notifications, {count} unread", { count }) : text("Notifications");
  const trigger = (
    <button
      type="button"
      aria-label={label}
      aria-haspopup="dialog"
      aria-expanded={open}
      className={cn("notification-bell", className)}
      onClick={desktop ? undefined : () => onOpenChange(!open)}
    >
      <Bell size={20} strokeWidth={1.5} aria-hidden />
      {count > 0 ? (
        <span className="notification-badge" aria-hidden data-testid="notification-badge">
          {count > 99 ? "99+" : count}
        </span>
      ) : null}
    </button>
  );

  if (desktop) {
    return (
      <Popover.Root open={open} onOpenChange={onOpenChange}>
        <Popover.Trigger asChild>{trigger}</Popover.Trigger>
        <Popover.Portal>
          <Popover.Content
            align="end"
            sideOffset={10}
            collisionPadding={12}
            aria-label={text("Notifications")}
            className="notification-popover z-50"
          >
            <NotificationList active={open} onNavigate={() => setOpen(false)} />
          </Popover.Content>
        </Popover.Portal>
      </Popover.Root>
    );
  }

  return (
    <>
      {trigger}
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent side="right" className="gap-0 bg-cream-light p-0 data-[side=right]:w-full data-[side=right]:sm:max-w-[420px]">
          <SheetHeader className="pb-0">
            <SheetTitle className="font-display">{text("Notifications")}</SheetTitle>
            <SheetDescription className="sr-only">{text("Your latest updates from Staryield.")}</SheetDescription>
          </SheetHeader>
          <NotificationList active={open} onNavigate={() => setOpen(false)} showTitle={false} />
        </SheetContent>
      </Sheet>
    </>
  );
}
