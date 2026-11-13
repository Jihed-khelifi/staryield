// Figma 482:2275 — Astrology Results, long-form section (French).
import type { ArticleContent } from "@/content/calculators/types";

const content: ArticleContent = {
  kind: "astrology",
  label: "Comprendre votre thème natal",
  sections: [
    {
      id: "natal-chart-vs-sun-sign",
      blocks: [
        { type: "h2", id: "natal-chart-vs-sun-sign", size: "lg", text: "Thème natal complet ou signe solaire : quelle différence ?" },
        {
          type: "p",
          weight: "light",
          text: "La plupart des horoscopes quotidiens ne s’intéressent qu’à votre signe solaire : la constellation que le Soleil traversait au moment de votre naissance. Si votre signe solaire représente votre identité profonde, votre énergie vitale et votre moi conscient, il n’est qu’un élément d’un instantané cosmique bien plus vaste.",
        },
        {
          type: "p",
          weight: "light",
          text: "Un thème natal complet (ou horoscope) situe les positions géocentriques exactes des dix grands corps célestes (Soleil, Lune, Mercure, Vénus, Mars, Jupiter, Saturne, Uranus, Neptune et Pluton) sur la roue de 360 degrés du zodiaque. En outre, un thème complet calcule les angles de l’espace local — en particulier l’Ascendant (signe ascendant) et le Milieu du Ciel —, qui divisent votre thème en douze domaines de vie distincts appelés maisons astrologiques.",
        },
        {
          type: "ul",
          items: [
            "Signe solaire : votre raison d’être profonde et votre volonté consciente.",
            "Signe lunaire : votre paysage émotionnel intérieur, vos réactions instinctives et vos besoins psychologiques.",
            "Signe ascendant (Ascendant) : le filtre, l’apparence physique et le point de vue principal à travers lesquels vous évoluez dans le monde extérieur.",
          ],
        },
      ],
    },
    {
      id: "birth-time-and-location",
      blocks: [
        { type: "h2", id: "birth-time-and-location", size: "lg", text: "Pourquoi l’heure et le lieu exacts de naissance comptent" },
        {
          type: "p",
          weight: "semibold",
          text: "Le calcul d’un thème natal précis repose sur trois coordonnées : votre date de naissance, votre lieu de naissance et votre heure de naissance exacte à la minute près.",
        },
        {
          type: "p",
          weight: "light",
          text: "Comme la Terre effectue une rotation de 360 degrés sur son axe en 24 heures, le ciel local avance d’environ 1 degré toutes les 4 minutes. Cette rotation détermine le calcul de votre Ascendant : le degré du zodiaque qui se levait à l’horizon est à la minute exacte de votre naissance.",
        },
        {
          type: "formula",
          formula: "ascendant",
          label: "La tangente de l’Ascendant est égale à moins le cosinus de RAMC, divisé par le sinus de RAMC fois le cosinus d’epsilon plus la tangente de phi fois le sinus d’epsilon.",
        },
        {
          type: "p",
          weight: "semibold",
          text: "Si votre heure de naissance est inexacte de seulement 15 à 20 minutes, votre Ascendant calculé peut basculer dans un tout autre signe, ce qui décale les douze cuspides des maisons et place vos planètes natales au mauvais endroit.",
        },
      ],
    },
    {
      id: "house-systems",
      blocks: [
        { type: "h2", id: "house-systems", size: "lg", text: "Comprendre les systèmes de maisons : Placidus ou signes entiers" },
        {
          type: "p",
          weight: "semibold",
          text: "Notre moteur de thème natal vous permet de choisir la manière dont l’espace autour de votre lieu de naissance est divisé en maisons. Les deux systèmes les plus utilisés sont :",
        },
        {
          type: "p",
          weight: "light",
          text: "**Système de maisons Placidus :** Le système de référence de l’astrologie occidentale moderne. Il divise les arcs diurne et nocturne des corps célestes en segments de temps égaux. Comme il repose sur des arcs de temps, la taille des maisons varie et produit parfois des « signes interceptés » aux latitudes extrêmes.",
        },
        {
          type: "p",
          weight: "light",
          text: "**Système des maisons en signes entiers :** Le principal système de l’astrologie hellénistique traditionnelle. Le signe qui contient votre Ascendant devient la maison 1 tout entière (de 0° à 30°), et chaque signe suivant forme la maison suivante, dans l’ordre. On obtient ainsi des maisons égales de 30° sur l’ensemble du thème.",
        },
      ],
    },
    {
      id: "faq",
      blocks: [
        { type: "h2", id: "faq", size: "lg", text: "Questions fréquentes" },
        {
          type: "faq",
          items: [
            {
              q: "Et si je ne connais pas mon heure exacte de naissance ?",
              a: "Si vous ne connaissez pas votre heure de naissance, sélectionnez l’option « Heure inconnue » dans notre calculateur. Le moteur calculera votre thème pour 12 h 00 (midi), heure locale. Vous obtiendrez ainsi des positions des planètes par signe (comme vos signes du Soleil et de Vénus), mais votre signe ascendant et les cuspides précises des maisons ne seront pas calculés.",
            },
            {
              q: "Qu’est-ce qu’un aspect dans un thème natal ?",
              a: "Les aspects sont des angles géométriques formés entre deux planètes ou plus, mesurés le long de la roue de l’écliptique. Les aspects majeurs sont la conjonction (0 degré), le sextile (60 degrés), le carré (90 degrés), le trigone (120 degrés) et l’opposition (180 degrés). Ils indiquent comment les différentes facettes de votre personnalité interagissent entre elles.",
            },
            {
              q: "Pourquoi des sites d’astrologie différents affichent-ils des positions de maisons différentes ?",
              a: "Ces écarts viennent de l’utilisation de systèmes de calcul des maisons différents selon les sites (par exemple Placidus ou signes entiers). Choisissez votre système préféré dans nos paramètres de calcul avancés.",
            },
          ],
        },
      ],
    },
  ],
};

export default content;
