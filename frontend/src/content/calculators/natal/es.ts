// Figma 482:2010 (node 1258:1317) — Natal Chart Results, long-form section (Spanish).
import type { ArticleContent } from "@/content/calculators/types";
import { natalDiagramImage, natalIntroImage } from "@/content/calculators/natal/en";

const content: ArticleContent = {
  kind: "natal",
  label: "Acerca de la tirada de tarot de las Doce Casas",
  sections: [
    {
      id: "twelve-houses-spread",
      blocks: [
        {
          type: "split",
          blocks: [
            {
              type: "p",
              weight: "light",
              text: "La integración sistemática de la geometría astrológica con el simbolismo cartomántico constituye uno de los desarrollos más duraderos de las tradiciones adivinatorias occidentales. Entre las distintas tiradas concebidas para unir ambos sistemas, la tirada de tarot de las Doce Casas funciona como un marco de diagnóstico integral. Al situar las cartas del tarot sobre las doce divisiones de la carta astral, esta tirada proyecta la naturaleza polifacética de la experiencia humana sobre una rueda ordenada.",
            },
          ],
          image: { ...natalIntroImage, alt: "Cartas de tarot extendidas sobre una tela estampada" },
        },
      ],
    },
    {
      id: "history",
      blocks: [
        {
          type: "p",
          text: "La práctica de disponer cartas adivinatorias en una rueda astrológica evolucionó durante más de dos siglos, desde la cartomancia francesa de finales del siglo XVIII, pasando por las sociedades esotéricas victorianas, hasta las publicaciones populares del siglo XX.",
        },
        {
          type: "timeline",
          label: "Cronología histórica",
          items: [
            { title: "Etteilla", period: "(1785–1791)" },
            { title: "Golden Dawn", period: "(1888)" },
            { title: "Russell Grant", period: "(1992)" },
            { title: "Calculadoras web digitales" },
          ],
        },
        { type: "h3", id: "etteilla", text: "La era preesotérica y la época de Etteilla (1785–1791)" },
        {
          type: "p",
          weight: "light",
          text: "La fusión formal del tarot con la astrología la inició Jean-Baptiste Alliette (1738–1791), un influyente cartomántico francés que publicaba con el seudónimo de Etteilla. Antes de la obra de Etteilla, los adivinos solían sacar las cartas una a una de la parte superior de la baraja y leían cada una por separado. Con su texto de 1770 Manière de se récréer avec un jeu des cartes y sus posteriores publicaciones sobre el tarot entre 1785 y 1791 —entre ellas Jeu des tarots ou le livre de Thot—, Etteilla sentó las bases de la cartomancia estructural.",
        },
        {
          type: "p",
          weight: "light",
          text: "Etteilla introdujo la primera tirada documentada en forma de rueda astrológica de 12 casas, a la que llamó «Tirada del Horóscopo» o «Tirada de Doce». Asignó signos del zodiaco, cuerpos planetarios y temperamentos elementales concretos a cada carta, y demostró que colocar las cartas en casas astrológicas determinadas modifica su interpretación básica según el ámbito que rige cada casa.",
        },
        { type: "h3", id: "golden-dawn", text: "La síntesis de la Golden Dawn (1888)" },
        {
          type: "p",
          weight: "light",
          text: "Aunque Etteilla estableció las primeras correspondencias, la Orden Hermética de la Golden Dawn —fundada en 1888— codificó la síntesis esotérica del tarot, la astrología y la cábala. El sistema de la Golden Dawn asoció directamente los 22 Arcanos Mayores a los 22 senderos del Árbol de la Vida, que corresponden a los 12 signos del zodiaco, los 7 planetas clásicos y los 3 elementos primarios.",
        },
        {
          type: "p",
          weight: "light",
          text: "En el gran ritual adivinatorio de la Golden Dawn, conocido como la Apertura de la Llave, la segunda operación exige expresamente repartir la baraja en doce montones distintos que corresponden a las casas astrológicas del cielo. Esta operación tenía un propósito diagnóstico concreto: la casa en la que caía la carta Significadora del consultante identificaba el ámbito vital preciso en el que se manifestaba la cuestión planteada.",
        },
        { type: "h3", id: "russell-grant", text: "La popularización de finales del siglo XX y el Astro-Tarot de Russell Grant (1992)" },
        {
          type: "p",
          weight: "light",
          text: "En 1992, el astrólogo británico Russell Grant lanzó el Russell Grant’s Astro-Tarot Pack, publicado por Virgin Books e ilustrado por Kay Smith. Grant simplificó la cartomancia esotérica para un público más amplio al diseñar una baraja híbrida personalizada de 64 cartas.",
        },
        {
          type: "p",
          weight: "light",
          text: "Grant modificó la estructura estándar de 78 cartas: suprimió las cartas numéricas de los Arcanos Menores (del 2 al 10) y conservó los 22 Arcanos Mayores y los 4 Ases. Para completar el juego de 64 cartas, añadió cartas especiales:",
        },
        {
          type: "ul",
          items: [
            "12 cartas que representan los signos del zodiaco occidental.",
            "12 cartas que muestran los animales del zodiaco chino.",
            "14 cartas que representan los cuerpos planetarios, las fases lunares (luna nueva y luna llena) y los ángulos astrológicos clave (Ascendente, Medio Cielo y Nodo Norte).",
          ],
        },
        {
          type: "p",
          weight: "light",
          text: "La guía de Grant presentaba la tirada de las Doce Casas como su tirada analítica fundamental. Al combinar una baraja de diseño accesible con definiciones posicionales de las casas, Grant mostró cómo los lectores podían utilizar las posiciones astrológicas para anclar el significado de las cartas en ámbitos concretos de la vida, como el empleo, la vivienda y las relaciones personales.",
        },
      ],
    },
    {
      id: "structural-mechanics",
      blocks: [
        {
          type: "h2",
          id: "structural-mechanics",
          size: "lg",
          align: "center",
          text: "Mecánica estructural y fundamentos arquitectónicos de la tirada",
        },
        {
          type: "split",
          blocks: [
            {
              type: "p",
              weight: "medium",
              text: "La tirada de las Doce Casas se construye colocando doce cartas en una rueda en sentido antihorario que reproduce la secuencia natural del zodiaco tropical. La disposición comienza con la Casa Primera en la posición tradicional del Ascendente (las 9 en punto) y avanza en orden alrededor del círculo hasta terminar en la Casa Duodécima.",
            },
            {
              type: "p",
              text: "Interpretar bien la tirada de las Doce Casas exige evaluar las relaciones estructurales en toda la carta en lugar de analizar doce cartas aisladas. Los lectores expertos recurren a la triangulación entre casas y a la dinámica de las polaridades para extraer lecturas llenas de matices.",
            },
          ],
          image: {
            ...natalDiagramImage,
            alt: "Diagrama «Energy Mapping Tarot Reading»: doce posiciones de cartas dispuestas en una rueda y numeradas del 1 al 12",
          },
        },
        {
          type: "twoColumnList",
          items: [
            { title: "Casa 1", subtitle: "Marte / Aries (Ascendente) El yo y la identidad", body: ["Aspecto físico, personalidad esencial, vitalidad general, proyectos personales y perspectiva inmediata. Es el ancla principal de toda la consulta."] },
            { title: "Casa 2", subtitle: "Venus / Tauro Valores materiales y dinero", body: ["Finanzas personales, activos líquidos, ingresos, bienes personales, seguridad material, autoestima y compras importantes."] },
            { title: "Casa 3", subtitle: "Mercurio / Géminis Mente y comunicación", body: ["Enfoque mental cotidiano, viajes cortos, comunicación inmediata, hermanos, vecinos, escritura y pensamientos diarios predominantes."] },
            { title: "Casa 4", subtitle: "Luna / Cáncer (Fondo del Cielo) Hogar y linaje", body: ["Vida doméstica, residencia, orígenes familiares, seguridad emocional, bases psicológicas, mudanzas y lazos maternos."] },
            { title: "Casa 5", subtitle: "Sol / Leo Creatividad y romance", body: ["Nuevos romances, ocio, relaciones informales, expresión creativa, aficiones, entretenimiento, expresión personal e hijos."] },
            { title: "Casa 6", subtitle: "Mercurio / Virgo Trabajo diario y bienestar", body: ["Condiciones de trabajo cotidianas, servicio rutinario, hábitos personales, salud física, entorno laboral y responsabilidades diarias."] },
            { title: "Casa 7", subtitle: "Venus / Libra (Descendente) Relaciones comprometidas", body: ["Matrimonio formal, socios comerciales, contratos vinculantes, relaciones públicas, alianzas legales y adversarios declarados."] },
            { title: "Casa 8", subtitle: "Plutón / Escorpio Transformación y bienes compartidos", body: ["Finanzas compartidas, deudas conjuntas, impuestos, herencias, inversiones emocionales profundas, renacimiento psicológico, intimidad y ciclos vitales."] },
            { title: "Casa 9", subtitle: "Júpiter / Sagitario Mente superior y expansión", body: ["Estudios superiores, asuntos legales, viajes largos, creencias filosóficas, publicaciones, marcos morales y cambios de perspectiva."] },
            { title: "Casa 10", subtitle: "Saturno / Capricornio (Medio Cielo) Estatus público y carrera", body: ["Ambiciones profesionales a largo plazo, reputación profesional, posición pública, deber, autoridad corporativa e influencia paterna."] },
            { title: "Casa 11", subtitle: "Urano / Acuario Redes sociales y aspiraciones", body: ["Círculos sociales, afiliaciones a grupos, ideales a largo plazo, participación comunitaria, planes de futuro y posibles intereses románticos que surgen de amistades."] },
            { title: "Casa 12", subtitle: "Neptuno / Piscis Subconsciente y retiro", body: ["Soledad, motivos ocultos, patrones inconscientes, retiro espiritual, obstáculos invisibles, miedos ocultos y acontecimientos a punto de surgir."] },
          ],
        },
      ],
    },
    {
      id: "advanced-techniques",
      blocks: [
        {
          type: "h2",
          id: "advanced-techniques",
          size: "lg",
          text: "Técnicas de interpretación avanzadas: tríadas, ejes y dinámicas de diagnóstico",
        },
        { type: "h3", id: "cross-house-triangulation", text: "Triangulación entre casas" },
        {
          type: "p",
          weight: "semibold",
          text: "La triangulación consiste en analizar conjuntamente casas relacionadas para obtener un contexto profundo sobre temas personales concretos.",
        },
        {
          type: "ul",
          items: [
            "**La tríada de las relaciones (casas 5, 7 y 8):** La casa 5 revela el interés romántico inicial, la atracción y las citas informales. La casa 7 rige los compromisos formales, el matrimonio legal y la dinámica de la pareja a largo plazo. La casa 8 refleja la intimidad emocional subyacente, la compatibilidad sexual y las obligaciones financieras compartidas. Cartas favorables en la casa 5 junto a cartas difíciles en la casa 7 indican un romance inicial emocionante que encuentra fricciones estructurales al avanzar hacia un compromiso duradero.",
            "**El eje de la carrera y los logros (casas 6 y 10):** La casa 6 rige las tareas diarias, el empleo rutinario y la satisfacción laboral inmediata. La casa 10 representa la ambición profesional a largo plazo, la reputación pública y los logros profesionales. Las discrepancias entre ambas casas revelan desequilibrios laborales; cartas fuertes en la casa 10 combinadas con cartas tensas en la casa 6 señalan un reconocimiento profesional obtenido a costa de un trabajo diario agotador.",
            "**El eje de la mente y la perspectiva (casas 3 y 9):** La casa 3 describe los pensamientos diarios, las preocupaciones inmediatas y las interacciones locales. La casa 9 representa la filosofía de vida, los estudios superiores y una visión más amplia. Cuando un consultante sufre una sobrecarga mental en la casa 3, las cartas negativas en la casa 9 indican que una perspectiva demasiado estrecha está frenando su crecimiento personal.",
          ],
        },
        { type: "h3", id: "polarity-axes", text: "Ejes de polaridad y tensión estructural" },
        {
          type: "p",
          weight: "semibold",
          text: "Las casas astrológicas forman seis pares complementarios que se oponen directamente en la rueda. Analizar estos ejes opuestos revela las tensiones centrales y las necesidades de equilibrio:",
        },
        {
          type: "ul",
          items: [
            "**Casa 1 / Casa 7 (el eje de la identidad):** Equilibra la autonomía personal y los objetivos propios con las obligaciones y los compromisos que exigen las relaciones comprometidas.",
            "**Casa 2 / Casa 8 (el eje de los recursos):** Contrapone los ingresos individuales y los bienes personales a las inversiones compartidas, las deudas conjuntas, los impuestos y los bienes conyugales.",
            "**Casa 3 / Casa 9 (el eje de la mente):** Yuxtapone la información local concreta y el pensamiento cotidiano con la filosofía abstracta, el aprendizaje superior y las visiones del mundo.",
            "**Casa 4 / Casa 10 (el eje del origen y del estatus):** Equilibra la vida doméstica privada, la seguridad familiar y las raíces psicológicas con los objetivos profesionales públicos, la reputación y el deber social.",
          ],
        },
      ],
    },
  ],
};

export default content;
