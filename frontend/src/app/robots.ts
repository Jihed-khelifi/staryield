import type { MetadataRoute } from "next";

import { buildRobots } from "@/lib/seo/robots";

/** /robots.txt (see `lib/seo/robots.ts`). */
export default function robots(): MetadataRoute.Robots {
  return buildRobots();
}
