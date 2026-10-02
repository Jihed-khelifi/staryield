import { readFile, readdir } from "node:fs/promises";
import { createHash } from "node:crypto";
import { extname } from "node:path";

const images = JSON.parse(await readFile(new URL("../src/data/r2-images.json", import.meta.url), "utf8"));
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
const imageTypes = new Set([".png", ".jpg", ".jpeg", ".svg", ".webp", ".gif", ".avif", ".ico"]);
async function checkNoImages(directory) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const file = `${directory}/${entry.name}`;
    if (entry.isDirectory()) await checkNoImages(file);
    else if (imageTypes.has(extname(file).toLowerCase())) throw new Error(`Image binary remains in repository: ${file}`);
  }
}
if (!process.argv.includes("--before-removal")) {
  await checkNoImages("public");
  await checkNoImages("src");
}
console.log(`Verified ${verified} public R2 images: checksums, MIME types, cache headers, and production CORS.`);
