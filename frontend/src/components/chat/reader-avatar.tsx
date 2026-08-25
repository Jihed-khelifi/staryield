import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { cn } from "@/lib/utils"

/**
 * The 40 × 40 square avatar used across the chatroom (Figma nodes 163:611,
 * 163:619, 163:635). shadcn's Avatar is round by default, so both the root and
 * its ring are squared off to 8px here.
 */
export function ReaderAvatar({
  initial,
  className,
}: {
  initial: string
  className?: string
}) {
  return (
    <Avatar
      size="lg"
      className={cn("size-10 rounded-lg after:rounded-lg after:border-0", className)}
    >
      <AvatarFallback className="rounded-lg bg-cream font-serif text-[18px] font-bold text-ink">
        {initial}
      </AvatarFallback>
    </Avatar>
  )
}
