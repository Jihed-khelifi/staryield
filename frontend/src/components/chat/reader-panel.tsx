"use client";
import Link from "next/link";
import { FiBookmark } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import type { ChatMessage, Participant, Psychic } from "@/lib/api/types";
import { formatRating } from "@/lib/format";
import type { PagedList } from "@/lib/use-paged-list";
import { RatingStar } from "@/components/brand/rating-star";
import { ChatImage } from "@/components/chat/chat-image";
import { SavedPages } from "@/components/chat/saved-list";
import { sinceLabel } from "@/components/chat/client-panel/panel-helpers";
import { useNow } from "@/components/chat/client-panel/use-now";

/** The advisor's details beside the client's chat (Figma 163:600, "Detail column"). */
export function ReaderPanel({
  counterpart,
  psychic,
  saved,
  onRemove,
}: {
  counterpart: Participant;
  psychic: Psychic | null;
  saved: PagedList<ChatMessage>;
  onRemove: (id: string) => void;
}) {
  const { text, locale } = useI18n();
  return (
    <div className="reader-panel" data-testid="reader-panel">
      <div className="reader-panel-head">
        <div className="flex items-center justify-between gap-3">
          <h2 className="reader-panel-name">{text(counterpart.name)}</h2>
          {psychic && psychic.reviews_count > 0 ? (
            <span className="reader-panel-rating">
              <RatingStar />
              {formatRating(psychic.rating, locale)}
            </span>
          ) : null}
        </div>
        {psychic ? <p className="reader-panel-meta">{text("{count} years of experience", { count: psychic.years_experience })}</p> : null}
      </div>
      {psychic ? (
        <>
          <section className="reader-panel-section" aria-labelledby="reader-overview">
            <h3 id="reader-overview" className="reader-panel-label">
              {text("Overview ")}
            </h3>
            <p className="reader-panel-body">{text(psychic.headline || psychic.about[0] || "")}</p>
          </section>
          <Link className="reader-panel-profile" href={`/${locale}/psychics/${psychic.slug}`}>
            {text("View full profile › ")}
          </Link>
        </>
      ) : null}
      <SavedMessages saved={saved} onRemove={onRemove} />
    </div>
  );
}

/** Bookmarked messages of the open conversation (text and images), five per page. */
export function SavedMessages({ saved, onRemove }: { saved: PagedList<ChatMessage>; onRemove: (id: string) => void }) {
  const { text, locale } = useI18n();
  const now = useNow();
  return (
    <section className="reader-saved" aria-labelledby="reader-saved-heading" data-testid="reader-saved">
      <h3 id="reader-saved-heading" className="panel-heading">
        <FiBookmark aria-hidden className="size-2.5" />
        <span>{text("Saved messages")}</span>
      </h3>
      <div className="mt-3">
        <SavedPages
          saved={saved}
          render={(items) => (
            <ul className="grid gap-2" aria-label={text("Saved messages")}>
              {items.map((message) => {
                const image = message.kind === "image";
                const label = image ? message.body.trim() || text("Photo") : message.body;
                const since = sinceLabel(message.created_at, locale, now);
                return (
                  <li key={message.id} className="reader-saved-card" data-testid="saved-message">
                    {image ? (
                      <div className="flex items-start gap-3">
                        <ChatImage attachment={message.attachments?.[0]} alt={label} caption={message.body || undefined} compact />
                        <p className="min-w-0 flex-1 text-xs break-words whitespace-pre-wrap">{label}</p>
                      </div>
                    ) : (
                      <p className="reader-saved-quote">{`“${message.body}”`}</p>
                    )}
                    <div className="mt-1 flex items-center justify-between gap-2">
                      <time className="reader-saved-meta" dateTime={message.created_at}>
                        {since.justNow ? text("Sent just now") : text("Sent {value0}", { value0: since.relative })}
                      </time>
                      <button
                        type="button"
                        className="reader-saved-remove"
                        onClick={() => onRemove(message.id)}
                        aria-label={text("Remove saved message: {value0}", { value0: label.slice(0, 60) })}
                      >
                        {text("Remove ")}
                      </button>
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        />
      </div>
    </section>
  );
}
