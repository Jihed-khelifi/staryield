// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api/errors";

const api = vi.hoisted(() => ({ pushApi: { config: vi.fn(), subscribe: vi.fn(), unsubscribe: vi.fn() } }));
vi.mock("@/lib/api/account", () => api);

import {
  PUSH_ENDPOINT_KEY_PREFIX,
  PUSH_REFRESH_INTERVAL_MS,
  PUSH_RESUBSCRIBE_KEY_PREFIX,
  PUSH_RESUBSCRIBE_RETRY_MS,
  PUSH_SYNC_KEY_PREFIX,
  PUSH_SIGN_OUT_CAP_MS,
  SERVICE_WORKER_TIMEOUT_MS,
  clearPushSyncMarkers,
  disablePush,
  enablePush,
  hashEndpoint,
  prepareSignOutPush,
  pushAvailability,
  pushOutcomeMessage,
  syncPush,
  vapidKeyBytes,
  type PushOutcome,
} from "@/lib/push";

const KEY = "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM";
const OUR_KEY_BYTES = Array.from(vapidKeyBytes(KEY));

function fakeSubscription(key: number[] | null = OUR_KEY_BYTES) {
  return {
    endpoint: "https://fcm.googleapis.com/fcm/send/abc",
    options: key ? { applicationServerKey: new Uint8Array(key).buffer } : {},
    toJSON: () => ({ endpoint: "https://fcm.googleapis.com/fcm/send/abc", keys: { p256dh: "p", auth: "a" }, expirationTime: null }),
    unsubscribe: vi.fn().mockResolvedValue(true),
  };
}

type Sub = ReturnType<typeof fakeSubscription>;

function setup(opts: { permission?: NotificationPermission; existing?: Sub | null } = {}) {
  const pushManager = {
    getSubscription: vi.fn().mockResolvedValue(opts.existing ?? null),
    subscribe: vi.fn().mockResolvedValue(fakeSubscription()),
  };
  const registration = { pushManager };
  const sw = {
    register: vi.fn().mockResolvedValue(registration),
    ready: Promise.resolve(registration),
    getRegistration: vi.fn().mockResolvedValue(registration),
  };
  Object.defineProperty(navigator, "serviceWorker", { configurable: true, value: sw });
  vi.stubGlobal("PushManager", class {});
  const notification = Object.assign(vi.fn(), {
    permission: opts.permission ?? "granted",
    requestPermission: vi.fn().mockResolvedValue(opts.permission ?? "granted"),
  });
  vi.stubGlobal("Notification", notification);
  Object.defineProperty(window, "isSecureContext", { configurable: true, value: true });
  return { pushManager, registration, sw, notification };
}

function domError(name: string, message = "x") {
  return new DOMException(message, name);
}

function setNavigator(props: { userAgent?: string; platform?: string; maxTouchPoints?: number; standalone?: boolean; brave?: boolean }) {
  const ua = props.userAgent ?? "Mozilla/5.0 (Macintosh)";
  Object.defineProperty(navigator, "userAgent", { configurable: true, value: ua });
  Object.defineProperty(navigator, "platform", { configurable: true, value: props.platform ?? "MacIntel" });
  Object.defineProperty(navigator, "maxTouchPoints", { configurable: true, value: props.maxTouchPoints ?? 0 });
  Object.defineProperty(navigator, "standalone", { configurable: true, value: props.standalone });
  Object.defineProperty(navigator, "brave", { configurable: true, value: props.brave ? { isBrave: () => Promise.resolve(true) } : undefined });
}

beforeEach(() => {
  api.pushApi.subscribe.mockReset().mockResolvedValue({ id: "1", created_at: "" });
  api.pushApi.unsubscribe.mockReset().mockResolvedValue(undefined);
  window.localStorage.clear();
  setNavigator({});
  window.matchMedia = vi.fn().mockReturnValue({ matches: false }) as unknown as typeof window.matchMedia;
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  Reflect.deleteProperty(navigator, "serviceWorker");
});

/** Run enablePush with timers faked so retry delays pass instantly. */
async function run(promise: Promise<PushOutcome>) {
  await vi.advanceTimersByTimeAsync(5000);
  return promise;
}

describe("pushOutcomeMessage", () => {
  it("has the docs §21.3 copy for every failure outcome", () => {
    const outcomes = ["denied", "dismissed", "unsupported", "ios_install", "brave_blocked", "service_error", "server_rejected", "failed"] as const;
    for (const o of outcomes) expect(pushOutcomeMessage(o).length).toBeGreaterThan(10);
    expect(pushOutcomeMessage("brave_blocked")).toContain("brave://settings/privacy");
    expect(pushOutcomeMessage("ios_install")).toContain("Add to Home Screen");
    expect(pushOutcomeMessage("failed")).toBe("We couldn't turn on notifications in this browser. Please try again.");
  });
});

describe("pushAvailability", () => {
  it("is available when everything is supported", () => {
    setup();
    expect(pushAvailability()).toBe("available");
  });
  it("is unsupported on a desktop browser without PushManager", () => {
    expect(pushAvailability()).toBe("unsupported");
  });
  it("asks iPhone browser tabs to install the app", () => {
    setNavigator({ userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)", platform: "iPhone" });
    expect(pushAvailability()).toBe("ios_install");
  });
  it("detects iPadOS that reports a desktop Mac", () => {
    setNavigator({ platform: "MacIntel", maxTouchPoints: 5 });
    expect(pushAvailability()).toBe("ios_install");
  });
  it("does not mistake a Mac without touch for an iPad", () => {
    setNavigator({ platform: "MacIntel", maxTouchPoints: 0 });
    expect(pushAvailability()).toBe("unsupported");
  });
  it("is not ios_install once the Home Screen app runs standalone (media query or navigator.standalone)", () => {
    setNavigator({ userAgent: "Mozilla/5.0 (iPad)", platform: "iPad" });
    window.matchMedia = vi.fn().mockReturnValue({ matches: true }) as unknown as typeof window.matchMedia;
    expect(pushAvailability()).toBe("unsupported");
    window.matchMedia = vi.fn().mockReturnValue({ matches: false }) as unknown as typeof window.matchMedia;
    setNavigator({ userAgent: "Mozilla/5.0 (iPad)", platform: "iPad", standalone: true });
    expect(pushAvailability()).toBe("unsupported");
  });
  it("is available in the standalone iOS app that has PushManager", () => {
    setup();
    setNavigator({ userAgent: "Mozilla/5.0 (iPhone)", platform: "iPhone", standalone: true });
    expect(pushAvailability()).toBe("available");
  });
});

describe("enablePush", () => {
  it("returns unsupported without push APIs", async () => {
    expect(await enablePush(KEY)).toBe("unsupported");
  });
  it("returns ios_install in an iPhone browser tab", async () => {
    setNavigator({ userAgent: "Mozilla/5.0 (iPhone)", platform: "iPhone" });
    expect(await enablePush(KEY)).toBe("ios_install");
  });
  it("subscribes, registers the worker with updateViaCache none and stores the subscription", async () => {
    const { sw, pushManager } = setup();
    expect(await enablePush(KEY)).toBe("subscribed");
    expect(sw.register).toHaveBeenCalledWith("/sw.js", { scope: "/", updateViaCache: "none" });
    expect(pushManager.subscribe).toHaveBeenCalledWith({ userVisibleOnly: true, applicationServerKey: expect.any(Uint8Array) });
    expect(api.pushApi.subscribe).toHaveBeenCalledTimes(1);
  });
  it("reuses an existing subscription made with the same key", async () => {
    const { pushManager } = setup({ existing: fakeSubscription() });
    expect(await enablePush(KEY)).toBe("subscribed");
    expect(pushManager.subscribe).not.toHaveBeenCalled();
  });
  it("replaces a subscription made with another application server key", async () => {
    const old = fakeSubscription([1, 2, 3]);
    const { pushManager } = setup({ existing: old });
    expect(await enablePush(KEY)).toBe("subscribed");
    expect(old.unsubscribe).toHaveBeenCalled();
    expect(pushManager.subscribe).toHaveBeenCalledTimes(1);
  });
  it("unsubscribes and subscribes again after InvalidStateError", async () => {
    const stale = fakeSubscription();
    const { pushManager } = setup();
    pushManager.getSubscription.mockResolvedValueOnce(null).mockResolvedValue(stale);
    pushManager.subscribe.mockRejectedValueOnce(domError("InvalidStateError")).mockResolvedValue(fakeSubscription());
    expect(await enablePush(KEY)).toBe("subscribed");
    expect(stale.unsubscribe).toHaveBeenCalled();
    expect(pushManager.subscribe).toHaveBeenCalledTimes(2);
  });

  describe("permission", () => {
    it("supports the promise form", async () => {
      const { notification } = setup({ permission: "default" });
      notification.requestPermission.mockResolvedValue("granted");
      expect(await enablePush(KEY)).toBe("subscribed");
    });
    it("supports the legacy callback form", async () => {
      const { notification } = setup({ permission: "default" });
      notification.requestPermission.mockImplementation((cb: (p: NotificationPermission) => void) => {
        cb("granted");
        return undefined;
      });
      expect(await enablePush(KEY)).toBe("subscribed");
    });
    it("reports denied and dismissed", async () => {
      const { notification } = setup({ permission: "default" });
      notification.requestPermission.mockResolvedValue("denied");
      expect(await enablePush(KEY)).toBe("denied");
      notification.requestPermission.mockResolvedValue("default");
      expect(await enablePush(KEY)).toBe("dismissed");
    });
    it("does not prompt when permission is already denied", async () => {
      const { notification } = setup({ permission: "denied" });
      expect(await enablePush(KEY)).toBe("denied");
      expect(notification.requestPermission).not.toHaveBeenCalled();
    });
    it("fails when the permission request throws", async () => {
      const { notification } = setup({ permission: "default" });
      notification.requestPermission.mockRejectedValue(new Error("boom"));
      expect(await enablePush(KEY)).toBe("failed");
    });
  });

  describe("push service errors", () => {
    const abort = () => domError("AbortError", "Registration failed - push service error");
    beforeEach(() => vi.useFakeTimers());

    it("retries an AbortError once after a second and succeeds", async () => {
      const { pushManager } = setup();
      pushManager.subscribe.mockRejectedValueOnce(abort()).mockResolvedValue(fakeSubscription());
      expect(await run(enablePush(KEY))).toBe("subscribed");
      expect(pushManager.subscribe).toHaveBeenCalledTimes(2);
    });
    it("reports brave_blocked when Brave keeps failing", async () => {
      setNavigator({ brave: true });
      const { pushManager } = setup();
      pushManager.subscribe.mockRejectedValue(abort());
      expect(await run(enablePush(KEY))).toBe("brave_blocked");
      expect(pushManager.subscribe).toHaveBeenCalledTimes(2);
    });
    it("reports service_error in other browsers", async () => {
      const { pushManager } = setup();
      pushManager.subscribe.mockRejectedValue(abort());
      expect(await run(enablePush(KEY))).toBe("service_error");
    });
    it("recognises the message even when the error name differs", async () => {
      const { pushManager } = setup();
      pushManager.subscribe.mockRejectedValue(new Error("Registration failed - push service error"));
      expect(await run(enablePush(KEY))).toBe("service_error");
    });
    it("treats a throwing navigator.brave as not Brave", async () => {
      Object.defineProperty(navigator, "brave", { configurable: true, value: { isBrave: () => Promise.reject(new Error("x")) } });
      const { pushManager } = setup();
      pushManager.subscribe.mockRejectedValue(abort());
      expect(await run(enablePush(KEY))).toBe("service_error");
    });
    it("maps NotAllowedError to denied without retrying", async () => {
      const { pushManager } = setup();
      pushManager.subscribe.mockRejectedValue(domError("NotAllowedError"));
      expect(await run(enablePush(KEY))).toBe("denied");
      expect(pushManager.subscribe).toHaveBeenCalledTimes(1);
    });
    it("maps anything else to failed", async () => {
      const { pushManager } = setup();
      pushManager.subscribe.mockRejectedValue(new Error("weird"));
      expect(await run(enablePush(KEY))).toBe("failed");
    });
  });

  describe("service worker", () => {
    it("returns service_error when ready never settles, never hanging", async () => {
      vi.useFakeTimers();
      const { sw } = setup();
      sw.ready = new Promise(() => undefined) as never;
      const promise = enablePush(KEY);
      await vi.advanceTimersByTimeAsync(SERVICE_WORKER_TIMEOUT_MS + 10);
      expect(await promise).toBe("service_error");
    });
    it("fails when registration throws", async () => {
      const { sw } = setup();
      sw.register.mockRejectedValue(new Error("nope"));
      expect(await enablePush(KEY)).toBe("failed");
    });
  });

  describe("API", () => {
    beforeEach(() => vi.useFakeTimers());
    it("maps a 422 to server_rejected without retrying", async () => {
      setup();
      api.pushApi.subscribe.mockRejectedValue(new ApiError({ code: "validation_failed", message: "x", status: 422 }));
      expect(await run(enablePush(KEY))).toBe("server_rejected");
      expect(api.pushApi.subscribe).toHaveBeenCalledTimes(1);
    });
    it("retries a network error once, then succeeds", async () => {
      setup();
      api.pushApi.subscribe.mockRejectedValueOnce(new ApiError({ code: "network_error", message: "x" })).mockResolvedValue({});
      expect(await run(enablePush(KEY))).toBe("subscribed");
      expect(api.pushApi.subscribe).toHaveBeenCalledTimes(2);
    });
    it("retries a 5xx once, then reports failed", async () => {
      setup();
      api.pushApi.subscribe.mockRejectedValue(new ApiError({ code: "internal_error", message: "x", status: 500 }));
      expect(await run(enablePush(KEY))).toBe("failed");
      expect(api.pushApi.subscribe).toHaveBeenCalledTimes(2);
    });
    it("does not retry other 4xx errors", async () => {
      setup();
      api.pushApi.subscribe.mockRejectedValue(new ApiError({ code: "forbidden", message: "x", status: 403 }));
      expect(await run(enablePush(KEY))).toBe("failed");
      expect(api.pushApi.subscribe).toHaveBeenCalledTimes(1);
    });
  });
});

describe("disablePush", () => {
  it("removes the server copy and the browser subscription", async () => {
    const sub = fakeSubscription();
    setup({ existing: sub });
    await disablePush();
    expect(api.pushApi.unsubscribe).toHaveBeenCalledWith(sub.endpoint);
    expect(sub.unsubscribe).toHaveBeenCalled();
  });
});

describe("syncPush", () => {
  const deps = (on = true, key: string | null = KEY) => ({
    userId: "u1",
    pushSwitchOn: vi.fn().mockResolvedValue(on),
    publicKey: vi.fn().mockResolvedValue(key),
  });

  it("does nothing without granted permission and never prompts", async () => {
    const { notification } = setup({ permission: "default" });
    const d = deps();
    expect(await syncPush(d)).toBe("skipped");
    expect(notification.requestPermission).not.toHaveBeenCalled();
    expect(api.pushApi.subscribe).not.toHaveBeenCalled();
    expect(d.pushSwitchOn).not.toHaveBeenCalled();
  });
  it("does nothing in unsupported browsers", async () => {
    expect(await syncPush(deps())).toBe("skipped");
  });
  it("re-sends an existing subscription and records the time", async () => {
    setup({ existing: fakeSubscription() });
    expect(await syncPush(deps())).toBe("refreshed");
    expect(api.pushApi.subscribe).toHaveBeenCalledTimes(1);
    expect(window.localStorage.getItem(PUSH_SYNC_KEY_PREFIX + "u1")).not.toBeNull();
  });
  it("throttles the refresh to once per 6 hours per user", async () => {
    setup({ existing: fakeSubscription() });
    window.localStorage.setItem(PUSH_SYNC_KEY_PREFIX + "u1", String(Date.now() - 60_000));
    expect(await syncPush(deps())).toBe("throttled");
    expect(api.pushApi.subscribe).not.toHaveBeenCalled();
    expect(await syncPush({ ...deps(), userId: "u2" })).toBe("refreshed");
    window.localStorage.setItem(PUSH_SYNC_KEY_PREFIX + "u1", String(Date.now() - PUSH_REFRESH_INTERVAL_MS - 1000));
    expect(await syncPush(deps())).toBe("refreshed");
  });
  it("still refreshes when localStorage throws", async () => {
    setup({ existing: fakeSubscription() });
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("private");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("private");
    });
    expect(await syncPush(deps())).toBe("refreshed");
    vi.restoreAllMocks();
  });
  it("resubscribes silently when the subscription is missing and the switch is on", async () => {
    const { pushManager, notification } = setup();
    expect(await syncPush(deps(true))).toBe("resubscribed");
    expect(pushManager.subscribe).toHaveBeenCalled();
    expect(notification.requestPermission).not.toHaveBeenCalled();
    expect(api.pushApi.subscribe).toHaveBeenCalledTimes(1);
  });
  it("retries a failed silent resubscribe (Brave's default) at most once an hour, not on every page load", async () => {
    vi.useFakeTimers();
    try {
      const { pushManager } = setup();
      pushManager.subscribe.mockRejectedValue(domError("AbortError", "Registration failed - push service error"));
      const first = syncPush(deps(true));
      await vi.advanceTimersByTimeAsync(5000);
      expect(await first).toBe("idle");
      expect(pushManager.subscribe).toHaveBeenCalledTimes(2);
      expect(await syncPush(deps(true))).toBe("throttled");
      expect(pushManager.subscribe).toHaveBeenCalledTimes(2);
      window.localStorage.setItem(PUSH_RESUBSCRIBE_KEY_PREFIX + "u1", String(Date.now() - PUSH_RESUBSCRIBE_RETRY_MS - 1000));
      pushManager.subscribe.mockResolvedValue(fakeSubscription());
      expect(await syncPush(deps(true))).toBe("resubscribed");
    } finally {
      vi.useRealTimers();
    }
  });
  it("does not resubscribe when the switch is off or the server has no key", async () => {
    const { pushManager } = setup();
    expect(await syncPush(deps(false))).toBe("idle");
    expect(await syncPush(deps(true, null))).toBe("idle");
    expect(pushManager.subscribe).not.toHaveBeenCalled();
  });
  it("swallows settings errors and never unsubscribes", async () => {
    const sub = fakeSubscription();
    const { pushManager } = setup();
    const d = deps();
    d.pushSwitchOn.mockRejectedValue(new Error("offline"));
    expect(await syncPush(d)).toBe("idle");
    expect(sub.unsubscribe).not.toHaveBeenCalled();
    expect(pushManager.subscribe).not.toHaveBeenCalled();
  });
});

describe("syncPush endpoint changes (§23.6)", () => {
  const deps = () => ({ userId: "u1", pushSwitchOn: vi.fn().mockResolvedValue(true), publicKey: vi.fn().mockResolvedValue(KEY) });
  const other = () => ({ ...fakeSubscription(), endpoint: "https://updates.push.services.mozilla.com/wpush/v2/new", toJSON: () => ({ endpoint: "https://updates.push.services.mozilla.com/wpush/v2/new", keys: { p256dh: "p", auth: "a" }, expirationTime: null }) });

  it("stores a hash of the endpoint it sent, not the endpoint itself", async () => {
    const sub = fakeSubscription();
    setup({ existing: sub });
    expect(await syncPush(deps())).toBe("refreshed");
    const stored = window.localStorage.getItem(PUSH_ENDPOINT_KEY_PREFIX + "u1");
    expect(stored).toBe(await hashEndpoint(sub.endpoint));
    expect(stored).not.toContain("fcm.googleapis.com");
  });

  it("re-sends at once when the browser's endpoint differs from the last one sent, despite the 6 h throttle", async () => {
    setup({ existing: fakeSubscription() });
    expect(await syncPush(deps())).toBe("refreshed");
    expect(await syncPush(deps())).toBe("throttled");
    expect(api.pushApi.subscribe).toHaveBeenCalledTimes(1);
    setup({ existing: other() });
    expect(await syncPush(deps())).toBe("refreshed");
    expect(api.pushApi.subscribe).toHaveBeenCalledTimes(2);
    expect(api.pushApi.subscribe.mock.calls[1][0].endpoint).toContain("mozilla");
    expect(await syncPush(deps())).toBe("throttled");
  });

  it("does not remember an endpoint the API did not accept", async () => {
    setup({ existing: fakeSubscription() });
    api.pushApi.subscribe.mockRejectedValue(new Error("offline"));
    expect(await syncPush(deps())).toBe("idle");
    expect(window.localStorage.getItem(PUSH_ENDPOINT_KEY_PREFIX + "u1")).toBeNull();
  });

  it("force (the service worker saw a change) skips the throttles", async () => {
    setup({ existing: fakeSubscription() });
    expect(await syncPush(deps())).toBe("refreshed");
    expect(await syncPush({ ...deps(), force: true })).toBe("refreshed");
    expect(api.pushApi.subscribe).toHaveBeenCalledTimes(2);
  });

  it("force also retries a missing subscription inside the hourly window", async () => {
    const { pushManager } = setup();
    window.localStorage.setItem(PUSH_RESUBSCRIBE_KEY_PREFIX + "u1", String(Date.now()));
    expect(await syncPush(deps())).toBe("throttled");
    expect(await syncPush({ ...deps(), force: true })).toBe("resubscribed");
    expect(pushManager.subscribe).toHaveBeenCalledTimes(1);
  });

  it("clearPushSyncMarkers forgets everything for that user only", () => {
    for (const prefix of [PUSH_SYNC_KEY_PREFIX, PUSH_RESUBSCRIBE_KEY_PREFIX, PUSH_ENDPOINT_KEY_PREFIX]) {
      window.localStorage.setItem(prefix + "u1", "x");
      window.localStorage.setItem(prefix + "u2", "x");
    }
    clearPushSyncMarkers("u1");
    for (const prefix of [PUSH_SYNC_KEY_PREFIX, PUSH_RESUBSCRIBE_KEY_PREFIX, PUSH_ENDPOINT_KEY_PREFIX]) {
      expect(window.localStorage.getItem(prefix + "u1")).toBeNull();
      expect(window.localStorage.getItem(prefix + "u2")).toBe("x");
    }
  });
});

describe("prepareSignOutPush (§23.6)", () => {
  it("returns this browser's endpoint and unsubscribes on demand", async () => {
    const sub = fakeSubscription();
    setup({ existing: sub });
    const push = await prepareSignOutPush();
    expect(push.endpoint).toBe(sub.endpoint);
    expect(sub.unsubscribe).not.toHaveBeenCalled();
    await push.unsubscribe();
    expect(sub.unsubscribe).toHaveBeenCalledTimes(1);
  });

  it("has nothing to do when push is unsupported or there is no subscription", async () => {
    expect((await prepareSignOutPush()).endpoint).toBeNull();
    setup({ existing: null });
    const push = await prepareSignOutPush();
    expect(push.endpoint).toBeNull();
    await expect(push.unsubscribe()).resolves.toBeUndefined();
  });

  it("never rejects when reading or removing the subscription fails", async () => {
    const sub = fakeSubscription();
    sub.unsubscribe.mockRejectedValue(new Error("boom"));
    const { sw } = setup({ existing: sub });
    const push = await prepareSignOutPush();
    await expect(push.unsubscribe()).resolves.toBeUndefined();
    sw.getRegistration.mockRejectedValue(new Error("no worker"));
    expect((await prepareSignOutPush()).endpoint).toBeNull();
  });

  it("gives up after the 3 s cap when the service worker never answers", async () => {
    vi.useFakeTimers();
    const { sw } = setup({ existing: fakeSubscription() });
    sw.getRegistration.mockReturnValue(new Promise(() => undefined));
    let done = false;
    const pending = prepareSignOutPush().then((p) => {
      done = true;
      return p;
    });
    await vi.advanceTimersByTimeAsync(PUSH_SIGN_OUT_CAP_MS - 10);
    expect(done).toBe(false);
    await vi.advanceTimersByTimeAsync(20);
    expect(done).toBe(true);
    expect((await pending).endpoint).toBeNull();
  });

  it("does not wait for a browser unsubscribe that never settles", async () => {
    vi.useFakeTimers();
    const sub = fakeSubscription();
    sub.unsubscribe.mockReturnValue(new Promise(() => undefined));
    setup({ existing: sub });
    const push = await prepareSignOutPush();
    let done = false;
    const pending = push.unsubscribe().then(() => {
      done = true;
    });
    await vi.advanceTimersByTimeAsync(PUSH_SIGN_OUT_CAP_MS + 10);
    await pending;
    expect(done).toBe(true);
  });
});
