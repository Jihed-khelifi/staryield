"use client";
/* eslint-disable @next/next/no-img-element */
import * as React from "react";
import { ImageOff } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import type { ChatAttachment } from "@/lib/api/types";
import { acquireAttachment, releaseAttachment } from "@/lib/chat/attachments";
import { logger } from "@/lib/logger";
import { toApiError } from "@/lib/api/errors";
import { useMediaQuery } from "@/lib/use-media-query";
import { cn } from "@/lib/utils";
import { ChatImageViewer } from "@/components/chat/chat-image-viewer";

const MAX_W = 260;
const MAX_H = 320;
const MAX_H_PHONE = 260;

type Source = { url: string | null; failed: boolean; retry: () => void };

/** A blob URL for a private attachment (shared cache, released on unmount). */
function useAttachmentUrl(id: string | undefined): Source {
  const [result, setResult] = React.useState<{ key: string; url: string | null; failed: boolean }>({ key: "", url: null, failed: false });
  const [nonce, setNonce] = React.useState(0);
  const key = `${id ?? ""}|${nonce}`;
  React.useEffect(() => {
    if (!id) return;
    let active = true;
    acquireAttachment(id).then(
      (url) => {
        if (active) setResult({ key, url, failed: false });
      },
      (err) => {
        logger.debug("attachment load failed", { code: String(toApiError(err).code) });
        if (active) setResult({ key, url: null, failed: true });
      },
    );
    return () => {
      active = false;
      releaseAttachment(id);
    };
  }, [id, key]);
  const current = result.key === key;
  return { url: current ? result.url : null, failed: current && result.failed, retry: () => setNonce((n) => n + 1) };
}

/** A blob URL for a local file (revoked on unmount). */
function useFileUrl(file: Blob | undefined): string | null {
  const [url, setUrl] = React.useState<{ file: Blob; url: string } | null>(null);
  React.useEffect(() => {
    if (!file) return;
    const u = URL.createObjectURL(file);
    // Publishing the URL is the effect's purpose; revoked in cleanup.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setUrl({ file, url: u });
    return () => URL.revokeObjectURL(u);
  }, [file]);
  return url && url.file === file ? url.url : null;
}

export function displaySize(width?: number, height?: number, max = MAX_W, maxH = MAX_H): { width: number; ratio: number } {
  const ratio = width && height && width > 0 && height > 0 ? width / height : 4 / 3;
  return { width: Math.round(Math.min(max, maxH * ratio, width || max)), ratio };
}

type ChatImageProps = {
  attachment?: ChatAttachment;
  /** Local file for an optimistic (not yet uploaded) image. */
  file?: Blob;
  alt: string;
  caption?: string;
  compact?: boolean;
  className?: string;
};

/**
 * A private chat image with an aspect-ratio placeholder from the attachment
 * metadata; clicking it opens an accessible lightbox.
 */
export function ChatImage({ attachment, file, alt, caption, compact = false, className }: ChatImageProps) {
  const { text } = useI18n();
  const remote = useAttachmentUrl(file ? undefined : attachment?.id);
  const local = useFileUrl(file);
  const url = local ?? remote.url;
  const [loaded, setLoaded] = React.useState<string | null>(null);
  const [natural, setNatural] = React.useState<{ w: number; h: number } | null>(null);
  const [open, setOpen] = React.useState(false);
  const frameRef = React.useRef<HTMLButtonElement>(null);
  const [broken, setBroken] = React.useState<string | null>(null);
  const phone = useMediaQuery("(max-width: 767px)");
  // Phones: a shorter cap keeps tall photos from filling the screen. The frame keeps the stored aspect ratio, so nothing shifts when the pixels arrive.
  const size = displaySize(attachment?.width || natural?.w, attachment?.height || natural?.h, compact ? 72 : MAX_W, compact ? 72 : phone ? MAX_H_PHONE : MAX_H);
  const ready = Boolean(url && loaded === url);

  const frameStyle: React.CSSProperties = compact ? { width: 72, height: 72 } : { width: size.width, aspectRatio: String(size.ratio) };

  if ((remote.failed && !local) || (url && broken === url)) {
    return (
      <div className={cn("chat-image-frame chat-image-error", className)} style={frameStyle} data-testid="chat-image-error">
        <ImageOff size={18} aria-hidden />
        {!compact ? (
          <>
            <span className="font-ui text-[11px]" role="status">
              {text("This image couldn't be loaded.")}
            </span>
            <button
              type="button"
              className="font-ui text-[11px] font-semibold underline"
              onClick={() => {
                setBroken(null);
                remote.retry();
              }}
            >
              {text("Retry")}
            </button>
          </>
        ) : (
          <span className="sr-only">{text("Image unavailable")}</span>
        )}
      </div>
    );
  }

  return (
    <>
      <button
        ref={frameRef}
        type="button"
        data-no-tap
        className={cn("chat-image-frame", !ready && "chat-image-loading", className)}
        style={frameStyle}
        onClick={() => url && setOpen(true)}
        disabled={!url}
        aria-label={text("Open image: {value0}", { value0: alt })}
      >
        {url ? (
          <img
            src={url}
            alt={alt}
            className={cn("size-full", compact ? "object-cover" : "object-contain", ready ? "opacity-100" : "opacity-0")}
            onLoad={(e) => {
              setLoaded(url);
              const img = e.currentTarget;
              if (img.naturalWidth && img.naturalHeight) setNatural({ w: img.naturalWidth, h: img.naturalHeight });
            }}
            onError={() => setBroken(url)}
            draggable={false}
          />
        ) : null}
      </button>
      <ChatImageViewer open={open} onOpenChange={setOpen} src={url} alt={alt} caption={caption} returnFocusTo={frameRef} />
    </>
  );
}
