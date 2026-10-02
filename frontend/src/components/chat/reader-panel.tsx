"use client";
import Link from "next/link";
import { useI18n } from "@/i18n/i18n-provider";
import type { Reader } from "@/lib/chatroom-data";
import { RatingStar } from "@/components/brand/rating-star";
import { Button } from "@/components/ui/button";

export type SavedMessage = { id: string; body: string };
export function ReaderPanel({
  reader,
  saved = [],
  onRemove,
}: {
  reader: Reader;
  saved?: SavedMessage[];
  onRemove?: (id: string) => void;
}) {
  const { locale } = useI18n();
  return (
    <div className="flex flex-col gap-4 px-4 py-16">
      <div>
        <h2 className="font-display text-xl">{reader.name}</h2>
        <p className="flex items-center text-sm">
          <RatingStar />
          4.8
        </p>
        <p className="mt-1 text-xs font-light">12 years of experience</p>
      </div>
      <div>
        <h3 className="mb-2 border-b border-gold/30 pb-2 font-display text-[9px] tracking-widest">
          Overview
        </h3>
        <p className="text-sm leading-relaxed">
          With over a decade of spiritual insight, I channel ancient wisdom to
          help you navigate life’s greatest mysteries. Every question has an
          answer waiting to be revealed.
        </p>
      </div>
      <Button
        asChild
        variant="ghost"
        className="my-3 w-full font-display text-[11px]"
      >
        <Link href={`/${locale}/psychics/${reader.id}`}>
          View full profile ›
        </Link>
      </Button>
      <section>
        <h3 className="border-b border-gold/30 pb-2 font-display text-[9px] tracking-widest">
          ♧ Saved messages
        </h3>
        <div className="mt-3 space-y-2">
          {saved.length ? (
            saved.map((message) => (
              <div key={message.id} className="rounded-lg bg-cream p-3">
                <p className="text-xs">{message.body}</p>
                <button
                  className="mt-2 text-[10px] text-ink/60 underline"
                  onClick={() => onRemove?.(message.id)}
                  aria-label={`Remove saved message: ${message.body}`}
                >
                  Remove
                </button>
              </div>
            ))
          ) : (
            <p className="text-xs font-light">
              No saved messages yet. Use the bookmark beside a message to save
              it.
            </p>
          )}
        </div>
      </section>
    </div>
  );
}
