"use client";

import * as React from "react";

import { reportEmailApi, type ReportEmailConfig } from "@/lib/api/calculator-report";
import { useAsync } from "@/lib/use-async";
import type { CalculatorKind } from "@/lib/schemas/calculator";
import { mayPrompt, PROMPT_AFTER_MS, PROMPT_SCROLL_SHARE, rememberPrompt, scrolledShare } from "@/components/calculators/report-email";

export type ReportEmailPrompt = {
  /** The report e-mail is switched on (also while the config is still loading or could not be read). */
  available: boolean;
  /** The marketing box starts ticked; false while loading, on failure and outside the pre-ticked countries. */
  prechecked: boolean;
  /** The config could not be read (the box then starts unticked). */
  configFailed: boolean;
  open: boolean;
  /** How the dialog was opened: by itself (no keyboard pop-up on phones) or by the visitor's click. */
  manual: boolean;
  setOpen: (open: boolean) => void;
  /** The "E-mail me this report" button. */
  openManually: () => void;
  /** The report was e-mailed: the dialog stays quiet for 30 days. */
  markSent: () => void;
};

/**
 * Drives the "Save your reading" dialog of one calculator result.
 *
 * It opens by itself, once per calculator per browser (and again only 30 days
 * after it was shown), when the result has been on screen for 6 s of visible
 * time or the visitor scrolled 60 % of the page, whichever comes first. It
 * never opens on the sample route (`active` false) nor when the API says the
 * report is switched off. The button opens it any time.
 */
export function useReportEmailPrompt(kind: CalculatorKind, active: boolean): ReportEmailPrompt {
  const config = useAsync<ReportEmailConfig>((signal) => reportEmailApi.config(signal), [kind], active);
  const [open, setOpenState] = React.useState(false);
  const [manual, setManual] = React.useState(false);
  // Shown once per page view even when the browser's storage is blocked and cannot remember it.
  const [shown, setShown] = React.useState(false);

  const settled = active && !config.loading;
  const available = !(config.data && !config.data.enabled);
  const armed = settled && available && !open && !shown && mayPrompt(kind);

  const show = React.useCallback(() => {
    rememberPrompt(kind);
    setShown(true);
    setManual(false);
    setOpenState(true);
  }, [kind]);

  React.useEffect(() => {
    if (!armed) return;
    let remaining = PROMPT_AFTER_MS;
    let startedAt = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let fired = false;
    const fire = () => {
      if (fired) return;
      fired = true;
      stop();
      show();
    };
    const run = () => {
      if (timer !== undefined || document.visibilityState !== "visible") return;
      startedAt = Date.now();
      timer = setTimeout(fire, remaining);
    };
    const pause = () => {
      if (timer === undefined) return;
      clearTimeout(timer);
      timer = undefined;
      remaining = Math.max(remaining - (Date.now() - startedAt), 0);
    };
    const onVisibility = () => (document.visibilityState === "visible" ? run() : pause());
    const onScroll = () => {
      if (scrolledShare() >= PROMPT_SCROLL_SHARE) fire();
    };
    function stop() {
      pause();
      document.removeEventListener("visibilitychange", onVisibility);
      window.removeEventListener("scroll", onScroll);
    }
    document.addEventListener("visibilitychange", onVisibility);
    window.addEventListener("scroll", onScroll, { passive: true });
    run();
    return stop;
  }, [armed, show]);

  const setOpen = React.useCallback((value: boolean) => setOpenState(value), []);
  const openManually = React.useCallback(() => {
    setManual(true);
    setOpenState(true);
  }, []);
  const markSent = React.useCallback(() => rememberPrompt(kind), [kind]);

  return {
    available,
    prechecked: Boolean(config.data?.enabled && config.data.marketing_prechecked),
    configFailed: Boolean(config.error) && !config.data,
    open,
    manual,
    setOpen,
    openManually,
    markSent,
  };
}
