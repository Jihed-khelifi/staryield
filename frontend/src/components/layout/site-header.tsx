"use client";
import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Menu, UserRound, Sun, Sparkles, MessageCircle, LogOut } from "lucide-react";
import { useI18n } from "@/i18n/i18n-provider";
import { useSession } from "@/lib/auth/session-provider";
import { psychicsPath } from "@/lib/psychics-link";
import { PromoBanner } from "@/components/layout/promo-banner";
import { cn } from "@/lib/utils";
import { SunLogo } from "@/components/brand/sun-logo";
import { ChatShortcut } from "@/components/layout/chat-shortcut";
import { LocaleSwitcher } from "@/components/layout/locale-switcher";
import { HelpSupportBlock, UserAvatar, UserMenu, useAccountLinks } from "@/components/layout/user-menu";
import { NotificationBell } from "@/components/notifications/notification-bell";
import { AdvisorHeaderActions, WalletPill } from "@/components/psychic/header-actions";
import { TopUpLink } from "@/components/billing/top-up/top-up-provider"; // revisions 5.0
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";

export function SiteHeader({
  publicSite = false,
  promo = false,
}: {
  publicSite?: boolean;
  promo?: boolean;
}) {
  const { text, locale, t } = useI18n();
  const { status, user, wallet, signOut } = useSession();
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const signedIn = status === "authenticated" && Boolean(user);
  // Advisors work from the chatroom and their profile (Figma 1174:2294).
  const advisor = signedIn && user?.role === "psychic";
  const items = publicSite
    ? [
        ["Psychics", psychicsPath(signedIn)],
        ["Natal Chart", "/calculators/natal-chart"],
        ["Path of Life", "/calculators/path-of-life"],
        ["Astrology", "/calculators/astrology"],
      ]
    : advisor
      ? [
          [t("nav.chatroom"), "/chatroom"],
          [t("nav.profile"), "/profile"],
        ]
      : [
          [t("nav.psychics"), psychicsPath(signedIn)],
          [t("nav.chatroom"), "/chatroom"],
          [t("nav.profile"), "/profile"],
        ];
  const active = (href: string) => pathname.startsWith(`/${locale}${href}`);
  const accountLinks = useAccountLinks();
  const menuLinks = signedIn
    ? [...items, ...accountLinks.map((l) => [l.label, l.href] as [string, string]).filter(([, href]) => !items.some(([, h]) => h === href))]
    : items;
  return (
    <>
      {promo && <PromoBanner />}
      <header className="site-header">
        <Link className="brand" href={`/${locale}`}>
          <SunLogo />
          <span>{text("Staryield")}</span>
        </Link>
        <nav
          aria-label={t("nav.menuTitle")}
          className={`desktop-nav ${publicSite ? "public-nav" : ""}`}
        >
          {items.map(([label, href]) => (
            <Link
              key={href}
              className={active(href) ? "active" : ""}
              aria-current={active(href) ? "page" : undefined}
              href={`/${locale}${href}`}
            >
              {text(label)}
            </Link>
          ))}
        </nav>
        <div className="header-actions">
          <LocaleSwitcher className={cn("rounded-full border border-gold/40 bg-cream/60 px-2 sm:px-3", signedIn && "max-md:hidden")} />
          {/* revisions 3.0: chat shortcut with unread badge, every header, signed-in users only. */}
          <ChatShortcut />
          {!signedIn ? (
            <div className={publicSite ? "hidden items-center gap-4 lg:flex" : "flex items-center gap-4"}>
              <Link href={`/${locale}/login`}>{text("Log In")}</Link>
              <Button asChild className={publicSite ? "gold-button w-40" : "gold-button max-md:hidden"}>
                <Link href={`/${locale}/signup`}>{text("Sign Up")}</Link>
              </Button>
            </div>
          ) : advisor ? (
            <AdvisorHeaderActions />
          ) : (
            <>
              {user?.role === "client" ? (
                // revisions 5.0: the balance opens the top-up in place; the href stays for middle-click and no-JS.
                <TopUpLink
                  href={`/${locale}/credits`}
                  className="balance-pill"
                  aria-label={text("Balance: {value0} credits", { value0: wallet?.balance ?? 0 })}
                >
                  <span className="lg:hidden">✦ </span>
                  <span className="hidden lg:inline">{text("Balance: ")}</span>
                  {(wallet?.balance ?? 0).toLocaleString(locale)}{" "}
                  <span className="balance-pill-unit md:capitalize">{text("credits")}</span>
                </TopUpLink>
              ) : null}
              <NotificationBell />
              <UserMenu className="user-menu-trigger hidden md:flex" />
            </>
          )}
          <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                aria-label={t("nav.openMenu")}
                className={publicSite ? "lg:hidden" : signedIn ? "md:hidden rounded-full p-0" : "hidden"}
                data-testid="mobile-menu-trigger"
              >
                {signedIn && !publicSite ? <UserAvatar size={32} /> : <Menu />}
              </Button>
            </SheetTrigger>
            <SheetContent>
              <SheetHeader>
                <SheetTitle>{text("Staryield")}</SheetTitle>
              </SheetHeader>
              <nav className="flex flex-col gap-6 px-6">
                {advisor ? <WalletPill className="self-start" onNavigate={() => setOpen(false)} /> : null}
                {signedIn && !publicSite ? (
                  <>
                    {/* The chatroom hides the bottom bar on phones: the same destinations live here. */}
                    <ul className="flex flex-col gap-1" aria-label={text("Main navigation")} data-testid="mobile-menu-links">
                      {MOBILE_NAV_LINKS.map(({ label, href, icon: Icon }) => (
                        <li key={label}>
                          <Link
                            onClick={() => setOpen(false)}
                            href={`/${locale}${href}`}
                            className="mobile-menu-link"
                            aria-current={isMobileNavActive(pathname, locale, label, href) ? "page" : undefined}
                          >
                            <Icon size={18} strokeWidth={1.5} aria-hidden />
                            {text(label)}
                          </Link>
                        </li>
                      ))}
                    </ul>
                    {accountLinks
                      .filter((l) => !MOBILE_NAV_LINKS.some((m) => m.href === l.href))
                      .map((l) => (
                        <Link key={l.href} onClick={() => setOpen(false)} href={`/${locale}${l.href}`}>
                          {text(l.label)}
                        </Link>
                      ))}
                  </>
                ) : (
                  menuLinks.map(([label, href]) => (
                    <Link key={href} onClick={() => setOpen(false)} href={`/${locale}${href}`}>
                      {text(label)}
                    </Link>
                  ))
                )}
                {signedIn ? <HelpSupportBlock onNavigate={() => setOpen(false)} /> : null}
                {signedIn ? (
                  <button
                    type="button"
                    className="flex items-center gap-2 text-left"
                    onClick={() => {
                      setOpen(false);
                      void signOut();
                    }}
                  >
                    <LogOut size={16} aria-hidden className="text-amber" />
                    {text("Sign out")}
                  </button>
                ) : (
                  <>
                    <Link onClick={() => setOpen(false)} href={`/${locale}/login`}>
                      {text("Log In ")}
                    </Link>
                    <Link onClick={() => setOpen(false)} href={`/${locale}/signup`}>
                      {text("Sign Up ")}
                    </Link>
                  </>
                )}
                <LocaleSwitcher />
              </nav>
            </SheetContent>
          </Sheet>
        </div>
      </header>
    </>
  );
}
/** Bottom bar entries; also listed (with icons) in the header menu sheet and the chat switcher. */
export const MOBILE_NAV_LINKS = [
  { label: "Today", href: "/profile", icon: Sun },
  { label: "Psychics", href: "/psychics", icon: Sparkles },
  { label: "Chat", href: "/chatroom", icon: MessageCircle },
  { label: "Profile", href: "/profile/chart", icon: UserRound },
] as const;

export function isMobileNavActive(path: string, locale: string, label: string, href: string): boolean {
  return path === `/${locale}${href}` || (label === "Profile" && /\/profile\/(settings|analytics)/.test(path)) || (href === "/chatroom" && path.startsWith(`/${locale}/chatroom`));
}

export function MobileNavigation() {
  const { text, locale } = useI18n();
  const path = usePathname();
  return (
    <nav className="mobile-navigation" aria-label={text("Main navigation")}>
      {MOBILE_NAV_LINKS.map(({ label, href, icon: Icon }) => (
        <Link key={label} href={`/${locale}${href}`} className={isMobileNavActive(path, locale, label, href) ? "active" : ""}>
          <Icon size={19} strokeWidth={1} />
          <span>{text(label)}</span>
        </Link>
      ))}
    </nav>
  );
}
