/**
 * Test helpers for components: render inside the real I18nProvider with the
 * English dictionaries, and run axe-core accessibility checks in jsdom.
 */
import * as React from "react";
import { render, type RenderOptions } from "@testing-library/react";
import axe from "axe-core";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import { TooltipProvider } from "@/components/ui/tooltip";

export const dictionary = { ...en, copy } as Dictionary;

export function renderWithI18n(ui: React.ReactElement, options?: Omit<RenderOptions, "wrapper">) {
  return render(ui, {
    wrapper: ({ children }) => (
      <I18nProvider locale="en" dictionary={dictionary}>
        {/* Like the [lang] layout: tooltips need a provider. */}
        <TooltipProvider>{children}</TooltipProvider>
      </I18nProvider>
    ),
    ...options,
  });
}

/**
 * Serious/critical axe violations inside `root`. Colour contrast needs real
 * layout, so it is checked in Playwright instead.
 */
export async function axeViolations(root: Element = document.body) {
  const results = await axe.run(root, {
    resultTypes: ["violations"],
    rules: { "color-contrast": { enabled: false }, region: { enabled: false } },
  });
  return results.violations
    .filter((violation) => violation.impact === "serious" || violation.impact === "critical")
    .map((violation) => `${violation.id}: ${violation.help} (${violation.nodes.map((node) => node.target.join(" ")).join(", ")})`);
}

/** jsdom lacks ResizeObserver, which Radix popper/size utilities expect. */
export function stubResizeObserver() {
  if (typeof globalThis.ResizeObserver !== "undefined") return;
  class ResizeObserverStub {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
  globalThis.ResizeObserver = ResizeObserverStub as unknown as typeof ResizeObserver;
}
