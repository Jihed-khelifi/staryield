// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import en from "@/i18n/dictionaries/en.json";
import copy from "@/i18n/dictionaries/copy-en.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import { ComposerNote } from "@/components/chat/composer-note";

const dictionary = { ...en, copy } as Dictionary;
const rate = { rate: 5, listRate: 5, discounted: false, discountPct: 0 };
function setup(p: Partial<React.ComponentProps<typeof ComposerNote>>) {
  return render(
    <I18nProvider locale="en" dictionary={dictionary}>
      <p data-testid="note">
        <ComposerNote role="client" free={false} freeMinutes={0} balance={1200} rate={rate} advisorLabel="Message" {...p} />
      </p>
    </I18nProvider>,
  );
}
const note = () => screen.getByTestId("note").textContent;

afterEach(() => cleanup());

describe("ComposerNote", () => {
  it("clients see the rate and balance", () => {
    setup({});
    expect(note()).toContain("5 credits/min");
    expect(note()).toContain("balance 1,200");
  });
  it("clients with free minutes see them first", () => {
    setup({ freeMinutes: 3 });
    expect(note()).toContain("3 free minutes, then");
    expect(note()).toContain("5 credits/min");
  });
  it("free mode says Staryield covers it, with no rate or balance", () => {
    setup({ free: true, freeMinutes: 3 });
    expect(note()).toBe("This chat is free. Staryield covers it.");
    expect(note()).not.toContain("credits");
    expect(note()).not.toContain("balance");
  });
  it("advisors keep their label, free or not", () => {
    setup({ role: "psychic", free: true, advisorLabel: "Message" });
    expect(note()).toBe("Message");
  });
  it("outside a live chat, clients see it is free and how many messages are left today (§22.3)", () => {
    setup({ offSession: { live: false, enabled: true, eligible: true, unlimited: false, limit: 20, used: 3, remaining: 17 } });
    expect(note()).toBe("Outside a live chat: free · 17 of 20 left today");
    expect(note()).not.toContain("credits");
  });
  it("outside a live chat, advisors see it is not billed", () => {
    setup({ role: "psychic", offSession: { live: false, enabled: true, eligible: true, unlimited: false, limit: 20, used: 20, remaining: 0 } });
    expect(note()).toBe("Outside a live chat: not billed · 0 of 20 left today");
  });
  it("in free mode there is no count", () => {
    setup({ offSession: { live: false, enabled: true, eligible: true, unlimited: true, limit: 20, used: 3, remaining: 20 } });
    expect(note()).toBe("Outside a live chat: free · unlimited during free mode");
  });
});
