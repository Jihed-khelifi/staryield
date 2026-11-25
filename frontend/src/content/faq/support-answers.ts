/**
 * conversion follow-ups, slice E (docs/platform-extensions.md §27.1): the answers of the FAQ page's support
 * questions and of its "Free minutes, invites and your account" group, as English source copy translated with
 * `text()` (the English text is the key of copy-{en,fr,es}.json).
 *
 * Every answer restates a rule of the product, not an invention: the Terms of Use (sections 3.8, 4.20, 6), the
 * Refund Policy, the account settings, the auth flow and the chat room. Answers may use the content modules'
 * inline markup (`[label](/path)`); the page renders the links and the FAQPage structured data keeps the labels
 * only (`plainText`). Amounts an administrator can change (the sign-up offer, the invite reward and limit) are
 * not written down: the Pricing and Invite friends pages show the amount in force.
 *
 * The wording avoids the words the payment processor and the FAQ test rule out (`faq.test.tsx`,
 * `i18n/marketing-claims.test.ts`): e-mail confirmation is "confirm your e-mail address", and memberships are
 * never mentioned (they are switched off, docs/platform-extensions.md §25.1).
 */
import type { FaqEntry } from "@/content/faq/live-readings";

/** "STARYIELD Website Functional" (Figma 764:2025). */
export const WEBSITE_FAQ: readonly FaqEntry[] = [
  {
    q: "Why is my astrological info wrong on STARYIELD Website?",
    a: "Your astrological details, such as your sun, moon and rising signs, your houses and your daily horoscope, are calculated automatically from the birth date, birth time and birth city saved in your profile, so a wrong entry there gives wrong results. Open [Settings](/profile/settings), go to Account Information, correct your date of birth, time of birth or birth city and select Save Changes. If you do not know your birth time we use noon, which keeps your rising sign and houses hidden. Other websites may use different methods, for example another house system, so their results can differ from ours. Like the rest of Staryield, these results are for entertainment purposes only.",
  },
  {
    q: "How can I log in to STARYIELD Website?",
    a: "Select Log In at the top of the page and sign in with the e-mail address and password of your account or, where the option is offered, with your Google account. If you have forgotten your password, select Forgot password? on the sign-in page and we will e-mail you a reset link. After several failed attempts you are asked to wait a moment before trying again. If you do not have an account yet, you can [create one](/signup) with an e-mail address or with Google; you must be 18 or older and no payment method is needed. If you disabled your account or asked to close it, signing in again lets you reactivate it, or cancel the closure within 30 days.",
  },
  {
    q: "Where can I find the settings on the STARYIELD Website?",
    a: "Open the account menu at the top of the page (on a phone, the menu button) and select [Settings](/profile/settings). There you can edit your name, e-mail address, location and birth details (Account Information), change your password (Password & Security), review the devices signed in to your account, choose how we notify you (Notifications), manage your privacy preferences and download a copy of your data (Data & Privacy), choose the appearance, connect or disconnect Google, manage your saved cards and see your payments (Billing), and disable or close your account (Account Actions).",
  },
  {
    q: "How to reset staryield Website account's password?",
    a: "On the [Log In](/login) page select Forgot password?, enter the e-mail address of your account and submit. If an account exists for it, we send a reset link; the page always shows the same message, so check your spam folder if nothing arrives. The link works once and expires after the time stated in the e-mail, and asking for a new link cancels the earlier ones. Open it, choose a new password and sign in. For your security, resetting your password signs you out on all your devices. If the link has expired, request a new one from the same page. When you are already signed in, you can also change your password in Settings, under Password & Security.",
  },
  {
    q: "How to confirm STARYIELD Website account's email?",
    a: "When you register with an e-mail address and a password, we send you a welcome e-mail with a link to confirm that the address is yours. The link works once, expires after 48 hours and can be opened on any device, even if you are not signed in. If it expired or did not arrive, check your spam folder and ask for a new link from the notice in your account; the number of links you can request is limited, and a new link cancels the earlier ones. If you registered with Google, your address already counts as confirmed. If you receive a confirmation e-mail for an account you did not create, ignore it and do not open the link.",
  },
  {
    q: "How can I change my personal data on STARYIELD Website?",
    a: "Open [Settings](/profile/settings) and go to Account Information. You can change your name, location, date of birth, time of birth and birth city, then select Save Changes. To change your e-mail address, select Change next to it and enter the new address and your current password: we send a confirmation link to the new address, and your current address keeps working until you confirm. If you signed up with Google, some actions such as changing your e-mail address first ask you to create a Staryield password. Under Data & Privacy you can also download a copy of your data.",
  },
];

/** "Chatting with Advisors" (Figma 764:2083). */
export const CHATTING_FAQ: readonly FaqEntry[] = [
  {
    q: "I didn’t get a response from an astrologer/I was not satisfied with the reading I got.",
    a: "If an advisor does not accept your chat request before its countdown ends, the request expires and nothing is charged: billing starts only when the chat starts. You can send a new request, choose another advisor who is online, or join the waiting list of a busy or offline advisor with Notify me when online. If a reading did not go as expected, write to us through the [Contact page](/contact), preferably within 48 hours, with the advisor's name, the date of the session and what happened; in the chat you can also select Report an issue. We review good-faith concerns and may offer help, a service adjustment or courtesy credits, which are not cash refunds. Readings are subjective, so being dissatisfied with one is not by itself a reason for a refund; see the [Refund Policy](/refund-policy).",
  },
  {
    q: "How can I choose the right advisor?",
    a: "Open [Our advisors](/psychics): each card shows the advisor's specialties, rating and number of reviews (new advisors are marked as new), the price in credits per minute and whether they are online, in a reading, away or offline. Select View full profile to read about the advisor and their approach before you decide. Advisors are independent and the choice is yours, so use your own judgment, and remember that readings are for entertainment purposes only. If your preferred advisor is busy or offline, join their waiting list and we will let you know once when they come online.",
  },
  {
    q: "Why did my minutes run out so fast on staryield Website?",
    a: "Live readings are billed per started minute: at the start of each minute of the chat, that minute's price in credits is taken from your balance, so a minute that has just begun counts in full. The price is set by the advisor and shown before you start. Your free minutes are used first, then purchased credits, then bonus credits. Time you spend typing counts, because the chat is running. Billing pauses if the connection drops or the reading is paused, and time on hold after your free minutes run out is not charged. The chat summary shows the billed minutes of every reading; if a deduction looks wrong, contact us through the [Contact page](/contact) with the advisor's name and the date and time.",
  },
  {
    q: "Why I can not send my question in chat on staryield Website?",
    a: "A few things can stop the message box from sending. If your chat request is still waiting for the advisor, you can write once the live chat starts. When no live chat is running, you can still write in an existing conversation that has had at least one live reading; these messages are free but limited per day, the page shows when you have reached the limit, and you can start a live chat to keep talking. Messages can be up to 1000 characters, and images must be PNG, JPEG, WebP or GIF and up to 8 MB. Check your internet connection and reload the page if the send button stays disabled. If it still does not work, write to us through the [Contact page](/contact) and tell us what you see.",
  },
  {
    q: "Where can I find my chat/answer on staryield Website?",
    a: "Select Chat in the menu to open the chat room. The Conversations list shows every advisor you have talked to: select one to read your messages, and bookmark the ones you want to find again. When a reading ends, the chat summary shows its duration and the minutes billed. After a reading you can keep writing in the same conversation; those messages are free but limited per day. The bell at the top of the page also shows chat requests and reading summaries.",
  },
  {
    q: "How can I use credits for chatting with advisors on staryield Website?",
    a: "Credits are the balance you spend on Staryield. Buy a credit pack from the top-up menu (the [Pricing](/pricing) page lists the packs), then start a live chat with an advisor: at the start of each minute, the advisor's per-minute rate in credits is deducted from your balance. If you have free minutes, they are used first. You can also use credits to order custom readings from advisors who offer them. Credits have no cash value outside Staryield and cannot be transferred or exchanged, and every purchase is a one-time credit pack that does not renew.",
  },
];

/** "STARYIELD Astrologers" (Figma 764:2334). */
export const ASTROLOGERS_FAQ: readonly FaqEntry[] = [
  {
    q: "How can I receive the refund of credits for chatting with astrologers in STARYIELD?",
    a: "Our [Refund Policy](/refund-policy) applies. Within 14 calendar days of buying a credit pack you can ask for a refund: the full amount if none of its credits were used, otherwise the unused purchased part (the price you paid minus the value of the purchased credits already used). Credits used in completed readings are generally not refundable just because you changed your mind or were not satisfied, and free, bonus and referral minutes have no cash value. To ask, open Settings, go to Billing and select Request a refund next to the payment, or e-mail [support@staryield.net](mailto:support@staryield.net) with the subject Refund Request. We aim to acknowledge requests within 2 business days. An approved refund goes back to the original payment method and we aim to submit it within 5 business days; your bank may then need another 5 to 10 business days to show it.",
  },
  {
    q: "How can I choose an astrologer?",
    a: "Compare advisors on [Our advisors](/psychics): their specialties, rating and reviews, price in credits per minute and live status are on each card, and a profile adds their years of experience and the services they offer, such as custom readings. If you have free minutes they are used first, so you can meet an advisor before spending credits. Advisors are independent, so use your own judgment, and remember that readings are for entertainment purposes only.",
  },
  {
    q: "My minutes in STARYIELD are running out very fast.",
    a: "Each advisor sets a price in credits per minute, shown on their card and profile before you start, and every started minute is charged at that rate, so a higher rate uses your minutes faster. Compare rates in the advisor list and on the [Pricing](/pricing) page: fewer credits per minute means more reading time from the same pack. Your chat summary shows the minutes billed and, if you had free minutes, how many of them were free. If you think you were charged for more minutes than your reading lasted, contact us through the [Contact page](/contact) with the advisor's name and the date and time.",
  },
  {
    q: "My credits on STARYIELD disappeared. (Tech issue)",
    a: "Credits leave your balance when a live reading is billed (free minutes first, then purchased credits, then bonus credits), when you order a custom reading (they are held for it and returned if the advisor does not deliver it), or if your account is suspended or closed. Unused credits from a payment that is under dispute with your bank may also be frozen until it is resolved. Check your chat summaries and your payments under Settings, Billing. If a deduction looks wrong or credits are missing, write to us through the [Contact page](/contact) as soon as you can with the date and approximate time, the advisor's name and a description of the problem, plus the transaction ID for a purchase. Please never send your full card number or security code. After review we may correct your balance, restore credits, provide a replacement service or issue a refund.",
  },
  {
    q: "How much does it cost to communicate with astrologers in STARYIELD?",
    a: "Staryield works with credits. Each advisor sets a price in credits per minute, shown on their card and profile and before you start a chat, and you are charged per started minute. Credit packs are one-time purchases: the price and the number of credits are shown before you pay, nothing renews and we never top up your balance automatically. The [Pricing](/pricing) page lists the packs and the range of advisor rates. When the sign-up offer is on, new accounts also receive free minutes with no payment method needed, and our calculators are free.",
  },
  {
    q: "How can I start chatting with astrologers in STARYIELD?",
    a: "Create a free account with an e-mail address or with Google, or log in; you must be 18 or older, and no payment method is needed to sign up. Then choose an advisor on [Our advisors](/psychics). When the advisor is online, select Chat now to send a request; if the advisor is busy, away or offline, join their waiting list and we notify you once when they come online. The advisor has a short countdown to accept, and the chat and billing start when you are both connected. Free minutes are used first; add credits from the top-up menu when you need more.",
  },
  {
    q: "I am not satisfied with the astrologer's service at STARYIELD",
    a: "We are sorry the reading did not go as you hoped. Readings are subjective and outcomes are not promised, but we review every good-faith concern. Write to us through the [Contact page](/contact), preferably within 48 hours, with the advisor's name, the session date and what happened; in the chat you can also select Report an issue. Depending on what happened we may offer help, a service adjustment or courtesy credits, which are not cash refunds. Time already used in completed readings is generally not refundable, as set out in the [Refund Policy](/refund-policy). You can also rate your advisor from one to five stars within 7 days after the reading ends.",
  },
  {
    q: "Astrologer has never answered my questions in STARYIELD.",
    a: "First check whether the chat actually started: a request the advisor does not accept in time expires and nothing is charged. During a live reading you are billed per started minute while the chat is running, so you can end it at any time with End chat. Outside live readings, advisors are not obliged to reply and we do not promise a response time. If an advisor stopped replying during a reading, or a technical problem interrupted it, write to us through the [Contact page](/contact) with the advisor's name and the date and time; after review we may correct your balance, restore credits, provide a replacement service or issue a refund.",
  },
];

/** "STARYIELD Billing" (Figma 764:2406). */
export const BILLING_FAQ: readonly FaqEntry[] = [
  {
    q: "I have purchased credits in STARYIELD, but the balance was not topped up.",
    a: "Credits are added to your balance once your payment has succeeded, and we notify you in the app when your balance is topped up. Your bank may ask you to confirm the payment, for example with 3-D Secure or a code sent to your phone; if the confirmation is not completed, the payment does not go through and no credits are added. If a payment is still being confirmed, wait a moment and reload the page. If your payment shows as completed under Settings, Billing, or your bank shows the charge but no credits arrived, write to us through the [Contact page](/contact) or at [support@staryield.net](mailto:support@staryield.net) with your transaction details: the date, the pack and the receipt or transaction ID. Please never send your full card number or security code.",
  },
  {
    q: "How can I receive a refund or compensation for STARYIELD?",
    a: "Refunds of credit packs follow our [Refund Policy](/refund-policy): you can ask within 14 calendar days of the purchase, and the refund is the amount you paid minus the value of the purchased credits you already used. If something went wrong with a reading or a charge, such as a duplicate or incorrect charge, missing credits or an interrupted service, contact us promptly through the [Contact page](/contact); after review we may correct your balance, restore credits, provide a replacement service or refund the payment, and for a concern about a reading we may offer courtesy credits, which are not cash refunds. Free, bonus and referral minutes have no cash value. A refund is returned to the original payment method, with a credit note in your account. Support is available around the clock and we reply within 1 to 2 days.",
  },
];

/** The group added after "Live readings". */
export const ACCOUNT_FAQ: readonly FaqEntry[] = [
  {
    q: "How many free minutes do I get when I sign up?",
    a: "When our sign-up offer is on, new accounts receive a number of free chat minutes without a payment method, one offer per person. Staryield sets the number, which is shown on the [Pricing](/pricing) page and during sign-up, and may change the offer or switch it off for accounts created afterwards. Free minutes can only be used for live chat readings, are used before any credits, and have no cash value, so they cannot be exchanged, transferred or refunded. They never turn into a charge: if they run out during a reading, the reading is put on hold for a short time and nothing is charged unless you choose to add credits.",
  },
  {
    q: "How do I earn free minutes by inviting friends?",
    a: "Clients aged 18 or older have a personal invite link on the [Invite friends](/profile/invite) page of their account. You earn free chat minutes for each friend who creates a new account with your link, confirms their e-mail address and completes a live reading of at least one minute, paid or free. The reward is currently 5 free chat minutes per friend; the page always shows the reward and the limit in force, and how many friends joined or completed a reading, never who. Your friend gets the standard welcome offer of every new client. Please share your link only with people you know or where you are allowed to post it.",
  },
  {
    q: "When are my invite reward minutes added?",
    a: "Rewards are added to your account automatically shortly after your friend qualifies, and we notify you when minutes are added. Until then, the reward is shown as pending on the Invite friends page, and pending minutes cannot be used yet. A reward can be held for review, for example when an unusual number of friends join in a short time, and we may reject rewards that break the program rules in our [Terms of Use](/terms#referral-program).",
  },
  {
    q: "Do I have to confirm my e-mail address?",
    a: "Not to use Staryield or to buy credits. We still recommend it: it secures your account, lets us reach you with receipts and account notices, and it is one of the conditions for an invited friend to count towards your invite rewards. Accounts created with Google already have a confirmed address. We may remind you, and you can ask for a new confirmation link at any time from your account, within limits on how often links can be sent.",
  },
  {
    q: "Are the calculators free?",
    a: "Yes. Our natal chart, life path number, zodiac sign, tarot birth card and zodiac compatibility calculators, and our guide pages, are free and need no account, no credits and no free minutes. Their results describe traditions and are for entertainment purposes only. We may show advisors next to some results, but using a calculator never commits you to a reading. Try the [natal chart calculator](/calculators/natal-chart).",
  },
];

export const ACCOUNT_FAQ_HEADING = "Free minutes, invites and your account";
export const ACCOUNT_FAQ_DESCRIPTION = "Free minutes, invite rewards, confirming your e-mail address and our free calculators.";
