"use client";

import Link from "next/link";
import { ChevronDown, LogOut, Pause, Play, Settings } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { psychicsApi } from "@/lib/api/endpoints";
import type { Availability } from "@/lib/api/types";
import { useRealtimeEvent, useSession } from "@/lib/auth/session-provider";
import { useAsync } from "@/lib/use-async";
import { cn } from "@/lib/utils";
import { UserAvatar } from "@/components/layout/user-menu";
import { NotificationBell } from "@/components/notifications/notification-bell";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { StatusBadge } from "@/components/ui/status-badge";
import { formatWallet } from "@/components/psychic/format";
import { isBusy, useOwnStatus, type OwnStatus } from "@/components/psychic/use-own-status";

const CHOICES: { value: Availability; label: string }[] = [
  { value: "online", label: "Online" },
  { value: "away", label: "Away" },
  { value: "offline", label: "Offline" },
];

/**
 * "Wallet : 1000 USD": the advisor's withdrawable earnings (GET
 * /psychic/balance), refreshed when a session settles. Opens the earnings
 * and withdrawals page. `compact` (header) keeps only the amount visible
 * below 1024px, where the header has no room for the label; screen readers
 * still hear "Wallet :".
 */
export function WalletPill({ className, compact = false, onNavigate }: { className?: string; compact?: boolean; onNavigate?: () => void }) {
  const { text, locale } = useI18n();
  const balance = useAsync((signal) => psychicsApi.balance(signal), []);
  const { reload } = balance;
  useRealtimeEvent("session.settled", () => reload());
  const value = balance.data ? formatWallet(balance.data.available_cents, balance.data.currency, locale) : balance.error ? "—" : "…";
  return (
    <Link
      href={`/${locale}/profile/analytics`}
      onClick={onNavigate}
      className={cn("balance-pill", className)}
      data-testid="wallet-pill"
      aria-busy={balance.loading || undefined}
    >
      <span className={compact ? "max-lg:sr-only" : undefined}>{text("Wallet :")}</span> {value}
    </Link>
  );
}

/**
 * The advisor's availability (Online / Away / Offline). While a live chat is
 * open the server reports "Busy", which is shown read-only. The state is
 * always spelled out next to an icon, never colour alone.
 */
export function AvailabilityMenu({ own, className }: { own: OwnStatus; className?: string }) {
  const { text } = useI18n();
  const busy = isBusy(own.status);
  const shown = busy ? "busy" : own.availability;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className={cn("availability-trigger", className)}
          data-testid="availability-menu"
          data-status={shown}
          aria-label={text("Availability: {value0}", { value0: text(busy ? "Busy" : (CHOICES.find((c) => c.value === own.availability)?.label ?? "Offline")) })}
          aria-busy={own.saving || undefined}
        >
          <StatusBadge status={shown} variant="dot" className="availability-trigger-label" />
          <ChevronDown aria-hidden className="size-3.5 opacity-70" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" sideOffset={8} className="advisor-menu w-60 min-w-60 p-0">
        <DropdownMenuLabel className="advisor-menu-header">{text("Availability")}</DropdownMenuLabel>
        {busy ? (
          <p className="advisor-menu-note" role="note">
            {text("You are in a live chat. Clients see you as busy until it ends.")}
          </p>
        ) : null}
        <DropdownMenuRadioGroup
          value={own.availability}
          onValueChange={(value) => void own.setAvailability(value as Availability)}
        >
          {CHOICES.map((choice) => (
            <DropdownMenuRadioItem
              key={choice.value}
              value={choice.value}
              disabled={busy || own.saving}
              className="advisor-menu-item"
              data-testid={`availability-${choice.value}`}
            >
              <StatusBadge status={choice.value} variant="dot" />
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/** The advisor's avatar menu: go online/offline, settings and log out. */
export function AdvisorMenu({ own, className }: { own: OwnStatus; className?: string }) {
  const { text, locale } = useI18n();
  const { user, signOut } = useSession();
  if (!user) return null;
  const name = user.psychic?.name || user.profile.nickname || user.display_name;
  const online = own.availability === "online";
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button type="button" aria-label={text("Account menu")} className={className} data-testid="user-menu-trigger">
          <UserAvatar />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" sideOffset={8} className="advisor-menu w-64 min-w-64 p-0">
        <DropdownMenuLabel className="advisor-menu-header">{name}</DropdownMenuLabel>
        <DropdownMenuItem
          className="advisor-menu-item"
          disabled={own.saving || isBusy(own.status)}
          onSelect={() => void own.setAvailability(online ? "offline" : "online")}
          data-testid="advisor-menu-toggle-online"
        >
          {online ? <Pause aria-hidden /> : <Play aria-hidden />}
          {text(online ? "Go offline" : "Go online")}
        </DropdownMenuItem>
        <DropdownMenuItem asChild className="advisor-menu-item">
          <Link href={`/${locale}/profile/settings`}>
            <Settings aria-hidden />
            {text("Settings")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem className="advisor-menu-item" onSelect={() => void signOut()}>
          <LogOut aria-hidden />
          {text("Log out")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/** Header actions for advisors (Figma: wallet, status, bell, avatar menu). */
export function AdvisorHeaderActions() {
  const own = useOwnStatus();
  return (
    <>
      <WalletPill compact className="max-md:hidden" />
      <AvailabilityMenu own={own} />
      <NotificationBell />
      <AdvisorMenu own={own} className="user-menu-trigger hidden md:flex" />
    </>
  );
}
