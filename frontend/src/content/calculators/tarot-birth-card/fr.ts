// Tarot birth card calculator — long-form section under the result (French).
import type { ArticleContent } from "@/content/calculators/types";
import { tarotBirthIntroImage } from "@/content/calculators/tarot-birth-card/en";

const content: ArticleContent = {
  kind: "tarot-birth-card",
  label: "Comprendre votre carte de naissance du tarot",
  sections: [
    {
      id: "what-is-a-tarot-birth-card",
      blocks: [
        {
          type: "split",
          blocks: [
            { type: "h2", id: "what-is-a-tarot-birth-card", text: "Qu’est-ce que la carte de naissance du tarot ?" },
            {
              type: "p",
              text: "Votre carte de naissance est l’arcane majeur qui correspond aux nombres de votre date de naissance. La méthode a été popularisée par l’autrice de tarot Mary K. Greer, qui a réuni la numérologie et les 22 cartes des arcanes majeurs : vous additionnez votre date de naissance, vous réduisez le total et vous lisez la carte obtenue comme le symbole des qualités et des leçons qui vous accompagnent tout au long de votre vie.",
            },
            {
              type: "p",
              weight: "light",
              text: "La plupart des dates donnent deux cartes : une **carte de personnalité**, qui décrit votre façon d’aborder le monde, et une **carte d’âme**, qui indique votre aspiration la plus profonde. Certaines dates donnent une seule carte qui joue les deux rôles, et une chaîne, 19/10/1, en donne trois. Comme toutes les lectures de Staryield, votre carte de naissance est proposée pour la réflexion et le divertissement, et non comme une prédiction.",
            },
          ],
          image: { ...tarotBirthIntroImage, alt: "La carte de tarot L’Étoile" },
        },
      ],
    },
    {
      id: "how-to-calculate",
      blocks: [
        { type: "h2", id: "how-to-calculate", text: "Comment calculer votre carte de naissance du tarot" },
        {
          type: "ol",
          items: [
            "Additionnez le mois, le jour et l’année de votre naissance en nombres entiers (par exemple 11 + 29 + 1990).",
            "Additionnez les chiffres du total. Si le résultat dépasse 22, additionnez de nouveau ses chiffres jusqu’à obtenir 22 ou moins.",
            "Le nombre obtenu est votre carte de personnalité : 1 est Le Bateleur, 2 La Papesse, et ainsi de suite jusqu’à 21, Le Monde. Un résultat de 22 correspond au Mat, qui porte le numéro 0 dans le jeu.",
            "Si ce nombre a deux chiffres, additionnez-les, et recommencez si besoin, jusqu’à obtenir un seul chiffre : c’est votre carte d’âme (par exemple 17 ⟹ 1 + 7 = 8, La Force). Un résultat à un seul chiffre est à la fois votre carte de personnalité et votre carte d’âme.",
          ],
        },
        { type: "h3", id: "worked-example", text: "Exemple détaillé : 29 novembre 1990" },
        {
          type: "ul",
          items: [
            "Additionnez la date : 11 + 29 + 1990 = 2030",
            "Additionnez les chiffres : 2 + 0 + 3 + 0 = 5",
            "5 n’a qu’un seul chiffre : Le Pape (V) est donc à la fois la carte de personnalité et la carte d’âme.",
          ],
        },
        { type: "h3", id: "two-card-example", text: "Deux cartes : 15 juin 1987" },
        {
          type: "ul",
          items: [
            "Additionnez la date : 6 + 15 + 1987 = 2008",
            "Additionnez les chiffres : 2 + 0 + 0 + 8 = 10, la carte de personnalité est donc La Roue de Fortune (X)",
            "Réduisez encore une fois : 1 + 0 = 1, la carte d’âme est donc Le Bateleur (I)",
          ],
        },
        {
          type: "p",
          weight: "light",
          text: "Sous votre résultat, « Le détail du calcul » reprend chaque étape pour votre propre date, afin que vous puissiez vérifier le calcul vous-même.",
        },
      ],
    },
    {
      id: "personality-and-soul-cards",
      blocks: [
        { type: "h2", id: "personality-and-soul-cards", text: "Carte de personnalité et carte d’âme" },
        {
          type: "p",
          text: "Les cartes fonctionnent en duo. Le nombre le plus élevé est votre carte de personnalité : l’image que vous montrez, les talents que vous utilisez au quotidien et votre manière d’affronter les difficultés. Le nombre à un chiffre est votre carte d’âme : un fil plus discret, celui du but que vous apprenez à habiter au fil de votre vie.",
        },
        {
          type: "ul",
          items: [
            "**Une seule carte (2 à 9) :** votre personnalité et votre âme partagent la même énergie, si bien que ses thèmes traversent votre vie avec une intensité particulière.",
            "**Deux cartes (10 à 18, 20, 21 et 22) :** la carte de personnalité mène et la carte d’âme, en dessous, révèle la leçon plus profonde. Le Mat (22) s’associe à L’Empereur (4).",
            "**Trois cartes (19) :** Le Soleil est la carte de personnalité et Le Bateleur la carte d’âme, avec La Roue de Fortune (10) comme pont entre les deux.",
          ],
        },
      ],
    },
    {
      id: "constellations",
      blocks: [
        { type: "h2", id: "constellations", text: "Les neuf constellations du tarot" },
        {
          type: "p",
          text: "Les cartes de naissance qui aboutissent à la même carte d’âme forment une constellation, qui porte le nom de sa carte à un chiffre. Les personnes d’une même constellation partagent la même leçon profonde, même lorsque leurs cartes de personnalité diffèrent.",
        },
        {
          type: "twoColumnList",
          items: [
            { title: "Constellation 1", subtitle: "Le Bateleur", body: ["10/1 : La Roue de Fortune et Le Bateleur", "19/10/1 : Le Soleil, La Roue de Fortune et Le Bateleur"] },
            { title: "Constellation 2", subtitle: "La Papesse", body: ["11/2 : La Justice et La Papesse", "20/2 : Le Jugement et La Papesse", "2 : La Papesse seule"] },
            { title: "Constellation 3", subtitle: "L’Impératrice", body: ["12/3 : Le Pendu et L’Impératrice", "21/3 : Le Monde et L’Impératrice", "3 : L’Impératrice seule"] },
            { title: "Constellation 4", subtitle: "L’Empereur", body: ["13/4 : L’Arcane sans nom et L’Empereur", "22/4 : Le Mat et L’Empereur", "4 : L’Empereur seul"] },
            { title: "Constellation 5", subtitle: "Le Pape", body: ["14/5 : Tempérance et Le Pape", "5 : Le Pape seul"] },
            { title: "Constellation 6", subtitle: "L’Amoureux", body: ["15/6 : Le Diable et L’Amoureux", "6 : L’Amoureux seul"] },
            { title: "Constellation 7", subtitle: "Le Chariot", body: ["16/7 : La Maison Dieu et Le Chariot", "7 : Le Chariot seul"] },
            { title: "Constellation 8", subtitle: "La Force", body: ["17/8 : L’Étoile et La Force", "8 : La Force seule"] },
            { title: "Constellation 9", subtitle: "L’Hermite", body: ["18/9 : La Lune et L’Hermite", "9 : L’Hermite seul"] },
          ],
        },
        {
          type: "p",
          weight: "light",
          text: "Votre résultat indique les autres cartes de naissance de votre constellation, pour que vous sachiez qui partage votre carte d’âme.",
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
              q: "Faut-il connaître mon heure de naissance pour trouver ma carte de naissance du tarot ?",
              a: "Non. Le calcul n’utilise que votre date de naissance : l’heure et le lieu de votre naissance ne changent rien. Votre date ne quitte jamais votre navigateur, car la carte est calculée sur votre appareil.",
            },
            {
              q: "En quoi est-ce différent de mon nombre du Chemin de vie ?",
              a: "Les deux partent de votre date de naissance, mais ne sont pas réduits de la même façon. Votre [nombre du Chemin de vie](/calculators/path-of-life) réduit séparément le mois, le jour et l’année et conserve les nombres maîtres 11, 22 et 33, alors que la carte de naissance additionne d’abord la date entière et s’arrête à 22, le nombre de cartes des arcanes majeurs.",
            },
            {
              q: "Pourquoi La Force porte-t-elle le numéro 8 ici alors que mon jeu attribue le 8 à La Justice ?",
              a: "Les jeux de type Marseille numérotent La Justice 8 et La Force 11, tandis que la tradition Rider–Waite–Smith, suivie par de nombreux jeux modernes, les inverse. Ce calculateur utilise l’ordre Rider–Waite–Smith : 8 est La Force et 11 La Justice. Si vous préférez l’ordre de Marseille, lisez simplement ces deux cartes dans l’autre sens.",
            },
            {
              q: "Ma carte de naissance est-elle la même que la carte de tarot de mon signe du zodiaque ?",
              a: "Non. Votre carte de naissance vient des nombres de votre date de naissance complète, alors que chaque signe du zodiaque est associé à un arcane majeur par l’astrologie (le Lion à La Force, par exemple). Deux personnes du même signe peuvent avoir des cartes de naissance différentes. Pour votre signe et le reste de votre thème, essayez le [calculateur de thème natal](/calculators/natal-chart).",
            },
            {
              q: "Ma carte de naissance peut-elle changer avec le temps ?",
              a: "Non. Elle est fixée par votre date de naissance. Certains tarologues calculent aussi une carte de l’année en ajoutant votre mois et votre jour de naissance à l’année en cours ; ce calculateur n’affiche que vos cartes de naissance.",
            },
            {
              q: "La carte de naissance du tarot est-elle une prédiction ?",
              a: "Non. Les cartes de naissance sont une façon symbolique de réfléchir à vos forces et à vos leçons, et elles sont proposées à titre de divertissement uniquement. Elles ne constituent ni une prévision ni un substitut à un avis professionnel.",
            },
          ],
        },
      ],
    },
  ],
};

export default content;
