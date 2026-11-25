"use client";
import * as React from "react";

import { safeNext } from "@/lib/navigation";

const noSubscription = () => () => {};

/**
 * The safe `?next=` of the current address (see `safeNext`), for links that
 * move between the sign-in and sign-up pages without losing where the person
 * was heading (a book button's chat, a gated page). `null` on the server and
 * during hydration, so the first paint matches the server's.
 */
export function useSafeNext(locale: string): string | null {
  const search = React.useSyncExternalStore(noSubscription, () => window.location.search, () => "");
  return React.useMemo(() => safeNext(search, locale), [search, locale]);
}
