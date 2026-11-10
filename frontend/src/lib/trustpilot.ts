/**
 * Single-flight loader for Trustpilot's widget bootstrap script. Many widgets
 * can ask for it at once (page body, footer, chat summary); the script is
 * injected once, never during server rendering, and the promise settles when
 * `window.Trustpilot` exists, on a script error or after `TIMEOUT_MS`.
 */
import { TRUSTPILOT_SCRIPT_URL } from "@/config/trustpilot";

export type TrustpilotGlobal = { loadFromElement: (element: HTMLElement, full?: boolean) => void };

declare global {
  interface Window {
    Trustpilot?: TrustpilotGlobal;
  }
}

export const TRUSTPILOT_TIMEOUT_MS = 8_000;
const SCRIPT_ATTRIBUTE = "data-trustpilot-bootstrap";

let pending: Promise<TrustpilotGlobal> | null = null;
let cleanup: (() => void) | null = null;

/** Forget any in-flight load and remove the script tag (tests, retries after failure). */
export function resetTrustpilotLoader() {
  cleanup?.();
  cleanup = null;
  pending = null;
  if (typeof document !== "undefined") document.querySelectorAll(`script[${SCRIPT_ATTRIBUTE}]`).forEach((node) => node.remove());
}

export function loadTrustpilot(): Promise<TrustpilotGlobal> {
  if (typeof window === "undefined" || typeof document === "undefined") return Promise.reject(new Error("trustpilot: no browser"));
  if (window.Trustpilot) return Promise.resolve(window.Trustpilot);
  if (pending) return pending;

  const attempt = new Promise<TrustpilotGlobal>((resolve, reject) => {
    const script = document.createElement("script");
    let poll: ReturnType<typeof setInterval> | undefined;
    const timer = setTimeout(() => fail("trustpilot: timed out"), TRUSTPILOT_TIMEOUT_MS);
    const stop = () => {
      clearInterval(poll);
      clearTimeout(timer);
      script.removeEventListener("load", onLoad);
      script.removeEventListener("error", onError);
    };
    const fail = (message: string) => {
      stop();
      script.remove();
      // A later widget may try again; concurrent callers already share this attempt.
      if (pending === attempt) pending = null;
      cleanup = null;
      reject(new Error(message));
    };
    const settle = () => {
      if (!window.Trustpilot) return false;
      stop();
      cleanup = null;
      resolve(window.Trustpilot);
      return true;
    };
    function onLoad() {
      // The bootstrap defines window.Trustpilot as it runs; poll briefly if it is a tick late.
      if (!settle()) poll = setInterval(settle, 50);
    }
    function onError() {
      fail("trustpilot: script failed to load");
    }
    cleanup = () => {
      stop();
    };
    script.type = "text/javascript";
    script.src = TRUSTPILOT_SCRIPT_URL;
    script.async = true;
    script.setAttribute(SCRIPT_ATTRIBUTE, "");
    script.addEventListener("load", onLoad);
    script.addEventListener("error", onError);
    document.head.appendChild(script);
  });
  pending = attempt;
  return attempt;
}
