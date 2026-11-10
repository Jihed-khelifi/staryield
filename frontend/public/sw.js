/*
 * Staryield service worker: Web Push notifications only (no offline cache).
 *
 * The API sends a JSON payload {id?, kind, title, body, url, tag}
 * (backend/internal/service/push.go PushPayload). Clicking a notification
 * focuses an open tab on that page, or navigates/opens one (with sy_n=<id> so
 * the app marks it read). Only same-origin URLs are opened, so a payload can
 * never send the user to another site. docs/platform-extensions.md §23.6.
 */

const ICON = "https://assets.staryield.net/assets/brand/sun-mark-192-c139a00d54.png";

self.addEventListener("install", () => {
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

/** A same-origin absolute URL for `raw`, or the site root. */
function safeUrl(raw) {
  const home = new URL("/", self.location.origin).href;
  if (typeof raw !== "string" || !raw) return home;
  try {
    const url = new URL(raw, self.location.origin);
    return url.origin === self.location.origin ? url.href : home;
  } catch {
    return home;
  }
}

function text(value, max) {
  return typeof value === "string" ? value.slice(0, max) : "";
}

/** The query parameter the app reads to mark the opened notification read. */
const OPENED_PARAM = "sy_n";

/** A notification id the app accepts (same pattern as the app). */
function safeId(value) {
  return typeof value === "string" && /^[A-Za-z0-9_-]{1,64}$/.test(value) ? value : "";
}

/** `url` with sy_n=<id> (or without any sy_n when `id` is empty). */
function withOpened(url, id) {
  try {
    const next = new URL(url);
    next.searchParams.delete(OPENED_PARAM);
    if (id) next.searchParams.set(OPENED_PARAM, id);
    return next.href;
  } catch {
    return url;
  }
}

/** Bring a window forward; resolves with the client to talk to, never rejects. */
async function focusClient(client) {
  try {
    return (await client.focus()) || client;
  } catch {
    return client;
  }
}

/** Set the app badge (Badging API, where it exists); never throws. */
async function setBadge() {
  try {
    const nav = self.navigator;
    if (nav && typeof nav.setAppBadge === "function") await nav.setAppBadge();
  } catch {
    /* the badge is a nicety */
  }
}

self.addEventListener("push", (event) => {
  let data = {};
  try {
    if (event.data) {
      try {
        data = event.data.json();
      } catch {
        data = { body: event.data.text() };
      }
    }
  } catch {
    data = {};
  }
  // Payloads that are not objects (null, a number, a string) are treated as empty.
  if (!data || typeof data !== "object" || Array.isArray(data)) data = {};
  const title = text(data.title, 120) || "Staryield";
  const tag = text(data.tag, 64);
  const options = {
    body: text(data.body, 300),
    icon: ICON,
    badge: ICON,
    data: { url: safeUrl(data.url), kind: text(data.kind, 64), id: safeId(data.id) },
  };
  if (tag) {
    options.tag = tag;
    options.renotify = true;
  }
  if (options.data.kind === "chat.requested") {
    // A live chat request: stay on screen until answered and buzz the phone.
    options.requireInteraction = true;
    options.vibrate = [200, 100, 200, 100, 400];
  }
  event.waitUntil(Promise.all([self.registration.showNotification(title, options), setBadge()]));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const info = event.notification.data || {};
  const id = safeId(info.id);
  const base = withOpened(safeUrl(info.url), "");
  const target = id ? withOpened(base, id) : base;
  event.waitUntil(
    (async () => {
      const windows = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
      const tell = (client) => {
        if (!id) return;
        try {
          client.postMessage({ type: "notification.opened", id });
        } catch {
          /* the URL parameter covers a page that does not hear us */
        }
      };
      // The page is already open on that URL: bring it forward, no reload.
      const exact = windows.find((client) => withOpened(client.url, "") === base);
      if (exact && "focus" in exact) {
        const focused = await focusClient(exact);
        tell(focused);
        return focused;
      }
      const sameSite = windows.find((client) => {
        try {
          return new URL(client.url).origin === self.location.origin;
        } catch {
          return false;
        }
      });
      if (sameSite && typeof sameSite.navigate === "function") {
        try {
          const focused = await focusClient(sameSite);
          tell(focused);
          // navigate() rejects for clients this worker does not control.
          const navigated = await focused.navigate(target);
          if (navigated) return navigated;
        } catch {
          /* fall back to a new window */
        }
      }
      return self.clients.openWindow(target);
    })()
  );
});

/**
 * The browser replaced or expired the push subscription (key rotation, the
 * push service changed endpoints): subscribe again with the same options and
 * tell open pages so PushSync sends the new endpoint to the API at once.
 */
self.addEventListener("pushsubscriptionchange", (event) => {
  event.waitUntil(
    (async () => {
      try {
        let subscription = event.newSubscription || null;
        if (!subscription) {
          const previous = event.oldSubscription && event.oldSubscription.options;
          let key = previous && previous.applicationServerKey;
          if (!key) {
            const current = await self.registration.pushManager.getSubscription().catch(() => null);
            key = current && current.options && current.options.applicationServerKey;
          }
          if (key) subscription = await self.registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: key });
        }
      } catch {
        /* the page re-subscribes silently on its next sync */
      }
      try {
        const windows = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
        for (const client of windows) client.postMessage({ type: "push.subscription_changed" });
      } catch {
        /* no open pages */
      }
    })()
  );
});
