import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

// Unit tests run without the vinext/Cloudflare plugins used for app builds.
export default defineConfig({
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  test: {
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    environment: "node",
    restoreMocks: true,
  },
});
