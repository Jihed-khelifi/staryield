"use client";

import * as React from "react";
import * as yup from "yup";
import { Users } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import type { TranslateCopy } from "@/i18n/translate-copy";
import type { Note } from "@/lib/api/types";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { FieldRow } from "@/components/ui/field";
import { Form, FormRootError } from "@/components/ui/form";
import { NOTE_MAX_CHARS, isTempNote, sinceLabel } from "@/components/chat/client-panel/panel-helpers";
import { noteBodyRule, submitOnModEnter } from "@/components/chat/client-panel/note-composer";

const editSchema = (text: TranslateCopy) => yup.object({ body: noteBodyRule(text) });

export type NoteCardActions = {
  /** Resolve when saved; reject with a translated message to show inline. */
  onEdit: (note: Note, body: string) => Promise<void>;
  onToggleShare: (note: Note) => void;
  onDelete: (note: Note) => void;
};

function EditForm({ note, onCancel, onSave }: { note: Note; onCancel: () => void; onSave: (body: string) => Promise<void> }) {
  const { text } = useI18n();
  const form = useSchemaForm<ReturnType<typeof editSchema>, { body: string }>(editSchema, { defaultValues: { body: note.body } });
  const submit = form.handleSubmit(async ({ body }) => {
    try {
      await onSave(body.trim());
    } catch (err) {
      form.setError("root.server", { type: "server", message: err instanceof Error ? err.message : String(err) });
    }
  });
  return (
    <Form {...form}>
      <form noValidate onSubmit={submit} className="grid gap-2" aria-label={text("Edit note")}>
        <FormRootError className="p-2 text-xs" />
        <FieldRow control={form.control} name="body" label={text("Edit note")} labelClassName="sr-only" maxLength={NOTE_MAX_CHARS} showValid={false} className="gap-1">
          {(field) => (
            <textarea
              {...field}
              rows={3}
              autoFocus
              className="note-input"
              aria-keyshortcuts="Meta+Enter Control+Enter"
              onKeyDown={(event) => {
                if (event.key === "Escape") {
                  event.preventDefault();
                  onCancel();
                } else submitOnModEnter(event);
              }}
            />
          )}
        </FieldRow>
        <div className="flex justify-end gap-2">
          <Button type="button" size="sm" variant="outline" className="border-ink/30 px-2.5" onClick={onCancel}>
            {text("Cancel")}
          </Button>
          <Button type="submit" size="sm" variant="cta" className="px-2.5" disabled={form.formState.isSubmitting}>
            {text("Save")}
          </Button>
        </div>
      </form>
    </Form>
  );
}

/**
 * One of the advisor's notes: when it was saved, whether it is shared, and
 * edit / share / delete. Delete asks for confirmation.
 */
export function NoteCard({ note, now, actions }: { note: Note; now: number; actions: NoteCardActions }) {
  const { text, locale } = useI18n();
  const [editing, setEditing] = React.useState(false);
  const pending = isTempNote(note.id);
  const edited = Date.parse(note.updated_at) - Date.parse(note.created_at) > 1000;
  const since = sinceLabel(edited ? note.updated_at : note.created_at, locale, now);
  const when = pending
    ? text("Saving…")
    : edited
      ? since.justNow
        ? text("Edited just now")
        : text("Edited {value0}", { value0: since.relative })
      : since.justNow
        ? text("Saved just now")
        : text("Saved {value0}", { value0: since.relative });

  return (
    <li className="panel-card" data-testid="note-card" data-shared={note.shared || undefined} aria-busy={pending || undefined}>
      {editing ? (
        <EditForm
          note={note}
          onCancel={() => setEditing(false)}
          onSave={async (body) => {
            await actions.onEdit(note, body);
            setEditing(false);
          }}
        />
      ) : (
        <p className="panel-card-body whitespace-pre-wrap">{note.body}</p>
      )}
      <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1">
        <p className="panel-card-meta">
          <time dateTime={note.updated_at}>{when}</time>
        </p>
        {note.shared ? (
          <span className="note-shared-badge" data-testid="note-shared-badge">
            <Users aria-hidden className="size-3" />
            {text("Shared")}
          </span>
        ) : null}
      </div>
      {editing || pending ? null : (
        <div className="note-actions">
          <button type="button" className="note-action" onClick={() => setEditing(true)}>
            {text("Edit")}
          </button>
          <button type="button" className="note-action" onClick={() => actions.onToggleShare(note)} data-testid="note-toggle-share">
            {text(note.shared ? "Stop sharing" : "Share")}
          </button>
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <button type="button" className="note-action">
                {text("Delete")}
              </button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>{text("Delete this note?")}</AlertDialogTitle>
                <AlertDialogDescription>{text("The note is removed for you and for any advisor you shared it with.")}</AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>{text("Cancel")}</AlertDialogCancel>
                <AlertDialogAction destructive onClick={() => actions.onDelete(note)}>
                  {text("Delete note")}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      )}
    </li>
  );
}
