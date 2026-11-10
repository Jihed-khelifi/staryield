// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";

import { TRUSTPILOT_BUSINESS_UNIT_ID, TRUSTPILOT_REVIEW_URL, TRUSTPILOT_SCRIPT_URL, TRUSTPILOT_TEMPLATE_ID } from "@/config/trustpilot";
import { I18nProvider } from "@/i18n/i18n-provider";
import { resetTrustpilotLoader } from "@/lib/trustpilot";
import { axeViolations, dictionary, renderWithI18n } from "@/test/render";
import { TrustpilotReviewCollector } from "@/components/reviews/trustpilot-review-collector";

const scripts = () => [...document.querySelectorAll<HTMLScriptElement>("script[data-trustpilot-bootstrap]")];
const widgets = () => screen.getAllByTestId("trustpilot-widget");

type Observer = { callback: IntersectionObserverCallback; options?: IntersectionObserverInit; target?: Element; disconnect: Mock<() => void> };
let observers: Observer[] = [];

function stubIntersectionObserver() {
  observers = [];
  class FakeObserver {
    record: Observer;
    constructor(callback: IntersectionObserverCallback, options?: IntersectionObserverInit) {
      this.record = { callback, options, disconnect: vi.fn<() => void>() };
      observers.push(this.record);
    }
    observe(target: Element) {
      this.record.target = target;
    }
    disconnect() {
      this.record.disconnect();
    }
    unobserve() {}
  }
  vi.stubGlobal("IntersectionObserver", FakeObserver);
}
const intersect = (index = 0) =>
  act(() => {
    const observer = observers[index];
    observer.callback([{ isIntersecting: true, target: observer.target } as IntersectionObserverEntry], {} as IntersectionObserver);
  });

function privacy(values: { gpc?: boolean; dnt?: string | null }) {
  Object.defineProperty(navigator, "doNotTrack", { value: values.dnt ?? null, configurable: true });
  Object.defineProperty(navigator, "globalPrivacyControl", { value: values.gpc, configurable: true });
}

beforeEach(() => {
  privacy({});
  stubIntersectionObserver();
  resetTrustpilotLoader();
  delete window.Trustpilot;
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  resetTrustpilotLoader();
  delete window.Trustpilot;
  privacy({});
});

describe("TrustpilotReviewCollector", () => {
  it("renders Trustpilot's markup with a translated fallback link and does not load anything before it is near the viewport", () => {
    renderWithI18n(<TrustpilotReviewCollector placement="home" />);
    const frame = screen.getByTestId("trustpilot-widget");
    expect(frame.getAttribute("data-placement")).toBe("home");
    expect(frame.getAttribute("data-state")).toBe("idle");
    const widget = frame.querySelector(".trustpilot-widget")!;
    expect(widget.getAttribute("data-locale")).toBe("en-US");
    expect(widget.getAttribute("data-template-id")).toBe(TRUSTPILOT_TEMPLATE_ID);
    expect(widget.getAttribute("data-businessunit-id")).toBe(TRUSTPILOT_BUSINESS_UNIT_ID);
    expect(widget.getAttribute("data-style-height")).toBe("52px");
    expect(widget.getAttribute("data-style-width")).toBe("100%");
    const link = widget.querySelector("a")!;
    expect(link.getAttribute("href")).toBe(TRUSTPILOT_REVIEW_URL);
    expect(link.getAttribute("target")).toBe("_blank");
    expect(link.getAttribute("rel")).toBe("noopener");
    expect(link.textContent).toBe("Review us on Trustpilot");
    expect(observers[0].options?.rootMargin).toBe("200px");
    expect(scripts()).toHaveLength(0);
  });

  it("uses the reader's locale for the widget", () => {
    render(
      <I18nProvider locale="fr" dictionary={dictionary}>
        <TrustpilotReviewCollector placement="reviews" />
      </I18nProvider>,
    );
    expect(document.querySelector(".trustpilot-widget")!.getAttribute("data-locale")).toBe("fr-FR");
  });

  it("loads the script when it intersects, then calls loadFromElement and shows the widget", async () => {
    renderWithI18n(<TrustpilotReviewCollector placement="reviews" />);
    await intersect();
    expect(screen.getByTestId("trustpilot-widget").getAttribute("data-state")).toBe("loading");
    expect(scripts()).toHaveLength(1);
    expect(scripts()[0].src).toBe(TRUSTPILOT_SCRIPT_URL);
    expect(observers[0].disconnect).toHaveBeenCalled();
    const loadFromElement = vi.fn();
    window.Trustpilot = { loadFromElement };
    await act(async () => {
      scripts()[0].dispatchEvent(new Event("load"));
    });
    await waitFor(() => expect(screen.getByTestId("trustpilot-widget").getAttribute("data-state")).toBe("loaded"));
    expect(loadFromElement).toHaveBeenCalledTimes(1);
    const [element, full] = loadFromElement.mock.calls[0];
    expect(element).toBe(document.querySelector(".trustpilot-widget"));
    expect(full).toBe(true);
    expect(await axeViolations()).toEqual([]);
  });

  it("loads at once when IntersectionObserver is unavailable", async () => {
    vi.stubGlobal("IntersectionObserver", undefined);
    renderWithI18n(<TrustpilotReviewCollector placement="footer" />);
    await waitFor(() => expect(scripts()).toHaveLength(1));
  });

  it.each([
    ["Global Privacy Control", { gpc: true }],
    ["Do Not Track", { dnt: "1" }],
  ])("does not load the script with %s and shows a plain link instead", async (_name, values) => {
    privacy(values);
    renderWithI18n(<TrustpilotReviewCollector placement="home" />);
    await waitFor(() => expect(screen.getByTestId("trustpilot-widget").getAttribute("data-state")).toBe("blocked"));
    // No observer is even created: nothing can trigger a load.
    expect(observers).toHaveLength(0);
    expect(scripts()).toHaveLength(0);
    expect(document.querySelector(".trustpilot-widget")).toBeNull();
    const link = screen.getByRole("link", { name: "Review us on Trustpilot (opens in a new tab)" });
    expect(link.getAttribute("href")).toBe(TRUSTPILOT_REVIEW_URL);
    expect(link.getAttribute("target")).toBe("_blank");
    expect(link.getAttribute("rel")).toBe("noopener");
    expect(link.textContent).toBe("Review us on Trustpilot");
    expect(await axeViolations()).toEqual([]);
  });

  it("falls back to the plain link when the script fails", async () => {
    renderWithI18n(<TrustpilotReviewCollector placement="chat-summary" />);
    await intersect();
    await act(async () => {
      scripts()[0].dispatchEvent(new Event("error"));
    });
    await waitFor(() => expect(screen.getByTestId("trustpilot-widget").getAttribute("data-state")).toBe("failed"));
    expect(screen.getByTestId("trustpilot-fallback").getAttribute("href")).toBe(TRUSTPILOT_REVIEW_URL);
    expect(document.querySelector(".trustpilot-widget")).toBeNull();
    expect(await axeViolations()).toEqual([]);
  });

  it("falls back when loadFromElement throws", async () => {
    window.Trustpilot = {
      loadFromElement: () => {
        throw new Error("boom");
      },
    };
    renderWithI18n(<TrustpilotReviewCollector placement="home" />);
    await intersect();
    await waitFor(() => expect(screen.getByTestId("trustpilot-widget").getAttribute("data-state")).toBe("failed"));
  });

  it("injects a single script for several widgets", async () => {
    renderWithI18n(
      <>
        <TrustpilotReviewCollector placement="home" />
        <TrustpilotReviewCollector placement="footer" />
        <TrustpilotReviewCollector placement="chat-summary" />
      </>,
    );
    expect(widgets()).toHaveLength(3);
    await intersect(0);
    await intersect(1);
    await intersect(2);
    expect(scripts()).toHaveLength(1);
    const loadFromElement = vi.fn();
    window.Trustpilot = { loadFromElement };
    await act(async () => {
      scripts()[0].dispatchEvent(new Event("load"));
    });
    await waitFor(() => expect(loadFromElement).toHaveBeenCalledTimes(3));
    expect(new Set(loadFromElement.mock.calls.map(([element]) => element)).size).toBe(3);
  });

  it("has no accessibility violations while idle", async () => {
    renderWithI18n(<TrustpilotReviewCollector placement="reviews" className="mt-3" />);
    expect(await axeViolations()).toEqual([]);
  });
});
