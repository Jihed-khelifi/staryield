package seed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// Growth: e-mail verification. Verified and unverified accounts in production
// proportions, with the single-use links they were sent, so the admin
// "verified / not verified" filters and the link lookups are checked at
// realistic sizes (TestExplainHotQueries "e-mail verification: …", "admin
// users: …"). Run it alone with
//
//	go run ./cmd/seed -profile=large -extend "email verification"
//
// About 80% of the synthetic clients confirm their address (from a few
// minutes to two days after signing up; their link is consumed), 12% never
// do (their link expired unused) and 8% hold an open link sent in the last two
// days (a recent resend). The token hashes derive from the run's tag and the
// payloads are placeholders (the API cannot open them, and never needs to).
// Skipped when load verification links already exist (-force adds more).
func init() {
	RegisterLarge(Generator{Name: "email verification", Run: func(_ context.Context, d *Dataset) error { return d.emailVerification() }})
}

// verifyDelays are the buckets of time between sign-up and verification: one
// UPDATE per bucket and batch keeps the stamped dates varied without a
// round trip per user.
var verifyDelays = []time.Duration{3 * time.Minute, 20 * time.Minute, 2 * time.Hour, 9 * time.Hour, 30 * time.Hour}

const verifyLinkTTL = 48 * time.Hour

func (d *Dataset) emailVerification() error {
	var seeded bool
	if err := d.DB.Raw(`SELECT EXISTS (SELECT 1 FROM user_action_tokens t JOIN users u ON u.id = t.user_id WHERE t.purpose = ? AND u.email LIKE ?)`,
		domain.ActionEmailVerify, "%@"+LoadEmailDomain).Row().Scan(&seeded); err != nil {
		return err
	}
	if seeded && !d.Opts.Force {
		d.Log.Info("email verification skipped: load verification links already exist (use -force to add more)")
		return nil
	}
	tokens := make([]domain.ActionToken, 0, d.Opts.BatchSize)
	flush := func() error {
		err := d.insert("user_action_tokens", &tokens, len(tokens), true)
		tokens = tokens[:0]
		return err
	}
	token := func(user uuid.UUID, created time.Time, used *time.Time) domain.ActionToken {
		sum := sha256.Sum256([]byte(fmt.Sprintf("verify:%s:%s:%d", d.Tag, user, created.UnixNano())))
		updated := created
		if used != nil {
			updated = *used
		}
		return domain.ActionToken{ID: d.ID(created), UserID: user, Purpose: domain.ActionEmailVerify, TokenHash: hex.EncodeToString(sum[:]),
			PayloadEnc: "seed", ExpiresAt: created.Add(verifyLinkTTL), UsedAt: used, CreatedAt: created, UpdatedAt: updated}
	}
	verified, open := 0, 0
	for start := 0; start < len(d.Clients); start += d.Opts.BatchSize {
		ids := d.Clients[start:min(start+d.Opts.BatchSize, len(d.Clients))]
		var rows []struct {
			ID        uuid.UUID
			CreatedAt time.Time
		}
		// One read per batch: only accounts that are not verified yet.
		if err := d.DB.Model(&domain.User{}).Select("id, created_at").Where("id IN ? AND email_verified_at IS NULL", ids).Scan(&rows).Error; err != nil {
			return err
		}
		byDelay := make([][]uuid.UUID, len(verifyDelays))
		for _, r := range rows {
			switch d.pick(80, 12, 8) {
			case 0: // verified: the welcome link was used
				k := d.Rand.IntN(len(verifyDelays))
				at := r.CreatedAt.Add(verifyDelays[k])
				if at.After(d.Now) {
					at = d.Now
				}
				byDelay[k] = append(byDelay[k], r.ID)
				tokens = append(tokens, token(r.ID, r.CreatedAt, &at))
				verified++
			case 1: // never verified: the welcome link expired unused
				tokens = append(tokens, token(r.ID, r.CreatedAt, nil))
			default: // a recent resend: an open link
				sent := d.Now.Add(-time.Duration(d.Rand.Int64N(int64(verifyLinkTTL - time.Hour)))).Truncate(time.Second)
				tokens = append(tokens, token(r.ID, sent, nil))
				open++
			}
		}
		for k, users := range byDelay {
			if len(users) == 0 {
				continue
			}
			if err := d.DB.Model(&domain.User{}).Where("id IN ?", users).
				UpdateColumn("email_verified_at", gorm.Expr("LEAST(created_at + ?::interval, ?)", fmt.Sprintf("%d seconds", int(verifyDelays[k].Seconds())), d.Now)).Error; err != nil {
				return fmt.Errorf("stamp verified users: %w", err)
			}
		}
		if err := flush(); err != nil {
			return err
		}
	}
	d.Counts["verified users"] += verified
	d.Log.Info("email verification generated", zap.Int("links", d.Counts["user_action_tokens"]), zap.Int("verified", verified), zap.Int("open", open))
	return nil
}
