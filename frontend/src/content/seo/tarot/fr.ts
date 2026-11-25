import type { TarotContent, TarotFrame } from "@/content/seo/tarot/types";
import { listOf } from "@/content/seo/names";

const content: TarotContent = {
  "the-fool": {
    keywords: "nouveaux départs, confiance, spontanéité",
    intro:
      "Le Mat est la carte de naissance du tarot de l’esprit libre. Il porte le numéro 0 dans les arcanes majeurs, et dans la méthode des cartes de naissance, un total qui se réduit à 22 se lit comme le Mat, associé à l’Empereur comme carte d’âme.",
    meaning:
      "En carte de naissance, le Mat évoque une vie qui recommence sans cesse : curieuse, confiante et prête à sauter avant que tous les détails soient clairs. La tradition voit dans ce personnage au bord du précipice l’innocence et le potentiel plutôt que la folie, quelqu’un pour qui tout reste à venir.",
    gifts: [
      "L’audace des commencements : elles essaient ce que d’autres repoussent.",
      "L’ouverture et un émerveillement d’enfant.",
      "Une foi dans le fait que les choses peuvent bien se passer, qui se communique aux autres.",
    ],
    shadow: [
      "L’imprudence : sauter sans vérifier le sol.",
      "Éviter l’engagement ou les conséquences.",
      "Une naïveté dont d’autres peuvent profiter.",
    ],
    note: "Le Mat porte le numéro 0, mais dans l’arithmétique des cartes de naissance son nombre de naissance est 22. C’est pourquoi sa constellation inclut l’Empereur : 2 + 2 = 4.",
  },
  "the-magician": {
    keywords: "volonté, savoir-faire, ingéniosité",
    intro:
      "Le Bateleur est la carte de naissance du tarot de la volonté habile. C’est la carte numéro 1 ; pour les dates de naissance actuelles, il apparaît comme carte d’âme, atteinte depuis les nombres de naissance 10 et 19, et non comme premier résultat du calcul.",
    meaning:
      "En carte de naissance, le Bateleur décrit quelqu’un qui fait advenir les choses : il communique bien, apprend vite et transforme l’intention en action. Les outils posés sur la table dans l’image traditionnelle représentent les quatre couleurs, c’est-à-dire les ressources déjà à sa portée.",
    gifts: [
      "Le savoir-faire et l’ingéniosité : elles travaillent avec ce qu’elles ont.",
      "Une communication claire et persuasive.",
      "L’élan qui transforme les idées en actes.",
    ],
    shadow: [
      "L’habileté sans sincérité, ou la tentation de manipuler.",
      "Disperser son talent sur trop de projets.",
      "Une fierté de son habileté qui empêche d’apprendre.",
    ],
  },
  "the-high-priestess": {
    keywords: "intuition, mystère, savoir intérieur",
    intro:
      "La Papesse est la carte de naissance du tarot de l’intuition et du savoir intérieur. C’est la carte numéro 2, et elle peut aussi apparaître comme carte d’âme d’autres nombres de naissance.",
    meaning:
      "En carte de naissance, la Papesse décrit quelqu’un qui observe avant de parler et se fie à ce qu’il perçoit sous la surface. Elle siège entre les colonnes du temple et garde ce qui est caché ; la tradition la relie aux rêves, aux secrets et à la patience.",
    gifts: [
      "Une intuition profonde et le sens de ce qui reste non dit.",
      "La discrétion : on peut leur confier un secret.",
      "Le calme face à l’incertitude.",
    ],
    shadow: [
      "La rétention : garder tant de choses en soi que les autres ne peuvent plus les atteindre.",
      "La passivité : attendre quand il faudrait agir.",
      "Douter des faits visibles au profit d’impressions vagues.",
    ],
  },
  "the-empress": {
    keywords: "attention aux autres, créativité, abondance",
    intro:
      "L’Impératrice est la carte de naissance du tarot de la fécondité, de la créativité et du soin. C’est la carte numéro 3, et elle peut aussi apparaître comme carte d’âme d’autres nombres de naissance.",
    meaning:
      "En carte de naissance, l’Impératrice évoque une personne sensuelle et généreuse qui fait grandir les choses, qu’il s’agisse d’un jardin, d’une famille, d’une entreprise ou d’une œuvre. La tradition la relie à Vénus, à la nature et au plaisir du confort et de la beauté.",
    gifts: [
      "Une chaleur nourricière qui aide les personnes et les projets à s’épanouir.",
      "La créativité et l’amour de la beauté.",
      "La générosité et une appréciation naturelle des plaisirs de la vie.",
    ],
    shadow: [
      "Une sollicitude étouffante ou l’excès de gâterie.",
      "Dépendre du confort ou de l’approbation.",
      "Une énergie créative entièrement consacrée aux autres.",
    ],
  },
  "the-emperor": {
    keywords: "structure, autorité, protection",
    intro:
      "L’Empereur est la carte de naissance du tarot de l’ordre et de l’autorité. C’est la carte numéro 4, et elle apparaît aussi comme carte d’âme d’autres nombres de naissance.",
    meaning:
      "En carte de naissance, l’Empereur décrit quelqu’un qui crée de la structure, pose des limites et prend la responsabilité des autres. La tradition le relie au Bélier, au leadership et au côté protecteur et stabilisant du pouvoir.",
    gifts: [
      "Le leadership et la capacité d’organiser.",
      "Un instinct protecteur et un fort sens du devoir.",
      "Des limites claires et une action décidée.",
    ],
    shadow: [
      "La rigidité et le besoin de contrôle.",
      "Des tendances autoritaires ou le refus d’être remis en question.",
      "Une distance émotionnelle derrière la compétence.",
    ],
  },
  "the-hierophant": {
    keywords: "tradition, enseignement, appartenance",
    intro:
      "Le Pape est la carte de naissance du tarot de la tradition et de l’enseignement. C’est la carte numéro 5, et elle apparaît aussi comme carte d’âme d’autres nombres de naissance.",
    meaning:
      "En carte de naissance, le Pape décrit quelqu’un qui valorise le savoir partagé, le rituel et la communauté, et qui devient souvent enseignant ou mentor. La tradition le relie au Taureau, aux institutions et à la transmission de la sagesse d’une génération à l’autre.",
    gifts: [
      "Enseigner et accompagner avec patience.",
      "Le respect des traditions qui donnent du sens à la vie.",
      "Créer confiance et sentiment d’appartenance dans les groupes.",
    ],
    shadow: [
      "Le dogmatisme : confondre la manière habituelle avec la seule manière.",
      "Le conformisme ou la peur d’être différent.",
      "Se servir des règles pour éviter sa responsabilité personnelle.",
    ],
  },
  "the-lovers": {
    keywords: "choix, union, valeurs",
    intro:
      "L’Amoureux est la carte de naissance du tarot du choix et de l’union. C’est la carte numéro 6 ; son nom évoque la romance, mais son sens plus ancien concerne le fait de choisir en accord avec ses valeurs.",
    meaning:
      "En carte de naissance, l’Amoureux décrit quelqu’un pour qui les relations et les décisions sont centrales : choisir des partenaires, des chemins et des principes qu’il peut pleinement assumer. La tradition relie la carte aux Gémeaux et à l’idée que tout vrai choix définit qui nous sommes.",
    gifts: [
      "Une capacité de lien profond et engagé.",
      "La clarté sur leurs valeurs quand cela compte.",
      "L’aisance dans le dialogue et pour voir les deux côtés.",
    ],
    shadow: [
      "L’indécision, ou laisser les autres choisir à leur place.",
      "Idéaliser ses partenaires et être déçu.",
      "Se définir trop à travers ses relations.",
    ],
  },
  "the-chariot": {
    keywords: "détermination, élan, discipline",
    intro:
      "Le Chariot est la carte de naissance du tarot de la détermination. C’est la carte numéro 7, et elle montre un conducteur qui dirige des forces opposées vers un même but.",
    meaning:
      "En carte de naissance, le Chariot décrit quelqu’un qui réussit par la concentration, la discipline et la maîtrise de soi, en apprenant à tenir ensemble des élans contraires pour avancer. La tradition relie la carte au Cancer, à la protection de ce qui compte et à une victoire qu’il faut mériter.",
    gifts: [
      "La concentration et la détermination.",
      "Une discipline qui transforme les objectifs en résultats.",
      "Le courage de prendre les commandes dans des conditions difficiles.",
    ],
    shadow: [
      "L’agressivité ou le besoin de gagner à tout prix.",
      "Foncer au lieu de s’arrêter pour réévaluer.",
      "Cacher sa vulnérabilité derrière l’élan.",
    ],
  },
  strength: {
    keywords: "courage, patience, force douce",
    intro:
      "La Force est la carte de naissance du tarot du courage intérieur. C’est la carte numéro 8 dans la numérotation Rider-Waite utilisée sur ce site, et elle montre une personne qui apaise un lion par la douceur plutôt que par la force.",
    meaning:
      "En carte de naissance, la Force décrit quelqu’un dont la puissance est faite de patience, de compassion et d’une maîtrise de soi tranquille. La tradition relie la carte au Lion et à l’idée qu’apprivoiser ses propres peurs et impulsions est le plus grand courage.",
    gifts: [
      "Un courage tranquille et une résilience émotionnelle.",
      "La patience envers les autres et envers les émotions difficiles.",
      "Une chaleur qui rassure.",
    ],
    shadow: [
      "Le doute de soi derrière un calme apparent.",
      "Étouffer sa colère ou sa peur au lieu de travailler avec elles.",
      "Se charger des fardeaux des autres.",
    ],
    note: "La numérotation varie selon les traditions. La Golden Dawn et le jeu Rider-Waite numérotent la Force VIII et la Justice XI, en accord avec leurs attributions astrologiques ; le Tarot de Marseille et les jeux plus anciens les inversent. Nous utilisons la numérotation Rider-Waite que suivent la plupart des jeux modernes.",
  },
  "the-hermit": {
    keywords: "sagesse, solitude, guidance",
    intro:
      "L’Hermite est la carte de naissance du tarot de la sagesse intérieure. C’est la carte numéro 9, et elle montre un personnage solitaire qui tient une lanterne levée.",
    meaning:
      "En carte de naissance, l’Hermite décrit quelqu’un qui cherche à comprendre par la réflexion, la solitude et l’étude, et qui éclaire ensuite le chemin des autres. La tradition relie la carte à la Vierge, au discernement et à la connaissance qui vient de l’expérience.",
    gifts: [
      "Une pensée profonde et un don de discernement.",
      "L’aisance dans la solitude et le silence.",
      "Une guidance nourrie par l’expérience vécue.",
    ],
    shadow: [
      "Le repli, la solitude ou l’évitement des autres.",
      "Le perfectionnisme et la sur-analyse.",
      "Garder pour soi ce qu’on sait.",
    ],
  },
  "wheel-of-fortune": {
    keywords: "cycles, changement, occasions",
    intro:
      "La Roue de Fortune est la carte de naissance du tarot des cycles et du changement. C’est la carte numéro 10, et elle montre une roue qui tourne, symbole des fortunes qui montent et descendent.",
    meaning:
      "En carte de naissance, la Roue de Fortune décrit quelqu’un dont la vie avance par chapitres distincts et qui apprend à s’adapter quand les circonstances tournent. La tradition relie la carte à Jupiter, à la chance et à l’expansion, et au rappel qu’aucune phase n’est définitive.",
    gifts: [
      "L’adaptabilité quand les circonstances changent.",
      "Un sens du moment juste et des occasions.",
      "L’optimisme et le recul face aux revers.",
    ],
    shadow: [
      "Laisser trop de place au hasard.",
      "Se sentir victime des circonstances.",
      "L’agitation quand rien ne bouge.",
    ],
  },
  justice: {
    keywords: "vérité, équité, responsabilité",
    intro:
      "La Justice est la carte de naissance du tarot de l’équité et de la vérité. C’est la carte numéro 11 dans la numérotation Rider-Waite utilisée sur ce site, et elle montre un personnage tenant une balance et une épée.",
    meaning:
      "En carte de naissance, la Justice décrit quelqu’un doté d’un fort sens du juste et de l’injuste, qui pèse les situations avec soin et assume la responsabilité de ses choix. La tradition relie la carte à la Balance, à l’équilibre et au principe selon lequel les actes ont des conséquences.",
    gifts: [
      "Le sens de l’équité et un jugement clair.",
      "L’honnêteté, y compris envers soi-même.",
      "Un sens de la responsabilité sur lequel on peut compter.",
    ],
    shadow: [
      "La dureté : se juger ou juger les autres trop sévèrement.",
      "Une pensée rigide du bien et du mal.",
      "La difficulté avec les zones grises et le pardon.",
    ],
    note: "La numérotation varie selon les traditions. La Golden Dawn et le jeu Rider-Waite numérotent la Justice XI et la Force VIII, en accord avec leurs attributions astrologiques ; le Tarot de Marseille et les jeux plus anciens les inversent. Nous utilisons la numérotation Rider-Waite que suivent la plupart des jeux modernes.",
  },
  "the-hanged-man": {
    keywords: "lâcher-prise, nouveau regard, pause",
    intro:
      "Le Pendu est la carte de naissance du tarot du lâcher-prise et du nouveau regard. C’est la carte numéro 12, et elle montre un personnage suspendu avec calme, la tête en bas, par un pied.",
    meaning:
      "En carte de naissance, le Pendu décrit quelqu’un qui apprend en s’arrêtant, en lâchant prise et en voyant les choses sous un autre angle. La tradition relie la carte à l’eau et au sacrifice consenti : renoncer à une chose pour une compréhension plus profonde.",
    gifts: [
      "L’ouverture d’esprit et des points de vue originaux.",
      "La patience d’attendre le bon moment.",
      "La capacité de lâcher ce qui ne leur sert plus.",
    ],
    shadow: [
      "La procrastination présentée comme de la patience.",
      "Le martyre : se sacrifier sans qu’on le demande.",
      "Se sentir bloqué et incapable d’agir.",
    ],
  },
  death: {
    keywords: "fins, transformation, renouveau",
    intro:
      "L’Arcane sans nom, que l’on appelle aussi La Mort, est la carte de naissance du tarot des fins et de la transformation. C’est la carte numéro 13 et, malgré ce surnom, elle ne se lit pas comme une mort littérale.",
    meaning:
      "En carte de naissance, l’Arcane sans nom décrit quelqu’un dont la vie est rythmée par des cycles de changement : laisser partir ce qui est terminé pour que quelque chose de nouveau puisse grandir. La tradition relie la carte au Scorpion, à la profondeur et au renouveau qui suit une perte.",
    gifts: [
      "La capacité de se transformer et de repartir.",
      "Le courage de regarder les vérités difficiles en face.",
      "La profondeur : les sujets difficiles leur font rarement peur.",
    ],
    shadow: [
      "S’accrocher à ce qui est fini, ou couper les liens trop brusquement.",
      "La peur du changement.",
      "Une intensité qui peut mettre les autres mal à l’aise.",
    ],
    note: "Dans le tarot de Marseille, la carte XIII ne porte pas de nom : d’où son appellation d’Arcane sans nom, même si on l’appelle couramment La Mort.",
  },
  temperance: {
    keywords: "équilibre, mesure, harmonie",
    intro:
      "La Tempérance est la carte de naissance du tarot de l’équilibre et de la mesure. C’est la carte numéro 14, et elle montre un ange qui verse un liquide d’une coupe à l’autre.",
    meaning:
      "En carte de naissance, la Tempérance décrit quelqu’un qui mêle les contraires et trouve la voie du milieu : patient, adaptable, apaisant dans son influence. La tradition relie la carte au Sagittaire, à la quête de sens et à l’art de combiner les bonnes proportions.",
    gifts: [
      "La patience et le don de l’équilibre.",
      "Un calme qui aide à résoudre les conflits.",
      "La capacité de réunir des idées et des personnes différentes.",
    ],
    shadow: [
      "Éviter les extrêmes au point que rien d’audacieux n’arrive.",
      "Faire trop de compromis.",
      "L’impatience devant la lenteur du mélange.",
    ],
  },
  "the-devil": {
    keywords: "attachement, désir, libération",
    intro:
      "Le Diable est la carte de naissance du tarot de l’attachement et du désir. C’est la carte numéro 15 et, malgré son nom, elle se lit comme le symbole des habitudes et des peurs qui nous lient, non comme le mal.",
    meaning:
      "En carte de naissance, le Diable décrit quelqu’un aux appétits forts, ambitieux et magnétique, qui apprend à distinguer ce qu’il choisit de ce qui a prise sur lui. La tradition relie la carte au Capricorne, à la vie matérielle et à une liberté qui commence en voyant que les chaînes sont lâches.",
    gifts: [
      "La passion, l’élan et une volonté forte.",
      "Une conscience honnête du désir et de la tentation.",
      "Le sens pratique dans les affaires matérielles.",
    ],
    shadow: [
      "Les compulsions, la dépendance ou l’excès.",
      "Le cynisme sur les motivations des autres.",
      "Se sentir piégé par ses rôles et ses obligations.",
    ],
  },
  "the-tower": {
    keywords: "bouleversement, révélation, percée",
    intro:
      "La Maison Dieu, ou la Tour, est la carte de naissance du tarot du changement soudain et de la révélation. C’est la carte numéro 16, et elle montre une tour frappée par la foudre.",
    meaning:
      "En carte de naissance, la Maison Dieu décrit quelqu’un dont la vie comporte souvent des tournants spectaculaires qui balaient ce qui n’était pas solide, et qui se fortifie en reconstruisant sur des fondations plus vraies. La tradition relie la carte à Mars, à l’énergie et aux percées qui arrivent soudainement.",
    gifts: [
      "La résilience : elles reconstruisent bien.",
      "Une franchise qui perce les illusions.",
      "Le courage en situation de crise.",
    ],
    shadow: [
      "Un drame ou un conflit qui remue les choses sans nécessité.",
      "Résister au changement jusqu’à ce qu’il s’impose.",
      "La peur de l’instabilité.",
    ],
  },
  "the-star": {
    keywords: "espoir, renouveau, inspiration",
    intro:
      "L’Étoile est la carte de naissance du tarot de l’espoir et du renouveau. C’est la carte numéro 17, et elle montre un personnage qui verse de l’eau sous un ciel d’étoiles.",
    meaning:
      "En carte de naissance, l’Étoile décrit quelqu’un qui apporte le calme, l’inspiration et la confiance en l’avenir, souvent après une période difficile. La tradition relie la carte au Verseau, à la générosité et à la foi tranquille que le renouveau est possible.",
    gifts: [
      "Un optimisme qui réconforte les autres.",
      "L’inspiration et un don d’encouragement.",
      "La sérénité et la sincérité.",
    ],
    shadow: [
      "Un idéalisme coupé des étapes concrètes.",
      "La déception quand l’espoir ne se réalise pas tout de suite.",
      "Donner sans compter et oublier de se ressourcer.",
    ],
  },
  "the-moon": {
    keywords: "intuition, imagination, inconscient",
    intro:
      "La Lune est la carte de naissance du tarot de l’imagination et de l’inconscient. C’est la carte numéro 18, et elle montre un chemin entre deux tours sous la lune.",
    meaning:
      "En carte de naissance, la Lune décrit quelqu’un à la vie intérieure vive, à l’intuition riche et sensible aux humeurs et aux rêves, qui apprend à distinguer la peur de l’intuition. La tradition relie la carte aux Poissons, aux cycles et à ce qui n’est pas encore clair.",
    gifts: [
      "L’imagination et la sensibilité artistique.",
      "L’intuition des personnes et des situations.",
      "L’empathie et l’ouverture à l’invisible.",
    ],
    shadow: [
      "L’anxiété, la peur ou la confusion.",
      "L’illusion : voir ce qu’on désire ou ce qu’on craint.",
      "Les sautes d’humeur et la fuite dans l’imaginaire.",
    ],
  },
  "the-sun": {
    keywords: "joie, vitalité, clarté",
    intro:
      "Le Soleil est la carte de naissance du tarot de la joie et de la vitalité. C’est la carte numéro 19, traditionnellement l’une des plus optimistes du jeu.",
    meaning:
      "En carte de naissance, le Soleil décrit quelqu’un qui rayonne de chaleur et d’enthousiasme, ouvert et franc, et qui est le plus vivant quand il partage la réussite et le jeu. La tradition relie la carte au Soleil, à la clarté et à une confiance d’enfant.",
    gifts: [
      "La chaleur et un optimisme naturel.",
      "L’ouverture et l’honnêteté.",
      "Une énergie qui encourage les autres.",
    ],
    shadow: [
      "La naïveté ou le fait d’ignorer les problèmes.",
      "Le besoin d’attention ou d’approbation.",
      "L’excès de confiance.",
    ],
  },
  judgement: {
    keywords: "éveil, bilan, appel",
    intro:
      "Le Jugement est la carte de naissance du tarot de l’éveil et de l’appel intérieur. C’est la carte numéro 20, et elle montre des personnages qui se relèvent au son d’une trompette.",
    meaning:
      "En carte de naissance, le Jugement décrit quelqu’un qui fait honnêtement le bilan de sa vie, pardonne et répond à un appel au changement. La tradition relie la carte au feu, au renouveau et au moment où le passé est assez compris pour être laissé derrière soi.",
    gifts: [
      "Une honnête réflexion sur soi.",
      "La capacité de pardonner et de recommencer.",
      "Un sens de la vocation et de la mission.",
    ],
    shadow: [
      "L’autocritique et le jugement sévère envers soi-même.",
      "Ruminer le passé.",
      "Ignorer l’appel parce que le changement semble trop grand.",
    ],
  },
  "the-world": {
    keywords: "accomplissement, unité, réussite",
    intro:
      "Le Monde est la carte de naissance du tarot de l’accomplissement et de l’unité. C’est la carte numéro 21, et elle montre un personnage qui danse au centre d’une couronne.",
    meaning:
      "En carte de naissance, le Monde décrit quelqu’un qui mène les choses à leur terme, relie le local au global et se sent à l’aise dans plusieurs mondes. La tradition relie la carte à Saturne, à la maîtrise acquise avec le temps et à la satisfaction d’un cycle bien achevé.",
    gifts: [
      "Le sens du suivi et de l’achèvement.",
      "Une vision large et l’aisance avec différentes cultures ou différents domaines.",
      "L’intégration : réunir les différentes parties de la vie.",
    ],
    shadow: [
      "Hésiter à finir par peur de la suite.",
      "Le perfectionnisme : ce n’est jamais tout à fait complet.",
      "La complaisance après la réussite.",
    ],
  },
};

export default content;

const list = (items: string[]) => listOf("fr", items);

export const frame: TarotFrame = {
  eyebrow: "Tarot · Cartes de naissance",
  title: (card) => `${card}, carte de naissance du tarot : signification`,
  description: (card, keywords) =>
    `${card} en carte de naissance du tarot : ${keywords}. Son numéro, son attribution selon la Golden Dawn, ses dons, son ombre, sa constellation et un exemple de calcul, avec un calculateur gratuit.`,
  h1: (card) => `${card}, carte de naissance du tarot`,
  headings: {
    meaning: (card) => `${card} en carte de naissance`,
    facts: "Numéro, chiffre romain et attribution de la Golden Dawn",
    gifts: () => "Les dons de cette carte",
    shadow: () => "La part d’ombre de cette carte",
    constellation: () => "Sa constellation",
    example: "Un exemple de calcul",
  },
  labels: {
    arcana: "Numéro dans les arcanes majeurs",
    numeral: "Chiffre romain",
    birthNumber: "Nombre de naissance",
    letter: "Lettre hébraïque",
    attribution: "Attribution de la Golden Dawn",
    element: "élément",
    planet: "planète",
    sign: "signe du zodiaque",
    total: "Mois + jour + année",
    personality: "carte de personnalité",
    middle: "carte intermédiaire",
    soul: "carte d’âme",
    both: "carte de personnalité et carte d’âme",
  },
  attribution: (card, letter, value, kind) =>
    `Dans le système de la Golden Dawn, qui a inspiré la plupart des jeux modernes, ${card} correspond à la lettre hébraïque ${letter} et ${kind === "element" ? `à l’élément ${value}` : kind === "planet" ? (value === "le Soleil" ? "au Soleil" : value === "la Lune" ? "à la Lune" : `à la planète ${value}`) : `au signe ${value}`}.`,
  constellation: ({ card, birthNumber, chain, soulOf, family, soulOnly }) => {
    const lines: string[] = [];
    if (soulOnly)
      lines.push(`Pour les dates de naissance depuis 1900, le total n’est jamais ${birthNumber} : cette carte (${card}) n’est jamais une première carte, elle apparaît comme carte d’âme de ${list(soulOf.map((item) => `${item.number} (${item.card})`))}.`);
    else if (chain.length === 1)
      lines.push(`Quand votre total est ${birthNumber}, ${card} est à la fois votre carte de personnalité et votre carte d’âme.`);
    else if (chain.length === 2)
      lines.push(`Quand votre total est ${birthNumber}, ${card} est votre carte de personnalité et ${chain[1]} votre carte d’âme.`);
    else
      lines.push(`Quand votre total est ${birthNumber}, vous avez trois cartes : ${card} comme carte de personnalité, puis ${chain[1]}, puis ${chain[2]} comme carte d’âme.`);
    if (!soulOnly && soulOf.length)
      lines.push(`Cette carte est aussi la carte d’âme de ${list(soulOf.map((item) => `${item.number} (${item.card})`))}.`);
    lines.push(`Sa constellation, c’est-à-dire les cartes qui partagent la même carte d’âme, réunit ${list(family)}.`);
    return lines;
  },
  example: (date) => `Prenons une personne née le ${date}. On additionne le mois, le jour et l’année, puis les chiffres du total jusqu’à obtenir 22 ou moins :`,
  faq: ({ card, entry, constellation, attribution, date, birthNumber, soulOnly, soulOf }) => [
    { q: `Que signifie ${card} en carte de naissance ?`, a: entry.meaning },
    {
      q: `Comment savoir si ${card} est ma carte de naissance ?`,
      a: soulOnly
        ? `Additionnez le mois, le jour et l’année de votre naissance, puis les chiffres du total jusqu’à obtenir 22 ou moins. Cette carte est la carte d’âme des totaux ${soulOf}. Par exemple, une personne née le ${date} l’a parmi ses cartes. Le calculateur en bas de cette page fait le calcul pour vous.`
        : `Additionnez le mois, le jour et l’année de votre naissance, puis les chiffres du total jusqu’à obtenir 22 ou moins. Un total de ${birthNumber} donne cette carte. Par exemple, une personne née le ${date} l’a parmi ses cartes. Le calculateur en bas de cette page fait le calcul pour vous.`,
    },
    { q: `Quelle est la constellation de ${card} ?`, a: constellation },
    { q: `Quelle correspondance astrologique a ${card} ?`, a: attribution },
    {
      q: "Une carte de naissance prédit-elle mon avenir ?",
      a: "Non. Les cartes de naissance sont une tradition de réflexion : elles décrivent des thèmes à méditer, pas des événements à venir. Elles sont proposées à des fins de divertissement uniquement.",
    },
  ],
  related: "Découvrir les autres cartes de naissance",
  hubLink: "Toutes les cartes de naissance",
  calculatorLink: "Calculateur de carte de naissance du tarot",
  hub: {
    title: "Cartes de naissance du tarot : les 22 arcanes majeurs",
    description:
      "Ce que signifie chaque carte de naissance du tarot, du Bateleur au Mat : numéros, attributions de la Golden Dawn, dons, ombres et constellations. Trouvez vos cartes avec le calculateur gratuit.",
    h1: "Les cartes de naissance du tarot et leur signification",
    intro:
      "Vos cartes de naissance sont les arcanes majeurs qui correspondent à la somme de votre date de naissance, une méthode popularisée par l’autrice de tarot Mary K. Greer. La plupart des gens en ont deux : une carte de personnalité et une carte d’âme. Saisissez votre date de naissance pour trouver les vôtres, ou choisissez une carte ci-dessous.",
    gridHeading: "Les 22 cartes de naissance",
    sections: [
      {
        id: "how-it-works",
        heading: "Comment se calculent les cartes de naissance",
        paragraphs: [
          "Additionnez le mois, le jour et l’année de votre naissance en nombres entiers. Additionnez ensuite les chiffres du total jusqu’à obtenir 22 ou moins : ce nombre est votre carte de personnalité. S’il a deux chiffres, additionnez-les encore une fois pour trouver votre carte d’âme. Un total de 22 correspond au Mat, dont la carte d’âme est l’Empereur.",
          "Les cartes 10 à 22 sont toujours des cartes de personnalité ; les cartes 1 à 9 peuvent être des cartes d’âme. Certains totaux donnent trois cartes : 19 mène au Soleil, à la Roue de Fortune puis au Bateleur.",
        ],
      },
      {
        id: "constellations",
        heading: "Les constellations",
        paragraphs: [
          "Les cartes qui partagent la même carte d’âme forment une constellation. L’Étoile et la Force vont ensemble car 1 + 7 = 8, de même que le Soleil, la Roue de Fortune et le Bateleur partagent la carte d’âme 1.",
        ],
      },
    ],
    faq: [
      {
        q: "Qu’est-ce qu’une carte de naissance du tarot ?",
        a: "C’est un arcane majeur lié à votre date de naissance. Les tarologues s’en servent comme d’un thème de vie à méditer, comme les astrologues utilisent le signe solaire.",
      },
      {
        q: "Quelle différence entre carte de personnalité et carte d’âme ?",
        a: "La carte de personnalité est le premier total de 22 ou moins ; elle décrirait votre façon de vous exprimer. La carte d’âme est ce nombre réduit à un chiffre ; elle décrirait votre but plus profond. Quand le total n’a déjà qu’un chiffre, c’est la même carte.",
      },
      {
        q: "Pourquoi le Mat porte-t-il le numéro 22 ?",
        a: "Le Mat est la carte 0 des arcanes majeurs, mais une somme n’est jamais nulle : la méthode des cartes de naissance lui donne donc le numéro 22, la place qui suit le Monde.",
      },
      {
        q: "Quelle numérotation utilisez-vous ?",
        a: "Nous suivons la numérotation Rider-Waite, héritée de la Golden Dawn : la Force est VIII et la Justice XI. Le Tarot de Marseille et les jeux plus anciens inversent ces deux cartes.",
      },
    ],
    tile: (birthNumber, numeral) => `Nombre de naissance ${birthNumber} · ${numeral}`,
  },
  breadcrumb: { hub: "Cartes de naissance du tarot" },
  cardAlt: (card) => `Carte de tarot : ${card}`,
};

