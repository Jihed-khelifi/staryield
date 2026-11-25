#!/usr/bin/env node
/**
 * The whole Playwright suite, as fast as it can run safely (npm run test:e2e:full).
 *
 *  1. One server for both phases. E2E_SERVER=prod (the default here) runs
 *     `next build` once and serves it with `next start`: pages are not compiled
 *     on demand, which is most of the CPU and wall time of a `next dev` run.
 *     E2E_SERVER=dev starts `next dev`. A server already answering at
 *     E2E_BASE_URL is reused as it is (its build may be stale).
 *  2. The public-* projects (playwright.config.ts PUBLIC_SPECS), E2E_WORKERS
 *     spec files at a time (default: a quarter of the CPUs, 2 to 4).
 *  3. The desktop and mobile projects, one test at a time: they share the seed
 *     accounts, advisor availability and site settings.
 *
 * Both phases always run; the exit code is non-zero when either failed. Extra
 * arguments go to both `playwright test` calls (a spec file, --grep, …). The
 * server's output is in test-results/e2e-server.log.
 *
 *   E2E_API_URL=http://localhost:18080 E2E_BASE_URL=http://localhost:13000 npm run test:e2e:full
 */
import { spawn, spawnSync } from "node:child_process";
import { mkdirSync, openSync } from "node:fs";
import os from "node:os";

const base = (process.env.E2E_BASE_URL ?? "http://localhost:3000").replace(/\/+$/, "");
const api = (process.env.E2E_API_URL ?? "http://localhost:8080").replace(/\/+$/, "");
const port = new URL(base).port || "3000";
const mode = process.env.E2E_SERVER ?? "prod";
const workers = Number(process.env.E2E_WORKERS) || Math.min(4, Math.max(2, Math.floor(os.cpus().length / 4)));
const extra = process.argv.slice(2);

async function answering() {
  try {
    const res = await fetch(`${base}/en`, { redirect: "manual" });
    return res.status < 500;
  } catch {
    return false;
  }
}

let server;
function stopServer() {
  if (!server) return;
  try {
    process.kill(-server.pid, "SIGTERM"); // the whole group: npm/npx and next
  } catch {
    // already gone
  }
  server = undefined;
}
process.on("SIGINT", () => {
  stopServer();
  process.exit(130);
});

if (!(await answering())) {
  const env = { ...process.env, NEXT_PUBLIC_API_URL: api, PORT: port, NEXT_TELEMETRY_DISABLED: "1" };
  mkdirSync("test-results", { recursive: true });
  const log = openSync("test-results/e2e-server.log", "w");
  if (mode === "prod") {
    console.log(`e2e: building the site against ${api} …`);
    const t0 = Date.now();
    const build = spawnSync("npm", ["run", "build"], { stdio: ["ignore", log, log], env });
    if (build.status !== 0) {
      console.error("e2e: next build failed, see test-results/e2e-server.log");
      process.exit(build.status ?? 1);
    }
    console.log(`e2e: built in ${Math.round((Date.now() - t0) / 1000)}s`);
    server = spawn("npx", ["next", "start", "--port", port], { stdio: ["ignore", log, log], env, detached: true });
  } else {
    server = spawn("npm", ["run", "dev"], { stdio: ["ignore", log, log], env, detached: true });
  }
  const deadline = Date.now() + 180_000;
  while (!(await answering())) {
    if (Date.now() > deadline) {
      stopServer();
      console.error("e2e: the server did not answer within 3 minutes, see test-results/e2e-server.log");
      process.exit(1);
    }
    await new Promise((r) => setTimeout(r, 500));
  }
}

function phase(name, projects, count) {
  const t0 = Date.now();
  const args = ["playwright", "test", ...projects.flatMap((p) => ["--project", p]), "--pass-with-no-tests", ...extra];
  const run = spawnSync("npx", args, {
    stdio: "inherit",
    env: { ...process.env, E2E_BASE_URL: base, E2E_API_URL: api, E2E_WORKERS: String(count) },
  });
  const s = Math.round((Date.now() - t0) / 1000);
  console.log(`e2e: ${name}: ${run.status === 0 ? "passed" : "FAILED"} in ${Math.floor(s / 60)}m${String(s % 60).padStart(2, "0")}s`);
  return run.status ?? 1;
}

const publicPages = phase(`public pages (${workers} workers)`, ["public-desktop", "public-mobile"], workers);
const journeys = phase("shared-state journeys (1 worker)", ["desktop", "mobile"], 1);
stopServer();
process.exit(publicPages || journeys);
