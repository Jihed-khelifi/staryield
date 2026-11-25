// Tarot birth card calculator — long-form section under the result (Spanish).
import type { ArticleContent } from "@/content/calculators/types";
import { tarotBirthIntroImage } from "@/content/calculators/tarot-birth-card/en";

const content: ArticleContent = {
  kind: "tarot-birth-card",
  label: "Comprender tu carta de nacimiento del tarot",
  sections: [
    {
      id: "what-is-a-tarot-birth-card",
      blocks: [
        {
          type: "split",
          blocks: [
            { type: "h2", id: "what-is-a-tarot-birth-card", text: "¿Qué es la carta de nacimiento del tarot?" },
            {
              type: "p",
              text: "Tu carta de nacimiento es el arcano mayor que corresponde a los números de tu fecha de nacimiento. El método lo popularizó la autora de tarot Mary K. Greer, que unió la numerología con las 22 cartas de los arcanos mayores: sumas tu fecha de nacimiento, reduces el total y lees la carta resultante como símbolo de las cualidades y las lecciones que te acompañan a lo largo de la vida.",
            },
            {
              type: "p",
              weight: "light",
              text: "La mayoría de las fechas dan dos cartas: una **carta de personalidad**, que describe cómo te relacionas con el mundo, y una **carta del alma**, que señala tu propósito más profundo. Algunas fechas dan una sola carta que cumple ambos papeles, y una cadena, 19/10/1, da tres. Como todas las lecturas de Staryield, tu carta de nacimiento se ofrece para la reflexión y el entretenimiento, no como una predicción.",
            },
          ],
          image: { ...tarotBirthIntroImage, alt: "La carta del tarot La Estrella" },
        },
      ],
    },
    {
      id: "how-to-calculate",
      blocks: [
        { type: "h2", id: "how-to-calculate", text: "Cómo calcular tu carta de nacimiento del tarot" },
        {
          type: "ol",
          items: [
            "Suma el mes, el día y el año de tu nacimiento como números enteros (por ejemplo, 11 + 29 + 1990).",
            "Suma los dígitos del total. Si el resultado es mayor que 22, vuelve a sumar sus dígitos hasta obtener 22 o menos.",
            "El número que obtienes es tu carta de personalidad: el 1 es El Mago, el 2 La Sacerdotisa y así sucesivamente hasta el 21, El Mundo. Un resultado de 22 corresponde a El Loco, que lleva el número 0 en la baraja.",
            "Si ese número tiene dos dígitos, súmalos, y repite si hace falta, hasta llegar a un solo dígito: esa es tu carta del alma (por ejemplo, 17 ⟹ 1 + 7 = 8, La Fuerza). Un resultado de un solo dígito es a la vez tu carta de personalidad y tu carta del alma.",
          ],
        },
        { type: "h3", id: "worked-example", text: "Ejemplo paso a paso: 29 de noviembre de 1990" },
        {
          type: "ul",
          items: [
            "Suma la fecha: 11 + 29 + 1990 = 2030",
            "Suma los dígitos: 2 + 0 + 3 + 0 = 5",
            "5 tiene un solo dígito, así que El Hierofante (V) es a la vez la carta de personalidad y la carta del alma.",
          ],
        },
        { type: "h3", id: "two-card-example", text: "Dos cartas: 15 de junio de 1987" },
        {
          type: "ul",
          items: [
            "Suma la fecha: 6 + 15 + 1987 = 2008",
            "Suma los dígitos: 2 + 0 + 0 + 8 = 10, así que la carta de personalidad es La Rueda de la Fortuna (X)",
            "Reduce una vez más: 1 + 0 = 1, así que la carta del alma es El Mago (I)",
          ],
        },
        {
          type: "p",
          weight: "light",
          text: "Bajo tu resultado, «Cómo se calcula» detalla cada paso para tu propia fecha, para que puedas comprobar el cálculo tú mismo.",
        },
      ],
    },
    {
      id: "personality-and-soul-cards",
      blocks: [
        { type: "h2", id: "personality-and-soul-cards", text: "Carta de personalidad y carta del alma" },
        {
          type: "p",
          text: "Las cartas funcionan en pareja. El número más alto es tu carta de personalidad: la imagen que muestras, los talentos que usas cada día y tu forma de afrontar los retos. El número de un solo dígito es tu carta del alma: un hilo más discreto de propósito en el que vas creciendo a lo largo de la vida.",
        },
        {
          type: "ul",
          items: [
            "**Una sola carta (del 2 al 9):** tu personalidad y tu alma comparten la misma energía, así que sus temas recorren tu vida con especial intensidad.",
            "**Dos cartas (del 10 al 18, 20, 21 y 22):** la carta de personalidad lleva la iniciativa y la carta del alma, debajo, muestra la lección más profunda. El Loco (22) forma pareja con El Emperador (4).",
            "**Tres cartas (19):** El Sol es la carta de personalidad y El Mago la carta del alma, con La Rueda de la Fortuna (10) como puente entre ambas.",
          ],
        },
      ],
    },
    {
      id: "constellations",
      blocks: [
        { type: "h2", id: "constellations", text: "Las nueve constelaciones del tarot" },
        {
          type: "p",
          text: "Las cartas de nacimiento que terminan en la misma carta del alma forman una constelación, que lleva el nombre de su carta de un solo dígito. Las personas de una misma constelación comparten la misma lección profunda, aunque sus cartas de personalidad sean distintas.",
        },
        {
          type: "twoColumnList",
          items: [
            { title: "Constelación 1", subtitle: "El Mago", body: ["10/1: La Rueda de la Fortuna y El Mago", "19/10/1: El Sol, La Rueda de la Fortuna y El Mago"] },
            { title: "Constelación 2", subtitle: "La Sacerdotisa", body: ["11/2: La Justicia y La Sacerdotisa", "20/2: El Juicio y La Sacerdotisa", "2: La Sacerdotisa sola"] },
            { title: "Constelación 3", subtitle: "La Emperatriz", body: ["12/3: El Colgado y La Emperatriz", "21/3: El Mundo y La Emperatriz", "3: La Emperatriz sola"] },
            { title: "Constelación 4", subtitle: "El Emperador", body: ["13/4: La Muerte y El Emperador", "22/4: El Loco y El Emperador", "4: El Emperador solo"] },
            { title: "Constelación 5", subtitle: "El Hierofante", body: ["14/5: La Templanza y El Hierofante", "5: El Hierofante solo"] },
            { title: "Constelación 6", subtitle: "Los Enamorados", body: ["15/6: El Diablo y Los Enamorados", "6: Los Enamorados solos"] },
            { title: "Constelación 7", subtitle: "El Carro", body: ["16/7: La Torre y El Carro", "7: El Carro solo"] },
            { title: "Constelación 8", subtitle: "La Fuerza", body: ["17/8: La Estrella y La Fuerza", "8: La Fuerza sola"] },
            { title: "Constelación 9", subtitle: "El Ermitaño", body: ["18/9: La Luna y El Ermitaño", "9: El Ermitaño solo"] },
          ],
        },
        {
          type: "p",
          weight: "light",
          text: "Tu resultado muestra las demás cartas de nacimiento de tu constelación, para que veas quién comparte tu carta del alma.",
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
              q: "¿Necesito mi hora de nacimiento para encontrar mi carta de nacimiento del tarot?",
              a: "No. El cálculo usa solo tu fecha de nacimiento, así que la hora y el lugar de tu nacimiento no cambian nada. Tu fecha nunca sale de tu navegador: la carta se calcula en tu dispositivo.",
            },
            {
              q: "¿En qué se diferencia de mi número del Camino de vida?",
              a: "Ambos parten de tu fecha de nacimiento, pero se reducen de forma distinta. Tu [número del Camino de vida](/calculators/path-of-life) reduce por separado el mes, el día y el año y conserva los números maestros 11, 22 y 33, mientras que la carta de nacimiento suma primero la fecha completa y se detiene en 22, el número de cartas de los arcanos mayores.",
            },
            {
              q: "¿Por qué La Fuerza lleva aquí el número 8 si en mi baraja el 8 es La Justicia?",
              a: "Las barajas de tipo Marsella numeran La Justicia como 8 y La Fuerza como 11, mientras que la tradición Rider–Waite–Smith, que siguen muchas barajas modernas, las intercambia. Esta calculadora usa el orden Rider–Waite–Smith, así que el 8 es La Fuerza y el 11 La Justicia. Si prefieres el orden de Marsella, lee esas dos cartas al revés.",
            },
            {
              q: "¿Mi carta de nacimiento es la misma que la carta del tarot de mi signo del zodiaco?",
              a: "No. Tu carta de nacimiento sale de los números de tu fecha de nacimiento completa, mientras que cada signo del zodiaco está vinculado a un arcano mayor a través de la astrología (Leo con La Fuerza, por ejemplo). Dos personas del mismo signo pueden tener cartas de nacimiento distintas. Para tu signo y el resto de tu carta astral, prueba la [calculadora de carta natal](/calculators/natal-chart).",
            },
            {
              q: "¿Puede cambiar mi carta de nacimiento con el tiempo?",
              a: "No. Queda fijada por tu fecha de nacimiento. Algunos lectores también calculan una carta del año sumando tu mes y tu día de nacimiento al año en curso; esta calculadora muestra solo tus cartas de nacimiento.",
            },
            {
              q: "¿La carta de nacimiento del tarot es una predicción?",
              a: "No. Las cartas de nacimiento son una forma simbólica de reflexionar sobre tus fortalezas y tus lecciones, y se ofrecen solo con fines de entretenimiento. No son un pronóstico ni sustituyen el consejo profesional.",
            },
          ],
        },
      ],
    },
  ],
};

export default content;
