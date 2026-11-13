-- 0030_purchase_consent: consent evidence on every credit purchase
-- (docs/platform-extensions.md section 25.2, revisions 5.1).
--
-- A buyer must accept the Terms of Use, the Privacy Policy and the Refund &
-- Cancellation Policy before paying. The versions accepted (the policies'
-- YYYY-MM-DD version dates), the server time and two correlation fields are
-- kept with the purchase as chargeback evidence (card-scheme compelling
-- evidence matches the IP address and device). All columns are nullable:
-- purchases made before this revision carry no consent record.
--
-- consent_terms_version    version date of the Terms of Use accepted
-- consent_privacy_version  version date of the Privacy Policy accepted
-- consent_refund_version   version date of the Refund & Cancellation Policy accepted
-- consent_at               server time of the purchase request
-- consent_ip_hash          HMAC of the client IP (analytics secret, as refund requests); never exported
-- consent_user_agent       cleaned browser user agent, at most 300 characters

ALTER TABLE credit_purchases ADD COLUMN IF NOT EXISTS consent_terms_version   text;
ALTER TABLE credit_purchases ADD COLUMN IF NOT EXISTS consent_privacy_version text;
ALTER TABLE credit_purchases ADD COLUMN IF NOT EXISTS consent_refund_version  text;
ALTER TABLE credit_purchases ADD COLUMN IF NOT EXISTS consent_at              timestamptz;
ALTER TABLE credit_purchases ADD COLUMN IF NOT EXISTS consent_ip_hash         text;
ALTER TABLE credit_purchases ADD COLUMN IF NOT EXISTS consent_user_agent      text;

COMMENT ON COLUMN credit_purchases.consent_terms_version   IS 'Terms of Use version (YYYY-MM-DD) the buyer accepted; NULL before revisions 5.1';
COMMENT ON COLUMN credit_purchases.consent_privacy_version IS 'Privacy Policy version (YYYY-MM-DD) the buyer accepted';
COMMENT ON COLUMN credit_purchases.consent_refund_version  IS 'Refund & Cancellation Policy version (YYYY-MM-DD) the buyer accepted';
COMMENT ON COLUMN credit_purchases.consent_at              IS 'Server time the consent was recorded (the purchase request)';
COMMENT ON COLUMN credit_purchases.consent_ip_hash         IS 'HMAC of the client IP with the analytics secret (chargeback evidence; never exported)';
COMMENT ON COLUMN credit_purchases.consent_user_agent      IS 'Cleaned user agent of the purchase request, at most 300 characters';
