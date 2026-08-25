"use client"

import * as React from "react"

import { useI18n } from "@/i18n/i18n-provider"
import {
  conversationThread,
  conversations,
  initialSessionSeconds,
  readers,
  type ChatMessage,
} from "@/lib/chatroom-data"
import { ChatFeed } from "@/components/chat/chat-feed"
import { ChatHeader } from "@/components/chat/chat-header"
import { ConversationList } from "@/components/chat/conversation-list"
import { MessageComposer } from "@/components/chat/message-composer"
import { ReaderPanel } from "@/components/chat/reader-panel"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"

export function ChatroomShell() {
  const { t } = useI18n()

  const [activeId, setActiveId] = React.useState(conversations[0].id)
  const [messages, setMessages] = React.useState<ChatMessage[]>(conversationThread)
  const [elapsed, setElapsed] = React.useState(initialSessionSeconds)
  const [inboxOpen, setInboxOpen] = React.useState(false)
  const [detailsOpen, setDetailsOpen] = React.useState(false)

  const reader = readers[activeId]

  React.useEffect(() => {
    const timer = window.setInterval(
      () => setElapsed((seconds) => seconds + 1),
      1000
    )
    return () => window.clearInterval(timer)
  }, [])

  function handleSend(body: string) {
    setMessages((current) => [
      ...current,
      {
        id: `sent-${current.length}`,
        author: "visitor",
        body,
        sentAt: new Date().toISOString(),
      },
    ])
  }

  function handleSelect(id: string) {
    setActiveId(id)
    setInboxOpen(false)
  }

  const inbox = (
    <ConversationList
      conversations={conversations}
      activeId={activeId}
      onSelect={handleSelect}
    />
  )

  return (
    <div className="grid min-h-0 flex-1 gap-4 lg:grid-cols-[260px_minmax(0,1fr)] xl:grid-cols-[260px_minmax(0,1fr)_280px]">
      <aside className="hidden min-h-0 overflow-hidden rounded-xl border border-border bg-cream lg:block">
        {inbox}
      </aside>

      <section className="flex min-h-0 flex-col overflow-hidden rounded-xl border border-border bg-cream">
        <ChatHeader
          reader={reader}
          elapsedSeconds={elapsed}
          onOpenInbox={() => setInboxOpen(true)}
          onOpenDetails={() => setDetailsOpen(true)}
        />
        <ChatFeed messages={messages} authorName={reader.name} typing />
        <MessageComposer
          onSend={handleSend}
          note={t("chatroom.composer.note")}
        />
      </section>

      <aside className="hidden min-h-0 overflow-y-auto rounded-xl border border-border bg-sage xl:block">
        <ReaderPanel reader={reader} />
      </aside>

      {/* Inbox as a drawer below lg */}
      <Sheet open={inboxOpen} onOpenChange={setInboxOpen}>
        <SheetContent side="left" className="w-[280px] gap-0 bg-cream p-0">
          <SheetHeader className="pb-0">
            <SheetTitle className="font-serif text-base font-bold">
              {t("chatroom.inbox.title")}
            </SheetTitle>
          </SheetHeader>
          <div className="min-h-0 flex-1">{inbox}</div>
        </SheetContent>
      </Sheet>

      {/* Reader details as a drawer below xl */}
      <Sheet open={detailsOpen} onOpenChange={setDetailsOpen}>
        <SheetContent
          side="right"
          className="w-[300px] gap-0 overflow-y-auto bg-sage p-0"
        >
          <SheetHeader className="pb-0">
            <SheetTitle className="sr-only">
              {t("chatroom.header.detailsTitle")}
            </SheetTitle>
          </SheetHeader>
          <ReaderPanel reader={reader} />
        </SheetContent>
      </Sheet>
    </div>
  )
}
