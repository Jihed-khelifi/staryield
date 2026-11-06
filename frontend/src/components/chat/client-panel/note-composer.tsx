"use client";

import * as React from "react";
import * as yup from "yup";

import { useI18n } from "@/i18n/i18n-provider";
import type { TranslateCopy } from "@/i18n/translate-copy";
import { useSchemaForm } from "@/lib/forms/use-schema-form";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { FieldRow } from "@/components/ui/field";
import { Form, FormRootError } from "@/components/ui/form";
import { NOTE_MAX_CHARS } from "@/components/chat/client-panel/panel-helpers";

/** 1–2000 characters after trimming, counted like the API (characters, not bytes). */
export function noteBodyRule(text: TranslateCopy) {
  return yup
    .string()
    .trim()
    .required(text("Write a note first."))
    .test("max-chars", text("Use at most {value0} characters.", { value0: NOTE_MAX_CHARS }), (value) =>
      value === undefined ? true : Array.from(value).length <= NOTE_MAX_CHARS,
    );
}

export const noteSchema = (text: TranslateCopy) => yup.object({ body: noteBodyRule(text), shared: yup.boolean().default(false) });

export type NoteValues = { body: string; shared: boolean };

/** Cmd+Enter / Ctrl+Enter submits the surrounding form. */
export function submitOnModEnter(event: React.KeyboardEvent<HTMLTextAreaElement>) {
  if (event.key === "Enter" && (event.metaKey || event.ctrlKey) && !event.nativeEvent.isComposing) {
    event.preventDefault();
    event.currentTarget.form?.requestSubmit();
  }
}

/** A failed save: the message is already translated for the composer. */
export class NoteSaveError extends Error {}

/**
 * New note about the client: private by default, optionally shared with the
 * other advisors who chat with them. The composer clears at once (the note
 * is shown optimistically); if `onCreate` rejects, the text comes back with
 * the reason.
 */
export function NoteComposer({ clientName, onCreate }: { clientName: string; onCreate: (values: NoteValues) => Promise<void> }) {
  const { text } = useI18n();
  const form = useSchemaForm<ReturnType<typeof noteSchema>, NoteValues>(noteSchema, {
    defaultValues: { body: "", shared: false },
    // An empty composer is not an error until the advisor tries to save.
    mode: "onSubmit",
  });
  const shareId = React.useId();
  const guidanceId = React.useId();
  const shared = form.watch("shared");

  const submit = form.handleSubmit(async (values) => {
    const draft = { body: values.body.trim(), shared: values.shared };
    form.reset({ body: "", shared: false });
    try {
      await onCreate(draft);
    } catch (err) {
      form.reset(draft);
      form.setError("root.server", { type: "server", message: err instanceof Error ? err.message : String(err) });
    }
  });

  return (
    <Form {...form}>
      <form noValidate onSubmit={submit} className="note-composer" data-testid="note-composer" aria-label={text("New note")}>
        <FormRootError className="p-2 text-xs" />
        <FieldRow
          control={form.control}
          name="body"
          label={text("Note about {value0}", { value0: clientName })}
          labelClassName="sr-only"
          maxLength={NOTE_MAX_CHARS}
          showValid={false}
          className="gap-1"
        >
          {(field) => (
            <textarea
              {...field}
              rows={2}
              className="note-input"
              placeholder={text("Write notes about this client…")}
              aria-keyshortcuts="Meta+Enter Control+Enter"
              onKeyDown={submitOnModEnter}
            />
          )}
        </FieldRow>
        <div className="flex items-center gap-2">
          <Checkbox
            id={shareId}
            checked={shared}
            onCheckedChange={(checked) => form.setValue("shared", checked === true, { shouldDirty: true })}
            aria-describedby={guidanceId}
            data-testid="note-share"
          />
          <label htmlFor={shareId} className="text-xs">
            {text("Share with other advisors")}
          </label>
        </div>
        <p id={guidanceId} className="panel-muted">
          {text("Private notes are only for you. Shared notes are visible to other advisors who chat with this client. Never include contact or payment details.")}
        </p>
        <div className="flex items-center justify-end gap-2">
          <span className="panel-muted" aria-hidden>
            {text("Cmd/Ctrl + Enter")}
          </span>
          <Button type="submit" size="sm" variant="cta" className="px-3" disabled={form.formState.isSubmitting}>
            {text("Save note")}
          </Button>
        </div>
      </form>
    </Form>
  );
}
