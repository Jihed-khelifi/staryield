// Figma 1279:1749 — Path Of Life Results, long-form section (Spanish).
import type { ArticleContent } from "@/content/calculators/types";

const content: ArticleContent = {
  kind: "life-path",
  label: "Comprender su Número del Camino de Vida",
  sections: [
    {
      id: "what-is-a-life-path-number",
      blocks: [
        { type: "h2", id: "what-is-a-life-path-number", text: "¿Qué es el Número del Camino de Vida?" },
        {
          type: "p",
          text: "En la numerología pitagórica, su Número del Camino de Vida es la coordenada más importante de toda su carta. Se calcula directamente a partir de su fecha de nacimiento completa y representa el plano central de su alma, los rasgos de carácter esenciales que le acompañan y las lecciones principales que está destinado a dominar a lo largo de su vida.",
        },
        {
          type: "p",
          text: "Mientras que su nombre de nacimiento revela sus habilidades activas y sus deseos internos, su Número del Camino de Vida establece el entorno general y los temas que encontrará. Tanto si vibra con un número raíz de una cifra (del 1 al 9) como con un Número Maestro intensificado (11, 22 o 33), comprender su número le aporta claridad sobre sus fortalezas naturales, su orientación profesional y la dinámica de sus relaciones.",
        },
      ],
    },
    {
      id: "how-to-calculate",
      blocks: [
        { type: "h2", id: "how-to-calculate", text: "Cómo calcular correctamente su Número del Camino de Vida" },
        {
          type: "p",
          weight: "light",
          text: "No todas las calculadoras de numerología son iguales. Muchos sitios web automatizados utilizan un método simple de «suma lineal»: sumar todas las cifras de una fecha de nacimiento en una sola fila. Aunque es rápido, este atajo elimina los Números Maestros intermedios en aproximadamente 1 de cada 7 fechas de nacimiento.",
        },
        {
          type: "p",
          weight: "light",
          text: "Nuestra calculadora utiliza el método tradicional de Reducción por Componentes (o de Ciclos de Periodo). Este enfoque reduce por separado los tres elementos de su fecha de nacimiento —el Mes, el Día y el Año— antes de combinarlos.",
        },
      ],
    },
    {
      id: "component-reduction-method",
      blocks: [
        { type: "h2", id: "component-reduction-method", text: "El método de Reducción por Componentes en 3 pasos" },
        {
          type: "ol",
          items: [
            "Reduzca el mes de nacimiento: convierta su mes de nacimiento en una sola cifra (del 1 al 9) o conserve el Número Maestro 11 (noviembre).",
            "Reduzca el día de nacimiento: reduzca su día de nacimiento a una sola cifra o conserve los Números Maestros 11 o 22 (por ejemplo, el día 11, 22 o 29).",
            "Reduzca el año de nacimiento: sume las cuatro cifras de su año de nacimiento y reduzca el resultado a una sola cifra o a un Número Maestro.",
            "Sume los resultados: sume los tres valores reducidos y realice una reducción final, salvo que el total sea 11, 22 o 33, en cuyo caso se conserva el Número Maestro.",
          ],
        },
        { type: "h3", id: "worked-example", text: "Ejemplo práctico: 29 de noviembre de 1990" },
        {
          type: "ul",
          items: [
            "Mes (noviembre): 11 (se conserva el Número Maestro)",
            "Día (29): 2 + 9 = 11 (se conserva el Número Maestro)",
            "Año (1990): 1 + 9 + 9 + 0 = 19 ⟹ 1 + 9 = 10 ⟹ 1 + 0 = 1",
            "Suma final: 11 + 11 + 1 = 23 ⟹ 2 + 3 = 5 ⟹ Camino de Vida 5",
          ],
        },
      ],
    },
    {
      id: "life-path-meanings",
      blocks: [
        {
          type: "twoColumnList",
          items: [
            { title: "Camino de Vida 1", subtitle: "El Líder", body: ["Innovación, independencia, autosuficiencia y visión pionera.", "Cuidado con las dudas sobre uno mismo, la impaciencia y las tendencias autoritarias."] },
            { title: "Camino de Vida 2", subtitle: "El Pacificador", body: ["Diplomacia, empatía, escucha activa y armonía en las relaciones.", "Gestionar la hipersensibilidad, la complacencia excesiva y la codependencia."] },
            { title: "Camino de Vida 3", subtitle: "El Comunicador", body: ["Expresión creativa, optimismo, talento artístico y carisma.", "Superar la dispersión, la superficialidad y la represión emocional."] },
            { title: "Camino de Vida 4", subtitle: "El Constructor", body: ["Disciplina, organización práctica, ejecución sistemática y lealtad.", "Evitar la rigidez extrema, la terquedad y el miedo a los cambios repentinos."] },
            { title: "Camino de Vida 5", subtitle: "El Buscador de Libertad", body: ["Adaptabilidad, ingenio, espíritu aventurero y versatilidad.", "Resistir la inquietud, la impulsividad y el miedo al compromiso a largo plazo."] },
            { title: "Camino de Vida 6", subtitle: "El Protector", body: ["Cuidado de la comunidad, armonía doméstica, responsabilidad protectora y equilibrio.", "Aprender a servir sin caer en el autosacrificio ni en el control."] },
            { title: "Camino de Vida 7", subtitle: "El Buscador", body: ["Investigación analítica, búsqueda espiritual, intuición y dominio técnico.", "Evitar el aislamiento cínico, el exceso de análisis y el distanciamiento emocional."] },
            { title: "Camino de Vida 8", subtitle: "El Poderoso", body: ["Liderazgo ejecutivo, dominio financiero, visión estratégica y autoridad.", "Equilibrar la ambición material con la responsabilidad ética y la generosidad."] },
            { title: "Camino de Vida 9", subtitle: "El Humanitario", body: ["Compasión universal, madurez artística, altruismo y plenitud.", "Soltar los apegos del pasado y establecer límites sanos."] },
          ],
        },
      ],
    },
    {
      id: "master-numbers",
      blocks: [
        { type: "h2", id: "master-numbers", text: "Los Números Maestros sagrados: 11, 22 y 33" },
        {
          type: "p",
          text: "En numerología, los números de dos cifras iguales (11, 22, 33) poseen un potencial vibratorio elevado. Conocidos como Números Maestros, actúan como octavas superiores de sus raíces de una sola cifra (2, 4 y 6).",
        },
        {
          type: "ul",
          items: [
            "**Camino de Vida 11/2 (El Iluminador Intuitivo):** Combina las cualidades pacificadoras de la raíz 2 con una intuición de alto voltaje y una visión profética. Quienes recorren el Camino 11 actúan como antenas espirituales, aunque a menudo deben gestionar una intensa energía nerviosa interior.",
            "**Camino de Vida 22/4 (El Maestro Constructor):** Toma la solidez estructural de la raíz 4 y la lleva a una escala grandiosa. Las personas del Camino 22 poseen la rara capacidad de materializar sueños visionarios abstractos en realidades tangibles y duraderas.",
            "**Camino de Vida 33/6 (El Gran Maestro):** Eleva las cualidades protectoras de la raíz 6 hacia el altruismo universal y el servicio espiritual. El Camino 33 representa la guía y la compasión desinteresadas, y exige una gran madurez emocional para evitar el agotamiento.",
          ],
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
              q: "¿Un nombre adoptado cambia mi Número del Camino de Vida?",
              a: "No. Su Número del Camino de Vida se calcula exclusivamente a partir de su fecha de nacimiento, por lo que permanece fijo independientemente de los cambios de nombre o de una adopción legal.",
            },
            {
              q: "¿Los Números Maestros son mejores o traen más suerte que los números de una cifra?",
              a: "No. Los Números Maestros (11, 22, 33) aportan un mayor potencial, pero también una mayor presión psicológica y lecciones de vida más intensas. Un Camino de Vida 4 u 8 de una sola cifra es igual de capaz de alcanzar un gran éxito y una plena realización que un Número Maestro.",
            },
            {
              q: "¿Por qué distintos sitios de numerología me dan números del Camino de Vida diferentes?",
              a: "Si su fecha de nacimiento contiene subciclos que suman 11 o 22, las calculadoras simplificadas que suman todas las cifras en una sola fila suelen reducir esos Números Maestros antes de tiempo. Nuestra calculadora utiliza el método estricto de Reducción por Componentes para garantizar cálculos intermedios exactos.",
            },
          ],
        },
      ],
    },
  ],
};

export default content;
