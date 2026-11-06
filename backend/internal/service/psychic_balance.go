package service

import (
	"context"

	"github.com/google/uuid"
)

// PsychicBalanceView is the advisor's wallet pill: earnings that can still be
// withdrawn, in credits and in money.
type PsychicBalanceView struct {
	AvailableCredits   int64  `json:"available_credits"`
	AvailableCents     int64  `json:"available_cents"`
	Currency           string `json:"currency"`
	ReservedCredits    int64  `json:"reserved_credits"`
	LifetimeNetCredits int64  `json:"lifetime_net_credits"`
}

// BalanceView converts the payout balance into the wallet pill shape.
func (s *WithdrawalService) BalanceView(ctx context.Context, psychicID uuid.UUID) (*PsychicBalanceView, error) {
	p, err := s.Balance(ctx, psychicID)
	if err != nil {
		return nil, err
	}
	return balanceView(p), nil
}

func balanceView(p *PayoutView) *PsychicBalanceView {
	return &PsychicBalanceView{AvailableCredits: p.AvailableCredits, AvailableCents: p.AvailableCredits * p.CentsPerCredit,
		Currency: p.Currency, ReservedCredits: p.ReservedCredits, LifetimeNetCredits: p.LifetimeNetCredits}
}
