// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  RINGTONE_FALLBACK_URL,
  RINGTONE_LOCK,
  RINGTONE_URL,
  configureRingtone,
  getRingtoneState,
  preloadRingtone,
  resetRingtone,
  setPendingRequests,
  startRinging,
  stopRinging,
  unlockRingtone,
} from "@/lib/chat/ringtone";

class FakeSource {
  buffer: unknown = null;
  loop = false;
  started = 0;
  stopped = 0;
  disconnected = 0;
  connect = vi.fn();
  start() {
    this.started++;
  }
  stop() {
    this.stopped++;
  }
  disconnect() {
    this.disconnected++;
  }
}

function makeContext(initial: "running" | "suspended", decodeFails = 0) {
  const sources: FakeSource[] = [];
  let failures = decodeFails;
  class FakeContext {
    static last: FakeContext;
    state = initial;
    destination = {};
    onstatechange: (() => void) | null = null;
    constructor() {
      FakeContext.last = this;
    }
    resume() {
      this.state = "running";
      this.onstatechange?.();
      return Promise.resolve();
    }
    decodeAudioData() {
      if (failures > 0) {
        failures--;
        return Promise.reject(new Error("decode"));
      }
      return Promise.resolve({ duration: 14.9 });
    }
    createBufferSource() {
      const s = new FakeSource();
      sources.push(s);
      return s;
    }
    createGain() {
      return { gain: { value: 0 }, connect: vi.fn(), disconnect: vi.fn() };
    }
  }
  return { Ctor: FakeContext as unknown as new () => AudioContext, sources, ctx: () => FakeContext.last };
}

const okFetch = () => vi.fn().mockResolvedValue({ ok: true, arrayBuffer: () => Promise.resolve(new ArrayBuffer(8)) });

class FakeAudio {
  static instances: FakeAudio[] = [];
  src = "";
  preload = "";
  loop = false;
  muted = false;
  currentTime = 0;
  plays = 0;
  pauses = 0;
  rejectPlay = false;
  constructor() {
    FakeAudio.instances.push(this);
  }
  play() {
    this.plays++;
    return this.rejectPlay ? Promise.reject(new Error("blocked")) : Promise.resolve();
  }
  pause() {
    this.pauses++;
  }
}

const flush = () => new Promise((r) => setTimeout(r, 0));

beforeEach(() => {
  resetRingtone();
  FakeAudio.instances = [];
});
afterEach(() => {
  configureRingtone(null);
  resetRingtone();
});

describe("preload", () => {
  it("fetches with CORS and no credentials, once", async () => {
    const f = okFetch();
    configureRingtone({ AudioContext: makeContext("running").Ctor, fetch: f as never, locks: null, Audio: FakeAudio });
    await Promise.all([preloadRingtone(), preloadRingtone()]);
    await preloadRingtone();
    expect(f).toHaveBeenCalledTimes(1);
    expect(f).toHaveBeenCalledWith(RINGTONE_URL, { mode: "cors", credentials: "omit" });
    expect(FakeAudio.instances).toHaveLength(0);
  });

  it("falls back to the WAV when the m4a cannot be decoded", async () => {
    const f = okFetch();
    configureRingtone({ AudioContext: makeContext("running", 1).Ctor, fetch: f as never, locks: null, Audio: FakeAudio });
    await preloadRingtone();
    expect(f.mock.calls.map((c) => c[0])).toEqual([RINGTONE_URL, RINGTONE_FALLBACK_URL]);
    expect(FakeAudio.instances).toHaveLength(0);
  });

  it("uses a looping preloaded media element when both decodes fail", async () => {
    configureRingtone({ AudioContext: makeContext("running", 2).Ctor, fetch: okFetch() as never, locks: null, Audio: FakeAudio });
    await preloadRingtone();
    const el = FakeAudio.instances[0];
    expect(el.preload).toBe("auto");
    expect(el.loop).toBe(true);
    startRinging();
    expect(el.plays).toBe(1);
    expect(getRingtoneState().status).toBe("ringing");
    stopRinging();
    expect(el.pauses).toBe(1);
    expect(getRingtoneState().status).toBe("idle");
  });

  it("uses the media element without Web Audio, and reports blocked when play is rejected", async () => {
    configureRingtone({ AudioContext: null, fetch: okFetch() as never, locks: null, Audio: FakeAudio });
    await preloadRingtone();
    const el = FakeAudio.instances[0];
    el.rejectPlay = true;
    startRinging();
    await flush();
    expect(getRingtoneState().status).toBe("blocked");
    el.rejectPlay = false;
    await unlockRingtone();
    expect(getRingtoneState().status).toBe("ringing");
    expect(el.plays).toBe(2);
  });

  it("gives the media element the WAV when the browser cannot play AAC", async () => {
    class NoAacAudio extends FakeAudio {
      canPlayType = () => "";
    }
    configureRingtone({ AudioContext: null, fetch: null, locks: null, Audio: NoAacAudio });
    await preloadRingtone();
    expect(FakeAudio.instances[0].src).toBe(RINGTONE_FALLBACK_URL);
  });

  it("is unavailable when nothing can play, and never throws", async () => {
    configureRingtone({ AudioContext: null, fetch: null, locks: null, Audio: null });
    await expect(preloadRingtone()).resolves.toBeUndefined();
    startRinging();
    expect(getRingtoneState().status).toBe("unavailable");
  });
});

describe("ringing", () => {
  it("loops the buffer and stops idempotently, disconnecting the nodes", async () => {
    const c = makeContext("running");
    configureRingtone({ AudioContext: c.Ctor, fetch: okFetch() as never, locks: null, Audio: FakeAudio });
    await preloadRingtone();
    startRinging();
    startRinging();
    expect(c.sources).toHaveLength(1);
    expect(c.sources[0].loop).toBe(true);
    expect(c.sources[0].started).toBe(1);
    expect(getRingtoneState().status).toBe("ringing");
    stopRinging();
    stopRinging();
    expect(c.sources[0].stopped).toBe(1);
    expect(c.sources[0].disconnected).toBe(1);
    expect(getRingtoneState().status).toBe("idle");
  });

  it("is blocked while the context is suspended, then rings as soon as it is unlocked", async () => {
    const c = makeContext("suspended");
    configureRingtone({ AudioContext: c.Ctor, fetch: okFetch() as never, locks: null, Audio: FakeAudio });
    // resume() succeeds only from a gesture in a real browser
    const ctx = () => c.ctx();
    await preloadRingtone();
    const resume = vi.spyOn(ctx(), "resume").mockImplementation(() => Promise.resolve());
    startRinging();
    expect(getRingtoneState().status).toBe("blocked");
    expect(c.sources).toHaveLength(0);
    resume.mockRestore();
    await unlockRingtone();
    expect(getRingtoneState().status).toBe("ringing");
    expect(c.sources).toHaveLength(1);
  });

  it("unlocks from the first global gesture", async () => {
    const c = makeContext("suspended");
    configureRingtone({ AudioContext: c.Ctor, fetch: okFetch() as never, locks: null, Audio: FakeAudio });
    await preloadRingtone();
    vi.spyOn(c.ctx(), "resume").mockImplementationOnce(() => Promise.resolve());
    startRinging();
    expect(getRingtoneState().status).toBe("blocked");
    document.dispatchEvent(new Event("pointerdown"));
    await flush();
    expect(getRingtoneState().status).toBe("ringing");
  });

  it("keeps listening for gestures: a later suspension (iOS interruption) is recovered by the next tap", async () => {
    const c = makeContext("running");
    configureRingtone({ AudioContext: c.Ctor, fetch: okFetch() as never, locks: null, Audio: FakeAudio });
    await preloadRingtone();
    document.dispatchEvent(new Event("pointerdown"));
    await flush();
    startRinging();
    expect(getRingtoneState().status).toBe("ringing");
    // The system interrupts the context and the automatic resume is refused.
    vi.spyOn(c.ctx(), "resume").mockImplementationOnce(() => Promise.resolve());
    (c.ctx() as unknown as { state: string }).state = "suspended";
    (c.ctx().onstatechange as (() => void) | null)?.();
    expect(getRingtoneState().status).toBe("blocked");
    expect(c.sources[0].stopped).toBe(1);
    document.dispatchEvent(new Event("pointerdown"));
    await flush();
    expect(getRingtoneState().status).toBe("ringing");
    expect(c.sources).toHaveLength(2);
  });

  it("tracks the pending count and rings only while it is positive", async () => {
    const c = makeContext("running");
    configureRingtone({ AudioContext: c.Ctor, fetch: okFetch() as never, locks: null, Audio: FakeAudio });
    await preloadRingtone();
    setPendingRequests(2);
    expect(getRingtoneState()).toEqual({ status: "ringing", pending: 2 });
    setPendingRequests(1);
    expect(c.sources).toHaveLength(1);
    setPendingRequests(0);
    expect(getRingtoneState()).toEqual({ status: "idle", pending: 0 });
  });

  it("sets the iOS audio session to playback while ringing", async () => {
    const session = { type: "auto" };
    Object.defineProperty(navigator, "audioSession", { configurable: true, value: session });
    configureRingtone({ AudioContext: makeContext("running").Ctor, fetch: okFetch() as never, locks: null, Audio: FakeAudio });
    await preloadRingtone();
    startRinging();
    expect(session.type).toBe("playback");
    stopRinging();
    expect(session.type).toBe("auto");
    delete (navigator as unknown as { audioSession?: unknown }).audioSession;
  });
});

describe("one tab rings", () => {
  function fakeLocks() {
    const grants: Array<() => Promise<void>> = [];
    const request = vi.fn((_name: string, _o: unknown, cb: () => Promise<void>) => {
      grants.push(cb);
      return new Promise<void>(() => undefined);
    });
    return { request, grants };
  }

  it("plays only after the lock is granted and releases it when stopped", async () => {
    const c = makeContext("running");
    const locks = fakeLocks();
    configureRingtone({ AudioContext: c.Ctor, fetch: okFetch() as never, locks: locks as never, Audio: FakeAudio });
    await preloadRingtone();
    startRinging();
    expect(locks.request).toHaveBeenCalledTimes(1);
    expect(locks.request.mock.calls[0][0]).toBe(RINGTONE_LOCK);
    expect(c.sources).toHaveLength(0);

    let released = false;
    const held = locks.grants[0]().then(() => {
      released = true;
    });
    await flush();
    expect(c.sources).toHaveLength(1);
    stopRinging();
    await held;
    expect(released).toBe(true);
    expect(c.sources[0].stopped).toBe(1);
  });

  it("does not request the lock while audio is locked", async () => {
    const c = makeContext("suspended");
    const locks = fakeLocks();
    configureRingtone({ AudioContext: c.Ctor, fetch: okFetch() as never, locks: locks as never, Audio: FakeAudio });
    await preloadRingtone();
    vi.spyOn(c.ctx(), "resume").mockImplementation(() => Promise.resolve());
    startRinging();
    expect(getRingtoneState().status).toBe("blocked");
    expect(locks.request).not.toHaveBeenCalled();
  });

  it("rings in every tab without navigator.locks", async () => {
    const c = makeContext("running");
    configureRingtone({ AudioContext: c.Ctor, fetch: okFetch() as never, locks: null, Audio: FakeAudio });
    await preloadRingtone();
    startRinging();
    expect(c.sources).toHaveLength(1);
  });
});
