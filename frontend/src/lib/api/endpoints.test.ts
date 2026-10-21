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
  document.documentElement.lang = "fr";
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit = {}) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/api/v1/auth/csrf")) return json(200, { csrf_token: "tok" });
      return respond(String(url), init);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  document.documentElement.lang = "";
});

const last = (part: string) => calls.filter((c) => c.url.includes(part)).at(-1)!;
const header = (c: Call, name: string) => new Headers(c.init.headers).get(name);

describe("chat attachments", () => {
  it("uploads an image as multipart with caption and client id (no JSON content type)", async () => {
    respond = () => json(201, { message: { id: "m1", kind: "image" } });
    const { chatApi } = await load();
    const image = new File([new Uint8Array([137, 80, 78, 71])], "moon.png", { type: "image/png" });
    const res = await chatApi.uploadAttachment("sess-1", image, "Full moon\n  tonight", "m-abc");
    expect(res.message.id).toBe("m1");
    const call = last("/attachments");
    expect(call.url).toMatch(/\/api\/v1\/chat\/sessions\/sess-1\/attachments$/);
    expect(call.init.method).toBe("POST");
    const body = call.init.body as FormData;
    expect(body).toBeInstanceOf(FormData);
    expect((body.get("image") as File).name).toBe("moon.png");
    expect((body.get("image") as File).type).toBe("image/png");
    expect(body.get("caption")).toBe("Full moon\n  tonight");
    expect(body.get("client_msg_id")).toBe("m-abc");
    expect(header(call, "Content-Type")).toBeNull();
    expect(header(call, "X-CSRF-Token")).toBe("tok");
    expect(header(call, "X-Locale")).toBe("fr");
    expect(call.init.credentials).toBe("include");
  });

  it("omits an empty caption", async () => {
    const { attachmentForm } = await load();
    const form = attachmentForm(new Blob(["x"], { type: "image/gif" }), "", "m-1");
    expect(form.has("caption")).toBe(false);
    expect((form.get("image") as File).name).toBe("image");
  });

  it("fetches image bytes as a blob with credentials", async () => {
    respond = () => new Response("png", { status: 200, headers: { "Content-Type": "image/png" } });
    const { chatApi } = await load();
    const blob = await chatApi.attachment("att-1");
    expect(blob.type).toBe("image/png");
    expect(await blob.text()).toBe("png");
    const call = last("/chat/attachments/att-1");
    expect(call.init.credentials).toBe("include");
    expect(header(call, "Accept")).toContain("image/*");
  });

  it("refreshes an expired session before fetching an image", async () => {
    localStorage.setItem("staryield.auth-hint", "1");
    let attempts = 0;
    respond = (url) => {
      if (url.endsWith("/auth/refresh")) return json(200, { csrf_token: "tok2" });
      return ++attempts === 1 ? json(401, { error: { code: "session_expired", message: "expired" } }) : new Response("ok", { status: 200 });
    };
    const { chatApi } = await load();
    await expect(chatApi.attachment("att-2").then((b) => b.text())).resolves.toBe("ok");
    expect(calls.filter((c) => c.url.endsWith("/auth/refresh"))).toHaveLength(1);
    localStorage.clear();
  });
});

describe("other new endpoints", () => {
  it("sends attribution on register", async () => {
    respond = () => json(201, { user: { id: "u" }, csrf_token: "c", access_expires_at: "x" });
    const { authApi } = await load();
    await authApi.register({ email: "a@b.co", password: "P4ss!word", accept_terms: true, locale: "fr", attribution: { utm_source: "ig", click_id: "c1" } });
    expect(JSON.parse(String(last("/auth/register").init.body))).toEqual({
      email: "a@b.co",
      password: "P4ss!word",
      accept_terms: true,
      locale: "fr",
      attribution: { utm_source: "ig", click_id: "c1" },
    });
  });

  it("marks notifications read by id or all", async () => {
    respond = () => json(200, { unread: 0 });
    const { notificationsApi } = await load();
    await notificationsApi.markRead(["n1", "n2"]);
    expect(JSON.parse(String(last("/me/notifications/read").init.body))).toEqual({ ids: ["n1", "n2"] });
    await notificationsApi.markAllRead();
    expect(JSON.parse(String(last("/me/notifications/read").init.body))).toEqual({ all: true });
  });

  it("builds analytics, reviews and withdrawal requests", async () => {
    respond = () => json(200, { items: [] });
    const { psychicsApi, reviewsApi, withdrawalsApi } = await load();
    await psychicsApi.analytics({ from: "2026-10-01T00:00:00Z", to: "2026-10-20T00:00:00Z", tz: "Europe/Paris", interval: "day" });
    const a = new URL(last("/psychic/analytics").url);
    expect(Object.fromEntries(a.searchParams)).toEqual({ from: "2026-10-01T00:00:00Z", to: "2026-10-20T00:00:00Z", tz: "Europe/Paris", interval: "day" });

    await reviewsApi.forPsychic("theo", "cur-2", 10);
    expect(last("/psychics/theo/reviews").url).toMatch(/cursor=cur-2/);

    respond = () => json(201, { id: "r1", rating: 5 });
    await reviewsApi.submit("sess-9", 5, "Great");
    expect(JSON.parse(String(last("/review").init.body))).toEqual({ rating: 5, comment: "Great" });
    await reviewsApi.submit("sess-9", 4);
    expect(JSON.parse(String(last("/review").init.body))).toEqual({ rating: 4 });

    await withdrawalsApi.request({ amount_credits: 3000, method: "paypal", destination: "me@x.io", idempotency_key: "k1" });
    const w = last("/psychic/withdrawals");
    expect(JSON.parse(String(w.init.body))).toEqual({ amount_credits: 3000, method: "paypal", destination: "me@x.io", idempotency_key: "k1" });
    await withdrawalsApi.cancel("w1");
    expect(last("/cancel").url).toMatch(/\/psychic\/withdrawals\/w1\/cancel$/);
  });

  it("loads the client profile of a conversation", async () => {
    respond = () => json(200, { id: "c", name: "Amélie" });
    const { chatApi } = await load();
    await chatApi.clientProfile("conv-1");
    expect(last("/client").url).toMatch(/\/chat\/conversations\/conv-1\/client$/);
  });
});
