"use client";

import * as React from "react";
import { useRouter } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import { waitlistApi } from "@/lib/api/endpoints";
import { toApiError } from "@/lib/api/errors";
import { useRequireAdult } from "@/lib/auth/age-gate";
import { useSession } from "@/lib/auth/session-provider";
import { loginUrl } from "@/lib/navigation";
import { useAsync } from "@/lib/use-async";
import { cn } from "@/lib/utils";
import { announce } from "@/components/feedback/live-region";
import { useToast } from "@/components/feedback/toast-provider";
import { Button } from "@/components/ui/button";

/** Open waiting lists per client are capped at 20 server-side, so one page of 50 holds them all. */
const MINE_LIMIT = 50;

export type WaitlistMembership = {
  /** The viewer can use the waiting list (visitor or client). */
  available: boolean;
  /** Known only for signed-in clients (`GET /me/waitlist`). */
  joined: boolean;
  /** The joined state is still loading. */
  checking: boolean;
  pending: boolean;
  join: () => Promise<void>;
  leave: () => Promise<void>;
};

/**
 * The viewer's place on one advisor's waiting list. Visitors are sent to
 * sign in first; clients join or leave; a 409 `advisor_available` means the
 * advisor is online now (`onAvailable`), any other error is a toast.
 */
export function useWaitlistMembership(slug: string, name: string, { onAvailable }: { onAvailable?: () => void } = {}): WaitlistMembership {
  const { text, locale } = useI18n();
  const router = useRouter();
  const toast = useToast();
  const { status, user } = useSession();
  const requireAdult = useRequireAdult(); // revisions 5.0: 18+ age gate (§24.6)
  const isClient = status === "authenticated" && user?.role === "client";
  const mine = useAsync((signal) => waitlistApi.mine({ limit: MINE_LIMIT }, signal), [user?.id ?? "", slug], isClient);
  const [override, setOverride] = React.useState<{ slug: string; joined: boolean } | null>(null);
  const [pending, setPending] = React.useState(false);

  const fetched = Boolean(mine.data?.items.some((e) => e.slug === slug));
  const joined = override && override.slug === slug ? override.joined : fetched;

  const join = React.useCallback(async () => {
    if (status !== "authenticated") {
      router.push(loginUrl(locale));
      return;
    }
    if (user?.role !== "client") return;
    if (!requireAdult()) return;
    setPending(true);
    try {
      await waitlistApi.join(slug);
      setOverride({ slug, joined: true });
      const message = text("You're on {value0}'s waiting list. We'll let you know when they're online.", { value0: name });
      toast.show({ tone: "success", message });
      announce(message);
    } catch (err) {
      const e = toApiError(err);
      if (e.code === "advisor_available") {
        onAvailable?.();
        toast.show({ tone: "info", message: text("{value0} is online now. Start your chat.", { value0: name }) });
      } else {
        toast.showError(e);
      }
    } finally {
      setPending(false);
    }
  }, [status, user?.role, router, locale, slug, text, name, toast, onAvailable, requireAdult]);

  const leave = React.useCallback(async () => {
    setPending(true);
    try {
      await waitlistApi.leave(slug);
      setOverride({ slug, joined: false });
      const message = text("You left {value0}'s waiting list.", { value0: name });
      toast.show({ tone: "info", message });
      announce(message);
    } catch (err) {
      toast.showError(err);
    } finally {
      setPending(false);
    }
  }, [slug, text, name, toast]);

  return {
    available: status !== "authenticated" || isClient,
    joined,
    checking: isClient && mine.loading && !mine.data,
    pending,
    join,
    leave,
  };
}

/**
 * Glass "Join Waiting List" button for an advisor who is busy, away or
 * offline. Renders nothing for advisors and admins (only clients can wait).
 */
export function WaitlistButton({
  slug,
  name,
  onAvailable,
  className,
  describedBy,
}: {
  slug: string;
  /** The advisor's display name (for messages). */
  name: string;
  /** The advisor turned out to be online (switch to "Start chat"). */
  onAvailable?: () => void;
  className?: string;
  /** Id of the element naming the advisor (repeated buttons). */
  describedBy?: string;
}) {
  const { text } = useI18n();
  const w = useWaitlistMembership(slug, name, { onAvailable });
  if (!w.available) return null;
  const busy = w.pending || w.checking;
  return w.joined ? (
    <Button
      type="button"
      variant="glass"
      className={cn("h-11 text-[14px]", className)}
      aria-describedby={describedBy}
      disabled={busy}
      aria-busy={busy || undefined}
      onClick={() => void w.leave()}
      data-testid="waitlist-button"
      data-joined="true"
    >
      {text("✓ On the waiting list")}
      <span className="sr-only">{text("(select to leave)")}</span>
    </Button>
  ) : (
    <Button
      type="button"
      variant="glass"
      className={cn("h-11 text-[14px]", className)}
      aria-describedby={describedBy}
      disabled={busy}
      aria-busy={busy || undefined}
      onClick={() => void w.join()}
      data-testid="waitlist-button"
    >
      {text(w.pending ? "Joining…" : "Join Waiting List ")}
    </Button>
  );
}
