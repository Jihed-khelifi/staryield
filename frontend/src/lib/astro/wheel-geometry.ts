/**
 * Chart-wheel geometry shared by the natal wheel, the zodiac wheel and the
 * twelve-house spread.
 *
 * The conventions are ported from AstroChart by Arthur Fücher
 * (https://github.com/AstroDraw/AstroChart, MIT licence, © 2015-2025):
 *   - `SHIFT_IN_DEGREES` = 180 puts the chart's starting point (the
 *     Ascendant, or 0° Aries without houses) at 9 o'clock;
 *   - `getPointPosition` turns an angle into SVG coordinates (y grows
 *     downwards), so increasing longitude runs counter-clockwise;
 *   - colliding planet symbols are pushed apart along the ring a degree at a
 *     time (`placePointsInCollision`) and keep a pointer line to their true
 *     longitude.
 * The collision pass here is an iterative, bounded rewrite of AstroChart's
 * recursive `assemble`, so it can never loop or throw.
 */

/** AstroChart's `SHIFT_IN_DEGREES`: angle 0 is drawn at 9 o'clock. */
export const SHIFT_IN_DEGREES = 180;

export type Point = { x: number; y: number };

/** Normalise an angle to [0, 360). */
export function norm360(angle: number): number {
  const a = angle % 360;
  return a < 0 ? a + 360 : a;
}

/**
 * Chart angle of an ecliptic longitude: degrees counter-clockwise from the
 * chart's starting point (the Ascendant, or 0° Aries when there are no
 * houses). AstroChart: `longitude + shift` with `shift = 360 − cusps[0]`.
 */
export function chartAngle(longitude: number, ascendant = 0): number {
  return norm360(longitude - ascendant);
}

/**
 * AstroChart `getPointPosition`: the point at `radius` from (cx, cy) for a
 * chart angle in degrees. Angle 0 → 9 o'clock, 90 → 6 o'clock, 180 → 3
 * o'clock, 270 → 12 o'clock (counter-clockwise on screen).
 */
export function pointPosition(cx: number, cy: number, radius: number, angle: number): Point {
  const radians = ((SHIFT_IN_DEGREES - angle) * Math.PI) / 180;
  return { x: cx + radius * Math.cos(radians), y: cy + radius * Math.sin(radians) };
}

/**
 * Rotation (degrees, SVG clockwise) that lays text along the ring with its
 * top facing outwards, the way the zodiac labels sit in the design.
 */
export function tangentRotation(angle: number): number {
  return norm360(270 - angle);
}

/** Shortest signed difference b − a in degrees, in (−180, 180]. */
export function angleDelta(a: number, b: number): number {
  const d = norm360(b - a);
  return d > 180 ? d - 360 : d;
}

export type GlyphInput = { id: string; angle: number };
export type PlacedGlyph = {
  id: string;
  /** The true chart angle (where the pointer line starts). */
  angle: number;
  /** Where the symbol is drawn, at least `minSeparation` from its neighbours. */
  displayAngle: number;
};

/**
 * Spread symbols along a ring so that neighbours are at least
 * `minSeparation` degrees apart, preserving their order around the circle.
 * Like AstroChart's `placePointsInCollision`, each colliding pair is nudged
 * apart symmetrically; the pass repeats until nothing collides (bounded).
 * When the ring is too small for every symbol the separation shrinks to
 * fit instead of failing.
 */
export function spreadGlyphs(points: GlyphInput[], minSeparation: number, maxPasses = 720): PlacedGlyph[] {
  const n = points.length;
  if (n === 0) return [];
  const sorted = [...points]
    .map((p) => ({ id: p.id, angle: norm360(p.angle) }))
    .sort((a, b) => a.angle - b.angle || a.id.localeCompare(b.id));
  if (n === 1) return [{ ...sorted[0], displayAngle: sorted[0].angle }];
  const gapNeeded = Math.min(minSeparation, 360 / n);
  // Unwrapped display angles: strictly increasing, spanning less than 360°.
  const display = sorted.map((p) => p.angle);
  const tolerance = 1e-6;
  for (let pass = 0; pass < maxPasses; pass++) {
    let moved = false;
    for (let i = 0; i < n; i++) {
      const j = (i + 1) % n;
      const gap = j === 0 ? display[0] + 360 - display[n - 1] : display[j] - display[i];
      if (gap + tolerance >= gapNeeded) continue;
      const push = (gapNeeded - gap) / 2;
      display[i] -= push;
      if (j === 0) display[0] += push;
      else display[j] += push;
      moved = true;
    }
    if (!moved) break;
  }
  return sorted.map((p, i) => ({ id: p.id, angle: p.angle, displayAngle: norm360(display[i]) }));
}

/** Smallest angular gap between neighbouring display angles (for tests). */
export function minimumGap(placed: PlacedGlyph[]): number {
  if (placed.length < 2) return 360;
  const angles = placed.map((p) => p.displayAngle).sort((a, b) => a - b);
  let gap = angles[0] + 360 - angles[angles.length - 1];
  for (let i = 1; i < angles.length; i++) gap = Math.min(gap, angles[i] - angles[i - 1]);
  return gap;
}

export type AspectLineStyle = {
  /** SVG stroke-dasharray, or undefined for a solid line. */
  dash?: string;
  width: number;
  opacity: number;
  color: string;
  /** Legend label (display copy). */
  label: string;
};

/**
 * Aspect line styles from the design's three line treatments: hard aspects
 * (squares, oppositions) solid, trines dashed, sextiles dotted.
 * Conjunctions join neighbouring points, so they are listed but not drawn.
 */
export function aspectLineStyle(type: string): AspectLineStyle | null {
  switch (type) {
    case "opposition":
    case "square":
      return { width: 1.1, opacity: 0.88, color: "#0e0e0e", label: "Squares and oppositions" };
    case "trine":
      return { dash: "7.4 7.4", width: 0.95, opacity: 0.78, color: "#333333", label: "Trines" };
    case "sextile":
      return { dash: "1.5 5.9", width: 0.85, opacity: 0.7, color: "#4f4f4f", label: "Sextiles" };
    default:
      return null;
  }
}
