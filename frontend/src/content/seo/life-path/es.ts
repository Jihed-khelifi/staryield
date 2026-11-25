import type { LifePathContent, LifePathFrame } from "@/content/seo/life-path/types";

const content: LifePathContent = {
  1: {
    title: "El Líder",
    keywords: "independencia, iniciativa, valentía",
    intro:
      "El camino de vida 1 es el número del pionero. En la numerología pitagórica describe a alguien que aprende empezando cosas, valiéndose por sí mismo y confiando en su propio criterio.",
    archetype:
      "El uno es el primer número, el punto desde el que se cuenta todo lo demás, y los numerólogos leen en él el impulso de empezar. A las personas de este camino se las describe tradicionalmente como originales, autosuficientes y directas, más felices cuando eligen su propia dirección.",
    strengths: [
      "Iniciativa: empiezan lo que otros solo comentan.",
      "Valentía para mantenerse solas ante una decisión impopular.",
      "Originalidad: suelen encontrar su propia manera de hacer las cosas.",
    ],
    challenges: [
      "Impaciencia con las personas y los procesos más lentos.",
      "Una terquedad que puede pasar por fortaleza.",
      "Dificultad para pedir ayuda o admitir que dependen de otros.",
    ],
    love: "En pareja, el 1 valora la sinceridad, la lealtad y una pareja que también sea independiente. El aprendizaje suele estar en compartir el mando: escuchar antes de decidir y dejar que a veces gane el plan del otro.",
    work: "El liderazgo, el emprendimiento y cualquier puesto con verdadera autonomía encajan con el retrato tradicional del 1: fundador, directivo, autónomo, inventor. La supervisión constante y los comités interminables tienden a agotarlo.",
    purpose:
      "El propósito que se atribuye al camino de vida 1 es el liderazgo de sí mismo: actuar por convicción sin necesidad de dominar y guiar de un modo que haga a los demás más capaces, no más dependientes.",
    compatibleNote:
      "Los numerólogos emparejan tradicionalmente el 1 con el 3 y el 5, números que comparten su energía y disfrutan del movimiento, el humor y los nuevos comienzos en lugar de disputarle el mando.",
  },
  2: {
    title: "El Pacificador",
    keywords: "cooperación, sensibilidad, diplomacia",
    intro:
      "El camino de vida 2 es el número del pacificador. En la numerología pitagórica describe a una persona cuyo don es la colaboración: percibir lo que sienten los demás y devolver el equilibrio entre las personas.",
    archetype:
      "El dos es el primer número que necesita a otro, y los numerólogos leen en él el principio de la relación, la paciencia y el tacto. A las personas de este camino se las describe como amables, observadoras y cooperativas, a menudo quienes notan el ambiente no dicho de una sala.",
    strengths: [
      "Diplomacia: saben calmar una conversación tensa.",
      "Sensibilidad ante las necesidades, los estados de ánimo y las señales no dichas.",
      "Paciencia para las tareas que exigen cuidado y repetición.",
    ],
    challenges: [
      "Evitar el conflicto hasta que crece el resentimiento.",
      "Absorber las emociones ajenas como si fueran propias.",
      "Dejar sus propias necesidades para el final y luego sentirse ignoradas.",
    ],
    love: "El 2 suele ser entregado, atento y romántico. Su trabajo es decir en voz alta lo que quiere, porque una pareja no puede responder a una necesidad que nunca se nombra, y conservar su identidad dentro de la relación.",
    work: "La mediación, el acompañamiento, la enseñanza, el diseño, la administración y los puestos de apoyo encajan con el retrato tradicional del 2, igual que cualquier equipo que valore la escucha atenta. Los entornos ruidosos y competitivos pueden agotarlo.",
    purpose:
      "El propósito vinculado al camino de vida 2 es la armonía sin borrarse a sí mismo: entender que la paz que vale la pena incluye la propia voz y que la cooperación es más fuerte cuando ambas partes son sinceras.",
    compatibleNote:
      "Los numerólogos emparejan tradicionalmente el 2 con el 4, el 6 y el 8, números estables que ofrecen la fiabilidad y el reconocimiento que busca.",
  },
  3: {
    title: "El Comunicador",
    keywords: "expresión, creatividad, optimismo",
    intro:
      "El camino de vida 3 es el número de la expresión personal. En la numerología pitagórica describe a alguien que procesa la vida hablando, escribiendo, creando o actuando, y que anima a quienes lo rodean.",
    archetype:
      "El tres se asocia tradicionalmente con la creatividad y la alegría, el número que aparece cuando dos se unen para crear algo nuevo. A las personas de este camino se las describe como ingeniosas, expresivas y sociables, con don para convertir sentimientos e ideas en palabras, arte o humor.",
    strengths: [
      "Un don expresivo para la palabra, la escritura, el arte o la actuación.",
      "Un optimismo que ayuda a los demás a ver posibilidades.",
      "Soltura social: hacen que la gente se sienta bienvenida.",
    ],
    challenges: [
      "Energía dispersa: muchos comienzos, menos finales.",
      "Usar el encanto o las bromas para evitar emociones difíciles.",
      "Sensibilidad a la crítica escondida tras una fachada alegre.",
    ],
    love: "El 3 aporta a la pareja juego, conversación y cariño, y necesita a alguien que disfrute de los tres. Crece al quedarse presente cuando la charla se vuelve seria, en lugar de aligerar demasiado pronto el momento.",
    work: "La escritura, la enseñanza, el diseño, el marketing, el espectáculo y otros campos creativos o de trato público encajan con el retrato tradicional del 3. La rutina sin espacio para las ideas suele aburrirlo.",
    purpose:
      "El propósito que se atribuye al camino de vida 3 es la expresión auténtica: elegir una o dos formas de creatividad y desarrollarlas con disciplina, para que la alegría se convierta en una obra y no solo en un estado de ánimo.",
    compatibleNote:
      "Los numerólogos emparejan tradicionalmente el 3 con el 1, el 5 y el 9, números vivaces que comparten su curiosidad y su gusto por la variedad.",
  },
  4: {
    title: "El Constructor",
    keywords: "estabilidad, disciplina, fiabilidad",
    intro:
      "El camino de vida 4 es el número del constructor. En la numerología pitagórica describe a una persona paciente y práctica que crea seguridad con trabajo constante, estructura clara y palabra cumplida.",
    archetype:
      "El cuatro se asocia con las cuatro esquinas, los cimientos y el cuadrado. A las personas de este camino se las describe como fiables, metódicas y leales, que prefieren los métodos probados y los resultados visibles a las promesas y el ruido.",
    strengths: [
      "Fiabilidad: se sabe que harán lo que dijeron.",
      "Organización: convierten las ideas en planes y pasos.",
      "Perseverancia en los proyectos largos.",
    ],
    challenges: [
      "Rigidez cuando los planes o las rutinas tienen que cambiar.",
      "Terquedad, con tendencia a insistir en que así es como se hacen las cosas.",
      "Exceso de trabajo, viendo el descanso como algo que hay que ganarse.",
    ],
    love: "El 4 demuestra su amor con hechos: estar presente, arreglar cosas, planear el futuro. El aprendizaje consiste en añadir espontaneidad y palabras cariñosas, para que la pareja oiga además de ver cuánto le importa.",
    work: "La ingeniería, la construcción, las finanzas, la contabilidad, la gestión de proyectos, el derecho, las operaciones y la artesanía encajan con el retrato tradicional del 4, igual que cualquier puesto donde importen la exactitud y el seguimiento.",
    purpose:
      "El propósito que se atribuye al camino de vida 4 es construir algo duradero, como un hogar, un negocio o una obra, y mantenerse lo bastante flexible para dejarlo crecer.",
    compatibleNote:
      "Los numerólogos emparejan tradicionalmente el 4 con el 2, el 6 y el 8, números que valoran el compromiso, la practicidad y un plan compartido a largo plazo.",
  },
  5: {
    title: "El Buscador de libertad",
    keywords: "libertad, curiosidad, cambio",
    intro:
      "El camino de vida 5 es el número de la libertad y el cambio. En la numerología pitagórica describe a alguien que aprende con la experiencia, los viajes y la variedad, y que está en su mejor momento cuando la vida sigue siendo interesante.",
    archetype:
      "El cinco está en el centro de los números del 1 al 9 y se asocia con el movimiento, los cinco sentidos y la adaptación. A las personas de este camino se las describe como curiosas, versátiles y rápidas para adaptarse, con una inquietud que es también su motor.",
    strengths: [
      "Adaptabilidad: afrontan el cambio mejor que la mayoría.",
      "Una curiosidad que las mantiene aprendiendo en varios campos.",
      "Una comunicación viva y persuasiva.",
    ],
    challenges: [
      "Inquietud y dificultad para terminar lo que empiezan.",
      "Decisiones impulsivas para escapar de la rutina.",
      "Reticencia a comprometerse antes de sentirse libres.",
    ],
    love: "El 5 quiere una pareja que sea compañera de aventuras y que respete su espacio. El compromiso le resulta más seguro cuando se elige libremente en lugar de exigirse, y cuando las rutinas compartidas dejan sitio a la sorpresa.",
    work: "Los viajes, las ventas, el periodismo, los medios, la consultoría, los eventos y el emprendimiento encajan con el retrato tradicional del 5, igual que los empleos variados y con mucha gente. El trabajo de oficina repetitivo y sin autonomía lo frena.",
    purpose:
      "El propósito que se atribuye al camino de vida 5 es la libertad con responsabilidad: poner su variedad de experiencias al servicio de los demás y entender que la profundidad también puede ser una aventura.",
    compatibleNote:
      "Los numerólogos emparejan tradicionalmente el 5 con el 1, el 3 y el 7, números que aportan independencia, humor o curiosidad sin querer retenerlo.",
  },
  6: {
    title: "El Protector",
    keywords: "cuidado, responsabilidad, armonía",
    intro:
      "El camino de vida 6 es el número de quien cuida. En la numerología pitagórica describe a alguien cuya vida gira en torno a la responsabilidad, el hogar y la comunidad, y que se siente más útil cuando cuida de los demás.",
    archetype:
      "El seis se asocia con la armonía, la familia y el equilibrio. A las personas de este camino se las describe como cálidas, fiables y protectoras, con un fuerte sentido de la justicia y buen ojo para la belleza del hogar y de las relaciones.",
    strengths: [
      "Una calidez que hace sentir a la gente cuidada.",
      "Un fuerte sentido de la responsabilidad y la justicia.",
      "Sentido estético: hacen acogedores los espacios y las reuniones.",
    ],
    challenges: [
      "Cargar con los problemas ajenos sin que se lo pidan.",
      "Perfeccionismo con el hogar, la familia o la imagen.",
      "Un resentimiento, a menudo callado, cuando el cuidado no se devuelve.",
    ],
    love: "El 6 es tradicionalmente la pareja o el progenitor más comprometido, que pone la familia y la lealtad primero. Para él, amar sanamente incluye aceptar cuidados además de darlos y dejar que los demás resuelvan sus propios problemas.",
    work: "La enseñanza, la sanidad, el acompañamiento, la hostelería, el diseño, el trabajo social y los puestos comunitarios encajan con el retrato tradicional del 6, igual que cualquier empleo con un sentido humano claro.",
    purpose:
      "El propósito que se atribuye al camino de vida 6 es el servicio con límites: cuidar de los demás de un modo que también proteja su propia energía y confiar en que el amor no tiene que ganarse con sacrificio.",
    compatibleNote:
      "Los numerólogos emparejan tradicionalmente el 6 con el 2, el 4, el 8 y el 9, números que valoran el compromiso, la lealtad o la generosidad.",
  },
  7: {
    title: "El Buscador de verdad",
    keywords: "análisis, intuición, soledad",
    intro:
      "El camino de vida 7 es el número del buscador. En la numerología pitagórica describe a una persona analítica e introspectiva que busca la verdad bajo la superficie y necesita tiempo a solas para pensar.",
    archetype:
      "El siete lleva mucho tiempo asociado con el estudio, la reflexión y el misterio. A las personas de este camino se las describe como reflexivas, reservadas e intuitivas, atraídas por la investigación, la filosofía, la ciencia o la espiritualidad, y escépticas ante las respuestas fáciles.",
    strengths: [
      "Profundidad analítica: miran más allá de las primeras impresiones.",
      "Una intuición respaldada por una observación cuidadosa.",
      "Comodidad con la soledad y con el estudio largo.",
    ],
    challenges: [
      "Aislamiento: retirarse justo cuando más necesitan conexión.",
      "Cinismo o exceso de análisis que bloquean la confianza.",
      "Dificultad para compartir sus sentimientos, incluso con los más cercanos.",
    ],
    love: "El 7 necesita una pareja que respete el silencio y disfrute de las conversaciones de verdad. La intimidad crece cuando comparte la vida interior que suele guardarse y cuando la pareja no toma su soledad como algo personal.",
    work: "La investigación, el análisis, la escritura, la tecnología, la ciencia, la vida académica, la indagación y el estudio espiritual encajan con el retrato tradicional del 7. Los entornos ruidosos, superficiales o apresurados lo desgastan.",
    purpose:
      "El propósito que se atribuye al camino de vida 7 es la búsqueda de comprensión y luego el regreso hacia los demás: compartir lo que aprende y confiar tanto en el saber interior como en las pruebas.",
    compatibleNote:
      "Los numerólogos emparejan tradicionalmente el 7 con el 5 y el 9, números curiosos que respetan su necesidad de espacio y de ideas.",
  },
  8: {
    title: "El Estratega",
    keywords: "ambición, estrategia, resiliencia",
    intro:
      "El camino de vida 8 es el número de la ambición y el logro. En la numerología pitagórica describe a una persona estratégica y resiliente, atraída por el liderazgo, los resultados y el uso responsable de los recursos.",
    archetype:
      "El ocho, cuya forma se cierra sobre sí misma, se asocia con los ciclos de esfuerzo y recompensa, y con el poder y el dinero. A las personas de este camino se las describe como capaces, decididas y prácticas, con talento para organizar personas y recursos hacia una meta.",
    strengths: [
      "Pensamiento estratégico y cabeza para los números y los recursos.",
      "Resiliencia: los contratiempos suelen afilar su determinación.",
      "Una autoridad que los demás están dispuestos a seguir.",
    ],
    challenges: [
      "Control: dificultad para delegar o confiar en los métodos ajenos.",
      "Adicción al trabajo y costumbre de medir su valor por los resultados.",
      "Tensión entre el éxito material y la vida emocional.",
    ],
    love: "El 8 demuestra su compromiso proveyendo y planeando en pareja, y valora a alguien que sea un verdadero igual. Su trabajo es dejar la oficina en la puerta y mostrar vulnerabilidad, no solo competencia.",
    work: "La dirección, la propiedad de negocios, las finanzas, el derecho, el sector inmobiliario, la política y otros campos de liderazgo y escala encajan con el retrato tradicional del 8, igual que los puestos donde puede construir y dirigir un sistema.",
    purpose:
      "El propósito que se atribuye al camino de vida 8 es el dominio del poder: usar la influencia y los recursos con generosidad y justicia, y medir el éxito por algo más que el dinero o el estatus.",
    compatibleNote:
      "Los numerólogos emparejan tradicionalmente el 8 con el 2, el 4 y el 6, números firmes y de apoyo que equilibran su empuje.",
  },
  9: {
    title: "El Humanitario",
    keywords: "compasión, idealismo, generosidad",
    intro:
      "El camino de vida 9 es el número de la compasión y la culminación. En la numerología pitagórica describe a una persona generosa e idealista que piensa en el panorama general y se siente llamada a ayudar.",
    archetype:
      "El nueve es la última cifra y se asocia con los finales, la sabiduría y el amor universal. A las personas de este camino se las describe como empáticas, idealistas y de mente amplia, con talento para ver todos los lados y deseo de marcar una diferencia.",
    strengths: [
      "Una compasión que llega más allá de su propio círculo.",
      "Una perspectiva amplia: conectan personas y causas.",
      "Generosidad con el tiempo, las habilidades y la atención.",
    ],
    challenges: [
      "Dar hasta el agotamiento o hacerse el mártir.",
      "Aferrarse al pasado, a personas o a viejas heridas.",
      "Un idealismo que la realidad cotidiana decepciona.",
    ],
    love: "El 9 ama con generosidad e idealismo y necesita una pareja que comparta sus valores. Crece dejando que las relaciones terminen bien cuando han cumplido su ciclo, y recibiendo tanto como da.",
    work: "El trabajo en organizaciones sin ánimo de lucro, la sanidad, la enseñanza, las artes, la defensa de causas, el acompañamiento y las carreras humanitarias o internacionales encajan con el retrato tradicional del 9, igual que cualquier puesto que sirva a una causa.",
    purpose:
      "El propósito que se atribuye al camino de vida 9 es soltar: aprender a dar sin llevar la cuenta, dejar ir lo que ha terminado y convertir la compasión en una ayuda constante y práctica.",
    compatibleNote:
      "Los numerólogos emparejan tradicionalmente el 9 con el 3, el 6 y el 7, números que comparten su calidez, su curiosidad o su sentido del cuidado.",
  },
  11: {
    title: "El Iluminador intuitivo",
    keywords: "intuición, inspiración, sensibilidad",
    intro:
      "El camino de vida 11 es el primer número maestro. En la numerología pitagórica se conserva como 11 en lugar de reducirse a 2, y describe a una persona muy intuitiva, inspiradora y sensible que a menudo es fuente de claridad para los demás.",
    archetype:
      "Donde el 2 es el pacificador, el 11 duplica esa sensibilidad y añade visión. A las personas de este camino se las describe como perceptivas e idealistas, con un fuerte sentido del significado y a menudo una energía nerviosa que nace de sentir mucho. Muchos numerólogos ven en el 2 su raíz cotidiana.",
    strengths: [
      "Intuición: captan enseguida los ambientes y los patrones.",
      "Inspiración: saben poner en palabras lo que otros sienten.",
      "Una empatía que hace que la gente se sienta comprendida.",
    ],
    challenges: [
      "Tensión nerviosa, sensación de desbordamiento y dudas sobre sí mismas.",
      "Estar a la altura de altos ideales, lo que puede sentirse como presión.",
      "Aterrizar sus visiones en pasos prácticos de cada día.",
    ],
    love: "El 11 busca un vínculo profundo y con sentido, y capta con mucha finura el ánimo de su pareja. Una pareja con los pies en la tierra y rutinas claras le ayudan a no absorber cada emoción, y hablar con franqueza evita que la sensibilidad se convierta en retraimiento.",
    work: "El acompañamiento, la enseñanza, la escritura, las artes, el diseño, la guía espiritual y otros puestos inspiradores o de consejo encajan con el retrato tradicional del 11. Los entornos duros y puramente competitivos le sientan mal.",
    purpose:
      "El propósito que se atribuye al camino de vida 11 es la iluminación: confiar en su intuición y desarrollarla, anclarla en hábitos prácticos y usarla para animar a otros sin perderse a sí mismo.",
    compatibleNote:
      "Los numerólogos emparejan tradicionalmente el 11 con el 2, el 6 y el 22, números que ofrecen estabilidad, cuidado o un sentido compartido de misión.",
  },
  22: {
    title: "El Maestro constructor",
    keywords: "visión, practicidad, grandes obras",
    intro:
      "El camino de vida 22 es el segundo número maestro. En la numerología pitagórica se conserva como 22 en lugar de reducirse a 4, y describe a una persona que une una gran visión con la habilidad práctica para construirla.",
    archetype:
      "Donde el 4 levanta cimientos, al 22 se lo llama tradicionalmente el maestro constructor: el soñador que además sabe planificar, organizar y terminar. A las personas de este camino se las describe como capaces de grandes proyectos y como quienes sienten la presión de su propio potencial.",
    strengths: [
      "Una visión unida a una planificación práctica, paso a paso.",
      "La capacidad de coordinar personas y recursos a gran escala.",
      "Una constancia que lleva los proyectos largos hasta el final.",
    ],
    challenges: [
      "La presión de las altas expectativas, las propias y las ajenas.",
      "El miedo a quedarse corto, que lleva al retraso o al exceso de trabajo.",
      "Una visión tan grande que se pierde de vista el primer paso.",
    ],
    love: "El 22 aporta estabilidad y lealtad y desea una pareja que comparta la visión a largo plazo. El desafío es dejar espacio para el descanso y el cariño en lugar de llevar la relación como un plan de proyecto.",
    work: "La arquitectura, la ingeniería, la empresa a gran escala, la creación de instituciones, el servicio público, el desarrollo y el emprendimiento encajan con el retrato tradicional del 22, allí donde una gran idea necesita una estructura sólida.",
    purpose:
      "El propósito que se atribuye al camino de vida 22 es convertir una gran visión en algo real y útil para mucha gente, una etapa realista tras otra.",
    compatibleNote:
      "Los numerólogos emparejan tradicionalmente el 22 con el 4, el 8 y el 11, números que respetan la ambición y aportan practicidad, fuerza o perspicacia.",
  },
  33: {
    title: "El Maestro docente",
    keywords: "compasión, guía, servicio",
    intro:
      "El camino de vida 33 es el tercer número maestro. En la numerología pitagórica se conserva como 33 en lugar de reducirse a 6, y describe a una persona entregada a la compasión, la enseñanza y el servicio.",
    archetype:
      "Donde el 6 cuida de la familia y la comunidad, el 33 amplía ese cuidado a todos, y algunos numerólogos lo llaman el maestro docente. No todas las escuelas usan el 33: los números maestros tradicionales son el 11 y el 22, y nuestra calculadora conserva también el 33. Muchos profesionales lo leen como un 6 con una llamada más fuerte a guiar y elevar.",
    strengths: [
      "Una compasión práctica, no solo sentimental.",
      "Un don para enseñar, animar y apoyar con su sola presencia.",
      "Una generosidad desinteresada con el tiempo y la atención.",
    ],
    challenges: [
      "El agotamiento por cargar con las cargas de todos.",
      "Un idealismo tan exigente que la vida ordinaria parece un fracaso.",
      "Dificultad para recibir ayuda o poner límites.",
    ],
    love: "El 33 ama con entrega y ve la relación como un lugar donde nutrir el crecimiento. El riesgo es cuidar de la pareja como si fuera un proyecto; el remedio son límites firmes y dejarse cuidar a su vez.",
    work: "La enseñanza, la sanidad, el acompañamiento, el ministerio, el trabajo social, las artes y el liderazgo comunitario encajan con el retrato tradicional del 33, allí donde la guía y la compasión son el centro del trabajo.",
    purpose:
      "El propósito que se atribuye al camino de vida 33 es elevar a los demás con el ejemplo: dar desde la plenitud y no desde el agotamiento, y tratar el propio bienestar como parte del trabajo.",
    compatibleNote:
      "Los numerólogos emparejan tradicionalmente el 33 con el 6, el 9 y el 11, números que comparten una llamada a cuidar o a inspirar.",
  },
};

export default content;

export const frame: LifePathFrame = {
  eyebrow: "Numerología · Camino de vida",
  title: (n, archetype) => `Camino de vida ${n}: significado (${archetype})`,
  description: (n, keywords) =>
    `El camino de vida ${n} en numerología: ${keywords}. Fortalezas, retos, amor, trabajo, números compatibles y un ejemplo de cálculo, con calculadora gratuita.`,
  h1: (n, archetype) => `Camino de vida ${n}: ${archetype}`,
  headings: {
    meaning: (n) => `Qué significa el camino de vida ${n}`,
    strengths: (n) => `Fortalezas del camino de vida ${n}`,
    challenges: (n) => `Retos del camino de vida ${n}`,
    love: (n) => `El camino de vida ${n} en el amor`,
    work: (n) => `El camino de vida ${n} en el trabajo y su propósito`,
    calculation: (n) => `Cómo se obtiene el camino de vida ${n}`,
    compatible: (n) => `Números compatibles con el camino de vida ${n}`,
  },
  labels: {
    number: "Número",
    kind: "Tipo",
    single: "Número de una cifra",
    master: "Número maestro",
    root: "Número raíz",
    month: "Mes",
    day: "Día",
    year: "Año",
    total: "Total",
  },
  method:
    "Se reducen por separado el mes, el día y el año de nacimiento sumando sus cifras hasta que quede una sola, sin reducir el 11, el 22 ni el 33. Después se suman los tres resultados y el total se reduce de la misma manera.",
  example: (date, n) => `Tomemos a alguien nacido el ${date}. Su camino de vida es ${n}:`,
  masterKept: (n, root) =>
    `El ${n} es un número maestro, así que se conserva como ${n} en lugar de reducirse a ${root}. Muchos numerólogos siguen leyendo las cualidades del ${root} en el fondo.`,
  masterHidden: (n, master) =>
    `Algunos totales pasan por el ${master} antes de llegar al ${n}. Como el ${master} es un número maestro, esas fechas de nacimiento dan el camino de vida ${master} y no el ${n}: comprueba cada paso con la calculadora.`,
  link: (n) => `Camino de vida ${n}`,
  faq: ({ n, master, root, date, entry }) => [
    { q: `¿Qué significa el camino de vida ${n}?`, a: entry.intro },
    {
      q: `¿Cómo sé si mi camino de vida es el ${n}?`,
      a: `Reduce el mes, el día y el año de tu nacimiento a una cifra (conservando el 11, el 22 y el 33), súmalos y reduce el total de la misma manera. Por ejemplo, una persona nacida el ${date} tiene el camino de vida ${n}. La calculadora al final de esta página hace cada paso por ti.`,
    },
    { q: `¿Qué números son compatibles con el camino de vida ${n}?`, a: entry.compatibleNote },
    master
      ? {
          q: `¿Es el ${n} un número maestro?`,
          a: `Sí. El ${n} es uno de los números maestros de la numerología. Cuando aparece se conserva como ${n} en lugar de reducirse a ${root}, y se lee como una versión más exigente del ${root}.`,
        }
      : {
          q: `¿Es el ${n} un número maestro?`,
          a: `No. Los números maestros son el 11, el 22 y el 33. El camino de vida ${n} es un número de una cifra, el resultado final de la reducción.`,
        },
    { q: `¿Qué profesiones encajan con el camino de vida ${n}?`, a: entry.work },
  ],
  related: "Descubre otros caminos de vida",
  hubLink: "Todos los caminos de vida",
  calculatorLink: "Calculadora del número del camino de vida",
  hub: {
    title: "Caminos de vida del 1 al 9, 11, 22 y 33: significados",
    description:
      "Qué significa cada camino de vida en numerología, del 1 al 9 y los números maestros 11, 22 y 33: fortalezas, retos, amor, trabajo y números compatibles. Calculadora gratuita.",
    h1: "Los caminos de vida y su significado",
    intro:
      "El camino de vida es el número principal de la numerología: tu fecha de nacimiento reducida a una sola cifra o a uno de los números maestros 11, 22 y 33. Introduce tu fecha de nacimiento para encontrar el tuyo, o elige un número para leer su significado.",
    gridHeading: "Los doce caminos de vida",
    sections: [
      {
        id: "how-it-works",
        heading: "Cómo se calcula el camino de vida",
        paragraphs: [
          "Los numerólogos reducen por separado el mes, el día y el año de nacimiento sumando sus cifras hasta que queda una sola, y después suman los tres resultados y vuelven a reducir el total. Reducir cada parte primero es lo que permite que aparezcan los números maestros.",
          "Tu camino de vida nunca cambia: sale solo de tu fecha de nacimiento, no de tu nombre.",
        ],
      },
      {
        id: "master-numbers",
        heading: "Los números maestros 11, 22 y 33",
        paragraphs: [
          "Cuando una reducción llega al 11, al 22 o al 33, se detiene ahí. Estos números maestros se leen como versiones más intensas del 2, el 4 y el 6. No todas las escuelas cuentan el 33; nuestra calculadora conserva los tres.",
        ],
      },
    ],
    faq: [
      {
        q: "¿Qué es el número del camino de vida?",
        a: "Es el número que los numerólogos extraen de tu fecha de nacimiento completa. Se dice que describe tus tendencias principales, tus talentos y tus lecciones, y es el número más usado de la numerología pitagórica.",
      },
      {
        q: "¿Cómo calculo mi camino de vida?",
        a: "Reduce el mes, el día y el año de tu nacimiento a una cifra (conservando el 11, el 22 y el 33), súmalos y reduce el total. La calculadora de esta página muestra cada paso.",
      },
      {
        q: "¿Qué son los números maestros?",
        a: "Se llaman números maestros el 11, el 22 y el 33. Cuando una reducción llega a uno de ellos, se conserva en lugar de reducirse a 2, 4 o 6, y se lee como una versión más fuerte y exigente de ese número.",
      },
      {
        q: "¿Puede cambiar mi camino de vida?",
        a: "No. Sale de tu fecha de nacimiento, que no cambia. Otros números, como tu año personal, cambian con el tiempo.",
      },
    ],
    tile: (n) => `Camino de vida ${n}`,
  },
  breadcrumb: { hub: "Caminos de vida", item: (n) => `Camino de vida ${n}` },
  imageAlt: "Cartas de tarot extendidas sobre una mesa de lectura",
};

