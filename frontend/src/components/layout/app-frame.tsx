import { cn } from "@/lib/utils";
import { SiteHeader } from "@/components/layout/site-header";
import { MobileNavigation } from "@/components/layout/site-header";

/**
 * Chatroom frame with a shared desktop header, inner scrolling, and mobile navigation.
 */
export function AppFrame({
  children,
  /** Fill the viewport exactly (chatroom-style screens with inner scrolling). */
  fill = false,
  className,
}: {
  children: React.ReactNode;
  fill?: boolean;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "dashboard-page mx-auto flex w-full max-w-[1440px] flex-col gap-4 pb-6 max-md:pb-[88px]",
        fill ? "h-dvh" : "min-h-dvh",
        className,
      )}
    >
      <SiteHeader />
      {children}
      <MobileNavigation />
    </div>
  );
}
