import { cn } from "@/lib/utils"
import { SiteHeader } from "@/components/layout/site-header"

/**
 * Page container matching the Figma frames: 1440 wide with 32px gutters,
 * 20px above the navbar and a 16px gap down to the content.
 */
export function AppFrame({
  children,
  /** Fill the viewport exactly (chatroom-style screens with inner scrolling). */
  fill = false,
  className,
}: {
  children: React.ReactNode
  fill?: boolean
  className?: string
}) {
  return (
    <div
      className={cn(
        "mx-auto flex w-full max-w-[1440px] flex-col gap-4 px-4 pt-5 pb-6 sm:px-6 xl:px-8",
        fill ? "h-dvh" : "min-h-dvh",
        className
      )}
    >
      <SiteHeader />
      {children}
    </div>
  )
}
