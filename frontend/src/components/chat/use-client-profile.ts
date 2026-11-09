"use client";

import * as React from "react";

import { chatApi } from "@/lib/api/endpoints";
import { toApiError, type ApiError } from "@/lib/api/errors";
import type { ClientProfile } from "@/lib/api/types";

type Entry = { data: ClientProfile | null; error: ApiError | null; at: number };

/** A cached profile is reused for this long (the header and the panel mount at different times). */
export const CLIENT_PROFILE_TTL_MS = 5 * 60 * 1000;

const cache = new Map<string, Entry>();
const inflight = new Map<string, Promise<void>>();
const listeners = new Set<() => void>();
const EMPTY: Entry = { data: null, error: null, at: 0 };

const emit = () => listeners.forEach((l) => l());

function load(conversationId: string, force: boolean) {
  const current = cache.get(conversationId);
  if (inflight.has(conversationId)) return;
  if (!force && current && !current.error && Date.now() - current.at < CLIENT_PROFILE_TTL_MS) return;
  const request = chatApi
    .clientProfile(conversationId)
    .then(
      (data) => {
        cache.set(conversationId, { data, error: null, at: Date.now() });
      },
      // A failed refresh keeps what was shown before.
      (err) => {
        cache.set(conversationId, { data: current?.data ?? null, error: toApiError(err), at: Date.now() });
      },
    )
    .finally(() => {
      inflight.delete(conversationId);
      emit();
    });
  inflight.set(conversationId, request);
  emit();
}

/** Forget everything (sign-out, tests). */
export function clearClientProfileCache() {
  cache.clear();
  inflight.clear();
  emit();
}

export type ClientProfileState = {
  profile: ClientProfile | null;
  /** The first answer is pending (a refresh with data on screen does not count). */
  loading: boolean;
  error: ApiError | null;
  reload: () => void;
};

/**
 * What an advisor sees about the client of a conversation (§9.3), shared by the
 * chat header and the client panel: one request per conversation (concurrent
 * callers wait for the same one), cached for a few minutes, `reload` forces a
 * new fetch.
 */
export function useClientProfile(conversationId: string | null): ClientProfileState {
  const subscribe = React.useCallback((listener: () => void) => {
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  }, []);
  const entry = React.useSyncExternalStore(
    subscribe,
    () => (conversationId ? (cache.get(conversationId) ?? EMPTY) : EMPTY),
    () => EMPTY,
  );
  React.useEffect(() => {
    if (conversationId) load(conversationId, false);
  }, [conversationId]);
  const reload = React.useCallback(() => {
    if (conversationId) load(conversationId, true);
  }, [conversationId]);

  const profile = entry.data && entry.data.id ? entry.data : null;
  return {
    profile,
    loading: Boolean(conversationId) && !profile && !entry.error,
    error: profile ? null : entry.error,
    reload,
  };
}
