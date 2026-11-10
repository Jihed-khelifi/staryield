"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { FiBookmark, FiCopy, FiCornerUpLeft, FiHeart } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import { cn } from "@/lib/utils";
import { announce } from "@/components/feedback/live-region";
import { MessageBookmark } from "@/components/chat/message-bookmark";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

/** One thin-outline action button beside a bubble (same visual language as the bookmark). */
function ActionButton({
  label,
  onClick,
  pressed,
  className,
  testId,
  children,
}: {
  label: string;
  onClick: () => void;
  pressed?: boolean;
  className?: string;
  testId: string;
  children: React.ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label={label}
          aria-pressed={pressed}
          onClick={onClick}
          data-testid={testId}
          className={cn(
            "message-action -m-1 flex shrink-0 cursor-pointer items-center justify-center rounded-sm p-1 focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
            className,
          )}
        >
          {children}
        </button>
      </TooltipTrigger>
      <TooltipContent side="top">{label}</TooltipContent>
    </Tooltip>
  );
}

/**
 * Reply, like and bookmark beside a bubble. Reply and like appear on hover and
 * focus (CSS: `.message-actions`), the bookmark stays as it was; on touch
 * screens the gestures and the long-press menu replace them.
 */
export function MessageActionButtons({
  mine,
  actionable,
  liked,
  saved,
  onReply,
  onLike,
  onBookmark,
}: {
  mine: boolean;
  /** Delivered text/image messages: reply and like exist. */
  actionable: boolean;
  liked: boolean;
  saved: boolean;
  onReply: () => void;
  onLike: () => void;
  onBookmark?: () => void;
}) {
  const { text } = useI18n();
  const reply = actionable ? (
    <ActionButton key="reply" label={text("Reply")} onClick={onReply} testId="message-reply" className="message-action-reply">
      <FiCornerUpLeft aria-hidden className="size-3.5" strokeWidth={1.5} />
    </ActionButton>
  ) : null;
  const like = actionable ? (
    <ActionButton key="like" label={text(liked ? "Unlike" : "Like")} onClick={onLike} pressed={liked} testId="message-like" className="message-action-like">
      <FiHeart aria-hidden className="size-3.5" strokeWidth={1.5} />
    </ActionButton>
  ) : null;
  const bookmark = <MessageBookmark key="bookmark" saved={saved} onToggle={onBookmark} />;
  return (
    <span className="message-actions flex shrink-0 items-center gap-2" data-mine={mine || undefined}>
      {mine ? [reply, like, bookmark] : [bookmark, like, reply]}
    </span>
  );
}

type MenuItem = { id: string; label: string; icon: React.ReactNode; onSelect: () => void };

/** The long-press menu of a bubble (touch): Reply, Like/Unlike, Save/Unsave, Copy text. */
export function MessageMenu({
  at,
  liked,
  saved,
  hasText,
  actionable,
  onReply,
  onLike,
  onBookmark,
  copyText,
  onClose,
}: {
  at: { x: number; y: number };
  liked: boolean;
  saved: boolean;
  hasText: boolean;
  actionable: boolean;
  onReply: () => void;
  onLike: () => void;
  onBookmark: () => void;
  copyText: string;
  onClose: () => void;
}) {
  const { text } = useI18n();
  const ref = React.useRef<HTMLDivElement>(null);
  const [pos, setPos] = React.useState<{ left: number; top: number } | null>(null);

  const copy = () => {
    const clipboard = typeof navigator !== "undefined" ? navigator.clipboard : undefined;
    if (!clipboard?.writeText) {
      announce(text("Couldn't copy the message."), "assertive");
      return;
    }
    clipboard.writeText(copyText).then(
      () => announce(text("Message copied")),
      () => announce(text("Couldn't copy the message."), "assertive"),
    );
  };

  const items: MenuItem[] = [];
  if (actionable) {
    items.push({ id: "reply", label: text("Reply"), icon: <FiCornerUpLeft aria-hidden className="size-4" strokeWidth={1.5} />, onSelect: onReply });
    items.push({ id: "like", label: text(liked ? "Unlike" : "Like"), icon: <FiHeart aria-hidden className="size-4" strokeWidth={1.5} />, onSelect: onLike });
  }
  items.push({ id: "save", label: text(saved ? "Unsave" : "Save"), icon: <FiBookmark aria-hidden className="size-4" strokeWidth={1.5} />, onSelect: onBookmark });
  if (hasText) items.push({ id: "copy", label: text("Copy text"), icon: <FiCopy aria-hidden className="size-4" strokeWidth={1.5} />, onSelect: copy });

  // Place the menu at the finger, kept inside the screen; focus the first item.
  React.useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const margin = 8;
    const left = Math.max(margin, Math.min(at.x - el.offsetWidth / 2, window.innerWidth - el.offsetWidth - margin));
    const top = Math.max(margin, Math.min(at.y - el.offsetHeight - 12, window.innerHeight - el.offsetHeight - margin));
    setPos({ left, top });
    el.querySelector<HTMLElement>("[role=menuitem]")?.focus({ preventScroll: true });
  }, [at]);

  React.useEffect(() => {
    const away = (e: Event) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    // A short delay: the touchend that released the long-press must not close it.
    const timer = window.setTimeout(() => {
      document.addEventListener("pointerdown", away, true);
      document.addEventListener("touchstart", away, true);
    }, 50);
    return () => {
      window.clearTimeout(timer);
      document.removeEventListener("pointerdown", away, true);
      document.removeEventListener("touchstart", away, true);
    };
  }, [onClose]);

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") {
      e.preventDefault();
      onClose();
      return;
    }
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    e.preventDefault();
    const buttons = [...(ref.current?.querySelectorAll<HTMLElement>("[role=menuitem]") ?? [])];
    const index = buttons.indexOf(document.activeElement as HTMLElement);
    buttons[(index + (e.key === "ArrowDown" ? 1 : buttons.length - 1)) % buttons.length]?.focus();
  };

  return createPortal(
    <div
      ref={ref}
      role="menu"
      aria-label={text("Message actions")}
      className="message-menu"
      data-testid="message-menu"
      style={pos ?? { left: 0, top: 0, visibility: "hidden" }}
      onKeyDown={onKeyDown}
    >
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          role="menuitem"
          className="message-menu-item"
          data-testid={`message-menu-${item.id}`}
          onClick={() => {
            onClose();
            item.onSelect();
          }}
        >
          {item.icon}
          <span>{item.label}</span>
        </button>
      ))}
    </div>,
    document.body,
  );
}
