"use client"

import { useI18n } from "@/i18n/i18n-provider"

/** The trailing "..." bubble from the design (Figma node 165:1308). */
export function TypingBubble({ authorName }: { authorName: string }) {
  const { t } = useI18n()

  return (
    <li className="flex w-full items-end">
      <div
        role="status"
        aria-label={t("chatroom.feed.typing", { name: authorName })}
        className="flex w-fit flex-col items-start rounded-lg bg-warm-gray p-3"
      >
        <span
          aria-hidden
          className="flex items-end gap-[3px] font-ui text-[13px] leading-[1.4] text-ink"
        >
          {[0, 1, 2].map((dot) => (
            <span
              key={dot}
              className="animate-pulse"
              style={{ animationDelay: `${dot * 180}ms` }}
            >
              .
            </span>
          ))}
        </span>
      </div>
    </li>
  )
}
