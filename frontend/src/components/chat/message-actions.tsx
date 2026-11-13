"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { FiBookmark, FiCopy, FiCornerUpLeft, FiHeart, FiMoreHorizontal } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import { cn } from "@/lib/utils";
import { announce } from "@/components/feedback/live-region";

/**
 * revisions 5.0 (§24.9): the three-dot button at the corner of a delivered
 * bubble. Icon only and quiet (CSS: `.message-menu-trigger`): top-left for the
 * viewer's own messages, top-right for received ones (logical properties, so
 * it mirrors). It opens {@link MessageMenu}; a long-press opens the same menu
 * at the finger.
 */
export const MessageMenuTrigger = React.forwardRef<
  HTMLButtonElement,
  /** onToggle: a click (or Enter/Space) opens or closes the menu; onOpen: the arrow keys only open it. */
  { mine: boolean; open: boolean; onOpen: () => void; onToggle?: () => void; className?: string }
>(function MessageMenuTrigger({ mine, open, onOpen, onToggle, className }, ref) {
  const { text } = useI18n();
  return (
    <button
      ref={ref}
      type="button"
      aria-label={text("Message actions")}
      aria-haspopup="menu"
      aria-expanded={open}
      onClick={onToggle ?? onOpen}
      onKeyDown={(e) => {
        // The menu-button pattern: the arrow keys open it too.
        if (e.key === "ArrowDown" || e.key === "ArrowUp") {
          e.preventDefault();
          onOpen();
        }
      }}
      // Touches on it are not a swipe, a double-tap or a long-press of the bubble.
      data-no-gesture
      data-testid="message-menu-trigger"
      data-mine={mine || undefined}
      className={cn("message-menu-trigger", className)}
    >
      <FiMoreHorizontal aria-hidden className="size-3.5" strokeWidth={1.5} />
    </button>
  );
});

type MenuItem = { id: string; label: string; icon: React.ReactNode; onSelect: () => void };

/** Where the menu opens: at a touch point (long-press) or under an element (the three-dot button). */
export type MenuAnchor = { x: number; y: number } | { element: HTMLElement; align: "start" | "end" };

const MARGIN = 8;
const GAP = 4;
const FINGER_GAP = 12;

/** Place a menu of `width` × `height`, kept inside the screen. */
export function menuPosition(anchor: MenuAnchor, width: number, height: number, viewport: { width: number; height: number }): { left: number; top: number } {
  const clamp = (value: number, max: number) => Math.max(MARGIN, Math.min(value, max));
  if ("element" in anchor) {
    const rect = anchor.element.getBoundingClientRect();
    const left = anchor.align === "end" ? rect.right - width : rect.left;
    // Under the button; above it when the screen ends there.
    const below = rect.bottom + GAP;
    const top = below + height + MARGIN > viewport.height ? rect.top - height - GAP : below;
    return { left: clamp(left, viewport.width - width - MARGIN), top: clamp(top, viewport.height - height - MARGIN) };
  }
  return { left: clamp(anchor.x - width / 2, viewport.width - width - MARGIN), top: clamp(anchor.y - height - FINGER_GAP, viewport.height - height - MARGIN) };
}

/**
 * The menu of a bubble: Reply, Bookmark / Remove bookmark, Like / Unlike and
 * Copy text. One component for the three-dot dropdown and the long-press
 * menu. Keyboard: arrows, Home and End move, Escape and Tab close and give
 * the focus back to the button that opened it.
 */
export function MessageMenu({
  anchor,
  liked,
  saved,
  hasText,
  copyText,
  onReply,
  onLike,
  onBookmark,
  onClose,
  returnFocusTo,
}: {
  anchor: MenuAnchor;
  liked: boolean;
  saved: boolean;
  hasText: boolean;
  copyText: string;
  onReply: () => void;
  onLike: () => void;
  onBookmark: () => void;
  onClose: () => void;
  /** Gets the focus back when the menu closes without moving it elsewhere (the three-dot button). */
  returnFocusTo?: HTMLElement | null;
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

  const icon = (Icon: typeof FiBookmark) => <Icon aria-hidden className="size-4" strokeWidth={1.5} />;
  const items: MenuItem[] = [
    { id: "reply", label: text("Reply"), icon: icon(FiCornerUpLeft), onSelect: onReply },
    { id: "bookmark", label: text(saved ? "Remove bookmark" : "Bookmark"), icon: icon(FiBookmark), onSelect: onBookmark },
    { id: "like", label: text(liked ? "Unlike" : "Like"), icon: icon(FiHeart), onSelect: onLike },
  ];
  if (hasText) items.push({ id: "copy", label: text("Copy text"), icon: icon(FiCopy), onSelect: copy });

  const place = React.useCallback(() => {
    const el = ref.current;
    if (!el) return;
    setPos(menuPosition(anchor, el.offsetWidth, el.offsetHeight, { width: window.innerWidth, height: window.innerHeight }));
  }, [anchor]);

  // Place the menu, kept inside the screen.
  React.useLayoutEffect(() => place(), [place]);

  // Focus the first item once the menu is placed: a menu that is still hidden cannot take the focus.
  const placed = pos !== null;
  React.useLayoutEffect(() => {
    if (placed) ref.current?.querySelector<HTMLElement>("[role=menuitem]")?.focus({ preventScroll: true });
  }, [placed]);

  // A menu under a button follows it when the feed scrolls (a new message can move it) or the window resizes.
  React.useEffect(() => {
    window.addEventListener("scroll", place, true);
    window.addEventListener("resize", place);
    return () => {
      window.removeEventListener("scroll", place, true);
      window.removeEventListener("resize", place);
    };
  }, [place]);

  const giveFocusBack = React.useCallback(() => returnFocusTo?.focus({ preventScroll: true }), [returnFocusTo]);

  // The three-dot button that opened the menu (none after a long-press).
  const opener = "element" in anchor ? anchor.element : null;
  React.useEffect(() => {
    const away = (e: Event) => {
      const target = e.target as Node;
      // The button toggles the menu itself; a press on it is not "away".
      if (opener?.contains(target)) return;
      if (ref.current && !ref.current.contains(target)) onClose();
    };
    const listen = () => {
      document.addEventListener("pointerdown", away, true);
      document.addEventListener("touchstart", away, true);
    };
    // After a long-press, the touchend that released it must not close the menu,
    // so a touch-opened menu listens a moment later; a button or keyboard opening
    // listens at once (a quick click elsewhere must close it).
    const timer = opener ? undefined : window.setTimeout(listen, 50);
    if (opener) listen();
    return () => {
      if (timer !== undefined) window.clearTimeout(timer);
      document.removeEventListener("pointerdown", away, true);
      document.removeEventListener("touchstart", away, true);
    };
  }, [onClose, opener]);

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape" || e.key === "Tab") {
      if (e.key === "Escape") e.preventDefault();
      onClose();
      giveFocusBack();
      return;
    }
    if (!["ArrowDown", "ArrowUp", "Home", "End"].includes(e.key)) return;
    e.preventDefault();
    const buttons = [...(ref.current?.querySelectorAll<HTMLElement>("[role=menuitem]") ?? [])];
    const index = buttons.indexOf(document.activeElement as HTMLElement);
    const next = e.key === "Home" ? 0 : e.key === "End" ? buttons.length - 1 : (index + (e.key === "ArrowDown" ? 1 : buttons.length - 1)) % buttons.length;
    buttons[next]?.focus();
  };

  return createPortal(
    <div
      ref={ref}
      role="menu"
      aria-label={text("Message actions")}
      aria-orientation="vertical"
      className="message-menu"
      data-testid="message-menu"
      data-anchor={"element" in anchor ? "button" : "point"}
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
            item.onSelect();
            // The action may have moved the focus (Reply goes to the composer): only take it back when it is still here.
            const stillHere = ref.current?.contains(document.activeElement) ?? false;
            onClose();
            if (stillHere) giveFocusBack();
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
