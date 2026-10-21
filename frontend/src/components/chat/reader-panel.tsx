"use client";
import Link from "next/link";
import { useI18n } from "@/i18n/i18n-provider";
import type { ChatMessage, Participant, Psychic } from "@/lib/api/types";
import { formatRating } from "@/lib/format";
import { RatingStar } from "@/components/brand/rating-star";
import { Button } from "@/components/ui/button";
import { ChatImage } from "@/components/chat/chat-image";

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
      <SavedMessages saved={saved} onRemove={onRemove} />
    </div>
  );
}

/** Bookmarked messages of the open conversation (text and images). */
export function SavedMessages({ saved, onRemove }: { saved: ChatMessage[]; onRemove: (id: string) => void }) {
  const { text } = useI18n();
  return (
    <section>
      <h3 className="border-b border-gold/30 pb-2 font-display text-[9px] tracking-widest">{text("♧ Saved messages ")}</h3>
      <div className="mt-3 space-y-2">
        {saved.length ? (
          saved.map((message) => {
            const image = message.kind === "image";
            const label = image ? message.body.trim() || text("Photo") : message.body;
            return (
              <div key={message.id} className="rounded-lg bg-cream p-3">
                {image ? (
                  <div className="flex items-start gap-3">
                    <ChatImage attachment={message.attachments?.[0]} alt={label} caption={message.body || undefined} compact />
                    <p className="min-w-0 flex-1 text-xs break-words whitespace-pre-wrap">{label}</p>
                  </div>
                ) : (
                  <p className="text-xs break-words whitespace-pre-wrap">{message.body}</p>
                )}
                <button
                  type="button"
                  className="mt-2 text-[10px] text-ink/60 underline"
                  onClick={() => onRemove(message.id)}
                  aria-label={text("Remove saved message: {value0}", { value0: label.slice(0, 60) })}
                >
                  {text("Remove ")}
                </button>
              </div>
            );
          })
        ) : (
          <p className="text-xs font-light">{text("No saved messages yet. Use the bookmark beside a message to save it. ")}</p>
        )}
      </div>
    </section>
  );
}
