package service

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// Advisor settings messages (translated at the API edge).
const (
	msgPhoneFormat    = "Enter a phone number in international format, e.g. +14155552671."
	msgCustomPrice    = "Use a price between 10 and 50,000 credits."
	msgCustomDays     = "Use between 1 and 60 days."
	msgCustomName     = "Use between 1 and 80 characters."
	msgCustomDesc     = "Use at most 1000 characters."
	msgCustomRequired = "This field is required."
)

// e164 is the international phone format: "+", country code, 7–15 digits.
var e164 = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

// AdvisorDeps are the advisor settings service's collaborators.
type AdvisorDeps struct {
	Tx       database.TxManager
	Psychics repository.PsychicRepository
	Photos   repository.PhotoSubmissionRepository
	// PsychicSvc refreshes the public directory and broadcasts status.
	PsychicSvc *PsychicService
	Audit      repository.AuditRepository
	Sealer     *security.Sealer
	// Private holds pending photos (the private bucket, advisor-photos/).
	Private storage.ObjectStore
	// Public is the public advisor image bucket (R2_ADVISOR_IMAGE).
	Public   storage.ObjectStore
	Notifier Notifier
	Tracker  analytics.Tracker
	Clock    clock.Clock
	Settings AdvisorSettings
}

// AdvisorSettings tune photo submissions.
type AdvisorSettings struct {
	// Image is the upload policy (bytes, pixels, longest side).
	Image ImagePolicy
	// MinSide is the smallest accepted width and height in pixels.
	MinSide int
	// DailyUploads caps uploads per advisor per 24 h (withdrawn ones count).
	DailyUploads int
	// RejectedRetention is how long a rejected photo stays visible to its
	// advisor before the clean-up deletes it.
	RejectedRetention time.Duration
	// PurgeAfter hard-deletes soft-deleted submissions after this long.
	PurgeAfter   time.Duration
	IPHashSecret string
}

func (s AdvisorSettings) withDefaults() AdvisorSettings {
	if s.MinSide <= 0 {
		s.MinSide = 256
	}
	if s.DailyUploads <= 0 {
		s.DailyUploads = 10
	}
	if s.RejectedRetention <= 0 {
		s.RejectedRetention = 7 * 24 * time.Hour
	}
	if s.PurgeAfter <= 0 {
		s.PurgeAfter = 90 * 24 * time.Hour
	}
	if s.Image.Field == "" {
		s.Image.Field = "photo"
	}
	return s
}

// AdvisorService serves the advisor's own settings page: contact details
// and profile text (GET/PATCH /psychic/settings), the photo submitted for
// review (/psychic/photo, reviewed under /admin/photo-submissions) and the
// custom offering (/psychic/service).
type AdvisorService struct {
	d   AdvisorDeps
	cfg AdvisorSettings
}

// NewAdvisorService wires the advisor settings service.
func NewAdvisorService(d AdvisorDeps) *AdvisorService {
	if d.Notifier == nil {
		d.Notifier = NoopNotifier{}
	}
	if d.Tracker == nil {
		d.Tracker = analytics.Noop{}
	}
	return &AdvisorService{d: d, cfg: d.Settings.withDefaults()}
}

func (s *AdvisorService) record(ctx context.Context, action, targetType, targetID string, meta map[string]any) {
	if s.d.Audit == nil {
		return
	}
	recordAudit(ctx, s.d.Audit, s.d.Clock, s.cfg.IPHashSecret, action, targetType, targetID, meta)
}

// ---------------------------------------------------------------------------
// Settings (GET/PATCH /psychic/settings)
// ---------------------------------------------------------------------------

// AdvisorSettingsView is the advisor's settings document. The phone number
// is only ever returned to its owner; PhoneMasked keeps the last two digits.
// Version increments on every save: send it back as expected_version.
type AdvisorSettingsView struct {
	Version     int      `json:"version"`
	Phone       string   `json:"phone"`
	PhoneMasked string   `json:"phone_masked"`
	Headline    string   `json:"headline"`
	About       []string `json:"about"`
	Philosophy  string   `json:"philosophy"`
	// Modalities are the public profile's skills.
	Modalities []string `json:"modalities"`
}

// AdvisorSettingsPatch is a partial settings update; absent fields are
// unchanged. An empty phone removes the number.
type AdvisorSettingsPatch struct {
	// ExpectedVersion, when sent, must match the current version (else 409
	// version_conflict).
	ExpectedVersion *int      `json:"expected_version"`
	Phone           *string   `json:"phone"`
	Headline        *string   `json:"headline"`
	About           *[]string `json:"about"`
	Philosophy      *string   `json:"philosophy"`
	Modalities      *[]string `json:"modalities"`
}

// phoneAD binds a sealed phone number to its advisor, so a ciphertext
// copied to another row does not open.
func phoneAD(psychicID uuid.UUID) string { return "advisor-phone:" + psychicID.String() }

// NormalizePhone strips the separators people type ("+44 20 7946-0958")
// and checks the E.164 format. It returns "" for an empty input.
func NormalizePhone(raw string) (string, bool) {
	v := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '-', '.', '(', ')', ' ', '\t':
			return -1
		}
		return r
	}, CleanLine(raw))
	if v == "" {
		return "", true
	}
	return v, e164.MatchString(v)
}

// MaskPhone hides every digit of an E.164 number but the last two.
func MaskPhone(phone string) string {
	if len(phone) < 4 || phone[0] != '+' {
		return ""
	}
	digits := phone[1:]
	return "+" + strings.Repeat("•", len(digits)-2) + digits[len(digits)-2:]
}

func (s *AdvisorService) settingsView(ctx context.Context, p *domain.Psychic) *AdvisorSettingsView {
	v := &AdvisorSettingsView{Version: p.SettingsVersion, Headline: p.Headline, Philosophy: p.Philosophy,
		About: []string(p.About), Modalities: []string(p.Skills)}
	if v.Version < 1 {
		v.Version = 1
	}
	if v.About == nil {
		v.About = []string{}
	}
	if v.Modalities == nil {
		v.Modalities = []string{}
	}
	if p.PhoneSealed != "" && s.d.Sealer != nil {
		phone, err := s.d.Sealer.Open(p.PhoneSealed, phoneAD(p.UserID))
		if err != nil {
			// A value that does not open is treated as absent (and logged),
			// never shown half-decrypted.
			warnErr(ctx, "advisor: open phone", err, zap.String("psychic_id", p.UserID.String()))
		} else {
			v.Phone, v.PhoneMasked = phone, MaskPhone(phone)
		}
	}
	return v
}

// Settings returns the advisor's settings document.
func (s *AdvisorService) Settings(ctx context.Context, psychicID uuid.UUID) (*AdvisorSettingsView, error) {
	p, err := s.d.Psychics.GetByUserID(ctx, psychicID)
	if err != nil {
		return nil, mapRepoErr(err, "Advisor")
	}
	return s.settingsView(ctx, p), nil
}

// UpdateSettings validates and saves a settings patch in one transaction,
// under a row lock so concurrent saves cannot silently overwrite each other.
func (s *AdvisorService) UpdateSettings(ctx context.Context, psychicID uuid.UUID, in AdvisorSettingsPatch) (*AdvisorSettingsView, error) {
	upd, fields := profileChanges(PsychicProfileUpdate{Headline: in.Headline, About: in.About, Philosophy: in.Philosophy,
		Skills: in.Modalities})
	if msg, ok := fields["skills"]; ok {
		delete(fields, "skills")
		fields["modalities"] = msg
	}
	var phone *string
	if in.Phone != nil {
		v, ok := NormalizePhone(*in.Phone)
		if !ok {
			fields["phone"] = msgPhoneFormat
		} else {
			phone = &v
		}
	}
	if in.ExpectedVersion != nil && *in.ExpectedVersion < 1 {
		fields["expected_version"] = "Has the wrong type."
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	var changed []string
	public := false
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		p, err := s.d.Psychics.GetForUpdate(ctx, psychicID)
		if err != nil {
			return err
		}
		version := max(p.SettingsVersion, 1)
		if in.ExpectedVersion != nil && *in.ExpectedVersion != version {
			return apperror.VersionConflict(version)
		}
		// Only real changes are written (and bump the version).
		cols := map[string]any{}
		if v, ok := upd["headline"].(string); ok && v != p.Headline {
			cols["headline"], public = v, true
		}
		if v, ok := upd["philosophy"].(string); ok && v != p.Philosophy {
			cols["philosophy"], public = v, true
		}
		if v, ok := upd["about"].(domain.StringList); ok && !equalStrings(v, p.About) {
			cols["about"], public = v, true
		}
		if v, ok := upd["skills"].(domain.StringList); ok && !equalStrings(v, p.Skills) {
			cols["skills"], public = v, true
		}
		if phone != nil {
			current := ""
			if p.PhoneSealed != "" && s.d.Sealer != nil {
				current, _ = s.d.Sealer.Open(p.PhoneSealed, phoneAD(p.UserID))
			}
			if *phone != current || (*phone == "" && p.PhoneSealed != "") {
				sealed := ""
				if *phone != "" {
					if s.d.Sealer == nil {
						return apperror.FeatureUnavailable("Phone numbers cannot be saved right now.")
					}
					if sealed, err = s.d.Sealer.Seal(*phone, phoneAD(p.UserID)); err != nil {
						return err
					}
				}
				cols["phone_sealed"] = sealed
			}
		}
		if len(cols) == 0 {
			return nil
		}
		for k := range cols {
			changed = append(changed, k)
		}
		cols["settings_version"] = version + 1
		return s.d.Psychics.Updates(ctx, psychicID, cols)
	})
	if err != nil {
		if _, ok := apperror.As(err); ok {
			return nil, err
		}
		return nil, mapRepoErr(err, "Advisor")
	}
	if len(changed) > 0 {
		sort.Strings(changed)
		for i, k := range changed {
			if k == "phone_sealed" {
				changed[i] = "phone" // the audit trail names fields, never values
			}
		}
		if public && s.d.PsychicSvc != nil {
			s.d.PsychicSvc.DirectoryChanged()
		}
		s.record(ctx, "psychic.settings_updated", "psychic", psychicID.String(), map[string]any{"fields": strings.Join(changed, ",")})
		s.d.Tracker.Track(ctx, "psychic.settings_updated", map[string]any{"fields": strings.Join(changed, ","), "count": len(changed)})
	}
	return s.Settings(ctx, psychicID)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Custom offering (GET/PUT/DELETE /psychic/service)
// ---------------------------------------------------------------------------

// CustomServiceView is the advisor's own custom-request offering. Active
// is false when the Staryield team has hidden it.
type CustomServiceView struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	PriceCredits int64     `json:"price_credits"`
	DeliveryDays int       `json:"delivery_days"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CustomServiceLimits are the bounds of the custom offering.
type CustomServiceLimits struct {
	NameMax        int   `json:"name_max"`
	DescriptionMax int   `json:"description_max"`
	MinPrice       int64 `json:"min_price_credits"`
	MaxPrice       int64 `json:"max_price_credits"`
	MinDays        int   `json:"min_delivery_days"`
	MaxDays        int   `json:"max_delivery_days"`
}

// CustomServiceState is GET /psychic/service: the offering (null when the
// advisor has none) and its bounds.
type CustomServiceState struct {
	Service *CustomServiceView  `json:"service"`
	Limits  CustomServiceLimits `json:"limits"`
}

// CustomServiceInput replaces the custom offering (PUT): name, price and
// delivery days are required, the description is optional.
type CustomServiceInput struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	PriceCredits *int64  `json:"price_credits"`
	DeliveryDays *int    `json:"delivery_days"`
}

var customLimits = CustomServiceLimits{NameMax: domain.CustomServiceNameMax, DescriptionMax: domain.CustomServiceDescriptionMax,
	MinPrice: domain.CustomServiceMinPrice, MaxPrice: domain.CustomServiceMaxPrice,
	MinDays: domain.CustomServiceMinDays, MaxDays: domain.CustomServiceMaxDays}

func customServiceView(svc *domain.PsychicService) *CustomServiceView {
	return &CustomServiceView{ID: svc.ID, Name: svc.Name, Description: svc.Description, PriceCredits: svc.PriceCredits,
		DeliveryDays: svc.DeliveryDays, Active: svc.Active, CreatedAt: svc.CreatedAt, UpdatedAt: svc.UpdatedAt}
}

// validCustomService validates a PUT body.
func validCustomService(in CustomServiceInput) (name, desc string, price int64, days int, fields map[string]string) {
	fields = map[string]string{}
	if in.Name == nil {
		fields["name"] = msgCustomRequired
	} else if name = CleanLine(*in.Name); name == "" || utf8.RuneCountInString(name) > domain.CustomServiceNameMax {
		fields["name"] = msgCustomName
	}
	if in.Description != nil {
		if desc = CleanText(*in.Description); utf8.RuneCountInString(desc) > domain.CustomServiceDescriptionMax {
			fields["description"] = msgCustomDesc
		}
	}
	if in.PriceCredits == nil {
		fields["price_credits"] = msgCustomRequired
	} else if price = *in.PriceCredits; price < domain.CustomServiceMinPrice || price > domain.CustomServiceMaxPrice {
		fields["price_credits"] = msgCustomPrice
	}
	if in.DeliveryDays == nil {
		fields["delivery_days"] = msgCustomRequired
	} else if days = *in.DeliveryDays; days < domain.CustomServiceMinDays || days > domain.CustomServiceMaxDays {
		fields["delivery_days"] = msgCustomDays
	}
	return name, desc, price, days, fields
}

// CustomService returns the advisor's custom offering (nil when none).
func (s *AdvisorService) CustomService(ctx context.Context, psychicID uuid.UUID) (*CustomServiceState, error) {
	out := &CustomServiceState{Limits: customLimits}
	svc, err := s.d.Psychics.CustomService(ctx, psychicID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return out, nil
		}
		return nil, apperror.Internal(err)
	}
	out.Service = customServiceView(svc)
	return out, nil
}

// PutCustomService creates or replaces the advisor's custom offering. It
// reports whether the offering was created. Admin-curated services are
// never touched, and an offering the team has hidden stays hidden.
func (s *AdvisorService) PutCustomService(ctx context.Context, psychicID uuid.UUID, in CustomServiceInput) (*CustomServiceState, bool, error) {
	name, desc, price, days, fields := validCustomService(in)
	if len(fields) > 0 {
		return nil, false, apperror.Validation(fields)
	}
	var svc *domain.PsychicService
	created := false
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		// The advisor row lock serialises concurrent saves of the offering.
		if _, err := s.d.Psychics.GetForUpdate(ctx, psychicID); err != nil {
			return err
		}
		existing, err := s.d.Psychics.CustomService(ctx, psychicID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		now := s.d.Clock.Now()
		if existing != nil {
			if err := s.d.Psychics.UpdateService(ctx, existing.ID, map[string]any{"name": name, "description": desc,
				"price_credits": price, "delivery_days": days}); err != nil {
				return err
			}
			existing.Name, existing.Description, existing.PriceCredits, existing.DeliveryDays = name, desc, price, days
			existing.UpdatedAt = now
			svc = existing
			return nil
		}
		svc = &domain.PsychicService{ID: domain.NewID(), PsychicID: psychicID, Name: name, Description: desc, PriceCredits: price,
			Active: true, SortOrder: domain.CustomServiceSortOrder, Kind: domain.ServiceCustom, DeliveryDays: days,
			CreatedAt: now, UpdatedAt: now}
		created = true
		return s.d.Psychics.CreateService(ctx, svc)
	})
	if err != nil {
		if database.IsUniqueViolation(err, "psychic_services_one_custom") {
			return nil, false, apperror.Conflict(apperror.CodeConflict, "Your offering was changed at the same time. Reload and try again.")
		}
		return nil, false, mapRepoErr(err, "Advisor")
	}
	action := "psychic.custom_service_updated"
	if created {
		action = "psychic.custom_service_created"
	}
	s.record(ctx, action, "psychic_service", svc.ID.String(), map[string]any{"price_credits": price, "delivery_days": days})
	s.d.Tracker.Track(ctx, "psychic.custom_service_saved", map[string]any{"created": created, "price_credits": price, "delivery_days": days})
	return &CustomServiceState{Service: customServiceView(svc), Limits: customLimits}, created, nil
}

// DeleteCustomService removes the advisor's custom offering. It is soft
// deleted, so orders placed for it keep their history.
func (s *AdvisorService) DeleteCustomService(ctx context.Context, psychicID uuid.UUID) error {
	var removed *domain.PsychicService
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.d.Psychics.GetForUpdate(ctx, psychicID); err != nil {
			return err
		}
		svc, err := s.d.Psychics.CustomService(ctx, psychicID)
		if err != nil {
			return err
		}
		removed = svc
		return s.d.Psychics.DeleteService(ctx, svc.ID)
	})
	if err != nil {
		return mapRepoErr(err, "Custom offering")
	}
	s.record(ctx, "psychic.custom_service_deleted", "psychic_service", removed.ID.String(), nil)
	s.d.Tracker.Track(ctx, "psychic.custom_service_deleted", nil)
	return nil
}
