"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import { useSession } from "@/lib/auth/session-provider";
import type { TopUpAdvisor } from "@/lib/billing/top-up";
import { loginUrl } from "@/lib/navigation";

import { CreditTopUp } from "./credit-top-up";

type TopUpContextValue = {
  /**
   * Open the top-up in place (never navigates, so a live chat keeps running).
   * From a live reading pass the advisor and the rate the client pays, and the
   * dialog states how many minutes each pack buys at that rate.
   */
  open: (context?: TopUpAdvisor) => void;
  close: () => void;
  isOpen: boolean;
  /**
   * The advisor of the conversation on screen (set by `TopUpAdvisorScope`).
   * Any top-up opened without its own context while it is set (the header's
   * balance pill during a live reading, for instance) uses it, so the estimate
   * always follows the reading in progress (§24.7).
   */
  setScopeAdvisor: (advisor: TopUpAdvisor | undefined) => void;
};

/** A context that can produce the "At {advisor}'s rate…" estimate. */
function hasAdvisor(context: TopUpAdvisor | undefined): context is TopUpAdvisor {
  return Boolean(context?.advisorName?.trim() && context.ratePerMinute && context.ratePerMinute > 0);
}

const TopUpContext = React.createContext<TopUpContextValue | null>(null);

/**
 * Owns the top-up dialog for everything signed in (§24.7). It lives in the
 * app layout, above the pages, so the header, the chatroom and any future
 * "Top up" button open the same dialog. The `/credits` route renders
 * `CreditTopUp` itself (the deep link).
 */
export function TopUpProvider({ children }: { children: React.ReactNode }) {
  const { locale } = useI18n();
  const router = useRouter();
  const { status } = useSession();
  const [request, setRequest] = React.useState<{ id: number; context?: TopUpAdvisor } | null>(null);
  const nextId = React.useRef(0);
  // A ref, not state: changing the advisor on screen must not re-render every consumer.
  const scopeAdvisor = React.useRef<TopUpAdvisor | undefined>(undefined);
  // What had the focus when the dialog opened (the balance pill, a "Top up" link…): it gets the
  // focus back on close. Kept here rather than left to the dialog's focus scope, which loses it
  // when the dialog mounts twice (React's development double effects) or is opened programmatically.
  const opener = React.useRef<HTMLElement | null>(null);

  const close = React.useCallback(() => {
    setRequest(null);
    const element = opener.current;
    opener.current = null;
    if (!element) return;
    window.requestAnimationFrame(() => {
      const active = document.activeElement;
      // Only when nothing else took the focus meanwhile (a navigation, a toast action…).
      if (element.isConnected && (!active || active === document.body)) element.focus({ preventScroll: true });
    });
  }, []);
  const open = React.useCallback(
    (context?: TopUpAdvisor) => {
      if (status === "anonymous") {
        router.push(loginUrl(locale));
        return;
      }
      const active = document.activeElement;
      opener.current = active instanceof HTMLElement && active !== document.body ? active : null;
      setRequest({ id: ++nextId.current, context: hasAdvisor(context) ? context : scopeAdvisor.current });
    },
    [status, locale, router],
  );
  const setScopeAdvisor = React.useCallback((advisor: TopUpAdvisor | undefined) => {
    scopeAdvisor.current = hasAdvisor(advisor) ? advisor : undefined;
  }, []);

  const isOpen = request !== null && status === "authenticated";
  const value = React.useMemo(() => ({ open, close, isOpen, setScopeAdvisor }), [open, close, isOpen, setScopeAdvisor]);
  return (
    <TopUpContext.Provider value={value}>
      {children}
      {isOpen ? <CreditTopUp key={request.id} context={request.context} onClose={close} restoresFocus /> : null}
    </TopUpContext.Provider>
  );
}

export function useTopUp(): TopUpContextValue {
  const ctx = React.useContext(TopUpContext);
  if (!ctx) throw new Error("useTopUp must be used inside <TopUpProvider>");
  return ctx;
}

type TopUpLinkProps = Omit<React.ComponentProps<typeof Link>, "href"> & TopUpAdvisor & { href?: string };

/**
 * A link to `/credits` that opens the top-up in place on a plain click. The
 * href stays for middle-click, "open in new tab" and no-JS; modified clicks and
 * a missing provider simply follow it.
 */
export function TopUpLink({ advisorName, ratePerMinute, href, onClick, ...rest }: TopUpLinkProps) {
  const { locale } = useI18n();
  const ctx = React.useContext(TopUpContext);
  return (
    <Link
      {...rest}
      href={href ?? `/${locale}/credits`}
      onClick={(event) => {
        onClick?.(event);
        if (!ctx || event.defaultPrevented || rest.target === "_blank" || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
        event.preventDefault();
        ctx.open({ advisorName, ratePerMinute });
      }}
    />
  );
}

/**
 * Declares the advisor of the conversation on screen while it is mounted, so
 * every top-up opened meanwhile shows the estimate at that advisor's rate
 * (the chatroom renders it; it renders nothing).
 */
export function TopUpAdvisorScope({ advisor }: { advisor: TopUpAdvisor | undefined }) {
  const ctx = React.useContext(TopUpContext);
  const setScopeAdvisor = ctx?.setScopeAdvisor;
  const name = advisor?.advisorName;
  const rate = advisor?.ratePerMinute;
  React.useEffect(() => {
    if (!setScopeAdvisor) return;
    setScopeAdvisor(name && rate ? { advisorName: name, ratePerMinute: rate } : undefined);
    return () => setScopeAdvisor(undefined);
  }, [setScopeAdvisor, name, rate]);
  return null;
}
