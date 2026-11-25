import type { TarotContent, TarotFrame } from "@/content/seo/tarot/types";
import { listOf } from "@/content/seo/names";

const content: TarotContent = {
  "the-fool": {
    keywords: "nuevos comienzos, confianza, espontaneidad",
    intro:
      "El Loco es la carta de nacimiento del tarot del espíritu libre. Lleva el número 0 en los arcanos mayores, y en el método de las cartas de nacimiento un total que se reduce a 22 se lee como El Loco, emparejado con El Emperador como carta del alma.",
    meaning:
      "Como carta de nacimiento, El Loco apunta a una vida que vuelve a empezar una y otra vez: curiosa, confiada y dispuesta a saltar antes de que todos los detalles estén claros. La tradición ve en la figura al borde del precipicio inocencia y potencial más que necedad: alguien para quien todo está aún por venir.",
    gifts: [
      "Valentía en los comienzos: prueban lo que otros aplazan.",
      "Apertura y un asombro casi infantil.",
      "Una fe en que las cosas pueden salir bien que contagia a los demás.",
    ],
    shadow: [
      "Temeridad: saltar sin comprobar el terreno.",
      "Evitar el compromiso o las consecuencias.",
      "Una ingenuidad de la que otros pueden aprovecharse.",
    ],
    note: "El Loco lleva el número 0, pero en la aritmética de las cartas de nacimiento su número de nacimiento es el 22. Por eso su constelación incluye a El Emperador: 2 + 2 = 4.",
  },
  "the-magician": {
    keywords: "voluntad, destreza, ingenio",
    intro:
      "El Mago es la carta de nacimiento del tarot de la voluntad hábil. Es la carta número 1; en las fechas de nacimiento actuales aparece como carta del alma, alcanzada desde los números de nacimiento 10 y 19, y no como primer resultado del cálculo.",
    meaning:
      "Como carta de nacimiento, El Mago describe a alguien que hace que las cosas ocurran: se comunica bien, aprende rápido y convierte la intención en acción. Las herramientas sobre la mesa en la imagen tradicional representan los cuatro palos, los recursos que ya tiene a su alcance.",
    gifts: [
      "Destreza e ingenio: trabajan con lo que tienen.",
      "Una comunicación clara y persuasiva.",
      "El impulso de convertir las ideas en acciones.",
    ],
    shadow: [
      "Habilidad sin sinceridad, o la tentación de manipular.",
      "Dispersar el talento en demasiados proyectos.",
      "Un orgullo de su propia destreza que les impide aprender.",
    ],
  },
  "the-high-priestess": {
    keywords: "intuición, misterio, saber interior",
    intro:
      "La Sacerdotisa es la carta de nacimiento del tarot de la intuición y el saber interior. Es la carta número 2, y también puede aparecer como carta del alma de otros números de nacimiento.",
    meaning:
      "Como carta de nacimiento, La Sacerdotisa describe a alguien que observa antes de hablar y confía en lo que percibe bajo la superficie. Se sienta entre las columnas del templo guardando lo oculto, y la tradición la vincula con los sueños, los secretos y la paciencia.",
    gifts: [
      "Una intuición profunda y olfato para lo que no se dice.",
      "Discreción: los demás les confían secretos.",
      "Calma ante la incertidumbre.",
    ],
    shadow: [
      "Reservarse: guardar tanto dentro que los demás no pueden llegar.",
      "Pasividad: esperar cuando haría falta actuar.",
      "Dudar de los hechos visibles a favor de impresiones vagas.",
    ],
  },
  "the-empress": {
    keywords: "cuidado, creatividad, abundancia",
    intro:
      "La Emperatriz es la carta de nacimiento del tarot de la fertilidad, la creatividad y el cuidado. Es la carta número 3, y también puede aparecer como carta del alma de otros números de nacimiento.",
    meaning:
      "Como carta de nacimiento, La Emperatriz apunta a una persona sensual y generosa que hace crecer las cosas, ya sea un jardín, una familia, un negocio o una obra creativa. La tradición la vincula con Venus, con la naturaleza y con el placer de la comodidad y la belleza.",
    gifts: [
      "Una calidez nutricia que ayuda a florecer a las personas y los proyectos.",
      "Creatividad y amor por la belleza.",
      "Generosidad y una apreciación fácil de los placeres de la vida.",
    ],
    shadow: [
      "Un cuidado asfixiante o el exceso de mimo.",
      "Depender de la comodidad o de la aprobación.",
      "Una energía creativa que se va entera en cuidar de los demás.",
    ],
  },
  "the-emperor": {
    keywords: "estructura, autoridad, protección",
    intro:
      "El Emperador es la carta de nacimiento del tarot del orden y la autoridad. Es la carta número 4, y también aparece como carta del alma de otros números de nacimiento.",
    meaning:
      "Como carta de nacimiento, El Emperador describe a alguien que crea estructura, pone límites y asume la responsabilidad por los demás. La tradición lo vincula con Aries, con el liderazgo y con el lado protector y estabilizador del poder.",
    gifts: [
      "Liderazgo y capacidad de organizar.",
      "Un instinto protector y un fuerte sentido del deber.",
      "Límites claros y acción decidida.",
    ],
    shadow: [
      "Rigidez y necesidad de control.",
      "Rasgos autoritarios o rechazo a ser cuestionados.",
      "Distancia emocional tras la competencia.",
    ],
  },
  "the-hierophant": {
    keywords: "tradición, enseñanza, pertenencia",
    intro:
      "El Hierofante es la carta de nacimiento del tarot de la tradición y la enseñanza. Es la carta número 5, y también aparece como carta del alma de otros números de nacimiento.",
    meaning:
      "Como carta de nacimiento, El Hierofante describe a alguien que valora el saber compartido, el ritual y la comunidad, y que a menudo se convierte en maestro o mentor. La tradición lo vincula con Tauro, con las instituciones y con la transmisión de la sabiduría de una generación a otra.",
    gifts: [
      "Enseñar y acompañar con paciencia.",
      "Respeto por las tradiciones que dan sentido a la vida.",
      "Crear confianza y sentido de pertenencia en los grupos.",
    ],
    shadow: [
      "Dogmatismo: confundir la manera habitual con la única manera.",
      "Conformismo o miedo a ser diferente.",
      "Usar las reglas para evitar la responsabilidad personal.",
    ],
  },
  "the-lovers": {
    keywords: "elección, unión, valores",
    intro:
      "Los Enamorados son la carta de nacimiento del tarot de la elección y la unión. Es la carta número 6; aunque su nombre sugiere romance, su sentido más antiguo trata de elegir de acuerdo con los propios valores.",
    meaning:
      "Como carta de nacimiento, Los Enamorados describen a alguien para quien las relaciones y las decisiones son centrales: elegir parejas, caminos y principios que pueda respaldar por completo. La tradición vincula la carta con Géminis y con la idea de que toda elección real define quiénes somos.",
    gifts: [
      "Capacidad de conexión profunda y comprometida.",
      "Claridad sobre sus valores cuando importa.",
      "Facilidad para el diálogo y para ver los dos lados.",
    ],
    shadow: [
      "Indecisión, o dejar que otros elijan por ellos.",
      "Idealizar a las parejas y decepcionarse.",
      "Definirse demasiado a través de las relaciones.",
    ],
  },
  "the-chariot": {
    keywords: "determinación, impulso, disciplina",
    intro:
      "El Carro es la carta de nacimiento del tarot de la determinación. Es la carta número 7 y muestra a un conductor que dirige fuerzas opuestas hacia una misma meta.",
    meaning:
      "Como carta de nacimiento, El Carro describe a alguien que triunfa con enfoque, disciplina y dominio de sí, aprendiendo a mantener unidos impulsos contrarios para avanzar. La tradición vincula la carta con Cáncer, con proteger lo que importa y con una victoria que hay que ganarse.",
    gifts: [
      "Enfoque y determinación.",
      "Una autodisciplina que convierte las metas en resultados.",
      "Valentía para tomar el timón en condiciones difíciles.",
    ],
    shadow: [
      "Agresividad o necesidad de ganar a toda costa.",
      "Seguir adelante en lugar de parar a reconsiderar.",
      "Esconder la vulnerabilidad tras el empuje.",
    ],
  },
  strength: {
    keywords: "valentía, paciencia, fuerza amable",
    intro:
      "La Fuerza es la carta de nacimiento del tarot del valor interior. Es la carta número 8 en la numeración Rider-Waite que usamos en este sitio y muestra a una persona que calma a un león con suavidad en lugar de fuerza bruta.",
    meaning:
      "Como carta de nacimiento, La Fuerza describe a alguien cuyo poder es la paciencia, la compasión y un dominio de sí sereno. La tradición vincula la carta con Leo y con la idea de que domar los propios miedos e impulsos es el mayor valor.",
    gifts: [
      "Una valentía tranquila y resiliencia emocional.",
      "Paciencia con las personas y con los sentimientos difíciles.",
      "Una calidez que serena a los demás.",
    ],
    shadow: [
      "Dudas sobre sí mismas tras una apariencia serena.",
      "Reprimir la ira o el miedo en lugar de trabajar con ellos.",
      "Cargar con las cargas ajenas.",
    ],
    note: "La numeración varía según las tradiciones. La Golden Dawn y el mazo Rider-Waite numeran la Fuerza como VIII y la Justicia como XI, de acuerdo con sus atribuciones astrológicas; el Tarot de Marsella y los mazos más antiguos las intercambian. Usamos la numeración Rider-Waite, que siguen la mayoría de los mazos modernos.",
  },
  "the-hermit": {
    keywords: "sabiduría, soledad, guía",
    intro:
      "El Ermitaño es la carta de nacimiento del tarot de la sabiduría interior. Es la carta número 9 y muestra a una figura solitaria que sostiene un farol en alto.",
    meaning:
      "Como carta de nacimiento, El Ermitaño describe a alguien que busca comprender mediante la reflexión, la soledad y el estudio, y que después ilumina el camino de otros. La tradición vincula la carta con Virgo, con el discernimiento y con el conocimiento que viene de la experiencia.",
    gifts: [
      "Profundidad de pensamiento y don de discernimiento.",
      "Comodidad con la soledad y el silencio.",
      "Una guía que nace de la experiencia vivida.",
    ],
    shadow: [
      "Retraimiento, soledad o evitar a la gente.",
      "Perfeccionismo y exceso de análisis.",
      "Guardarse lo que saben.",
    ],
  },
  "wheel-of-fortune": {
    keywords: "ciclos, cambio, oportunidad",
    intro:
      "La Rueda de la Fortuna es la carta de nacimiento del tarot de los ciclos y el cambio. Es la carta número 10 y muestra una rueda que gira, símbolo de las fortunas que suben y bajan.",
    meaning:
      "Como carta de nacimiento, la Rueda de la Fortuna describe a alguien cuya vida avanza en capítulos distintos y que aprende a adaptarse cuando las circunstancias giran. La tradición vincula la carta con Júpiter, con la suerte y la expansión, y con el recordatorio de que ninguna fase es permanente.",
    gifts: [
      "Adaptabilidad cuando las circunstancias cambian.",
      "Buen ojo para el momento y la oportunidad.",
      "Optimismo y perspectiva ante los contratiempos.",
    ],
    shadow: [
      "Dejar demasiado al azar.",
      "Sentirse víctima de las circunstancias.",
      "Inquietud cuando todo permanece quieto.",
    ],
  },
  justice: {
    keywords: "verdad, equidad, responsabilidad",
    intro:
      "La Justicia es la carta de nacimiento del tarot de la equidad y la verdad. Es la carta número 11 en la numeración Rider-Waite que usamos en este sitio y muestra a una figura con una balanza y una espada.",
    meaning:
      "Como carta de nacimiento, La Justicia describe a alguien con un fuerte sentido de lo justo y lo injusto, que sopesa las situaciones con cuidado y asume la responsabilidad de sus decisiones. La tradición vincula la carta con Libra, con el equilibrio y con el principio de que los actos tienen consecuencias.",
    gifts: [
      "Imparcialidad y un juicio claro.",
      "Honestidad, también consigo mismas.",
      "Un sentido de la responsabilidad con el que se puede contar.",
    ],
    shadow: [
      "Dureza: juzgarse a sí mismas o a otros con demasiada severidad.",
      "Un pensamiento rígido sobre lo correcto y lo incorrecto.",
      "Dificultad con las zonas grises y con el perdón.",
    ],
    note: "La numeración varía según las tradiciones. La Golden Dawn y el mazo Rider-Waite numeran la Justicia como XI y la Fuerza como VIII, de acuerdo con sus atribuciones astrológicas; el Tarot de Marsella y los mazos más antiguos las intercambian. Usamos la numeración Rider-Waite, que siguen la mayoría de los mazos modernos.",
  },
  "the-hanged-man": {
    keywords: "entrega, nueva perspectiva, pausa",
    intro:
      "El Colgado es la carta de nacimiento del tarot de la entrega y la nueva perspectiva. Es la carta número 12 y muestra a una figura suspendida con calma, cabeza abajo, de un pie.",
    meaning:
      "Como carta de nacimiento, El Colgado describe a alguien que aprende deteniéndose, soltando y viendo las cosas desde otro ángulo. La tradición vincula la carta con el agua y con el sacrificio voluntario: renunciar a algo a cambio de una comprensión más profunda.",
    gifts: [
      "Mente abierta y puntos de vista originales.",
      "La paciencia para esperar el momento justo.",
      "Disposición a soltar lo que ya no les sirve.",
    ],
    shadow: [
      "Procrastinación disfrazada de paciencia.",
      "Martirio: sacrificarse sin que nadie lo pida.",
      "Sentirse atascados e incapaces de actuar.",
    ],
  },
  death: {
    keywords: "finales, transformación, renovación",
    intro:
      "La Muerte es la carta de nacimiento del tarot de los finales y la transformación. Es la carta número 13 y, a pesar de su nombre, no se lee como una muerte literal.",
    meaning:
      "Como carta de nacimiento, La Muerte describe a alguien cuya vida está marcada por ciclos de cambio: dejar ir lo que ha terminado para que algo nuevo pueda crecer. La tradición vincula la carta con Escorpio, con la profundidad y con la renovación que sigue a una pérdida.",
    gifts: [
      "La capacidad de transformarse y empezar de nuevo.",
      "Valentía para afrontar las verdades difíciles.",
      "Profundidad: rara vez temen los temas difíciles.",
    ],
    shadow: [
      "Aferrarse a lo que terminó, o cortar los lazos con demasiada brusquedad.",
      "Miedo al cambio.",
      "Una intensidad que puede incomodar a los demás.",
    ],
    note: "En algunos mazos antiguos esta carta no lleva nombre, por lo que a veces se la llama el arcano sin nombre.",
  },
  temperance: {
    keywords: "equilibrio, moderación, armonía",
    intro:
      "La Templanza es la carta de nacimiento del tarot del equilibrio y la moderación. Es la carta número 14 y muestra a un ángel que vierte un líquido entre dos copas.",
    meaning:
      "Como carta de nacimiento, La Templanza describe a alguien que mezcla los opuestos y encuentra el camino medio: paciente, adaptable y sereno en su influencia. La tradición vincula la carta con Sagitario, con la búsqueda de sentido y con el arte de combinar las proporciones justas.",
    gifts: [
      "Paciencia y un don para el equilibrio.",
      "Una calma que ayuda a resolver conflictos.",
      "La capacidad de unir ideas y personas diferentes.",
    ],
    shadow: [
      "Evitar los extremos hasta el punto de que nada audaz sucede.",
      "Ceder demasiado.",
      "Impaciencia con la lentitud de la mezcla.",
    ],
  },
  "the-devil": {
    keywords: "apego, deseo, liberación",
    intro:
      "El Diablo es la carta de nacimiento del tarot del apego y el deseo. Es la carta número 15 y, a pesar de su nombre, se lee como símbolo de los hábitos y miedos que nos atan, no como el mal.",
    meaning:
      "Como carta de nacimiento, El Diablo describe a alguien con apetitos fuertes, ambición y magnetismo, que aprende a distinguir lo que elige de lo que lo tiene atrapado. La tradición vincula la carta con Capricornio, con la vida material y con una libertad que empieza al ver que las cadenas están sueltas.",
    gifts: [
      "Pasión, impulso y una voluntad fuerte.",
      "Una conciencia honesta del deseo y la tentación.",
      "Ingenio en los asuntos prácticos y materiales.",
    ],
    shadow: [
      "Compulsiones, dependencia o exceso.",
      "Cinismo sobre las motivaciones ajenas.",
      "Sentirse atrapados por sus roles y obligaciones.",
    ],
  },
  "the-tower": {
    keywords: "trastorno, revelación, avance",
    intro:
      "La Torre es la carta de nacimiento del tarot del cambio repentino y la revelación. Es la carta número 16 y muestra una torre alcanzada por un rayo.",
    meaning:
      "Como carta de nacimiento, La Torre describe a alguien cuya vida suele incluir giros dramáticos que despejan lo que no era sólido, y que se fortalece reconstruyendo sobre cimientos más verdaderos. La tradición vincula la carta con Marte, con la energía y con avances que llegan de golpe.",
    gifts: [
      "Resiliencia: reconstruyen bien.",
      "Una honestidad que atraviesa las ilusiones.",
      "Valentía en una crisis.",
    ],
    shadow: [
      "Dramas o conflictos que agitan las cosas sin necesidad.",
      "Resistirse al cambio hasta que se impone.",
      "Miedo a la inestabilidad.",
    ],
  },
  "the-star": {
    keywords: "esperanza, renovación, inspiración",
    intro:
      "La Estrella es la carta de nacimiento del tarot de la esperanza y la renovación. Es la carta número 17 y muestra a una figura que vierte agua bajo un cielo de estrellas.",
    meaning:
      "Como carta de nacimiento, La Estrella describe a alguien que aporta calma, inspiración y confianza en el futuro, a menudo tras una época difícil. La tradición vincula la carta con Acuario, con la generosidad y con la fe tranquila en que la renovación es posible.",
    gifts: [
      "Un optimismo que reconforta a los demás.",
      "Inspiración y un don para animar.",
      "Serenidad y sinceridad.",
    ],
    shadow: [
      "Un idealismo desconectado de los pasos prácticos.",
      "Decepción cuando la esperanza no se cumple enseguida.",
      "Dar sin medida y olvidarse de recargarse.",
    ],
  },
  "the-moon": {
    keywords: "intuición, imaginación, inconsciente",
    intro:
      "La Luna es la carta de nacimiento del tarot de la imaginación y el inconsciente. Es la carta número 18 y muestra un camino entre dos torres bajo la luna.",
    meaning:
      "Como carta de nacimiento, La Luna describe a alguien con una vida interior vívida, una intuición rica y sensibilidad a los estados de ánimo y a los sueños, que aprende a separar el miedo de la intuición. La tradición vincula la carta con Piscis, con los ciclos y con lo que aún no está claro.",
    gifts: [
      "Imaginación y sensibilidad artística.",
      "Intuición sobre las personas y las situaciones.",
      "Empatía y apertura a lo invisible.",
    ],
    shadow: [
      "Ansiedad, miedo o confusión.",
      "Ilusión: ver lo que se desea o se teme.",
      "Cambios de humor y evasión.",
    ],
  },
  "the-sun": {
    keywords: "alegría, vitalidad, claridad",
    intro:
      "El Sol es la carta de nacimiento del tarot de la alegría y la vitalidad. Es la carta número 19 y tradicionalmente una de las más optimistas de la baraja.",
    meaning:
      "Como carta de nacimiento, El Sol describe a alguien que irradia calidez y entusiasmo, abierto y directo, y que está más vivo cuando comparte el éxito y el juego. La tradición vincula la carta con el Sol, con la claridad y con una confianza infantil.",
    gifts: [
      "Calidez y optimismo natural.",
      "Apertura y honestidad.",
      "Una energía que anima a los demás.",
    ],
    shadow: [
      "Ingenuidad o ignorar los problemas.",
      "Necesidad de atención o aprobación.",
      "Exceso de confianza.",
    ],
  },
  judgement: {
    keywords: "despertar, balance, llamada",
    intro:
      "El Juicio es la carta de nacimiento del tarot del despertar y la llamada interior. Es la carta número 20 y muestra a figuras que se levantan al sonar una trompeta.",
    meaning:
      "Como carta de nacimiento, El Juicio describe a alguien que revisa su vida con honestidad, perdona y responde a una llamada al cambio. La tradición vincula la carta con el fuego, con la renovación y con el momento en que el pasado se entiende lo bastante como para soltarlo.",
    gifts: [
      "Una reflexión honesta sobre sí mismas.",
      "La capacidad de perdonar y empezar de nuevo.",
      "Un sentido de vocación y propósito.",
    ],
    shadow: [
      "La autocrítica y el juicio severo hacia sí mismas.",
      "Rumiar el pasado.",
      "Ignorar la llamada porque el cambio parece demasiado grande.",
    ],
  },
  "the-world": {
    keywords: "culminación, plenitud, logro",
    intro:
      "El Mundo es la carta de nacimiento del tarot de la culminación y la plenitud. Es la carta número 21 y muestra a una figura que danza dentro de una corona de laurel.",
    meaning:
      "Como carta de nacimiento, El Mundo describe a alguien que lleva las cosas a su término, conecta lo local con lo global y se siente a gusto en muchos mundos. La tradición vincula la carta con Saturno, con la maestría ganada con el tiempo y con la satisfacción de un ciclo bien cerrado.",
    gifts: [
      "Constancia y sentido de la culminación.",
      "Una perspectiva amplia y soltura con culturas o campos distintos.",
      "Integración: unir las diferentes partes de la vida.",
    ],
    shadow: [
      "Resistirse a terminar por miedo a lo que viene después.",
      "Perfeccionismo: nunca está del todo completo.",
      "Autocomplacencia tras el éxito.",
    ],
  },
};

export default content;

const list = (items: string[]) => listOf("es", items);

export const frame: TarotFrame = {
  eyebrow: "Tarot · Cartas de nacimiento",
  title: (card) => `${card}, carta de nacimiento del tarot: significado`,
  description: (card, keywords) =>
    `${card} como carta de nacimiento del tarot: ${keywords}. Su número, su atribución según la Golden Dawn, sus dones, su sombra, su constelación y un ejemplo de cálculo, con calculadora gratuita.`,
  h1: (card) => `${card}, carta de nacimiento del tarot`,
  headings: {
    meaning: (card) => `${card} como carta de nacimiento`,
    facts: "Número, numeral romano y atribución de la Golden Dawn",
    gifts: () => "Los dones de esta carta",
    shadow: () => "El lado de sombra de esta carta",
    constellation: () => "Su constelación",
    example: "Un ejemplo de cálculo",
  },
  labels: {
    arcana: "Número en los arcanos mayores",
    numeral: "Numeral romano",
    birthNumber: "Número de nacimiento",
    letter: "Letra hebrea",
    attribution: "Atribución de la Golden Dawn",
    element: "elemento",
    planet: "planeta",
    sign: "signo del zodiaco",
    total: "Mes + día + año",
    personality: "carta de personalidad",
    middle: "carta intermedia",
    soul: "carta del alma",
    both: "carta de personalidad y del alma",
  },
  attribution: (card, letter, value, kind) =>
    `En el sistema de la Golden Dawn, que inspiró la mayoría de los mazos modernos, ${card} corresponde a la letra hebrea ${letter} y ${kind === "element" ? `al elemento ${value}` : kind === "planet" ? (value === "el Sol" ? "al Sol" : value === "la Luna" ? "a la Luna" : `al planeta ${value}`) : `al signo ${value}`}.`,
  constellation: ({ card, birthNumber, chain, soulOf, family, soulOnly }) => {
    const lines: string[] = [];
    if (soulOnly)
      lines.push(`En las fechas de nacimiento desde 1900 el total nunca es ${birthNumber}, así que esta carta (${card}) nunca es la primera: aparece como carta del alma de ${list(soulOf.map((item) => `${item.number} (${item.card})`))}.`);
    else if (chain.length === 1)
      lines.push(`Cuando tu total es ${birthNumber}, ${card} es a la vez tu carta de personalidad y tu carta del alma.`);
    else if (chain.length === 2)
      lines.push(`Cuando tu total es ${birthNumber}, ${card} es tu carta de personalidad y ${chain[1]} tu carta del alma.`);
    else
      lines.push(`Cuando tu total es ${birthNumber}, tienes tres cartas: ${card} como carta de personalidad, después ${chain[1]} y, por último, ${chain[2]} como carta del alma.`);
    if (!soulOnly && soulOf.length)
      lines.push(`Esta carta es también la carta del alma de ${list(soulOf.map((item) => `${item.number} (${item.card})`))}.`);
    lines.push(`Su constelación, es decir, las cartas que comparten la misma carta del alma, reúne a ${list(family)}.`);
    return lines;
  },
  example: (date) => `Tomemos a alguien nacido el ${date}. Se suman el mes, el día y el año, y después las cifras del total hasta llegar a 22 o menos:`,
  faq: ({ card, entry, constellation, attribution, date, birthNumber, soulOnly, soulOf }) => [
    { q: `¿Qué significa ${card} como carta de nacimiento?`, a: entry.meaning },
    {
      q: `¿Cómo sé si ${card} es mi carta de nacimiento?`,
      a: soulOnly
        ? `Suma el mes, el día y el año de tu nacimiento y después las cifras del total hasta llegar a 22 o menos. Esta carta es la carta del alma de los totales ${soulOf}. Por ejemplo, una persona nacida el ${date} la tiene entre sus cartas. La calculadora al final de esta página hace el cálculo por ti.`
        : `Suma el mes, el día y el año de tu nacimiento y después las cifras del total hasta llegar a 22 o menos. Un total de ${birthNumber} da esta carta. Por ejemplo, una persona nacida el ${date} la tiene entre sus cartas. La calculadora al final de esta página hace el cálculo por ti.`,
    },
    { q: `¿Cuál es la constelación de ${card}?`, a: constellation },
    { q: `¿Qué correspondencia astrológica tiene ${card}?`, a: attribution },
    {
      q: "¿Una carta de nacimiento predice mi futuro?",
      a: "No. Las cartas de nacimiento son una tradición para la reflexión: describen temas sobre los que pensar, no hechos que vayan a ocurrir. Se ofrecen solo con fines de entretenimiento.",
    },
  ],
  related: "Descubre otras cartas de nacimiento",
  hubLink: "Todas las cartas de nacimiento",
  calculatorLink: "Calculadora de la carta de nacimiento del tarot",
  hub: {
    title: "Cartas de nacimiento del tarot: los 22 arcanos mayores",
    description:
      "Qué significa cada carta de nacimiento del tarot, de El Mago a El Loco: números, atribuciones de la Golden Dawn, dones, sombras y constelaciones. Encuentra tus cartas con la calculadora gratuita.",
    h1: "Las cartas de nacimiento del tarot y su significado",
    intro:
      "Tus cartas de nacimiento son los arcanos mayores que corresponden a la suma de tu fecha de nacimiento, un método que popularizó la autora de tarot Mary K. Greer. La mayoría de las personas tiene dos: una carta de personalidad y una carta del alma. Introduce tu fecha de nacimiento para encontrar las tuyas, o elige una carta más abajo.",
    gridHeading: "Las 22 cartas de nacimiento",
    sections: [
      {
        id: "how-it-works",
        heading: "Cómo se calculan las cartas de nacimiento",
        paragraphs: [
          "Suma el mes, el día y el año de tu nacimiento como números enteros. Después suma las cifras del total hasta que el resultado sea 22 o menos: ese número es tu carta de personalidad. Si tiene dos cifras, súmalas una vez más para encontrar tu carta del alma. Un total de 22 es El Loco, cuya carta del alma es El Emperador.",
          "Las cartas del 10 al 22 son siempre cartas de personalidad; las del 1 al 9 pueden ser cartas del alma. Algunos totales dan tres cartas: el 19 lleva a El Sol, a La Rueda de la Fortuna y a El Mago.",
        ],
      },
      {
        id: "constellations",
        heading: "Las constelaciones",
        paragraphs: [
          "Las cartas que comparten la misma carta del alma forman una constelación. La Estrella y La Fuerza van juntas porque 1 + 7 = 8, del mismo modo que El Sol, La Rueda de la Fortuna y El Mago comparten la carta del alma 1.",
        ],
      },
    ],
    faq: [
      {
        q: "¿Qué es una carta de nacimiento del tarot?",
        a: "Es un arcano mayor vinculado a tu fecha de nacimiento. Los tarotistas la usan como un tema vital sobre el que reflexionar, igual que los astrólogos usan el signo solar.",
      },
      {
        q: "¿Qué diferencia hay entre la carta de personalidad y la carta del alma?",
        a: "La carta de personalidad es el primer total de 22 o menos y describiría tu forma de expresarte. La carta del alma es ese número reducido a una cifra y describiría tu propósito más profundo. Cuando el total ya tiene una sola cifra, ambas son la misma carta.",
      },
      {
        q: "¿Por qué El Loco lleva el número 22?",
        a: "El Loco es la carta 0 de los arcanos mayores, pero una suma nunca es cero, así que el método de las cartas de nacimiento le da el número 22, el lugar que sigue a El Mundo.",
      },
      {
        q: "¿Qué numeración usáis?",
        a: "Seguimos la numeración Rider-Waite, heredada de la Golden Dawn: La Fuerza es el VIII y La Justicia el XI. El Tarot de Marsella y los mazos más antiguos intercambian estas dos cartas.",
      },
    ],
    tile: (birthNumber, numeral) => `Número de nacimiento ${birthNumber} · ${numeral}`,
  },
  breadcrumb: { hub: "Cartas de nacimiento del tarot" },
  cardAlt: (card) => `Carta del tarot: ${card}`,
};

