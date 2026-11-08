import { readFile, readdir } from "node:fs/promises";
import { createHash } from "node:crypto";
import { extname, join } from "node:path";

const images = JSON.parse(await readFile(new URL("../src/data/r2-images.json", import.meta.url), "utf8"));

// 1. Every asset URL written literally in the app (TS/TSX/CSS/JSON) must be
//    a registered R2 object, so nothing ships pointing at a missing image.
const registered = new Set(images.map((image) => image.url));
const ASSET_URL = /https:\/\/assets\.staryield\.net\/[^\s"'`)\\]+?\.(?:png|jpe?g|svg|webp|gif|avif|ico|m4a|mp3|wav|ogg)\b/gi;
const SOURCE = new Set([".ts", ".tsx", ".css", ".json", ".mjs"]);
const unregistered = [];
async function scan(directory) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const file = join(directory, entry.name);
    if (entry.isDirectory()) await scan(file);
    else if (SOURCE.has(extname(file)) && !file.endsWith("r2-images.json")) {
      const text = await readFile(file, "utf8");
      for (const match of text.matchAll(ASSET_URL))
        if (!registered.has(match[0])) unregistered.push(`${file}: ${match[0]}`);
    }
  }
}
await scan(new URL("../src", import.meta.url).pathname);
if (unregistered.length)
  throw new Error(`Asset URLs missing from src/data/r2-images.json (upload with npm run assets:upload):\n${unregistered.join("\n")}`);

// 2. Every registered object is live with the right bytes, type, cache and CORS.
const queue = [...images];
let verified = 0;
const failures = [];
await Promise.all(Array.from({ length: 8 }, async () => {
  for (;;) {
    const image = queue.pop();
    if (!image) return;
    try {
      const response = await fetch(image.url, { headers: { Origin: "https://staryield.net" }, signal: AbortSignal.timeout(30000) });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      if (response.headers.get("content-type")?.split(";")[0] !== image.contentType) throw new Error("Incorrect Content-Type");
      if (response.headers.get("access-control-allow-origin") !== "https://staryield.net") throw new Error("Missing production CORS origin");
      if (!/max-age=[1-9][0-9]*/.test(response.headers.get("cache-control") ?? "")) throw new Error("Missing Cache-Control");
      const bytes = Buffer.from(await response.arrayBuffer());
      if (bytes.length !== image.bytes || createHash("sha256").update(bytes).digest("hex") !== image.sha256) throw new Error("Content checksum mismatch");
      verified++;
    } catch (error) {
      failures.push(`${image.key}: ${error.message}`);
    }
  }
}));
if (failures.length) throw new Error(failures.join("\n"));

// 3. No image or sound binaries in the repository (they belong in R2).
const imageTypes = new Set([".png", ".jpg", ".jpeg", ".svg", ".webp", ".gif", ".avif", ".ico", ".m4a", ".mp3", ".wav", ".ogg"]);
async function checkNoImages(directory) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const file = `${directory}/${entry.name}`;
    if (entry.isDirectory()) await checkNoImages(file);
    else if (imageTypes.has(extname(file).toLowerCase())) throw new Error(`Binary asset remains in repository: ${file}`);
  }
}
if (!process.argv.includes("--before-removal")) {
  await checkNoImages("public");
  await checkNoImages("src");
}
console.log(`Verified ${verified} public R2 assets (images and sounds): checksums, MIME types, cache headers, and production CORS; every asset URL in src/ is registered.`);
