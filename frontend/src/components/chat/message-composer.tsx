"use client";

import * as React from "react";
import { Loader2 } from "lucide-react";
import { FiCornerUpLeft, FiImage, FiSend, FiX } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import type { ReplyPreview } from "@/lib/api/types";
import { MESSAGE_MAX_LENGTH } from "@/lib/schemas/message-schema";
import { ACCEPT_ATTRIBUTE, attachmentProblemCopy, firstFile, validateAttachment } from "@/lib/chat/attachments";
import { PrepareImageError, prepareImage, prepareProblemCopy } from "@/lib/chat/prepare-image";
import { clearDraft, draftKey, loadDraft, pruneDrafts, saveDraft } from "@/lib/chat/drafts";
import { insertAtCaret } from "@/lib/chat/message-format";
import { useMediaQuery } from "@/lib/use-media-query";
import { cn } from "@/lib/utils";
import { composerKeyAction } from "@/components/chat/composer-keys";
import { EmojiPicker } from "@/components/chat/emoji-picker";
import { ChatImage } from "@/components/chat/chat-image";
import { useMessageActions } from "@/components/chat/message-actions-context";
import { Button } from "@/components/ui/button";

/** Typing indicators: announce at most every 2s, stop after 3s idle. */
const TYPING_EVERY_MS = 2000;
const TYPING_IDLE_MS = 3000;
const MAX_LINES = 6;
/** Show the character counter from 80% of the limit. */
const COUNTER_FROM = Math.floor(MESSAGE_MAX_LENGTH * 0.8);

/** Typing pauses this long before the draft is written to storage. */
export const DRAFT_DEBOUNCE_MS = 400;
/** How long the "Draft saved" hint stays visible. */
const DRAFT_HINT_MS = 2200;

export type ComposerSubmit = {
  body: string;
  file?: File;
  /** Pixel size of the prepared image (absent when unknown). */
  size?: { width: number; height: number };
  /** The message being answered (§23.2); present only when replying. */
  replyTo?: ReplyPreview;
};

/** What the conversation column can do with the composer (drop target, §22.4). */
export type ComposerHandle = {
  /** Attach an image the same way the picker does (prepared, validated, previewed). */
  attach: (file: File | null) => Promise<void>;
  focus: () => void;
};

const MIRROR_STYLES = [
  "fontFamily",
  "fontSize",
  "fontWeight",
  "fontStyle",
  "letterSpacing",
  "wordSpacing",
  "textTransform",
  "textIndent",
  "lineHeight",
  "tabSize",
  "paddingTop",
  "paddingRight",
  "paddingBottom",
  "paddingLeft",
  "borderTopWidth",
  "borderRightWidth",
  "borderBottomWidth",
  "borderLeftWidth",
  "boxSizing",
] as const;

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
  userId,
  draftId,
  draftFallbackId,
  disabledAction,
  ref,
}: {
  ref?: React.Ref<ComposerHandle>;
  /** Next to the disabled reason (e.g. the "Start live chat" button for clients). */
  disabledAction?: React.ReactNode;
  onSend: (message: ComposerSubmit) => void;
  onTyping?: (typing: boolean) => void;
  /** Rate and balance line under the field (may contain a struck-through list rate). */
  note: React.ReactNode;
  disabled?: boolean;
  disabledReason?: string;
  allowAttachments?: boolean;
  /** Drafts are kept per user and per `draftId` (conversation id, or an advisor slug before one exists). */
  userId?: string;
  draftId?: string | null;
  /** A second key to read from when `draftId` has no draft (the slug key used before the conversation existed). */
  draftFallbackId?: string | null;
}) {
  const { t, text, locale } = useI18n();
  const touch = useMediaQuery("(hover: none) and (pointer: coarse)");
  // The message being answered lives in the chatroom (per conversation); the composer shows and clears it.
  const { replyTarget, cancelReply, nameOf, viewerId } = useMessageActions();
  const draftsOn = Boolean(userId && draftId);
  const readDraft = React.useCallback(
    () => (userId && draftId ? loadDraft(userId, draftId) || (draftFallbackId ? loadDraft(userId, draftFallbackId) : "") : ""),
    [userId, draftId, draftFallbackId],
  );
  const [value, setValue] = React.useState(readDraft);
  // Switching conversation restores that conversation's draft (adjusted during render).
  const [loadedFor, setLoadedFor] = React.useState(`${userId}|${draftId}`);
  if (loadedFor !== `${userId}|${draftId}`) {
    setLoadedFor(`${userId}|${draftId}`);
    setValue(readDraft());
  }
  const [draftHint, setDraftHint] = React.useState(false);
  const valueRef = React.useRef(value);
  React.useEffect(() => {
    valueRef.current = value;
  });
  const [file, setFile] = React.useState<File | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [fileSize, setFileSize] = React.useState<{ width: number; height: number } | null>(null);
  const [preparing, setPreparing] = React.useState(false);
  const prepareToken = React.useRef(0);
  const textareaRef = React.useRef<HTMLTextAreaElement>(null);
  const mirrorRef = React.useRef<HTMLDivElement>(null);
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

  // Auto-grow up to MAX_LINES, then scroll. The height comes from a hidden mirror
  // that has the textarea's text and metrics, so the live textarea is never
  // collapsed to measure it (that made the field, and the feed above it, jump
  // while typing). The height is written only when it changes.
  const resize = React.useCallback((content: string) => {
    const el = textareaRef.current;
    const mirror = mirrorRef.current;
    if (!el || !mirror) return;
    const style = window.getComputedStyle(el);
    for (const prop of MIRROR_STYLES) if (mirror.style[prop] !== style[prop]) mirror.style[prop] = style[prop];
    const width = `${el.offsetWidth}px`;
    if (mirror.style.width !== width) mirror.style.width = width;
    // A trailing zero-width character keeps a final newline visible as an empty line.
    mirror.textContent = `${content}\u200b`;
    const line = parseFloat(style.lineHeight) || 20;
    const borders = (parseFloat(style.borderTopWidth) || 0) + (parseFloat(style.borderBottomWidth) || 0);
    const padding = (parseFloat(style.paddingTop) || 0) + (parseFloat(style.paddingBottom) || 0);
    const max = line * MAX_LINES + padding + borders;
    const needed = mirror.scrollHeight + borders;
    const height = `${Math.min(needed, max)}px`;
    if (el.style.height !== height) el.style.height = height;
    const overflow = needed > max ? "auto" : "hidden";
    if (el.style.overflowY !== overflow) el.style.overflowY = overflow;
  }, []);

  // Debounced save; a blank value removes the draft. Skipped until the restored value is in place.
  React.useEffect(() => {
    if (!userId || !draftId || loadedFor !== `${userId}|${draftId}`) return;
    const stored = loadDraft(userId, draftId);
    if (stored === value) return;
    const timer = window.setTimeout(() => {
      const kept = saveDraft(userId, draftId, value);
      if (kept) {
        if (draftFallbackId) clearDraft(userId, draftFallbackId);
        setDraftHint(true);
      }
    }, DRAFT_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [value, userId, draftId, draftFallbackId, loadedFor]);
  React.useEffect(() => {
    if (!draftHint) return;
    const timer = window.setTimeout(() => setDraftHint(false), DRAFT_HINT_MS);
    return () => window.clearTimeout(timer);
  }, [draftHint]);
  // A reload or tab switch right after typing must not lose the text.
  React.useEffect(() => {
    if (!userId || !draftId) return;
    pruneDrafts(userId);
    const flush = () => {
      if (document.visibilityState === "hidden") saveDraft(userId, draftId, valueRef.current);
    };
    const flushNow = () => void saveDraft(userId, draftId, valueRef.current);
    document.addEventListener("visibilitychange", flush);
    window.addEventListener("pagehide", flushNow);
    return () => {
      document.removeEventListener("visibilitychange", flush);
      window.removeEventListener("pagehide", flushNow);
    };
  }, [userId, draftId]);
  // Another tab edited this draft: follow it unless the user is typing here.
  React.useEffect(() => {
    if (!userId || !draftId) return;
    const key = draftKey(userId, draftId);
    const onStorage = (e: StorageEvent) => {
      if (e.key !== key || document.activeElement === textareaRef.current) return;
      setValue(loadDraft(userId, draftId));
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, [userId, draftId]);

  // Starting a reply puts the caret in the field (not when another conversation brings its own pending reply along).
  const focusedReply = React.useRef<{ draft: string | null | undefined; id: string | undefined }>({ draft: draftId, id: replyTarget?.id });
  React.useEffect(() => {
    const before = focusedReply.current;
    focusedReply.current = { draft: draftId, id: replyTarget?.id };
    if (replyTarget && before.draft === draftId && before.id !== replyTarget.id) textareaRef.current?.focus();
  }, [replyTarget, draftId]);

  const placeholder = file ? text("Add a caption…") : t("chatroom.composer.placeholder");

  React.useLayoutEffect(() => {
    resize(value);
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
        resize(valueRef.current);
      }
    });
    ro.observe(el);
    void document.fonts?.ready.then(() => resize(valueRef.current));
    return () => ro.disconnect();
  }, [resize]);

  /** Prepare (sniff, downscale, re-encode, §22.4), validate and preview an image; every failure is shown. */
  async function attach(candidate: File | null) {
    if (!candidate || !allowAttachments) return;
    if (disabled) {
      // Never drop a file silently: say why it cannot be attached.
      setError(disabledReason || null);
      return;
    }
    const token = ++prepareToken.current;
    setPreparing(true);
    setError(null);
    try {
      const prepared = await prepareImage(candidate);
      if (token !== prepareToken.current) return;
      const problem = validateAttachment(prepared.file);
      if (problem) {
        setError(text(attachmentProblemCopy[problem]));
        return;
      }
      setFile(prepared.file);
      setFileSize(prepared.width > 0 && prepared.height > 0 ? { width: prepared.width, height: prepared.height } : null);
      textareaRef.current?.focus();
    } catch (err) {
      if (token !== prepareToken.current) return;
      setError(text(err instanceof PrepareImageError ? prepareProblemCopy[err.problem] : prepareProblemCopy.empty));
    } finally {
      if (token === prepareToken.current) setPreparing(false);
    }
  }
  React.useImperativeHandle(ref, () => ({ attach, focus: () => textareaRef.current?.focus() }));

  function removeFile() {
    prepareToken.current++; // a preparation still running is abandoned
    setPreparing(false);
    setFile(null);
    setFileSize(null);
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
    if (disabled || preparing) return;
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
    onSend({ body, ...(file ? { file, ...(fileSize ? { size: fileSize } : {}) } : {}), ...(replyTarget ? { replyTo: replyTarget } : {}) });
    cancelReply();
    // Sent: the draft is done (the outbox keeps the message itself).
    if (userId && draftId) {
      clearDraft(userId, draftId);
      if (draftFallbackId) clearDraft(userId, draftFallbackId);
    }
    setDraftHint(false);
    setValue("");
    setFile(null);
    setFileSize(null);
    setError(null);
    textareaRef.current?.focus();
  }

  const length = value.length;
  const showCounter = length >= COUNTER_FROM;
  const canSend = !disabled && !preparing && (Boolean(file) || value.trim().length > 0) && length <= MESSAGE_MAX_LENGTH;

  return (
    <form
      className="composer"
      onSubmit={(e) => {
        e.preventDefault();
        submit();
      }}
    >
      {preparing ? (
        <div className="composer-attachment" role="status" data-testid="composer-preparing">
          <Loader2 aria-hidden className="size-4 shrink-0 animate-spin motion-reduce:animate-none" />
          <p className="min-w-0 flex-1 font-ui text-xs">{text("Preparing image…")}</p>
          <button type="button" className="composer-tool" onClick={removeFile} aria-label={text("Cancel")}>
            <FiX className="size-4" aria-hidden />
          </button>
        </div>
      ) : null}
      {file && !preparing ? (
        <div className="composer-attachment" data-testid="composer-attachment">
          <ChatImage file={file} alt={file.name || text("Image")} compact />
          <div className="min-w-0 flex-1 font-ui text-xs">
            <p className="truncate font-semibold">{file.name || text("Image")}</p>
            <p className="opacity-70">
              {formatBytes(file.size, locale)} · {text("Add a caption below (optional).")}
            </p>
          </div>
          <button type="button" className="composer-tool" onClick={removeFile} aria-label={text("Remove image")}>
            <FiX className="size-4" aria-hidden />
          </button>
        </div>
      ) : null}
      {replyTarget ? (
        <div
          className="composer-reply"
          data-testid="composer-reply"
          role="group"
          aria-label={text("Replying")}
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              e.preventDefault();
              cancelReply();
              textareaRef.current?.focus();
            }
          }}
        >
          <FiCornerUpLeft aria-hidden className="size-4 shrink-0" />
          <div className="min-w-0 flex-1 font-ui text-xs">
            <p className="composer-reply-title truncate font-semibold">
              {replyTarget.sender_id && replyTarget.sender_id === viewerId
                ? text("Replying to yourself")
                : text("Replying to {value0}", { value0: nameOf(replyTarget.sender_id) })}
            </p>
            <p className="composer-reply-excerpt truncate">{replyTarget.excerpt || text("Photo")}</p>
          </div>
          <button type="button" className="composer-tool" onClick={() => { cancelReply(); textareaRef.current?.focus(); }} aria-label={text("Cancel reply")} data-testid="composer-reply-cancel">
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
        <div className="composer-field">
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
              if (e.key === "Escape" && replyTarget && !e.nativeEvent.isComposing) {
                e.preventDefault();
                cancelReply();
                return;
              }
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
          <div ref={mirrorRef} className="composer-mirror" aria-hidden />
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
      </div>
      {disabled && disabledReason ? (
        <p className="composer-reason" role="status" data-testid="composer-reason">
          <span>{disabledReason}</span>
          {disabledAction}
        </p>
      ) : null}
      <div className="composer-meta">
        <p className="composer-note font-ui text-[10px] text-ink" data-testid="composer-note">
          {note}
        </p>
        {/* A fixed-size slot: the hint and the counter come and go inside it, so the composer never changes height while typing. */}
        <div className="composer-meta-aside" data-testid="composer-aside">
          {draftsOn && draftHint && value.trim() ? (
            <span className="composer-draft-hint" role="status" data-testid="draft-saved">
              {text("Draft saved")}
            </span>
          ) : null}
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
      </div>
      {error ? (
        <p id={errorId} role="alert" className="font-ui text-[11px] text-red-800">
          {error}
        </p>
      ) : null}
    </form>
  );
}
