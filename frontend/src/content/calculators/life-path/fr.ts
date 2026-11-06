// Figma 1279:1749 — Path Of Life Results, long-form section (French).
import type { ArticleContent } from "@/content/calculators/types";

const content: ArticleContent = {
  kind: "life-path",
  label: "Comprendre votre nombre du Chemin de vie",
  sections: [
    {
      id: "what-is-a-life-path-number",
      blocks: [
        { type: "h2", id: "what-is-a-life-path-number", text: "Qu’est-ce que le nombre du Chemin de vie ?" },
        {
          type: "p",
          text: "En numérologie pythagoricienne, votre nombre du Chemin de vie est la coordonnée la plus importante de votre thème. Calculé directement à partir de votre date de naissance complète, il représente le plan central de votre âme, les traits de caractère essentiels que vous portez et les principales leçons que vous êtes appelé à maîtriser au cours de votre vie.",
        },
        {
          type: "p",
          text: "Si votre nom de naissance révèle vos talents actifs et vos désirs profonds, votre nombre du Chemin de vie définit le cadre général et les thèmes que vous rencontrerez. Que vous portiez une vibration racine à un chiffre (de 1 à 9) ou un Nombre Maître intensifié (11, 22 ou 33), comprendre votre nombre éclaire vos forces naturelles, votre orientation professionnelle et la dynamique de vos relations.",
        },
      ],
    },
    {
      id: "how-to-calculate",
      blocks: [
        { type: "h2", id: "how-to-calculate", text: "Comment calculer correctement votre nombre du Chemin de vie" },
        {
          type: "p",
          weight: "light",
          text: "Tous les calculateurs de numérologie ne se valent pas. De nombreux sites automatisés utilisent une simple méthode d’« addition linéaire » : additionner tous les chiffres d’une date de naissance sur une seule ligne. Rapide, ce raccourci efface pourtant les Nombres Maîtres intermédiaires pour environ 1 date de naissance sur 7.",
        },
        {
          type: "p",
          weight: "light",
          text: "Notre calculateur utilise la méthode traditionnelle de réduction par composantes (ou des cycles de période). Cette approche réduit séparément les trois éléments de votre date de naissance — le mois, le jour et l’année — avant de les combiner.",
        },
      ],
    },
    {
      id: "component-reduction-method",
      blocks: [
        { type: "h2", id: "component-reduction-method", text: "La méthode de réduction par composantes en 3 étapes" },
        {
          type: "ol",
          items: [
            "Réduire le mois de naissance : ramenez votre mois de naissance à un seul chiffre (de 1 à 9) ou conservez le Nombre Maître 11 (novembre).",
            "Réduire le jour de naissance : ramenez votre jour de naissance à un seul chiffre ou conservez les Nombres Maîtres 11 ou 22 (par exemple le 11, le 22 ou le 29).",
            "Réduire l’année de naissance : additionnez les quatre chiffres de votre année de naissance et ramenez la somme à un seul chiffre ou à un Nombre Maître.",
            "Additionner les résultats : additionnez les trois valeurs réduites et effectuez une réduction finale, sauf si le total est égal à 11, 22 ou 33, auquel cas le Nombre Maître est conservé.",
          ],
        },
        { type: "h3", id: "worked-example", text: "Exemple détaillé : 29 novembre 1990" },
        {
          type: "ul",
          items: [
            "Mois (novembre) : 11 (Nombre Maître conservé)",
            "Jour (29) : 2 + 9 = 11 (Nombre Maître conservé)",
            "Année (1990) : 1 + 9 + 9 + 0 = 19 ⟹ 1 + 9 = 10 ⟹ 1 + 0 = 1",
            "Somme finale : 11 + 11 + 1 = 23 ⟹ 2 + 3 = 5 ⟹ Chemin de vie 5",
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
            { title: "Chemin de vie 1", subtitle: "Le Meneur", body: ["Innovation, indépendance, autonomie et vision pionnière.", "Se garder du manque de confiance en soi, de l’impatience et des tendances autoritaires."] },
            { title: "Chemin de vie 2", subtitle: "Le Pacificateur", body: ["Diplomatie, empathie, écoute active et harmonie relationnelle.", "Apprivoiser l’hypersensibilité, la complaisance excessive et la codépendance."] },
            { title: "Chemin de vie 3", subtitle: "Le Communicant", body: ["Expression créative, optimisme, sens artistique et charisme.", "Surmonter la dispersion, la superficialité et la répression des émotions."] },
            { title: "Chemin de vie 4", subtitle: "Le Bâtisseur", body: ["Discipline, organisation pratique, exécution méthodique et loyauté.", "Éviter la rigidité extrême, l’entêtement et la peur des changements soudains."] },
            { title: "Chemin de vie 5", subtitle: "L’Esprit libre", body: ["Adaptabilité, ingéniosité, esprit d’aventure et polyvalence.", "Résister à l’agitation, à l’impulsivité et à la peur de l’engagement durable."] },
            { title: "Chemin de vie 6", subtitle: "Le Protecteur", body: ["Attention à la communauté, harmonie du foyer, sens des responsabilités et équilibre.", "Apprendre à servir sans tomber dans le sacrifice de soi ni dans le contrôle."] },
            { title: "Chemin de vie 7", subtitle: "Le Chercheur", body: ["Recherche analytique, quête spirituelle, intuition et maîtrise technique.", "Prévenir l’isolement cynique, la sur-analyse et le détachement émotionnel."] },
            { title: "Chemin de vie 8", subtitle: "La Force motrice", body: ["Leadership, maîtrise financière, vision stratégique et autorité.", "Équilibrer l’ambition matérielle avec la responsabilité éthique et la générosité."] },
            { title: "Chemin de vie 9", subtitle: "L’Humaniste", body: ["Compassion universelle, maturité artistique, altruisme et accomplissement.", "Se détacher du passé et poser des limites saines."] },
          ],
        },
      ],
    },
    {
      id: "master-numbers",
      blocks: [
        { type: "h2", id: "master-numbers", text: "Les Nombres Maîtres sacrés : 11, 22 et 33" },
        {
          type: "p",
          text: "En numérologie, les nombres à deux chiffres identiques (11, 22, 33) portent un potentiel vibratoire accru. Appelés Nombres Maîtres, ils agissent comme des octaves supérieures de leur racine à un chiffre (2, 4 et 6).",
        },
        {
          type: "ul",
          items: [
            "**Chemin de vie 11/2 (L’Illuminateur intuitif) :** Associe les qualités pacificatrices de la racine 2 à une intuition à haute tension et à une vision inspirée. Les personnes du Chemin 11 agissent comme des antennes spirituelles, même si elles doivent souvent composer avec une forte énergie nerveuse intérieure.",
            "**Chemin de vie 22/4 (Le Maître bâtisseur) :** Reprend l’ancrage structurel de la racine 4 et le porte à une échelle grandiose. Les personnes du Chemin 22 possèdent la rare capacité de concrétiser des rêves visionnaires abstraits en réalisations tangibles et durables.",
            "**Chemin de vie 33/6 (Le Maître enseignant) :** Élève les qualités protectrices de la racine 6 vers l’altruisme universel et le service spirituel. Le Chemin 33 incarne l’accompagnement et la guérison désintéressés, et exige une grande maturité émotionnelle pour éviter l’épuisement.",
          ],
        },
      ],
    },
    {
      id: "faq",
      blocks: [
        { type: "h2", id: "faq", size: "lg", text: "Questions fréquentes" },
        {
          type: "faq",
          items: [
            {
              q: "Un nom d’adoption change-t-il mon nombre du Chemin de vie ?",
              a: "Non. Votre nombre du Chemin de vie est calculé exclusivement à partir de votre date de naissance : il reste donc définitivement le même, quels que soient les changements de nom ou une adoption légale.",
            },
            {
              q: "Les Nombres Maîtres sont-ils meilleurs ou plus chanceux que les nombres à un chiffre ?",
              a: "Non. Les Nombres Maîtres (11, 22, 33) offrent un plus grand potentiel, mais ils s’accompagnent aussi d’une pression psychologique plus forte et de leçons de vie plus intenses. Un Chemin de vie 4 ou 8 à un chiffre est tout aussi capable d’atteindre une grande réussite et un profond épanouissement qu’un Nombre Maître.",
            },
            {
              q: "Pourquoi des sites de numérologie différents me donnent-ils des nombres du Chemin de vie différents ?",
              a: "Si votre date de naissance contient des sous-cycles égaux à 11 ou 22, les calculateurs simplifiés qui additionnent tous les chiffres sur une seule ligne réduisent souvent ces Nombres Maîtres trop tôt. Notre calculateur applique strictement la méthode de réduction par composantes pour garantir des calculs intermédiaires exacts.",
            },
          ],
        },
      ],
    },
  ],
};

export default content;
