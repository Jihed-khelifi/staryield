package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// ServiceView is a listed custom request offering (FR-06).
type ServiceView struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Subtitle     string    `json:"subtitle"`
	Description  string    `json:"description"`
	PriceCredits int64     `json:"price_credits"`
	Active       bool      `json:"active"`
}

// PsychicView is the public advisor profile with live status.
type PsychicView struct {
	ID              uuid.UUID     `json:"id"`
	Slug            string        `json:"slug"`
	Name            string        `json:"name"`
	Headline        string        `json:"headline"`
	About           []string      `json:"about"`
	Philosophy      string        `json:"philosophy"`
	ImageURL        string        `json:"image_url"`
	Zodiac          string        `json:"zodiac"`
	Skills          []string      `json:"skills"`
	Rating          float64       `json:"rating"`
	ReviewsCount    int           `json:"reviews_count"`
	ReadingsCount   int           `json:"readings_count"`
	YearsExperience int           `json:"years_experience"`
	RatePerMinute   int64         `json:"rate_per_minute"`
	Status          string        `json:"status"`
	Listed          bool          `json:"listed"`
	Services        []ServiceView `json:"services"`
}

// PsychicStatusEvent is broadcast when an advisor's effective status changes.
type PsychicStatusEvent struct {
	PsychicID uuid.UUID `json:"psychic_id"`
	Slug      string    `json:"slug"`
	Status    string    `json:"status"`
}

// EarningView is one earning record.
type EarningView struct {
	ID         uuid.UUID `json:"id"`
	SourceType string    `json:"source_type"`
	SourceID   uuid.UUID `json:"source_id"`
	Kind       string    `json:"kind"`
	Gross      int64     `json:"gross_credits"`
	Commission int64     `json:"commission_credits"`
	Net        int64     `json:"net_credits"`
	Share      int       `json:"share_percent"`
	CreatedAt  time.Time `json:"created_at"`
}

// EarningsReport is the psychic earnings page (FR-12).
type EarningsReport struct {
	Summary repository.EarningsSummary `json:"summary"`
	Entries Page[EarningView]          `json:"entries"`
}

// PsychicProfileUpdate is the subset of the profile a psychic may edit.
type PsychicProfileUpdate struct {
	Headline   *string   `json:"headline"`
	About      *[]string `json:"about"`
	Philosophy *string   `json:"philosophy"`
	Skills     *[]string `json:"skills"`
}

// PsychicService serves the advisor directory and advisor tools.
type PsychicService struct {
	psychics repository.PsychicRepository
	chat     repository.ChatRepository
	earnings repository.EarningRepository
	presence realtime.Presence
	events   EventPublisher
	tracker  analytics.Tracker
	clock    clock.Clock
}

// NewPsychicService wires the psychic service.
func NewPsychicService(psychics repository.PsychicRepository, chat repository.ChatRepository, earnings repository.EarningRepository,
	presence realtime.Presence, events EventPublisher, tracker analytics.Tracker, clk clock.Clock) *PsychicService {
	return &PsychicService{psychics: psychics, chat: chat, earnings: earnings, presence: presence, events: events, tracker: tracker, clock: clk}
}

// EffectiveStatus combines chosen availability, live connections and open
// sessions into the status clients see.
func EffectiveStatus(av domain.Availability, connected, busy bool) domain.PresenceStatus {
	switch {
	case av == domain.AvailabilityOffline || !connected:
		return domain.PresenceOffline
	case busy:
		return domain.PresenceBusy
	case av == domain.AvailabilityAway:
		return domain.PresenceAway
	}
	return domain.PresenceOnline
}

func servicesView(in []domain.PsychicService) []ServiceView {
	out := make([]ServiceView, 0, len(in))
	for _, s := range in {
		out = append(out, ServiceView{ID: s.ID, Name: s.Name, Subtitle: s.Subtitle, Description: s.Description, PriceCredits: s.PriceCredits, Active: s.Active})
	}
	return out
}

func psychicView(p *domain.Psychic, status domain.PresenceStatus) PsychicView {
	about, skills := []string(p.About), []string(p.Skills)
	if about == nil {
		about = []string{}
	}
	if skills == nil {
		skills = []string{}
	}
	return PsychicView{
		ID: p.UserID, Slug: p.Slug, Name: p.Name, Headline: p.Headline, About: about, Philosophy: p.Philosophy,
		ImageURL: p.ImageURL, Zodiac: p.Zodiac, Skills: skills, Rating: p.Rating, ReviewsCount: p.ReviewsCount,
		ReadingsCount: p.ReadingsCount, YearsExperience: p.YearsExperience, RatePerMinute: p.RatePerMinute,
		Status: string(status), Listed: p.Listed, Services: servicesView(p.Services),
	}
}

func (s *PsychicService) statuses(ctx context.Context, list []domain.Psychic) (map[uuid.UUID]domain.PresenceStatus, error) {
	ids := make([]uuid.UUID, len(list))
	for i, p := range list {
		ids[i] = p.UserID
	}
	online, err := s.presence.Online(ctx, ids)
	if err != nil {
		// Presence is best-effort: show advisors as offline rather than fail.
		warnErr(ctx, "psychics: presence lookup failed", err)
		online = map[uuid.UUID]bool{}
	}
	busyIDs, err := s.chat.BusyPsychics(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	busy := make(map[uuid.UUID]bool, len(busyIDs))
	for _, id := range busyIDs {
		busy[id] = true
	}
	out := make(map[uuid.UUID]domain.PresenceStatus, len(list))
	for _, p := range list {
		out[p.UserID] = EffectiveStatus(p.Availability, online[p.UserID], busy[p.UserID])
	}
	return out, nil
}

// List returns the listed advisors with live status (FR-06).
func (s *PsychicService) List(ctx context.Context) ([]PsychicView, error) {
	list, err := s.psychics.List(ctx, false)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	st, err := s.statuses(ctx, list)
	if err != nil {
		return nil, err
	}
	out := make([]PsychicView, 0, len(list))
	for i := range list {
		out = append(out, psychicView(&list[i], st[list[i].UserID]))
	}
	return out, nil
}

// Get returns one advisor profile by slug.
func (s *PsychicService) Get(ctx context.Context, slug string) (*PsychicView, error) {
	p, err := s.psychics.GetBySlug(ctx, slug)
	if err != nil {
		return nil, mapRepoErr(err, "Advisor")
	}
	if !p.Listed {
		return nil, apperror.NotFound("Advisor")
	}
	st, err := s.statuses(ctx, []domain.Psychic{*p})
	if err != nil {
		return nil, err
	}
	v := psychicView(p, st[p.UserID])
	return &v, nil
}

// Status returns one advisor's effective status.
func (s *PsychicService) Status(ctx context.Context, psychicID uuid.UUID) (domain.PresenceStatus, *domain.Psychic, error) {
	p, err := s.psychics.GetByUserID(ctx, psychicID)
	if err != nil {
		return "", nil, mapRepoErr(err, "Advisor")
	}
	st, err := s.statuses(ctx, []domain.Psychic{*p})
	if err != nil {
		return "", nil, err
	}
	return st[p.UserID], p, nil
}

// PublishStatus broadcasts an advisor's current effective status.
func (s *PsychicService) PublishStatus(ctx context.Context, psychicID uuid.UUID) {
	st, p, err := s.Status(ctx, psychicID)
	if err != nil {
		if !errors.Is(err, apperror.ErrNotFound) {
			warnErr(ctx, "psychics: status for broadcast", err)
		}
		return
	}
	warnErr(ctx, "psychics: broadcast status", s.events.Broadcast(ctx, realtime.EventPsychicStatusChanged,
		PsychicStatusEvent{PsychicID: p.UserID, Slug: p.Slug, Status: string(st)}))
}

// SetAvailability lets a psychic go online/away/offline.
func (s *PsychicService) SetAvailability(ctx context.Context, psychicID uuid.UUID, av domain.Availability) (*PsychicView, error) {
	if !av.Valid() {
		return nil, apperror.Validation(map[string]string{"availability": "Choose online, away or offline."})
	}
	if err := s.psychics.Updates(ctx, psychicID, map[string]any{"availability": av}); err != nil {
		return nil, mapRepoErr(err, "Advisor")
	}
	s.tracker.Track(ctx, "psychic.availability_changed", map[string]any{"availability": string(av)})
	s.PublishStatus(ctx, psychicID)
	st, p, err := s.Status(ctx, psychicID)
	if err != nil {
		return nil, err
	}
	v := psychicView(p, st)
	return &v, nil
}

// UpdateOwn applies a psychic's edits to their own profile (FR-08).
// Pricing is admin-only (BR-09) and therefore not editable here.
func (s *PsychicService) UpdateOwn(ctx context.Context, psychicID uuid.UUID, in PsychicProfileUpdate) (*PsychicView, error) {
	fields := map[string]string{}
	upd := map[string]any{}
	if in.Headline != nil {
		v := CleanLine(*in.Headline)
		if len([]rune(v)) > 200 {
			fields["headline"] = "Use at most 200 characters."
		} else {
			upd["headline"] = v
		}
	}
	if in.Philosophy != nil {
		v := CleanText(*in.Philosophy)
		if len([]rune(v)) > 600 {
			fields["philosophy"] = "Use at most 600 characters."
		} else {
			upd["philosophy"] = v
		}
	}
	if in.About != nil {
		if len(*in.About) > 6 {
			fields["about"] = "Use at most 6 paragraphs."
		} else {
			paras := make(domain.StringList, 0, len(*in.About))
			for _, p := range *in.About {
				v := CleanText(p)
				if len([]rune(v)) > 2000 {
					fields["about"] = "Each paragraph must be at most 2000 characters."
					break
				}
				if v != "" {
					paras = append(paras, v)
				}
			}
			upd["about"] = paras
		}
	}
	if in.Skills != nil {
		if len(*in.Skills) > 8 {
			fields["skills"] = "Use at most 8 skills."
		} else {
			skills := make(domain.StringList, 0, len(*in.Skills))
			for _, sk := range *in.Skills {
				v := CleanLine(sk)
				if v == "" || len([]rune(v)) > 40 {
					fields["skills"] = "Each skill must be between 1 and 40 characters."
					break
				}
				skills = append(skills, v)
			}
			upd["skills"] = skills
		}
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if len(upd) > 0 {
		if err := s.psychics.Updates(ctx, psychicID, upd); err != nil {
			return nil, mapRepoErr(err, "Advisor")
		}
	}
	st, p, err := s.Status(ctx, psychicID)
	if err != nil {
		return nil, err
	}
	v := psychicView(p, st)
	return &v, nil
}

// Earnings returns the psychic's earnings summary and entries (FR-12).
func (s *PsychicService) Earnings(ctx context.Context, psychicID uuid.UUID, from, to *time.Time, cursor string, limit int) (*EarningsReport, error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	sum, err := s.earnings.Summary(ctx, psychicID, from, to)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	rows, err := s.earnings.List(ctx, psychicID, from, to, c, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	page := Page[EarningView]{Items: make([]EarningView, 0, len(rows))}
	for _, e := range rows {
		page.Items = append(page.Items, EarningView{ID: e.ID, SourceType: string(e.SourceType), SourceID: e.SourceID, Kind: e.EntryKind,
			Gross: e.GrossCredits, Commission: e.CommissionCredits, Net: e.NetCredits, Share: e.SharePercent, CreatedAt: e.CreatedAt})
	}
	if len(rows) == clampPage(limit, 50, 200) && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return &EarningsReport{Summary: sum, Entries: page}, nil
}
