"use client";
/* eslint-disable @next/next/no-img-element */

import { FiInfo, FiMessageSquare } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import type { Reader } from "@/lib/chatroom-data";
import { formatDuration } from "@/lib/format";
import { ReaderAvatar } from "@/components/chat/reader-avatar";
import { Button } from "@/components/ui/button";

type ChatHeaderProps = {
  reader: Reader;
  elapsedSeconds: number;
  onOpenInbox: () => void;
  onOpenDetails: () => void;
  onEnd: () => void;
  ended: boolean;
};

export function ChatHeader({
  reader,
  elapsedSeconds,
  onOpenInbox,
  onOpenDetails,
  onEnd,
  ended,
}: ChatHeaderProps) {
  const { t } = useI18n();

  return (
    <div className="chat-header flex items-center justify-between gap-3 bg-sage p-4">
      <div className="flex min-w-0 items-center gap-2 sm:gap-3">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("chatroom.inbox.openLabel")}
          onClick={onOpenInbox}
          className="text-ink hover:bg-cream/40 max-md:hidden lg:hidden"
        >
          <FiMessageSquare className="size-4" aria-hidden />
        </Button>

        <button
          type="button"
          className="md:hidden"
          onClick={onOpenDetails}
          aria-label={`View ${reader.name}'s details`}
        >
          <img
            className="chat-reader-portrait"
            alt={reader.name}
            src={
              reader.id === "ramone"
                ? "https://assets.staryield.net/assets/figma/47a9f.png"
                : reader.id === "theo"
                  ? "https://assets.staryield.net/assets/figma/1f3a8.png"
                  : "https://assets.staryield.net/assets/figma/54c44.png"
            }
          />
        </button>
        <ReaderAvatar initial={reader.initial} className="hidden md:flex" />

        <div className="flex min-w-0 flex-col gap-0.5 text-ink">
          <span className="truncate font-serif text-[18px] font-bold">
            {reader.name}
          </span>
          <span className="chat-reader-status font-ui text-xs opacity-80">
            {t(`chatroom.status.${reader.status}`)}
            <span className="md:hidden">
              {" "}
              · {reader.id === "ramone"
                ? 40
                : reader.id === "theo"
                  ? 55
                  : 45}{" "}
              credits/min
            </span>
          </span>
        </div>
      </div>

      <div className="flex items-center gap-2">
        <div className="flex flex-col items-end gap-0.5 text-ink">
          <button
            type="button"
            onClick={onEnd}
            aria-label={
              ended
                ? `Resume chat with ${reader.name}`
                : t("chatroom.header.endChatLabel", { name: reader.name })
            }
            className="cursor-pointer rounded-sm font-serif text-base font-medium whitespace-nowrap transition-colors hover:text-cream-light focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
          >
            {ended ? "Resume chat" : t("chatroom.header.endChat")}
          </button>
          <span
            aria-label={t("chatroom.header.elapsed")}
            className="font-ui text-xs tabular-nums opacity-80"
          >
            {formatDuration(elapsedSeconds)}
          </span>
        </div>

        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("chatroom.header.openDetails")}
          onClick={onOpenDetails}
          className="text-ink hover:bg-cream/40 max-md:hidden xl:hidden"
        >
          <FiInfo className="size-4" aria-hidden />
        </Button>
      </div>
    </div>
  );
}
