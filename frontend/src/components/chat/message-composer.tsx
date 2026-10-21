"use client";

import * as React from "react";
import { FiImage, FiSend, FiX } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import { MESSAGE_MAX_LENGTH } from "@/lib/schemas/message-schema";
import { ACCEPT_ATTRIBUTE, attachmentProblemCopy, firstFile, validateAttachment } from "@/lib/chat/attachments";
import { insertAtCaret } from "@/lib/chat/message-format";
import { useMediaQuery } from "@/lib/use-media-query";
import { cn } from "@/lib/utils";
import { composerKeyAction } from "@/components/chat/composer-keys";
import { EmojiPicker } from "@/components/chat/emoji-picker";
import { ChatImage } from "@/components/chat/chat-image";
import { Button } from "@/components/ui/button";

/** Typing indicators: announce at most every 2s, stop after 3s idle. */
const TYPING_EVERY_MS = 2000;
const TYPING_IDLE_MS = 3000;
const MAX_LINES = 6;
/** Show the character counter from 80% of the limit. */
const COUNTER_FROM = Math.floor(MESSAGE_MAX_LENGTH * 0.8);

export type ComposerSubmit = { body: string; file?: File };

/** Drop leading blank lines and trailing whitespace; inner spacing is kept exactly. */
export function normalizeOutgoing(value: string): string {
  return value.replace(/^(?:[ \t]*\r?\n)+/, "").replace(/\s+$/, "");
}

function formatBytes(bytes: number, locale: string): string {
  const mb = bytes / (1024 * 1024);
  return mb >= 1
    ? `${new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(mb)} MB`
    : `${new Intl.NumberFormat(locale, { maximumFractionDigits: 0 }).format(Math.max(1, bytes / 1024))} KB`;
}

export function MessageComposer({
  onSend,
  onTyping,
  note,
  disabled = false,
  disabledReason,
  allowAttachments = true,
}: {
  onSend: (message: ComposerSubmit) => void;
  onTyping?: (typing: boolean) => void;
  note: string;
  disabled?: boolean;
  disabledReason?: string;
  allowAttachments?: boolean;
}) {
  const { t, text, locale } = useI18n();
  const touch = useMediaQuery("(hover: none) and (pointer: coarse)");
  const [value, setValue] = React.useState("");
  const [file, setFile] = React.useState<File | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [dragging, setDragging] = React.useState(false);
  const textareaRef = React.useRef<HTMLTextAreaElement>(null);
  const fileRef = React.useRef<HTMLInputElement>(null);
  const caretRef = React.useRef<number | null>(null);
  const lastTyping = React.useRef(0);
  const idleTimer = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const errorId = React.useId();
  const counterId = React.useId();

  const stopTyping = React.useCallback(() => {
    if (idleTimer.current) clearTimeout(idleTimer.current);
    idleTimer.current = null;
    if (lastTyping.current) {
      lastTyping.current = 0;
      onTyping?.(false);
    }
  }, [onTyping]);

  React.useEffect(() => stopTyping, [stopTyping]);

  const typed = React.useCallback(() => {
    if (!onTyping || disabled) return;
    const now = Date.now();
    if (now - lastTyping.current > TYPING_EVERY_MS) {
      lastTyping.current = now;
      onTyping(true);
    }
    if (idleTimer.current) clearTimeout(idleTimer.current);
    idleTimer.current = setTimeout(stopTyping, TYPING_IDLE_MS);
  }, [onTyping, disabled, stopTyping]);

  // Auto-grow up to MAX_LINES, then scroll.
  const resize = React.useCallback(() => {
    const el = textareaRef.current;
    if (!el) return;
    const style = window.getComputedStyle(el);
    const line = parseFloat(style.lineHeight) || 20;
    const borders = parseFloat(style.borderTopWidth) + parseFloat(style.borderBottomWidth);
    const max = line * MAX_LINES + parseFloat(style.paddingTop) + parseFloat(style.paddingBottom) + borders;
    el.style.height = "auto";
    const content = el.scrollHeight + borders;
    el.style.height = `${Math.min(content, max)}px`;
    el.style.overflowY = content > max ? "auto" : "hidden";
  }, []);

  const placeholder = disabled && disabledReason ? disabledReason : file ? text("Add a caption…") : t("chatroom.composer.placeholder");

  React.useLayoutEffect(() => {
    resize();
    // Restore the caret after an emoji insert.
    const el = textareaRef.current;
    if (el && caretRef.current !== null) {
      el.setSelectionRange(caretRef.current, caretRef.current);
      caretRef.current = null;
    }
  }, [value, placeholder, resize]);

  // Width changes (layout, fonts, rotation) change the wrapped height.
  React.useEffect(() => {
    const el = textareaRef.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    let width = el.clientWidth;
    const ro = new ResizeObserver(() => {
      if (el.clientWidth !== width) {
        width = el.clientWidth;
        resize();
      }
    });
    ro.observe(el);
    void document.fonts?.ready.then(resize);
    return () => ro.disconnect();
  }, [resize]);

  function attach(candidate: File | null) {
    if (!candidate || !allowAttachments || disabled) return;
    const problem = validateAttachment(candidate);
    if (problem) {
      setError(text(attachmentProblemCopy[problem]));
      return;
    }
    setFile(candidate);
    setError(null);
    textareaRef.current?.focus();
  }

  function insertEmoji(emoji: string) {
    const el = textareaRef.current;
    const start = el?.selectionStart ?? value.length;
    const end = el?.selectionEnd ?? value.length;
    const next = insertAtCaret(value, start, end, emoji);
    if (next.value.length > MESSAGE_MAX_LENGTH) return;
    caretRef.current = next.caret;
    setValue(next.value);
    typed();
  }

  function submit() {
    if (disabled) return;
    const body = normalizeOutgoing(value);
    if (!file && !body.trim()) {
      setError(t("chatroom.composer.errors.required"));
      textareaRef.current?.focus();
      return;
    }
    if (body.length > MESSAGE_MAX_LENGTH) {
      setError(t("chatroom.composer.errors.max", { max: MESSAGE_MAX_LENGTH }));
      return;
    }
    stopTyping();
    onSend(file ? { body, file } : { body });
    setValue("");
    setFile(null);
    setError(null);
    textareaRef.current?.focus();
  }

  const hasFiles = (e: React.DragEvent) => Array.from(e.dataTransfer?.types ?? []).includes("Files");
  const length = value.length;
  const showCounter = length >= COUNTER_FROM;
  const canSend = !disabled && (Boolean(file) || value.trim().length > 0) && length <= MESSAGE_MAX_LENGTH;

  return (
    <form
      className={cn("composer", dragging && "composer-dragging")}
      onSubmit={(e) => {
        e.preventDefault();
        submit();
      }}
      onDragEnter={(e) => {
        if (!allowAttachments || disabled || !hasFiles(e)) return;
        e.preventDefault();
        setDragging(true);
      }}
      onDragOver={(e) => {
        if (!allowAttachments || disabled || !hasFiles(e)) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = "copy";
      }}
      onDragLeave={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setDragging(false);
      }}
      onDrop={(e) => {
        if (!allowAttachments || disabled || !hasFiles(e)) return;
        e.preventDefault();
        setDragging(false);
        attach(firstFile(e.dataTransfer.files));
      }}
    >
      {file ? (
        <div className="composer-attachment" data-testid="composer-attachment">
          <ChatImage file={file} alt={file.name || text("Image")} compact />
          <div className="min-w-0 flex-1 font-ui text-xs">
            <p className="truncate font-semibold">{file.name || text("Image")}</p>
            <p className="opacity-70">
              {formatBytes(file.size, locale)} · {text("Add a caption below (optional).")}
            </p>
          </div>
          <button type="button" className="composer-tool" onClick={() => setFile(null)} aria-label={text("Remove image")}>
            <FiX className="size-4" aria-hidden />
          </button>
        </div>
      ) : null}
      <div className="composer-row">
        <div className="composer-tools">
          <EmojiPicker onSelect={insertEmoji} disabled={disabled} onClose={() => textareaRef.current?.focus()} />
          {allowAttachments ? (
            <>
              <button
                type="button"
                className="composer-tool"
                onClick={() => fileRef.current?.click()}
                disabled={disabled}
                aria-label={text("Attach an image")}
                data-testid="attach-image"
              >
                <FiImage className="size-[18px]" aria-hidden />
              </button>
              <input
                ref={fileRef}
                type="file"
                accept={ACCEPT_ATTRIBUTE}
                className="sr-only"
                tabIndex={-1}
                aria-hidden
                data-testid="attach-input"
                onChange={(e) => {
                  attach(e.target.files?.[0] ?? null);
                  e.target.value = "";
                }}
              />
            </>
          ) : null}
        </div>
        <label className="sr-only" htmlFor="chat-composer-input">
          {t("chatroom.composer.label")}
        </label>
        <textarea
          id="chat-composer-input"
          ref={textareaRef}
          rows={1}
          value={value}
          disabled={disabled}
          maxLength={MESSAGE_MAX_LENGTH}
          autoComplete="off"
          enterKeyHint={touch ? "enter" : "send"}
          placeholder={placeholder}
          aria-invalid={error ? true : undefined}
          aria-describedby={cn(error && errorId, showCounter && counterId) || undefined}
          className="composer-input"
          onChange={(e) => {
            setValue(e.target.value);
            if (error) setError(null);
            typed();
          }}
          onKeyDown={(e) => {
            const action = composerKeyAction(
              {
                key: e.key,
                shiftKey: e.shiftKey,
                altKey: e.altKey,
                ctrlKey: e.ctrlKey,
                metaKey: e.metaKey,
                isComposing: e.nativeEvent.isComposing,
                keyCode: e.keyCode,
              },
              touch,
            );
            if (action === "send") {
              e.preventDefault();
              submit();
            }
          }}
          onPaste={(e) => {
            const pasted = firstFile(e.clipboardData?.files);
            if (pasted && pasted.type.startsWith("image/")) {
              e.preventDefault();
              attach(pasted);
            }
          }}
        />
        <Button
          type="submit"
          disabled={!canSend}
          size="icon"
          variant="ghost"
          aria-label={t("chatroom.composer.send")}
          className="composer-send shrink-0 text-ink hover:bg-warm-gray"
        >
          <FiSend className="size-4" aria-hidden />
        </Button>
      </div>
      <div className="composer-meta">
        <p className="font-ui text-[10px] text-ink opacity-50">{note}</p>
        {showCounter ? (
          <span
            id={counterId}
            className={cn("composer-counter", length >= MESSAGE_MAX_LENGTH && "composer-counter-limit")}
            aria-live="polite"
            aria-label={text("{count} of {max} characters", { count: length, max: MESSAGE_MAX_LENGTH })}
          >
            {length}/{MESSAGE_MAX_LENGTH}
          </span>
        ) : null}
      </div>
      {error ? (
        <p id={errorId} role="alert" className="font-ui text-[11px] text-red-800">
          {error}
        </p>
      ) : null}
      {dragging ? (
        <div className="composer-drop" aria-hidden>
          {text("Drop the image to attach it")}
        </div>
      ) : null}
    </form>
  );
}
