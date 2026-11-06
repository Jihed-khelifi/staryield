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

/**
 * 409 version_conflict: the settings were saved elsewhere (another tab or
 * device) after this page loaded. The user either reloads them — losing
 * the edits made here — or saves the fields they changed on top.
 */
export function ConflictDialog({
  open,
  busy,
  onReload,
  onOverwrite,
  onCancel,
}: {
  open: boolean;
  busy: boolean;
  onReload: () => void;
  onOverwrite: () => void;
  onCancel: () => void;
}) {
  const { text } = useI18n();
  return (
    <AlertDialog open={open} onOpenChange={(next) => (!next && !busy ? onCancel() : undefined)}>
      <AlertDialogContent data-testid="conflict-dialog">
        <AlertDialogHeader>
          <AlertDialogTitle>{text("Your settings changed somewhere else")}</AlertDialogTitle>
          <AlertDialogDescription>
            {text("They were saved on another device or tab after you opened this page. Reload them to see the latest version, or save the fields you changed here over them.")}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel
            disabled={busy}
            onClick={(event) => {
              event.preventDefault();
              onReload();
            }}
            data-testid="conflict-reload"
          >
            {text("Reload settings")}
          </AlertDialogCancel>
          <AlertDialogAction
            disabled={busy}
            aria-busy={busy}
            onClick={(event) => {
              event.preventDefault();
              onOverwrite();
            }}
            data-testid="conflict-overwrite"
          >
            {text(busy ? "Saving…" : "Save my changes")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
