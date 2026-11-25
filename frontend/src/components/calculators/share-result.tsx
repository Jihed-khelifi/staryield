"use client";

import * as React from "react";
import { AlertCircle, CheckCircle2, Download, Link2, Share2 } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { track } from "@/lib/analytics";
import type { CalculatorResult } from "@/components/calculators/calculate";
import { useShareImage } from "@/components/calculators/use-share-image";
import { shareCardSpec } from "@/lib/share/card-spec";
import { canShareFile, copyText, downloadBlob, shareFile, shareFileName, shareNative, type ShareMethod } from "@/lib/share/share-actions";
import { calculatorShareUrl } from "@/lib/share/share-url";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * Opens the share dialog of the result on screen. The result view provides it
 * once there is something of the visitor's to share (null on the sample and
 * while the result loads), so the buttons placed through the result's
 * sections need no prop.
 */
const ShareResultContext = React.createContext<(() => void) | null>(null);

export function ShareResultProvider({ onShare, children }: { onShare: (() => void) | null; children: React.ReactNode }) {
  return <ShareResultContext.Provider value={onShare}>{children}</ShareResultContext.Provider>;
}

/** The share dialog's opener, or null when this result cannot be shared. */
export function useShareResult(): (() => void) | null {
  return React.useContext(ShareResultContext);
}

/** "Share your result": opens the share dialog (one dialog per result, several buttons); nothing when there is nothing to share. */
export function ShareResultButton({ className }: { className?: string }) {
  const { text } = useI18n();
  const onShare = useShareResult();
  if (!onShare) return null;
  return (
    <button type="button" className={className ?? "calc-email-button"} onClick={onShare} data-testid="share-result-button">
      <Share2 aria-hidden className="size-4" />
      {text("Share your result")}
    </button>
  );
}

/** `seq` renews the message's node, so the same outcome twice ("Link copied.") is announced twice. */
type Feedback = { tone: "success" | "info" | "error"; message: string; seq: number } | null;

/**
 * The share dialog of a calculator result: the 9:16 image (rendered when it
 * opens), "Share" (Web Share API with the PNG when the browser can), "Download
 * image" and "Copy link". Birth data never appears on the image or in the
 * link (the link is the calculator's page without its fragment).
 */
export function ShareResultDialog({ result, open, onOpenChange }: { result: CalculatorResult; open: boolean; onOpenChange: (open: boolean) => void }) {
  const { text, locale } = useI18n();
  const kind = result.kind;
  const spec = React.useMemo(() => shareCardSpec(result, text), [result, text]);
  const { state, retry } = useShareImage(spec, open);
  const [feedback, setFeedbackState] = React.useState<Feedback>(null);
  const setFeedback = React.useCallback(
    (next: Omit<NonNullable<Feedback>, "seq"> | null) => setFeedbackState((current) => (next ? { ...next, seq: (current?.seq ?? 0) + 1 } : null)),
    [],
  );
  const [manualLink, setManualLink] = React.useState(false);
  const [sharing, setSharing] = React.useState(false);
  const url = React.useMemo(() => (open ? calculatorShareUrl(kind, locale) : ""), [open, kind, locale]);
  const blob = state.status === "ready" ? state.blob : null;
  const file = React.useMemo(() => (blob ? shareFile(blob, kind) : null), [blob, kind]);
  const nativeShare = React.useMemo(() => (file ? canShareFile(file) : false), [file]);

  // Every opening starts clean (state adjusted during render) and counts once.
  const [wasOpen, setWasOpen] = React.useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) {
      setFeedback(null);
      setManualLink(false);
    }
  }
  React.useEffect(() => {
    if (open) track("share_opened", { kind });
  }, [open, kind]);
  // Several buttons open this dialog (none is a Radix trigger): focus goes back to the one used.
  const opener = React.useRef<HTMLElement | null>(null);

  // The dialog's own status region (a polite live region) announces the outcome, once.
  function done(method: ShareMethod, message: string) {
    track("share_completed", { kind, method });
    setFeedback({ tone: "success", message });
  }

  async function share() {
    if (!file || sharing) return;
    setSharing(true);
    try {
      const outcome = await shareNative({ file, text: spec.shareText, url, title: spec.headline });
      if (outcome === "shared") done("native", text("Shared. Thank you!"));
      // A dismissed share sheet is the visitor's choice: no message, no error.
    } catch {
      setFeedback({ tone: "error", message: text("Sharing did not work. You can download the image or copy the link instead.") });
    } finally {
      setSharing(false);
    }
  }

  function download() {
    if (!blob) return;
    try {
      downloadBlob(blob, shareFileName(kind));
      done("download", text("Image saved. Post it anywhere you like."));
    } catch {
      setFeedback({ tone: "error", message: text("The image could not be saved. Press and hold the image to save it.") });
    }
  }

  async function copy() {
    if (await copyText(url)) {
      setManualLink(false);
      done("copy_link", text("Link copied."));
    } else {
      setManualLink(true);
      setFeedback({ tone: "info", message: text("Copy the link below.") });
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        size="md"
        data-testid="share-dialog"
        aria-describedby="share-dialog-description"
        onOpenAutoFocus={() => {
          opener.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
        }}
        onCloseAutoFocus={(event) => {
          if (!opener.current?.isConnected) return;
          event.preventDefault();
          opener.current.focus();
        }}
      >
        <DialogHeader>
          <DialogTitle>{text("Share your result")}</DialogTitle>
          <DialogDescription id="share-dialog-description">{text("A picture for your story or your friends. It shows your result only, never your birth details.")}</DialogDescription>
        </DialogHeader>

        <div className="grid justify-items-center gap-4" aria-busy={state.status === "rendering"}>
          {state.status === "ready" ? (
            // eslint-disable-next-line @next/next/no-img-element -- an object URL of the rendered PNG
            <img src={state.url} alt={spec.alt} width={1080} height={1920} className="aspect-[9/16] max-h-[52dvh] w-auto rounded-lg border border-gold/50 shadow-md" data-testid="share-preview" />
          ) : state.status === "error" ? (
            <div role="alert" className="flex w-full items-start gap-2 rounded-lg border border-danger/50 bg-danger/5 p-3 text-sm" data-testid="share-error">
              <AlertCircle aria-hidden className="mt-0.5 size-4 shrink-0 text-danger" />
              <div className="grid gap-2">
                <p>{text("We could not create your image.")}</p>
                <Button type="button" variant="outline" size="sm" className="w-fit border-gold bg-cream" onClick={retry} data-testid="share-retry">
                  {text("Try again")}
                </Button>
              </div>
            </div>
          ) : (
            <div className="grid justify-items-center gap-2" data-testid="share-loading">
              <p role="status" className="sr-only">
                {text("Preparing your image…")}
              </p>
              <Skeleton className="aspect-[9/16] max-h-[52dvh] w-[min(60vw,260px)] rounded-lg" />
            </div>
          )}

          {state.status === "ready" ? (
            <div className="grid w-full gap-2">
              {nativeShare ? (
                <Button type="button" className="gold-button h-12 w-full" onClick={() => void share()} disabled={sharing} aria-busy={sharing} data-testid="share-native">
                  <Share2 aria-hidden className="size-4" />
                  {text("Share")}
                </Button>
              ) : null}
              <Button type="button" variant={nativeShare ? "outline" : "default"} className={nativeShare ? "h-11 w-full border-gold bg-cream" : "gold-button h-12 w-full"} onClick={download} data-testid="share-download">
                <Download aria-hidden className="size-4" />
                {text("Download image")}
              </Button>
              <Button type="button" variant="outline" className="h-11 w-full border-gold bg-cream" onClick={() => void copy()} data-testid="share-copy">
                <Link2 aria-hidden className="size-4" />
                {text("Copy link")}
              </Button>
              {!nativeShare ? <p className="text-center text-xs font-light text-[var(--muted-ink)]">{text("This browser cannot open the share menu with an image. Download it or copy the link.")}</p> : null}
            </div>
          ) : null}

          {manualLink ? (
            <input
              readOnly
              value={url}
              aria-label={text("Link to this calculator")}
              className="w-full rounded-md border border-gold/60 bg-white px-3 py-2 text-sm"
              onFocus={(event) => event.currentTarget.select()}
              data-testid="share-link-field"
            />
          ) : null}

          <div role="status" aria-live="polite" className="min-h-5 text-center text-sm" data-testid="share-feedback">
            {feedback ? (
              <span key={feedback.seq} className={feedback.tone === "error" ? "text-danger" : "inline-flex items-center gap-1.5"}>
                {feedback.tone === "success" ? <CheckCircle2 aria-hidden className="size-4 text-success" /> : null}
                {feedback.message}
              </span>
            ) : null}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
