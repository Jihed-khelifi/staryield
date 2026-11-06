"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { SIGNS, SIGN_ARTWORK, isSign } from "@/lib/astro/zodiac";
import { pointPosition, tangentRotation } from "@/lib/astro/wheel-geometry";
import { cn } from "@/lib/utils";

const SIZE = 800;
const C = SIZE / 2;
/** Ring radii of the Figma "Container / Astronomical Chart" (464:11072, 800 px). */
const R = { outer: 398.5, dashed: 380, disk: 345, names: 300, signs: 255, glyphs: 210, inner: 155, core: 85 } as const;
const STAR = "https://assets.staryield.net/assets/figma/f90ea.png";

/**
 * The zodiac wheel of the Astrology calculator (Figma 464:11072), drawn as
 * SVG from data. Signs run counter-clockwise from 0° Aries at 9 o'clock
 * (AstroChart's convention, the same as the natal wheel); the sun sign
 * returned by the API is highlighted and, on a cusp day, the other possible
 * sign gets a dashed ring.
 */
export function ZodiacWheel({
  sign,
  alternatives = [],
  className,
}: {
  sign?: string | null;
  alternatives?: string[];
  className?: string;
}) {
  const { text, locale } = useI18n();
  const id = React.useId().replace(/:/g, "");
  const highlighted = sign && isSign(sign) ? sign : null;
  const others = alternatives.filter((s) => isSign(s) && s !== highlighted);
  const title = text("Zodiac wheel");
  const description = highlighted
    ? others.length > 0
      ? text("The twelve signs run counter-clockwise from Aries. {value0} is highlighted; on your birthday the Sun was also in {value1}.", {
          value0: text(highlighted),
          value1: others.map((s) => text(s)).join(", "),
        })
      : text("The twelve signs run counter-clockwise from Aries. Your sun sign, {value0}, is highlighted.", { value0: text(highlighted) })
    : text("The twelve signs run counter-clockwise from Aries.");

  return (
    <svg
      viewBox={`0 0 ${SIZE} ${SIZE}`}
      role="img"
      aria-labelledby={`${id}-title ${id}-desc`}
      data-testid="zodiac-wheel"
      data-highlight={highlighted ?? undefined}
      className={cn("block h-auto w-full", className)}
    >
      <title id={`${id}-title`}>{title}</title>
      <desc id={`${id}-desc`}>{description}</desc>
      <defs>
        <radialGradient id={`${id}-glow`} cx="50%" cy="50%" r="50%">
          <stop offset="0%" stopColor="#e3c044" stopOpacity="0.95" />
          <stop offset="30%" stopColor="#d8bd5a" stopOpacity="0.55" />
          <stop offset="62%" stopColor="#c9b97d" stopOpacity="0" />
        </radialGradient>
        <filter id={`${id}-halo`} x="-50%" y="-50%" width="200%" height="200%">
          <feDropShadow dx="0" dy="0" stdDeviation="8" floodColor="#c29a3d" floodOpacity="0.95" />
        </filter>
      </defs>

      <circle cx={C} cy={C} r={R.outer} fill="#c1bba9" stroke="#000" strokeWidth={3} />
      <circle cx={C} cy={C} r={R.dashed} fill="none" stroke="#000" strokeWidth={1} strokeDasharray="3 4" opacity={0.75} />
      <circle cx={C} cy={C} r={R.disk} fill="#b6b08f" stroke="#000" strokeWidth={1.5} />
      <circle cx={C} cy={C} r={R.disk} fill={`url(#${id}-glow)`} />
      <circle cx={C} cy={C} r={R.signs} fill="none" stroke="#000" strokeWidth={2} />
      <circle cx={C} cy={C} r={R.inner} fill="none" stroke="#000" strokeWidth={2} />

      {SIGNS.map((_, i) => {
        const from = pointPosition(C, C, R.core, i * 30);
        const to = pointPosition(C, C, R.disk, i * 30);
        return <line key={i} x1={from.x} y1={from.y} x2={to.x} y2={to.y} stroke="#000" strokeWidth={1} opacity={0.6} />;
      })}

      {SIGNS.map((s, i) => {
        const angle = i * 30 + 15;
        const at = pointPosition(C, C, R.names, angle);
        return (
          <text
            key={s}
            x={at.x}
            y={at.y}
            transform={`rotate(${tangentRotation(angle)} ${at.x} ${at.y})`}
            textAnchor="middle"
            dominantBaseline="central"
            className="font-display"
            fontSize={14}
            letterSpacing={0.42}
            fill="#000"
          >
            {text(s).toLocaleUpperCase(locale)}
          </text>
        );
      })}

      {SIGNS.map((s, i) => {
        const at = pointPosition(C, C, R.glyphs, i * 30 + 15);
        const art = SIGN_ARTWORK[s];
        const isHighlight = s === highlighted;
        const isAlternative = others.includes(s);
        return (
          <g key={s} data-sign={s} data-highlighted={isHighlight || undefined} data-alternative={isAlternative || undefined}>
            {isHighlight ? (
              <circle cx={at.x} cy={at.y} r={42} fill="#c29a3d" stroke="#000" strokeWidth={2.5} filter={`url(#${id}-halo)`} />
            ) : null}
            {isAlternative ? (
              <circle cx={at.x} cy={at.y} r={42} fill="rgb(194 154 61 / 0.25)" stroke="#000" strokeWidth={2} strokeDasharray="6 5" />
            ) : null}
            <image href={art.src} x={at.x - art.width / 2} y={at.y - art.height / 2} width={art.width} height={art.height} />
          </g>
        );
      })}

      <circle cx={C} cy={C} r={R.core} fill="#978f6e" stroke="#000" strokeWidth={2.5} />
      <image href={STAR} x={C - 50} y={C - 50} width={100} height={100} />
    </svg>
  );
}
