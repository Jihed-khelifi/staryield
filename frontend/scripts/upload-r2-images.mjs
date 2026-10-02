import { readdir } from "node:fs/promises";
import { resolve, relative, extname } from "node:path";
import { spawn } from "node:child_process";

const source = process.argv[2];
if (!source) throw new Error("Usage: npm run assets:upload -- /absolute/path/to/images (keys are relative to this folder)");
const root = resolve(source);
const types = { ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".svg": "image/svg+xml", ".webp": "image/webp", ".gif": "image/gif", ".avif": "image/avif", ".ico": "image/x-icon" };
const files = [];
async function walk(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const file = resolve(dir, entry.name);
    if (entry.isDirectory()) await walk(file);
    else if (entry.isFile() && types[extname(file).toLowerCase()]) files.push(file);
  }
}
await walk(root);
const total = files.length;
let done = 0;
async function upload(file) {
  const key = relative(root, file).split("\\").join("/");
  await new Promise((res, rej) => {
    const child = spawn(process.execPath, ["node_modules/wrangler/bin/wrangler.js", "r2", "object", "put", `staryield-app-assets/${key}`, "--remote", "--file", file, "--content-type", types[extname(file).toLowerCase()], "--cache-control", "public, max-age=3600"], { stdio: ["ignore", "pipe", "pipe"] });
    let output = "";
    child.stdout.on("data", (data) => { output += data; });
    child.stderr.on("data", (data) => { output += data; });
    child.on("error", rej);
    child.on("exit", (code) => code === 0 ? res() : rej(new Error(`Upload failed: ${key}\n${output}`)));
  });
  done++;
  if (done % 25 === 0 || done === total) console.log(`Uploaded ${done}/${total} images`);
}
await Promise.all(Array.from({ length: 6 }, async () => {
  for (;;) {
    const file = files.pop();
    if (!file) return;
    await upload(file);
  }
}));
console.log("R2 upload complete (only image files were uploaded).");
