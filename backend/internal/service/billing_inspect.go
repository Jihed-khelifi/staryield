package service

import (
	"context"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/payment"
)

// revisions 5.0: the card behind a purchase source (§24.2). The welcome
// offer needs the card's fingerprint before the charge.

// SourceFingerprint returns the provider fingerprint of the card a purchase
// will be charged to, without charging it. A saved card carries it already;
// a fresh single-use token is read through the provider's optional
// payment.TokenInspector capability (discovered by type assertion, so no
// provider is special-cased). It returns "" when the fingerprint cannot be
// known (the provider cannot inspect tokens, or reports none).
func (s *BillingService) SourceFingerprint(ctx context.Context, src *PurchaseSource) (string, error) {
	if src == nil {
		return "", nil
	}
	if src.Method != nil {
		return src.Method.FingerprintHash, nil
	}
	p, ok := s.registry.Get(src.Provider)
	if !ok {
		return "", nil
	}
	ins, ok := p.(payment.TokenInspector)
	if !ok {
		return "", nil
	}
	ictx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	meta, err := ins.InspectToken(ictx, src.token)
	if err != nil {
		return "", attachError(ctx, err)
	}
	return meta.Fingerprint, nil
}
