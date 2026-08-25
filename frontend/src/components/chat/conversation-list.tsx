"use client"

import { useI18n } from "@/i18n/i18n-provider"
import type { Conversation } from "@/lib/chatroom-data"
import { cn } from "@/lib/utils"
import { ReaderAvatar } from "@/components/chat/reader-avatar"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"

type ConversationListProps = {
  conversations: Conversation[]
  activeId: string
  onSelect: (id: string) => void
  className?: string
}

export function ConversationList({
  conversations,
  activeId,
  onSelect,
  className,
}: ConversationListProps) {
  const { t } = useI18n()

  return (
    <ScrollArea className={cn("h-full", className)}>
      <ul
        aria-label={t("chatroom.inbox.srHeading")}
        className="flex flex-col gap-2 p-3"
      >
        {conversations.map((conversation) => {
          const isActive = conversation.id === activeId

          return (
            <li key={conversation.id}>
              <button
                type="button"
                onClick={() => onSelect(conversation.id)}
                aria-current={isActive ? "true" : undefined}
                className={cn(
                  "flex w-full cursor-pointer items-center gap-3 rounded-lg p-3 text-left transition-colors focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
                  isActive ? "bg-sage" : "bg-transparent hover:bg-sage/40"
                )}
              >
                <ReaderAvatar initial={conversation.initial} />

                <span className="flex min-w-0 flex-1 flex-col gap-0.5 overflow-hidden text-ink">
                  <span className="block truncate font-serif text-base font-bold">
                    {conversation.name}
                  </span>
                  <span className="block w-full truncate font-ui text-xs opacity-70">
                    {t(conversation.previewKey)}
                  </span>
                </span>

                {conversation.unread > 0 ? (
                  <Badge
                    aria-label={t("chatroom.inbox.unread", {
                      count: conversation.unread,
                    })}
                    className="h-4 shrink-0 rounded-sm bg-deep-black px-1.5 py-0.5 font-ui text-[10px] font-bold text-white"
                  >
                    {conversation.unread}
                  </Badge>
                ) : null}
              </button>
            </li>
          )
        })}
      </ul>
    </ScrollArea>
  )
}
