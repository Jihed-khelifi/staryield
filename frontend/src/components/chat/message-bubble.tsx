"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import type { ChatMessage } from "@/lib/chatroom-data";
import { formatMessageTime } from "@/lib/format";
import { cn } from "@/lib/utils";
import { MessageBookmark } from "@/components/chat/message-bookmark";
import { Button } from "@/components/ui/button";

export function MessageBubble({
  message,
  authorName,
  saved = false,
  onBookmark,
}: {
  message: ChatMessage;
  authorName: string;
  saved?: boolean;
  onBookmark?: () => void;
}) {
  const { locale, t } = useI18n();
  const [revealed, setRevealed] = React.useState(false);

  const isVisitor = message.author === "visitor";
  const isHidden = Boolean(message.locked) && !revealed;
  const time = formatMessageTime(message.sentAt, locale);

  const bubble = (
    <div
      className={cn(
        "message-bubble flex w-fit max-w-[85%] flex-col gap-2 rounded-lg bg-cream p-3 text-ink sm:max-w-[380px]",
        message.locked && "items-center",
      )}
    >
      <p
        className={cn(
          "w-full font-serif text-[13px] leading-[1.4] whitespace-pre-line",
          isHidden && "blur-[4px] select-none",
        )}
        aria-hidden={isHidden}
      >
        {message.body ?? (message.bodyKey ? t(message.bodyKey) : "")}
      </p>

      {isHidden ? (
        <>
          <span className="sr-only">{t("chatroom.feed.lockedAnswer")}</span>
          <Button
            variant="gold"
            onClick={() => setRevealed(true)}
            className="h-auto w-full px-4 py-2 text-[12px]"
          >
            {t("chatroom.feed.revealAnswer")}
          </Button>
        </>
      ) : null}

      <time
        dateTime={message.sentAt}
        className="w-full text-right font-ui text-[10px] opacity-60"
      >
        {time}
      </time>
    </div>
  );

  return (
    <li
      className={cn(
        "flex w-full items-end gap-[5px]",
        isVisitor && "justify-end",
      )}
    >
      <span className="sr-only">
        {isVisitor ? t("chatroom.feed.you") : authorName}
      </span>
      {isVisitor ? (
        <>
          <MessageBookmark saved={saved} onToggle={onBookmark} />
          {bubble}
        </>
      ) : (
        <>
          {bubble}
          <MessageBookmark saved={saved} onToggle={onBookmark} />
        </>
      )}
    </li>
  );
}
