"use client";

import * as React from "react";
import Link from "next/link";
import { Loader2 } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { errorMessage } from "@/lib/api/errors";
import type { PresenceStatus } from "@/lib/api/types";
import { track } from "@/lib/analytics";
import { useRequireAdult } from "@/lib/auth/age-gate";
import { ageStatusOf } from "@/lib/auth/age-status";
import { useSession } from "@/lib/auth/session-provider";
import { rememberBookingIntent, takeBookingIntent } from "@/lib/booking/booking-intent";
import { bookingState, type BookingCopy } from "@/lib/booking/booking-state";
import { useWaitlist } from "@/lib/booking/waitlist-provider";
import { signupUrl } from "@/lib/navigation";
import { useFreeMode, useSignupFreeMinutes } from "@/lib/site-config/site-config-provider";
import { cn } from "@/lib/utils";
import { announce } from "@/components/feedback/live-region";
import { useToast } from "@/components/feedback/toast-provider";
import { Button } from "@/components/ui/button";

export type BookAdvisor = {
  slug: string;
  /** Display name, for messages. */
  name: string;
  status: PresenceStatus;
  rate_per_minute: number;
};

export type BookButtonSize = "md" | "lg" | "full";

const SIZE: Record<BookButtonSize, string> = {
  md: "min-h-11 px-5 text-[14px]",
  lg: "min-h-14 px-8 text-[16px]",
  full: "min-h-12 w-full px-5 text-[14px]",
};

/** States that are still settling (session, lists, a request in flight). */
const SETTLING = new Set(["session_loading", "waitlist_loading", "pending"]);

/**
 * The standard "book" action of an advisor: the same state machine
 * (`bookingState`), copy and analytics on every surface. Online advisors get
 * the gold call to action (the chat, or sign-up first for visitors, then the
 * chat); the others get the glass waiting-list action. Needs a
 * `WaitlistProvider` (or `WaitlistScope`) above it.
 */
export function BookButton({
  advisor,
  placement,
  size = "md",
  describedBy,
  className,
  onAvailable,
}: {
  advisor: BookAdvisor;
  /** Where the button sits ("home_featured", "directory", "profile_hero", ...): the analytics `placement`. */
  placement: string;
  size?: BookButtonSize;
  /** Id of the element naming the advisor, so repeated buttons are told apart. */
  describedBy?: string;
  className?: string;
  /** The server said the advisor is online while the page showed them busy or away. */
  onAvailable?: () => void;
}) {
  const { text, locale } = useI18n();
  const toast = useToast();
  const { status: sessionStatus, user, wallet } = useSession();
  const requireAdult = useRequireAdult();
  const waitlist = useWaitlist();
  const freeMode = useFreeMode();
  const signupOfferMinutes = useSignupFreeMinutes();

  const { slug, name, rate_per_minute } = advisor;
  // A 409 `advisor_available` makes the chat show until the real status changes.
  const status: PresenceStatus = waitlist.availableFrom(slug) === advisor.status ? "online" : advisor.status;
  const isClient = sessionStatus === "authenticated" && user?.role === "client";
  const needsWaitlist = isClient && status !== "online";
  const { demand } = waitlist;
  React.useEffect(() => {
    if (needsWaitlist) demand();
  }, [needsWaitlist, demand]);

  const state = bookingState({
    advisorStatus: status,
    ratePerMinute: rate_per_minute,
    session: sessionStatus,
    role: user?.role ?? null,
    freeMinutes: wallet?.free_minutes ?? 0,
    signupOfferMinutes,
    freeMode,
    waitlist: isClient ? waitlist.joined(slug) : "not_joined",
    pending: isClient ? waitlist.pending(slug) : null,
    age: ageStatusOf(user),
  });

  const copy = (c: BookingCopy) => {
    const values = c.values && Object.fromEntries(Object.entries(c.values).map(([k, v]) => [k, v.toLocaleString(locale)]));
    return text(c.key, values);
  };

  const say = (tone: "success" | "info", message: string) => {
    toast.show({ tone, message });
    announce(message);
  };

  const join = async () => {
    const res = await waitlist.join(slug);
    if (res.ok) {
      say("success", text("You're on {value0}'s waiting list. We'll let you know when they're online.", { value0: name }));
    } else if (res.error.code === "advisor_available") {
      waitlist.markAvailable(slug, status);
      onAvailable?.();
      say("info", text("{value0} is online now. Start your chat.", { value0: name }));
    } else {
      toast.show({ tone: "error", message: text(errorMessage(res.error)), actionLabel: text("Try again"), onAction: () => void join() });
    }
  };

  const leave = async () => {
    const res = await waitlist.leave(slug);
    if (res.ok) {
      say("info", text("You left {value0}'s waiting list.", { value0: name }));
    } else {
      toast.show({ tone: "error", message: text(errorMessage(res.error)), actionLabel: text("Try again"), onAction: () => void leave() });
    }
  };

  // A visitor who asked to wait, then signed up or in, comes back here as a
  // client: the first settled button for that advisor finishes the join
  // (or says the advisor is online now). See booking-intent.ts.
  const settled = isClient && !SETTLING.has(state.reason ?? "");
  const resume = React.useEffectEvent(() => {
    if (!takeBookingIntent(slug)) return;
    if (state.action === "join_waitlist" && state.reason === null) void join();
    else if (state.action === "chat" && state.reason === null) say("info", text("{value0} is online now. Start your chat.", { value0: name }));
  });
  React.useEffect(() => {
    if (settled) resume();
  }, [settled, slug]);

  const clicked = () => {
    track("book_cta_clicked", { slug, status, placement, action: state.action });
    // The visitor's wish to wait survives the sign-up (finished by `resume` above).
    if (state.action === "signup" && status !== "online") rememberBookingIntent(slug);
  };

  if (state.action === "none") {
    return (
      <p role="note" className={cn("font-serif text-[13px] leading-snug text-[var(--muted-ink)]", className)} data-testid="book-note">
        {copy(state.label)}
      </p>
    );
  }

  const classes = cn(
    "relative h-auto flex-col gap-0 py-2 leading-tight whitespace-normal text-center",
    SIZE[size],
    state.tone === "success" && "border-gold",
    className,
  );
  const variant = state.tone === "primary" ? "cta" : "glass";
  const busy = SETTLING.has(state.reason ?? "");
  const content = (
    <>
      <span className="inline-flex items-center gap-1.5">
        {state.reason === "pending" ? <Loader2 aria-hidden className="size-4 animate-spin motion-reduce:animate-none" /> : null}
        {copy(state.label)}
      </span>
      {state.sublabel ? <span className="text-[12px] font-normal">{copy(state.sublabel)}</span> : null}
      {state.action === "leave_waitlist" ? <span className="sr-only">{text("(select to leave)")}</span> : null}
    </>
  );
  const common = {
    "data-testid": "book-button",
    "data-action": state.action,
    "data-state": state.reason ?? "ready",
    "aria-describedby": describedBy,
  };

  // Links where the action is a place to go; buttons where it is something to do.
  const gated = state.reason === "age_gate";
  if (!state.disabled && !gated && (state.action === "chat" || state.action === "signup" || state.action === "view")) {
    const chat = `/${locale}/chatroom?reader=${encodeURIComponent(slug)}`;
    const profile = `/${locale}/psychics/${encodeURIComponent(slug)}`;
    // Visitors sign up, then go on to the chat (online) or back to the profile, where the join finishes.
    const href = state.action === "chat" ? chat : state.action === "view" ? profile : signupUrl(locale, status === "online" ? chat : profile);
    return (
      <Button asChild variant={variant} className={classes}>
        <Link href={href} onClick={clicked} {...common}>
          {content}
        </Link>
      </Button>
    );
  }

  const onClick = () => {
    if (state.disabled) return;
    clicked();
    // Not finished with the age step: the existing age flow takes over.
    if (gated && !requireAdult()) return;
    if (state.action === "join_waitlist") void join();
    else if (state.action === "leave_waitlist") void leave();
  };
  return (
    <Button
      type="button"
      variant={variant}
      className={classes}
      disabled={state.disabled}
      aria-busy={busy || undefined}
      onClick={onClick}
      {...common}
    >
      {content}
    </Button>
  );
}
