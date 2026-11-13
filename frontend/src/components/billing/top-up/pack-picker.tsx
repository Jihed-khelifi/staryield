"use client";

import * as React from "react";
import { Circle, CircleDot } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import type { PricingPack, RateRange } from "@/lib/api/pricing";
import { isIntroPack, minutesRange, packEyebrow, standardPacks } from "@/lib/billing/top-up";
import { formatPackPrice } from "@/lib/pricing";

/**
 * One pack as a radio card (Figma 1352:2879…2911): the state is a drawn radio
 * plus the words "Selected" / "Select pack", so colour is never the only cue.
 * Choosing never charges; the payer confirms in the dock below.
 */
function PackCard({ pack, range, selected, onSelect }: { pack: PricingPack; range: RateRange; selected: boolean; onSelect: (pack: PricingPack) => void }) {
  const { text, locale } = useI18n();
  const id = React.useId();
  const intro = isIntroPack(pack);
  const Indicator = selected ? CircleDot : Circle;
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      // One tab stop for the group: the selected card (roving tabindex).
      tabIndex={selected ? 0 : -1}
      data-state={selected ? "checked" : "unchecked"}
      data-pack-id={pack.id}
      onClick={() => onSelect(pack)}
      className="top-up-pack"
      data-intro={intro || undefined}
      data-testid={`top-up-pack-${pack.code}`}
      aria-labelledby={`${id}-name ${id}-credits ${id}-price`}
      aria-describedby={`${id}-eyebrow ${id}-minutes ${id}-note`}
    >
      <span className="top-up-pack-state">
        <Indicator aria-hidden size={16} />
        {selected ? text("Selected") : text("Select pack")}
      </span>
      <span id={`${id}-eyebrow`} className="top-up-pack-eyebrow">
        {packEyebrow(pack, text)}
      </span>
      <span id={`${id}-name`} className="top-up-pack-name">
        {pack.name}
      </span>
      <span id={`${id}-credits`} className="top-up-pack-credits">
        {text("{value0} credits", { value0: pack.credits.toLocaleString(locale) })}
      </span>
      <span id={`${id}-price`} className="top-up-pack-price">
        {text("{value0} USD", { value0: formatPackPrice(pack.price_cents, pack.currency, locale) })}
      </span>
      <span id={`${id}-minutes`} className="top-up-pack-minutes">
        {minutesRange(pack.credits, range, locale, text)}
      </span>
      <span id={`${id}-note`} className="top-up-pack-note">
        {intro ? text("Your one-time new-client offer.") : text("One-time purchase")}
      </span>
    </button>
  );
}

/** Arrow keys (and Home/End) move the choice; the key a person presses is the move they make. */
const KEY_STEP: Record<string, number | "first" | "last"> = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1, Home: "first", End: "last" };

/**
 * The packs the account can buy now: the eligible Welcome Pack across the top,
 * then the standard packs in 3 columns (2 on tablets, 1 on phones). A radio
 * group: arrow keys move the choice (and the focus) at once, Tab leaves it, the
 * focus ring is visible. Choosing never charges.
 */
export function PackPicker({
  packs,
  range,
  selectedId,
  onSelect,
}: {
  packs: readonly PricingPack[];
  range: RateRange;
  selectedId: string;
  onSelect: (pack: PricingPack) => void;
}) {
  const { text } = useI18n();
  const group = React.useRef<HTMLDivElement>(null);
  const intro = packs.find(isIntroPack);
  const standard = standardPacks(packs);
  // Reading order: the offer, then the standard packs.
  const ordered = intro ? [intro, ...standard] : standard;

  function onKeyDown(event: React.KeyboardEvent) {
    const step = KEY_STEP[event.key];
    if (step === undefined || event.altKey || event.ctrlKey || event.metaKey) return;
    event.preventDefault();
    const from = ordered.findIndex((p) => p.id === selectedId);
    const to = step === "first" ? 0 : step === "last" ? ordered.length - 1 : (from + step + ordered.length) % ordered.length;
    const next = ordered[to];
    if (!next) return;
    onSelect(next);
    group.current?.querySelector<HTMLElement>(`[data-pack-id="${next.id}"]`)?.focus();
  }

  return (
    <div ref={group} role="radiogroup" className="top-up-packs" aria-label={text("Credit packs")} onKeyDown={onKeyDown}>
      {intro ? <PackCard pack={intro} range={range} selected={intro.id === selectedId} onSelect={onSelect} /> : null}
      <div className="top-up-pack-grid">
        {standard.map((pack) => (
          <PackCard key={pack.id} pack={pack} range={range} selected={pack.id === selectedId} onSelect={onSelect} />
        ))}
      </div>
    </div>
  );
}
