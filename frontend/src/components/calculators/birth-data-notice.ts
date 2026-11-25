/**
 * What the calculators say about the birth data they are given (revisions 6.0,
 * §26.4). The birth data is only used to calculate the report; it is kept only
 * when the visitor asks for the report by e-mail, and only until the e-mail is
 * sent. One sentence for the three calculators, under their forms and results
 * (it replaces the former "never stored" lines).
 */
export const BIRTH_DATA_NOTICE =
  "Your birth data is used only to calculate this report. We keep it only if you ask us to e-mail the report, and only until it is sent.";

/** The tarot birth card is worked out in the browser: nothing is sent, so nothing can be kept. */
export const TAROT_BIRTH_NOTICE = "Your birth date is used only in your browser to find your card. It is never sent to us.";
