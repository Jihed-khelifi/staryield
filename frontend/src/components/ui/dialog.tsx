"use client"

import * as React from "react"
import { Dialog as DialogPrimitive } from "radix-ui"
import { XIcon } from "lucide-react"

import { useI18n } from "@/i18n/i18n-provider"
import { cn } from "@/lib/utils"

/** Backdrop style: "blur" (default, revisions 2.0) or a plain dark "dim". */
type OverlayKind = "blur" | "dim"
type DialogSize = "sm" | "md" | "lg"

function Dialog({ ...props }: React.ComponentProps<typeof DialogPrimitive.Root>) {
  return <DialogPrimitive.Root data-slot="dialog" {...props} />
}

function DialogTrigger({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Trigger>) {
  return <DialogPrimitive.Trigger data-slot="dialog-trigger" {...props} />
}

function DialogPortal({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Portal>) {
  return <DialogPrimitive.Portal data-slot="dialog-portal" {...props} />
}

function DialogClose({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Close>) {
  return <DialogPrimitive.Close data-slot="dialog-close" {...props} />
}

/**
 * Full-screen backdrop. The blur/dim styles live in globals.css
 * (`[data-slot="dialog-overlay"]`) so bespoke dialogs (e.g. the top-up
 * checkout) can reuse them with their own positioning class.
 */
function DialogOverlay({
  className,
  overlay = "blur",
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Overlay> & {
  overlay?: OverlayKind
}) {
  return (
    <DialogPrimitive.Overlay
      data-slot="dialog-overlay"
      data-overlay={overlay}
      className={cn(
        "fixed inset-0 z-60 duration-150 data-open:animate-in data-open:fade-in-0 data-closed:animate-out data-closed:fade-out-0 motion-reduce:animate-none",
        className
      )}
      {...props}
    />
  )
}

const sizes: Record<DialogSize, string> = {
  sm: "md:max-w-sm",
  md: "md:max-w-lg",
  lg: "md:max-w-2xl",
}

/**
 * Modal dialog. Below 768px it is a bottom sheet (thumb reach, full width);
 * from 768px it is a centred card. Focus is trapped and restored by Radix;
 * Escape and the close button dismiss it.
 */
function DialogContent({
  className,
  children,
  overlay = "blur",
  size = "md",
  showCloseButton = true,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Content> & {
  overlay?: OverlayKind
  size?: DialogSize
  showCloseButton?: boolean
}) {
  const { text } = useI18n()
  return (
    <DialogPortal>
      <DialogOverlay overlay={overlay} />
      <DialogPrimitive.Content
        data-slot="dialog-content"
        data-size={size}
        className={cn(
          // Phones: bottom sheet.
          "fixed inset-x-0 bottom-0 z-61 flex max-h-[92dvh] w-full flex-col gap-4 overflow-y-auto rounded-t-2xl border border-gold/40 bg-cream-light p-6 pb-[max(1.5rem,env(safe-area-inset-bottom))] text-ink shadow-[0_-8px_32px_rgb(21_16_11/0.18)] outline-none",
          "duration-200 data-open:animate-in data-open:fade-in-0 data-open:slide-in-from-bottom-10 data-closed:animate-out data-closed:fade-out-0 data-closed:slide-out-to-bottom-10 motion-reduce:animate-none",
          // Tablets and desktop: centred card.
          "md:inset-x-auto md:top-1/2 md:bottom-auto md:left-1/2 md:w-[calc(100vw-32px)] md:-translate-x-1/2 md:-translate-y-1/2 md:rounded-2xl md:pb-6 md:shadow-[0_24px_40px_rgb(21_16_11/0.25)] md:data-open:slide-in-from-bottom-0 md:data-open:zoom-in-95 md:data-closed:slide-out-to-bottom-0 md:data-closed:zoom-out-95",
          sizes[size],
          className
        )}
        {...props}
      >
        {children}
        {showCloseButton ? (
          <DialogPrimitive.Close
            data-slot="dialog-close"
            className="absolute top-4 right-4 inline-flex size-9 items-center justify-center rounded-full bg-warm-gray/30 text-ink transition-colors hover:bg-warm-gray/50"
          >
            <XIcon aria-hidden className="size-4" />
            <span className="sr-only">{text("Close")}</span>
          </DialogPrimitive.Close>
        ) : null}
      </DialogPrimitive.Content>
    </DialogPortal>
  )
}

function DialogHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-header"
      className={cn("flex flex-col gap-1.5 pr-10", className)}
      {...props}
    />
  )
}

function DialogFooter({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-footer"
      className={cn(
        "mt-2 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end",
        className
      )}
      {...props}
    />
  )
}

function DialogTitle({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Title>) {
  return (
    <DialogPrimitive.Title
      data-slot="dialog-title"
      className={cn("font-display text-2xl leading-tight", className)}
      {...props}
    />
  )
}

function DialogDescription({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Description>) {
  return (
    <DialogPrimitive.Description
      data-slot="dialog-description"
      className={cn("text-sm text-muted-foreground", className)}
      {...props}
    />
  )
}

export {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
}
export type { DialogSize, OverlayKind }
