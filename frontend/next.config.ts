import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  images: {
    // Deliver the originals directly from public R2, including SVG artwork.
    unoptimized: true,
    remotePatterns: [
      { protocol: "https", hostname: "assets.staryield.net", pathname: "/assets/**" },
    ],
  },
};

export default nextConfig;
