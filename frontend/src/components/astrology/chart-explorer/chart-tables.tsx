"use client";

import { useI18n } from "@/i18n/i18n-provider";
import type { NatalChart } from "@/lib/api/astrology";
import { ASPECT_NAMES, BODY_ORDER, degreeInSign, houseLabel } from "@/lib/astro/zodiac";
import { aspectLineStyle } from "@/lib/astro/wheel-geometry";

/** The chart as plain tables: positions, house cusps and aspects (an alternative to the wheel). */
export function ChartTables({ chart }: { chart: NatalChart }) {
  const { text } = useI18n();
  const bodies = chart.positions.filter((p) => (BODY_ORDER as readonly string[]).includes(p.body));
  const legend = ["square", "trine", "sextile"].map((type) => ({ type, style: aspectLineStyle(type)! }));
  return (
    <div className="grid gap-8 text-sm lg:grid-cols-2">
      <table className="w-full border-collapse text-left">
        <caption className="mb-2 text-left font-semibold">{text("Positions")}</caption>
        <thead>
          <tr className="border-b border-gold/50 text-xs tracking-wide uppercase">
            <th scope="col" className="py-2 pr-3 font-semibold">{text("Point")}</th>
            <th scope="col" className="py-2 pr-3 font-semibold">{text("Sign and degree")}</th>
            <th scope="col" className="py-2 pr-3 font-semibold">{text("House")}</th>
            <th scope="col" className="py-2 font-semibold">{text("Motion")}</th>
          </tr>
        </thead>
        <tbody>
          {bodies.map((p) => (
            <tr key={p.body} className="border-b border-gold/20">
              <th scope="row" className="py-1.5 pr-3 font-normal">{text(p.body)}</th>
              <td className="py-1.5 pr-3">
                {text(p.sign)} {p.degree}°{p.minute}′{p.second}″
              </td>
              <td className="py-1.5 pr-3">{p.house ?? "—"}</td>
              <td className="py-1.5">{p.retrograde ? text("Retrograde") : text("Direct")}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="grid content-start gap-8">
        {chart.houses && chart.houses.length > 0 ? (
          <table className="w-full border-collapse text-left">
            <caption className="mb-2 text-left font-semibold">{text("House cusps")}</caption>
            <thead>
              <tr className="border-b border-gold/50 text-xs tracking-wide uppercase">
                <th scope="col" className="py-2 pr-3 font-semibold">{text("House")}</th>
                <th scope="col" className="py-2 font-semibold">{text("Cusp")}</th>
              </tr>
            </thead>
            <tbody>
              {chart.houses.map((h) => (
                <tr key={h.number} className="border-b border-gold/20">
                  <th scope="row" className="py-1.5 pr-3 font-normal">{text(houseLabel(h.number))}</th>
                  <td className="py-1.5">
                    {text(h.sign)} {degreeInSign(h.cusp)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : null}
        <table className="w-full border-collapse text-left">
          <caption className="mb-2 text-left font-semibold">{text("Aspects")}</caption>
          <thead>
            <tr className="border-b border-gold/50 text-xs tracking-wide uppercase">
              <th scope="col" className="py-2 pr-3 font-semibold">{text("Aspect")}</th>
              <th scope="col" className="py-2 pr-3 font-semibold">{text("Orb")}</th>
              <th scope="col" className="py-2 font-semibold">{text("Phase")}</th>
            </tr>
          </thead>
          <tbody>
            {chart.aspects.length === 0 ? (
              <tr>
                <td colSpan={3} className="py-2">
                  {text("No major aspects.")}
                </td>
              </tr>
            ) : (
              chart.aspects.map((a) => (
                <tr key={`${a.a}-${a.b}-${a.type}`} className="border-b border-gold/20">
                  <th scope="row" className="py-1.5 pr-3 font-normal">
                    {text("{value0} {value1} {value2}", { value0: text(a.a), value1: text(ASPECT_NAMES[a.type] ?? a.type).toLowerCase(), value2: text(a.b) })}
                  </th>
                  <td className="py-1.5 pr-3">{a.orb.toFixed(1)}°</td>
                  <td className="py-1.5">{a.applying ? text("Applying") : text("Separating")}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
        <ul className="flex flex-wrap gap-x-6 gap-y-2 text-xs" aria-label={text("Aspect lines")}>
          {legend.map(({ type, style }) => (
            <li key={type} className="flex items-center gap-2">
              <svg width="36" height="6" aria-hidden="true">
                <line x1="0" y1="3" x2="36" y2="3" stroke={style.color} strokeWidth={1.4} strokeDasharray={style.dash} opacity={style.opacity} />
              </svg>
              {text(style.label)}
            </li>
          ))}
          <li>{text("Conjunctions are listed but not drawn.")}</li>
        </ul>
      </div>
    </div>
  );
}
