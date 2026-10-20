"use client";
import Link from "next/link";
import { useI18n } from "@/i18n/i18n-provider";
import type { ChatMessage, Participant, Psychic } from "@/lib/api/types";
import { formatRating } from "@/lib/format";
import { RatingStar } from "@/components/brand/rating-star";
import { Button } from "@/components/ui/button";

export function ReaderPanel({
  counterpart,
  psychic,
  saved,
  onRemove,
}: {
  counterpart: Participant;
  psychic: Psychic | null;
  saved: ChatMessage[];
  onRemove: (id: string) => void;
}) {
  const { text, locale } = useI18n();
  return (
    <div className="flex flex-col gap-4 px-4 py-16">
      <div>
        <h2 className="font-display text-xl">{text(counterpart.name)}</h2>
        {psychic ? (
          <>
            <p className="flex items-center text-sm">
              <RatingStar />
              {formatRating(psychic.rating, locale)}
            </p>
            <p className="mt-1 text-xs font-light">{text("{count} years of experience", { count: psychic.years_experience })}</p>
          </>
        ) : null}
      </div>
      {psychic ? (
        <>
          <div>
            <h3 className="mb-2 border-b border-gold/30 pb-2 font-display text-[9px] tracking-widest">{text("Overview ")}</h3>
            <p className="text-sm leading-relaxed">{text(psychic.headline || psychic.about[0] || "")}</p>
          </div>
          <Button asChild variant="ghost" className="my-3 w-full font-display text-[11px]">
            <Link href={`/${locale}/psychics/${psychic.slug}`}>{text("View full profile › ")}</Link>
          </Button>
        </>
      ) : null}
      <section>
        <h3 className="border-b border-gold/30 pb-2 font-display text-[9px] tracking-widest">{text("♧ Saved messages ")}</h3>
        <div className="mt-3 space-y-2">
          {saved.length ? (
            saved.map((message) => (
              <div key={message.id} className="rounded-lg bg-cream p-3">
                <p className="text-xs break-words whitespace-pre-line">{message.body}</p>
                <button
                  className="mt-2 text-[10px] text-ink/60 underline"
                  onClick={() => onRemove(message.id)}
                  aria-label={text("Remove saved message: {value0}", { value0: message.body.slice(0, 60) })}
                >
                  {text("Remove ")}
                </button>
              </div>
            ))
          ) : (
            <p className="text-xs font-light">{text("No saved messages yet. Use the bookmark beside a message to save it. ")}</p>
          )}
        </div>
      </section>
    </div>
  );
}
