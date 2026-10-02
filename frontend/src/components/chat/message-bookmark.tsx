"use client";

import * as React from "react";
import Image from "next/image";

import { useI18n } from "@/i18n/i18n-provider";
import { cn } from "@/lib/utils";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

/**
 * The 9 × 9 bookmark that sits beside every bubble (Figma node 171:598).
 * The glyph is the exported asset; the button only adds a touch target.
 */
export function MessageBookmark({
  className,
  saved,
  onToggle,
}: {
  className?: string;
  saved?: boolean;
  onToggle?: () => void;
}) {
  const { t } = useI18n();
  const [localBookmark, setBookmarked] = React.useState(false);
  const bookmarked = saved ?? localBookmark;
  const label = t(
    bookmarked ? "chatroom.feed.bookmarked" : "chatroom.feed.bookmark",
  );

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label={label}
          aria-pressed={bookmarked}
          onClick={() =>
            onToggle ? onToggle() : setBookmarked((value) => !value)
          }
          className={cn(
            "-m-2 flex shrink-0 cursor-pointer items-center justify-center rounded-sm p-2 transition-opacity focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
            bookmarked ? "opacity-100" : "opacity-70 hover:opacity-100",
            className,
          )}
        >
          <Image
            src="https://assets.staryield.net/assets/bookmark.png"
            alt=""
            width={9}
            height={9}
            className="size-[9px] object-cover"
          />
        </button>
      </TooltipTrigger>
      <TooltipContent side="top">{label}</TooltipContent>
    </Tooltip>
  );
}
