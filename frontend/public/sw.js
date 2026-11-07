/*
 * Staryield service worker: Web Push notifications only (no offline cache).
 *
 * The API sends a JSON payload {kind, title, body, url, tag}
 * (backend/internal/service/push.go PushPayload). Clicking a notification
 * focuses an open tab on that page, or navigates/opens one. Only same-origin
 * URLs are opened, so a payload can never send the user to another site.
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

self.addEventListener("push", (event) => {
  let data = {};
  if (event.data) {
    try {
      data = event.data.json() || {};
    } catch {
      data = { body: event.data.text() };
    }
  }
  const title = text(data.title, 120) || "Staryield";
  const tag = text(data.tag, 64);
  const options = {
    body: text(data.body, 300),
    icon: ICON,
    badge: ICON,
    data: { url: safeUrl(data.url), kind: text(data.kind, 64) },
  };
  if (tag) {
    options.tag = tag;
    options.renotify = true;
  }
  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const target = safeUrl(event.notification.data && event.notification.data.url);
  event.waitUntil(
    (async () => {
      const windows = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
      const exact = windows.find((client) => client.url === target);
      if (exact && "focus" in exact) return exact.focus();
      const sameSite = windows.find((client) => new URL(client.url).origin === self.location.origin);
      if (sameSite && "navigate" in sameSite) {
        await sameSite.focus();
        return sameSite.navigate(target);
      }
      return self.clients.openWindow(target);
    })()
  );
});
