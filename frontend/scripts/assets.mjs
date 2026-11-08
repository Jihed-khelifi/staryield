/**
 * Image and sound assets live in the public R2 bucket `staryield-app-assets`
 * (https://assets.staryield.net), never in Git. This script uploads new
 * files and keeps `src/data/r2-images.json` (the registry checked by
 * `npm run test:r2`) in sync.
 *
 *   npm run assets:upload -- <dir> [--prefix assets/social] [--hashed] [--dry-run] [--force]
 *   npm run assets:verify
 *
 * upload
 *   Every image or sound under <dir> becomes an object. The key is `<prefix>/<path
 *   relative to dir>`; with --hashed a content hash is added before the
 *   extension (`facebook-1a2b3c4d5e.svg`) and the object is cached for a year
 *   as immutable, so a changed file always gets a new URL.
 *   Files whose checksum is already registered under the same key are
 *   skipped. A key that is registered with *different* content is never
 *   overwritten unless --force is given (existing URLs must not change).
 *   --dry-run prints the plan and writes nothing.
 * verify
 *   Runs the registry checks (scripts/check-r2-images.mjs).
 */
import { createHash } from "node:crypto";
import { spawn } from "node:child_process";
import { readFile, readdir, writeFile } from "node:fs/promises";
import { extname, relative, resolve } from "node:path";
import { pathToFileURL } from "node:url";

const BUCKET = "staryield-app-assets";
const ORIGIN = "https://assets.staryield.net";
const REGISTRY = new URL("../src/data/r2-images.json", import.meta.url);
export const TYPES = {
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".svg": "image/svg+xml",
  ".webp": "image/webp",
  ".gif": "image/gif",
  ".avif": "image/avif",
  ".ico": "image/x-icon",
  // Sounds (the advisor's incoming-request ringtone).
  ".m4a": "audio/mp4",
  ".mp3": "audio/mpeg",
  ".wav": "audio/wav",
  ".ogg": "audio/ogg",
};

/** Registry key for a file (pure, unit-tested). */
export function assetKey({ prefix = "", path, sha256, hashed = false }) {
  const clean = path.split("\\").join("/").replace(/^\/+/, "");
  const ext = extname(clean);
  const base = hashed ? `${clean.slice(0, clean.length - ext.length)}-${sha256.slice(0, 10)}${ext}` : clean;
  const root = prefix.replace(/^\/+|\/+$/g, "");
  return root ? `${root}/${base}` : base;
}

/** Merge entries into the registry, sorted by key (pure, unit-tested). */
export function mergeRegistry(registry, entries) {
  const byKey = new Map(registry.map((entry) => [entry.key, entry]));
  for (const entry of entries) byKey.set(entry.key, entry);
  return [...byKey.values()].sort((a, b) => (a.key < b.key ? -1 : a.key > b.key ? 1 : 0));
}

/** What to do with a file given the registry: "skip" | "upload" | "conflict". */
export function planEntry(registry, entry) {
  const existing = registry.find((item) => item.key === entry.key);
  if (!existing) return "upload";
  return existing.sha256 === entry.sha256 && existing.bytes === entry.bytes ? "skip" : "conflict";
}

async function walk(dir, out = []) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const file = resolve(dir, entry.name);
    if (entry.isDirectory()) await walk(file, out);
    else if (entry.isFile() && TYPES[extname(file).toLowerCase()]) out.push(file);
  }
  return out;
}

function wranglerPut(key, file, contentType, cacheControl) {
  return new Promise((resolvePut, reject) => {
    const child = spawn(
      process.execPath,
      [
        "node_modules/wrangler/bin/wrangler.js",
        "r2",
        "object",
        "put",
        `${BUCKET}/${key}`,
        "--remote",
        "--file",
        file,
        "--content-type",
        contentType,
        "--cache-control",
        cacheControl,
      ],
      { stdio: ["ignore", "pipe", "pipe"] },
    );
    let output = "";
    child.stdout.on("data", (data) => (output += data));
    child.stderr.on("data", (data) => (output += data));
    child.on("error", reject);
    child.on("exit", (code) => (code === 0 ? resolvePut() : reject(new Error(`Upload failed: ${key}\n${output}`))));
  });
}

async function upload(args) {
  const flags = new Set(args.filter((arg) => arg.startsWith("--") && !arg.includes("=")));
  const prefixIndex = args.indexOf("--prefix");
  const prefix = prefixIndex >= 0 ? args[prefixIndex + 1] ?? "" : args.find((a) => a.startsWith("--prefix="))?.slice(9) ?? "";
  const source = args.find((arg, i) => !arg.startsWith("--") && args[i - 1] !== "--prefix");
  if (!source) throw new Error("Usage: npm run assets:upload -- <dir> [--prefix assets/x] [--hashed] [--dry-run] [--force]");
  const dryRun = flags.has("--dry-run");
  const hashed = flags.has("--hashed");
  const force = flags.has("--force");
  const root = resolve(source);

  const registry = JSON.parse(await readFile(REGISTRY, "utf8"));
  const files = await walk(root);
  const plan = [];
  for (const file of files) {
    const bytes = await readFile(file);
    const sha256 = createHash("sha256").update(bytes).digest("hex");
    const key = assetKey({ prefix, path: relative(root, file), sha256, hashed });
    const entry = {
      key,
      url: `${ORIGIN}/${key}`,
      contentType: TYPES[extname(file).toLowerCase()],
      bytes: bytes.length,
      sha256,
    };
    plan.push({ file, entry, action: planEntry(registry, entry) });
  }

  const conflicts = plan.filter((item) => item.action === "conflict");
  if (conflicts.length && !force) {
    throw new Error(
      `Refusing to overwrite registered keys with different content (use --hashed for new URLs, or --force):\n${conflicts
        .map((item) => `  ${item.entry.key}`)
        .join("\n")}`,
    );
  }
  for (const item of plan) console.log(`${item.action.padEnd(8)} ${item.entry.key} (${item.entry.bytes} B)`);
  const todo = plan.filter((item) => item.action !== "skip");
  if (dryRun) {
    console.log(`Dry run: ${todo.length} to upload, ${plan.length - todo.length} unchanged. Nothing written.`);
    return;
  }

  const cacheControl = hashed ? "public, max-age=31536000, immutable" : "public, max-age=3600";
  const queue = [...todo];
  let done = 0;
  await Promise.all(
    Array.from({ length: 6 }, async () => {
      for (;;) {
        const item = queue.pop();
        if (!item) return;
        await wranglerPut(item.entry.key, item.file, item.entry.contentType, cacheControl);
        done++;
        console.log(`Uploaded ${done}/${todo.length}: ${item.entry.url}`);
      }
    }),
  );
  const next = mergeRegistry(
    registry,
    plan.map((item) => item.entry),
  );
  await writeFile(REGISTRY, `${JSON.stringify(next, null, 2)}\n`);
  console.log(`Registry updated: ${next.length} assets (src/data/r2-images.json).`);
}

const [command, ...rest] = process.argv.slice(2);
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  if (command === "upload") await upload(rest);
  else if (command === "verify") await import("./check-r2-images.mjs");
  else {
    console.error("Usage: node scripts/assets.mjs <upload|verify> …");
    process.exit(1);
  }
}
