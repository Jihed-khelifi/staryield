"use client";

import * as React from "react";

import { PSYCHICS_SECTION_ID } from "@/lib/psychics-link";

/**
 * Smooth-scrolls to the home "Psychics" section when the address has
 * `#psychics` (on arrival, on a same-page link click, on back/forward) and
 * moves focus to its heading so keyboard and screen-reader users land there.
 * With `prefers-reduced-motion` the jump is instant.
 */
export function useScrollToPsychics() {
  React.useEffect(() => {
    const scrollAndFocus = () => {
      const section = document.getElementById(PSYCHICS_SECTION_ID);
      if (!section) return;
      const reduce = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
      section.scrollIntoView({ behavior: reduce ? "auto" : "smooth", block: "start" });
      document.getElementById("psychics-heading")?.focus({ preventScroll: true });
    };
    const go = () => {
      if (window.location.hash === `#${PSYCHICS_SECTION_ID}`) scrollAndFocus();
    };
    // Same-page links navigate through the router: the URL changes with
    // pushState a moment later and no hashchange event fires, so a plain
    // left click on a link to the section (present on this page) acts directly.
    const onClick = (event: MouseEvent) => {
      if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      const anchor = (event.target as Element | null)?.closest?.(`a[href$='#${PSYCHICS_SECTION_ID}']`);
      if (anchor && document.getElementById(PSYCHICS_SECTION_ID)) window.setTimeout(scrollAndFocus, 50);
    };
    const first = window.setTimeout(go, 0);
    window.addEventListener("hashchange", go);
    window.addEventListener("popstate", go);
    document.addEventListener("click", onClick);
    return () => {
      window.clearTimeout(first);
      window.removeEventListener("hashchange", go);
      window.removeEventListener("popstate", go);
      document.removeEventListener("click", onClick);
    };
  }, []);
}
