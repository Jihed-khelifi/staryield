"use client"

import * as React from "react"

import { useI18n } from "@/i18n/i18n-provider"
import type { ChatMessage } from "@/lib/chatroom-data"
import { MessageBubble } from "@/components/chat/message-bubble"
import { TypingBubble } from "@/components/chat/typing-bubble"
import { ScrollArea } from "@/components/ui/scroll-area"

export function ChatFeed({
  messages,
  authorName,
  typing,
}: {
  messages: ChatMessage[]
  authorName: string
  typing: boolean
}) {
  const { t } = useI18n()
  const endRef = React.useRef<HTMLDivElement>(null)

  React.useEffect(() => {
    endRef.current?.scrollIntoView({ block: "end" })
  }, [messages.length, typing])

  return (
    <ScrollArea className="min-h-0 flex-1">
      <ol
        aria-label={t("chatroom.feed.label", { name: authorName })}
        className="flex flex-col gap-3 p-4"
      >
        {messages.map((message) => (
          <MessageBubble
            key={message.id}
            message={message}
            authorName={authorName}
          />
        ))}
        {typing ? <TypingBubble authorName={authorName} /> : null}
      </ol>
      <div ref={endRef} />
    </ScrollArea>
  )
}
