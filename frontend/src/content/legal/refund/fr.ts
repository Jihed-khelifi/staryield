// Refund & Cancellation Policy (Figma 1360:2766). English text is the Figma copy; fr/es are translations.
// Keep section and block ids identical across locales (npm run test:content).
// Acknowledgement (2 business days), refund submission (5 business days) and the card-statement descriptor
// (STARYIELD MEDIA LLC) were confirmed by the business on 2026-11-13 (docs/legal-review-checklist.md row 65).
// Growth release: §2 describes the configured sign-up offer (admin setting, no fixed number of minutes: checklist row 74),
// free mode and the free tools; §6 adds free minutes and referral reward minutes; §12 adds their forfeiture on closure.
import type { LegalDocumentContent } from "@/content/legal/types";

const content: LegalDocumentContent = {
  "meta": {
    "version": "2026-11-25",
    "effective": "2026-11-25",
    "lastUpdated": "2026-11-25",
    "badge": "STARYIELD · POLITIQUES",
    "title": "Politique de remboursement et d’annulation"
  },
  "labels": {
    "toc": "Sur cette page",
    "effective": "Date d’entrée en vigueur :",
    "lastUpdated": "Dernière mise à jour :",
    "backToTop": "Retour en haut"
  },
  "intro": [
    {
      "type": "p",
      "text": "Cette politique explique les remboursements applicables aux packs de crédits, aux consultations de voyance et aux crédits promotionnels achetés via Staryield, exploité par Staryield Media LLC. Elle explique également comment sont traitées les minutes offertes, y compris les récompenses de parrainage, ainsi que nos fonctionnalités gratuites."
    },
    {
      "type": "p",
      "text": "En achetant des crédits ou en utilisant des services payants, vous reconnaissez cette politique. Rien dans le présent document ne limite les droits que vous confère la législation applicable en matière de protection des consommateurs."
    }
  ],
  "sections": [
    {
      "id": "one-time-purchases",
      "n": "1",
      "title": "Achats uniques",
      "blocks": [
        {
          "type": "p",
          "text": "Les packs de crédits sont des achats uniques. Nous ne proposons ni abonnements ni rechargements automatiques : un pack ne crée jamais d’adhésion récurrente et nous ne rechargeons pas votre moyen de paiement lorsque votre solde est bas."
        }
      ]
    },
    {
      "id": "free-trial",
      "n": "2",
      "title": "Essai gratuit",
      "blocks": [
        {
          "type": "p",
          "text": "Lorsque notre offre d’inscription est disponible, les nouveaux comptes reçoivent un certain nombre de minutes de consultation offertes, sans avoir à fournir de moyen de paiement. Staryield fixe ce nombre de minutes et l’indique sur notre page [Tarifs](/pricing) ainsi que lors de l’inscription. Nous pouvons modifier l’offre ou la désactiver à tout moment ; une modification ne s’applique qu’aux comptes créés ensuite, de sorte que vous bénéficiez de l’offre en vigueur au moment de la création de votre compte. L’offre est réservée aux nouveaux comptes, à raison d’une seule par personne. L’essai ne nécessite aucun achat, ne se transforme pas en abonnement et n’entraîne aucun prélèvement. Les minutes d’essai gratuites n’ont aucune valeur en espèces ni valeur remboursable."
        },
        {
          "type": "p",
          "text": "Si vos minutes offertes sont épuisées pendant une consultation en direct, la consultation est mise en attente pendant une courte durée au lieu de se terminer (voir les [Conditions d’utilisation](/terms#free-trials-and-promotional-offers)). La mise en attente n’est pas facturée : vous ne payez que si vous ajoutez des Crédits pour continuer, ou si vous choisissez ensuite de révéler les réponses masquées pendant la mise en attente, et le prix vous est indiqué au préalable."
        },
        {
          "type": "p",
          "text": "Les offres affichées sur les boutons de réservation, les pages ou les rappels, comme « minutes gratuites pour les nouveaux membres », décrivent l’offre en vigueur au moment de leur affichage et suivent les mêmes règles."
        },
        {
          "type": "h4",
          "id": "refund-free-mode",
          "title": "Chats gratuits pour tous (« mode gratuit ») :"
        },
        {
          "type": "p",
          "text": "Il peut nous arriver de rendre les chats en direct gratuits pour tous pendant une période limitée ; les boutons de réservation peuvent alors afficher « Gratuit pour une durée limitée ». Un chat demandé pendant le mode gratuit ne consomme ni minutes offertes ni Crédits et n’entraîne aucun paiement : il n’y a donc rien à rembourser. Les Crédits que vous achetez pendant le mode gratuit restent couverts par la présente politique, comme d’habitude. Voir les [Conditions d’utilisation, section 6.1.4](/terms#free-trials-and-promotional-offers)."
        },
        {
          "type": "h4",
          "id": "refund-free-tools",
          "title": "Calculateurs, images de résultats et pages de guide gratuits :"
        },
        {
          "type": "p",
          "text": "Nos calculateurs (y compris le thème natal complet, le calculateur de carte de naissance du tarot et le calculateur de compatibilité des signes), les images de résultats que vous pouvez créer et partager à partir de ces calculateurs, le rapport de calcul envoyé par e-mail ainsi que nos pages de guide consacrées aux chemins de vie, aux cartes de naissance du tarot et aux associations de signes du zodiaque sont gratuits ([Conditions d’utilisation, section 2.5](/terms#free-calculators-and-guides)). Leur utilisation n’entraîne aucun paiement et ne consomme ni Crédits ni minutes offertes : il n’y a donc rien à rembourser à leur titre."
        }
      ]
    },
    {
      "id": "credit-delivery",
      "n": "3",
      "title": "Livraison des crédits",
      "blocks": [
        {
          "type": "p",
          "text": "Les crédits sont normalement ajoutés après la confirmation du paiement. Si votre paiement a abouti mais que les crédits n’ont pas été livrés, contactez [support@staryield.net](mailto:support@staryield.net) en indiquant les détails de votre transaction."
        }
      ]
    },
    {
      "id": "credit-pack-refunds",
      "n": "4",
      "title": "Remboursement des packs de crédits",
      "blocks": [
        {
          "type": "p",
          "text": "Vous pouvez demander un remboursement dans les 14 jours calendaires suivant l’achat initial, sous réserve des règles suivantes :"
        },
        {
          "type": "h4",
          "id": "refund-unused-packs",
          "title": "Packs entièrement inutilisés :"
        },
        {
          "type": "p",
          "text": "Si aucun des crédits achetés ou promotionnels n’a été utilisé, vous pouvez demander le remboursement intégral du montant effectivement payé."
        },
        {
          "type": "h4",
          "id": "refund-partial-packs",
          "title": "Packs partiellement utilisés :"
        },
        {
          "type": "p",
          "text": "Vous pouvez demander le remboursement de la partie achetée non utilisée qui est éligible."
        },
        {
          "type": "callout",
          "text": "Montant remboursé = prix réel du pack payé − valeur des crédits achetés déjà utilisés."
        },
        {
          "type": "p",
          "text": "La valeur par crédit correspond au prix réellement payé divisé par le nombre de crédits achetés initialement inclus. Les crédits achetés sont considérés comme consommés avant les crédits bonus attribués séparément. Les remboursements ne peuvent pas dépasser le montant payé."
        },
        {
          "type": "h4",
          "id": "refund-completed-readings",
          "title": "Temps de consultation effectué :"
        },
        {
          "type": "p",
          "text": "Les crédits consommés pour des consultations terminées ne sont généralement pas remboursables au seul motif que vous avez changé d’avis ou que vous êtes insatisfait. Cela n’affecte pas les recours en cas de problèmes techniques ou de facturation avérés, ni les remboursements exigés par la loi."
        },
        {
          "type": "h4",
          "id": "refund-right-of-withdrawal",
          "title": "Droit de rétractation (EEE, Royaume-Uni et Suisse) :"
        },
        {
          "type": "p",
          "text": "Si vous résidez dans l’Espace économique européen, au Royaume-Uni ou en Suisse, vous pouvez disposer d’un droit légal de rétractation de 14 jours, sans avoir à vous justifier, pour l’achat de contenus numériques. Lors du paiement, nous vous demandons d’accepter que vos crédits soient livrés immédiatement ; une fois ajoutés à votre solde, le droit de rétractation pour ce pack prend fin. Les packs inutilisés peuvent toujours être remboursés selon la présente politique. Voir [Droit de rétractation dans les Conditions d’utilisation](/terms#right-of-withdrawal)."
        }
      ]
    },
    {
      "id": "welcome-pack-and-promotions",
      "n": "5",
      "title": "Pack de bienvenue et promotions",
      "blocks": [
        {
          "type": "p",
          "text": "Les remboursements du pack de bienvenue et des promotions sont calculés sur le montant réduit effectivement payé, et non sur la valeur standard ou annoncée des crédits."
        },
        {
          "type": "p",
          "text": "L’éligibilité à une promotion peut ne pas être rétablie après un remboursement lorsque cela permettrait une utilisation en double d’une offre unique."
        }
      ]
    },
    {
      "id": "bonus-and-promotional-credits",
      "n": "6",
      "title": "Crédits bonus et promotionnels",
      "blocks": [
        {
          "type": "p",
          "text": "Les crédits bonus, gratuits, de parrainage, de fidélité et de courtoisie n’ont aucune valeur en espèces propre, ne peuvent pas être échangés contre de l’argent et ne sont pas remboursables de manière indépendante, sauf si la loi l’exige."
        },
        {
          "type": "h4",
          "id": "refund-free-minutes",
          "title": "Minutes offertes :"
        },
        {
          "type": "p",
          "text": "Les minutes de consultation offertes sont promotionnelles, qu’elles proviennent de l’offre d’inscription (section 2), du programme de parrainage ou d’une autre promotion. Elles ne peuvent servir qu’à des consultations par chat en direct sur Staryield et sont utilisées avant tout Crédit de votre solde. Elles n’ont aucune valeur en espèces et ne peuvent être ni remboursées, ni échangées contre de l’argent ou des Crédits, ni vendues, ni transférées vers un autre compte, sauf si la loi l’exige."
        },
        {
          "type": "p",
          "text": "Le temps de consultation couvert par des minutes offertes n’entraîne aucun paiement : il n’y a donc rien à rembourser en argent. Les minutes offertes utilisées lors d’une consultation ne réduisent pas le montant remboursable d’un pack de crédits, car seuls les crédits achetés déjà utilisés sont déduits (section 4). Si un problème est survenu lors d’une consultation ayant utilisé des minutes offertes, vous pouvez tout de même nous contacter (section 8) ; tout geste commercial que nous proposons ne constitue pas un remboursement en espèces."
        },
        {
          "type": "h4",
          "id": "refund-referral-rewards",
          "title": "Minutes de récompense du parrainage :"
        },
        {
          "type": "p",
          "text": "Les minutes gagnées en invitant des amis ([Conditions d’utilisation, section 4.20](/terms#referral-program)) sont des minutes offertes et suivent les règles ci-dessus. Une récompense est ajoutée à votre compte peu après que votre ami a rempli les conditions ; d’ici là, une récompense en attente ne peut être ni utilisée, ni remboursée, ni échangée. Une fois les minutes ajoutées, un remboursement ou une contestation de paiement ultérieurs de la part de l’ami que vous avez invité ne les modifient pas, et la modification, la suspension ou la fin du programme ne vous les retirent pas. Avant qu’une récompense soit ajoutée, nous pouvons la retenir ou la refuser si nous avons des raisons de penser qu’elle enfreint les règles du programme ou nos Conditions. Si nous suspendons ou fermons un compte pour abus du programme, ses récompenses en attente et ses minutes offertes peuvent être perdues."
        }
      ]
    },
    {
      "id": "technical-problems-and-incorrect-charges",
      "n": "7",
      "title": "Problèmes techniques et prélèvements erronés",
      "blocks": [
        {
          "type": "p",
          "text": "Contactez-nous rapidement en cas de prélèvements en double ou non autorisés, de déductions incorrectes, de crédits manquants, d’interruptions de la plateforme ou de services non fournis."
        },
        {
          "type": "p",
          "text": "Indiquez l’identifiant de la transaction, la date et l’heure approximatives, le nom du conseiller et une description du problème. N’envoyez pas votre numéro de carte complet ni votre code de sécurité."
        },
        {
          "type": "p",
          "text": "Après examen, nous pouvons corriger votre solde, rétablir des crédits, fournir un service de remplacement ou émettre un remboursement, selon le cas."
        }
      ]
    },
    {
      "id": "concerns-about-a-reading",
      "n": "8",
      "title": "Préoccupations concernant une consultation",
      "blocks": [
        {
          "type": "p",
          "text": "Les consultations de voyance sont subjectives et aucune prédiction ni aucun résultat particulier n’est garanti."
        },
        {
          "type": "p",
          "text": "Si quelque chose s’est mal passé, contactez [support@staryield.net](mailto:support@staryield.net), de préférence dans les 48 heures, en indiquant le nom du conseiller, la date de la séance et les détails. Nous examinerons les préoccupations de bonne foi et pourrons proposer une assistance, un ajustement du service ou des crédits de courtoisie. Les crédits de courtoisie ne sont pas des remboursements en espèces."
        }
      ]
    },
    {
      "id": "requesting-a-refund",
      "n": "9",
      "title": "Demander un remboursement",
      "blocks": [
        {
          "type": "p",
          "text": "Écrivez à [support@staryield.net](mailto:support@staryield.net) avec l’objet « Refund Request ». Indiquez :"
        },
        {
          "type": "ul",
          "items": [
            "Nom et adresse e-mail d’inscription",
            "Date d’achat",
            "Nom du pack et montant payé",
            "Identifiant de transaction ou de reçu",
            "Solde restant approximatif",
            "Motif de la demande",
            "Détails techniques ou de séance pertinents"
          ]
        },
        {
          "type": "p",
          "text": "Nous visons à accuser réception des demandes sous 2 jours ouvrés."
        }
      ]
    },
    {
      "id": "approved-refunds",
      "n": "10",
      "title": "Remboursements approuvés",
      "blocks": [
        {
          "type": "p",
          "text": "Les remboursements approuvés sont versés sur le moyen de paiement d’origine chaque fois que possible. Nous visons à les transmettre sous 5 jours ouvrés après l’approbation. Votre banque ou votre prestataire de paiement peut avoir besoin d’un délai supplémentaire pour les comptabiliser."
        },
        {
          "type": "p",
          "text": "Remboursements par carte. Le remboursement d’un paiement par carte est crédité sur la carte d’origine via Stripe. Nous le transmettons à Stripe sous 5 jours ouvrés après l’approbation, puis votre banque peut avoir besoin de 5 à 10 jours ouvrés supplémentaires pour l’afficher sur votre relevé. Pour chaque remboursement, nous émettons un avoir, que vous retrouvez dans votre compte à côté de la facture d’origine."
        },
        {
          "type": "p",
          "text": "Les remboursements sont généralement effectués dans la devise de l’achat initial. Les établissements financiers externes décident eux-mêmes de leurs délais de traitement et de la conversion des devises."
        }
      ]
    },
    {
      "id": "payment-disputes",
      "n": "11",
      "title": "Litiges de paiement",
      "blocks": [
        {
          "type": "p",
          "text": "Vous pouvez d’abord nous contacter afin que nous vous aidions à résoudre rapidement un problème de facturation. Cette démarche est facultative et ne limite pas votre droit de contacter votre banque ou votre prestataire de paiement."
        },
        {
          "type": "p",
          "text": "Votre relevé peut afficher les achats sous la mention STARYIELD MEDIA LLC."
        },
        {
          "type": "p",
          "text": "Tant qu’un litige de paiement est ouvert, les Crédits non utilisés achetés avec le paiement contesté peuvent être gelés et les nouveaux achats suspendus, comme prévu dans nos [Conditions d’utilisation](/terms#chargebacks-and-payment-disputes). Un paiement contesté ne peut pas être également remboursé : nous répondons alors au litige."
        },
        {
          "type": "p",
          "text": "Si l’émetteur de votre carte ou Stripe nous signale qu’un paiement pourrait être frauduleux, nous pouvons le rembourser immédiatement sur la carte d’origine et examiner votre compte. Un avoir est également émis pour ce remboursement."
        }
      ]
    },
    {
      "id": "account-closure",
      "n": "12",
      "title": "Clôture du compte",
      "blocks": [
        {
          "type": "p",
          "text": "Pour demander la clôture de votre compte, contactez [support@staryield.net](mailto:support@staryield.net). S’il vous reste des crédits, contactez-nous avant la clôture afin que nous vous expliquions les options disponibles. La clôture du compte ne supprime pas les droits à remboursement applicables."
        },
        {
          "type": "p",
          "text": "Les minutes offertes, y compris les minutes de récompense du parrainage, ne sont ni remboursées ni versées en argent lors de la clôture d’un compte. Lorsque la clôture devient définitive ([Conditions d’utilisation, section 3.6](/terms#account-deletion)), les minutes offertes restant sur le compte et les récompenses de parrainage non encore ajoutées sont perdues. Cela n’affecte pas les droits à remboursement des packs de crédits achetés décrits dans la présente politique."
        }
      ]
    },
    {
      "id": "policy-updates",
      "n": "13",
      "title": "Mises à jour de la politique",
      "blocks": [
        {
          "type": "p",
          "text": "Nous pouvons mettre à jour cette politique pour des raisons opérationnelles, juridiques ou liées au traitement des paiements. Les modifications importantes ne supprimeront pas rétroactivement les droits attachés à un achat antérieur, sauf si la loi l’exige ou si la modification offre un recours plus favorable."
        }
      ]
    },
    {
      "id": "contact-us",
      "n": "14",
      "title": "Nous contacter",
      "blocks": [
        {
          "type": "p",
          "text": "Écrivez-nous via la [Page de contact](/contact) ou à [support@staryield.net](mailto:support@staryield.net)."
        },
        {
          "type": "p",
          "text": "L’assistance est disponible 24/7. Nous répondons sous 1 à 2 jours."
        },
        {
          "type": "button",
          "label": "Contacter l’assistance",
          "href": "/contact"
        }
      ]
    }
  ],
  "policy": {
    "actions": [
      {
        "label": "Contacter l’assistance",
        "href": "/contact"
      },
      {
        "label": "Voir les tarifs",
        "href": "/pricing"
      }
    ],
    "related": {
      "title": "Liens associés :",
      "links": [
        {
          "label": "Conditions d’utilisation",
          "href": "/terms"
        },
        {
          "label": "Politique de confidentialité",
          "href": "/privacy"
        },
        {
          "label": "Tarifs et crédits",
          "href": "/pricing"
        }
      ]
    },
    "breadcrumb": {
      "home": "Accueil",
      "current": "Politique de remboursement"
    }
  }
};

export default content;
