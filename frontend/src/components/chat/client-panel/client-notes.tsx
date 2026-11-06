"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { notesApi } from "@/lib/api/endpoints";
import { toApiError } from "@/lib/api/errors";
import type { ChatSession, Note } from "@/lib/api/types";
import { useRealtimeEvent } from "@/lib/auth/session-provider";
import { usePagedList } from "@/lib/use-paged-list";
import { useToast } from "@/components/feedback/toast-provider";
import { ErrorState } from "@/components/feedback/error-state";
import { CursorPager } from "@/components/ui/pagination";
import { SkeletonList } from "@/components/ui/skeleton";
import { PanelSection } from "@/components/chat/client-panel/panel-section";
import { NoteCard } from "@/components/chat/client-panel/note-card";
import { NoteComposer, NoteSaveError, type NoteValues } from "@/components/chat/client-panel/note-composer";
import { OtherNotesDialog } from "@/components/chat/client-panel/other-notes-dialog";
import { NOTES_PAGE_SIZE, noteErrorMessage } from "@/components/chat/client-panel/panel-helpers";
import { useNow } from "@/components/chat/client-panel/use-now";

let tempSeq = 0;

/**
 * The advisor's private notes about the client of this conversation, five
 * per page, with a composer and the notes other advisors shared. Creating is
 * optimistic and rolled back (with the reason) when the API refuses.
 */
export function ClientNotes({ conversationId, clientName }: { conversationId: string; clientName: string }) {
  const { text } = useI18n();
  const toast = useToast();
  const now = useNow();
  const list = usePagedList<Note>((cursor, signal) => notesApi.list(conversationId, cursor, NOTES_PAGE_SIZE, signal), [conversationId]);
  const { prependItem, replaceItem, removeItem, reset, reload } = list;
  const onFirstPage = list.page === 1;
  const blocked = list.error?.code === "forbidden";

  // Notes open up with the first started session: a pending request shows
  // the panel before that, so load again once this conversation's chat starts.
  useRealtimeEvent<{ session?: Pick<ChatSession, "conversation_id"> }>("session.started", (data) => {
    // reload (not reset): it refetches even when the first page had failed.
    if (data?.session?.conversation_id === conversationId) reload();
  });

  const create = React.useCallback(
    async ({ body, shared }: NoteValues) => {
      const at = new Date().toISOString();
      const temp: Note = { id: `temp-${++tempSeq}`, body, shared, created_at: at, updated_at: at };
      prependItem(temp);
      try {
        const saved = await notesApi.create(conversationId, { body, shared });
        replaceItem((n) => n.id === temp.id, saved);
        // New notes go on top: bring the advisor back to the first page.
        if (!onFirstPage) reset();
      } catch (err) {
        removeItem((n) => n.id === temp.id);
        throw new NoteSaveError(text(noteErrorMessage(err)));
      }
    },
    [conversationId, onFirstPage, prependItem, removeItem, replaceItem, reset, text],
  );

  const edit = React.useCallback(
    async (note: Note, body: string) => {
      if (body === note.body) return;
      try {
        replaceItem((n) => n.id === note.id, await notesApi.update(note.id, { body }));
      } catch (err) {
        throw new NoteSaveError(text(noteErrorMessage(err)));
      }
    },
    [replaceItem, text],
  );

  const toggleShare = React.useCallback(
    async (note: Note) => {
      const next = !note.shared;
      replaceItem((n) => n.id === note.id, { ...note, shared: next });
      try {
        replaceItem((n) => n.id === note.id, await notesApi.update(note.id, { shared: next }));
        toast.show({ tone: "success", message: text(next ? "Note shared with other advisors." : "Note is private again.") });
      } catch (err) {
        replaceItem((n) => n.id === note.id, note);
        toast.show({ tone: "error", message: text(noteErrorMessage(err)) });
      }
    },
    [replaceItem, text, toast],
  );

  const items = list.items;
  const remove = React.useCallback(
    async (note: Note) => {
      try {
        await notesApi.remove(note.id);
        removeItem((n) => n.id === note.id);
        // An emptied page starts over from the first one.
        if (items.length === 1) reset();
        toast.show({ tone: "success", message: text("Note deleted.") });
      } catch (err) {
        if (toApiError(err).code === "not_found") removeItem((n) => n.id === note.id);
        toast.show({ tone: "error", message: text(noteErrorMessage(err)) });
      }
    },
    [items.length, removeItem, reset, text, toast],
  );

  const actions = React.useMemo(
    () => ({ onEdit: edit, onToggleShare: (n: Note) => void toggleShare(n), onDelete: (n: Note) => void remove(n) }),
    [edit, remove, toggleShare],
  );

  return (
    <PanelSection title={text("Notes")} testId="client-notes">
      {blocked ? (
        <p className="panel-muted" data-testid="notes-blocked">
          {text(noteErrorMessage(list.error))}
        </p>
      ) : list.error ? (
        <ErrorState error={list.error} onRetry={list.reload} compact />
      ) : list.loading ? (
        <div role="status" aria-label={text("Loading notes…")}>
          <SkeletonList rows={2} rowClassName="h-12 bg-cream/70" />
        </div>
      ) : items.length ? (
        <ul className="grid gap-2" aria-label={text("Your notes")} data-testid="note-list">
          {items.map((note) => (
            <NoteCard key={note.id} note={note} now={now} actions={actions} />
          ))}
        </ul>
      ) : (
        <p className="panel-muted">{text("No notes yet. Notes help you remember what matters to this client.")}</p>
      )}
      <CursorPager
        className="pager-compact"
        page={list.page}
        hasPrev={list.hasPrev}
        hasNext={list.hasNext}
        onPrev={list.prev}
        onNext={list.next}
        loading={list.loading}
        label={text("Note pages")}
      />
      {blocked ? null : (
        <>
          <NoteComposer clientName={clientName} onCreate={create} />
          <OtherNotesDialog conversationId={conversationId} />
        </>
      )}
    </PanelSection>
  );
}
