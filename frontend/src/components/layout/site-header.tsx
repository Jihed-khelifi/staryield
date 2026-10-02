"use client";
import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Menu, UserRound, Sun, Sparkles, MessageCircle } from "lucide-react";
import { useI18n } from "@/i18n/i18n-provider";
import { SunLogo } from "@/components/brand/sun-logo";
import { LocaleSwitcher } from "@/components/layout/locale-switcher";
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
  const { locale, t } = useI18n();
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
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
  return (
    <>
      {promo && (
        <Link href={`/${locale}/signup`} className="promo-bar paper-page">
          <span>3 MINUTES FREE + 80% OFF FOR NEW CUSTOMERS</span>
          <span className="hidden sm:inline">CLAIM INTRO OFFER →</span>
        </Link>
      )}
      <header className="site-header">
        <Link
          className={`brand ${pathname.endsWith("/chatroom") ? "chat-desktop-brand" : ""}`}
          href={`/${locale}`}
        >
          <SunLogo />
          <span>Staryield</span>
        </Link>
        {pathname.endsWith("/chatroom") && (
          <Link className="chat-mobile-heading" href={`/${locale}/psychics`}>
            <span aria-hidden="true">‹</span> Chatroom
          </Link>
        )}
        <nav aria-label={t("nav.menuTitle")} className="desktop-nav">
          {items.map(([label, href]) => (
            <Link
              key={href}
              className={active(href) ? "active" : ""}
              aria-current={active(href) ? "page" : undefined}
              href={`/${locale}${href}`}
            >
              {label}
            </Link>
          ))}
        </nav>
        <div className="flex items-center gap-3">
          {publicSite ? (
            <div className="hidden items-center gap-4 md:flex">
              <Link href={`/${locale}/login`}>Log In</Link>
              <Button asChild className="gold-button w-40">
                <Link href={`/${locale}/signup`}>Sign In</Link>
              </Button>
            </div>
          ) : (
            <>
              <Link href={`/${locale}/credits`} className="balance-pill">
                <span className="md:hidden">✦ </span>
                <span className="hidden md:inline">Balance: </span>320{" "}
                <span className="md:capitalize">credits</span>
              </Link>
              <Link
                href={`/${locale}/profile/settings`}
                aria-label="Account settings"
                className="hidden size-10 items-center justify-center rounded-full bg-gold text-cream-light md:flex"
              >
                <UserRound size={20} />
              </Link>
            </>
          )}
          <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                aria-label={t("nav.openMenu")}
                className={publicSite ? "md:hidden" : "hidden"}
              >
                <Menu />
              </Button>
            </SheetTrigger>
            <SheetContent>
              <SheetHeader>
                <SheetTitle>Staryield</SheetTitle>
              </SheetHeader>
              <nav className="flex flex-col gap-6 px-6">
                {items.map(([label, href]) => (
                  <Link
                    key={href}
                    onClick={() => setOpen(false)}
                    href={`/${locale}${href}`}
                  >
                    {label}
                  </Link>
                ))}
                <Link onClick={() => setOpen(false)} href={`/${locale}/login`}>
                  Log In
                </Link>
                <Link onClick={() => setOpen(false)} href={`/${locale}/signup`}>
                  Sign Up
                </Link>
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
  const { locale } = useI18n();
  const path = usePathname();
  const links = [
    ["Today", "/profile", Sun],
    ["Psychics", "/psychics", Sparkles],
    ["Chat", "/chatroom", MessageCircle],
    ["Profile", "/profile/chart", UserRound],
  ] as const;
  return (
    <nav className="mobile-navigation" aria-label="Main navigation">
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
          <span>{label}</span>
        </Link>
      ))}
    </nav>
  );
}
