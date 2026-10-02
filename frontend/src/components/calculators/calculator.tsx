"use client";
import { TarotSpread } from "./tarot-spread";
import { useState } from "react";
import Link from "next/link";
import { LockKeyhole } from "lucide-react";
import { useI18n } from "@/i18n/i18n-provider";
import { PublicFrame } from "@/components/layout/public-frame";
import { RadiantPanel } from "@/components/forms/radiant-panel";
import { ZodiacWheel } from "@/components/astrology/ZodiacWheel";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  lifePath,
  personalCycles,
  validBirthDate,
  zodiacForDate,
} from "@/lib/calculators";

export type CalculatorKind = "natal-chart" | "path-of-life" | "astrology";
const descriptions = {
  "natal-chart":
    "Discover your cosmic blueprint — enter your birth date, time, and location to generate a personalized natal chart.",
  "path-of-life":
    "Your birth date holds the mathematical blueprint of your destiny. We use it to calculate your Life Path number—the primary force shaping your journey.",
  astrology:
    "Discover your zodiac sign instantly — enter your birth date and uncover the traits, elements, and cosmic insights tied to your star sign.",
};
export { TarotSpread } from "./tarot-spread";
export function Calculator({
  kind,
  showResults = false,
}: {
  kind: CalculatorKind;
  showResults?: boolean;
}) {
  const { text, locale } = useI18n();
  const [birth, setBirth] = useState(showResults ? "1945-04-30" : "");
  const [error, setError] = useState("");
  const [submitted, setSubmitted] = useState(showResults);
  const [day, month, year] = birth
    ? birth.split("-").reverse().map(Number)
    : [0, 0, 0];
  const { personalYear, personalMonth } = personalCycles(birth);
  const zodiac = birth ? zodiacForDate(month, day) : "Gemini";
  function calculate(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const d = Number(data.get("day")),
      m = Number(data.get("month")),
      y = Number(data.get("year"));
    if (!validBirthDate(d, m, y)) {
      setError("Enter a valid birth date that is not in the future.");
      return;
    }
    setBirth(
      `${y}-${String(m).padStart(2, "0")}-${String(d).padStart(2, "0")}`,
    );
    setSubmitted(true);
    setError("");
    window.scrollTo({ top: 0, behavior: "instant" });
  }
  return (
    <PublicFrame>
      {!submitted ? (
        <RadiantPanel>
          <form className="calculator-card paper-page" onSubmit={calculate}>
            <h1 className="text-center text-4xl">{text("Date of Birth")}</h1>
            <p className="mt-4 text-center text-sm font-light leading-relaxed">
              {text(descriptions[kind])}
            </p>
            <div className="my-8 grid grid-cols-3 gap-4">
              {[
                ["Day", "day", "DD", 1, 31],
                ["Month", "month", "MM", 1, 12],
                ["Year", "year", "YYYY", 1900, new Date().getFullYear()],
              ].map(([label, name, placeholder, min, max]) => (
                <label className="field text-xs text-sage" key={name}>
                  {text(label)}
                  <Input
                    name={String(name)}
                    type="number"
                    inputMode="numeric"
                    placeholder={text(String(placeholder))}
                    min={Number(min)}
                    max={Number(max)}
                    required
                    className="h-10 bg-warm-gray/25"
                  />
                </label>
              ))}
            </div>
            <p role="alert" className="mb-3 text-sm text-red-800">
              {text(error)}
            </p>
            <Button className="gold-button h-12 w-full" type="submit">
              {text("Reveal your path ")}
            </Button>
            <p className="mt-4 flex items-center justify-center gap-2 text-xs text-sage">
              <LockKeyhole size={12} />
              {text("Your data is calculated locally & never stored. ")}
            </p>
          </form>
        </RadiantPanel>
      ) : (
        <div className="calculator-results">
          <section className="result-date">
            <h1 className="font-display text-2xl md:text-[32px]">
              {text("Natal alignment calculated for: ")}
            </h1>
            <button
              type="button"
              className="result-date-field gold-button"
              onClick={() => setSubmitted(false)}
              aria-label={text("Change birth date")}
            >
              {text(String(day).padStart(2, "0"))} /{" "}
              {text(String(month).padStart(2, "0"))} / {year}
            </button>
            {showResults && (
              <span className="sr-only">
                {text(
                  "Sample date from the design. Change the birth date to calculate your own result. ",
                )}
              </span>
            )}
          </section>
          <section className="result-art">
            <h2 className="font-display text-[32px] md:text-[40px]">
              {text("✦ Your")}{" "}
              {text(
                kind === "natal-chart"
                  ? "spread"
                  : kind === "path-of-life"
                    ? "numbers"
                    : "zodiac",
              )}{" "}
              ✦
            </h2>
            {kind === "natal-chart" ? (
              <>
                <p className="mt-3 text-sm">
                  {text(
                    "A 12-card spread exploring the current landscape, themes, and guidance for your journey. ",
                  )}
                </p>
                <TarotSpread />
                <p className="text-xs text-sage">
                  {text(
                    "Illustrative tarot spread. A planetary birth chart requires the astrology service. ",
                  )}
                </p>
              </>
            ) : kind === "path-of-life" ? (
              <div className="number-tiles">
                {[
                  [
                    "Life Path",
                    lifePath(birth),
                    "The foundational blueprint of your earthly journey, structuring your natural potential and ultimate purpose.",
                  ],
                  [
                    "Personal year",
                    personalYear,
                    "The overarching theme and energetic vibration defining your current twelve-month developmental cycle.",
                  ],
                  [
                    "Personal month",
                    personalMonth,
                    "Your immediate energetic climate, highlighting focused opportunities and acute lessons for this cycle.",
                  ],
                ].map(([label, number, copy]) => (
                  <article className="number-tile" key={label}>
                    <h3 className="font-display text-sm tracking-wider">
                      {text(label)}
                    </h3>
                    <p className="my-4 font-display text-[72px] text-amber">
                      {text(number)}
                    </p>
                    <p className="text-sm font-light">{text(copy)}</p>
                  </article>
                ))}
              </div>
            ) : (
              <div className="zodiac-wheel-viewport">
                <ZodiacWheel sign={zodiac} />
              </div>
            )}
          </section>
          {kind === "path-of-life" && (
            <section className="result-insight paper-page">
              <h2 className="font-display text-[28px]">
                {text("Life path ")}
                {lifePath(birth)}
              </h2>
              <p className="mt-4">
                {text(
                  lifePath(birth) === 6
                    ? "Deeply responsible, prioritizing family, domestic life, and acts of service. Life Path 6s are the ultimate caregivers. Their dark side is a tendency to become self-righteous meddlers or professional martyrs."
                    : "Your life path number is calculated by reducing the digits of your birth date, preserving the master numbers 11, 22, and 33.",
                )}
              </p>
            </section>
          )}
          {kind === "astrology" && (
            <section className="result-insight paper-page">
              <h2 className="text-[28px] font-semibold">
                {text(
                  "According to your alignment, the sun resides in the sign of",
                )}{" "}
                <span className="font-display text-[48px]">{text(zodiac)}</span>
              </h2>
              <p className="mt-4 text-lg leading-relaxed">
                {text(
                  zodiac === "Gemini"
                    ? "This highlighted house reveals your native energy flow, communicative versatility, and intellectual curiosity. Gemini is ruled by Mercury and belongs to the Air element."
                    : "The highlighted sign shows your sun sign based on your birth date. A complete natal chart also considers the time and place of birth.",
                )}
              </p>
            </section>
          )}
          <section className="result-cta paper-page">
            <h2 className="mx-auto max-w-4xl font-display text-2xl">
              {text(
                kind === "astrology"
                  ? "Want to discover your planetary coordinates?"
                  : kind === "natal-chart"
                    ? "Get a better understanding of what this spread means with the guidance of one of our amazing psychics!"
                    : "Get a deeper understanding of what a life path really means and the alignment that comes with it",
              )}
            </h2>
            <p className="mx-auto mt-4 max-w-2xl text-sm font-light">
              {text(
                "Speak with our certified, strategic psychic advisors to map out your life alignment metrics, address upcoming planetary transitions, and clear your karmic debt with complete confidence. ",
              )}
            </p>
            <Button asChild className="gold-button mt-6 h-11 w-full max-w-sm">
              <Link
                href={`/${locale}/${kind === "astrology" ? "signup" : "psychics"}`}
              >
                {text(kind === "astrology" ? "Sign in / up" : "Click here!")}
              </Link>
            </Button>
          </section>
        </div>
      )}
    </PublicFrame>
  );
}
