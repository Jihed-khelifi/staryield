// Refund & Cancellation Policy (Figma 1360:2766). English text is the Figma copy; fr/es are translations.
// Keep section and block ids identical across locales (npm run test:content).
// Acknowledgement (2 business days), refund submission (5 business days) and the card-statement descriptor
// (STARYIELD MEDIA LLC) were confirmed by the business on 2026-11-13 (docs/legal-review-checklist.md row 65).
// Growth release: §2 describes the configured sign-up offer (admin setting, no fixed number of minutes: checklist row 74),
// free mode and the free tools; §6 adds free minutes and referral reward minutes; §12 adds their forfeiture on closure.
import type { LegalDocumentContent } from "@/content/legal/types";

const content: LegalDocumentContent = {
  "meta": {
    "version": "2026-11-19",
    "effective": "2026-11-19",
    "lastUpdated": "2026-11-19",
    "badge": "STARYIELD · POLICIES",
    "title": "Refund & Cancellation Policy"
  },
  "labels": {
    "toc": "On this page",
    "effective": "Effective date:",
    "lastUpdated": "Last updated:",
    "backToTop": "Back to top"
  },
  "intro": [
    {
      "type": "p",
      "text": "This policy explains refunds for credit packs, psychic readings, and promotional credits purchased through Staryield, operated by Staryield Media LLC. It also explains how free minutes, including referral rewards, and our free features are treated."
    },
    {
      "type": "p",
      "text": "By purchasing credits or using paid services, you acknowledge this policy. Nothing here limits rights provided by applicable consumer-protection law."
    }
  ],
  "sections": [
    {
      "id": "one-time-purchases",
      "n": "1",
      "title": "One-time purchases",
      "blocks": [
        {
          "type": "p",
          "text": "Credit packs are one-time purchases. We do not offer subscriptions or automatic top-ups: a pack never creates a recurring membership, and we do not recharge your payment method when your balance runs low."
        }
      ]
    },
    {
      "id": "free-trial",
      "n": "2",
      "title": "Free trial",
      "blocks": [
        {
          "type": "p",
          "text": "When our sign-up offer is available, new accounts receive a number of free reading minutes without providing a payment method. Staryield sets the number of minutes and shows it on our [Pricing](/pricing) page and during sign-up. We may change the offer or switch it off at any time; a change applies only to accounts created afterwards, so you receive the offer in force when your account is created. The offer is for new accounts only, one per person. The trial requires no purchase, does not convert into a subscription, and does not result in a charge. Free trial minutes have no cash or refund value."
        },
        {
          "type": "p",
          "text": "If your free minutes run out during a live reading, the reading is put on hold for a short time instead of ending (see [Terms of Use](/terms#free-trials-and-promotional-offers)). Nothing is charged for the hold: you pay only if you add Credits to continue, or later choose to reveal the replies hidden during the hold, and you are shown the price first."
        },
        {
          "type": "p",
          "text": "Offers shown on booking buttons, pages or reminders, such as “free minutes for new members”, describe the offer in force when they were shown and follow the same rules."
        },
        {
          "type": "h4",
          "id": "refund-free-mode",
          "title": "Free chats for everyone (“free mode”):"
        },
        {
          "type": "p",
          "text": "From time to time we may make live chats free for everyone for a limited period, and booking buttons may then show “Free for a limited time”. A chat requested while free mode is on uses no free minutes or Credits and involves no payment, so there is nothing to refund. Credits you buy during free mode are covered by this policy as usual. See [Terms of Use, Section 6.1.4](/terms#free-trials-and-promotional-offers)."
        },
        {
          "type": "h4",
          "id": "refund-free-tools",
          "title": "Free calculators, result images and guide pages:"
        },
        {
          "type": "p",
          "text": "Our calculators (including the full natal chart, the tarot birth card calculator and the zodiac compatibility calculator), the result images you can create and share from them, the calculator report sent by e-mail, and our guide pages about life path numbers, tarot birth cards and zodiac sign pairings are free ([Terms of Use, Section 2.5](/terms#free-calculators-and-guides)). Using them involves no payment and uses no Credits or free minutes, so there is nothing to refund for them."
        }
      ]
    },
    {
      "id": "credit-delivery",
      "n": "3",
      "title": "Credit delivery",
      "blocks": [
        {
          "type": "p",
          "text": "Credits are normally added after successful payment confirmation. If your payment completed but credits were not delivered, contact [support@staryield.net](mailto:support@staryield.net) with your transaction details."
        }
      ]
    },
    {
      "id": "credit-pack-refunds",
      "n": "4",
      "title": "Credit-pack refunds",
      "blocks": [
        {
          "type": "p",
          "text": "You may request a refund within 14 calendar days of the original purchase, subject to these rules:"
        },
        {
          "type": "h4",
          "id": "refund-unused-packs",
          "title": "Completely unused packs:"
        },
        {
          "type": "p",
          "text": "If none of the purchased or promotional credits have been used, you may request a full refund of the amount actually paid."
        },
        {
          "type": "h4",
          "id": "refund-partial-packs",
          "title": "Partially used packs:"
        },
        {
          "type": "p",
          "text": "You may request a refund of the eligible unused purchased portion."
        },
        {
          "type": "callout",
          "text": "Refund amount = actual pack price paid − value of purchased credits already used."
        },
        {
          "type": "p",
          "text": "The per-credit value equals the actual price paid divided by the number of purchased credits originally included. Purchased credits are treated as consumed before separately awarded bonus credits. Refunds cannot exceed the amount paid."
        },
        {
          "type": "h4",
          "id": "refund-completed-readings",
          "title": "Completed reading time:"
        },
        {
          "type": "p",
          "text": "Credits consumed for completed readings are generally not refundable simply because you changed your mind or were dissatisfied. This does not affect remedies for verified technical or billing problems or refunds required by law."
        },
        {
          "type": "h4",
          "id": "refund-right-of-withdrawal",
          "title": "Right of withdrawal (EEA, UK and Switzerland):"
        },
        {
          "type": "p",
          "text": "If you live in the European Economic Area, the United Kingdom or Switzerland, you may have a statutory right to withdraw from a purchase of digital content within 14 days without giving a reason. When you pay, we ask you to agree that we deliver your credits immediately; once they have been added to your balance, the right of withdrawal for that pack ends. Unused packs can still be refunded under this policy. See [Right of Withdrawal in the Terms of Use](/terms#right-of-withdrawal)."
        }
      ]
    },
    {
      "id": "welcome-pack-and-promotions",
      "n": "5",
      "title": "Welcome Pack and promotions",
      "blocks": [
        {
          "type": "p",
          "text": "Welcome Pack and promotional refunds are calculated using the actual discounted amount paid, not the standard or advertised credit value."
        },
        {
          "type": "p",
          "text": "Promotional eligibility may not be restored after a refund where restoring it would permit duplicate use of a one-time offer."
        }
      ]
    },
    {
      "id": "bonus-and-promotional-credits",
      "n": "6",
      "title": "Bonus and promotional credits",
      "blocks": [
        {
          "type": "p",
          "text": "Bonus, free, referral, loyalty, and courtesy credits have no independent cash value, cannot be exchanged for cash, and are not independently refundable, except where required by law."
        },
        {
          "type": "h4",
          "id": "refund-free-minutes",
          "title": "Free minutes:"
        },
        {
          "type": "p",
          "text": "Free reading minutes are promotional, whether they come from the sign-up offer (Section 2), the referral program or another promotion. They can be used only for live chat readings on Staryield and are used before any Credits in your balance. They have no cash value and cannot be refunded, exchanged for money or Credits, sold or transferred to another account, except where required by law."
        },
        {
          "type": "p",
          "text": "Reading time covered by free minutes involves no payment, so there is nothing to refund in money. Free minutes used in a reading do not reduce the refundable amount of a credit pack, because only purchased credits already used are deducted (Section 4). If something went wrong in a reading that used free minutes, you can still contact us (Section 8); any courtesy remedy we offer is not a cash refund."
        },
        {
          "type": "h4",
          "id": "refund-referral-rewards",
          "title": "Referral reward minutes:"
        },
        {
          "type": "p",
          "text": "Minutes earned by inviting friends ([Terms of Use, Section 4.20](/terms#referral-program)) are free minutes and follow the rules above. Rewards earned before we launch the referral rewards are shown as pending and are added to your account at launch, on a date we choose; a pending reward cannot be used, refunded or exchanged. Once minutes are added, a later refund or payment dispute by the friend you invited does not change them, and changing, pausing or ending the program does not take them away. Before a reward is added, we may withhold or reject it if we reasonably believe it breaks the program rules or our Terms. If we suspend or close an account for abusing the program, its pending rewards and free minutes may be forfeited."
        }
      ]
    },
    {
      "id": "technical-problems-and-incorrect-charges",
      "n": "7",
      "title": "Technical problems and incorrect charges",
      "blocks": [
        {
          "type": "p",
          "text": "Contact us promptly about duplicate or unauthorized charges, incorrect deductions, missing credits, platform interruptions, or undelivered services."
        },
        {
          "type": "p",
          "text": "Include the transaction ID, approximate date and time, advisor name, and a description of the issue. Do not send your full card number or security code."
        },
        {
          "type": "p",
          "text": "After review, we may correct your balance, restore credits, provide a replacement service, or issue a refund, as appropriate."
        }
      ]
    },
    {
      "id": "concerns-about-a-reading",
      "n": "8",
      "title": "Concerns about a reading",
      "blocks": [
        {
          "type": "p",
          "text": "Psychic readings are subjective, and particular predictions or outcomes are not guaranteed."
        },
        {
          "type": "p",
          "text": "If something went wrong, contact [support@staryield.net](mailto:support@staryield.net), preferably within 48 hours, with the advisor’s name, session date, and details. We will review good-faith concerns and may provide assistance, a service adjustment, or courtesy credits. Courtesy credits are not cash refunds."
        }
      ]
    },
    {
      "id": "requesting-a-refund",
      "n": "9",
      "title": "Requesting a refund",
      "blocks": [
        {
          "type": "p",
          "text": "Email [support@staryield.net](mailto:support@staryield.net) with the subject “Refund Request.” Include:"
        },
        {
          "type": "ul",
          "items": [
            "Registered name and email",
            "Purchase date",
            "Pack name and amount paid",
            "Transaction or receipt ID",
            "Approximate remaining balance",
            "Reason for the request",
            "Relevant technical or session details"
          ]
        },
        {
          "type": "p",
          "text": "We aim to acknowledge requests within 2 business days."
        }
      ]
    },
    {
      "id": "approved-refunds",
      "n": "10",
      "title": "Approved refunds",
      "blocks": [
        {
          "type": "p",
          "text": "Approved refunds are returned to the original payment method whenever possible. We aim to submit them within 5 business days after approval. Your bank or payment provider may require additional posting time."
        },
        {
          "type": "p",
          "text": "Card refunds. A refund of a card payment goes back to the original card through Stripe. We submit it to Stripe within 5 business days after approval, and your bank may then take another 5–10 business days to show it on your statement. For every refund we issue a credit note, which you can find in your account next to the original invoice."
        },
        {
          "type": "p",
          "text": "Refunds generally use the original purchase currency. External financial institutions control their own posting times and currency conversion."
        }
      ]
    },
    {
      "id": "payment-disputes",
      "n": "11",
      "title": "Payment disputes",
      "blocks": [
        {
          "type": "p",
          "text": "You may contact us first so we can help resolve a billing concern quickly. Doing so is optional and does not limit your right to contact your bank or payment provider."
        },
        {
          "type": "p",
          "text": "Your statement may display purchases as STARYIELD MEDIA LLC."
        },
        {
          "type": "p",
          "text": "While a payment dispute is open, the unused Credits bought with the disputed payment may be frozen and new purchases paused, as set out in our [Terms of Use](/terms#chargebacks-and-payment-disputes). A payment that is under dispute cannot also be refunded: we answer the dispute instead."
        },
        {
          "type": "p",
          "text": "If your card issuer or Stripe warns us that a payment may be fraudulent, we may refund it to the original card straight away and review your account. A credit note is issued for that refund too."
        }
      ]
    },
    {
      "id": "account-closure",
      "n": "12",
      "title": "Account closure",
      "blocks": [
        {
          "type": "p",
          "text": "To request account closure, contact [support@staryield.net](mailto:support@staryield.net). If credits remain, contact us before closure so we can explain available options. Account closure does not eliminate applicable refund rights."
        },
        {
          "type": "p",
          "text": "Free minutes, including referral reward minutes, are not refunded or paid out when an account is closed. When the closure becomes final ([Terms of Use, Section 3.6](/terms#account-deletion)), the free minutes left in the account and any referral rewards not yet added are forfeited. This does not affect the refund rights for purchased credit packs described in this policy."
        }
      ]
    },
    {
      "id": "policy-updates",
      "n": "13",
      "title": "Policy updates",
      "blocks": [
        {
          "type": "p",
          "text": "We may update this policy for operational, legal, or payment-processing requirements. Material changes will not retroactively remove rights attached to an earlier purchase unless required by law or the change provides a more favorable remedy."
        }
      ]
    },
    {
      "id": "contact-us",
      "n": "14",
      "title": "Contact us",
      "blocks": [
        {
          "type": "p",
          "text": "Write to us through the [Contact page](/contact) or at [support@staryield.net](mailto:support@staryield.net)."
        },
        {
          "type": "p",
          "text": "Support is available 24/7. We reply within 1–2 days."
        },
        {
          "type": "button",
          "label": "Contact Support",
          "href": "/contact"
        }
      ]
    }
  ],
  "policy": {
    "actions": [
      {
        "label": "Contact Support",
        "href": "/contact"
      },
      {
        "label": "Go to Pricing",
        "href": "/pricing"
      }
    ],
    "related": {
      "title": "Related links:",
      "links": [
        {
          "label": "Terms of Service",
          "href": "/terms"
        },
        {
          "label": "Privacy Policy",
          "href": "/privacy"
        },
        {
          "label": "Pricing and Credits",
          "href": "/pricing"
        }
      ]
    },
    "breadcrumb": {
      "home": "Home",
      "current": "Refund Policy"
    }
  }
};

export default content;
