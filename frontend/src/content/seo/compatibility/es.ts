import type { CompatibilityCopy, PairFacts } from "@/content/seo/compatibility/types";
import { listOf } from "@/content/seo/names";

const lower = (value: string) => value.toLowerCase();
const PLURAL_MODALITY: Record<string, string> = { cardinal: "cardinales", fijo: "fijos", mutable: "mutables" };

/** "un signo de Fuego y otro de Aire, uno cardinal y otro mutable" (never both the same). */
function makeUp(f: PairFacts): string {
  const elements = f.elementA === f.elementB ? `dos signos de ${f.elementA}` : `un signo de ${f.elementA} y otro de ${f.elementB}`;
  const [ma, mb] = [lower(f.modalityA), lower(f.modalityB)];
  const modalities = ma === mb ? `ambos ${PLURAL_MODALITY[ma] ?? ma}` : `uno ${ma} y otro ${mb}`;
  return `${elements}, ${modalities}`;
}

const copy: CompatibilityCopy = {
  aspects: {
    "same-sign": {
      name: "mismo signo",
      withArticle: "el mismo signo",
      angle: "0°, el mismo signo",
      essence:
        "Dos personas del mismo signo comparten elemento, modalidad y planeta regente. La astrología tradicional lo interpreta como un reconocimiento inmediato: cada uno entiende las motivaciones del otro, pero ambos comparten también los mismos puntos ciegos.",
      strength: "Comprensión inmediata: los mismos instintos, el mismo ritmo y los mismos valores hacen que cada uno resulte fácil de leer para el otro.",
      friction: "Un efecto espejo: los rasgos compartidos, también los difíciles, se duplican en lugar de equilibrarse.",
      tip: "Sumar lo que ninguno de los dos tiene de forma natural: amistades, aficiones y costumbres de fuera del signo compartido.",
    },
    "semi-sextile": {
      name: "semisextil",
      withArticle: "un semisextil",
      angle: "30°, signos vecinos",
      essence:
        "Dos signos vecinos forman un semisextil. No comparten elemento ni modalidad, y la tradición dice que cada signo nace del anterior, así que el vínculo resulta a la vez cercano y desconocido.",
      strength: "Habilidades complementarias: cada uno tiene lo que el otro todavía está aprendiendo.",
      friction: "Instintos distintos: los vecinos suelen malinterpretar las prioridades y el ritmo del otro.",
      tip: "Ver las diferencias como un intercambio de habilidades y no como una competición: preguntar cómo lo haría el otro.",
    },
    sextile: {
      name: "sextil",
      withArticle: "un sextil",
      angle: "60°, a dos signos de distancia",
      essence:
        "Los signos separados por dos posiciones forman un sextil y pertenecen a elementos afines, Fuego con Aire o Tierra con Agua. La astrología tradicional lo ve como un vínculo fácil y cooperativo que premia el esfuerzo: las oportunidades aparecen cuando ambos las aprovechan.",
      strength: "Comunicación fluida y un espíritu amistoso y cooperativo.",
      friction: "La comodidad puede volverse inercia: el vínculo funciona mejor cuando se cultiva activamente.",
      tip: "Planear proyectos o viajes en común: el sextil florece cuando tiene algo que hacer.",
    },
    square: {
      name: "cuadratura",
      withArticle: "una cuadratura",
      angle: "90°, a tres signos de distancia",
      essence:
        "Los signos separados por tres posiciones forman una cuadratura. Comparten modalidad pero no elemento, así que actúan de la misma manera persiguiendo metas distintas. La tradición lee la cuadratura como una tensión que impulsa el crecimiento.",
      strength: "Energía y atracción: la fricción mantiene viva la relación y empuja a ambos a crecer.",
      friction: "Luchas de poder: ambos pueden empujar a la vez en direcciones opuestas.",
      tip: "Acordar quién lleva cada área de la vida, y discutir sobre el problema, no sobre quién tiene razón.",
    },
    trine: {
      name: "trígono",
      withArticle: "un trígono",
      angle: "120°, a cuatro signos de distancia",
      essence:
        "Los signos separados por cuatro posiciones forman un trígono y comparten el mismo elemento. La astrología tradicional considera el trígono el ángulo más armonioso: una afinidad natural de valores y temperamento.",
      strength: "Armonía natural: unos valores compartidos y un temperamento parecido hacen que cooperar resulte fácil.",
      friction: "Demasiada comodidad: con tan poca fricción, los problemas pueden ignorarse hasta que crecen.",
      tip: "Seguir estimulándose con cariño, para que la facilidad no se convierta en rutina.",
    },
    quincunx: {
      name: "quincuncio",
      withArticle: "un quincuncio",
      angle: "150°, a cinco signos de distancia",
      essence:
        "Los signos separados por cinco posiciones forman un quincuncio, también llamado inconjunción. No comparten elemento ni modalidad, y la tradición ve en él un ajuste constante entre dos formas de vivir muy distintas.",
      strength: "Fascinación por la diferencia: cada uno le muestra al otro una forma de vivir que nunca habría elegido.",
      friction: "Malentendidos: las necesidades y las prioridades pueden parecer desacompasadas sin que nadie tenga la culpa.",
      tip: "Decir con claridad lo que se necesita: con un quincuncio poco se puede dar por sentado, y no pasa nada.",
    },
    opposition: {
      name: "oposición",
      withArticle: "una oposición",
      angle: "180°, signos opuestos",
      essence:
        "Los signos opuestos se miran de frente a través del zodiaco. Comparten modalidad y sus elementos se complementan, y la astrología tradicional ve en la oposición una fuerte atracción entre las dos mitades de un mismo eje.",
      strength: "Atracción magnética: cada uno encarna las cualidades que le faltan al otro.",
      friction: "Polarización: bajo presión, cada uno se atrinchera en su extremo del eje.",
      tip: "Buscar el punto medio del eje: tomar prestada la manera de hacer del otro en lugar de defender la propia.",
    },
  },
  elements: {
    "Fire-Fire": {
      summary: "Dos signos de Fuego comparten entusiasmo, valentía y gusto por la acción, así que la energía entre ellos es intensa.",
      strength: "Entusiasmo compartido: se animan mutuamente a probar, a atreverse y a divertirse.",
      friction: "Dos fuegos pueden encenderse a la vez: genio vivo y competencia por el protagonismo.",
      tip: "Turnarse para ir delante, y calmarse antes de resolver un desacuerdo.",
    },
    "Fire-Earth": {
      summary:
        "El Fuego aporta chispa y visión; la Tierra, paciencia y sentido práctico. La tradición los considera una pareja difícil, pero juntos pueden convertir ideas en resultados concretos.",
      strength: "Las ideas se encuentran con la constancia: uno empieza y el otro hace que dure.",
      friction: "Una cuestión de ritmo: el Fuego ve lenta a la Tierra, y la Tierra ve imprudente al Fuego.",
      tip: "Dejar que el Fuego elija el destino y la Tierra planifique el camino, respetando el ritmo de cada uno.",
    },
    "Fire-Air": {
      summary: "El Aire alimenta al Fuego: las ideas y la conversación avivan el entusiasmo y la acción. La astrología tradicional considera el Fuego y el Aire elementos afines.",
      strength: "Un intercambio animado: las ideas se convierten enseguida en planes y aventuras compartidas.",
      friction: "Inquietud: a ninguno de los dos elementos le gusta la rutina, así que lo práctico puede quedar olvidado.",
      tip: "Hacer juntos las tareas aburridas, para que la diversión tenga una base sólida.",
    },
    "Fire-Water": {
      summary:
        "El Fuego es directo y extrovertido; el Agua, sensible e introspectiva. La tradición lo ve como una mezcla difícil, aunque puede ser intensa y apasionada cuando ambos se cuidan.",
      strength: "Pasión y profundidad: el Fuego aporta calor y valentía; el Agua, profundidad emocional.",
      friction: "El Fuego puede herir al Agua sin darse cuenta, y los estados de ánimo del Agua pueden apagar el impulso del Fuego.",
      tip: "El Fuego puede frenar y escuchar; el Agua puede decir con claridad lo que siente en lugar de retraerse.",
    },
    "Earth-Earth": {
      summary: "Dos signos de Tierra comparten una mirada práctica, leal y paciente, y ambos valoran la estabilidad y lo que perdura.",
      strength: "Fiabilidad: las promesas se cumplen y construir una vida en común resulta natural.",
      friction: "Rutina y terquedad: la vida en pareja puede volverse previsible, y a ninguno le gusta ceder.",
      tip: "Introducir novedades con regularidad, como un viaje o un proyecto en común, y practicar el acuerdo en las cosas pequeñas.",
    },
    "Earth-Air": {
      summary: "La Tierra quiere lo concreto; el Aire vive de ideas y conversaciones. La tradición los ve como una pareja extraña, pero pueden aprender mucho el uno del otro.",
      strength: "Las ideas se vuelven prácticas: el Aire aporta perspectiva y la Tierra las hace viables.",
      friction: "La Tierra puede ver poco fiable al Aire, y el Aire puede ver a la Tierra demasiado aferrada a sus costumbres.",
      tip: "Acordar qué es fijo y qué queda abierto, y darse libertad en todo lo demás.",
    },
    "Earth-Water": {
      summary: "El Agua nutre a la Tierra: la profundidad emocional se encuentra con el cuidado práctico. La astrología tradicional considera la Tierra y el Agua elementos afines.",
      strength: "Seguridad y ternura: ambos valoran la lealtad, la comodidad y un hogar seguro.",
      friction: "Retraimiento: ambos pueden refugiarse en el silencio o la preocupación en lugar de hablar.",
      tip: "Poner nombre a las preocupaciones pronto: los dos son más fuertes cuando las comparten.",
    },
    "Air-Air": {
      summary: "Dos signos de Aire comparten curiosidad, gusto por la conversación y necesidad de espacio mental; rara vez se quedan sin temas.",
      strength: "Conversación sin fin y mucha libertad para cada uno.",
      friction: "Vivir en la cabeza: los sentimientos y los asuntos prácticos se comentan más de lo que se resuelven.",
      tip: "Convertir las conversaciones en pasos pequeños y concretos, y reservar tiempo para la cercanía física.",
    },
    "Air-Water": {
      summary: "El Aire piensa y el Agua siente. La tradición lo ve como una mezcla difícil de lógica y emoción, en la que cada uno puede enseñar al otro un nuevo idioma.",
      strength: "Cabeza y corazón: juntos pueden entender una situación desde los dos lados.",
      friction: "El Aire puede parecerle distante al Agua, y el Agua puede resultarle abrumadora al Aire.",
      tip: "El Aire puede reconocer las emociones antes de analizarlas; el Agua puede poner sus emociones en palabras.",
    },
    "Water-Water": {
      summary: "Dos signos de Agua comparten sensibilidad, intuición y profundidad emocional, y a menudo se entienden sin palabras.",
      strength: "Una empatía profunda y una gran cercanía emocional.",
      friction: "Los estados de ánimo se amplifican: las preocupaciones y las heridas pueden dar vueltas sin una mirada externa.",
      tip: "Mantener amistades y rutinas que den estabilidad a ambos, y preguntar antes de suponer lo que siente el otro.",
    },
  },
  modalities: {
    "Cardinal-Cardinal": {
      summary: "Dos signos cardinales son iniciadores: cada uno quiere empezar, decidir y dirigir.",
      friction: "Dos líderes pueden pelearse por el volante.",
      tip: "Repartir el liderazgo por áreas, para que cada uno tenga algo que dirigir.",
    },
    "Cardinal-Fixed": {
      summary: "Un signo cardinal pone las cosas en marcha y uno fijo las mantiene: uno arranca y el otro hace que duren.",
      friction: "El signo fijo se resiste a los cambios de rumbo repentinos del cardinal.",
      tip: "Hablar de los cambios antes de hacerlos, y dejar que la parte fija marque el ritmo del seguimiento.",
    },
    "Cardinal-Mutable": {
      summary: "Un signo cardinal marca la dirección y uno mutable la adapta: la iniciativa se encuentra con la flexibilidad.",
      friction: "Los cambios de opinión del signo mutable pueden frustrar los planes del cardinal.",
      tip: "Dejar que la parte cardinal elija la meta y que la mutable ajuste el camino.",
    },
    "Fixed-Fixed": {
      summary: "Dos signos fijos son constantes y decididos: leales, perseverantes y lentos para cambiar.",
      friction: "Terquedad por ambas partes: los desacuerdos pueden enquistarse.",
      tip: "Acordar de antemano cómo resolver los desacuerdos, y agradecer al otro cuando cede.",
    },
    "Fixed-Mutable": {
      summary: "Un signo fijo aporta constancia y uno mutable, capacidad de adaptación: la estabilidad se encuentra con el cambio.",
      friction: "El signo fijo puede ver inconstante al mutable, y el mutable puede ver rígido al fijo.",
      tip: "Mantener algunos rituales fijos y dejar el resto abierto al cambio.",
    },
    "Mutable-Mutable": {
      summary: "Dos signos mutables son flexibles y curiosos: tranquilos, adaptables y abiertos al cambio.",
      friction: "Nadie lleva el timón: los planes se diluyen y las decisiones se aplazan.",
      tip: "Poner por escrito las metas y las fechas en común, para que la flexibilidad no se convierta en deriva.",
    },
  },
  signs: {
    Aries: {
      traits: "audaz, directo y rápido para actuar",
      brings: "iniciativa y valentía",
      needs: "espacio para liderar y actuar por impulso",
      love: "Aries ama de forma abierta y apasionada, y le gusta ir a por lo que desea.",
    },
    Taurus: {
      traits: "estable, sensual y leal",
      brings: "paciencia y sensación de seguridad",
      needs: "estabilidad, comodidad y tiempo para decidir",
      love: "Tauro ama despacio y con lealtad, y muestra su cariño con presencia, contacto y cuidados.",
    },
    Gemini: {
      traits: "curioso, ingenioso y versátil",
      brings: "conversación, humor e ideas nuevas",
      needs: "variedad y estímulo intelectual",
      love: "Géminis ama a través de las palabras y el juego, y necesita una pareja que también sea su amiga.",
    },
    Cancer: {
      traits: "cuidadoso, protector e intuitivo",
      brings: "ternura y lealtad emocional",
      needs: "seguridad emocional y un verdadero hogar",
      love: "Cáncer ama protegiendo y recuerda cada gesto amable; se abre cuando se siente seguro.",
    },
    Leo: {
      traits: "cálido, generoso y expresivo",
      brings: "calidez, lealtad y sentido de la diversión",
      needs: "reconocimiento y un lugar donde brillar",
      love: "Leo ama a lo grande y con generosidad, y quiere sentirse admirado a cambio.",
    },
    Virgo: {
      traits: "atento, práctico y reflexivo",
      brings: "cuidado por los detalles y ayuda práctica",
      needs: "orden, utilidad y una sinceridad amable",
      love: "Virgo ama con gestos útiles y atención a los detalles más que con grandes declaraciones.",
    },
    Libra: {
      traits: "diplomático, elegante y justo",
      brings: "armonía, encanto y sentido del equilibrio",
      needs: "justicia, belleza y una verdadera relación de pareja",
      love: "Libra ama con encanto y cortesía, y busca una relación entre iguales.",
    },
    Scorpio: {
      traits: "intenso, perspicaz y reservado",
      brings: "profundidad, lealtad y sinceridad emocional",
      needs: "confianza y un compromiso total",
      love: "Escorpio ama con intensidad y lealtad, y espera la misma profundidad a cambio.",
    },
    Sagittarius: {
      traits: "aventurero, franco y optimista",
      brings: "entusiasmo, humor y horizontes más amplios",
      needs: "libertad, viajes y sentido",
      love: "Sagitario ama con libertad y sinceridad, y quiere una pareja que también sea compañera de camino.",
    },
    Capricorn: {
      traits: "ambicioso, disciplinado y fiable",
      brings: "estructura, compromiso y planes a largo plazo",
      needs: "respeto, metas claras y avances constantes",
      love: "Capricornio ama en serio y a largo plazo, y demuestra sus sentimientos con compromiso.",
    },
    Aquarius: {
      traits: "independiente, inventivo e idealista",
      brings: "originalidad y espíritu de amistad",
      needs: "independencia y espacio para sus ideales",
      love: "Acuario ama primero como amigo, y valora la libertad y la sinceridad en la pareja.",
    },
    Pisces: {
      traits: "imaginativo, empático y dulce",
      brings: "compasión, imaginación y entrega",
      needs: "comprensión y espacio para soñar",
      love: "Piscis ama con ternura y romanticismo, y a menudo intuye lo que siente su pareja antes de que lo diga.",
    },
  },
  planets: {
    Sun: "el Sol, astro de la identidad y la vitalidad",
    Moon: "la Luna, astro de la emoción y el instinto",
    Mercury: "Mercurio, el planeta del pensamiento y la comunicación",
    Venus: "Venus, el planeta del amor, el placer y la armonía",
    Mars: "Marte, el planeta del impulso y la valentía",
    Jupiter: "Júpiter, el planeta del crecimiento y el optimismo",
    Saturn: "Saturno, el planeta de la estructura y la responsabilidad",
    Uranus: "Urano, el planeta del cambio y la originalidad",
    Neptune: "Neptuno, el planeta de la imaginación y la compasión",
    Pluto: "Plutón, el planeta de la profundidad y la transformación",
  },
  frame: {
    eyebrow: "Astrología · Compatibilidad",
    title: (a, b) => `Compatibilidad de ${a} y ${b}: amor, amistad y trabajo`,
    description: (f) =>
      f.same
        ? `Compatibilidad de ${f.a} con ${f.a}: dos signos de ${f.elementA} con el mismo regente. Fortalezas, puntos de fricción, consejos prácticos y puntuaciones en el amor, la amistad y el trabajo. Calculadora gratuita.`
        : `Compatibilidad de ${f.a} y ${f.b}: ${f.aspectWithArticle} entre un signo de ${f.elementA} y otro de ${f.elementB}. Fortalezas, puntos de fricción, consejos prácticos y puntuaciones en el amor, la amistad y el trabajo. Calculadora gratuita.`,
    h1: (a, b) => `Compatibilidad de ${a} y ${b}`,
    intro: (f) =>
      f.same
        ? `Dos personas de ${f.a} comparten el mismo elemento (${f.elementA}), la misma modalidad (${lower(f.modalityA)}) y el mismo regente (${f.rulerA}). En la astrología tradicional, es ${f.levelText}, basada en un reconocimiento inmediato y en fortalezas y puntos ciegos compartidos.`
        : `${f.a} y ${f.b} forman ${f.aspectWithArticle} (${f.angle}), que une ${makeUp(f)}. En la astrología tradicional, es ${f.levelText}.`,
    pairName: (f) => (f.same ? `dos personas de ${f.a}` : `${f.a} y ${f.b}`),
    headings: {
      overview: (pair) => `Cómo se llevan ${pair}`,
      elements: "Elementos y modalidades",
      rulers: "Planetas regentes",
      strengths: (pair) => `Qué funciona entre ${pair}`,
      friction: (pair) => `Dónde pueden chocar ${pair}`,
      tips: "Consejos prácticos",
      indicators: "Amor, amistad y trabajo",
    },
    labels: {
      element: (of) => `Elemento ${of}`,
      modality: (of) => `Modalidad ${of}`,
      ruler: (of) => `Regente ${of}`,
      love: "Amor",
      friendship: "Amistad",
      work: "Trabajo",
      outOf: (value) => `${value} de 5`,
    },
    portrait: (f, traitsA, traitsB) =>
      f.same ? `Ambos miembros de la pareja comparten el carácter de ${f.a}: ${traitsA}.` : `${f.a} es ${traitsA}; ${f.b} es ${traitsB}.`,
    brings: (f, bringsA, bringsB) =>
      f.same ? `Los dos aportan ${bringsA}, así que en la relación hay de sobra.` : `${f.a} aporta ${bringsA}, mientras que ${f.b} aporta ${bringsB}.`,
    needs: (f, needsA, needsB) =>
      f.same
        ? `Los dos necesitan ${needsA}, lo que puede llevarlos a competir por lo mismo.`
        : `${f.a} necesita ${needsA}, mientras que ${f.b} necesita ${needsB}: hacer sitio a ambas cosas forma parte del trabajo.`,
    rulers: (f, planetA, planetB) =>
      f.same
        ? `${f.a} está regido por ${planetA}. Compartir regente significa querer lo mismo de la vida, a veces en el mismo momento.`
        : f.sharedRuler
          ? `${f.a} y ${f.b} están regidos por ${planetA}. Ese regente común les da un terreno compartido, aunque sus elementos lo expresen de forma distinta.`
          : `${f.a} está regido por ${planetA}, y ${f.b}, por ${planetB}. Cada regente describe lo que busca su signo, y armonizar ambos forma parte de la relación.`,
    reading: (f) => {
      const scores = f.indicators;
      const max = Math.max(scores.love, scores.friendship, scores.work);
      const names = { love: "en el amor", friendship: "en la amistad", work: "en el trabajo" } as const;
      const best = (Object.keys(names) as (keyof typeof names)[]).filter((key) => scores[key] === max).map((key) => names[key]);
      return best.length === 3
        ? `Esta combinación puntúa igual en el amor, la amistad y el trabajo (${max} de 5).`
        : `Esta combinación destaca sobre todo ${listOf("es", best)} (${max} de 5).`;
    },
    indicatorNote:
      "Estas puntuaciones son una heurística de la astrología tradicional, ofrecida con fines de entretenimiento. Salen del ángulo entre los dos signos, de sus elementos, modalidades y planetas regentes, no de datos sobre parejas reales. Una relación depende de las personas, y una carta natal completa dice mucho más que los signos solares.",
    levels: {
      harmonious: "una de las combinaciones más armoniosas",
      good: "una buena combinación que crece con el esfuerzo",
      mixed: "una combinación ambivalente, entre la atracción y la fricción",
      demanding: "una combinación exigente que pide paciencia por ambas partes",
    },
    levelShort: { harmonious: "Armoniosa", good: "Buena", mixed: "Ambivalente", demanding: "Exigente" },
    faq: (f, { essence, loveA, loveB, elementSummary }) => [
      {
        q: f.same ? `¿Son compatibles dos personas de ${f.a}?` : `¿Son compatibles ${f.a} y ${f.b}?`,
        a: f.same
          ? `En la astrología tradicional, dos personas de ${f.a} forman ${f.levelText}. ${essence}`
          : `En la astrología tradicional, ${f.a} y ${f.b} forman ${f.levelText}. ${essence}`,
      },
      {
        q: f.same ? `¿Hacen buena pareja dos personas de ${f.a}?` : `¿Hacen buena pareja ${f.a} y ${f.b} en el amor?`,
        a: f.same
          ? `${loveA} En nuestra escala tradicional, dos personas de ${f.a} obtienen ${f.indicators.love} de 5 en el amor.`
          : `${loveA} ${loveB} En nuestra escala tradicional, esta combinación obtiene ${f.indicators.love} de 5 en el amor.`,
      },
      { q: f.same ? `¿Qué tienen en común dos personas de ${f.a}?` : `¿Qué tienen en común ${f.a} y ${f.b}?`, a: elementSummary },
      {
        q: "¿Es fiable la compatibilidad de los signos solares?",
        a: "Los signos solares solo dan una idea general. Los astrólogos comparan también la Luna, Venus, Marte y el ascendente de ambas cartas, y una relación depende de las personas, no de sus signos. Esta página se ofrece solo con fines de entretenimiento.",
      },
    ],
    related: "Descubre otras combinaciones",
    hubLink: "Todas las combinaciones de signos",
    calculatorLink: "Calculadora de signo zodiacal",
    hub: {
      title: "Compatibilidad de signos: las 78 combinaciones del zodiaco",
      description:
        "La compatibilidad de los signos del zodiaco en sus 78 combinaciones: el ángulo entre los signos, elementos, modalidades, planetas regentes, fortalezas, puntos de fricción y consejos. Introduce dos fechas de nacimiento para encontrar la tuya.",
      h1: "Compatibilidad de los signos del zodiaco",
      intro:
        "La compatibilidad astrológica compara dos signos solares a través del ángulo que los separa, sus elementos, sus modalidades y sus planetas regentes. Introduce dos fechas de nacimiento para encontrar tu combinación, o elige una de las 78 combinaciones de abajo.",
      gridHeading: "Todas las combinaciones de signos",
      gridIntro: "Elige tu signo y después el de tu pareja, un amigo o un compañero.",
      sections: [
        {
          id: "how-it-works",
          heading: "Cómo leemos la compatibilidad",
          paragraphs: [
            "Cada combinación se lee a partir de cuatro factores tradicionales: el ángulo entre los dos signos en la rueda del zodiaco, sus elementos (Fuego, Tierra, Aire, Agua), sus modalidades (cardinal, fija, mutable) y sus planetas regentes. Usamos los regentes modernos: Escorpio está regido por Plutón, Acuario por Urano y Piscis por Neptuno.",
            "Las puntuaciones de amor, amistad y trabajo de cada página salen de esos factores con una regla fija, así que los mismos dos signos siempre obtienen la misma respuesta. Son una heurística de entretenimiento, no una medición.",
          ],
        },
      ],
      aspectsHeading: "Las siete relaciones entre signos",
      aspectLine: (name, angle, essence) => `**${name.charAt(0).toUpperCase()}${name.slice(1)}** (${angle}): ${essence}`,
      faq: [
        {
          q: "¿Cómo se calcula la compatibilidad astrológica?",
          a: "Compara los dos signos solares: su distancia en la rueda del zodiaco, si sus elementos y modalidades se apoyan y qué planetas los rigen. Nuestra calculadora encuentra los dos signos a partir de dos fechas de nacimiento.",
        },
        {
          q: "¿Y si uno de los dos nació en una cúspide?",
          a: "Los días en que el Sol cambia de signo, el signo depende de la hora y el lugar de nacimiento. La calculadora te lo indica y te deja elegir, o añadir tu hora de nacimiento en la calculadora de signo zodiacal.",
        },
        {
          q: "¿Qué signos son los más compatibles?",
          a: "La astrología tradicional favorece a los signos del mismo elemento (trígonos) y de elementos afines (sextiles), mientras que los signos opuestos combinan una fuerte atracción con cierta tensión. Cada combinación tiene fortalezas y puntos de fricción, y estas páginas describen ambos.",
        },
        {
          q: "¿Son predicciones estas lecturas de compatibilidad?",
          a: "No. Describen temas astrológicos tradicionales para la reflexión y el entretenimiento. No pueden decir si una relación va a funcionar.",
        },
      ],
    },
    breadcrumb: { hub: "Compatibilidad de signos" },
    imageAlt: "Rayos de sol atravesando las nubes al amanecer",
    signAlt: (sign) => `Símbolo del signo ${sign}`,
  },
};

export default copy;
