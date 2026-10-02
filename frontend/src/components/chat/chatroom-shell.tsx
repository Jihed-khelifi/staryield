"use client";
import * as React from "react";
import { useI18n } from "@/i18n/i18n-provider";
import {
  conversationThread,
  conversations,
  initialSessionSeconds,
  readers,
  type ChatMessage,
} from "@/lib/chatroom-data";
import threads from "@/data/chat-threads.json";
import { ChatFeed } from "@/components/chat/chat-feed";
import { ChatHeader } from "@/components/chat/chat-header";
import { ConversationList } from "@/components/chat/conversation-list";
import { MessageComposer } from "@/components/chat/message-composer";
import { ReaderPanel, type SavedMessage } from "@/components/chat/reader-panel";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";

export function ChatroomShell({ initialReader }: { initialReader?: string }) {
  const { t } = useI18n();
  const [activeId, setActiveId] = React.useState(
    initialReader && readers[initialReader]
      ? initialReader
      : conversations[0].id,
  );
  const [messagesByReader, setMessages] = React.useState<
    Record<string, ChatMessage[]>
  >({
    ramone: conversationThread,
    theo: threads.theo as ChatMessage[],
    solomon: threads.solomon as ChatMessage[],
  });
  const [elapsed, setElapsed] = React.useState(
    activeId === "theo"
      ? 927
      : activeId === "solomon"
        ? 521
        : initialSessionSeconds,
  );
  const [ended, setEnded] = React.useState(false);
  const [inboxOpen, setInboxOpen] = React.useState(false);
  const [detailsOpen, setDetailsOpen] = React.useState(false);
  const [notice, setNotice] = React.useState("");
  const [saved, setSaved] = React.useState<SavedMessage[]>([
    { id: "sample-1", body: '"Your path is lit by the stars."' },
    { id: "sample-2", body: '"Trust your intuition."' },
  ]);
  const reader = readers[activeId];
  const messages = messagesByReader[activeId];
  React.useEffect(() => {
    if (ended) return;
    const timer = window.setInterval(
      () => setElapsed((seconds) => seconds + 1),
      1000,
    );
    return () => window.clearInterval(timer);
  }, [ended]);
  function handleSend(body: string) {
    if (ended) return;
    setMessages((current) => ({
      ...current,
      [activeId]: [
        ...current[activeId],
        {
          id: `${activeId}-sent-${Date.now()}`,
          author: "visitor",
          body,
          sentAt: new Date().toISOString(),
        },
      ],
    }));
    setNotice(
      "Message added to this preview. Live delivery is unavailable until the chat service is connected.",
    );
  }
  function handleSelect(id: string) {
    setActiveId(id);
    setInboxOpen(false);
    setNotice("");
    setEnded(false);
    setElapsed(
      id === "theo" ? 927 : id === "solomon" ? 521 : initialSessionSeconds,
    );
  }
  function bookmark(message: ChatMessage) {
    const body = message.body ?? (message.bodyKey ? t(message.bodyKey) : "");
    setSaved((current) =>
      current.some((item) => item.id === message.id)
        ? current.filter((item) => item.id !== message.id)
        : [...current, { id: message.id, body }],
    );
  }
  const inbox = (
    <ConversationList
      conversations={conversations}
      activeId={activeId}
      onSelect={handleSelect}
    />
  );
  const panel = (
    <ReaderPanel
      reader={reader}
      saved={saved}
      onRemove={(id) =>
        setSaved((current) => current.filter((item) => item.id !== id))
      }
    />
  );
  return (
    <main
      aria-label="Chatroom"
      className="chat-workspace grid min-h-0 flex-1 gap-4 px-8 lg:grid-cols-[260px_minmax(0,1fr)] xl:grid-cols-[260px_minmax(0,1fr)_280px]"
    >
      <aside className="hidden min-h-0 overflow-hidden rounded-xl border border-border bg-[#d3cbb9] lg:block">
        {inbox}
      </aside>
      <section className="flex min-h-0 flex-col overflow-hidden rounded-xl border border-border bg-[#d3cbb9]">
        <ChatHeader
          reader={reader}
          elapsedSeconds={elapsed}
          onOpenInbox={() => setInboxOpen(true)}
          onOpenDetails={() => setDetailsOpen(true)}
          onEnd={() => setEnded(!ended)}
          ended={ended}
        />
        <ChatFeed
          messages={messages}
          authorName={reader.name}
          typing={!ended && activeId === "ramone"}
          savedIds={saved.map((item) => item.id)}
          onBookmark={bookmark}
        />
        <p className="px-4 text-xs" role="status">
          {ended ? "Session ended. Resume to continue." : notice}
        </p>
        <MessageComposer
          onSend={handleSend}
          disabled={ended}
          note={t("chatroom.composer.note")}
        />
      </section>
      <aside className="hidden min-h-0 overflow-y-auto rounded-xl border border-border bg-[#a9a185] xl:block">
        {panel}
      </aside>
      <Sheet open={inboxOpen} onOpenChange={setInboxOpen}>
        <SheetContent side="left" className="w-[280px] gap-0 bg-cream p-0">
          <SheetHeader className="pb-0">
            <SheetTitle>{t("chatroom.inbox.title")}</SheetTitle>
          </SheetHeader>
          <div className="min-h-0 flex-1">{inbox}</div>
        </SheetContent>
      </Sheet>
      <Sheet open={detailsOpen} onOpenChange={setDetailsOpen}>
        <SheetContent
          side="right"
          className="w-[300px] gap-0 overflow-y-auto bg-sage p-0"
        >
          <SheetHeader className="pb-0">
            <SheetTitle className="sr-only">{reader.name}</SheetTitle>
          </SheetHeader>
          {panel}
        </SheetContent>
      </Sheet>
    </main>
  );
}
