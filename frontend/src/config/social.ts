/**
 * Staryield's social profiles, shown in every footer (Figma "Footer /
 * Corporate", node 1283:2192). Icons are the Figma exports on R2. A profile
 * without an `href` (the apps are not published yet) is not rendered.
 */
export type SocialId = "facebook" | "x" | "instagram" | "reddit" | "tiktok" | "google-play" | "app-store";

export type SocialLink = {
  id: SocialId;
  /** Platform name (brand names are not translated). */
  label: string;
  href?: string;
  icon: string;
  /** Rendered size in px, as designed (Instagram is 24, the rest 22). */
  size: number;
};

const ICONS = "https://assets.staryield.net/assets/social";

export const SOCIAL_LINKS: readonly SocialLink[] = [
  {
    id: "facebook",
    label: "Facebook",
    href: "https://www.facebook.com/profile.php?id=61595272673949",
    icon: `${ICONS}/facebook-fe82eb80d5.svg`,
    size: 22,
  },
  { id: "x", label: "X", href: "https://x.com/StaryieldCoven", icon: `${ICONS}/x-3e7a43860a.svg`, size: 22 },
  {
    id: "instagram",
    label: "Instagram",
    href: "https://www.instagram.com/staryield/",
    icon: `${ICONS}/instagram-cf7ff32b33.svg`,
    size: 24,
  },
  {
    id: "reddit",
    label: "Reddit",
    href: "https://www.reddit.com/r/staryield/",
    icon: `${ICONS}/reddit-b92a9b5fbe.svg`,
    size: 22,
  },
  {
    id: "tiktok",
    label: "TikTok",
    href: "https://www.tiktok.com/@staryield.media",
    icon: `${ICONS}/tiktok-d4c1acc79e.svg`,
    size: 22,
  },
  // Store listings are not live yet: add the URL to show the icon.
  { id: "google-play", label: "Google Play", icon: `${ICONS}/google-play-70cc64ee4e.svg`, size: 22 },
  { id: "app-store", label: "App Store", icon: `${ICONS}/app-store-0526e10fa3.svg`, size: 22 },
];

/** Profiles that can be linked: only absolute https URLs are rendered. */
export function visibleSocialLinks(links: readonly SocialLink[] = SOCIAL_LINKS): SocialLink[] {
  return links.filter((link): link is SocialLink & { href: string } => {
    if (!link.href) return false;
    try {
      return new URL(link.href).protocol === "https:";
    } catch {
      return false;
    }
  });
}
