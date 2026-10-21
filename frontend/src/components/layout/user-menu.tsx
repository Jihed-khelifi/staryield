"use client";
/* eslint-disable @next/next/no-img-element */
import Link from "next/link";
import { ChartColumn, LogOut, Settings, UserRound } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { useSession } from "@/lib/auth/session-provider";
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
  ];
  if (user?.role === "psychic") links.push({ label: "Analytics", href: "/profile/analytics", icon: ChartColumn });
  return links;
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
        <DropdownMenuItem className={ITEM} onSelect={() => void signOut()}>
          <LogOut aria-hidden />
          {text("Sign out")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
