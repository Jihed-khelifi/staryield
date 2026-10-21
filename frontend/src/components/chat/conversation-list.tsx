"use client";

import { useI18n } from "@/i18n/i18n-provider";
import type { Conversation } from "@/lib/api/types";
import { cn } from "@/lib/utils";
import { ReaderAvatar } from "@/components/chat/reader-avatar";
import { systemCopy } from "@/components/chat/chat-model";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";

type ConversationListProps = {
  conversations: Conversation[];
  /** Advisors see each client's online/offline presence. */
  viewerRole?: "client" | "psychic";
  activeId: string | null;
  onSelect: (conversation: Conversation) => void;
  className?: string;
};

export function ConversationList({ conversations, viewerRole = "client", activeId, onSelect, className }: ConversationListProps) {
  const { t, text } = useI18n();

  if (conversations.length === 0) {
    return <p className="p-4 text-sm font-light">{t("chatroom.inbox.empty")}</p>;
  }

  return (
    <ScrollArea className={cn("conversation-scroll h-full", className)}>
      <ul aria-label={t("chatroom.inbox.srHeading")} className="flex flex-col gap-2 p-3">
        {conversations.map((conversation) => {
          const isActive = conversation.id === activeId;
          const last = conversation.last_message;
          const preview = last
            ? last.kind === "system"
              ? text(systemCopy(last.body))
              : last.kind === "image"
                ? `📷 ${last.body.trim() || text("Photo")}`
                : last.body
            : "";
          const live = conversation.open_session?.status;
          const status = viewerRole === "psychic" ? conversation.counterpart.status : undefined;
          const online = status ? status !== "offline" : null;
          return (
            <li key={conversation.id}>
              <button
                type="button"
                onClick={() => onSelect(conversation)}
                aria-current={isActive ? "true" : undefined}
                className={cn(
                  "flex w-full cursor-pointer items-center gap-3 rounded-lg p-3 text-left transition-colors focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
                  isActive ? "bg-sage" : "bg-transparent hover:bg-sage/40",
                )}
              >
                <span className="relative shrink-0">
                  <ReaderAvatar initial={conversation.counterpart.name.charAt(0).toUpperCase()} image={conversation.counterpart.avatar_url} />
                  {online !== null ? <span className="presence-dot presence-dot-avatar" data-online={online || undefined} aria-hidden /> : null}
                </span>
                <span className="flex min-w-0 flex-1 flex-col gap-0.5 overflow-hidden text-ink">
                  <span className="block truncate font-serif text-base font-bold">
                    {conversation.counterpart.name}
                    {online !== null ? <span className="sr-only">{` · ${text(online ? "Online" : "Offline")}`}</span> : null}
                  </span>
                  <span className="block w-full truncate font-ui text-xs opacity-70">
                    {live === "requested" ? text("New chat request") : live === "active" ? text("Live now") : preview}
                  </span>
                </span>
                {conversation.unread > 0 ? (
                  <Badge
                    aria-label={t("chatroom.inbox.unread", { count: conversation.unread })}
                    className="h-4 shrink-0 rounded-sm bg-deep-black px-1.5 py-0.5 font-ui text-[10px] font-bold text-white"
                  >
                    {conversation.unread}
                  </Badge>
                ) : null}
              </button>
            </li>
          );
        })}
      </ul>
    </ScrollArea>
  );
}
