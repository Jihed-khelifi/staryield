/**
 * A controllable EventSource for unit tests: install it, open the code under
 * test, then drive the stream with `emit`, `fail` and `readyState`.
 */
import { vi } from "vitest";

export class FakeEventSource {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSED = 2;
  static instances: FakeEventSource[] = [];
  static get last(): FakeEventSource {
    return FakeEventSource.instances[FakeEventSource.instances.length - 1];
  }
  readyState = 0;
  closed = false;
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(
    public url: string,
    public init?: { withCredentials?: boolean },
  ) {
    FakeEventSource.instances.push(this);
  }
  close() {
    this.closed = true;
    this.readyState = 2;
  }
  /** A server frame. */
  emit(payload: unknown) {
    this.readyState = 1;
    this.onmessage?.({ data: typeof payload === "string" ? payload : JSON.stringify(payload) });
  }
  /** `closed`: the browser gave up (refused); otherwise it is reconnecting by itself. */
  fail(closed = false) {
    this.readyState = closed ? 2 : 0;
    this.onerror?.();
  }
}

export function installFakeEventSource() {
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
}
