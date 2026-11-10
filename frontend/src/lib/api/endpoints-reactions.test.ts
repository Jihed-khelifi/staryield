// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

type Call = { url: string; init: RequestInit };
let calls: Call[];
let respond: (url: string, init: RequestInit) => Response;

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

async function load() {
  vi.resetModules();
  return import("@/lib/api/endpoints");
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

describe("reply_to_id (§23.2): sent only when replying", () => {
  const REPLY = "55555555-5555-4555-8555-555555555555";

  it("text: in the body when replying, absent otherwise", async () => {
    respond = () => json(201, { message: { id: "m1" } });
    const { chatApi } = await load();
    await chatApi.sendToConversation("c1", "hello", "k1");
    expect(JSON.parse(String(last("/conversations/c1/messages").init.body))).toEqual({ body: "hello", client_msg_id: "k1" });
    await chatApi.sendToConversation("c1", "hello", "k2", REPLY);
    expect(JSON.parse(String(last("/conversations/c1/messages").init.body))).toEqual({ body: "hello", client_msg_id: "k2", reply_to_id: REPLY });
  });

  it("image uploads: a multipart field when replying, absent otherwise", async () => {
    const { attachmentForm } = await load();
    const blob = new Blob(["x"], { type: "image/png" });
    expect(attachmentForm(blob, "cap", "k1").has("reply_to_id")).toBe(false);
    expect(attachmentForm(blob, "cap", "k1", "image", REPLY).get("reply_to_id")).toBe(REPLY);
    respond = () => json(201, { message: { id: "m1" } });
    const { chatApi } = await load();
    await chatApi.uploadToConversation("c1", blob, undefined, "k3", undefined, REPLY);
    expect((last("/conversations/c1/attachments").init.body as FormData).get("reply_to_id")).toBe(REPLY);
    await chatApi.uploadToConversation("c1", blob, undefined, "k4");
    expect((last("/conversations/c1/attachments").init.body as FormData).has("reply_to_id")).toBe(false);
  });
});

describe("likes (§23.3) and nicknames (§23.4)", () => {
  it("PUT and DELETE the like of a message", async () => {
    respond = () => json(200, { message_id: "m1", conversation_id: "c1", seq: 3, liked_by: ["u1"] });
    const { chatApi } = await load();
    await expect(chatApi.like("m1")).resolves.toMatchObject({ liked_by: ["u1"] });
    expect(last("/messages/m1/like").init.method).toBe("PUT");
    await chatApi.unlike("m1");
    expect(last("/messages/m1/like").init.method).toBe("DELETE");
  });

  it("sets, clears and looks up nicknames", async () => {
    respond = (_url, init) => (init.method === "DELETE" ? new Response(null, { status: 204 }) : json(200, { client_id: "k1", alias: "Moon Paul", updated_at: "2026-11-05T10:00:00Z", items: [{ client_id: "k1", alias: "Moon Paul", updated_at: "x" }] }));
    const { aliasApi } = await load();
    await aliasApi.set("c1", "Moon Paul");
    expect(JSON.parse(String(last("/conversations/c1/alias").init.body))).toEqual({ alias: "Moon Paul" });
    expect(last("/conversations/c1/alias").init.method).toBe("PUT");
    await aliasApi.clear("c1");
    expect(last("/conversations/c1/alias").init.method).toBe("DELETE");
    await expect(aliasApi.lookup(["k1", "k2"])).resolves.toEqual([{ client_id: "k1", alias: "Moon Paul", updated_at: "x" }]);
    expect(decodeURIComponent(last("/chat/aliases").url)).toContain("client_ids=k1,k2");
  });
});
