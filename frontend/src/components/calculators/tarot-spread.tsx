"use client";
/* eslint-disable @next/next/no-img-element */
import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import type { TarotCard, TarotSpread as Spread } from "@/lib/api/astrology";
import { CARD_RADIUS, placeCard, spreadGeometry } from "@/lib/astro/spread-layout";
import { HOUSE_KEYWORDS, TAROT_CARDS, houseLabel, tarotImage } from "@/lib/astro/zodiac";
import { cn } from "@/lib/utils";

const GEOMETRY = spreadGeometry();
/** Native artwork size (210 × 355) → stage size. */
const SCALE = GEOMETRY.cardWidth / 210;

/** Display name of a card slug (unknown slugs keep their slug). */
export function cardName(slug: string): string {
  return TAROT_CARDS[slug] ?? slug;
}

/**
 * Why a card sits in its house, e.g. "Sun in Leo in the tenth house" or
 * "Cusp sign: Leo — no planet in this house".
 */
export function useCardSource() {
  const { text } = useI18n();
  return React.useCallback(
    (card: TarotCard) => {
      const house = text(houseLabel(card.house)).toLowerCase();
      if (card.source.kind === "planet" && card.source.body) {
        const base = text("{value0} in {value1}, in the {value2}", {
          value0: text(card.source.body),
          value1: text(card.source.sign),
          value2: house,
        });
        return card.reversed ? `${base} · ${text("retrograde, so the card is reversed")}` : base;
      }
      return text("Cusp sign: {value0} (no planet in this house)", { value0: text(card.source.sign) });
    },
    [text],
  );
}

/**
 * The Twelve Houses spread, drawn from the API's `tarot` block: one card per
 * house, counter-clockwise from the first house at 9 o'clock (the
 * Ascendant), each with its house number; a reversed card (retrograde
 * planet) is turned upside down. Frames have an 11 px radius at the
 * artwork's native size. The picture is one image for assistive technology;
 * the list below it carries every card and its reason.
 */
export function TarotSpread({ spread, className }: { spread: Spread; className?: string }) {
  const { text } = useI18n();
  const id = React.useId().replace(/:/g, "");
  const cards = [...spread.cards].filter((c) => c.house >= 1 && c.house <= 12).sort((a, b) => a.house - b.house);
  const w = GEOMETRY.cardWidth;
  const h = GEOMETRY.cardHeight;
  const radius = CARD_RADIUS * SCALE;

  return (
    <svg
      viewBox={`0 0 ${GEOMETRY.width.toFixed(1)} ${GEOMETRY.height.toFixed(1)}`}
      role="img"
      aria-labelledby={`${id}-title ${id}-desc`}
      data-testid="tarot-spread"
      className={cn("block h-auto w-full", className)}
    >
      <title id={`${id}-title`}>{text("Twelve-card tarot spread")}</title>
      <desc id={`${id}-desc`}>
        {text("Twelve cards laid counter-clockwise from the first house at 9 o'clock. Each card and the reason it was drawn are listed below.")}
      </desc>
      <defs>
        <clipPath id={`${id}-card`} clipPathUnits="userSpaceOnUse">
          <rect x={-w / 2} y={-h / 2} width={w} height={h} rx={radius} ry={radius} />
        </clipPath>
        <filter id={`${id}-shadow`} x="-20%" y="-20%" width="140%" height="140%">
          <feDropShadow dx="0" dy="3" stdDeviation="2.5" floodColor="#000" floodOpacity="0.14" />
        </filter>
      </defs>

      {/* Spokes and house themes (desktop sizes only; tiny on phones). */}
      <g aria-hidden="true">
        <ellipse cx={GEOMETRY.cx} cy={GEOMETRY.cy} rx={GEOMETRY.rx * 0.36} ry={GEOMETRY.ry * 0.36} fill="none" stroke="#aa8e62" strokeWidth={1} opacity={0.5} />
        {cards.map((card) => {
          const at = placeCard(card.house, card.reversed, GEOMETRY);
          const label = {
            x: GEOMETRY.cx + (at.badge.x - GEOMETRY.cx) * 0.62,
            y: GEOMETRY.cy + (at.badge.y - GEOMETRY.cy) * 0.62,
          };
          return (
            <g key={`spoke-${card.house}`}>
              <line x1={GEOMETRY.cx} y1={GEOMETRY.cy} x2={at.badge.x} y2={at.badge.y} stroke="#aa8e62" strokeWidth={1} opacity={0.45} />
              <text x={label.x} y={label.y} textAnchor="middle" dominantBaseline="central" fontSize={15} fill="#4d4632" className="max-md:hidden">
                {text(HOUSE_KEYWORDS[card.house - 1])}
              </text>
            </g>
          );
        })}
        <circle cx={GEOMETRY.cx} cy={GEOMETRY.cy} r={5} fill="#c29a3d" />
      </g>

      {cards.map((card) => {
        const at = placeCard(card.house, card.reversed, GEOMETRY);
        const src = tarotImage(card.card);
        return (
          <g key={card.house} data-house={card.house} data-card={card.card} data-reversed={card.reversed || undefined}>
            <g transform={`translate(${at.x} ${at.y}) rotate(${at.rotation})`} filter={`url(#${id}-shadow)`}>
              <g clipPath={`url(#${id}-card)`}>
                <rect x={-w / 2} y={-h / 2} width={w} height={h} fill="#f4f0ea" />
                {src ? <image href={src} x={-w / 2} y={-h / 2} width={w} height={h} preserveAspectRatio="xMidYMid slice" /> : null}
              </g>
              <rect
                x={-w / 2}
                y={-h / 2}
                width={w}
                height={h}
                rx={radius}
                ry={radius}
                fill="none"
                stroke={card.uncertain ? "#8a6a1f" : "#2e2a22"}
                strokeWidth={card.uncertain ? 1.6 : 1}
                strokeDasharray={card.uncertain ? "5 4" : undefined}
              />
              {!src ? (
                <text x={0} y={0} textAnchor="middle" dominantBaseline="central" fontSize={12} fill="#15100b">
                  {text(cardName(card.card))}
                </text>
              ) : null}
            </g>
            <circle cx={at.badge.x} cy={at.badge.y} r={15} fill="#f4f0ea" stroke="#15100b" strokeWidth={1.2} />
            <text x={at.badge.x} y={at.badge.y} textAnchor="middle" dominantBaseline="central" fontSize={15} fontWeight={600} fill="#15100b">
              {card.house}
            </text>
          </g>
        );
      })}
    </svg>
  );
}

/** The spread's cards with their reasons (the accessible companion of the picture). */
export function SpreadCardList({ spread }: { spread: Spread }) {
  const { text } = useI18n();
  const source = useCardSource();
  const cards = [...spread.cards].sort((a, b) => a.house - b.house);
  return (
    <ol className="grid gap-x-8 gap-y-5 text-left sm:grid-cols-2 lg:grid-cols-3" aria-label={text("Your twelve cards")} data-testid="spread-cards">
      {cards.map((card) => {
        const src = tarotImage(card.card);
        return (
          <li key={card.house} className="flex items-start gap-3" data-house={card.house}>
            <span className="relative shrink-0" aria-hidden="true">
              {src ? (
                <img
                  src={src}
                  alt=""
                  width={42}
                  height={71}
                  loading="lazy"
                  decoding="async"
                  className={cn("h-[71px] w-[42px] rounded-[2.2px] border border-ink/70 object-cover", card.reversed && "rotate-180")}
                />
              ) : (
                <span className="block h-[71px] w-[42px] rounded-[2.2px] border border-ink/70 bg-cream-light" />
              )}
            </span>
            <span className="min-w-0">
              <span className="block text-[11px] font-semibold tracking-[1.2px] uppercase text-ink/75">
                {text("House {value0}", { value0: card.house })}
                {" · "}
                {text(HOUSE_KEYWORDS[card.house - 1] ?? "")}
              </span>
              <span className="block font-display text-[17px] leading-tight">
                {text(cardName(card.card))}
                {card.reversed ? <span className="ml-2 align-middle font-serif text-[11px] font-semibold tracking-wide uppercase">{text("Reversed")}</span> : null}
              </span>
              <span className="mt-0.5 block text-[13px] leading-snug font-light">{source(card)}</span>
              {card.uncertain ? (
                <span className="mt-0.5 block text-[12px] leading-snug text-[#6b5214]">{text("This card may change with your exact birth time.")}</span>
              ) : null}
            </span>
          </li>
        );
      })}
    </ol>
  );
}
