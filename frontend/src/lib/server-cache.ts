/**
 * Per-isolate memory cache for public API reads made while rendering on the
 * server (site configuration, pricing). On the edge every render reaches the
 * API from a few shared egress addresses (one rate-limit bucket), so each
 * isolate asks at most once per TTL and key, shares one request between
 * concurrent renders, serves the last good copy while refreshing, and backs
 * off briefly after a failure.
 */
type Entry<T> = { value: T | null; expires: number; pending?: Promise<T | null> };

export type ServerCache<T> = {
  /** Cached value for `key`, loading it with `load` when missing or expired (null on failure with nothing cached). */
  get(key: string, load: () => Promise<T | null>, now?: () => number): Promise<T | null>;
  /** Forget every cached value (tests). */
  clear(): void;
};

export function createServerCache<T>({ ttlMs, failureTtlMs }: { ttlMs: number; failureTtlMs: number }): ServerCache<T> {
  const memory = new Map<string, Entry<T>>();
  return {
    clear: () => memory.clear(),
    async get(key, load, now = Date.now) {
      const hit = memory.get(key);
      if (hit && hit.expires > now()) return hit.value;
      const stale = hit?.value ?? null;
      // A refresh is in flight: serve the last good copy at once, else wait for it.
      if (hit?.pending) return stale ?? hit.pending;
      const pending = load().then((fresh) => {
        // A failed refresh keeps the last good copy and is retried soon.
        const value = fresh ?? stale;
        memory.set(key, { value, expires: now() + (fresh ? ttlMs : failureTtlMs) });
        return value;
      });
      memory.set(key, { value: stale, expires: 0, pending });
      return stale ?? pending;
    },
  };
}
