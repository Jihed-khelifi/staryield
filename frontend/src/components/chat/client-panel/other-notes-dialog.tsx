"use client";

import * as React from "react";
import { NotebookText } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { notesApi } from "@/lib/api/endpoints";
import type { SharedNote } from "@/lib/api/types";
import { formatDate } from "@/lib/format";
import { usePagedList } from "@/lib/use-paged-list";
import { EmptyState } from "@/components/feedback/empty-state";
import { ErrorState } from "@/components/feedback/error-state";
import { Dialog, DialogContent, DialogDescription, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { CursorPager } from "@/components/ui/pagination";
import { SkeletonList } from "@/components/ui/skeleton";
import { NOTES_PAGE_SIZE, noteErrorMessage } from "@/components/chat/client-panel/panel-helpers";

function SharedNotesList({ conversationId }: { conversationId: string }) {
  const { text, locale } = useI18n();
  const list = usePagedList<SharedNote>((cursor, signal) => notesApi.shared(conversationId, cursor, NOTES_PAGE_SIZE, signal), [conversationId]);

  if (list.error) {
    return list.error.code === "forbidden" ? (
      <p className="p-6 text-sm">{text(noteErrorMessage(list.error))}</p>
    ) : (
      <ErrorState error={list.error} onRetry={list.reload} compact className="p-6" />
    );
  }
  if (list.loading) {
    return (
      <div className="p-6" role="status" aria-label={text("Loading notes…")}>
        <SkeletonList rows={3} />
      </div>
    );
  }
  if (!list.items.length && list.page === 1) {
    return (
      <EmptyState
        className="m-6"
        icon={<NotebookText />}
        title={text("No shared notes yet")}
        description={text("When other advisors share notes about this client, they appear here.")}
      />
    );
  }
  return (
    <>
      <ul className="other-notes" data-testid="other-notes">
        {list.items.map((note) => (
          <li key={note.id} className="other-note" data-testid="other-note">
            <p className="min-w-0 flex-1 text-sm whitespace-pre-wrap">{note.body}</p>
            <div className="shrink-0 text-right">
              <p className="other-note-author">{note.author.name}</p>
              <p className="text-[11px] font-light">
                <time dateTime={note.created_at}>{formatDate(note.created_at, locale)}</time>
              </p>
            </div>
          </li>
        ))}
      </ul>
      <CursorPager
        className="px-6 pb-4"
        page={list.page}
        hasPrev={list.hasPrev}
        hasNext={list.hasNext}
        onPrev={list.prev}
        onNext={list.next}
        loading={list.loading}
        label={text("Shared note pages")}
      />
    </>
  );
}

/**
 * "+ Check other advisor's notes on this user": notes other advisors chose
 * to share about this client (Figma 1174:2449). Loaded when opened.
 */
export function OtherNotesDialog({ conversationId }: { conversationId: string }) {
  const { text } = useI18n();
  const [open, setOpen] = React.useState(false);
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <button type="button" className="panel-expand" data-testid="other-notes-trigger">
          {text("+ Check other advisor's notes on this user")}
        </button>
      </DialogTrigger>
      <DialogContent className="advisor-dialog gap-0 p-0" data-testid="other-notes-dialog">
        <DialogTitle className="advisor-dialog-title">{text("Notes from other psychics")}</DialogTitle>
        <DialogDescription className="sr-only">{text("Notes other advisors shared about this client. Only advisors who have chatted with them can see these notes.")}</DialogDescription>
        {open ? <SharedNotesList conversationId={conversationId} /> : null}
      </DialogContent>
    </Dialog>
  );
}
