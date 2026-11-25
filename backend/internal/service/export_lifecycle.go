package service

// growth: lifecycle and onboarding. The personal-data export includes the
// lifecycle messages of the account: which reminder or nudge was recorded and
// when, whether it was sent or skipped (and why), the channels attempted, and
// whether a reading followed it within the attribution window (conversion).
// The checklist's dismissal is in the profile of account.json.

import (
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
)

// collectLifecycle adds lifecycle-messages.json to an export when the account
// has lifecycle messages.
func collectLifecycle(b *ExportBundle, rows exportRows, capped func(int, string)) error {
	var msgs []domain.LifecycleMessage
	if err := rows(&msgs, "user_id"); err != nil {
		return err
	}
	if len(msgs) == 0 {
		return nil
	}
	capped(len(msgs), "lifecycle messages")
	out := make([]map[string]any, 0, len(msgs))
	for i := range msgs {
		m := &msgs[i]
		channels := map[string]any(m.Channels)
		if channels == nil {
			channels = map[string]any{}
		}
		entry := map[string]any{"journey": m.Journey, "cycle": m.Cycle, "status": m.Status, "channels_attempted": channels,
			"converted_at": m.ConvertedAt, "at": m.CreatedAt}
		if m.SkipReason != "" {
			entry["skip_reason"] = m.SkipReason
		}
		out = append(out, entry)
	}
	b.Documents = append(b.Documents, ExportDocument{Path: "lifecycle-messages.json", Data: out})
	return nil
}
