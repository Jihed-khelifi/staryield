"use client";

/**
 * Chart.js wrappers styled with the Staryield palette. Only the pieces used
 * are registered (tree-shaken). One value axis per chart; gross earnings are
 * a dashed line (or outlined bars) so they never rely on colour alone.
 */
import * as React from "react";
import {
  ArcElement,
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  Filler,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  Tooltip,
  type ChartOptions,
} from "chart.js";
import { Bar, Doughnut, Line } from "react-chartjs-2";

ChartJS.register(ArcElement, BarElement, CategoryScale, Filler, Legend, LinearScale, LineElement, PointElement, Tooltip);

export const CHART_COLORS = {
  net: "#a77b1f",
  netFill: "rgba(194, 154, 61, 0.22)",
  gross: "#5c4a32",
  grid: "rgba(170, 142, 98, 0.22)",
  ink: "#15100b",
  muted: "rgba(21, 16, 11, 0.65)",
  track: "#d6cfbd",
  surface: "#f4f0ea",
};

function fontFamily(): string {
  if (typeof document === "undefined") return "ui-sans-serif, system-ui, sans-serif";
  const inter = getComputedStyle(document.body).getPropertyValue("--font-inter").trim();
  return `${inter ? `${inter}, ` : ""}ui-sans-serif, system-ui, sans-serif`;
}

function reducedMotion(): boolean {
  return typeof window !== "undefined" && typeof window.matchMedia === "function" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

type EarningsProps = {
  kind: "line" | "bar";
  labels: string[];
  net: number[];
  gross: number[];
  netLabel: string;
  grossLabel: string;
  ariaLabel: string;
  locale: string;
};

/** Net (solid, filled) and gross (dashed) credits per bucket, one shared axis. */
export function EarningsChart({ kind, labels, net, gross, netLabel, grossLabel, ariaLabel, locale }: EarningsProps) {
  const family = React.useMemo(() => fontFamily(), []);
  const number = React.useMemo(() => new Intl.NumberFormat(locale), [locale]);
  const options = React.useMemo<ChartOptions<"line" | "bar">>(
    () => ({
      responsive: true,
      maintainAspectRatio: false,
      animation: reducedMotion() ? false : { duration: 400 },
      interaction: { mode: "index", intersect: false },
      plugins: {
        legend: {
          position: "bottom",
          align: "start",
          labels: { color: CHART_COLORS.ink, font: { family, size: 12 }, usePointStyle: true, pointStyle: "line", boxWidth: 24, padding: 16 },
        },
        tooltip: {
          backgroundColor: CHART_COLORS.surface,
          borderColor: "#aa8e62",
          borderWidth: 1,
          titleColor: CHART_COLORS.ink,
          bodyColor: CHART_COLORS.ink,
          titleFont: { family, weight: 600 },
          bodyFont: { family },
          padding: 10,
          callbacks: { label: (ctx) => `${ctx.dataset.label}: ${number.format(Number(ctx.parsed.y ?? 0))}` },
        },
      },
      scales: {
        x: { grid: { display: false }, border: { color: CHART_COLORS.grid }, ticks: { color: CHART_COLORS.muted, font: { family, size: 11 }, maxRotation: 0, autoSkipPadding: 12 } },
        y: {
          beginAtZero: true,
          grid: { color: CHART_COLORS.grid },
          border: { display: false },
          ticks: { color: CHART_COLORS.muted, font: { family, size: 11 }, precision: 0, callback: (v) => number.format(Number(v)) },
        },
      },
    }),
    [family, number],
  );

  if (kind === "bar") {
    return (
      <Bar
        role="img"
        aria-label={ariaLabel}
        options={options as ChartOptions<"bar">}
        data={{
          labels,
          datasets: [
            { label: netLabel, data: net, backgroundColor: CHART_COLORS.net, borderRadius: 4, borderSkipped: "start", maxBarThickness: 28, categoryPercentage: 0.7 },
            {
              label: grossLabel,
              data: gross,
              backgroundColor: "transparent",
              borderColor: CHART_COLORS.gross,
              // Outlined (not filled): distinguishable from net without colour.
              borderWidth: 2,
              borderRadius: 4,
              borderSkipped: "start",
              maxBarThickness: 28,
              categoryPercentage: 0.7,
            },
          ],
        }}
      />
    );
  }
  return (
    <Line
      role="img"
      aria-label={ariaLabel}
      options={options as ChartOptions<"line">}
      data={{
        labels,
        datasets: [
          {
            label: netLabel,
            data: net,
            borderColor: CHART_COLORS.net,
            backgroundColor: CHART_COLORS.netFill,
            fill: "origin",
            borderWidth: 2,
            tension: 0.3,
            pointRadius: labels.length > 31 ? 0 : 3,
            pointHoverRadius: 5,
            pointBackgroundColor: CHART_COLORS.net,
          },
          {
            label: grossLabel,
            data: gross,
            borderColor: CHART_COLORS.gross,
            backgroundColor: "transparent",
            borderDash: [5, 4],
            borderWidth: 2,
            tension: 0.3,
            pointRadius: 0,
            pointHoverRadius: 4,
          },
        ],
      }}
    />
  );
}

/** Half-donut gauge for the 0..100 success score. */
export function ScoreGauge({ value, ariaLabel }: { value: number; ariaLabel: string }) {
  const v = Math.max(0, Math.min(100, value));
  return (
    <Doughnut
      role="img"
      aria-label={ariaLabel}
      options={{
        responsive: true,
        maintainAspectRatio: false,
        rotation: -90,
        circumference: 180,
        cutout: "74%",
        animation: reducedMotion() ? false : { duration: 500 },
        plugins: { legend: { display: false }, tooltip: { enabled: false } },
      }}
      data={{
        labels: ["score", "rest"],
        datasets: [{ data: [v, 100 - v], backgroundColor: [CHART_COLORS.net, CHART_COLORS.track], borderWidth: 0, borderRadius: 4 }],
      }}
    />
  );
}
