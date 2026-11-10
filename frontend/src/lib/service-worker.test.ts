import { readFileSync } from "node:fs";
import { join } from "node:path";
import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * public/sw.js runs in a ServiceWorkerGlobalScope, so it is evaluated here
 * against a mocked scope (docs/platform-extensions.md §23.6): push, click and
 * pushsubscriptionchange handlers, the app badge, and malformed input.
 */
const SOURCE = readFileSync(join(process.cwd(), "public", "sw.js"), "utf8");
const ORIGIN = "https://staryield.net";

type Handler = (event: Record<string, unknown>) => void;

type FakeClient = {
  url: string;
  focus: ReturnType<typeof vi.fn>;
  navigate?: ReturnType<typeof vi.fn>;
  postMessage: ReturnType<typeof vi.fn>;
};

function client(url: string, over: Partial<FakeClient> = {}): FakeClient {
  const c: FakeClient = { url, focus: vi.fn(), navigate: vi.fn(), postMessage: vi.fn(), ...over };
  if (!("focus" in over)) c.focus.mockImplementation(async () => c);
  if (!("navigate" in over)) c.navigate!.mockImplementation(async () => c);
  return c;
}

function boot(opts: { badge?: boolean } = {}) {
  const handlers: Record<string, Handler> = {};
  const scope = {
    location: { origin: ORIGIN },
    addEventListener: (type: string, fn: Handler) => {
      handlers[type] = fn;
    },
    skipWaiting: vi.fn(),
    registration: { showNotification: vi.fn().mockResolvedValue(undefined), pushManager: { getSubscription: vi.fn().mockResolvedValue(null), subscribe: vi.fn() } },
    clients: { claim: vi.fn(), matchAll: vi.fn().mockResolvedValue([]), openWindow: vi.fn().mockResolvedValue(null) },
    navigator: opts.badge === false ? {} : { setAppBadge: vi.fn().mockResolvedValue(undefined) },
  };
  new Function("self", SOURCE)(scope);
  return { scope, handlers };
}

/** Run a handler and wait for what it handed to event.waitUntil. */
async function dispatch(handler: Handler, event: Record<string, unknown>) {
  const waits: Promise<unknown>[] = [];
  handler({ ...event, waitUntil: (p: Promise<unknown>) => waits.push(Promise.resolve(p)) });
  await Promise.all(waits);
}

const pushEvent = (data: unknown) => ({ data: { json: () => data, text: () => JSON.stringify(data) } });
const clickEvent = (data: Record<string, unknown>) => ({ notification: { close: vi.fn(), data } });

describe("lifecycle", () => {
  it("activates at once and claims open pages", async () => {
    const { scope, handlers } = boot();
    handlers.install({});
    expect(scope.skipWaiting).toHaveBeenCalled();
    await dispatch(handlers.activate, {});
    expect(scope.clients.claim).toHaveBeenCalled();
  });
});

describe("push", () => {
  it("shows the notification with the payload's copy, same-origin url, id and kind", async () => {
    const { scope, handlers } = boot();
    await dispatch(handlers.push, pushEvent({ id: "n1", kind: "export.ready", title: "Ready", body: "Download it", url: "/en/profile/settings", tag: "export" }));
    const [title, options] = scope.registration.showNotification.mock.calls[0];
    expect(title).toBe("Ready");
    expect(options).toMatchObject({
      body: "Download it",
      tag: "export",
      renotify: true,
      data: { url: `${ORIGIN}/en/profile/settings`, kind: "export.ready", id: "n1" },
    });
    expect(options.requireInteraction).toBeUndefined();
  });

  it("keeps chat requests on screen and buzzing", async () => {
    const { scope, handlers } = boot();
    await dispatch(handlers.push, pushEvent({ kind: "chat.requested", title: "New chat request", body: "x", url: "/en/chatroom", tag: "chat-request" }));
    const options = scope.registration.showNotification.mock.calls[0][1];
    expect(options.requireInteraction).toBe(true);
    expect(options.vibrate).toEqual([200, 100, 200, 100, 400]);
    expect(options.tag).toBe("chat-request");
    expect(options.data.id).toBe("");
  });

  it("chat message pushes have no id and no requireInteraction", async () => {
    const { scope, handlers } = boot();
    await dispatch(handlers.push, pushEvent({ kind: "chat.message", title: "Theo", body: "Hello", url: "/en/chatroom?reader=theo", tag: "conv-1" }));
    const options = scope.registration.showNotification.mock.calls[0][1];
    expect(options.data.id).toBe("");
    expect(options.requireInteraction).toBeUndefined();
    expect(options.tag).toBe("conv-1");
  });

  it("sets the app badge where the Badging API exists", async () => {
    const { scope, handlers } = boot();
    await dispatch(handlers.push, pushEvent({ title: "x" }));
    expect(scope.navigator.setAppBadge).toHaveBeenCalledTimes(1);
  });

  it("does not fail without the Badging API or when it throws", async () => {
    const without = boot({ badge: false });
    await dispatch(without.handlers.push, pushEvent({ title: "x" }));
    expect(without.scope.registration.showNotification).toHaveBeenCalled();
    const throwing = boot();
    (throwing.scope.navigator.setAppBadge as ReturnType<typeof vi.fn>).mockRejectedValue(new Error("nope"));
    await dispatch(throwing.handlers.push, pushEvent({ title: "x" }));
    expect(throwing.scope.registration.showNotification).toHaveBeenCalled();
  });

  it("never sends people to another origin and drops an unsafe id", async () => {
    const { scope, handlers } = boot();
    await dispatch(handlers.push, pushEvent({ id: "../../x?y", title: "x", url: "https://evil.example/phish" }));
    const options = scope.registration.showNotification.mock.calls[0][1];
    expect(options.data.url).toBe(`${ORIGIN}/`);
    expect(options.data.id).toBe("");
  });

  it.each([
    ["no data", {}],
    ["text that is not JSON", { data: { json: () => { throw new SyntaxError("bad"); }, text: () => "hello there" } }],
    ["null", pushEvent(null)],
    ["a number", pushEvent(42)],
    ["an array", pushEvent([1, 2])],
    ["a string", pushEvent("oops")],
    ["wrong field types", pushEvent({ title: 5, body: {}, url: 7, tag: [], id: 9, kind: null })],
    ["data that throws on every read", { data: { json: () => { throw new Error("a"); }, text: () => { throw new Error("b"); } } }],
  ])("shows a generic notification and never throws for %s", async (_name, event) => {
    const { scope, handlers } = boot();
    await expect(dispatch(handlers.push, event as Record<string, unknown>)).resolves.toBeUndefined();
    expect(scope.registration.showNotification).toHaveBeenCalledTimes(1);
    expect(scope.registration.showNotification.mock.calls[0][0]).toBe("Staryield");
  });

  it("truncates oversized fields", async () => {
    const { scope, handlers } = boot();
    await dispatch(handlers.push, pushEvent({ title: "t".repeat(500), body: "b".repeat(900), tag: "x".repeat(200) }));
    const [title, options] = scope.registration.showNotification.mock.calls[0];
    expect(title.length).toBe(120);
    expect(options.body.length).toBe(300);
    expect(options.tag.length).toBe(64);
  });
});

describe("notificationclick", () => {
  const data = { url: `${ORIGIN}/en/chatroom?reader=theo`, kind: "waitlist.available", id: "n9" };

  it("focuses an open tab already on that page and tells it which notification was opened (no reload)", async () => {
    const { scope, handlers } = boot();
    const open = client(`${ORIGIN}/en/chatroom?reader=theo`);
    const other = client(`${ORIGIN}/en/profile`);
    scope.clients.matchAll.mockResolvedValue([other, open]);
    const event = clickEvent(data);
    await dispatch(handlers.notificationclick, event);
    expect(event.notification.close).toHaveBeenCalled();
    expect(open.focus).toHaveBeenCalled();
    expect(open.navigate).not.toHaveBeenCalled();
    expect(open.postMessage).toHaveBeenCalledWith({ type: "notification.opened", id: "n9" });
    expect(scope.clients.openWindow).not.toHaveBeenCalled();
  });

  it("recognises the page even when it still carries an earlier sy_n", async () => {
    const { scope, handlers } = boot();
    const open = client(`${ORIGIN}/en/chatroom?reader=theo&sy_n=old`);
    scope.clients.matchAll.mockResolvedValue([open]);
    await dispatch(handlers.notificationclick, clickEvent(data));
    expect(open.navigate).not.toHaveBeenCalled();
    expect(open.postMessage).toHaveBeenCalledWith({ type: "notification.opened", id: "n9" });
  });

  it("navigates another tab of the site to the URL with sy_n=<id>", async () => {
    const { scope, handlers } = boot();
    const other = client(`${ORIGIN}/en/profile`);
    scope.clients.matchAll.mockResolvedValue([other]);
    await dispatch(handlers.notificationclick, clickEvent(data));
    expect(other.focus).toHaveBeenCalled();
    expect(other.navigate).toHaveBeenCalledWith(`${ORIGIN}/en/chatroom?reader=theo&sy_n=n9`);
    expect(other.postMessage).toHaveBeenCalledWith({ type: "notification.opened", id: "n9" });
    expect(scope.clients.openWindow).not.toHaveBeenCalled();
  });

  it("falls back to openWindow when navigate() rejects", async () => {
    const { scope, handlers } = boot();
    const other = client(`${ORIGIN}/en/profile`, { navigate: vi.fn().mockRejectedValue(new TypeError("not controlled")) });
    scope.clients.matchAll.mockResolvedValue([other]);
    await dispatch(handlers.notificationclick, clickEvent(data));
    expect(scope.clients.openWindow).toHaveBeenCalledWith(`${ORIGIN}/en/chatroom?reader=theo&sy_n=n9`);
  });

  it("falls back to openWindow when navigate() resolves with nothing or does not exist", async () => {
    const a = boot();
    a.scope.clients.matchAll.mockResolvedValue([client(`${ORIGIN}/en`, { navigate: vi.fn().mockResolvedValue(null) })]);
    await dispatch(a.handlers.notificationclick, clickEvent(data));
    expect(a.scope.clients.openWindow).toHaveBeenCalledTimes(1);
    const b = boot();
    const bare = client(`${ORIGIN}/en`);
    delete bare.navigate;
    b.scope.clients.matchAll.mockResolvedValue([bare]);
    await dispatch(b.handlers.notificationclick, clickEvent(data));
    expect(b.scope.clients.openWindow).toHaveBeenCalledWith(`${ORIGIN}/en/chatroom?reader=theo&sy_n=n9`);
  });

  it("opens a window when no tab is open", async () => {
    const { scope, handlers } = boot();
    await dispatch(handlers.notificationclick, clickEvent(data));
    expect(scope.clients.openWindow).toHaveBeenCalledWith(`${ORIGIN}/en/chatroom?reader=theo&sy_n=n9`);
  });

  it("without an id (chat messages) the URL is left alone and nothing is posted", async () => {
    const { scope, handlers } = boot();
    const other = client(`${ORIGIN}/en/profile`);
    scope.clients.matchAll.mockResolvedValue([other]);
    await dispatch(handlers.notificationclick, clickEvent({ url: `${ORIGIN}/en/chatroom?reader=theo`, kind: "chat.message" }));
    expect(other.navigate).toHaveBeenCalledWith(`${ORIGIN}/en/chatroom?reader=theo`);
    expect(other.postMessage).not.toHaveBeenCalled();
    const empty = boot();
    await dispatch(empty.handlers.notificationclick, clickEvent({ url: `${ORIGIN}/en/chatroom` }));
    expect(empty.scope.clients.openWindow).toHaveBeenCalledWith(`${ORIGIN}/en/chatroom`);
  });

  it("ignores clients of other origins and survives missing notification data", async () => {
    const { scope, handlers } = boot();
    const foreign = client("https://evil.example/x");
    scope.clients.matchAll.mockResolvedValue([foreign]);
    await dispatch(handlers.notificationclick, { notification: { close: vi.fn(), data: null } });
    expect(foreign.navigate).not.toHaveBeenCalled();
    expect(scope.clients.openWindow).toHaveBeenCalledWith(`${ORIGIN}/`);
  });

  it("a page that cannot be messaged still gets the URL parameter", async () => {
    const { scope, handlers } = boot();
    const other = client(`${ORIGIN}/en/profile`, { postMessage: vi.fn(() => { throw new Error("closed"); }) });
    scope.clients.matchAll.mockResolvedValue([other]);
    await dispatch(handlers.notificationclick, clickEvent(data));
    expect(other.navigate).toHaveBeenCalledWith(`${ORIGIN}/en/chatroom?reader=theo&sy_n=n9`);
  });
});

describe("pushsubscriptionchange", () => {
  const key = new Uint8Array([4, 1, 2, 3]).buffer;

  it("re-subscribes with the previous key and tells open pages", async () => {
    const { scope, handlers } = boot();
    const a = client(`${ORIGIN}/en`);
    const b = client(`${ORIGIN}/fr`);
    scope.clients.matchAll.mockResolvedValue([a, b]);
    scope.registration.pushManager.subscribe.mockResolvedValue({ endpoint: "https://push.example/new" });
    await dispatch(handlers.pushsubscriptionchange, { oldSubscription: { options: { applicationServerKey: key, userVisibleOnly: true } } });
    expect(scope.registration.pushManager.subscribe).toHaveBeenCalledWith({ userVisibleOnly: true, applicationServerKey: key });
    for (const c of [a, b]) expect(c.postMessage).toHaveBeenCalledWith({ type: "push.subscription_changed" });
  });

  it("does not subscribe again when the browser already made the new subscription", async () => {
    const { scope, handlers } = boot();
    const a = client(`${ORIGIN}/en`);
    scope.clients.matchAll.mockResolvedValue([a]);
    await dispatch(handlers.pushsubscriptionchange, { newSubscription: { endpoint: "https://push.example/new" }, oldSubscription: null });
    expect(scope.registration.pushManager.subscribe).not.toHaveBeenCalled();
    expect(a.postMessage).toHaveBeenCalledWith({ type: "push.subscription_changed" });
  });

  it("takes the key of the current subscription when the old one is gone", async () => {
    const { scope, handlers } = boot();
    scope.registration.pushManager.getSubscription.mockResolvedValue({ options: { applicationServerKey: key } });
    scope.registration.pushManager.subscribe.mockResolvedValue({});
    await dispatch(handlers.pushsubscriptionchange, {});
    expect(scope.registration.pushManager.subscribe).toHaveBeenCalledWith({ userVisibleOnly: true, applicationServerKey: key });
  });

  it("without any key it cannot subscribe, but still tells the pages so they re-sync", async () => {
    const { scope, handlers } = boot();
    const a = client(`${ORIGIN}/en`);
    scope.clients.matchAll.mockResolvedValue([a]);
    await dispatch(handlers.pushsubscriptionchange, {});
    expect(scope.registration.pushManager.subscribe).not.toHaveBeenCalled();
    expect(a.postMessage).toHaveBeenCalledWith({ type: "push.subscription_changed" });
  });

  it("never throws: a failing subscribe or an unreachable page is swallowed", async () => {
    const { scope, handlers } = boot();
    const a = client(`${ORIGIN}/en`, { postMessage: vi.fn(() => { throw new Error("gone"); }) });
    scope.clients.matchAll.mockResolvedValue([a]);
    scope.registration.pushManager.subscribe.mockRejectedValue(new DOMException("denied", "NotAllowedError"));
    await expect(dispatch(handlers.pushsubscriptionchange, { oldSubscription: { options: { applicationServerKey: key } } })).resolves.toBeUndefined();
    scope.clients.matchAll.mockRejectedValue(new Error("no clients"));
    await expect(dispatch(handlers.pushsubscriptionchange, { oldSubscription: { options: { applicationServerKey: key } } })).resolves.toBeUndefined();
  });
});

beforeEach(() => vi.restoreAllMocks());
