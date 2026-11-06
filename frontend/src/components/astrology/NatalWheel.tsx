"use client";

import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import type { NatalChart } from "@/lib/api/astrology";
import { BODY_ORDER, PLANET_GLYPHS, SIGNS, houseLabel } from "@/lib/astro/zodiac";
import { aspectLineStyle, chartAngle, pointPosition, spreadGlyphs, tangentRotation } from "@/lib/astro/wheel-geometry";
import { cn } from "@/lib/utils";

/*
 * Pure React SVG natal wheel (replaces the static AstrologicalChart image).
 * Geometry conventions are ported from AstroChart (MIT, © Arthur Fücher,
 * https://github.com/AstroDraw/AstroChart) — see lib/astro/wheel-geometry.ts:
 * the Ascendant sits at 9 o'clock, longitudes run counter-clockwise,
 * colliding symbols are spread along the ring with pointer lines to their
 * true degree, and aspects join the true positions on the inner circle.
 * Ring proportions and colours follow Figma "Container / Astronomical Chart"
 * (172:598 desktop, 867:16922 mobile).
 */

const SIZE = 640;
const C = SIZE / 2;
const R0 = 280;
const R = {
  outer: R0,
  sign: R0 * 0.8857, // black zodiac ring inner edge / gold ring outer edge
  gold: R0 * 0.8572, // gold ring inner edge / planet ring outer edge
  disk: R0 * 0.6914, // inner disk
  aspect: R0 * 0.6629, // aspect circle
  glyph: R0 * 0.778,
  label: R0 * 0.943,
} as const;
const GLYPH_SIZE = 26;
const MIN_SEPARATION = 7.5;
const SYMBOL_FONT = '"Apple Symbols", "Segoe UI Symbol", "Noto Sans Symbols", "Noto Sans Symbols 2", "DejaVu Sans", var(--font-serif), serif';

export type WheelLayout = {
  ascendant: number;
  points: { body: string; longitude: number; angle: number; displayAngle: number; retrograde: boolean }[];
};

/** Where every point is drawn (exported for tests and the chart tab). */
export function wheelLayout(chart: Pick<NatalChart, "angles" | "positions" | "time_known">): WheelLayout {
  const ascendant = chart.angles?.asc ?? 0;
  const bodies = chart.positions.filter((p) => (BODY_ORDER as readonly string[]).includes(p.body));
  const placed = spreadGlyphs(
    bodies.map((p) => ({ id: p.body, angle: chartAngle(p.longitude, ascendant) })),
    MIN_SEPARATION,
  );
  const byId = new Map(placed.map((p) => [p.id, p]));
  return {
    ascendant,
    points: bodies.map((p) => {
      const at = byId.get(p.body)!;
      return { body: p.body, longitude: p.longitude, angle: at.angle, displayAngle: at.displayAngle, retrograde: p.retrograde };
    }),
  };
}

export function NatalWheel({
  chart,
  highlight,
  className,
}: {
  chart: NatalChart;
  /** A body to emphasise (the open accordion row). */
  highlight?: string | null;
  className?: string;
}) {
  const { text, locale } = useI18n();
  const id = React.useId().replace(/:/g, "");
  const layout = React.useMemo(() => wheelLayout(chart), [chart]);
  const asc = layout.ascendant;
  const longitudes = new Map(chart.positions.map((p) => [p.body, p.longitude]));
  const cusps = chart.houses ?? [];

  const summary = chart.positions
    .filter((p) => (BODY_ORDER as readonly string[]).includes(p.body))
    .map((p) =>
      p.house
        ? text("{value0} in {value1}, {value2}", { value0: text(p.body), value1: text(p.sign), value2: text(houseLabel(p.house)).toLowerCase() })
        : text("{value0} in {value1}", { value0: text(p.body), value1: text(p.sign) }),
    )
    .join(". ");

  return (
    <svg
      viewBox={`0 0 ${SIZE} ${SIZE}`}
      role="img"
      aria-labelledby={`${id}-title ${id}-desc`}
      data-testid="natal-wheel"
      data-ascendant={asc}
      className={cn("block h-auto w-full", className)}
    >
      <title id={`${id}-title`}>{text("Natal chart wheel")}</title>
      <desc id={`${id}-desc`}>
        {chart.time_known && chart.rising_sign
          ? text("The Ascendant ({value0}) is at 9 o'clock and the signs run counter-clockwise.", { value0: text(chart.rising_sign) })
          : text("Without a birth time the wheel starts at 0° Aries at 9 o'clock; houses and the Ascendant are not shown.")}{" "}
        {summary}.
      </desc>
      <defs>
        <filter id={`${id}-shadow`} x="-20%" y="-20%" width="140%" height="140%">
          <feDropShadow dx="0" dy="6.4" stdDeviation="15.5" floodColor="#000" floodOpacity="0.25" />
          <feDropShadow dx="0" dy="4" stdDeviation="7.5" floodColor="#c29a3d" floodOpacity="0.42" />
        </filter>
      </defs>

      {/* Rings: black zodiac band, gold rule, planet band, inner disk. */}
      <circle cx={C} cy={C} r={R.outer} fill="#000" filter={`url(#${id}-shadow)`} />
      <circle cx={C} cy={C} r={R.sign} fill="#aa9063" stroke="#b8b5ad" strokeWidth={1} />
      <circle cx={C} cy={C} r={R.gold} fill="#c1bba9" stroke="#adaba3" strokeWidth={0.8} />
      <circle cx={C} cy={C} r={R.disk} fill="#fbf2e3" fillOpacity={0.75} stroke="#bdbab3" strokeWidth={0.66} />
      <circle cx={C} cy={C} r={R.aspect} fill="none" stroke="#bdbab3" strokeWidth={0.6} opacity={0.62} />

      {/* Sign boundaries and names on the black band. */}
      <g data-layer="signs">
        {SIGNS.map((sign, i) => {
          const start = chartAngle(i * 30, asc);
          const a = pointPosition(C, C, R.sign, start);
          const b = pointPosition(C, C, R.outer, start);
          const mid = chartAngle(i * 30 + 15, asc);
          const at = pointPosition(C, C, R.label, mid);
          return (
            <g key={sign} data-sign={sign}>
              <line x1={a.x} y1={a.y} x2={b.x} y2={b.y} stroke="#e0d5c3" strokeWidth={0.8} opacity={0.55} />
              <text
                x={at.x}
                y={at.y}
                transform={`rotate(${tangentRotation(mid)} ${at.x} ${at.y})`}
                textAnchor="middle"
                dominantBaseline="central"
                className="font-display"
                fontSize={12.8}
                letterSpacing={1.28}
                fill="#f6f4f0"
              >
                {text(sign).toLocaleUpperCase(locale)}
              </text>
            </g>
          );
        })}
      </g>

      {/* House cusps across the planet band; the angles are drawn stronger. */}
      <g data-layer="cusps">
        {cusps.map((house) => {
          const angle = chartAngle(house.cusp, asc);
          const a = pointPosition(C, C, R.disk, angle);
          const b = pointPosition(C, C, R.gold, angle);
          const axis = house.number === 1 || house.number === 4 || house.number === 7 || house.number === 10;
          return (
            <line
              key={house.number}
              data-house={house.number}
              x1={a.x}
              y1={a.y}
              x2={b.x}
              y2={b.y}
              stroke={axis ? "#f6f4f0" : "#e0d5c3"}
              strokeWidth={axis ? 1.6 : 1.1}
              opacity={0.9}
            />
          );
        })}
      </g>

      {/* Aspects between true positions, styled by aspect type. */}
      <g data-layer="aspects">
        {chart.aspects.map((aspect) => {
          const style = aspectLineStyle(aspect.type);
          const la = longitudes.get(aspect.a);
          const lb = longitudes.get(aspect.b);
          if (!style || la === undefined || lb === undefined) return null;
          const a = pointPosition(C, C, R.aspect, chartAngle(la, asc));
          const b = pointPosition(C, C, R.aspect, chartAngle(lb, asc));
          const emphasised = highlight && (aspect.a === highlight || aspect.b === highlight);
          return (
            <line
              key={`${aspect.a}-${aspect.b}-${aspect.type}`}
              data-aspect={aspect.type}
              x1={a.x}
              y1={a.y}
              x2={b.x}
              y2={b.y}
              stroke={emphasised ? "#8a6a1f" : style.color}
              strokeWidth={emphasised ? style.width * 1.8 : style.width}
              strokeDasharray={style.dash}
              opacity={highlight && !emphasised ? style.opacity * 0.45 : style.opacity}
            />
          );
        })}
      </g>

      {/* Planets: a tick at the true degree, the symbol (spread apart when crowded). */}
      <g data-layer="points">
        {layout.points.map((point) => {
          const tickA = pointPosition(C, C, R.gold, point.angle);
          const tickB = pointPosition(C, C, R.gold - 7, point.angle);
          const at = pointPosition(C, C, R.glyph, point.displayAngle);
          const moved = Math.abs(point.angle - point.displayAngle) > 0.5;
          const towards = pointPosition(C, C, R.glyph + GLYPH_SIZE * 0.55, point.displayAngle);
          const active = highlight === point.body;
          const glyph = PLANET_GLYPHS[point.body] ?? point.body.slice(0, 2);
          const isText = point.body === "Midheaven";
          return (
            <g
              key={point.body}
              data-body={point.body}
              data-angle={point.angle.toFixed(3)}
              data-display-angle={point.displayAngle.toFixed(3)}
              data-retrograde={point.retrograde || undefined}
              data-active={active || undefined}
            >
              <line x1={tickA.x} y1={tickA.y} x2={tickB.x} y2={tickB.y} stroke="#0e0e0e" strokeWidth={1.2} />
              {moved ? <line x1={tickB.x} y1={tickB.y} x2={towards.x} y2={towards.y} stroke="#0e0e0e" strokeWidth={0.6} opacity={0.55} /> : null}
              {active ? <circle cx={at.x} cy={at.y} r={17} fill="rgb(194 154 61 / 0.35)" stroke="#c29a3d" strokeWidth={1.5} /> : null}
              <text
                x={at.x}
                y={at.y}
                textAnchor="middle"
                dominantBaseline="central"
                fontSize={isText ? 13 : point.body === "Ascendant" ? GLYPH_SIZE * 0.82 : GLYPH_SIZE}
                className={isText ? "font-display" : undefined}
                style={isText ? undefined : { fontFamily: SYMBOL_FONT }}
                fill={active ? "#5c4510" : "#0e0e0e"}
              >
                {glyph}
              </text>
              {point.retrograde ? (
                <text
                  x={at.x + GLYPH_SIZE * 0.5}
                  y={at.y + GLYPH_SIZE * 0.42}
                  textAnchor="middle"
                  dominantBaseline="central"
                  fontSize={11}
                  style={{ fontFamily: SYMBOL_FONT }}
                  fill="#7a1f1f"
                >
                  ℞
                </text>
              ) : null}
            </g>
          );
        })}
      </g>
    </svg>
  );
}
