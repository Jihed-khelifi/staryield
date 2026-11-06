// Figma 482:2275 — Astrology Results, long-form section (Spanish).
import type { ArticleContent } from "@/content/calculators/types";

const content: ArticleContent = {
  kind: "astrology",
  label: "Comprender su carta natal",
  sections: [
    {
      id: "natal-chart-vs-sun-sign",
      blocks: [
        { type: "h2", id: "natal-chart-vs-sun-sign", size: "lg", text: "¿Qué diferencia hay entre una carta natal completa y su signo solar?" },
        {
          type: "p",
          weight: "light",
          text: "La mayoría de los horóscopos diarios se centran únicamente en su signo solar: la constelación por la que pasaba el Sol en el momento de su nacimiento. Aunque su signo solar representa su identidad esencial, su energía vital y su ego consciente, no es más que una parte de una instantánea cósmica mucho más amplia.",
        },
        {
          type: "p",
          weight: "light",
          text: "Una carta natal completa (u horóscopo) sitúa las posiciones geocéntricas exactas de los diez grandes cuerpos celestes (Sol, Luna, Mercurio, Venus, Marte, Júpiter, Saturno, Urano, Neptuno y Plutón) en la rueda de 360 grados del zodiaco. Además, una carta completa calcula los ángulos del espacio local —en concreto el Ascendente (signo ascendente) y el Medio Cielo—, que dividen su carta en doce ámbitos vitales distintos llamados casas astrológicas.",
        },
        {
          type: "ul",
          items: [
            "Signo solar: su propósito esencial y su voluntad consciente.",
            "Signo lunar: su mundo emocional interior, sus instintos automáticos y sus necesidades psicológicas.",
            "Signo ascendente (Ascendente): el filtro, el aspecto físico y la perspectiva principal con la que se desenvuelve en el mundo exterior.",
          ],
        },
      ],
    },
    {
      id: "birth-time-and-location",
      blocks: [
        { type: "h2", id: "birth-time-and-location", size: "lg", text: "Por qué importan la hora y el lugar exactos de nacimiento" },
        {
          type: "p",
          weight: "semibold",
          text: "Para calcular una carta natal precisa se necesitan tres coordenadas: su fecha de nacimiento, su lugar de nacimiento y su hora de nacimiento exacta al minuto.",
        },
        {
          type: "p",
          weight: "light",
          text: "Como la Tierra gira 360 grados sobre su eje cada 24 horas, el cielo local avanza aproximadamente 1 grado cada 4 minutos. Esta rotación determina el cálculo de su Ascendente: el grado del zodiaco que se elevaba por el horizonte este en el minuto exacto de su nacimiento.",
        },
        {
          type: "formula",
          formula: "ascendant",
          label: "La tangente del Ascendente es igual a menos el coseno de RAMC, dividido entre el seno de RAMC por el coseno de épsilon más la tangente de fi por el seno de épsilon.",
        },
        {
          type: "p",
          weight: "semibold",
          text: "Si su hora de nacimiento tiene un error de tan solo 15 o 20 minutos, su Ascendente calculado puede pasar a un signo completamente distinto, desplazando las doce cúspides de las casas y situando incorrectamente sus planetas natales.",
        },
      ],
    },
    {
      id: "house-systems",
      blocks: [
        { type: "h2", id: "house-systems", size: "lg", text: "Comprender los sistemas de casas: Placidus frente a Signos Enteros" },
        {
          type: "p",
          weight: "semibold",
          text: "Nuestro motor de cartas natales le permite elegir cómo se divide en casas el espacio que rodea su lugar de nacimiento. Los dos sistemas más utilizados son:",
        },
        {
          type: "p",
          weight: "light",
          text: "**Sistema de casas Placidus:** El sistema estándar de la astrología occidental moderna. Divide los arcos diurno y nocturno de los cuerpos celestes en segmentos temporales iguales. Como se basa en arcos de tiempo, el tamaño de las casas varía y, en latitudes extremas, a veces aparecen «signos interceptados».",
        },
        {
          type: "p",
          weight: "light",
          text: "**Sistema de casas de Signos Enteros:** El sistema principal de la astrología helenística tradicional. El signo que contiene su Ascendente se convierte en toda la Casa 1 (de 0° a 30°), y cada signo siguiente forma la casa siguiente en orden. Así se obtienen casas iguales de 30° en toda la carta.",
        },
      ],
    },
    {
      id: "faq",
      blocks: [
        { type: "h2", id: "faq", size: "lg", text: "Preguntas frecuentes" },
        {
          type: "faq",
          items: [
            {
              q: "¿Y si no conozco mi hora exacta de nacimiento?",
              a: "Si no conoce su hora de nacimiento, seleccione la opción «Hora desconocida» en nuestra calculadora. El motor fijará su carta a las 12:00 (mediodía), hora local. Aunque así se obtienen posiciones exactas de los planetas por signo (como sus signos de Sol y Venus), no se calcularán su signo ascendente ni las cúspides concretas de las casas.",
            },
            {
              q: "¿Qué es un aspecto en una carta natal?",
              a: "Los aspectos son ángulos geométricos que forman dos o más planetas medidos a lo largo de la rueda de la eclíptica. Los aspectos principales son la conjunción (0 grados), el sextil (60 grados), la cuadratura (90 grados), el trígono (120 grados) y la oposición (180 grados). Indican cómo interactúan entre sí las distintas facetas de su personalidad.",
            },
            {
              q: "¿Por qué distintos sitios de astrología muestran posiciones de casas diferentes?",
              a: "Las diferencias se deben a que cada sitio web utiliza un sistema de cálculo de casas distinto (como Placidus o Signos Enteros). Elija su sistema preferido en nuestros ajustes de cálculo avanzados.",
            },
          ],
        },
      ],
    },
  ],
};

export default content;
