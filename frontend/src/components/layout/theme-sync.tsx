"use client";

import * as React from "react";

import { useSession } from "@/lib/auth/session-provider";
import { applyTheme, currentTheme, themeFromCookieHeader, themeSlug } from "@/lib/theme";

/**
 * Keeps this browser's theme (and its `sy_theme` cookie, read by the server
 * render) in line with the signed-in user's saved theme, e.g. after it was
 * changed on another device. Renders nothing.
 */
export function ThemeSync() {
  const { user } = useSession();
  const saved = user?.profile.theme;
  React.useEffect(() => {
    if (!saved) return;
    const slug = themeSlug(saved);
    if (slug !== currentTheme() || slug !== themeFromCookieHeader(document.cookie)) applyTheme(slug);
  }, [saved]);
  return null;
}
