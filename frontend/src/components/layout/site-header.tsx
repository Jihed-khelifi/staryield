"use client"

import * as React from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { FiMenu } from "react-icons/fi"

import { useI18n } from "@/i18n/i18n-provider"
import { cn } from "@/lib/utils"
import { StaryieldMark } from "@/components/brand/staryield-mark"
import { LocaleSwitcher } from "@/components/layout/locale-switcher"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet"

const navItems = [
  { href: "/psychics", labelKey: "nav.psychics" },
  { href: "/chatroom", labelKey: "nav.chatroom" },
  { href: "/profile", labelKey: "nav.profile" },
] as const

export function SiteHeader() {
  const { locale, t } = useI18n()
  const pathname = usePathname()
  const [menuOpen, setMenuOpen] = React.useState(false)

  const isActive = (href: string) => pathname === `/${locale}${href}`

  return (
    <header className="flex h-[35px] w-full items-start justify-between border-b border-border pb-3">
      <Link
        href={`/${locale}/chatroom`}
        className="flex items-center gap-[5px] rounded-sm focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
      >
        <StaryieldMark />
        <span className="font-display text-lg leading-none text-ink sm:text-[20px]">
          STARYIELD
        </span>
      </Link>

      <div className="flex items-center gap-2">
        <nav
          aria-label={t("nav.menuTitle")}
          className="hidden items-center font-serif text-[18px] text-deep-black md:flex md:gap-10 lg:gap-[102px]"
        >
          {navItems.map((item) => (
            <Link
              key={item.href}
              href={`/${locale}${item.href}`}
              aria-current={isActive(item.href) ? "page" : undefined}
              className={cn(
                "whitespace-nowrap rounded-sm transition-colors hover:text-gold focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
                isActive(item.href) && "underline decoration-solid"
              )}
            >
              {t(item.labelKey)}
            </Link>
          ))}
        </nav>

        <LocaleSwitcher className="hidden md:inline-flex" />

        <Sheet open={menuOpen} onOpenChange={setMenuOpen}>
          <SheetTrigger asChild>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={t("nav.openMenu")}
              className="md:hidden"
            >
              <FiMenu className="size-5" aria-hidden />
            </Button>
          </SheetTrigger>
          <SheetContent side="right" className="w-72 bg-cream">
            <SheetHeader>
              <SheetTitle className="font-display text-base tracking-wide uppercase">
                {t("nav.menuTitle")}
              </SheetTitle>
            </SheetHeader>
            <nav className="flex flex-col gap-1 px-4 font-serif text-[18px]">
              {navItems.map((item) => (
                <Link
                  key={item.href}
                  href={`/${locale}${item.href}`}
                  onClick={() => setMenuOpen(false)}
                  aria-current={isActive(item.href) ? "page" : undefined}
                  className={cn(
                    "rounded-md px-2 py-2 text-ink transition-colors hover:bg-gold/15",
                    isActive(item.href) && "bg-sage/40 underline"
                  )}
                >
                  {t(item.labelKey)}
                </Link>
              ))}
            </nav>
            <Separator className="mx-4 w-auto" />
            <div className="flex items-center justify-between px-4 pb-4">
              <span className="font-serif text-sm text-muted-foreground">
                {t("nav.language")}
              </span>
              <LocaleSwitcher />
            </div>
          </SheetContent>
        </Sheet>
      </div>
    </header>
  )
}
