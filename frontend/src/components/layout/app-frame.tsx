import { cn } from "@/lib/utils";
import { SiteHeader } from "@/components/layout/site-header";
import { MobileNavigation } from "@/components/layout/site-header";

/**
 * Account/chatroom frame with a shared header, optional inner scrolling and
 * the mobile bottom navigation.
 */
export function AppFrame({
  children,
  /** Fill the viewport exactly (chatroom-style screens with inner scrolling). */
  fill = false,
  /** Use the width up to 2560px instead of 1440px (chatroom on large screens). */
  wide = false,
  /** Show the floating bottom navigation below 768px (the chatroom hides it for space). */
  mobileNav = true,
  className,
}: {
  children: React.ReactNode;
  fill?: boolean;
  wide?: boolean;
  mobileNav?: boolean;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "dashboard-page mx-auto flex w-full flex-col gap-4 pb-6",
        wide ? "max-w-[2560px]" : "max-w-[1440px]",
        mobileNav && "max-md:pb-[88px]",
        fill ? "h-dvh" : "min-h-dvh",
        // Chatroom on phones follows the visible area (keyboard) and has no reserved nav space.
        fill && !mobileNav && "chat-frame",
        className,
      )}
      data-mobile-nav={mobileNav ? undefined : "off"}
    >
      <SiteHeader />
      {children}
      {mobileNav ? <MobileNavigation /> : null}
    </div>
  );
}
