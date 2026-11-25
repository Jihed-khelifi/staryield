// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";

import type { Locale } from "@/i18n/config";
import copyEn from "@/i18n/dictionaries/copy-en.json";
import copyEs from "@/i18n/dictionaries/copy-es.json";
import copyFr from "@/i18n/dictionaries/copy-fr.json";
import en from "@/i18n/dictionaries/en.json";
import es from "@/i18n/dictionaries/es.json";
import fr from "@/i18n/dictionaries/fr.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import { axeViolations } from "@/test/render";

import Disclaimer from "@/components/content/disclaimer";

afterEach(() => cleanup());

const dictionaries: Record<Locale, Dictionary> = {
  en: { ...en, copy: copyEn } as Dictionary,
  fr: { ...fr, copy: copyFr } as Dictionary,
  es: { ...es, copy: copyEs } as Dictionary,
};
const locales = ["en", "fr", "es"] as const;

function renderIn(locale: Locale) {
  return render(
    <I18nProvider locale={locale} dictionary={dictionaries[locale]}>
      <Disclaimer />
    </I18nProvider>,
  );
}

const SECTION = "free-tools-shared-results-and-messages";
/** Stable anchors of the growth-release clauses (e.g. /en/disclaimer#zodiac-compatibility). */
const CLAUSES = ["natal-chart", "tarot-birth-cards", "life-path-numbers", "zodiac-compatibility", "sharing-results-and-invitations", "informational-messages"];

const sectionOf = (container: HTMLElement) => {
  const section = container.querySelector<HTMLElement>(`section#${SECTION}`);
  if (!section) throw new Error("no free tools section");
  return section;
};
const clauseText = (container: HTMLElement, id: string) => container.querySelector(`#${id}`)?.textContent ?? "";

/** What each clause must say, per locale: entertainment-only, symbolic, not a prediction, and the facts the code keeps. */
const FACTS: Record<Locale, Record<string, RegExp[]>> = {
  en: {
    [SECTION]: [/for entertainment purposes only/, /not predictions/, /medical, psychological, legal, financial, relationship or any other form of professional advice/],
    "natal-chart": [/full chart free of charge/, /rising sign and the houses need both a birth time and a birthplace/, /not statements of fact about you/, /do not predict events/],
    "tarot-birth-cards": [/Mary K\. Greer/, /one, two or three of the 22 Major Arcana/, /for some dates, a card in between/, /Rider–Waite–Smith/, /Golden Dawn/, /birth date alone/, /not who you are or what will happen to you/],
    "life-path-numbers": [/Pythagorean numerology/, /master numbers 11, 22 and 33/, /symbolic/],
    "zodiac-compatibility": [/two sun signs only/, /love, friendship and work scores/, /fixed rule/, /traditional-astrology heuristic, not a measurement/, /not based on data about real couples/, /cannot tell whether a romantic relationship, a friendship or a working relationship will work/, /not relationship advice/],
    "sharing-results-and-invitations": [/not a prediction, a reading by an advisor or advice/, /must not promise predictions or outcomes/, /not responsible for what users write or post/, /for entertainment purposes only/],
    "informational-messages": [/how many advisors are online now/, /describe the moment the message was sent/, /does not guarantee that an advisor will be available/, /not readings or predictions/],
  },
  fr: {
    [SECTION]: [/uniquement à des fins de divertissement/, /ce ne sont pas des prédictions/, /conseil médical, psychologique, juridique, financier, relationnel/],
    "natal-chart": [/gratuitement le thème complet/, /L’ascendant et les maisons nécessitent à la fois une heure et un lieu de naissance/, /ne prédisent aucun événement/],
    "tarot-birth-cards": [/Mary K\. Greer/, /une, deux ou trois des 22 cartes des arcanes majeurs/, /pour certaines dates, une carte intermédiaire/, /Rider–Waite–Smith/, /Golden Dawn/, /uniquement de la date de naissance/],
    "life-path-numbers": [/numérologie pythagoricienne/, /nombres maîtres 11, 22 et 33/, /symboliques/],
    "zodiac-compatibility": [/deux signes solaires/, /scores d’amour, d’amitié et de travail/, /règle fixe/, /heuristique de l’astrologie traditionnelle, et non d’une mesure/, /aucune donnée concernant de vrais couples/, /ne peuvent pas dire si une relation/],
    "sharing-results-and-invitations": [/ni une prédiction, ni une consultation avec un conseiller, ni un conseil/, /ne devez ni promettre de prédiction/, /pas responsables de ce que les utilisateurs écrivent ou publient/, /à des fins de divertissement/],
    "informational-messages": [/combien de conseillers sont en ligne/, /au moment de l’envoi/, /ne garantit donc pas qu’un conseiller sera disponible/, /ni des consultations ni des prédictions/],
  },
  es: {
    [SECTION]: [/únicamente con fines de entretenimiento/, /no son predicciones/, /asesoramiento médico, psicológico, jurídico, financiero, sentimental/],
    "natal-chart": [/gratis la carta completa/, /El ascendente y las casas requieren tanto la hora como el lugar de nacimiento/, /no predicen acontecimientos/],
    "tarot-birth-cards": [/Mary K\. Greer/, /una, dos o tres de las 22 cartas de los arcanos mayores/, /en algunas fechas, una carta intermedia/, /Rider–Waite–Smith/, /Golden Dawn/, /únicamente de la fecha de nacimiento/],
    "life-path-numbers": [/numerología pitagórica/, /números maestros 11, 22 y 33/, /simbólicos/],
    "zodiac-compatibility": [/dos signos solares/, /puntuaciones de amor, amistad y trabajo/, /regla fija/, /heurística de la astrología tradicional, no una medición/, /no se basan en datos sobre parejas reales/, /no pueden decir si una relación/],
    "sharing-results-and-invitations": [/no es una predicción, ni una consulta con un asesor, ni asesoramiento/, /no debe prometer predicciones/, /No somos responsables de lo que los usuarios escriben o publican/, /con fines de entretenimiento/],
    "informational-messages": [/cuántos asesores están en línea/, /el momento en que se envió el mensaje/, /no garantiza que un asesor vaya a estar disponible/, /no son consultas ni predicciones/],
  },
};

describe("Disclaimer page: free calculators, guides, shared results and messages", () => {
  it.each(locales)("%s: the section and its clauses keep the same anchors in every locale", (locale) => {
    const { container } = renderIn(locale);
    const section = sectionOf(container);
    const heading = section.querySelector("h2");
    expect(heading?.id).toBe(`${SECTION}-title`);
    expect(section.getAttribute("aria-labelledby")).toBe(heading?.id);
    expect([...section.querySelectorAll("p[id]")].map((p) => p.id)).toEqual(CLAUSES);
  });

  it.each(locales)("%s: every clause says what the product does and that it is for entertainment only", (locale) => {
    const { container } = renderIn(locale);
    for (const [id, facts] of Object.entries(FACTS[locale])) {
      const text = id === SECTION ? (sectionOf(container).querySelector("p:not([id])")?.textContent ?? "") : clauseText(container, id);
      for (const fact of facts) expect(text, `${locale} #${id}`).toMatch(fact);
    }
  });

  it.each(["fr", "es"] as const)("%s: the new section is fully translated", (locale) => {
    const { container } = renderIn(locale);
    const english = sectionOf(renderIn("en").container).textContent ?? "";
    const translated = sectionOf(container).textContent ?? "";
    expect(translated).not.toBe(english);
    expect(translated).not.toMatch(/entertainment purposes|not predictions|sun signs|advisors are online|Pythagorean|for reflection/);
  });

  it.each(locales)("%s: the new section makes no accuracy, certification, healing or therapy claim", (locale) => {
    const { container } = renderIn(locale);
    const text = sectionOf(container).textContent ?? "";
    expect(text).not.toMatch(/accura|exactitud|certif|verifi|vérifi|vetted|healing|guéri|curaci|sanaci|therap|thérap|terap|life-changing|scientifically/i);
  });

  it("keeps the existing Disclaimer text before the new section", () => {
    const { container } = renderIn("en");
    const text = container.textContent ?? "";
    const existing = [
      "THIS FULL DISCLAIMER",
      "You must be at least eighteen (18) years old",
      "entertainment purposes only",
      "You are solely responsible for selecting an advisor (psychic)",
      "Advisors (psychics) are not employees of the Company.",
      "The Company makes no representations, warranties, or guarantees",
      "is limited by the terms and conditions of the Terms.",
    ];
    const section = text.indexOf("Free Calculators, Guides, Shared Results and Messages");
    expect(section).toBeGreaterThan(0);
    for (const sentence of existing) {
      expect(text).toContain(sentence);
      expect(text.indexOf(sentence)).toBeLessThan(section);
    }
  });

  it("the legal content card grows with its content instead of a fixed Figma height", () => {
    const { container } = renderIn("en");
    const card = container.querySelector('[data-node-id="796:2324"]');
    expect(card?.className).toContain("min-h-[1440px]");
    expect(card?.className).not.toMatch(/(^|\s)h-\[1440px\]/);
  });

  it("has a single h1, an h2 for the new section and no serious accessibility violation", async () => {
    const { container } = renderIn("en");
    expect(container.querySelectorAll("h1")).toHaveLength(1);
    expect(container.querySelectorAll("h2")).toHaveLength(1);
    expect(await axeViolations(container)).toEqual([]);
  });
});
