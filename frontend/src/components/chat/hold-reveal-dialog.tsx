"use client";

import { useI18n } from "@/i18n/i18n-provider";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import type { RevealConfirm } from "@/components/chat/use-hold-actions";

/**
 * revisions 6.0 (§26.1): "Reveal {n} hidden messages from {name} for {price}
 * credits?". The price is the hold's (the server charges its own number). A
 * failed attempt stays on screen with its reason; "Try again" repeats the same
 * request, which the server replays safely.
 */
export function HoldRevealDialog({
  confirm,
  name,
  busy,
  onConfirm,
  onCancel,
}: {
  confirm: RevealConfirm | null;
  name: string;
  busy: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const { text } = useI18n();
  const count = confirm?.count ?? 0;
  const price = confirm?.price ?? 0;
  return (
    <AlertDialog open={confirm !== null} onOpenChange={(open) => !open && !busy && onCancel()}>
      <AlertDialogContent data-testid="hold-reveal-dialog">
        <AlertDialogHeader>
          <AlertDialogTitle>{text("Reveal hidden messages")}</AlertDialogTitle>
          <AlertDialogDescription data-testid="hold-reveal-question">
            {count === 1
              ? text("Reveal 1 hidden message from {value0} for {value1} credits?", { value0: name, value1: price })
              : text("Reveal {value0} hidden messages from {value1} for {value2} credits?", { value0: count, value1: name, value2: price })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        {confirm?.error ? (
          <p role="alert" className="text-sm text-red-800" data-testid="hold-reveal-error">
            {confirm.error}
          </p>
        ) : null}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={busy}>{text("Not now")}</AlertDialogCancel>
          <AlertDialogAction
            disabled={busy}
            aria-busy={busy || undefined}
            data-testid="hold-reveal-confirm"
            onClick={(event) => {
              // Stay open while the request runs (and after it failed): the action closes it on success.
              event.preventDefault();
              onConfirm();
            }}
          >
            {confirm?.error ? text("Try again") : text("Reveal for {value0} credits", { value0: price })}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
