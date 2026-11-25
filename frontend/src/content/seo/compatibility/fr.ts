import type { CompatibilityCopy, PairFacts } from "@/content/seo/compatibility/types";
import { listOf } from "@/content/seo/names";

const lower = (value: string) => value.toLowerCase();
/** "de Feu", "d’Air", "d’Eau", "de Terre". */
const de = (element: string) => (/^[AEIOUÉ]/.test(element) ? `d’${element}` : `de ${element}`);
const PLURAL_MODALITY: Record<string, string> = { cardinal: "cardinaux", fixe: "fixes", mutable: "mutables" };

/** "un signe de Feu et un signe d’Air, l’un cardinal, l’autre mutable" (never both the same). */
function makeUp(f: PairFacts): string {
  const elements = f.elementA === f.elementB ? `deux signes ${de(f.elementA)}` : `un signe ${de(f.elementA)} et un signe ${de(f.elementB)}`;
  const [ma, mb] = [lower(f.modalityA), lower(f.modalityB)];
  const modalities = ma === mb ? `tous deux ${PLURAL_MODALITY[ma] ?? ma}` : `l’un ${ma}, l’autre ${mb}`;
  return `${elements}, ${modalities}`;
}

const copy: CompatibilityCopy = {
  aspects: {
    "same-sign": {
      name: "même signe",
      withArticle: "le même signe",
      angle: "0°, le même signe",
      essence:
        "Deux personnes du même signe partagent un élément, une modalité et une planète maîtresse. L’astrologie traditionnelle y voit une reconnaissance immédiate : vous comprenez les motivations de l’autre, mais vous partagez aussi les mêmes angles morts.",
      strength: "Une compréhension immédiate : les mêmes instincts, le même rythme et les mêmes valeurs rendent chacun facile à lire pour l’autre.",
      friction: "Un effet miroir : les traits que vous partagez, y compris les plus difficiles, se doublent au lieu de s’équilibrer.",
      tip: "Faites entrer ce qu’aucun de vous n’a naturellement : des amis, des centres d’intérêt et des habitudes venus d’ailleurs.",
    },
    "semi-sextile": {
      name: "semi-sextile",
      withArticle: "un semi-sextile",
      angle: "30°, signes voisins",
      essence:
        "Deux signes voisins forment un semi-sextile. Ils ne partagent ni élément ni modalité, et la tradition veut que chaque signe naisse du précédent : le lien paraît à la fois proche et étranger.",
      strength: "Des talents complémentaires : chacun possède ce que l’autre est encore en train d’apprendre.",
      friction: "Des instincts différents : des voisins se méprennent souvent sur les priorités et le rythme de l’autre.",
      tip: "Voyez vos différences comme un échange de savoir-faire plutôt qu’une compétition : demandez à l’autre comment il s’y prendrait.",
    },
    sextile: {
      name: "sextile",
      withArticle: "un sextile",
      angle: "60°, à deux signes d’écart",
      essence:
        "Deux signes séparés de deux places forment un sextile et appartiennent à des éléments amis, le Feu avec l’Air ou la Terre avec l’Eau. L’astrologie traditionnelle y voit un lien facile et coopératif qui récompense l’effort : les occasions se présentent quand les deux s’en saisissent.",
      strength: "Une communication facile et un esprit amical et coopératif.",
      friction: "Le confort peut mener à la facilité : le lien fonctionne mieux quand vous le faites vivre activement.",
      tip: "Prévoyez des projets ou des voyages communs : le sextile s’épanouit quand il a de quoi s’occuper.",
    },
    square: {
      name: "carré",
      withArticle: "un carré",
      angle: "90°, à trois signes d’écart",
      essence:
        "Deux signes séparés de trois places forment un carré. Ils partagent une modalité mais pas un élément : ils agissent de la même manière en poursuivant des buts différents. La tradition lit le carré comme une tension qui fait grandir.",
      strength: "Énergie et attirance : la friction garde la relation vivante et pousse chacun à grandir.",
      friction: "Des rapports de force : vous pouvez pousser au même moment dans des directions opposées.",
      tip: "Décidez qui mène quel domaine de la vie, et discutez du problème plutôt que de qui a raison.",
    },
    trine: {
      name: "trigone",
      withArticle: "un trigone",
      angle: "120°, à quatre signes d’écart",
      essence:
        "Deux signes séparés de quatre places forment un trigone et partagent le même élément. L’astrologie traditionnelle voit dans le trigone l’angle le plus harmonieux : un accord naturel de valeurs et de tempérament.",
      strength: "Une harmonie naturelle : des valeurs partagées et un tempérament proche rendent la coopération facile.",
      friction: "Trop de confort : avec si peu de friction, les problèmes peuvent être ignorés jusqu’à ce qu’ils grossissent.",
      tip: "Continuez à vous stimuler avec bienveillance, pour que la facilité ne tourne pas à la routine.",
    },
    quincunx: {
      name: "quinconce",
      withArticle: "un quinconce",
      angle: "150°, à cinq signes d’écart",
      essence:
        "Deux signes séparés de cinq places forment un quinconce. Ils ne partagent ni élément ni modalité, et la tradition y voit un ajustement constant entre deux façons de vivre très différentes.",
      strength: "La fascination de la différence : chacun montre à l’autre une manière de vivre qu’il n’aurait jamais choisie.",
      friction: "Des malentendus : besoins et priorités peuvent sembler décalés sans que personne ait tort.",
      tip: "Dites clairement ce dont vous avez besoin : avec un quinconce, peu de choses vont de soi, et ce n’est pas grave.",
    },
    opposition: {
      name: "opposition",
      withArticle: "une opposition",
      angle: "180°, signes opposés",
      essence:
        "Deux signes opposés se font face de part et d’autre du zodiaque. Ils partagent une modalité et leurs éléments se complètent, et l’astrologie traditionnelle voit dans l’opposition une forte attirance entre les deux moitiés d’un même axe.",
      strength: "Une attirance magnétique : chacun incarne les qualités qui manquent à l’autre.",
      friction: "La polarisation : sous la pression, chacun se retranche à son bout de l’axe.",
      tip: "Cherchez le milieu de l’axe : empruntez la manière de faire de l’autre au lieu de défendre la vôtre.",
    },
  },
  elements: {
    "Fire-Fire": {
      summary: "Deux signes de Feu partagent l’enthousiasme, le courage et le goût de l’action : l’énergie entre eux est vive.",
      strength: "Un enthousiasme partagé : vous vous poussez mutuellement à essayer, à oser et à vous amuser.",
      friction: "Deux feux peuvent s’embraser en même temps : emportements et rivalité pour la lumière.",
      tip: "Passez chacun votre tour au premier plan, et calmez-vous avant de régler un désaccord.",
    },
    "Fire-Earth": {
      summary:
        "Le Feu apporte l’étincelle et la vision, la Terre la patience et le sens pratique. La tradition les dit mal assortis, mais ensemble ils peuvent transformer des idées en résultats concrets.",
      strength: "Les idées rencontrent la persévérance : l’un lance, l’autre fait durer.",
      friction: "Une question de rythme : le Feu trouve la Terre lente, et la Terre trouve le Feu imprudent.",
      tip: "Laissez le Feu choisir la destination et la Terre tracer l’itinéraire, et respectez le rythme de chacun.",
    },
    "Fire-Air": {
      summary: "L’Air nourrit le Feu : les idées et la conversation alimentent l’enthousiasme et l’action. L’astrologie traditionnelle compte le Feu et l’Air parmi les éléments amis.",
      strength: "Un échange animé : les idées deviennent vite des projets et des aventures communes.",
      friction: "L’impatience : aucun des deux éléments n’aime la routine, et le quotidien peut être négligé.",
      tip: "Attaquez ensemble les tâches ennuyeuses, pour que le plaisir repose sur une base solide.",
    },
    "Fire-Water": {
      summary:
        "Le Feu est direct et tourné vers l’extérieur, l’Eau est sensible et tournée vers l’intérieur. La tradition y voit un mélange délicat, qui peut pourtant être intense et passionné quand chacun fait attention.",
      strength: "Passion et profondeur : le Feu apporte chaleur et courage, l’Eau la profondeur des émotions.",
      friction: "Le Feu peut blesser l’Eau sans s’en rendre compte, et les humeurs de l’Eau peuvent éteindre l’élan du Feu.",
      tip: "Le Feu peut ralentir et écouter ; l’Eau peut dire clairement ce qu’elle ressent au lieu de se replier.",
    },
    "Earth-Earth": {
      summary: "Deux signes de Terre partagent un regard pratique, loyal et patient, et tiennent tous deux à la stabilité et à ce qui dure.",
      strength: "La fiabilité : les promesses sont tenues et construire une vie commune vient naturellement.",
      friction: "Routine et entêtement : la vie à deux peut devenir prévisible, et aucun n’aime céder.",
      tip: "Prévoyez régulièrement de la nouveauté, un voyage ou un projet commun, et exercez-vous au compromis sur les petites choses.",
    },
    "Earth-Air": {
      summary: "La Terre veut du concret, l’Air vit d’idées et de conversations. La tradition les dit dépareillés, mais ils ont beaucoup à apprendre l’un de l’autre.",
      strength: "Les idées deviennent pratiques : l’Air apporte du recul, la Terre rend les choses réalisables.",
      friction: "La Terre peut trouver l’Air peu fiable, et l’Air trouver la Terre trop attachée à ses habitudes.",
      tip: "Mettez-vous d’accord sur ce qui est fixe et ce qui reste ouvert, et laissez-vous de la liberté pour le reste.",
    },
    "Earth-Water": {
      summary: "L’Eau nourrit la Terre : la profondeur émotionnelle rencontre l’attention concrète. L’astrologie traditionnelle compte la Terre et l’Eau parmi les éléments amis.",
      strength: "Sécurité et tendresse : vous tenez tous deux à la loyauté, au confort et à un foyer sûr.",
      friction: "Le repli : vous pouvez tous deux vous réfugier dans le silence ou l’inquiétude au lieu de parler.",
      tip: "Nommez vos inquiétudes tôt : vous êtes plus forts quand vous les partagez.",
    },
    "Air-Air": {
      summary: "Deux signes d’Air partagent la curiosité, le goût de la conversation et le besoin d’espace mental ; ils sont rarement à court de sujets.",
      strength: "Une conversation sans fin et beaucoup de liberté pour chacun.",
      friction: "Vivre dans sa tête : sentiments et questions pratiques sont discutés plutôt que réglés.",
      tip: "Transformez les discussions en petits pas concrets, et gardez du temps pour la proximité physique.",
    },
    "Air-Water": {
      summary: "L’Air pense et l’Eau ressent. La tradition y voit un mélange délicat de logique et d’émotion, où chacun peut apprendre à l’autre un nouveau langage.",
      strength: "La tête et le cœur : ensemble, vous comprenez une situation des deux côtés.",
      friction: "L’Air peut sembler distant à l’Eau, et l’Eau peut paraître envahissante à l’Air.",
      tip: "L’Air peut reconnaître les émotions avant de les analyser ; l’Eau peut mettre ses émotions en mots.",
    },
    "Water-Water": {
      summary: "Deux signes d’Eau partagent la sensibilité, l’intuition et la profondeur émotionnelle, et se comprennent souvent sans parler.",
      strength: "Une profonde empathie et une grande proximité émotionnelle.",
      friction: "Les humeurs s’amplifient : inquiétudes et blessures peuvent tourner en boucle sans regard extérieur.",
      tip: "Gardez des amis et des habitudes qui vous ancrent tous les deux, et demandez avant de supposer ce que l’autre ressent.",
    },
  },
  modalities: {
    "Cardinal-Cardinal": {
      summary: "Deux signes cardinaux sont tous deux des initiateurs : chacun veut lancer, décider et mener.",
      friction: "Deux meneurs peuvent se disputer le volant.",
      tip: "Partagez la direction par domaine, pour que chacun ait quelque chose à piloter.",
    },
    "Cardinal-Fixed": {
      summary: "Un signe cardinal lance les choses et un signe fixe les entretient : l’un démarre, l’autre fait durer.",
      friction: "Le signe fixe résiste aux brusques changements de cap du signe cardinal.",
      tip: "Parlez des changements avant de les faire, et laissez le partenaire fixe donner le rythme du suivi.",
    },
    "Cardinal-Mutable": {
      summary: "Un signe cardinal fixe la direction et un signe mutable l’adapte : l’initiative rencontre la souplesse.",
      friction: "Les revirements du signe mutable peuvent contrarier les projets du signe cardinal.",
      tip: "Laissez le partenaire cardinal choisir le but et le partenaire mutable ajuster le chemin.",
    },
    "Fixed-Fixed": {
      summary: "Deux signes fixes sont tous deux constants et déterminés : loyaux, persévérants et lents à changer.",
      friction: "L’entêtement des deux côtés : les désaccords peuvent se figer en impasse.",
      tip: "Convenez à l’avance de la façon de régler vos désaccords, et remerciez-vous quand l’un cède du terrain.",
    },
    "Fixed-Mutable": {
      summary: "Un signe fixe apporte la constance et un signe mutable la faculté d’adaptation : la stabilité rencontre le changement.",
      friction: "Le signe fixe peut trouver le mutable inconstant, et le signe mutable trouver le fixe rigide.",
      tip: "Gardez quelques rituels fixes et laissez le reste ouvert au changement.",
    },
    "Mutable-Mutable": {
      summary: "Deux signes mutables sont tous deux souples et curieux : faciles à vivre, adaptables et ouverts au changement.",
      friction: "Personne ne tient la barre : les projets dérivent et les décisions sont repoussées.",
      tip: "Notez vos objectifs communs et vos échéances, pour que la souplesse ne devienne pas dérive.",
    },
  },
  signs: {
    Aries: {
      traits: "audacieux, directs et prompts à agir",
      brings: "l’initiative et le courage",
      needs: "de place pour mener et agir sur l’instant",
      love: "Les natifs du Bélier aiment avec franchise et passion, et vont volontiers au-devant de ce qu’ils désirent.",
    },
    Taurus: {
      traits: "stables, sensuels et fidèles",
      brings: "la patience et un sentiment de sécurité",
      needs: "de stabilité, de confort et de temps pour décider",
      love: "Les natifs du Taureau aiment lentement et fidèlement, et montrent leur affection par leur présence, le toucher et les attentions.",
    },
    Gemini: {
      traits: "curieux, spirituels et polyvalents",
      brings: "la conversation, l’humour et des idées neuves",
      needs: "de variété et de stimulation intellectuelle",
      love: "Les natifs des Gémeaux aiment par les mots et le jeu, et ont besoin d’un partenaire qui soit aussi un ami.",
    },
    Cancer: {
      traits: "attentionnés, protecteurs et intuitifs",
      brings: "la tendresse et une loyauté affective",
      needs: "de sécurité affective et d’un vrai foyer",
      love: "Les natifs du Cancer aiment en protégeant et n’oublient aucune attention ; ils s’ouvrent quand ils se sentent en sécurité.",
    },
    Leo: {
      traits: "chaleureux, généreux et expressifs",
      brings: "la chaleur, la loyauté et le goût de la fête",
      needs: "de reconnaissance et d’une place où briller",
      love: "Les natifs du Lion aiment en grand et avec générosité, et veulent se sentir admirés en retour.",
    },
    Virgo: {
      traits: "attentifs, pratiques et réfléchis",
      brings: "le soin du détail et une aide concrète",
      needs: "d’ordre, d’utilité et d’une honnêteté bienveillante",
      love: "Les natifs de la Vierge aiment par des gestes utiles et une attention aux détails plutôt que par de grandes déclarations.",
    },
    Libra: {
      traits: "diplomates, gracieux et soucieux d’équité",
      brings: "l’harmonie, le charme et le sens de l’équilibre",
      needs: "d’équité, de beauté et d’un vrai partenariat",
      love: "Les natifs de la Balance aiment avec charme et courtoisie, et recherchent une relation d’égal à égal.",
    },
    Scorpio: {
      traits: "intenses, perspicaces et secrets",
      brings: "la profondeur, la loyauté et la sincérité émotionnelle",
      needs: "de confiance et d’un engagement total",
      love: "Les natifs du Scorpion aiment intensément et loyalement, et attendent la même profondeur en retour.",
    },
    Sagittarius: {
      traits: "aventureux, francs et optimistes",
      brings: "l’enthousiasme, l’humour et des horizons plus larges",
      needs: "de liberté, de voyages et de sens",
      love: "Les natifs du Sagittaire aiment librement et honnêtement, et veulent un partenaire qui soit aussi un compagnon de route.",
    },
    Capricorn: {
      traits: "ambitieux, disciplinés et fiables",
      brings: "la structure, l’engagement et des projets à long terme",
      needs: "de respect, d’objectifs clairs et de progrès réguliers",
      love: "Les natifs du Capricorne aiment sérieusement et sur la durée, et prouvent leurs sentiments par l’engagement.",
    },
    Aquarius: {
      traits: "indépendants, inventifs et idéalistes",
      brings: "l’originalité et un esprit d’amitié",
      needs: "d’indépendance et de place pour leurs idéaux",
      love: "Les natifs du Verseau aiment d’abord en amis, et tiennent à la liberté et à la franchise dans le couple.",
    },
    Pisces: {
      traits: "imaginatifs, empathiques et doux",
      brings: "la compassion, l’imagination et le dévouement",
      needs: "de compréhension et d’espace pour rêver",
      love: "Les natifs des Poissons aiment avec tendresse et romantisme, et sentent souvent ce que ressent l’autre avant qu’il ne le dise.",
    },
  },
  planets: {
    Sun: "le Soleil, astre de l’identité et de la vitalité",
    Moon: "la Lune, astre de l’émotion et de l’instinct",
    Mercury: "Mercure, la planète de la pensée et de la communication",
    Venus: "Vénus, la planète de l’amour, du plaisir et de l’harmonie",
    Mars: "Mars, la planète de l’élan et du courage",
    Jupiter: "Jupiter, la planète de la croissance et de l’optimisme",
    Saturn: "Saturne, la planète de la structure et de la responsabilité",
    Uranus: "Uranus, la planète du changement et de l’originalité",
    Neptune: "Neptune, la planète de l’imagination et de la compassion",
    Pluto: "Pluton, la planète de la profondeur et de la transformation",
  },
  frame: {
    eyebrow: "Astrologie · Compatibilité",
    title: (a, b) => `Compatibilité ${a} et ${b} : amour, amitié et travail`,
    description: (f) =>
      f.same
        ? `Compatibilité ${f.a} et ${f.a} : deux signes ${de(f.elementA)} gouvernés par la même planète. Forces, points de friction, conseils pratiques et scores en amour, en amitié et au travail. Calculateur gratuit.`
        : `Compatibilité ${f.a} et ${f.b} : ${f.aspectWithArticle} entre un signe ${de(f.elementA)} et un signe ${de(f.elementB)}. Forces, points de friction, conseils pratiques et scores en amour, en amitié et au travail. Calculateur gratuit.`,
    h1: (a, b) => `Compatibilité ${a} et ${b}`,
    intro: (f) =>
      f.same
        ? `Deux natifs ${f.ofA} partagent le même élément (${f.elementA}), la même modalité (${lower(f.modalityA)}) et le même maître (${f.rulerA}). En astrologie traditionnelle, c’est ${f.levelText}, fondée sur une reconnaissance immédiate et sur des forces comme des angles morts partagés.`
        : `${f.a} et ${f.b} forment ${f.aspectWithArticle} (${f.angle}), qui réunit ${makeUp(f)}. En astrologie traditionnelle, c’est ${f.levelText}.`,
    pairName: (f) => (f.same ? `deux natifs ${f.ofA}` : `${f.a} et ${f.b}`),
    headings: {
      overview: (pair) => `Comment s’entendent ${pair}`,
      elements: "Éléments et modalités",
      rulers: "Planètes maîtresses",
      strengths: (pair) => `Ce qui fonctionne entre ${pair}`,
      friction: (pair) => `Les points de friction entre ${pair}`,
      tips: "Conseils pratiques",
      indicators: "Amour, amitié et travail",
    },
    labels: {
      element: (of) => `Élément ${of}`,
      modality: (of) => `Modalité ${of}`,
      ruler: (of) => `Maître ${of}`,
      love: "Amour",
      friendship: "Amitié",
      work: "Travail",
      outOf: (value) => `${value} sur 5`,
    },
    portrait: (f, traitsA, traitsB) =>
      f.same ? `Les deux partenaires sont ${traitsA}.` : `Les natifs ${f.ofA} sont ${traitsA} ; ceux ${f.ofB} sont ${traitsB}.`,
    brings: (f, bringsA, bringsB) =>
      f.same ? `Tous deux apportent ${bringsA}, en double dose.` : `Les natifs ${f.ofA} apportent ${bringsA} ; ceux ${f.ofB} apportent ${bringsB}.`,
    needs: (f, needsA, needsB) =>
      f.same
        ? `Tous deux ont besoin ${needsA}, ce qui peut les mettre en concurrence pour la même chose.`
        : `Les natifs ${f.ofA} ont besoin ${needsA}. Ceux ${f.ofB} ont besoin ${needsB}. Faire de la place aux deux fait partie du travail.`,
    rulers: (f, planetA, planetB) =>
      f.same
        ? `Le signe ${f.ofA} est gouverné par ${planetA}. Partager le même maître, c’est vouloir les mêmes choses de la vie, parfois au même moment.`
        : f.sharedRuler
          ? `${f.a} et ${f.b} sont tous deux gouvernés par ${planetA}. Ce maître commun leur donne un terrain d’entente, même si leurs éléments l’expriment différemment.`
          : `Le signe ${f.ofA} est gouverné par ${planetA}, celui ${f.ofB} par ${planetB}. Chaque maître décrit ce que recherche son signe, et accorder les deux fait partie de la relation.`,
    reading: (f) => {
      const scores = f.indicators;
      const max = Math.max(scores.love, scores.friendship, scores.work);
      const names = { love: "en amour", friendship: "en amitié", work: "au travail" } as const;
      const best = (Object.keys(names) as (keyof typeof names)[]).filter((key) => scores[key] === max).map((key) => names[key]);
      return best.length === 3
        ? `Cette association obtient le même score en amour, en amitié et au travail (${max} sur 5).`
        : `Cette association est la plus forte ${listOf("fr", best)} (${max} sur 5).`;
    },
    indicatorNote:
      "Ces scores sont une méthode de l’astrologie traditionnelle, proposée à des fins de divertissement. Ils découlent de l’angle entre les deux signes, de leurs éléments, de leurs modalités et de leurs planètes maîtresses, et non de données sur de vrais couples. Une relation dépend avant tout des personnes, et un thème astral complet en dit bien plus que les signes solaires.",
    levels: {
      harmonious: "l’une des associations les plus harmonieuses",
      good: "une bonne association qui s’enrichit avec l’effort",
      mixed: "une association contrastée, entre attirance et friction",
      demanding: "une association exigeante qui demande de la patience des deux côtés",
    },
    levelShort: { harmonious: "Harmonieuse", good: "Bonne entente", mixed: "Contrastée", demanding: "Exigeante" },
    faq: (f, { essence, loveA, loveB, elementSummary }) => [
      {
        q: f.same ? `Deux natifs ${f.ofA} sont-ils compatibles ?` : `${f.a} et ${f.b} sont-ils compatibles ?`,
        a: f.same
          ? `En astrologie traditionnelle, deux natifs ${f.ofA} forment ${f.levelText}. ${essence}`
          : `En astrologie traditionnelle, ${f.a} et ${f.b} forment ${f.levelText}. ${essence}`,
      },
      {
        q: f.same ? `Deux natifs ${f.ofA} vont-ils bien ensemble en amour ?` : `${f.a} et ${f.b} vont-ils bien ensemble en amour ?`,
        a: f.same
          ? `${loveA} Sur notre échelle traditionnelle, deux natifs ${f.ofA} obtiennent ${f.indicators.love} sur 5 en amour.`
          : `${loveA} ${loveB} Sur notre échelle traditionnelle, cette association obtient ${f.indicators.love} sur 5 en amour.`,
      },
      { q: f.same ? `Qu’ont en commun deux natifs ${f.ofA} ?` : `Qu’ont en commun ${f.a} et ${f.b} ?`, a: elementSummary },
      {
        q: "La compatibilité des signes solaires est-elle fiable ?",
        a: "Les signes solaires ne donnent qu’une image générale. Les astrologues comparent aussi la Lune, Vénus, Mars et l’ascendant des deux thèmes, et une relation dépend des personnes, pas de leurs signes. Cette page est proposée à des fins de divertissement uniquement.",
      },
    ],
    related: "Découvrir d’autres associations",
    hubLink: "Toutes les associations de signes",
    calculatorLink: "Calculateur de signe astrologique",
    hub: {
      title: "Compatibilité amoureuse des signes : les 78 associations",
      description:
        "La compatibilité des signes du zodiaque pour les 78 associations : l’angle entre les signes, les éléments, les modalités, les planètes maîtresses, les forces, les points de friction et des conseils. Saisissez deux dates de naissance pour trouver la vôtre.",
      h1: "Compatibilité des signes du zodiaque",
      intro:
        "La compatibilité astrologique compare deux signes solaires à travers l’angle qui les sépare, leurs éléments, leurs modalités et leurs planètes maîtresses. Saisissez deux dates de naissance pour trouver votre association, ou choisissez l’une des 78 combinaisons ci-dessous.",
      gridHeading: "Toutes les associations de signes",
      gridIntro: "Choisissez votre signe, puis celui de votre partenaire, d’un ami ou d’un collègue.",
      sections: [
        {
          id: "how-it-works",
          heading: "Comment nous lisons la compatibilité",
          paragraphs: [
            "Chaque association se lit à partir de quatre facteurs traditionnels : l’angle entre les deux signes sur la roue du zodiaque, leurs éléments (Feu, Terre, Air, Eau), leurs modalités (cardinale, fixe, mutable) et leurs planètes maîtresses. Nous utilisons les maîtres modernes : le Scorpion est gouverné par Pluton, le Verseau par Uranus et les Poissons par Neptune.",
            "Les scores en amour, en amitié et au travail de chaque page découlent de ces facteurs selon une règle fixe : les deux mêmes signes obtiennent toujours la même réponse. C’est une méthode de divertissement, pas une mesure.",
          ],
        },
      ],
      aspectsHeading: "Les sept relations entre signes",
      aspectLine: (name, angle, essence) => `**${name.charAt(0).toUpperCase()}${name.slice(1)}** (${angle}) : ${essence}`,
      faq: [
        {
          q: "Comment se calcule la compatibilité astrologique ?",
          a: "Elle compare les deux signes solaires : leur distance sur la roue du zodiaque, l’accord de leurs éléments et de leurs modalités, et les planètes qui les gouvernent. Notre calculateur trouve les deux signes à partir de deux dates de naissance.",
        },
        {
          q: "Et si l’un de nous est né sur une cuspide ?",
          a: "Les jours où le Soleil change de signe, le signe dépend de l’heure et du lieu de naissance. Le calculateur vous le signale et vous laisse choisir, ou ajouter votre heure de naissance dans le calculateur de signe astrologique.",
        },
        {
          q: "Quels signes sont les plus compatibles ?",
          a: "L’astrologie traditionnelle favorise les signes du même élément (trigones) et des éléments amis (sextiles), tandis que les signes opposés allient une forte attirance à une certaine tension. Chaque association a ses forces et ses points de friction, et ces pages décrivent les deux.",
        },
        {
          q: "Ces lectures de compatibilité sont-elles des prédictions ?",
          a: "Non. Elles décrivent des thèmes astrologiques traditionnels, à des fins de réflexion et de divertissement. Elles ne peuvent pas dire si une relation va fonctionner.",
        },
      ],
    },
    breadcrumb: { hub: "Compatibilité des signes" },
    imageAlt: "Des rayons de soleil perçant les nuages au lever du jour",
    signAlt: (sign) => `Symbole du signe ${sign}`,
  },
};

export default copy;
