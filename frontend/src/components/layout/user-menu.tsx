"use client";
/* eslint-disable @next/next/no-img-element */
import Link from "next/link";
import { ChartColumn, LifeBuoy, LogOut, Mail, MessageCircle, Settings, UserRound } from "lucide-react";

import { COMPANY, supportMailto } from "@/config/company";
import { useI18n } from "@/i18n/i18n-provider";
import { useSession } from "@/lib/auth/session-provider";
import { SupportHours } from "@/components/account/support-contact";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

const ITEM = "gap-2.5 rounded-lg px-2 py-2 text-sm text-ink focus:bg-gold/15 focus:text-ink [&_svg]:text-amber";

/** Account links for the signed-in user (shared by the desktop menu and mobile sheets). */
export function useAccountLinks() {
  const { user } = useSession();
  const links: { label: string; href: string; icon: typeof UserRound }[] = [
    { label: "Profile", href: "/profile", icon: UserRound },
    { label: "Settings", href: "/profile/settings", icon: Settings },
    { label: "Support requests", href: "/support/requests", icon: LifeBuoy },
  ];
  if (user?.role === "psychic") links.push({ label: "Analytics", href: "/profile/analytics", icon: ChartColumn });
  return links;
}

/**
 * Customer-service details for signed-in pages, which have no footer
 * (docs/platform-extensions.md §25.4): the support e-mail, a link to the
 * Contact page and the support hours. Used by the account menu and the mobile
 * account sheet.
 */
export function HelpSupportMenuItems() {
  const { text, locale } = useI18n();
  return (
    <>
      <DropdownMenuLabel className="px-2 pt-2 pb-1 font-serif text-xs font-semibold tracking-wide text-ink/60 uppercase">{text("Help & support")}</DropdownMenuLabel>
      <DropdownMenuItem asChild className={ITEM}>
        <a href={supportMailto()} data-testid="help-support-email">
          <Mail aria-hidden />
          <span className="truncate">{COMPANY.supportEmail}</span>
        </a>
      </DropdownMenuItem>
      <DropdownMenuItem asChild className={ITEM}>
        <Link href={`/${locale}/contact`} data-testid="help-support-contact">
          <MessageCircle aria-hidden />
          {text("Contact us")}
        </Link>
      </DropdownMenuItem>
      <p className="px-2 pb-1.5 text-xs leading-snug text-ink/60" data-testid="help-support-hours">
        <SupportHours />
      </p>
    </>
  );
}

/** The same details in the mobile account sheet (plain links, not menu items). */
export function HelpSupportBlock({ onNavigate }: { onNavigate?: () => void }) {
  const { text, locale } = useI18n();
  return (
    <section aria-label={text("Help & support")} className="grid gap-1.5" data-testid="help-support">
      <p className="text-xs font-semibold tracking-wide text-ink/60 uppercase">{text("Help & support")}</p>
      <a href={supportMailto()} className="flex items-center gap-2" data-testid="help-support-email">
        <Mail size={16} aria-hidden className="text-amber" />
        {COMPANY.supportEmail}
      </a>
      <Link href={`/${locale}/contact`} onClick={onNavigate} className="flex items-center gap-2" data-testid="help-support-contact">
        <MessageCircle size={16} aria-hidden className="text-amber" />
        {text("Contact us")}
      </Link>
      <p className="text-xs leading-snug text-ink/60" data-testid="help-support-hours">
        <SupportHours />
      </p>
    </section>
  );
}

export function UserAvatar({ size = 40 }: { size?: number }) {
  const { user } = useSession();
  const name = user ? user.profile.nickname || user.display_name : "";
  return (
    <span
      className="flex items-center justify-center overflow-hidden rounded-full bg-gold font-serif text-cream-light"
      style={{ width: size, height: size }}
    >
      {user?.profile.avatar_url ? (
        <img src={user.profile.avatar_url} alt="" className="size-full object-cover" />
      ) : name ? (
        <span aria-hidden>{name.charAt(0).toUpperCase()}</span>
      ) : (
        <UserRound size={size / 2} aria-hidden />
      )}
    </span>
  );
}

/** Desktop account menu on the header avatar. */
export function UserMenu({ className }: { className?: string }) {
  const { text, locale } = useI18n();
  const { user, signOut } = useSession();
  const links = useAccountLinks();
  if (!user) return null;
  const name = user.profile.nickname || user.display_name;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button type="button" aria-label={text("Account menu")} className={className} data-testid="user-menu-trigger">
          <UserAvatar />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" sideOffset={8} className="w-60 min-w-60 rounded-xl border border-gold/40 bg-cream-light p-1.5 font-serif text-ink shadow-lg">
        <DropdownMenuLabel className="flex flex-col gap-0.5 px-2 py-2">
          <span className="truncate font-serif text-sm font-semibold text-ink">{name}</span>
          <span className="truncate font-ui text-xs font-normal text-ink/60">{user.email}</span>
        </DropdownMenuLabel>
        <DropdownMenuSeparator className="bg-gold/30" />
        {links.map(({ label, href, icon: Icon }) => (
          <DropdownMenuItem key={href} asChild className={ITEM}>
            <Link href={`/${locale}${href}`}>
              <Icon aria-hidden />
              {text(label)}
            </Link>
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator className="bg-gold/30" />
        <HelpSupportMenuItems />
        <DropdownMenuSeparator className="bg-gold/30" />
        <DropdownMenuItem className={ITEM} onSelect={() => void signOut()}>
          <LogOut aria-hidden />
          {text("Sign out")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
