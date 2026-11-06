"use client";

import { useSyncExternalStore } from "react";

/**
 * The URL fragment as React state. Reads `location.hash`, follows
 * back/forward (popstate), manual edits (hashchange) and our own writes, and
 * renders "" on the server, so no birth data is ever part of the server
 * render.
 */
const LOCAL_EVENT = "staryield:hash";

function subscribe(onChange: () => void): () => void {
  window.addEventListener("hashchange", onChange);
  window.addEventListener("popstate", onChange);
  window.addEventListener(LOCAL_EVENT, onChange);
  return () => {
    window.removeEventListener("hashchange", onChange);
    window.removeEventListener("popstate", onChange);
    window.removeEventListener(LOCAL_EVENT, onChange);
  };
}

export function useLocationHash(): string {
  return useSyncExternalStore(
    subscribe,
    () => window.location.hash,
    () => "",
  );
}

/**
 * Set the fragment without a navigation. `push` adds a history entry (a
 * new result), otherwise the current entry is replaced. Passing "" removes
 * the fragment (back to the form).
 */
export function writeLocationHash(hash: string, push = true) {
  const url = window.location.pathname + window.location.search + (hash && hash !== "#" ? hash : "");
  if (url === window.location.pathname + window.location.search + window.location.hash) return;
  try {
    if (push) window.history.pushState(null, "", url);
    else window.history.replaceState(null, "", url);
  } catch {
    // Some embedded browsers forbid history writes; fall back to the hash.
    window.location.hash = hash;
  }
  window.dispatchEvent(new Event(LOCAL_EVENT));
}
