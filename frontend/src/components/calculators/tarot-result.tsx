"use client";
/* eslint-disable @next/next/no-img-element */
import * as React from "react";

import { useI18n } from "@/i18n/i18n-provider";
import { birthCard, type BirthCard, type TarotBirthResult } from "@/lib/astro/tarot-birth-card";
import { TAROT_BIRTH_COPY } from "@/lib/astro/tarot-birth-copy";
import { cardNumeral, chainLabel, constellationPeers, tarotBirthSteps } from "@/lib/astro/tarot-birth-steps";
import { usePsychicsHref } from "@/lib/psychics-link";
import { AdvisorCta, Lock, StarHeading } from "@/components/calculators/results";
import { TAROT_BIRTH_NOTICE } from "@/components/calculators/birth-data-notice";
import { cn } from "@/lib/utils";

/** Which role a card plays in the chain from the personality card to the soul card (English source copy). */
export function roleOf(index: number, count: number): string {
  if (count === 1) return "Your birth card";
  if (index === 0) return "Personality card";
  if (index === count - 1) return "Soul card";
  return "Bridge card";
}

/**
 * Tarot birth card: the card (or cards) with its artwork, what it means, how
 * the date adds up and the constellation of cards that share the soul card.
 * Computed in the browser from the birth date alone.
 */
export function TarotBirthResultView({ result, date }: { result: TarotBirthResult; date: string }) {
  const { text } = useI18n();
  const psychicsHref = usePsychicsHref();
  const cards = result.birthCards;
  const first = cards[0];
  const soul = cards[cards.length - 1];
  const many = cards.length > 1;
  // The other chains that end in the same soul card ("10/1" for a 19/10/1).
  const constellation = constellationPeers(first.number, soul.number);
  return (
    <>
      <section className="flex flex-col items-center gap-8 bg-cream px-5 pt-14 pb-14 text-center md:gap-10 md:px-20 md:pt-20 md:pb-20" aria-labelledby="tarot-title" data-testid="tarot-birth-cards">
        <div id="tarot-title">
          <StarHeading>{text(many ? "Your tarot birth cards" : "Your tarot birth card")}</StarHeading>
        </div>
        <ul className="flex flex-wrap items-start justify-center gap-6 md:gap-10">
          {cards.map((card, index) => (
            <li key={card.number} className="flex flex-col items-center gap-3" data-testid="birth-card" data-card={card.slug} data-role={roleOf(index, cards.length)}>
              <img
                src={card.image}
                alt={text(card.name)}
                width={210}
                height={355}
                className={cn("h-auto rounded-xl border-2 border-gold shadow-md", many ? "w-[min(40vw,190px)] md:w-[210px]" : "w-[min(60vw,240px)] md:w-[260px]")}
              />
              <p className="font-display text-2xl tracking-[2px] text-[#8a6a1f]" data-testid="birth-card-numeral">
                {cardNumeral(card)}
              </p>
              <p className="font-display text-sm tracking-[1.6px] uppercase">{text(roleOf(index, cards.length))}</p>
            </li>
          ))}
        </ul>
        <div className="grid max-w-3xl gap-3">
          <p className="font-display text-[24px] leading-snug tracking-[0.6px] uppercase md:text-[34px]" data-testid="tarot-headline">
            {text("Your tarot birth card is {value0}", { value0: text(first.name) })}
          </p>
          <p className="text-base leading-normal font-light md:text-lg">
            {many
              ? text("Your personality card, {value0}, is how you meet the world. Your soul card, {value1}, is the deeper purpose you grow toward.", {
                  value0: text(first.name),
                  value1: text(soul.name),
                })
              : text("You have a single birth card, so your personality and your soul share the same energy.")}
          </p>
        </div>
      </section>

      <section className="calc-olive px-5 py-12 text-white md:px-[120px] md:py-20" data-tone="dark" style={{ "--calc-texture": 0.35 } as React.CSSProperties} aria-labelledby="tarot-meaning-title">
        <div className="mx-auto flex max-w-[1200px] flex-col gap-10 md:px-12">
          <h2 id="tarot-meaning-title" className="font-display text-[26px] tracking-[0.56px] uppercase md:text-[28px]">
            {text(many ? "What your birth cards mean" : "What your birth card means")}
          </h2>
          {cards.map((card, index) => {
            const copy = TAROT_BIRTH_COPY[card.number];
            return (
              <article key={card.number} className="grid gap-2" data-testid="birth-card-meaning" data-card={card.slug}>
                <h3 className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
                  <span className="font-display text-[22px] tracking-[0.5px] uppercase md:text-2xl">
                    {cardNumeral(card)} · {text(card.name)}
                  </span>
                  <span className="text-sm font-light tracking-[0.28px]">{text(roleOf(index, cards.length))}</span>
                </h3>
                {copy ? (
                  <>
                    <p className="text-sm font-semibold tracking-[0.3px]">{text(copy.keywords)}</p>
                    <p className="text-[17px] leading-[1.6] font-light md:text-lg">{text(copy.meaning)}</p>
                  </>
                ) : null}
              </article>
            );
          })}
        </div>
      </section>

      <section className="calc-paper flex flex-col items-center gap-10 px-5 py-14 text-center md:px-[120px] md:py-20" aria-labelledby="tarot-how-title">
        <div className="grid max-w-3xl gap-3">
          <h2 id="tarot-how-title" className="font-display text-[22px] tracking-[0.56px] uppercase md:text-[28px]">
            {text("How it adds up")}
          </h2>
          <p className="text-base leading-normal font-light">
            {text("We add your birth month, day and year, then keep adding the digits of the total until it is 22 or less. The cards run from your personality card to your soul card.")}
          </p>
          <p className="font-display text-lg tracking-[0.5px] text-[#8a6a1f] md:text-xl" data-testid="tarot-steps">
            {tarotBirthSteps(date, result)}
          </p>
          {many ? (
            <p className="text-sm font-light" data-testid="tarot-chain">
              {cards.map((card) => `${card.number} ${text(card.name)}`).join(" → ")}
            </p>
          ) : null}
        </div>

        {constellation.length > 0 ? (
          <div className="grid max-w-3xl gap-3" data-testid="tarot-constellation">
            <h2 className="font-display text-[22px] tracking-[0.56px] uppercase md:text-[28px]">{text("Your constellation")}</h2>
            <p className="text-base leading-normal font-light">
              {text("Birth cards that end in the same soul card form a constellation. They share the same deeper lesson.")}
            </p>
            <p className="text-base font-semibold">{text("Others with {value0} as soul card:", { value0: text(soul.name) })}</p>
            <ul className="grid gap-1 text-base font-light">
              {constellation.map((chain) => (
                <li key={chainLabel(chain)} data-chain={chainLabel(chain)}>
                  <span className="font-semibold">{chainLabel(chain)}</span>
                  <span aria-hidden="true"> · </span>
                  {chain
                    .map((n) => birthCard(n))
                    .filter((c): c is BirthCard => c !== null)
                    .map((c) => text(c.name))
                    .join(" → ")}
                </li>
              ))}
            </ul>
          </div>
        ) : null}
        <Lock>{text(TAROT_BIRTH_NOTICE)}</Lock>
      </section>

      <AdvisorCta
        title={text("Talk to an advisor about your card")}
        body={text("Chat with one of our psychic advisors about what your birth card means for you, for entertainment purposes only.")}
        href={psychicsHref}
        placement="calculator_tarot_birth_card"
      />
    </>
  );
}
