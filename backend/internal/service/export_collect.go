package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// collect gathers a user's personal data. Notes advisors wrote about the
// user are deliberately excluded (see exportExcluded); secrets (password
// hash, token hashes, sealed values, IP hashes) never leave the database.
func (s *ExportService) collect(ctx context.Context, userID uuid.UUID) (*ExportBundle, error) {
	u, err := s.d.Users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	b := &ExportBundle{UserID: userID, Locale: normalizeLocale(u.Locale), GeneratedAt: s.d.Clock.Now()}
	limit := s.cfg.RowLimit
	rows := func(dest any, column string) error { return s.d.Data.Rows(ctx, dest, column, userID, limit) }
	capped := func(n int, what string) {
		if n >= limit {
			b.Omitted = append(b.Omitted, fmt.Sprintf("Only the first %d %s are included.", limit, what))
		}
	}

	// Account, profile and preferences.
	account := map[string]any{"id": u.ID, "email": u.Email, "display_name": u.DisplayName, "role": u.Role, "status": u.Status,
		"locale": u.Locale, "email_verified_at": u.EmailVerifiedAt, "terms_accepted_at": u.TermsAcceptedAt,
		"last_login_at": u.LastLoginAt, "created_at": u.CreatedAt}
	doc := map[string]any{"account": account}
	p, err := s.d.Users.GetProfile(ctx, userID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if p != nil {
		var birth any
		if p.BirthDate != nil {
			birth = p.BirthDate.Format("2006-01-02")
		}
		doc["profile"] = map[string]any{"nickname": p.Nickname, "birth_date": birth, "birth_time": p.BirthTime,
			"birth_place_id": p.BirthPlaceID, "birth_place": p.BirthPlace, "birth_place_label": p.BirthPlaceLabel,
			"birth_latitude": p.BirthLatitude, "birth_longitude": p.BirthLongitude, "birth_timezone": p.BirthTimezone,
			"country_code": p.CountryCode, "location": p.Location, "mood": p.Mood, "theme": p.Theme, "time_zone": p.Timezone,
			"house_system": p.HouseSystem, "avatar_url": p.AvatarURL,
			"notifications": map[string]any{"email": p.NotifyEmail, "push": p.NotifyPush, "marketing": p.NotifyMarketing,
				"horoscope": p.NotifyHoroscope, "events": p.NotifyEvents, "chart": p.NotifyChart},
			"share_birth_details_with_advisors": p.ShareBirthDetailsWithAdvisors, "created_at": p.CreatedAt, "updated_at": p.UpdatedAt}
	}
	var wallets []domain.Wallet
	if err := rows(&wallets, "user_id"); err != nil {
		return nil, err
	}
	if len(wallets) > 0 {
		w := wallets[0]
		doc["wallet"] = map[string]any{"balance_credits": w.Balance, "held_credits": w.Held, "free_minutes": w.FreeMinutes}
	}
	var attributions []domain.Attribution
	if err := rows(&attributions, "user_id"); err != nil {
		return nil, err
	}
	if len(attributions) > 0 {
		a := attributions[0]
		doc["sign_up_source"] = map[string]any{"utm_source": a.UTMSource, "utm_medium": a.UTMMedium, "utm_campaign": a.UTMCampaign,
			"utm_content": a.UTMContent, "utm_term": a.UTMTerm, "referrer_host": a.ReferrerHost, "landing_path": a.LandingPath,
			"first_seen_at": a.FirstSeenAt}
	}
	if u.Role == domain.RolePsychic {
		var advisors []domain.Psychic
		if err := rows(&advisors, "user_id"); err != nil {
			return nil, err
		}
		var services []domain.PsychicService
		if err := rows(&services, "psychic_id"); err != nil {
			return nil, err
		}
		// advisor settings: photos submitted for review (metadata only).
		var photos []domain.PhotoSubmission
		if err := rows(&photos, "psychic_id"); err != nil {
			return nil, err
		}
		if len(advisors) > 0 {
			a := advisors[0]
			svc := make([]map[string]any, 0, len(services))
			for _, x := range services {
				svc = append(svc, map[string]any{"id": x.ID, "name": x.Name, "subtitle": x.Subtitle, "description": x.Description,
					"price_credits": x.PriceCredits, "active": x.Active, "kind": x.Kind, "delivery_days": x.DeliveryDays, "created_at": x.CreatedAt})
			}
			ph := make([]map[string]any, 0, len(photos))
			for _, x := range photos {
				ph = append(ph, map[string]any{"status": x.Status, "width": x.Width, "height": x.Height, "reason": x.Reason,
					"submitted_at": x.CreatedAt, "reviewed_at": x.ReviewedAt})
			}
			phone := ""
			if a.PhoneSealed != "" && s.d.Sealer != nil {
				if plain, err := s.d.Sealer.Open(a.PhoneSealed, phoneAD(a.UserID)); err == nil {
					phone = plain
				}
			}
			doc["advisor_profile"] = map[string]any{"slug": a.Slug, "name": a.Name, "headline": a.Headline, "about": a.About,
				"philosophy": a.Philosophy, "image_url": a.ImageURL, "zodiac": a.Zodiac, "skills": a.Skills,
				"years_experience": a.YearsExperience, "rate_per_minute": a.RatePerMinute, "listed": a.Listed,
				"phone": phone, "phone_on_file": a.PhoneSealed != "", "created_at": a.CreatedAt, "services": svc,
				"photo_submissions": ph}
		}
	}
	b.Documents = append(b.Documents, ExportDocument{Path: "account.json", Data: doc})

	// Devices and consent history.
	var devices []domain.UserSession
	if err := rows(&devices, "user_id"); err != nil {
		return nil, err
	}
	dev := make([]map[string]any, 0, len(devices))
	for _, d := range devices {
		v := deviceView(&d, uuid.Nil)
		dev = append(dev, map[string]any{"label": v.Label, "device_type": v.DeviceType, "browser": v.Browser, "os": v.OS,
			"user_agent": d.UserAgent, "signed_in_at": d.CreatedAt, "last_seen_at": d.LastSeenAt, "signed_out_at": d.RevokedAt})
	}
	b.Documents = append(b.Documents, ExportDocument{Path: "devices.json", Data: dev})
	var consents []domain.Consent
	if err := rows(&consents, "user_id"); err != nil {
		return nil, err
	}
	var history []domain.ConsentEvent
	if err := rows(&history, "user_id"); err != nil {
		return nil, err
	}
	hist := make([]map[string]any, 0, len(history))
	for _, h := range history {
		hist = append(hist, map[string]any{"category": h.Category, "granted": h.Granted, "policy_version": h.PolicyVersion,
			"source": h.Source, "at": h.CreatedAt})
	}
	b.Documents = append(b.Documents, ExportDocument{Path: "consents.json", Data: map[string]any{
		"current": consentRecords(consents), "history": hist}})

	// Credits.
	var ledger []domain.LedgerEntry
	if err := rows(&ledger, "user_id"); err != nil {
		return nil, err
	}
	capped(len(ledger), "credit movements")
	led := make([]map[string]any, 0, len(ledger))
	for _, l := range ledger {
		led = append(led, map[string]any{"id": l.ID, "type": l.EntryType, "amount": l.Amount, "held_amount": l.HeldAmount,
			"balance_after": l.BalanceAfter, "held_after": l.HeldAfter, "reference_type": l.ReferenceType, "reference_id": l.ReferenceID,
			"description": l.Description, "at": l.CreatedAt})
	}
	var purchases []domain.CreditPurchase
	if err := rows(&purchases, "user_id"); err != nil {
		return nil, err
	}
	pur := make([]map[string]any, 0, len(purchases))
	for _, x := range purchases {
		pur = append(pur, map[string]any{"id": x.ID, "credits": x.Credits, "price_cents": x.PriceCents, "currency": x.Currency,
			"status": x.Status, "provider": x.Provider, "receipt_email": x.ReceiptEmail, "created_at": x.CreatedAt, "completed_at": x.CompletedAt})
	}
	b.Documents = append(b.Documents, ExportDocument{Path: "credits/ledger.json", Data: led},
		ExportDocument{Path: "credits/purchases.json", Data: pur})

	if err := s.collectChat(ctx, b, u, rows, capped); err != nil {
		return nil, err
	}
	if err := s.collectActivity(ctx, b, u, rows, capped); err != nil {
		return nil, err
	}
	if err := s.collectSupport(ctx, b, u.ID, rows, capped); err != nil { // revisions 3.0: support desk
		return nil, err
	}
	if p != nil && p.AvatarKey != "" && s.d.Avatars != nil {
		if data, _, err := s.d.Avatars.Get(ctx, p.AvatarKey, 4<<20); err == nil {
			b.Blobs = append(b.Blobs, ExportBlob{Path: "profile/avatar.jpg", Data: data})
		}
	}
	return b, nil
}

func consentRecords(rows []domain.Consent) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, c := range rows {
		out = append(out, map[string]any{"category": c.Category, "granted": c.Granted, "policy_version": c.PolicyVersion,
			"source": c.Source, "updated_at": c.UpdatedAt})
	}
	return out
}

type exportRows func(dest any, column string) error

// collectChat adds conversations, sessions, messages, bookmarks and the
// images the user sent.
func (s *ExportService) collectChat(ctx context.Context, b *ExportBundle, u *domain.User, rows exportRows, capped func(int, string)) error {
	convs, err := s.d.Data.Conversations(ctx, u.ID, s.cfg.RowLimit)
	if err != nil {
		return err
	}
	others := make([]uuid.UUID, 0, len(convs))
	for _, c := range convs {
		others = append(others, c.Counterpart(u.ID))
	}
	names := map[uuid.UUID]string{}
	if len(others) > 0 {
		people, err := s.d.Users.GetByIDs(ctx, others)
		if err != nil {
			return err
		}
		for _, p := range people {
			names[p.ID] = p.DisplayName
		}
	}
	conv := make([]map[string]any, 0, len(convs))
	for _, c := range convs {
		conv = append(conv, map[string]any{"id": c.ID, "with": names[c.Counterpart(u.ID)], "you_are": roleIn(&c, u.ID),
			"started_at": c.CreatedAt, "last_message_at": c.LastMessageAt})
	}
	var sessions []domain.Session
	column := "client_id"
	if u.Role == domain.RolePsychic {
		column = "psychic_id"
	}
	if err := rows(&sessions, column); err != nil {
		return err
	}
	sess := make([]map[string]any, 0, len(sessions))
	for _, x := range sessions {
		sess = append(sess, map[string]any{"id": x.ID, "conversation_id": x.ConversationID, "status": x.Status, "end_reason": x.EndReason,
			"rate_per_minute": x.RatePerMinute, "requested_at": x.RequestedAt, "started_at": x.StartedAt, "ended_at": x.EndedAt,
			"active_seconds": x.ActiveSeconds, "billed_minutes": x.BilledMinutes, "free_minutes_used": x.FreeMinutesUsed,
			"credits_charged": x.CreditsCharged})
	}
	msgs, err := s.d.Data.ConversationMessages(ctx, u.ID, s.cfg.RowLimit)
	if err != nil {
		return err
	}
	capped(len(msgs), "chat messages")
	msg := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		from := "counterpart"
		switch {
		case m.Kind == domain.MessageSystem || m.SenderID == nil:
			from = "system"
		case *m.SenderID == u.ID:
			from = "you"
		}
		msg = append(msg, map[string]any{"id": m.ID, "conversation_id": m.ConversationID, "session_id": m.SessionID, "off_session": m.SessionID == nil && m.Kind != domain.MessageSystem, "from": from,
			"kind": m.Kind, "body": m.Body, "at": m.CreatedAt})
	}
	var saved []domain.SavedMessage
	if err := rows(&saved, "user_id"); err != nil {
		return err
	}
	sav := make([]map[string]any, 0, len(saved))
	for _, x := range saved {
		sav = append(sav, map[string]any{"message_id": x.MessageID, "saved_at": x.CreatedAt})
	}
	b.Documents = append(b.Documents, ExportDocument{Path: "chat/conversations.json", Data: map[string]any{"conversations": conv, "sessions": sess}},
		ExportDocument{Path: "chat/messages.json", Data: msg}, ExportDocument{Path: "chat/saved_messages.json", Data: sav})

	var attachments []domain.Attachment
	if err := rows(&attachments, "uploader_id"); err != nil {
		return err
	}
	att := make([]map[string]any, 0, len(attachments))
	budget := s.cfg.MaxAttachmentBytes
	skipped := 0
	for _, a := range attachments {
		entry := map[string]any{"id": a.ID, "message_id": a.MessageID, "conversation_id": a.ConversationID, "content_type": a.ContentType,
			"width": a.Width, "height": a.Height, "bytes": a.Bytes, "sent_at": a.CreatedAt}
		ext := imageExt(a.ContentType)
		if s.d.ChatImages != nil && ext != "" && int64(a.Bytes) <= budget {
			data, _, err := s.d.ChatImages.Get(ctx, a.ObjectKey, budget)
			switch {
			case err == nil:
				path := "chat/attachments/" + a.ID.String() + "." + ext
				b.Blobs = append(b.Blobs, ExportBlob{Path: path, Data: data})
				budget -= int64(len(data))
				entry["file"] = path
			case errors.Is(err, storage.ErrNotFound):
			default:
				return err
			}
		} else if ext != "" {
			skipped++
		}
		att = append(att, entry)
	}
	if skipped > 0 {
		b.Omitted = append(b.Omitted, fmt.Sprintf("%d images were too large to include; they are listed in chat/attachments.json.", skipped))
	}
	b.Documents = append(b.Documents, ExportDocument{Path: "chat/attachments.json", Data: att})
	return nil
}

func roleIn(c *domain.Conversation, userID uuid.UUID) string {
	if c.ClientID == userID {
		return "client"
	}
	return "advisor"
}

func imageExt(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	}
	return ""
}

// collectActivity adds orders, reviews, notifications, advisor notes the
// user wrote, payouts and the export history.
func (s *ExportService) collectActivity(ctx context.Context, b *ExportBundle, u *domain.User, rows exportRows, capped func(int, string)) error {
	column := "client_id"
	if u.Role == domain.RolePsychic {
		column = "psychic_id"
	}
	var requests []domain.CustomRequest
	if err := rows(&requests, column); err != nil {
		return err
	}
	req := make([]map[string]any, 0, len(requests))
	for _, r := range requests {
		req = append(req, map[string]any{"id": r.ID, "service": r.ServiceName, "price_credits": r.PriceCredits, "status": r.Status,
			"note": r.ClientNote, "preferred_at": r.PreferredAt, "delivery": r.Delivery, "decline_reason": r.DeclineReason,
			"dispute_reason": r.DisputeReason, "resolution_note": r.ResolutionNote, "created_at": r.CreatedAt,
			"accepted_at": r.AcceptedAt, "delivered_at": r.DeliveredAt, "closed_at": r.ClosedAt})
	}
	var reviews []domain.Review
	if err := rows(&reviews, column); err != nil {
		return err
	}
	rev := make([]map[string]any, 0, len(reviews))
	for _, r := range reviews {
		rev = append(rev, map[string]any{"id": r.ID, "session_id": r.SessionID, "rating": r.Rating, "comment": r.Comment,
			"status": r.Status, "created_at": r.CreatedAt})
	}
	reviewKey := "written"
	if u.Role == domain.RolePsychic {
		reviewKey = "received"
	}
	var notifications []domain.Notification
	if err := rows(&notifications, "user_id"); err != nil {
		return err
	}
	capped(len(notifications), "notifications")
	notes := make([]map[string]any, 0, len(notifications))
	for _, n := range notifications {
		notes = append(notes, map[string]any{"kind": n.Kind, "data": n.Data, "link": n.Link, "read_at": n.ReadAt, "at": n.CreatedAt})
	}
	b.Documents = append(b.Documents, ExportDocument{Path: "requests.json", Data: req},
		ExportDocument{Path: "reviews.json", Data: map[string]any{reviewKey: rev}},
		ExportDocument{Path: "notifications.json", Data: notes})

	if u.Role == domain.RolePsychic {
		if err := s.collectAdvisor(ctx, b, u, rows); err != nil {
			return err
		}
	}
	var jobs []domain.DataExportJob
	if err := rows(&jobs, "user_id"); err != nil {
		return err
	}
	hist := make([]map[string]any, 0, len(jobs))
	for _, j := range jobs {
		hist = append(hist, map[string]any{"id": j.ID, "status": j.Status, "requested_at": j.RequestedAt, "completed_at": j.CompletedAt,
			"downloads": j.DownloadCount})
	}
	b.Documents = append(b.Documents, ExportDocument{Path: "exports.json", Data: hist})
	return nil
}

// collectAdvisor adds the advisor's own notes (never the notes other
// advisors wrote about them), earnings and payouts.
func (s *ExportService) collectAdvisor(ctx context.Context, b *ExportBundle, u *domain.User, rows exportRows) error {
	var written []domain.ClientNote
	if err := rows(&written, "author_id"); err != nil {
		return err
	}
	notes := make([]map[string]any, 0, len(written))
	for _, n := range written {
		body := ""
		if s.d.Sealer != nil {
			if plain, err := s.d.Sealer.Open(n.BodyEnc, noteAD(n.ID, n.AuthorID)); err == nil {
				body = plain
			} else {
				warnErr(ctx, "export: open note", err)
			}
		}
		notes = append(notes, map[string]any{"id": n.ID, "conversation_id": n.ConversationID, "body": body, "shared": n.Shared,
			"created_at": n.CreatedAt, "updated_at": n.UpdatedAt})
	}
	var earnings []domain.Earning
	if err := rows(&earnings, "psychic_id"); err != nil {
		return err
	}
	earn := make([]map[string]any, 0, len(earnings))
	for _, e := range earnings {
		earn = append(earn, map[string]any{"id": e.ID, "source_type": e.SourceType, "source_id": e.SourceID, "kind": e.EntryKind,
			"gross_credits": e.GrossCredits, "commission_credits": e.CommissionCredits, "net_credits": e.NetCredits,
			"share_percent": e.SharePercent, "at": e.CreatedAt})
	}
	var withdrawals []domain.Withdrawal
	if err := rows(&withdrawals, "psychic_id"); err != nil {
		return err
	}
	wd := make([]map[string]any, 0, len(withdrawals))
	for _, w := range withdrawals {
		wd = append(wd, map[string]any{"id": w.ID, "amount_credits": w.AmountCredits, "amount_cents": w.AmountCents, "currency": w.Currency,
			"method": w.Method, "destination": w.DestinationHint, "status": w.Status, "note": w.Note, "requested_at": w.RequestedAt,
			"processed_at": w.ProcessedAt})
	}
	b.Documents = append(b.Documents, ExportDocument{Path: "advisor/notes_written.json", Data: notes},
		ExportDocument{Path: "advisor/earnings.json", Data: earn}, ExportDocument{Path: "advisor/withdrawals.json", Data: wd})
	return nil
}
