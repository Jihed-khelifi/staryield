"use client";

import * as React from "react";

import { aliasApi } from "@/lib/api/endpoints";
import type { ClientAliasEvent } from "@/lib/api/types";
import { useRealtimeEvent, useSession } from "@/lib/auth/session-provider";
import { logger } from "@/lib/logger";

/** The API takes at most this many ids per lookup (§23.4). */
export const ALIAS_LOOKUP_MAX = 50;
/** `resolve` gives up waiting for a lookup after this long. */
export const ALIAS_RESOLVE_MS = 1500;
/** Ids asked for within this window share one request. */
export const ALIAS_BATCH_MS = 10;

/** `string`: the nickname; `null`: known to have none; missing: not known yet. */
type Aliases = Record<string, string | null>;

type Ctx = {
  /** Advisors only: everything else renders real names and never asks for aliases. */
  enabled: boolean;
  aliases: Aliases;
  /** Ask for the nicknames of clients that are not cached (batched, deduplicated, errors tolerated). */
  request: (clientIds: readonly (string | undefined)[]) => void;
  /** Cache what another response already told us (inbox rows, client profile); never overrides a known value. */
  prime: (entries: readonly { clientId: string | undefined; alias?: string | null }[]) => void;
  /**
   * The nickname for a one-off use (a toast, a notification): the cached value, or the answer of a
   * lookup it starts (waits at most `ALIAS_RESOLVE_MS`). `undefined`: none, unknown or not an advisor.
   */
  resolve: (clientId: string) => Promise<string | undefined>;
  /** Optimistic; rolls back and rejects with the API's error on failure. An empty value clears. */
  setAlias: (conversationId: string, clientId: string, value: string) => Promise<void>;
  clearAlias: (conversationId: string, clientId: string) => Promise<void>;
};

const NOOP = () => undefined;
const DISABLED: Ctx = {
  enabled: false,
  aliases: {},
  request: NOOP,
  prime: NOOP,
  resolve: () => Promise.resolve(undefined),
  setAlias: () => Promise.resolve(),
  clearAlias: () => Promise.resolve(),
};

const ClientAliasContext = React.createContext<Ctx>(DISABLED);

/** Collapse inner whitespace like the API does, so the optimistic value matches what is stored. */
export function normalizeAlias(value: string): string {
  return value.trim().replace(/\s+/g, " ");
}

/**
 * Advisors' private client nicknames (§23.4), cached by client id: primed from
 * the inbox and the client profile, resolved lazily in batches, kept current by
 * `client_alias.updated`. For clients (and signed-out visitors) it does
 * nothing: no state, no requests.
 */
export function ClientAliasProvider({ children }: { children: React.ReactNode }) {
  const { status, user } = useSession();
  const enabled = status === "authenticated" && user?.role === "psychic";
  const userId = user?.id ?? "";
  return <Provider enabled={enabled} userId={userId}>{children}</Provider>;
}

function Provider({ enabled, userId, children }: { enabled: boolean; userId: string; children: React.ReactNode }) {
  const [aliases, setAliases] = React.useState<Aliases>({});
  const cache = React.useRef<Aliases>({});
  /** Bumped by every write that must win over a lookup that started earlier. */
  const clock = React.useRef(0);
  const touched = React.useRef(new Map<string, number>());
  const queued = React.useRef(new Set<string>());
  const inflight = React.useRef(new Set<string>());
  const timer = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const alive = React.useRef(true);
  const waiters = React.useRef(new Map<string, Array<() => void>>());

  const commit = React.useCallback((patch: Aliases) => {
    cache.current = { ...cache.current, ...patch };
    setAliases(cache.current);
    for (const id of Object.keys(patch)) {
      waiters.current.get(id)?.forEach((wake) => wake());
      waiters.current.delete(id);
    }
  }, []);

  /** A write that wins over any lookup started before it. */
  const write = React.useCallback(
    (clientId: string, value: string | null) => {
      touched.current.set(clientId, ++clock.current);
      commit({ [clientId]: value });
    },
    [commit],
  );

  // A different account (or signing out) starts from nothing.
  React.useEffect(() => {
    alive.current = true;
    const pending = { touched: touched.current, queued: queued.current, inflight: inflight.current };
    return () => {
      alive.current = false;
      if (timer.current) clearTimeout(timer.current);
      timer.current = null;
      cache.current = {};
      pending.touched.clear();
      pending.queued.clear();
      pending.inflight.clear();
      setAliases({});
    };
  }, [enabled, userId]);

  const flush = React.useCallback(async () => {
    timer.current = null;
    const ids = [...queued.current];
    queued.current.clear();
    for (let i = 0; i < ids.length; i += ALIAS_LOOKUP_MAX) {
      const chunk = ids.slice(i, i + ALIAS_LOOKUP_MAX);
      const startedAt = clock.current;
      chunk.forEach((id) => inflight.current.add(id));
      try {
        const items = await aliasApi.lookup(chunk);
        if (!alive.current) return;
        const found = new Map(items.map((item) => [item.client_id, item.alias]));
        const patch: Aliases = {};
        for (const id of chunk) {
          // Something newer (an event, an edit) already answered for this client.
          if ((touched.current.get(id) ?? 0) > startedAt) continue;
          patch[id] = found.get(id) ?? null;
        }
        commit(patch);
      } catch (err) {
        // Nicknames are a convenience: the real name stays on screen.
        logger.debug("alias lookup failed", { code: String((err as { code?: unknown })?.code ?? "error") });
      } finally {
        chunk.forEach((id) => inflight.current.delete(id));
      }
    }
  }, [commit]);

  const request = React.useCallback(
    (clientIds: readonly (string | undefined)[]) => {
      if (!enabled) return;
      let added = false;
      for (const id of clientIds) {
        if (!id || id in cache.current || queued.current.has(id) || inflight.current.has(id)) continue;
        queued.current.add(id);
        added = true;
      }
      if (added && !timer.current) timer.current = setTimeout(() => void flush(), ALIAS_BATCH_MS);
    },
    [enabled, flush],
  );

  const resolve = React.useCallback(
    (clientId: string) =>
      new Promise<string | undefined>((done) => {
        if (!enabled) return done(undefined);
        const known = () => cache.current[clientId] ?? undefined;
        if (clientId in cache.current) return done(known());
        const timeout = setTimeout(() => done(undefined), ALIAS_RESOLVE_MS);
        const list = waiters.current.get(clientId) ?? [];
        list.push(() => {
          clearTimeout(timeout);
          done(known());
        });
        waiters.current.set(clientId, list);
        request([clientId]);
      }),
    [enabled, request],
  );

  const prime = React.useCallback(
    (entries: readonly { clientId: string | undefined; alias?: string | null }[]) => {
      if (!enabled) return;
      const patch: Aliases = {};
      for (const { clientId, alias } of entries) {
        if (!clientId || clientId in cache.current || clientId in patch) continue;
        patch[clientId] = alias ? alias : null;
        queued.current.delete(clientId);
      }
      if (Object.keys(patch).length) commit(patch);
    },
    [enabled, commit],
  );

  useRealtimeEvent<ClientAliasEvent>("client_alias.updated", (e) => {
    if (!enabled || !e?.client_id) return;
    write(e.client_id, e.alias ? e.alias : null);
  });

  // Events may have been missed: forget what is cached; names are asked for again as they are shown.
  const forget = React.useCallback(() => {
    if (!enabled) return;
    cache.current = {};
    setAliases({});
  }, [enabled]);
  useRealtimeEvent("resync_required", forget);
  useRealtimeEvent<{ reconnected?: boolean; resumed?: boolean }>("hello", (d) => {
    if (d?.reconnected && !d.resumed) forget();
  });

  const save = React.useCallback(
    async (clientId: string, next: string | null, call: () => Promise<string | null>) => {
      const previous = clientId in cache.current ? cache.current[clientId] : undefined;
      write(clientId, next);
      const mine = clock.current;
      try {
        write(clientId, await call());
      } catch (err) {
        // Roll back unless a newer write (an event) has replaced the optimistic value meanwhile.
        if (touched.current.get(clientId) === mine) {
          touched.current.set(clientId, ++clock.current);
          if (previous === undefined) {
            const { [clientId]: _removed, ...rest } = cache.current;
            void _removed;
            cache.current = rest;
            setAliases(rest);
          } else commit({ [clientId]: previous });
        }
        throw err;
      }
    },
    [write, commit],
  );

  const setAlias = React.useCallback(
    (conversationId: string, clientId: string, value: string) => {
      const alias = normalizeAlias(value);
      if (!alias) return save(clientId, null, () => aliasApi.clear(conversationId).then(() => null));
      return save(clientId, alias, () => aliasApi.set(conversationId, alias).then((r) => r.alias));
    },
    [save],
  );
  const clearAlias = React.useCallback(
    (conversationId: string, clientId: string) => save(clientId, null, () => aliasApi.clear(conversationId).then(() => null)),
    [save],
  );

  const value = React.useMemo<Ctx>(
    () => (enabled ? { enabled, aliases, request, prime, resolve, setAlias, clearAlias } : DISABLED),
    [enabled, aliases, request, prime, resolve, setAlias, clearAlias],
  );
  return <ClientAliasContext.Provider value={value}>{children}</ClientAliasContext.Provider>;
}

/** The nickname of a client for the signed-in advisor: `undefined` (none, unknown, or not an advisor). Asks for it when unknown. */
export function useClientAlias(clientId: string | null | undefined): string | undefined {
  const { aliases, request } = React.useContext(ClientAliasContext);
  const known = Boolean(clientId) && (clientId as string) in aliases;
  React.useEffect(() => {
    if (clientId && !known) request([clientId]);
  }, [clientId, known, request]);
  return clientId ? (aliases[clientId] ?? undefined) : undefined;
}

/** What an advisor sees as a client's name: the nickname when there is one, else the real name. */
export function useClientName(clientId: string | null | undefined, fallbackName: string): string {
  return useClientAlias(clientId) ?? fallbackName;
}

/**
 * Name lookup for lists and notifications: `nameFor(id, name)` gives the nickname or the
 * real name; `isKnown(id)` says whether the answer is final (always true for
 * clients and signed-out visitors). Asks for the ids given, in an effect.
 */
export function useClientNames(clientIds?: readonly (string | undefined)[]) {
  const { aliases, request, enabled } = React.useContext(ClientAliasContext);
  const key = (clientIds ?? []).join(",");
  React.useEffect(() => {
    if (key) request(key.split(","));
  }, [key, request]);
  const nameFor = React.useCallback((clientId: string | undefined, fallback: string) => (clientId ? (aliases[clientId] ?? fallback) : fallback), [aliases]);
  const isKnown = React.useCallback((clientId: string | undefined) => !enabled || !clientId || clientId in aliases, [aliases, enabled]);
  return { nameFor, isKnown };
}

/** Prime the cache with rows already loaded (inbox items, the client profile). Fill-only: never overrides a known nickname. */
export function usePrimeClientAliases(entries: readonly { clientId: string | undefined; alias?: string | null }[]) {
  const { prime } = React.useContext(ClientAliasContext);
  React.useEffect(() => {
    if (entries.length) prime(entries);
  }, [entries, prime]);
}

/** Edit a client's nickname (advisors); `enabled` is false for everyone else. `resolve`: see the context. */
export function useClientAliasActions() {
  const { enabled, setAlias, clearAlias, resolve } = React.useContext(ClientAliasContext);
  return { enabled, setAlias, clearAlias, resolve };
}
