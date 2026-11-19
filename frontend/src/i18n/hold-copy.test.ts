import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

import { HOLD_REASON_COPY } from "@/lib/chat/hold";
import { endReasonCopy } from "@/components/chat/chat-model";

const read = (locale: string) => JSON.parse(readFileSync(new URL(`./dictionaries/copy-${locale}.json`, import.meta.url), "utf8")) as Record<string, string>;
const en = read("en");
const fr = read("fr");
const es = read("es");
const placeholders = (value: string) => [...value.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort();

/** Every English string of the free-trial hold (§26.1): contract copy first, then the rest of the web UI. */
const HOLD_COPY = [
  "Hidden · Top up to read",
  "Hidden message from {value0}. Top up to read it.",
  "Hidden message from {value0}",
  "Hidden message",
  "Hidden from client",
  "Not revealed",
  "Reading on hold",
  "Time left",
  "your advisor",
  "Your free minutes are over.",
  "Your credits ran out.",
  "Top up now to keep chatting with {value0} and read their replies. Continuing costs {value1} credits per minute.",
  "{value0}'s free minutes are over. Your replies are hidden from them until they top up.",
  "{value0}'s credits ran out. Your replies are hidden from them until they top up.",
  "Top up",
  "Continue reading ({value0} credits)",
  "Reveal hidden messages",
  "Reveal 1 hidden message from {value0} for {value1} credits?",
  "Reveal {value0} hidden messages from {value1} for {value2} credits?",
  "Reveal for {value0} credits",
  "Top up to read 1 hidden message",
  "Top up to read {value0} hidden messages",
  "This reading just changed. Please try again.",
  "The hidden messages from {value0} are now readable.",
  "Your reading with {value0} continues.",
  "Your reading is on hold. Top up within {value0} seconds to keep chatting with {value1}.",
  "{value0}'s reading is on hold. Your replies are hidden from them until they top up.",
  "{value0} topped up. Your replies are visible to them now.",
  "The reading with {value0} ended before a top-up arrived.",
  "The reading with {value0} ended before they topped up. Your replies were not revealed.",
  "The chat ended because no top-up arrived in time. No credits were used.",
  "Top up to read this message first.",
];

describe("free-trial hold copy (§26.1)", () => {
  it.each(HOLD_COPY)("%s exists in English, French and Spanish with the same placeholders", (key) => {
    for (const [name, dictionary] of [["en", en], ["fr", fr], ["es", es]] as const) {
      expect(dictionary[key], `${name}: ${key}`).toBeTruthy();
      expect(placeholders(dictionary[key]), `${name} placeholders: ${key}`).toEqual(placeholders(key));
    }
  });

  it("is translated, not copied, in French and Spanish (apart from words that read the same)", () => {
    const sameInEveryLanguage = new Set(["Not revealed"]);
    for (const key of HOLD_COPY.filter((k) => !sameInEveryLanguage.has(k))) {
      expect(fr[key], `fr: ${key}`).not.toBe(key);
      expect(es[key], `es: ${key}`).not.toBe(key);
    }
  });

  it("covers the reason lines and the new end reason that are looked up dynamically", () => {
    for (const line of Object.values(HOLD_REASON_COPY)) expect(HOLD_COPY).toContain(line);
    expect(HOLD_COPY).toContain(endReasonCopy.hold_expired);
  });
});
