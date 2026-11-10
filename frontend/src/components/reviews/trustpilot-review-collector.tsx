"use client";

import { useEffect, useRef, useState } from "react";

import {
  TRUSTPILOT_BUSINESS_UNIT_ID,
  TRUSTPILOT_LOCALES,
  TRUSTPILOT_REVIEW_URL,
  TRUSTPILOT_TEMPLATE_ID,
  TRUSTPILOT_WIDGET_HEIGHT,
} from "@/config/trustpilot";
import { useI18n } from "@/i18n/i18n-provider";
import { thirdPartyEmbedsAllowed } from "@/lib/consent";
import { loadTrustpilot } from "@/lib/trustpilot";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

export type TrustpilotPlacement = "reviews" | "home" | "footer" | "chat-summary";
type State = "idle" | "loading" | "loaded" | "failed" | "blocked";

const escapeHtml = (value: string) =>
  value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

/**
 * Trustpilot's "Review Collector" TrustBox. The markup is Trustpilot's own;
 * the script is fetched only when the widget nears the viewport and only when
 * the browser has not sent Global Privacy Control or Do Not Track. Otherwise,
 * or if the script fails, a plain "Review us on Trustpilot" link is shown.
 * Every placement shows the same widget to everyone: invitations are never
 * selected by rating and nothing is offered in exchange for a review.
 */
export function TrustpilotReviewCollector({ placement, className }: { placement: TrustpilotPlacement; className?: string }) {
  const { text, locale } = useI18n();
  const frame = useRef<HTMLDivElement>(null);
  const widget = useRef<HTMLDivElement>(null);
  const [state, setState] = useState<State>("idle");

  useEffect(() => {
    if (!thirdPartyEmbedsAllowed()) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- the browser's signal is only known on the client
      setState("blocked");
      return;
    }
    let cancelled = false;
    const load = () => {
      setState("loading");
      loadTrustpilot()
        .then((trustpilot) => {
          const element = widget.current;
          if (cancelled || !element) return;
          trustpilot.loadFromElement(element, true);
          setState("loaded");
        })
        .catch(() => {
          if (!cancelled) setState("failed");
        });
    };
    const target = frame.current;
    if (!target || typeof IntersectionObserver === "undefined") {
      load();
      return () => {
        cancelled = true;
      };
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (!entries.some((entry) => entry.isIntersecting)) return;
        observer.disconnect();
        load();
      },
      { rootMargin: "200px" },
    );
    observer.observe(target);
    return () => {
      cancelled = true;
      observer.disconnect();
    };
  }, []);

  const label = text("Review us on Trustpilot");
  const newTabLabel = text("Review us on Trustpilot (opens in a new tab)");
  const fallback = state === "failed" || state === "blocked";
  const reserved = { minHeight: TRUSTPILOT_WIDGET_HEIGHT };

  return (
    <div
      ref={frame}
      data-testid="trustpilot-widget"
      data-placement={placement}
      data-state={state}
      className={cn("relative w-full", className)}
      style={reserved}
    >
      {fallback ? (
        <a
          href={TRUSTPILOT_REVIEW_URL}
          target="_blank"
          rel="noopener"
          aria-label={newTabLabel}
          data-testid="trustpilot-fallback"
          className={cn(buttonVariants({ variant: "glass", size: "lg" }), "mx-auto w-fit whitespace-normal text-center")}
        >
          {label}
        </a>
      ) : (
        <>
          <div
            ref={widget}
            className="trustpilot-widget"
            data-locale={TRUSTPILOT_LOCALES[locale]}
            data-template-id={TRUSTPILOT_TEMPLATE_ID}
            data-businessunit-id={TRUSTPILOT_BUSINESS_UNIT_ID}
            data-style-height={`${TRUSTPILOT_WIDGET_HEIGHT}px`}
            data-style-width="100%"
            // Trustpilot replaces this node's children, so React must never reconcile them.
            dangerouslySetInnerHTML={{
              __html: `<a href="${TRUSTPILOT_REVIEW_URL}" target="_blank" rel="noopener" aria-label="${escapeHtml(newTabLabel)}">${escapeHtml(label)}</a>`,
            }}
          />
          {state === "loaded" ? null : <Skeleton className="absolute inset-0 rounded-lg" style={reserved} />}
        </>
      )}
    </div>
  );
}
