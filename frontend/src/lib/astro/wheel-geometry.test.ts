import { describe, expect, it } from "vitest";

import { chartAngle, minimumGap, norm360, pointPosition, spreadGlyphs, tangentRotation } from "@/lib/astro/wheel-geometry";
import { CARD_RADIUS, houseAngle, placeCard, spreadGeometry } from "@/lib/astro/spread-layout";
import { wheelLayout } from "@/components/astrology/NatalWheel";

const near = (a: number, b: number) => expect(a).toBeCloseTo(b, 6);

describe("wheel geometry (AstroChart conventions)", () => {
  it("puts angle 0 (the Ascendant) at 9 o'clock", () => {
    const p = pointPosition(100, 100, 50, 0);
    near(p.x, 50);
    near(p.y, 100);
  });
  it("runs counter-clockwise: 90° is 6 o'clock, 180° is 3 o'clock, 270° is 12 o'clock", () => {
    const six = pointPosition(100, 100, 50, 90);
    near(six.x, 100);
    near(six.y, 150);
    const three = pointPosition(100, 100, 50, 180);
    near(three.x, 150);
    const twelve = pointPosition(100, 100, 50, 270);
    near(twelve.y, 50);
  });
  it("measures chart angles from the Ascendant", () => {
    near(chartAngle(40.5, 40.5), 0);
    near(chartAngle(10, 40), 330);
    near(norm360(-30), 330);
    near(tangentRotation(270), 0);
  });
  it("places the Ascendant of a real chart at 9 o'clock", () => {
    const layout = wheelLayout({
      time_known: true,
      angles: { asc: 40.52, mc: 288.7, dsc: 220.52, ic: 108.7 },
      positions: [
        { body: "Ascendant", longitude: 40.52, latitude: 0, speed: 0, sign: "Taurus", degree: 10, minute: 31, second: 14, formatted: "", retrograde: false },
        { body: "Sun", longitude: 353.47, latitude: 0, speed: 1, sign: "Pisces", degree: 23, minute: 28, second: 1, formatted: "", retrograde: false },
      ],
    });
    const asc = layout.points.find((p) => p.body === "Ascendant")!;
    near(asc.angle, 0);
    near(layout.points.find((p) => p.body === "Sun")!.angle, norm360(353.47 - 40.52));
  });
});

describe("spreadGlyphs", () => {
  it("spreads colliding glyphs apart, keeping their order", () => {
    const placed = spreadGlyphs(
      [
        { id: "Sun", angle: 100 },
        { id: "Mercury", angle: 101 },
        { id: "Venus", angle: 102.5 },
        { id: "Mars", angle: 250 },
      ],
      8,
    );
    expect(minimumGap(placed)).toBeGreaterThanOrEqual(8 - 1e-6);
    const order = [...placed].sort((a, b) => a.displayAngle - b.displayAngle).map((p) => p.id);
    expect(order).toEqual(["Sun", "Mercury", "Venus", "Mars"]);
    expect(placed.find((p) => p.id === "Mars")!.displayAngle).toBeCloseTo(250, 3);
    expect(placed.find((p) => p.id === "Sun")!.angle).toBe(100); // true angle kept for the pointer
  });
  it("handles clusters across 0° and total pile-ups without failing", () => {
    const wrap = spreadGlyphs([{ id: "a", angle: 359 }, { id: "b", angle: 1 }, { id: "c", angle: 0 }], 9);
    expect(minimumGap(wrap)).toBeGreaterThanOrEqual(9 - 1e-6);
    const pile = spreadGlyphs(Array.from({ length: 12 }, (_, i) => ({ id: String(i), angle: 77 })), 7.5);
    expect(minimumGap(pile)).toBeGreaterThanOrEqual(7.5 - 1e-6);
    const tooMany = spreadGlyphs(Array.from({ length: 12 }, (_, i) => ({ id: String(i), angle: i })), 40);
    expect(minimumGap(tooMany)).toBeGreaterThan(29.9);
    expect(spreadGlyphs([], 5)).toEqual([]);
  });
});

describe("tarot spread layout", () => {
  const g = spreadGeometry();
  it("starts at 9 o'clock and runs counter-clockwise", () => {
    const first = placeCard(1, false, g);
    expect(first.x).toBeLessThan(g.cx - g.rx * 0.9); // left side
    expect(first.y).toBeGreaterThan(g.cy); // just below 9 o'clock
    const fourth = placeCard(4, false, g);
    expect(fourth.y).toBeGreaterThan(g.cy + g.ry * 0.9); // 6 o'clock side
    const seventh = placeCard(7, false, g);
    expect(seventh.x).toBeGreaterThan(g.cx + g.rx * 0.9); // 3 o'clock side
    const tenth = placeCard(10, false, g);
    expect(tenth.y).toBeLessThan(g.cy - g.ry * 0.9); // 12 o'clock side
    expect(houseAngle(1)).toBe(15);
  });
  it("turns reversed cards 180° and keeps neighbours apart", () => {
    expect(placeCard(7, true, g).rotation).toBe(180);
    expect(placeCard(7, false, g).rotation).toBe(0);
    for (let h = 1; h <= 12; h++) {
      const a = placeCard(h, false, g);
      const b = placeCard((h % 12) + 1, false, g);
      const overlapX = Math.abs(a.x - b.x) < g.cardWidth - 0.5;
      const overlapY = Math.abs(a.y - b.y) < g.cardHeight - 0.5;
      expect(overlapX && overlapY).toBe(false);
    }
  });
  it("uses frames of radius 11", () => {
    expect(CARD_RADIUS).toBe(11);
  });
});
