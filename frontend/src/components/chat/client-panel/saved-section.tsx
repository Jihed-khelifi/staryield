"use client";

import * as React from "react";
import { Bookmark, X } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import type { ChatMessage } from "@/lib/api/types";
import type { PagedList } from "@/lib/use-paged-list";
import { ChatImage } from "@/components/chat/chat-image";
import { SavedPages } from "@/components/chat/saved-list";
import { PanelSection } from "@/components/chat/client-panel/panel-section";
import { SAVED_PREVIEW, sinceLabel } from "@/components/chat/client-panel/panel-helpers";
import { useNow } from "@/components/chat/client-panel/use-now";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";

function SavedCard({ message, now, onRemove }: { message: ChatMessage; now: number; onRemove: (id: string) => void }) {
  const { text, locale } = useI18n();
  const image = message.kind === "image";
  const label = image ? message.body.trim() || text("Photo") : message.body;
  // Saved messages carry the time they were sent, not when they were saved.
  const since = sinceLabel(message.created_at, locale, now);
  return (
    <li className="panel-card" data-testid="saved-message">
      <div className="flex items-start gap-2">
        {image ? <ChatImage attachment={message.attachments?.[0]} alt={label} caption={message.body || undefined} compact /> : null}
        <p className="panel-card-body min-w-0 flex-1">{image ? label : `“${message.body}”`}</p>
        <button
          type="button"
          className="panel-icon-button"
          onClick={() => onRemove(message.id)}
          aria-label={text("Remove saved message: {value0}", { value0: label.slice(0, 60) })}
        >
          <X aria-hidden className="size-3.5" />
        </button>
      </div>
      <p className="panel-card-meta">
        <time dateTime={message.created_at}>{since.justNow ? text("Sent just now") : text("Sent {value0}", { value0: since.relative })}</time>
      </p>
    </li>
  );
}

/**
 * "Saved messages" (Figma 1174:1858): two at first; "+ expand" shows the
 * rest of the page and, past five, Previous/Next pages.
 */
export function SavedSection({ saved, onRemove }: { saved: PagedList<ChatMessage>; onRemove: (id: string) => void }) {
  const { text } = useI18n();
  const now = useNow();
  const [open, setOpen] = React.useState(false);
  const label = text("Saved messages");
  return (
    <PanelSection title={label} icon={<Bookmark className="size-3" />} testId="client-saved">
      <SavedPages
        saved={saved}
        showPager={open}
        emptyClassName="panel-muted"
        render={(items) => {
          const rest = items.slice(SAVED_PREVIEW);
          const more = rest.length > 0 || saved.hasNext || saved.hasPrev;
          const card = (m: ChatMessage) => <SavedCard key={m.id} message={m} now={now} onRemove={onRemove} />;
          return (
            <Collapsible open={open} onOpenChange={setOpen}>
              <ul className="grid gap-2" aria-label={label}>
                {items.slice(0, SAVED_PREVIEW).map(card)}
              </ul>
              {more ? (
                <>
                  <CollapsibleContent>
                    {rest.length ? (
                      <ul className="mt-2 grid gap-2" aria-label={label}>
                        {rest.map(card)}
                      </ul>
                    ) : null}
                  </CollapsibleContent>
                  <CollapsibleTrigger className="panel-expand" data-testid="panel-expand">
                    {text(open ? "- collapse" : "+ expand")}
                  </CollapsibleTrigger>
                </>
              ) : null}
            </Collapsible>
          );
        }}
      />
    </PanelSection>
  );
}
