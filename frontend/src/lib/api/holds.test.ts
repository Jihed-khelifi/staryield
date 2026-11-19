// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

type Call = { url: string; init: RequestInit };
let calls: Call[];
let respond: (url: string, init: RequestInit) => Response;

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

async function load() {
  vi.resetModules();
  return import("@/lib/api/holds");
}

beforeEach(() => {
  calls = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit = {}) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/api/v1/auth/csrf")) return json(200, { csrf_token: "tok" });
      return respond(String(url), init);
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

const last = (part: string) => calls.filter((c) => c.url.includes(part)).at(-1)!;
const HOLD = { id: "h/1", session_id: "s1", conversation_id: "c1", reason: "trial_ended", status: "open", started_at: "", expires_at: "", price_credits: 40, held_messages: 0 };

describe("holdsApi (§26.1)", () => {
  it("GET /chat/holds/:id (the id is escaped)", async () => {
    respond = () => json(200, HOLD);
    const { holdsApi } = await load();
    await expect(holdsApi.get("h/1")).resolves.toMatchObject({ price_credits: 40 });
    const call = last("/chat/holds/");
    expect(call.url).toContain("/api/v1/chat/holds/h%2F1");
    expect(call.init.method ?? "GET").toBe("GET");
  });

  it("POST /chat/sessions/:id/hold/continue with the idempotency key", async () => {
    respond = () => json(200, { session: { id: "s1" }, hold: HOLD, wallet: { balance: 60, held: 0, free_minutes: 0 } });
    const { holdsApi } = await load();
    await holdsApi.continueReading("s1", "key-1");
    const call = last("/chat/sessions/s1/hold/continue");
    expect(call.init.method).toBe("POST");
    expect(JSON.parse(String(call.init.body))).toEqual({ idempotency_key: "key-1" });
  });

  it("POST /chat/holds/:id/reveal with the idempotency key, and a key of its own by default", async () => {
    respond = () => json(200, { hold: HOLD, messages: [], wallet: { balance: 20, held: 0, free_minutes: 0 } });
    const { holdsApi } = await load();
    await holdsApi.reveal("h1", "key-2");
    expect(JSON.parse(String(last("/chat/holds/h1/reveal").init.body))).toEqual({ idempotency_key: "key-2" });
    await holdsApi.reveal("h1");
    expect(JSON.parse(String(last("/chat/holds/h1/reveal").init.body)).idempotency_key).toMatch(/\S{8,}/);
  });

  it("402 insufficient_credits keeps the server's numbers", async () => {
    respond = () => json(402, { error: { code: "insufficient_credits", message: "no", details: { required: 40, balance: 5 } } });
    const { holdsApi } = await load();
    await expect(holdsApi.reveal("h1", "k")).rejects.toMatchObject({ code: "insufficient_credits", status: 402, details: { required: 40, balance: 5 } });
  });
});
