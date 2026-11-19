# Stripe setup (revisions 6.0)

How to turn on card payments through Stripe for Staryield. The code is in
place (contract: `docs/platform-extensions.md` §26.5); this is the account and
configuration side. Do it in **test mode** first, then repeat with live keys.

## 1. Account

1. Create the Stripe account for **Staryield Media LLC** (United States,
   Wyoming), not the existing "Speak Silq" account. Stripe only prohibits
   psychic services for accounts based in Japan, Mexico and Thailand.
2. Business description: live chat readings with independent advisors, sold as
   prepaid credits, for entertainment purposes only. Website
   `https://staryield.net`, support `support@staryield.net`, Terms, Privacy and
   Refund Policy URLs (`/en/terms`, `/en/privacy`, `/en/refund-policy`).
3. **Statement descriptor**: Settings → Business → Public details →
   "STARYIELD MEDIA LLC". It must equal `STARYIELD_PAYMENTS_STATEMENT_DESCRIPTOR`;
   the admin console's Stripe page shows a warning when they differ.
4. Payment methods (Settings → Payment methods): keep **Cards** on, with
   Apple Pay and Google Pay. The app requests card payments only, so methods
   that confirm later (bank debits, vouchers) never appear.

## 2. Keys and environment

API (Railway service variables):

| Variable | Value |
| --- | --- |
| `STARYIELD_PAYMENTS_PROVIDER` | `stripe` (turns the legacy hosted sandbox checkout off and keeps the card providers on; production refuses `sandbox`) |
| `STARYIELD_PAYMENTS_PROVIDERS` | `stripe` (development can list `sandbox_a,sandbox_b,stripe`) |
| `STARYIELD_PAYMENTS_DEFAULT_PROVIDER` | `stripe` |
| `STARYIELD_STRIPE_SECRET_KEY` | a **restricted key** (below) or the secret key |
| `STARYIELD_STRIPE_PUBLISHABLE_KEY` | `pk_test_…` / `pk_live_…` |
| `STARYIELD_STRIPE_WEBHOOK_SECRET` | `whsec_…` of the endpoint (step 3) |

Test and live keys can't be mixed, and production refuses test keys unless
`STARYIELD_STRIPE_ALLOW_TEST_IN_PRODUCTION=true`.

Restricted key permissions (Developers → API keys → Create restricted key):
write on PaymentIntents, SetupIntents, Customers, PaymentMethods, Refunds,
Disputes and Files; read on ConfirmationTokens, Charges, Events, Webhook
Endpoints, Balance and the Account.

## 3. Webhook endpoint

Developers → Webhooks → Add endpoint:

* URL: `https://<API origin>/api/v1/webhooks/payments/stripe`
* API version: **2026-10-28.endive** (the version `stripe-go` v87.1.0 uses).
* Events:
  `payment_intent.succeeded`, `payment_intent.payment_failed`,
  `payment_intent.processing`, `payment_intent.canceled`,
  `refund.created`, `refund.updated`, `refund.failed`,
  `charge.dispute.created`, `charge.dispute.updated`, `charge.dispute.closed`,
  `charge.dispute.funds_withdrawn`, `charge.dispute.funds_reinstated`,
  `radar.early_fraud_warning.created`, `radar.early_fraud_warning.updated`,
  `payment_method.detached`, `customer.deleted`,
  `review.opened`, `review.closed`.

Copy the signing secret into `STARYIELD_STRIPE_WEBHOOK_SECRET`. The admin
console (Payments → Stripe) lists the endpoint and any missing events.

## 4. Routing

The database seeds a routing rule named "Default" that sends payments to
`sandbox_a`. In the admin console (Payments → Routing) **edit that rule to
use `stripe`**, or add a rule with a lower priority number (purpose "any",
provider `stripe`) above it. Until then purchases keep going to the sandbox
(which production refuses). Saved cards and new cards then go through Stripe;
the sandbox providers stay for development.

## 5. Apple Pay and Google Pay

Settings → Payment method domains: add `staryield.net` (Apple Pay needs it;
Google Pay needs nothing). Apple Pay only shows in Safari with a card in
Wallet.

## 6. Fraud and disputes

* Radar (included): consider rules that block when the CVC or postal-code
  check fails, and "Request 3D Secure if :risk_level: = 'elevated'".
* Admin console → Payments → Policies: "Block purchases while a dispute is
  open" and "Freeze the disputed purchase's credits" are on by default;
  "Refund automatically on early fraud warnings" is off; 3-D Secure
  "automatic" (or "every payment").
* Disputes are answered from the console (Payments → Disputes): the evidence
  form is prefilled with the purchase, the consent record, the usage log and
  the invoice.

## 7. Fonts and badge (R2)

The card fields load Source Serif 4 from
`https://assets.staryield.net/assets/fonts/source-serif-4-latin-variable-8e38e3fbd7.woff2`
inside Stripe's frame, so the bucket must allow Stripe's origin. Apply the
updated CORS file once:

```
cd frontend && npx wrangler r2 bucket cors set staryield-app-assets --file r2-cors.json
```

Without it the fields fall back to Georgia. The "Powered by Stripe" badge is
`assets/payments/powered-by-stripe-e434f84af2.svg` (official, dark variant).

## 8. Test before going live

Test cards (any future date, any CVC): `4242 4242 4242 4242` succeeds,
`4000 0025 0000 3155` asks for 3-D Secure, `4000 0000 0000 9995` is declined
for insufficient funds, `4000 0000 0000 0259` opens a dispute after payment.
Check: a top-up adds credits at once; a 3-D Secure top-up completes in the
dialog; Payments → Webhook events shows the deliveries as processed; the
invoice appears in the transaction history and the receipt e-mail; a refund
from the console issues a credit note; a test dispute freezes the credits and
blocks purchases until it is closed.

The Playwright spec `frontend/e2e/stripe-checkout.spec.ts` runs against Stripe
test mode when `E2E_STRIPE_PUBLISHABLE_KEY` is set (with the API running on
the matching test keys). Locally:

1. `backend/.env`: the test keys, `STARYIELD_PAYMENTS_PROVIDER=stripe`,
   `STARYIELD_PAYMENTS_PROVIDERS=stripe`, `STARYIELD_PAYMENTS_DEFAULT_PROVIDER=stripe`,
   and `STARYIELD_STRIPE_WEBHOOK_SECRET` from `stripe listen --print-secret`
   (the Stripe CLI's own secret, logged in to the Staryield account).
2. Point the "Default" routing rule at `stripe` (section 4).
3. Forward the events (the CLI needs them named):
   `stripe listen --events payment_intent.succeeded,payment_intent.payment_failed,payment_intent.processing,payment_intent.canceled,refund.created,refund.updated,refund.failed,charge.dispute.created,charge.dispute.updated,charge.dispute.closed,charge.dispute.funds_withdrawn,charge.dispute.funds_reinstated,radar.early_fraud_warning.created,radar.early_fraud_warning.updated,payment_method.detached,customer.deleted,review.opened,review.closed --forward-to http://localhost:8080/api/v1/webhooks/payments/stripe`
4. `E2E_STRIPE_PUBLISHABLE_KEY=pk_test_… npx playwright test e2e/stripe-checkout.spec.ts e2e/stripe-elements.spec.ts`.

`npm run dev` runs Next.js, while production runs vinext. To test the build
that ships, build it with the API origin and serve the worker locally:
`NEXT_PUBLIC_API_URL=http://localhost:8080 npx vite build`, then
`npx wrangler dev --config dist/server/wrangler.json --port 3000 --local`, and
run Playwright with `E2E_BASE_URL=http://localhost:3000` (it reuses that server).

A test-mode webhook endpoint that points at production also receives these
local test events; production acknowledges them as "no payment matches this
event" (or refuses them when it verifies with a live secret).

The card form offers cards only (Apple Pay and Google Pay are card wallets).
Link stays off in code (`wallets.link: "never"`): `allowedPaymentMethodTypes`
does not hide it, and it brings bank and Klarna payments that the API's
card-only intents refuse.
