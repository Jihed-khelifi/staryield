package service

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// AccountSettings tune the account self-service flows.
type AccountSettings struct {
	FrontendURL  string
	IPHashSecret string
	MinimumAge   int
	// EmailChangeTTL bounds the confirmation link sent to a new address.
	EmailChangeTTL time.Duration
	// RevertWindow is how long the previous address can cancel or undo a
	// change.
	RevertWindow time.Duration
	// ClosureGrace is the delay before a closed account is erased.
	ClosureGrace time.Duration
	// ResetTTL bounds the password-reset link sent after a restore.
	ResetTTL time.Duration
}

func (c AccountSettings) withDefaults() AccountSettings {
	if c.EmailChangeTTL <= 0 {
		c.EmailChangeTTL = 30 * time.Minute
	}
	if c.RevertWindow <= 0 {
		c.RevertWindow = 7 * 24 * time.Hour
	}
	if c.ClosureGrace <= 0 {
		c.ClosureGrace = 30 * 24 * time.Hour
	}
	if c.ResetTTL <= 0 {
		c.ResetTTL = time.Hour
	}
	return c
}

// AccountDeps are the account service collaborators. Payouts, Chat,
// PsychicSvc, Eraser and Notifier are optional.
type AccountDeps struct {
	Tx         database.TxManager
	Users      repository.UserRepository
	Accounts   repository.AccountRepository
	Tokens     repository.TokenRepository
	Actions    repository.ActionTokenRepository
	Psychics   repository.PsychicRepository
	Requests   repository.CustomRequestRepository
	Audit      repository.AuditRepository
	Auth       *AuthService
	UserSvc    *UserService
	Consents   *ConsentService
	Payouts    *WithdrawalService
	Chat       *ChatService
	PsychicSvc *PsychicService
	Eraser     *AccountEraser
	Mail       EmailQueue
	Notifier   Notifier
	Events     EventPublisher
	Hasher     *security.PasswordHasher
	Sealer     *security.Sealer
	Places     PlaceLookup
	Tracker    analytics.Tracker
	Clock      clock.Clock
	Settings   AccountSettings
}

// AccountService implements the account self-service API: settings, e-mail
// change, deactivation, closure and reactivation.
type AccountService struct {
	d   AccountDeps
	cfg AccountSettings
}

// NewAccountService wires the account service.
func NewAccountService(d AccountDeps) *AccountService {
	if d.Notifier == nil {
		d.Notifier = NoopNotifier{}
	}
	if d.Events == nil {
		d.Events = NoopPublisher{}
	}
	if d.Tracker == nil {
		d.Tracker = analytics.Noop{}
	}
	return &AccountService{d: d, cfg: d.Settings.withDefaults()}
}

// ---------------------------------------------------------------------------
// Settings (GET/PATCH /me/settings)
// ---------------------------------------------------------------------------

// SettingsView is the whole settings page in one document. Version
// increments on every save; send it back as expected_version.
type SettingsView struct {
	Version       int                  `json:"version"`
	Account       AccountSettingsView  `json:"account"`
	Profile       ProfileSettingsView  `json:"profile"`
	Appearance    AppearanceSettings   `json:"appearance"`
	Notifications NotificationSettings `json:"notifications"`
	Privacy       PrivacySettingsView  `json:"privacy"`
}

// AccountSettingsView is the account section (e-mail changes go through
// POST /me/email-change).
type AccountSettingsView struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	// PendingEmail is the masked address awaiting confirmation, if any.
	PendingEmail string    `json:"pending_email,omitempty"`
	DisplayName  string    `json:"display_name"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// ProfileSettingsView is the personal and birth data section.
type ProfileSettingsView struct {
	Nickname     string `json:"nickname"`
	BirthDate    string `json:"birth_date"`
	BirthTime    string `json:"birth_time"`
	BirthPlaceID string `json:"birth_place_id"`
	// BirthPlace is the label shown for the birthplace.
	BirthPlace     string   `json:"birth_place"`
	BirthLatitude  *float64 `json:"birth_latitude"`
	BirthLongitude *float64 `json:"birth_longitude"`
	BirthTimezone  string   `json:"birth_timezone"`
	CountryCode    string   `json:"country_code"`
	Location       string   `json:"location"`
	Mood           string   `json:"mood"`
	HouseSystem    string   `json:"house_system"`
	AvatarURL      string   `json:"avatar_url"`
}

// AppearanceSettings is the appearance section.
type AppearanceSettings struct {
	Theme  string `json:"theme"`
	Locale string `json:"locale"`
	// TimeZone is an IANA zone; "" follows the browser.
	TimeZone string `json:"time_zone"`
}

// NotificationSettings are the delivery switches. In-app notifications are
// always recorded.
type NotificationSettings struct {
	Email     bool `json:"email"`
	Push      bool `json:"push"`
	Marketing bool `json:"marketing"`
	Horoscope bool `json:"horoscope"`
	Events    bool `json:"events"`
	Chart     bool `json:"chart"`
}

// PrivacySettingsView is the privacy section.
type PrivacySettingsView struct {
	ShareBirthDetailsWithAdvisors bool `json:"share_birth_details_with_advisors"`
	// Consents are the effective choices (defaults applied) per optional
	// category; see GET /consent-policy.
	Consents      map[string]bool `json:"consents"`
	PolicyVersion string          `json:"policy_version"`
}

// SettingsPatch is a partial settings update; nil fields are unchanged.
type SettingsPatch struct {
	// ExpectedVersion, when sent, must match the current version (else
	// 409 version_conflict).
	ExpectedVersion *int                  `json:"expected_version"`
	Account         *AccountSettingsPatch `json:"account"`
	Profile         *ProfileSettingsPatch `json:"profile"`
	Appearance      *AppearancePatch      `json:"appearance"`
	Notifications   *NotificationsPatch   `json:"notifications"`
	Privacy         *PrivacyPatch         `json:"privacy"`
}

// AccountSettingsPatch edits the account section.
type AccountSettingsPatch struct {
	DisplayName *string `json:"display_name"`
}

// ProfileSettingsPatch edits personal and birth data. An empty string
// clears birth_date, birth_time or birth_place_id.
type ProfileSettingsPatch struct {
	Nickname     *string `json:"nickname"`
	BirthDate    *string `json:"birth_date"`
	BirthTime    *string `json:"birth_time"`
	BirthPlaceID *string `json:"birth_place_id"`
	// BirthPlaceLabel overrides the displayed birthplace name.
	BirthPlaceLabel *string `json:"birth_place_label"`
	Location        *string `json:"location"`
	Mood            *string `json:"mood"`
	HouseSystem     *string `json:"house_system"`
}

// AppearancePatch edits the appearance section.
type AppearancePatch struct {
	Theme    *string `json:"theme"`
	Locale   *string `json:"locale"`
	TimeZone *string `json:"time_zone"`
}

// NotificationsPatch edits the delivery switches.
type NotificationsPatch struct {
	Email     *bool `json:"email"`
	Push      *bool `json:"push"`
	Marketing *bool `json:"marketing"`
	Horoscope *bool `json:"horoscope"`
	Events    *bool `json:"events"`
	Chart     *bool `json:"chart"`
}

// PrivacyPatch edits the privacy section.
type PrivacyPatch struct {
	ShareBirthDetailsWithAdvisors *bool           `json:"share_birth_details_with_advisors"`
	Consents                      map[string]bool `json:"consents"`
}

// settingsChange is a validated patch: column updates per table plus
// consent choices.
type settingsChange struct {
	user     map[string]any
	profile  map[string]any
	consents map[domain.ConsentCategory]bool
	sections []string
}

func (c *settingsChange) empty() bool {
	return len(c.user) == 0 && len(c.profile) == 0 && len(c.consents) == 0
}

// validation shapes field paths and legacy leniency: /me/settings reports
// dotted paths ("profile.birth_date"); the PATCH /me/profile alias keeps
// its flat names and lenient locale.
type validation struct {
	key     func(section, field string) string
	lenient bool
}

var (
	settingsPaths = validation{key: func(section, field string) string { return section + "." + field }}
	profilePaths  = validation{key: func(_, field string) string { return field }, lenient: true}
	// geoNamesID matches the GeoNames place ids of the place search
	// ("gn:2988507"), accepted even before the gazetteer can resolve them.
	geoNamesID = regexp.MustCompile(`^gn:[0-9]{1,12}$`)
)

// ValidTimeZone reports whether tz is "" or a loadable IANA zone name.
func ValidTimeZone(tz string) bool {
	if tz == "" {
		return true
	}
	if len(tz) > 64 || tz == "Local" || strings.HasPrefix(tz, "/") || strings.Contains(tz, "..") {
		return false
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}

func (s *AccountService) lookupPlace(id string) (PlaceInfo, bool) {
	if s.d.Places == nil || len(id) > 80 {
		return PlaceInfo{}, false
	}
	return s.d.Places(id)
}

// validateSettings checks a patch without touching storage. Every problem
// is reported (field path → message); nothing is written unless all pass.
func (s *AccountService) validateSettings(in SettingsPatch, v validation) (*settingsChange, map[string]string) {
	fields := map[string]string{}
	ch := &settingsChange{user: map[string]any{}, profile: map[string]any{}}
	now := s.d.Clock.Now()
	if a := in.Account; a != nil {
		ch.sections = append(ch.sections, "account")
		if a.DisplayName != nil {
			name := CleanLine(*a.DisplayName)
			if name == "" || len([]rune(name)) > 80 {
				fields[v.key("account", "display_name")] = "Use between 1 and 80 characters."
			} else {
				ch.user["display_name"] = name
			}
		}
	}
	if p := in.Profile; p != nil {
		ch.sections = append(ch.sections, "profile")
		s.validateProfile(p, v, now, ch, fields)
	}
	if a := in.Appearance; a != nil {
		ch.sections = append(ch.sections, "appearance")
		if a.Theme != nil {
			if !allowedThemes[*a.Theme] {
				fields[v.key("appearance", "theme")] = "Choose a theme from the list."
			} else {
				ch.profile["theme"] = *a.Theme
			}
		}
		if a.Locale != nil {
			switch l := strings.ToLower(strings.TrimSpace(*a.Locale)); {
			case l == "en" || l == "fr" || l == "es":
				ch.user["locale"] = l
			case v.lenient:
				ch.user["locale"] = normalizeLocale(l)
			default:
				fields[v.key("appearance", "locale")] = "Choose English, French or Spanish."
			}
		}
		if a.TimeZone != nil {
			tz := strings.TrimSpace(*a.TimeZone)
			if !ValidTimeZone(tz) {
				fields[v.key("appearance", "time_zone")] = "Choose a time zone from the list."
			} else {
				ch.profile["timezone"] = tz
			}
		}
	}
	if n := in.Notifications; n != nil {
		ch.sections = append(ch.sections, "notifications")
		for col, val := range map[string]*bool{"notify_email": n.Email, "notify_push": n.Push, "notify_marketing": n.Marketing,
			"notify_horoscope": n.Horoscope, "notify_events": n.Events, "notify_chart": n.Chart} {
			if val != nil {
				ch.profile[col] = *val
			}
		}
	}
	if p := in.Privacy; p != nil {
		ch.sections = append(ch.sections, "privacy")
		if p.ShareBirthDetailsWithAdvisors != nil {
			ch.profile["share_birth_details_with_advisors"] = *p.ShareBirthDetailsWithAdvisors
		}
		if p.Consents != nil {
			ch.consents = ParseConsentChoices(p.Consents, v.key("privacy", "consents")+".", fields)
		}
	}
	return ch, fields
}

func (s *AccountService) validateProfile(p *ProfileSettingsPatch, v validation, now time.Time, ch *settingsChange, fields map[string]string) {
	if p.Nickname != nil {
		if n := CleanLine(*p.Nickname); len([]rune(n)) > 40 {
			fields[v.key("profile", "nickname")] = "Use at most 40 characters."
		} else {
			ch.profile["nickname"] = n
		}
	}
	if p.BirthDate != nil {
		if strings.TrimSpace(*p.BirthDate) == "" {
			ch.profile["birth_date"] = nil
		} else if d, msg := ParseBirthDate(*p.BirthDate, now, s.cfg.MinimumAge); msg != "" {
			fields[v.key("profile", "birth_date")] = msg
		} else {
			ch.profile["birth_date"] = *d
		}
	}
	if p.BirthTime != nil {
		t := strings.TrimSpace(*p.BirthTime)
		if t != "" {
			if _, err := time.Parse("15:04", t); err != nil || len(t) != 5 {
				fields[v.key("profile", "birth_time")] = "Use the 24-hour format HH:MM."
			}
		}
		if _, bad := fields[v.key("profile", "birth_time")]; !bad {
			ch.profile["birth_time"] = t
		}
	}
	var label *string
	if p.BirthPlaceLabel != nil {
		l := CleanLine(*p.BirthPlaceLabel)
		if len([]rune(l)) > 120 {
			fields[v.key("profile", "birth_place_label")] = "Use at most 120 characters."
		} else {
			label = &l
		}
	}
	if p.BirthPlaceID != nil {
		id := strings.TrimSpace(*p.BirthPlaceID)
		switch place, ok := s.lookupPlace(id); {
		case id == "":
			ch.profile["birth_place_id"], ch.profile["birth_place"], ch.profile["birth_place_label"] = "", "", ""
			ch.profile["birth_latitude"], ch.profile["birth_longitude"], ch.profile["birth_timezone"] = nil, nil, ""
			ch.profile["country_code"] = ""
		case ok:
			name := place.Name
			if place.Country != "" {
				name += ", " + place.Country
			}
			ch.profile["birth_place_id"], ch.profile["birth_place"], ch.profile["birth_place_label"] = place.ID, name, name
			ch.profile["birth_latitude"], ch.profile["birth_longitude"], ch.profile["birth_timezone"] = place.Latitude, place.Longitude, place.TimeZone
			ch.profile["country_code"] = strings.ToUpper(place.CountryCode)
		case geoNamesID.MatchString(id):
			// A place from the GeoNames search the gazetteer cannot resolve
			// yet: keep the id (coordinates follow once it resolves).
			ch.profile["birth_place_id"], ch.profile["birth_place"] = id, ""
			ch.profile["birth_latitude"], ch.profile["birth_longitude"], ch.profile["birth_timezone"] = nil, nil, ""
			ch.profile["country_code"] = ""
			if label == nil {
				ch.profile["birth_place_label"] = ""
			}
		default:
			fields[v.key("profile", "birth_place_id")] = "Choose a birth place from the list."
		}
	}
	if label != nil {
		if _, bad := fields[v.key("profile", "birth_place_id")]; !bad {
			ch.profile["birth_place_label"] = *label
			if p.BirthPlaceID != nil && geoNamesID.MatchString(strings.TrimSpace(*p.BirthPlaceID)) {
				if _, resolved := s.lookupPlace(strings.TrimSpace(*p.BirthPlaceID)); !resolved {
					ch.profile["birth_place"] = *label
				}
			}
		}
	}
	if p.Location != nil {
		if l := CleanLine(*p.Location); len([]rune(l)) > 60 {
			fields[v.key("profile", "location")] = "Use at most 60 characters."
		} else {
			ch.profile["location"] = l
		}
	}
	if p.Mood != nil {
		if !allowedMoods[*p.Mood] {
			fields[v.key("profile", "mood")] = "Choose a mood from the list."
		} else {
			ch.profile["mood"] = *p.Mood
		}
	}
	if p.HouseSystem != nil {
		if h := domain.HouseSystem(*p.HouseSystem); !h.Valid() {
			fields[v.key("profile", "house_system")] = "Choose a house system from the list."
		} else {
			ch.profile["house_system"] = h
		}
	}
}

// Settings returns the settings document of a user.
func (s *AccountService) Settings(ctx context.Context, userID uuid.UUID) (*SettingsView, error) {
	u, err := s.d.Users.GetByID(ctx, userID)
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	p, err := s.d.Users.GetProfile(ctx, userID)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, apperror.Internal(err)
		}
		p = newProfile(userID, nil, s.d.Clock.Now())
		p.NotifyEmail = true
	}
	version, err := s.d.Accounts.SettingsVersion(ctx, userID, false)
	if errors.Is(err, repository.ErrNotFound) {
		version, err = 1, nil
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	consents := map[string]bool{}
	if s.d.Consents != nil {
		eff, err := s.d.Consents.Effective(ctx, userID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for c, g := range eff {
			consents[string(c)] = g
		}
	}
	view := &SettingsView{
		Version: version,
		Account: AccountSettingsView{Email: u.Email, EmailVerified: u.EmailVerifiedAt != nil, DisplayName: u.DisplayName,
			Role: string(u.Role), CreatedAt: u.CreatedAt, PendingEmail: s.pendingEmail(ctx, userID)},
		Profile: ProfileSettingsView{Nickname: p.Nickname, BirthTime: p.BirthTime, BirthPlaceID: p.BirthPlaceID,
			BirthPlace: p.BirthPlace, BirthLatitude: p.BirthLatitude, BirthLongitude: p.BirthLongitude, BirthTimezone: p.BirthTimezone,
			CountryCode: p.CountryCode, Location: p.Location, Mood: p.Mood, HouseSystem: string(p.HouseSystem), AvatarURL: p.AvatarURL},
		Appearance: AppearanceSettings{Theme: p.Theme, Locale: u.Locale, TimeZone: p.Timezone},
		Notifications: NotificationSettings{Email: p.NotifyEmail, Push: p.NotifyPush, Marketing: p.NotifyMarketing,
			Horoscope: p.NotifyHoroscope, Events: p.NotifyEvents, Chart: p.NotifyChart},
		Privacy: PrivacySettingsView{ShareBirthDetailsWithAdvisors: p.ShareBirthDetailsWithAdvisors, Consents: consents,
			PolicyVersion: ConsentPolicyVersion},
	}
	if p.BirthPlaceLabel != "" {
		view.Profile.BirthPlace = p.BirthPlaceLabel
	}
	if view.Profile.HouseSystem == "" {
		view.Profile.HouseSystem = string(domain.HousePlacidus)
	}
	if p.BirthDate != nil {
		view.Profile.BirthDate = p.BirthDate.Format("2006-01-02")
	}
	return view, nil
}

// pendingEmail returns the masked address of an e-mail change awaiting
// confirmation ("" when none).
func (s *AccountService) pendingEmail(ctx context.Context, userID uuid.UUID) string {
	if s.d.Actions == nil || s.d.Sealer == nil {
		return ""
	}
	t, err := s.d.Actions.LatestOpen(ctx, userID, domain.ActionEmailChange, s.d.Clock.Now())
	if err != nil {
		return ""
	}
	var pl emailChangePayload
	if err := s.openPayload(t, &pl); err != nil {
		return ""
	}
	return maskEmail(pl.New)
}

// UpdateSettings validates a patch and saves it in one transaction. The
// version is checked under a row lock so concurrent saves cannot silently
// overwrite each other.
func (s *AccountService) UpdateSettings(ctx context.Context, userID uuid.UUID, in SettingsPatch) (*SettingsView, error) {
	ch, fields := s.validateSettings(in, settingsPaths)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if err := s.save(ctx, userID, in.ExpectedVersion, ch); err != nil {
		return nil, err
	}
	return s.Settings(ctx, userID)
}

// UpdateProfile is the PATCH /me/profile alias: the legacy body and field
// names, saved through the same transactional path as the settings.
func (s *AccountService) UpdateProfile(ctx context.Context, userID uuid.UUID, in ProfileUpdate) (*UserView, error) {
	patch := SettingsPatch{
		Profile: &ProfileSettingsPatch{Nickname: in.Nickname, BirthDate: in.BirthDate, BirthTime: in.BirthTime,
			BirthPlaceID: in.BirthPlaceID, Location: in.Location, Mood: in.Mood},
		Appearance:    &AppearancePatch{Theme: in.Theme, Locale: in.Locale},
		Notifications: &NotificationsPatch{Horoscope: in.NotifyHoroscope, Events: in.NotifyEvents, Chart: in.NotifyChart},
	}
	if in.DisplayName != nil {
		patch.Account = &AccountSettingsPatch{DisplayName: in.DisplayName}
	}
	ch, fields := s.validateSettings(patch, profilePaths)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if err := s.save(ctx, userID, nil, ch); err != nil {
		return nil, err
	}
	return s.d.UserSvc.Me(ctx, userID)
}

func (s *AccountService) save(ctx context.Context, userID uuid.UUID, expected *int, ch *settingsChange) error {
	if ch.empty() {
		if expected != nil {
			v, err := s.d.Accounts.SettingsVersion(ctx, userID, false)
			if err == nil && v != *expected {
				return apperror.VersionConflict(v)
			}
		}
		return nil
	}
	var changedConsents []string
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		version, err := s.d.Accounts.SettingsVersion(ctx, userID, true)
		if errors.Is(err, repository.ErrNotFound) {
			p := newProfile(userID, nil, s.d.Clock.Now())
			p.NotifyEmail = true
			if err := s.d.Users.CreateProfile(ctx, p); err != nil {
				return err
			}
			version, err = 1, nil
		}
		if err != nil {
			return err
		}
		if expected != nil && *expected != version {
			return apperror.VersionConflict(version)
		}
		if len(ch.user) > 0 {
			if err := s.d.Users.Updates(ctx, userID, ch.user); err != nil {
				return err
			}
		}
		ch.profile["settings_version"] = version + 1
		if err := s.d.Users.UpdateProfile(ctx, userID, ch.profile); err != nil {
			return err
		}
		if len(ch.consents) > 0 && s.d.Consents != nil {
			changedConsents, err = s.d.Consents.apply(ctx, userID, ch.consents, "settings")
			return err
		}
		return nil
	})
	if err != nil {
		if _, ok := apperror.As(err); ok {
			return err
		}
		return mapRepoErr(err, "Profile")
	}
	if len(changedConsents) > 0 {
		s.d.Consents.record(ctx, userID, ch.consents, changedConsents, "settings")
	}
	keys := make([]string, 0, len(ch.user)+len(ch.profile))
	for k := range ch.user {
		keys = append(keys, k)
	}
	for k := range ch.profile {
		if k != "settings_version" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	s.d.Tracker.Track(ctx, "profile.updated", map[string]any{"fields": strings.Join(keys, ","), "count": len(keys),
		"sections": strings.Join(ch.sections, ",")})
	return nil
}
