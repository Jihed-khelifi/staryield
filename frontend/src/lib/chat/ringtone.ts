/**
 * Incoming-request ringtone engine (docs §21.1). Framework-free: a Web Audio
 * loop with an HTMLAudioElement fallback, one-tab-rings through a Web Lock,
 * and a tiny external store (`useRingtoneState`).
 *
 * The engine never touches chat presence; it only plays a sound while
 * `setPendingRequests(n > 0)` (or `startRinging()`) is in effect.
 */
import * as React from "react";

export const RINGTONE_URL = "https://assets.staryield.net/assets/sounds/incoming-request-ef54be5299.m4a";
export const RINGTONE_FALLBACK_URL = "https://assets.staryield.net/assets/sounds/incoming-request-e7d7a4bf80.wav";
export const RINGTONE_LOCK = "staryield:ringtone";
const GAIN = 0.8;

export type RingtoneStatus = "idle" | "ringing" | "blocked" | "unavailable";
export type RingtoneState = { status: RingtoneStatus; pending: number };

type LockManagerLike = {
  request: (name: string, options: { signal?: AbortSignal }, cb: () => Promise<void>) => Promise<unknown>;
};
type AudioElementLike = {
  src: string;
  preload: string;
  loop: boolean;
  muted: boolean;
  currentTime: number;
  play: () => Promise<void> | void;
  pause: () => void;
  load?: () => void;
  canPlayType?: (type: string) => string;
};

/** Dependencies, injectable for tests. */
export type RingtoneDeps = {
  AudioContext?: (new () => AudioContext) | null;
  fetch?: typeof fetch | null;
  locks?: LockManagerLike | null;
  Audio?: (new () => AudioElementLike) | null;
};

function browserDeps(): RingtoneDeps {
  if (typeof window === "undefined") return {};
  const w = window as unknown as { AudioContext?: new () => AudioContext; webkitAudioContext?: new () => AudioContext };
  return {
    AudioContext: w.AudioContext ?? w.webkitAudioContext ?? null,
    fetch: typeof fetch === "function" ? fetch.bind(globalThis) : null,
    locks: (typeof navigator !== "undefined" && (navigator as unknown as { locks?: LockManagerLike }).locks) || null,
    Audio: typeof Audio === "function" ? (Audio as unknown as new () => AudioElementLike) : null,
  };
}

let injected: RingtoneDeps | null = null;
const deps = (): RingtoneDeps => injected ?? browserDeps();

// --- state ---------------------------------------------------------------------

let state: RingtoneState = { status: "idle", pending: 0 };
const listeners = new Set<() => void>();

let wanted = false;
let preloadStarted = false;
let preloadDone = false;
let ctx: AudioContext | null = null;
let buffer: AudioBuffer | null = null;
let media: AudioElementLike | null = null;
let mediaBlocked = false;
let mediaPrimed = false;
let source: AudioBufferSourceNode | null = null;
let gain: GainNode | null = null;
let mediaPlaying = false;
let lockRequested = false;
let lockHeld = false;
let lockRelease: (() => void) | null = null;
let lockAbort: AbortController | null = null;
let unlockInstalled = false;

function publish(next: Partial<RingtoneState>) {
  const merged = { ...state, ...next };
  if (merged.status === state.status && merged.pending === state.pending) return;
  state = merged;
  listeners.forEach((l) => l());
}

export function subscribeRingtone(l: () => void): () => void {
  listeners.add(l);
  return () => listeners.delete(l);
}
export const getRingtoneState = (): RingtoneState => state;
const serverState: RingtoneState = { status: "idle", pending: 0 };

/** Hook over the external store. */
export function useRingtoneState(): RingtoneState {
  return React.useSyncExternalStore(subscribeRingtone, getRingtoneState, () => serverState);
}

// --- audio helpers -------------------------------------------------------------

function audioContext(): AudioContext | null {
  if (ctx) return ctx;
  const Ctor = deps().AudioContext;
  if (!Ctor) return null;
  try {
    ctx = new Ctor();
    ctx.onstatechange = () => evaluate();
  } catch {
    ctx = null;
  }
  return ctx;
}

function makeMedia(): AudioElementLike | null {
  if (media) return media;
  const Ctor = deps().Audio;
  if (!Ctor) return null;
  try {
    const el = new Ctor();
    el.preload = "auto";
    el.loop = true;
    // Browsers without AAC (some Chromium builds) get the WAV.
    const aac = typeof el.canPlayType === "function" ? el.canPlayType('audio/mp4; codecs="mp4a.40.2"') : "maybe";
    el.src = aac ? RINGTONE_URL : RINGTONE_FALLBACK_URL;
    el.load?.();
    media = el;
  } catch {
    media = null;
  }
  return media;
}

async function decode(url: string): Promise<AudioBuffer | null> {
  const f = deps().fetch;
  const c = audioContext();
  if (!f || !c) return null;
  try {
    const res = await f(url, { mode: "cors", credentials: "omit" });
    if (!res.ok) return null;
    const bytes = await res.arrayBuffer();
    return await c.decodeAudioData(bytes);
  } catch {
    return null;
  }
}

/** Fetch and decode the ringtone once per page load. Idempotent, never throws. */
export function preloadRingtone(): Promise<void> {
  if (typeof window === "undefined") return Promise.resolve();
  installUnlockListeners();
  if (preloadStarted) return preloadPromise;
  preloadStarted = true;
  preloadPromise = (async () => {
    try {
      if (audioContext()) {
        buffer = (await decode(RINGTONE_URL)) ?? (await decode(RINGTONE_FALLBACK_URL));
      }
      if (!buffer) makeMedia();
    } catch {
      /* fall through to unavailable */
    }
    preloadDone = true;
    evaluate();
  })();
  return preloadPromise;
}
let preloadPromise: Promise<void> = Promise.resolve();

const usable = () => Boolean(buffer || media);
function unlocked(): boolean {
  if (buffer && ctx) return ctx.state === "running";
  return !mediaBlocked;
}

function setAudioSession(type: "playback" | "auto") {
  try {
    const session = (navigator as unknown as { audioSession?: { type: string } }).audioSession;
    if (session) session.type = type;
  } catch {
    /* unsupported */
  }
}

function playNow() {
  if (buffer && ctx) {
    if (source) return;
    const src = ctx.createBufferSource();
    const g = ctx.createGain();
    src.buffer = buffer;
    src.loop = true;
    g.gain.value = GAIN;
    src.connect(g);
    g.connect(ctx.destination);
    src.start(0);
    source = src;
    gain = g;
    setAudioSession("playback");
  } else if (media && !mediaPlaying) {
    mediaPlaying = true;
    media.muted = false;
    media.loop = true;
    setAudioSession("playback");
    try {
      const p = media.play();
      if (p && typeof p.then === "function") {
        p.then(
          () => {
            mediaPrimed = true;
          },
          () => {
            mediaPlaying = false;
            mediaBlocked = true;
            evaluate();
          },
        );
      }
    } catch {
      mediaPlaying = false;
      mediaBlocked = true;
    }
  }
}

function stopNow() {
  if (source) {
    try {
      source.stop();
    } catch {
      /* already stopped */
    }
    source.disconnect();
    gain?.disconnect();
    source = null;
    gain = null;
  }
  if (media && mediaPlaying) {
    mediaPlaying = false;
    try {
      media.pause();
      media.currentTime = 0;
    } catch {
      /* ignore */
    }
  }
  setAudioSession("auto");
}

function acquireLock() {
  const locks = deps().locks;
  if (!locks || lockRequested) return;
  lockRequested = true;
  lockAbort = typeof AbortController === "function" ? new AbortController() : null;
  const done = () => {
    lockRequested = false;
    lockHeld = false;
    lockRelease = null;
    lockAbort = null;
    evaluate();
  };
  try {
    void Promise.resolve(
      locks.request(RINGTONE_LOCK, { signal: lockAbort?.signal }, () => {
        if (!wanted) return Promise.resolve();
        return new Promise<void>((resolve) => {
          lockHeld = true;
          lockRelease = resolve;
          evaluate();
        });
      }),
    ).then(done, done);
  } catch {
    lockRequested = false;
  }
}

function releaseLock() {
  if (lockRelease) lockRelease();
  else if (lockRequested) lockAbort?.abort();
}

/** Reconcile the wanted state with what is playing. */
function evaluate() {
  if (!wanted) {
    stopNow();
    releaseLock();
    publish({ status: "idle" });
    return;
  }
  if (preloadDone && !usable()) {
    publish({ status: "unavailable" });
    return;
  }
  if (!preloadDone) {
    publish({ status: unlockedGuess() ? "ringing" : "blocked" });
    return;
  }
  if (!unlocked()) {
    stopNow();
    releaseLock();
    publish({ status: "blocked" });
    // The browser may allow it without a gesture (autoplay policy flag, engaged site).
    if (ctx && ctx.state === "suspended" && !resumeTried) {
      resumeTried = true;
      void ctx.resume().then(evaluate, () => undefined);
    }
    return;
  }
  publish({ status: "ringing" });
  const locks = deps().locks;
  if (!locks) {
    playNow();
    return;
  }
  acquireLock();
  if (lockHeld) playNow();
  else stopNow();
}
let resumeTried = false;
const unlockedGuess = () => (ctx ? ctx.state === "running" : !mediaBlocked);

export function startRinging() {
  if (wanted) return;
  wanted = true;
  resumeTried = false;
  evaluate();
}

export function stopRinging() {
  if (!wanted) return;
  wanted = false;
  evaluate();
}

/** Ring while `count > 0`; the count is exposed through the store. */
export function setPendingRequests(count: number) {
  const n = Math.max(0, Math.floor(count));
  state = { ...state, pending: n };
  if (n > 0) startRinging();
  else stopRinging();
  listeners.forEach((l) => l());
}

/** Resume the audio context / prime the media element (call from a user gesture). */
export function unlockRingtone(): Promise<void> {
  const tasks: Promise<unknown>[] = [];
  if (ctx && ctx.state !== "running") tasks.push(ctx.resume().catch(() => undefined));
  if (!buffer && media) {
    mediaBlocked = false;
    if (!wanted) {
      // Prime: a muted play inside the gesture lets later plays through.
      try {
        media.muted = true;
        const p = media.play();
        tasks.push(
          Promise.resolve(p)
            .then(() => {
              mediaPrimed = true;
              media?.pause();
              if (media) media.muted = false;
            })
            .catch(() => {
              mediaBlocked = true;
            }),
        );
      } catch {
        mediaBlocked = true;
      }
    }
  }
  return Promise.all(tasks).then(() => evaluate());
}

/**
 * Whether a gesture has something to unlock. The listeners stay installed for
 * the page's lifetime: iOS can suspend or interrupt a running context later
 * (calls, other audio), and the next tap must be able to bring it back.
 */
function needsUnlock(): boolean {
  if (ctx && ctx.state !== "running") return true;
  return Boolean(!buffer && media && (mediaBlocked || !mediaPrimed));
}

const GESTURES = ["pointerdown", "keydown", "touchend"] as const;
const onGesture = () => {
  if (needsUnlock()) void unlockRingtone();
};
function installUnlockListeners() {
  if (unlockInstalled || typeof document === "undefined") return;
  unlockInstalled = true;
  GESTURES.forEach((t) => document.addEventListener(t, onGesture, { capture: true, passive: true }));
}
function removeUnlockListeners() {
  if (!unlockInstalled || typeof document === "undefined") return;
  unlockInstalled = false;
  GESTURES.forEach((t) => document.removeEventListener(t, onGesture, { capture: true }));
}

// --- tests ---------------------------------------------------------------------

/** Replace the browser dependencies (tests). */
export function configureRingtone(next: RingtoneDeps | null) {
  injected = next;
}

/** Reset every piece of module state (tests). */
export function resetRingtone() {
  stopNow();
  removeUnlockListeners();
  if (lockRelease) lockRelease();
  wanted = false;
  preloadStarted = false;
  preloadDone = false;
  preloadPromise = Promise.resolve();
  if (ctx) ctx.onstatechange = null;
  ctx = null;
  buffer = null;
  media = null;
  mediaBlocked = false;
  mediaPrimed = false;
  mediaPlaying = false;
  lockRequested = false;
  lockHeld = false;
  lockRelease = null;
  lockAbort = null;
  resumeTried = false;
  state = { status: "idle", pending: 0 };
  listeners.forEach((l) => l());
}
