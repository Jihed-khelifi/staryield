"use client"

import { FiInfo, FiMessageSquare } from "react-icons/fi"

import { useI18n } from "@/i18n/i18n-provider"
import type { Reader } from "@/lib/chatroom-data"
import { formatDuration } from "@/lib/format"
import { ReaderAvatar } from "@/components/chat/reader-avatar"
import { Button } from "@/components/ui/button"

type ChatHeaderProps = {
  reader: Reader
  elapsedSeconds: number
  onOpenInbox: () => void
  onOpenDetails: () => void
}

export function ChatHeader({
  reader,
  elapsedSeconds,
  onOpenInbox,
  onOpenDetails,
}: ChatHeaderProps) {
  const { t } = useI18n()

  return (
    <div className="flex items-center justify-between gap-3 bg-sage p-4">
      <div className="flex min-w-0 items-center gap-2 sm:gap-3">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("chatroom.inbox.openLabel")}
          onClick={onOpenInbox}
          className="text-ink hover:bg-cream/40 lg:hidden"
        >
          <FiMessageSquare className="size-4" aria-hidden />
        </Button>

        <ReaderAvatar initial={reader.initial} />

        <div className="flex min-w-0 flex-col gap-0.5 text-ink">
          <span className="truncate font-serif text-[18px] font-bold">
            {reader.name}
          </span>
          <span className="font-ui text-xs opacity-80">
            {t(`chatroom.status.${reader.status}`)}
          </span>
        </div>
      </div>

      <div className="flex items-center gap-2">
        <div className="flex flex-col items-end gap-0.5 text-ink">
          <button
            type="button"
            aria-label={t("chatroom.header.endChatLabel", { name: reader.name })}
            className="cursor-pointer rounded-sm font-serif text-base font-medium whitespace-nowrap transition-colors hover:text-cream-light focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
          >
            {t("chatroom.header.endChat")}
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
          className="text-ink hover:bg-cream/40 xl:hidden"
        >
          <FiInfo className="size-4" aria-hidden />
        </Button>
      </div>
    </div>
  )
}
