"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import type { ChatMessage } from "@/lib/api/types";
import type { PagedList } from "@/lib/use-paged-list";
import { ErrorState } from "@/components/feedback/error-state";
import { CursorPager } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";

/** Saved messages per page (the API pages them; more than five means pages). */
export const SAVED_PAGE_SIZE = 5;

/**
 * Loading, error and empty states around one page of saved messages, with
 * Previous/Next when there is more than one page. `render` draws the items
 * of the page on screen.
 */
export function SavedPages({
  saved,
  render,
  emptyClassName,
  showPager = true,
}: {
  saved: PagedList<ChatMessage>;
  render: (items: ChatMessage[]) => React.ReactNode;
  emptyClassName?: string;
  /** Hide Previous/Next (e.g. while a preview is collapsed). */
  showPager?: boolean;
}) {
  const { text } = useI18n();
  if (saved.error && saved.items.length === 0) {
    return <ErrorState error={saved.error} onRetry={saved.reload} compact title={text("Saved messages could not be loaded.")} />;
  }
  if (saved.loading && saved.items.length === 0) {
    return (
      <div className="grid gap-2" role="status" aria-label={text("Loading saved messages…")} data-testid="saved-loading">
        <Skeleton className="h-12 w-full rounded-lg bg-cream/70" />
        <Skeleton className="h-12 w-full rounded-lg bg-cream/70" />
      </div>
    );
  }
  if (saved.items.length === 0) {
    return <p className={emptyClassName ?? "text-xs font-light"}>{text("No saved messages yet. Open a message's menu and choose Bookmark to save it.")}</p>;
  }
  return (
    <>
      {render(saved.items)}
      {showPager ? (
        <CursorPager
          page={saved.page}
          hasPrev={saved.hasPrev}
          hasNext={saved.hasNext}
          onPrev={saved.prev}
          onNext={saved.next}
          loading={saved.loading}
          label={text("Saved messages pages")}
          className="chat-panel-pager"
        />
      ) : null}
    </>
  );
}
