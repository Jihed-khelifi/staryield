/**
 * Composer drafts (revisions 3.0): the text a user typed but did not send
 * survives reloads and conversation switches. Stored per user and per
 * conversation (or per advisor slug before a conversation exists) in
 * localStorage, with a 7-day TTL. Every storage access is guarded: private
 * mode, blocked storage and quota errors degrade to "no draft".
 * Image attachments are never persisted.
 */
import { MESSAGE_MAX_LENGTH } from "@/lib/schemas/message-schema";

export const DRAFT_PREFIX = "staryield.chat.draft.v1:";
export const DRAFT_TTL_MS = 7 * 24 * 60 * 60 * 1000;

type Stored = { v: 1; text: string; at: number };

export const draftKey = (userId: string, key: string) => `${DRAFT_PREFIX}${userId}:${key}`;

function storage(): Storage | null {
  try {
    return typeof window === "undefined" ? null : window.localStorage;
  } catch {
    return null;
  }
}

/** The saved draft, or "" when there is none, it expired or it is corrupt (corrupt entries are removed). */
export function loadDraft(userId: string, key: string, now: number = Date.now()): string {
  if (!userId || !key) return "";
  const store = storage();
  if (!store) return "";
  const k = draftKey(userId, key);
  try {
    const raw = store.getItem(k);
    if (!raw) return "";
    const parsed = JSON.parse(raw) as Partial<Stored> | null;
    if (!parsed || typeof parsed.text !== "string" || typeof parsed.at !== "number" || now - parsed.at > DRAFT_TTL_MS || parsed.at > now + 60_000) {
      store.removeItem(k);
      return "";
    }
    return parsed.text.slice(0, MESSAGE_MAX_LENGTH);
  } catch {
    try {
      store.removeItem(k);
    } catch {
      /* nothing more to do */
    }
    return "";
  }
}

/** Save (or, for blank text, remove) a draft. Returns whether the text is stored. */
export function saveDraft(userId: string, key: string, value: string, now: number = Date.now()): boolean {
  if (!userId || !key) return false;
  const store = storage();
  if (!store) return false;
  const k = draftKey(userId, key);
  try {
    if (!value.trim()) {
      store.removeItem(k);
      return false;
    }
    const entry: Stored = { v: 1, text: value.slice(0, MESSAGE_MAX_LENGTH), at: now };
    store.setItem(k, JSON.stringify(entry));
    return true;
  } catch {
    return false;
  }
}

export function clearDraft(userId: string, key: string): void {
  if (!userId || !key) return;
  try {
    storage()?.removeItem(draftKey(userId, key));
  } catch {
    /* ignore */
  }
}

/** Remove this user's expired drafts (cheap; run once on mount). */
export function pruneDrafts(userId: string, now: number = Date.now()): void {
  const store = storage();
  if (!store || !userId) return;
  try {
    const prefix = `${DRAFT_PREFIX}${userId}:`;
    const keys: string[] = [];
    for (let i = 0; i < store.length; i++) {
      const k = store.key(i);
      if (k?.startsWith(prefix)) keys.push(k);
    }
    for (const k of keys) loadDraft(userId, k.slice(prefix.length), now);
  } catch {
    /* ignore */
  }
}

/**
 * Remove every draft of this user (sign-out): like unsent outbox messages,
 * drafts never outlive the session on a shared device.
 */
export function clearDrafts(userId: string): void {
  const store = storage();
  if (!store || !userId) return;
  try {
    const prefix = `${DRAFT_PREFIX}${userId}:`;
    const keys: string[] = [];
    for (let i = 0; i < store.length; i++) {
      const k = store.key(i);
      if (k?.startsWith(prefix)) keys.push(k);
    }
    for (const k of keys) store.removeItem(k);
  } catch {
    /* Storage unavailable: nothing was persisted. */
  }
}
