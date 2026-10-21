"use client";
import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Menu, UserRound, Sun, Sparkles, MessageCircle, LogOut } from "lucide-react";
import { useI18n } from "@/i18n/i18n-provider";
import { useSession } from "@/lib/auth/session-provider";
import { cn } from "@/lib/utils";
import { SunLogo } from "@/components/brand/sun-logo";
import { LocaleSwitcher } from "@/components/layout/locale-switcher";
import { UserAvatar, UserMenu, useAccountLinks } from "@/components/layout/user-menu";
import { NotificationBell } from "@/components/notifications/notification-bell";
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
  const items = publicSite
    ? [
        ["Psychics", "/psychics"],
        ["Natal Chart", "/calculators/natal-chart"],
        ["Path of Life", "/calculators/path-of-life"],
        ["Astrology", "/calculators/astrology"],
      ]
    : [
        [t("nav.psychics"), "/psychics"],
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
      {promo && (
        <Link href={`/${locale}/signup`} className="promo-bar paper-page">
          <span>{text("3 MINUTES FREE + 80% OFF FOR NEW CUSTOMERS")}</span>
          <span className="hidden sm:inline">
            {text("CLAIM INTRO OFFER →")}
          </span>
        </Link>
      )}
      <header className="site-header">
        <Link
          className={`brand ${pathname.endsWith("/chatroom") ? "chat-desktop-brand" : ""}`}
          href={`/${locale}`}
        >
          <SunLogo />
          <span>{text("Staryield")}</span>
        </Link>
        {pathname.endsWith("/chatroom") && (
          <Link className="chat-mobile-heading" href={`/${locale}/psychics`}>
            <span aria-hidden="true">‹</span> {text("Chatroom ")}
          </Link>
        )}
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
          {!signedIn ? (
            <div className={publicSite ? "hidden items-center gap-4 lg:flex" : "flex items-center gap-4"}>
              <Link href={`/${locale}/login`}>{text("Log In")}</Link>
              <Button asChild className={publicSite ? "gold-button w-40" : "gold-button max-md:hidden"}>
                <Link href={`/${locale}/signup`}>{text("Sign Up")}</Link>
              </Button>
            </div>
          ) : (
            <>
              {user?.role === "client" ? (
                <Link
                  href={`/${locale}/credits`}
                  className="balance-pill"
                  aria-label={text("Balance: {value0} credits", { value0: wallet?.balance ?? 0 })}
                >
                  <span className="lg:hidden">✦ </span>
                  <span className="hidden lg:inline">{text("Balance: ")}</span>
                  {(wallet?.balance ?? 0).toLocaleString(locale)}{" "}
                  <span className="balance-pill-unit md:capitalize">{text("credits")}</span>
                </Link>
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
                {menuLinks.map(([label, href]) => (
                  <Link
                    key={href}
                    onClick={() => setOpen(false)}
                    href={`/${locale}${href}`}
                  >
                    {text(label)}
                  </Link>
                ))}
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
export function MobileNavigation() {
  const { text, locale } = useI18n();
  const path = usePathname();
  const links = [
    ["Today", "/profile", Sun],
    ["Psychics", "/psychics", Sparkles],
    ["Chat", "/chatroom", MessageCircle],
    ["Profile", "/profile/chart", UserRound],
  ] as const;
  return (
    <nav className="mobile-navigation" aria-label={text("Main navigation")}>
      {links.map(([label, href, Icon]) => (
        <Link
          key={label}
          href={`/${locale}${href}`}
          className={
            path === `/${locale}${href}` ||
            (label === "Profile" && path.includes("/settings"))
              ? "active"
              : ""
          }
        >
          <Icon size={19} strokeWidth={1} />
          <span>{text(label)}</span>
        </Link>
      ))}
    </nav>
  );
}
