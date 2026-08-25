"use client"

import { useI18n } from "@/i18n/i18n-provider"
import type { Reader } from "@/lib/chatroom-data"
import { formatRating } from "@/lib/format"
import { cn } from "@/lib/utils"
import { RatingStar } from "@/components/brand/rating-star"
import { Button } from "@/components/ui/button"

/** Right-hand "detail-column" of the chatroom (Figma node 163:674). */
export function ReaderPanel({
  reader,
  className,
}: {
  reader: Reader
  className?: string
}) {
  const { locale, t } = useI18n()

  return (
    <div className={cn("flex flex-col items-center gap-4 pb-4", className)}>
      <div
        role="img"
        aria-label={t("chatroom.panel.portraitAlt", { name: reader.name })}
        className="flex h-[268px] w-full items-end justify-center bg-cream"
      >
        <span className="pb-3 font-serif text-[11px] text-ink/40">
          {t("chatroom.panel.portraitPending")}
        </span>
      </div>

      <p className="text-center font-serif text-xs font-light text-ink opacity-80">
        {t("chatroom.panel.freeMinutes", { count: reader.freeMinutes })}
      </p>

      <div className="flex w-[238px] max-w-full flex-col gap-1.5">
        <div className="flex flex-col">
          <h2 className="-mb-[7px] font-serif text-2xl font-medium text-ink">
            {reader.name}
          </h2>
          <p
            className="flex items-center gap-px font-serif text-sm text-ink"
            aria-label={t("chatroom.panel.ratingLabel", {
              rating: formatRating(reader.rating, locale),
            })}
          >
            <RatingStar />
            <span aria-hidden>{formatRating(reader.rating, locale)}</span>
          </p>
        </div>
        <p className="font-serif text-xs text-ink opacity-80">
          {t("chatroom.panel.experience", { count: reader.years })}
        </p>
      </div>

      <p className="w-[238px] max-w-full font-serif text-xs leading-[1.5] text-ink opacity-90">
        {t(reader.bioKey)}
      </p>

      <Button variant="gold" className="h-[30px] w-full px-4 text-[13px]">
        {t("chatroom.panel.viewProfile")}
      </Button>
    </div>
  )
}
