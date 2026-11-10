"use client";

import * as React from "react";
import { Loader2 } from "lucide-react";
import { FiCheck, FiEdit2, FiX } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import { errorMessage, toApiError } from "@/lib/api/errors";
import { normalizeAlias, useClientAlias, useClientAliasActions } from "@/lib/chat/client-alias-provider";
import { cn } from "@/lib/utils";
import { announce } from "@/components/feedback/live-region";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

/** Same limit as the API (§23.4). */
export const ALIAS_MAX_LENGTH = 40;

/**
 * The client's name in the advisor's chat header (§23.4): the nickname when
 * there is one, with a pen that swaps the name for an input in place —
 * ✓ / Enter saves, ✗ / Esc cancels, an empty value clears the nickname. The
 * request runs with the controls disabled; a failure shows under the field and
 * keeps the editor open for another try. Renders just the name when nicknames
 * are not available (clients).
 */
export function ClientNameField({
  conversationId,
  clientId,
  realName,
  className,
}: {
  conversationId: string | null;
  clientId: string;
  /** The name on the account. */
  realName: string;
  className?: string;
}) {
  const { text } = useI18n();
  const { enabled, setAlias } = useClientAliasActions();
  const alias = useClientAlias(clientId);
  const shown = alias ?? realName;
  const [editing, setEditing] = React.useState(false);
  const [draft, setDraft] = React.useState("");
  const [saving, setSaving] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const inputRef = React.useRef<HTMLInputElement>(null);
  const penRef = React.useRef<HTMLButtonElement>(null);
  const restoreFocus = React.useRef(false);
  const errorId = React.useId();

  // The editor opens with the current nickname selected; closing puts focus back on the pen.
  React.useEffect(() => {
    if (editing) {
      inputRef.current?.focus();
      inputRef.current?.select();
    } else if (restoreFocus.current) {
      restoreFocus.current = false;
      penRef.current?.focus();
    }
  }, [editing]);

  const nameNode = (
    <span className="chat-header-name truncate font-serif text-[18px] font-bold" title={alias ? text("Name on the account: {value0}", { value0: realName }) : undefined} data-testid="chat-header-name">
      {text(shown)}
    </span>
  );
  // Clients (and a missing conversation): just the name.
  if (!enabled || !conversationId) return nameNode;

  const open = () => {
    setDraft(alias ?? "");
    setError(null);
    setEditing(true);
  };
  const close = () => {
    restoreFocus.current = true;
    setEditing(false);
    setError(null);
  };

  async function save() {
    if (saving || !conversationId) return;
    const next = normalizeAlias(draft);
    // Nothing changed (or nothing to clear): no request.
    if (next === (alias ?? "")) {
      close();
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await setAlias(conversationId, clientId, next);
      announce(text(next ? "Nickname saved" : "Nickname removed"));
      close();
    } catch (err) {
      const e = toApiError(err);
      setError(text(e.fields.alias ?? errorMessage(e)));
      inputRef.current?.focus();
    } finally {
      setSaving(false);
    }
  }

  if (!editing) {
    return (
      <span className={cn("chat-header-name-row", className)}>
        {nameNode}
        <Tooltip>
          <TooltipTrigger asChild>
            <button ref={penRef} type="button" className="chat-nickname-edit" onClick={open} aria-label={text("Edit nickname")} data-testid="edit-nickname">
              <FiEdit2 aria-hidden className="size-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="bottom">{alias ? text("Edit nickname. Name on the account: {value0}", { value0: realName }) : text("Edit nickname")}</TooltipContent>
        </Tooltip>
      </span>
    );
  }

  return (
    <form
      className={cn("chat-nickname-form", className)}
      data-testid="nickname-form"
      onSubmit={(e) => {
        e.preventDefault();
        void save();
      }}
    >
      <span className="chat-nickname-controls">
        <input
          ref={inputRef}
          value={draft}
          onChange={(e) => {
            setDraft(e.target.value);
            if (error) setError(null);
          }}
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              e.preventDefault();
              e.stopPropagation();
              if (!saving) close();
            }
          }}
          maxLength={ALIAS_MAX_LENGTH}
          placeholder={realName}
          disabled={saving}
          autoComplete="off"
          autoCapitalize="words"
          enterKeyHint="done"
          aria-label={text("Nickname for {value0}", { value0: realName })}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? errorId : undefined}
          aria-busy={saving || undefined}
          className="chat-nickname-input"
          data-testid="nickname-input"
        />
        <button type="submit" className="chat-nickname-button" disabled={saving} aria-label={text("Save nickname")} data-testid="nickname-save">
          {saving ? <Loader2 aria-hidden className="size-4 animate-spin motion-reduce:animate-none" data-testid="nickname-saving" /> : <FiCheck aria-hidden className="size-4" />}
        </button>
        <button type="button" className="chat-nickname-button" disabled={saving} onClick={close} aria-label={text("Cancel")} data-testid="nickname-cancel">
          <FiX aria-hidden className="size-4" />
        </button>
      </span>
      {error ? (
        <p id={errorId} role="alert" className="chat-nickname-error" data-testid="nickname-error">
          {error}
        </p>
      ) : null}
    </form>
  );
}
