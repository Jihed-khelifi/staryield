"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { FiBookmark, FiUser } from "react-icons/fi";

import { useI18n } from "@/i18n/i18n-provider";
import type { Participant, Wallet } from "@/lib/api/types";
import { MOBILE_NAV_LINKS, isMobileNavActive } from "@/components/layout/site-header";
import { ReaderAvatar } from "@/components/chat/reader-avatar";
import { RatePrice } from "@/components/chat/rate-price";
import type { RateView } from "@/components/chat/chat-model";

const statusCopy: Record<string, string> = {
  online: "Online",
  busy: "Busy",
  away: "Away",
  offline: "Offline",
};

/**
 * Phone "session tools" row under the chat header (Figma 867:16787): the
 * client's credit balance (a top-up link) or the advisor's session
 * earnings, and the button that opens the side panel as a sheet.
 */
export function SessionTools({
  role,
  wallet,
  earned,
  returnPath,
  onOpenDetails,
  free = false,
}: {
  role: "client" | "psychic";
  wallet: Wallet | null;
  /** Advisor: formatted earnings of the live session. */
  earned?: string | null;
  returnPath: string;
  onOpenDetails: () => void;
  /** Free mode or sponsored session: no balance or top-up link. */
  free?: boolean;
}) {
  const { text, locale } = useI18n();
  return (
    <div className="chat-session-tools" data-testid="chat-session-tools">
      {role === "client" && free ? (
        <span className="chat-credit-pill" data-testid="free-chat-pill">
          <strong>{text("Free chat")}</strong>
        </span>
      ) : role === "client" ? (
        <Link
          href={`/${locale}/credits?next=${encodeURIComponent(returnPath)}`}
          className="chat-credit-pill"
          aria-label={text("Balance: {value0} credits. Top up credits", { value0: (wallet?.balance ?? 0).toLocaleString(locale) })}
        >
          <strong>{(wallet?.balance ?? 0).toLocaleString(locale)}</strong>
          <span>{text("credits")}</span>
        </Link>
      ) : earned ? (
        <span className="chat-credit-pill" data-testid="session-earnings-pill">
          <strong>{earned}</strong>
          <span>{text("earned this session")}</span>
        </span>
      ) : (
        <span />
      )}
      <button type="button" className="chat-tools-button" onClick={onOpenDetails}>
        {role === "client" ? <FiBookmark aria-hidden className="size-3" /> : <FiUser aria-hidden className="size-3" />}
        {text(role === "client" ? "Bookmarks" : "Client details")}
      </button>
    </div>
  );
}

/**
 * Top of the conversation switcher sheet (Figma 1261:1537): what switching
 * means for a paid session, then the conversation on screen.
 */
export function SwitcherHeader({
  role,
  counterpart,
  rate,
  liveWith,
}: {
  role: "client" | "psychic";
  counterpart: Participant | null;
  rate?: RateView | null;
  /** Name of the advisor the client is in a live, paid chat with. */
  liveWith?: string | null;
}) {
  const { text } = useI18n();
  return (
    <div className="chat-switcher-head">
      {liveWith ? (
        <p className="chat-switcher-notice" role="note">
          <span aria-hidden>✦</span>
          <span>
            {text("Confirming ends your paid session with {value0}. The next advisor's rate applies; your free trial does not restart.", { value0: liveWith })}
          </span>
        </p>
      ) : null}
      {counterpart ? (
        <div className="chat-switcher-current" data-testid="switcher-current">
          <ReaderAvatar initial={counterpart.name.charAt(0).toUpperCase()} image={counterpart.avatar_url} className="chat-switcher-avatar" />
          <div className="min-w-0 flex-1">
            <p className="chat-switcher-eyebrow">{text(role === "client" ? "Current advisor" : "Current client")}</p>
            <p className="chat-switcher-name">{text(counterpart.name)}</p>
            <p className="chat-switcher-meta">
              {counterpart.status ? text(statusCopy[counterpart.status] ?? counterpart.status) : null}
              {rate && rate.rate > 0 ? (
                <>
                  {" · "}
                  <RatePrice rate={rate} />
                </>
              ) : null}
            </p>
          </div>
        </div>
      ) : null}
      <p className="chat-switcher-eyebrow">{text(role === "client" ? "Choose another advisor" : "Other conversations")}</p>
    </div>
  );
}

/**
 * Compact quick links in the conversation switcher: the chatroom hides the
 * bottom navigation on phones, so its destinations live here (and in the
 * header menu).
 */
export function SwitcherQuickLinks({ onNavigate }: { onNavigate?: () => void }) {
  const { text, locale } = useI18n();
  const path = usePathname();
  return (
    <nav className="chat-quick-links" aria-label={text("Main navigation")} data-testid="chat-quick-links">
      {MOBILE_NAV_LINKS.map(({ label, href, icon: Icon }) => (
        <Link
          key={label}
          href={`/${locale}${href}`}
          onClick={onNavigate}
          aria-current={isMobileNavActive(path, locale, label, href) ? "page" : undefined}
        >
          <Icon size={16} strokeWidth={1.5} aria-hidden />
          <span>{text(label)}</span>
        </Link>
      ))}
    </nav>
  );
}
