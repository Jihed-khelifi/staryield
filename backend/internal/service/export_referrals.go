package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// collectReferrals adds referrals.json to an export: the user's code, their
// counters and whether they were invited. Never who invited them and never who
// they invited.
func (s *ExportService) collectReferrals(_ context.Context, b *ExportBundle, userID uuid.UUID, rows exportRows) error {
	var codes []domain.ReferralCode
	if err := rows(&codes, "user_id"); err != nil {
		return err
	}
	var invited []domain.Referral
	if err := rows(&invited, "referrer_id"); err != nil {
		return err
	}
	var asFriend []domain.Referral
	if err := rows(&asFriend, "referee_id"); err != nil {
		return err
	}
	if len(codes) == 0 && len(invited) == 0 && len(asFriend) == 0 {
		return nil
	}
	doc := map[string]any{}
	if len(codes) > 0 {
		doc["code"], doc["code_created_at"] = codes[0].Code, codes[0].CreatedAt
	}
	var joined, qualified, pending, granted int64
	for _, r := range invited {
		if r.Status == domain.ReferralRejected {
			continue
		}
		joined++
		switch r.Status {
		case domain.ReferralQualified:
			qualified, pending = qualified+1, pending+int64(r.RewardMinutes)
		case domain.ReferralGranted:
			qualified, granted = qualified+1, granted+int64(r.RewardMinutes)
		}
	}
	doc["friends_joined"], doc["friends_completed_a_reading"] = joined, qualified
	doc["free_minutes_pending"], doc["free_minutes_granted"] = pending, granted
	doc["invited_by_a_friend"] = len(asFriend) > 0
	if len(asFriend) > 0 {
		doc["joined_with_an_invite_at"] = asFriend[0].JoinedAt
	}
	b.Documents = append(b.Documents, ExportDocument{Path: "referrals.json", Data: doc})
	return nil
}
