// Figma 482:2010 (node 1258:1317) — Natal Chart Results, long-form section (French).
import type { ArticleContent } from "@/content/calculators/types";
import { natalDiagramImage, natalIntroImage } from "@/content/calculators/natal/en";

const content: ArticleContent = {
  kind: "natal",
  label: "À propos du tirage de tarot des Douze Maisons",
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
              text: "L’intégration méthodique de la géométrie astrologique et du symbolisme cartomantique constitue l’une des évolutions les plus durables des traditions divinatoires occidentales. Parmi les différents tirages conçus pour réunir ces deux systèmes, le tirage de tarot des Douze Maisons sert de cadre de diagnostic complet. En plaçant les cartes du tarot sur les douze divisions du thème astral, ce tirage projette la nature multiple de l’expérience humaine sur une roue ordonnée.",
            },
          ],
          image: { ...natalIntroImage, alt: "Cartes de tarot étalées sur un tissu à motifs" },
        },
      ],
    },
    {
      id: "history",
      blocks: [
        {
          type: "p",
          text: "L’art de disposer des cartes divinatoires sur une roue astrologique a évolué pendant plus de deux siècles, de la cartomancie française de la fin du XVIIIᵉ siècle aux publications populaires du XXᵉ siècle, en passant par les sociétés ésotériques victoriennes.",
        },
        {
          type: "timeline",
          label: "Chronologie historique",
          items: [
            { title: "Etteilla", period: "(1785–1791)" },
            { title: "Golden Dawn", period: "(1888)" },
            { title: "Russell Grant", period: "(1992)" },
            { title: "Calculateurs en ligne" },
          ],
        },
        { type: "h3", id: "etteilla", text: "L’ère pré-ésotérique et l’époque d’Etteilla (1785–1791)" },
        {
          type: "p",
          weight: "light",
          text: "Le rapprochement formel du tarot et de l’astrologie a été initié par Jean-Baptiste Alliette (1738–1791), cartomancien français influent qui publiait sous le pseudonyme d’Etteilla. Avant son œuvre, les diseurs de bonne aventure tiraient généralement les cartes une à une du haut du paquet et lisaient chacune isolément. Avec son ouvrage de 1770, Manière de se récréer avec un jeu des cartes, puis ses publications sur le tarot entre 1785 et 1791 — dont Jeu des tarots ou le livre de Thot —, Etteilla a posé les bases de la cartomancie structurelle.",
        },
        {
          type: "p",
          weight: "light",
          text: "Etteilla a introduit le premier tirage documenté en forme de roue astrologique à 12 maisons, qu’il appelait le « tirage de l’Horoscope » ou « tirage des Douze ». Il a attribué des signes du zodiaque, des corps planétaires et des tempéraments élémentaires précis à chaque carte, montrant que placer les cartes dans des maisons astrologiques déterminées modifie leur interprétation de base selon le domaine régi par chaque maison.",
        },
        { type: "h3", id: "golden-dawn", text: "La synthèse de la Golden Dawn (1888)" },
        {
          type: "p",
          weight: "light",
          text: "Si Etteilla a établi les premières correspondances, l’Ordre hermétique de la Golden Dawn — fondé en 1888 — a codifié la synthèse ésotérique du tarot, de l’astrologie et de la kabbale. Le système de la Golden Dawn associe directement les 22 arcanes majeurs aux 22 sentiers de l’Arbre de vie, qui correspondent aux 12 signes du zodiaque, aux 7 planètes classiques et aux 3 éléments primordiaux.",
        },
        {
          type: "p",
          weight: "light",
          text: "Dans le grand rituel divinatoire de la Golden Dawn, appelé l’Ouverture de la Clé, la deuxième opération impose explicitement de répartir le jeu en douze tas distincts correspondant aux maisons astrologiques du ciel. Cette opération avait une fonction de diagnostic précise : la maison où tombait la carte Significatrice du consultant indiquait le domaine de vie exact dans lequel se manifestait la question posée.",
        },
        { type: "h3", id: "russell-grant", text: "La popularisation de la fin du XXᵉ siècle et l’Astro-Tarot de Russell Grant (1992)" },
        {
          type: "p",
          weight: "light",
          text: "En 1992, l’astrologue britannique Russell Grant a publié le Russell Grant’s Astro-Tarot Pack, édité par Virgin Books et illustré par Kay Smith. Grant a rendu la cartomancie ésotérique accessible à un plus large public en concevant un jeu hybride personnalisé de 64 cartes.",
        },
        {
          type: "p",
          weight: "light",
          text: "Grant a modifié la structure standard de 78 cartes en supprimant les cartes numérotées des arcanes mineurs (du 2 au 10) tout en conservant les 22 arcanes majeurs et les 4 as. Pour compléter le jeu de 64 cartes, il a ajouté des cartes spécifiques :",
        },
        {
          type: "ul",
          items: [
            "12 cartes représentant les signes du zodiaque occidental.",
            "12 cartes représentant les animaux du zodiaque chinois.",
            "14 cartes représentant les corps planétaires, les phases lunaires (nouvelle lune et pleine lune) et les angles astrologiques clés (Ascendant, Milieu du Ciel et Nœud Nord).",
          ],
        },
        {
          type: "p",
          weight: "light",
          text: "Le guide de Grant présentait le tirage des Douze Maisons comme son tirage d’analyse de référence. En associant un jeu au design accessible à des définitions positionnelles des maisons, Grant a montré comment les lecteurs pouvaient s’appuyer sur les positions astrologiques pour ancrer le sens des cartes dans des domaines de vie concrets, comme l’emploi, le logement et les relations personnelles.",
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
          text: "Mécanique structurelle et fondements architecturaux du tirage",
        },
        {
          type: "split",
          blocks: [
            {
              type: "p",
              weight: "medium",
              text: "Le tirage des Douze Maisons se construit en plaçant douze cartes sur une roue dans le sens inverse des aiguilles d’une montre, qui reproduit la séquence naturelle du zodiaque tropical. La disposition commence par la première maison à la position traditionnelle de l’Ascendant (9 heures) et se poursuit dans l’ordre autour du cercle jusqu’à la douzième maison.",
            },
            {
              type: "p",
              text: "Bien interpréter le tirage des Douze Maisons suppose d’évaluer les relations structurelles sur l’ensemble du thème plutôt que d’analyser douze cartes isolées. Les lecteurs expérimentés s’appuient sur la triangulation entre maisons et sur la dynamique des polarités pour obtenir des lectures nuancées.",
            },
          ],
          image: {
            ...natalDiagramImage,
            alt: "Diagramme « Energy Mapping Tarot Reading » : douze positions de cartes disposées en roue et numérotées de 1 à 12",
          },
        },
        {
          type: "twoColumnList",
          items: [
            { title: "Maison 1", subtitle: "Mars / Bélier (Ascendant) Soi et identité", body: ["Apparence physique, personnalité profonde, vitalité générale, projets personnels et perspectives immédiates. Elle sert de point d’ancrage principal à l’ensemble de la question."] },
            { title: "Maison 2", subtitle: "Vénus / Taureau Valeurs matérielles et argent", body: ["Finances personnelles, liquidités, revenus, biens personnels, sécurité matérielle, estime de soi et achats importants."] },
            { title: "Maison 3", subtitle: "Mercure / Gémeaux Pensée et communication", body: ["Préoccupations quotidiennes, déplacements courts, communication immédiate, fratrie, voisins, écriture et pensées dominantes du quotidien."] },
            { title: "Maison 4", subtitle: "Lune / Cancer (Fond du Ciel) Foyer et origines", body: ["Vie domestique, lieu de résidence, origines familiales, sécurité affective, fondations psychologiques, déménagements et liens maternels."] },
            { title: "Maison 5", subtitle: "Soleil / Lion Créativité et romance", body: ["Nouvelles romances, loisirs, relations légères, expression créative, passe-temps, divertissement, expression de soi et enfants."] },
            { title: "Maison 6", subtitle: "Mercure / Vierge Travail quotidien et bien-être", body: ["Conditions de travail au quotidien, service routinier, habitudes personnelles, santé physique, environnement professionnel et responsabilités quotidiennes."] },
            { title: "Maison 7", subtitle: "Vénus / Balance (Descendant) Partenariats engagés", body: ["Mariage, partenariats d’affaires, contrats, relations publiques, alliances juridiques et adversaires déclarés."] },
            { title: "Maison 8", subtitle: "Pluton / Scorpion Transformation et biens partagés", body: ["Finances communes, dettes conjointes, impôts, héritages, investissements émotionnels profonds, renaissance psychologique, intimité et cycles de vie."] },
            { title: "Maison 9", subtitle: "Jupiter / Sagittaire Pensée supérieure et expansion", body: ["Études supérieures, affaires juridiques, voyages lointains, convictions philosophiques, édition, repères moraux et changements de perspective."] },
            { title: "Maison 10", subtitle: "Saturne / Capricorne (Milieu du Ciel) Statut public et carrière", body: ["Ambitions professionnelles à long terme, réputation professionnelle, position sociale, devoir, autorité dans l’entreprise et influence paternelle."] },
            { title: "Maison 11", subtitle: "Uranus / Verseau Réseaux sociaux et aspirations", body: ["Cercles d’amis, appartenance à des groupes, idéaux à long terme, engagement collectif, projets d’avenir et intérêts amoureux nés d’une amitié."] },
            { title: "Maison 12", subtitle: "Neptune / Poissons Inconscient et retraite", body: ["Solitude, motivations cachées, schémas inconscients, retraite spirituelle, obstacles invisibles, peurs enfouies et événements prêts à émerger."] },
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
          text: "Techniques d’interprétation avancées : triades, axes et dynamiques de diagnostic",
        },
        { type: "h3", id: "cross-house-triangulation", text: "La triangulation entre maisons" },
        {
          type: "p",
          weight: "semibold",
          text: "La triangulation consiste à analyser ensemble des maisons liées afin d’obtenir un contexte approfondi sur des sujets personnels précis.",
        },
        {
          type: "ul",
          items: [
            "**La triade relationnelle (maisons 5, 7 et 8) :** La maison 5 révèle l’intérêt amoureux initial, l’attirance et les relations légères. La maison 7 régit les engagements officiels, le mariage et la dynamique du couple sur le long terme. La maison 8 reflète l’intimité émotionnelle profonde, la compatibilité sexuelle et les obligations financières partagées. Des cartes favorables en maison 5 associées à des cartes difficiles en maison 7 annoncent une romance enthousiasmante qui se heurte à des frictions structurelles au moment de s’engager durablement.",
            "**L’axe de la carrière et de la réussite (maisons 6 et 10) :** La maison 6 régit les tâches quotidiennes, l’emploi courant et la satisfaction professionnelle immédiate. La maison 10 représente l’ambition de carrière à long terme, la réputation publique et la réussite professionnelle. Les écarts entre ces maisons mettent en évidence des déséquilibres professionnels ; des cartes fortes en maison 10 combinées à des cartes tendues en maison 6 signalent une reconnaissance professionnelle obtenue au prix d’un travail quotidien épuisant.",
            "**L’axe de la pensée et de la perspective (maisons 3 et 9) :** La maison 3 décrit les pensées quotidiennes, les soucis immédiats et les échanges de proximité. La maison 9 représente la philosophie de vie, les études supérieures et une vision plus large. Lorsqu’un consultant subit une surcharge mentale en maison 3, des cartes négatives en maison 9 indiquent qu’une perspective trop étroite freine son évolution personnelle.",
          ],
        },
        { type: "h3", id: "polarity-axes", text: "Axes de polarité et tension structurelle" },
        {
          type: "p",
          weight: "semibold",
          text: "Les maisons astrologiques forment six paires complémentaires directement opposées sur la roue. L’analyse de ces axes opposés révèle les tensions fondamentales et les besoins d’équilibre :",
        },
        {
          type: "ul",
          items: [
            "**Maison 1 / Maison 7 (l’axe de l’identité) :** Équilibre l’autonomie personnelle et les objectifs propres avec les obligations et les compromis qu’exigent les relations engagées.",
            "**Maison 2 / Maison 8 (l’axe des ressources) :** Oppose les revenus individuels et les biens personnels aux investissements partagés, aux dettes conjointes, aux impôts et aux biens du couple.",
            "**Maison 3 / Maison 9 (l’axe de la pensée) :** Met en regard l’information concrète de proximité et la pensée quotidienne avec la philosophie abstraite, les savoirs supérieurs et les visions du monde.",
            "**Maison 4 / Maison 10 (l’axe des origines et du statut) :** Équilibre la vie privée et domestique, la sécurité familiale et les racines psychologiques avec les objectifs de carrière publics, la réputation et le devoir social.",
          ],
        },
      ],
    },
  ],
};

export default content;
