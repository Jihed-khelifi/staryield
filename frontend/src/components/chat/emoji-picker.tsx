"use client";

import * as React from "react";
import { Popover } from "radix-ui";
import { FiSmile } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import { cn } from "@/lib/utils";

/** A small, dependency-free set of common emoji (screen readers announce each glyph's name). */
export const EMOJI_CATEGORIES: { id: string; label: string; icon: string; emoji: string[] }[] = [
  {
    id: "smileys",
    label: "Smileys",
    icon: "😊",
    emoji: "😀 😃 😄 😁 😆 😅 😂 🤣 🙂 😊 😇 🥰 😍 🤩 😘 😚 😋 😜 🤪 🤗 🤭 🤔 🤨 😐 😶 🙄 😏 😌 😔 😴 🥺 😢 😭 😤 😳 😱 😬 😮 🥲 😎".split(" "),
  },
  {
    id: "hearts",
    label: "Hearts",
    icon: "❤️",
    emoji: "❤️ 🧡 💛 💚 💙 💜 🤎 🖤 🤍 💖 💗 💓 💞 💕 💘 💝 💟 ❣️ 💔 ❤️‍🔥 ❤️‍🩹 💋 💌 🫶".split(" "),
  },
  {
    id: "gestures",
    label: "Gestures",
    icon: "🙏",
    emoji: "🙏 👍 👎 👏 🙌 👐 🤲 🤝 👋 🤞 ✌️ 🤟 👌 🤌 👉 👈 👆 👇 ☝️ ✋ 🖐️ 💪 🫡 🙇".split(" "),
  },
  {
    id: "mystic",
    label: "Mystic",
    icon: "🔮",
    emoji: "🔮 ✨ 🌟 ⭐ 💫 🌙 🌛 🌕 🌑 ☀️ 🌞 🪐 🌌 🌠 ☄️ 🃏 🕯️ 🧿 🪬 💎 🗝️ 📿 ♈ ♉ ♊ ♋ ♌ ♍ ♎ ♏ ♐ ♑ ♒ ♓".split(" "),
  },
  {
    id: "nature",
    label: "Nature",
    icon: "🌸",
    emoji: "🌸 🌹 🌺 🌻 🌼 🌷 🪷 🍀 🌿 🌱 🍃 🍂 🌳 🌊 🔥 🌈 ☁️ ⚡ ❄️ 🦋 🐝 🕊️ 🐈 🦉".split(" "),
  },
  {
    id: "objects",
    label: "Objects",
    icon: "🎁",
    emoji: "🎁 🎉 🎊 🍵 ☕ 🍷 🥂 📚 📖 ✍️ 📝 💌 📅 ⏳ 🔔 🎶 🧘 💤 💡 🏠 ✈️ 🧭 💰 🍀".split(" "),
  },
];

const RECENT_KEY = "staryield.emoji.recent";
const RECENT_MAX = 24;
const COLUMNS = 8;

function readRecent(): string[] {
  try {
    const v = JSON.parse(localStorage.getItem(RECENT_KEY) ?? "[]");
    return Array.isArray(v) ? v.filter((x): x is string => typeof x === "string").slice(0, RECENT_MAX) : [];
  } catch {
    return [];
  }
}

function writeRecent(list: string[]) {
  try {
    localStorage.setItem(RECENT_KEY, JSON.stringify(list.slice(0, RECENT_MAX)));
  } catch {
    // Recent emoji are a convenience only.
  }
}

/** Move focus within a grid of buttons with the arrow keys (roving tabindex). */
export function nextGridIndex(index: number, key: string, total: number, columns = COLUMNS): number {
  if (total <= 0) return 0;
  switch (key) {
    case "ArrowRight":
      return Math.min(total - 1, index + 1);
    case "ArrowLeft":
      return Math.max(0, index - 1);
    case "ArrowDown":
      return index + columns < total ? index + columns : index;
    case "ArrowUp":
      return index - columns >= 0 ? index - columns : index;
    case "Home":
      return index - (index % columns);
    case "End":
      return Math.min(total - 1, index - (index % columns) + columns - 1);
    default:
      return index;
  }
}

export function EmojiPicker({
  onSelect,
  disabled,
  onClose,
}: {
  onSelect: (emoji: string) => void;
  disabled?: boolean;
  /** Called when the picker closes (focus returns to the composer). */
  onClose?: () => void;
}) {
  const { text } = useI18n();
  const [open, setOpen] = React.useState(false);
  const [recent, setRecent] = React.useState<string[]>([]);
  const [tab, setTab] = React.useState<string>("smileys");
  const [focusIndex, setFocusIndex] = React.useState(0);
  const gridRef = React.useRef<HTMLDivElement>(null);
  const tabsRef = React.useRef<HTMLDivElement>(null);

  const categories = React.useMemo(
    () => (recent.length ? [{ id: "recent", label: "Recent", icon: "🕘", emoji: recent }, ...EMOJI_CATEGORIES] : EMOJI_CATEGORIES),
    [recent],
  );
  const current = categories.find((c) => c.id === tab) ?? categories[0];

  function onOpenChange(next: boolean) {
    if (next) {
      const r = readRecent();
      setRecent(r);
      setTab(r.length ? "recent" : "smileys");
      setFocusIndex(0);
    }
    setOpen(next);
  }

  function pick(emoji: string) {
    onSelect(emoji);
    const next = [emoji, ...readRecent().filter((e) => e !== emoji)].slice(0, RECENT_MAX);
    writeRecent(next);
  }

  function focusCell(index: number) {
    setFocusIndex(index);
    const btn = gridRef.current?.querySelectorAll<HTMLButtonElement>("button")[index];
    btn?.focus();
  }

  function onGridKey(e: React.KeyboardEvent) {
    if (!["ArrowRight", "ArrowLeft", "ArrowDown", "ArrowUp", "Home", "End"].includes(e.key)) return;
    e.preventDefault();
    focusCell(nextGridIndex(focusIndex, e.key, current.emoji.length));
  }

  function onTabKey(e: React.KeyboardEvent, index: number) {
    if (e.key !== "ArrowRight" && e.key !== "ArrowLeft") return;
    e.preventDefault();
    const next = (index + (e.key === "ArrowRight" ? 1 : -1) + categories.length) % categories.length;
    setTab(categories[next].id);
    setFocusIndex(0);
    tabsRef.current?.querySelectorAll<HTMLButtonElement>("[role=tab]")[next]?.focus();
  }

  return (
    <Popover.Root open={open} onOpenChange={onOpenChange}>
      <Popover.Trigger asChild>
        <button type="button" className="composer-tool" aria-label={text("Insert emoji")} disabled={disabled} data-testid="emoji-trigger">
          <FiSmile className="size-[18px]" aria-hidden />
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          side="top"
          align="start"
          sideOffset={8}
          collisionPadding={8}
          className="emoji-picker z-50"
          aria-label={text("Emoji")}
          onCloseAutoFocus={(e) => {
            if (onClose) {
              e.preventDefault();
              onClose();
            }
          }}
        >
          <div role="tablist" aria-label={text("Emoji categories")} className="emoji-tabs" ref={tabsRef}>
            {categories.map((c, i) => (
              <button
                key={c.id}
                type="button"
                role="tab"
                id={`emoji-tab-${c.id}`}
                aria-selected={c.id === current.id}
                aria-controls="emoji-panel"
                aria-label={text(c.label)}
                title={text(c.label)}
                tabIndex={c.id === current.id ? 0 : -1}
                className="emoji-tab"
                onClick={() => {
                  setTab(c.id);
                  setFocusIndex(0);
                }}
                onKeyDown={(e) => onTabKey(e, i)}
              >
                <span aria-hidden>{c.icon}</span>
              </button>
            ))}
          </div>
          <div id="emoji-panel" role="tabpanel" aria-labelledby={`emoji-tab-${current.id}`}>
            <p className="emoji-heading">{text(current.label)}</p>
            <div ref={gridRef} role="group" aria-label={text(current.label)} className="emoji-grid" onKeyDown={onGridKey}>
              {current.emoji.map((emoji, i) => (
                <button
                  key={`${current.id}-${emoji}-${i}`}
                  type="button"
                  className={cn("emoji-cell")}
                  tabIndex={i === Math.min(focusIndex, current.emoji.length - 1) ? 0 : -1}
                  onFocus={() => setFocusIndex(i)}
                  onClick={() => pick(emoji)}
                >
                  {emoji}
                </button>
              ))}
            </div>
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
