/**
 * Layout of the Twelve Houses spread: one card per house on an ellipse, the
 * first house just below 9 o'clock (the Ascendant) and the houses running
 * counter-clockwise, exactly like the houses of a chart wheel. Cards stay
 * upright so a reversed card (retrograde planet) is simply turned 180°.
 */
import { pointPosition } from "@/lib/astro/wheel-geometry";

/** Card artwork is 210 × 355 (Figma "Component / Tarot Card"). */
export const CARD_ASPECT = 210 / 355;
/** Frame corner radius at the artwork's native size (revisions 2.0: 11). */
export const CARD_RADIUS = 11;

export type SpreadGeometry = {
  width: number;
  height: number;
  cx: number;
  cy: number;
  rx: number;
  ry: number;
  cardWidth: number;
  cardHeight: number;
};

/**
 * Stage size for cards `cardWidth` wide. The radii keep neighbouring cards
 * apart: houses 12 and 1 sit 15° above and below 9 o'clock, so the vertical
 * radius must clear a card height between them; houses 1 and 2 must clear a
 * card width horizontally.
 */
export function spreadGeometry(cardWidth = 105, gap = 8, margin = 14): SpreadGeometry {
  const cardHeight = cardWidth / CARD_ASPECT;
  const sin15 = Math.sin((15 * Math.PI) / 180);
  const ry = (cardHeight + gap) / (2 * sin15);
  const rx = (cardWidth + gap) / (Math.cos((15 * Math.PI) / 180) - Math.cos((45 * Math.PI) / 180));
  const width = 2 * rx + cardWidth + 2 * margin;
  const height = 2 * ry + cardHeight + 2 * margin;
  return { width, height, cx: width / 2, cy: height / 2, rx, ry, cardWidth, cardHeight };
}

/** Chart angle of a house's card: the middle of its 30° sector from the Ascendant. */
export function houseAngle(house: number): number {
  return (house - 1) * 30 + 15;
}

export type CardPlacement = {
  house: number;
  /** Card centre. */
  x: number;
  y: number;
  /** Clockwise rotation of the card face: 180 when reversed, else 0. */
  rotation: number;
  /** House-number badge centre (on the card edge that faces the centre). */
  badge: { x: number; y: number };
};

/** Where each house's card goes; `reversed` turns that card upside down. */
export function placeCard(house: number, reversed: boolean, g: SpreadGeometry = spreadGeometry()): CardPlacement {
  const angle = houseAngle(house);
  // Points on the ellipse: scale the unit-circle position per axis.
  const unit = pointPosition(0, 0, 1, angle);
  const x = g.cx + unit.x * g.rx;
  const y = g.cy + unit.y * g.ry;
  // The badge sits where the line to the centre leaves the card.
  const dx = g.cx - x;
  const dy = g.cy - y;
  const scale = Math.min(Math.abs(dx) > 1e-9 ? g.cardWidth / 2 / Math.abs(dx) : Infinity, Math.abs(dy) > 1e-9 ? g.cardHeight / 2 / Math.abs(dy) : Infinity);
  return { house, x, y, rotation: reversed ? 180 : 0, badge: { x: x + dx * scale, y: y + dy * scale } };
}
