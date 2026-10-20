package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png" // register decoders
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // register decoder

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/logger"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
	"github.com/Jihed-khelifi/staryield/backend/internal/storage"
)

// PlaceInfo is a resolved birth place.
type PlaceInfo struct {
	ID        string
	Name      string
	Country   string
	Latitude  float64
	Longitude float64
	TimeZone  string
}

// PlaceLookup resolves a gazetteer place ID.
type PlaceLookup func(id string) (PlaceInfo, bool)

// ProfileView is the editable personal profile.
type ProfileView struct {
	Nickname        string   `json:"nickname"`
	BirthDate       string   `json:"birth_date"`
	BirthTime       string   `json:"birth_time"`
	BirthPlaceID    string   `json:"birth_place_id"`
	BirthPlace      string   `json:"birth_place"`
	BirthLatitude   *float64 `json:"birth_latitude"`
	BirthLongitude  *float64 `json:"birth_longitude"`
	BirthTimezone   string   `json:"birth_timezone"`
	Location        string   `json:"location"`
	Mood            string   `json:"mood"`
	Theme           string   `json:"theme"`
	AvatarURL       string   `json:"avatar_url"`
	NotifyHoroscope bool     `json:"notify_horoscope"`
	NotifyEvents    bool     `json:"notify_events"`
	NotifyChart     bool     `json:"notify_chart"`
}

// PsychicSelfView is the psychic's own advisor settings.
type PsychicSelfView struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	ImageURL      string `json:"image_url"`
	Availability  string `json:"availability"`
	RatePerMinute int64  `json:"rate_per_minute"`
	Listed        bool   `json:"listed"`
}

// UserView is the signed-in user ("/me").
type UserView struct {
	ID          uuid.UUID        `json:"id"`
	Email       string           `json:"email"`
	Role        string           `json:"role"`
	Status      string           `json:"status"`
	DisplayName string           `json:"display_name"`
	Locale      string           `json:"locale"`
	CreatedAt   time.Time        `json:"created_at"`
	Profile     ProfileView      `json:"profile"`
	Wallet      *WalletView      `json:"wallet,omitempty"`
	Psychic     *PsychicSelfView `json:"psychic,omitempty"`
}

// ProfileUpdate is a partial profile update (nil fields are unchanged).
type ProfileUpdate struct {
	DisplayName     *string `json:"display_name"`
	Nickname        *string `json:"nickname"`
	BirthDate       *string `json:"birth_date"`
	BirthTime       *string `json:"birth_time"`
	BirthPlaceID    *string `json:"birth_place_id"`
	Location        *string `json:"location"`
	Mood            *string `json:"mood"`
	Theme           *string `json:"theme"`
	Locale          *string `json:"locale"`
	NotifyHoroscope *bool   `json:"notify_horoscope"`
	NotifyEvents    *bool   `json:"notify_events"`
	NotifyChart     *bool   `json:"notify_chart"`
}

// Allowed enumerated profile values (mirroring the settings form).
var (
	allowedMoods  = map[string]bool{"Empowered": true, "Hopeful": true, "Reflective": true, "Calm": true, "Curious": true}
	allowedThemes = map[string]bool{"Warm Beige": true, "Light Cream": true}
)

// AvatarSettings controls avatar processing.
type AvatarSettings struct {
	MaxBytes  int64
	MaxPixels int
	Size      int
}

// UserService manages the signed-in user's account and profile.
type UserService struct {
	users      repository.UserRepository
	wallets    repository.WalletRepository
	psychics   repository.PsychicRepository
	store      storage.ObjectStore
	places     PlaceLookup
	tracker    analytics.Tracker
	clock      clock.Clock
	minimumAge int
	avatar     AvatarSettings
}

// NewUserService wires the user service.
func NewUserService(users repository.UserRepository, wallets repository.WalletRepository, psychics repository.PsychicRepository,
	store storage.ObjectStore, places PlaceLookup, tracker analytics.Tracker, clk clock.Clock, minimumAge int, avatar AvatarSettings) *UserService {
	return &UserService{users: users, wallets: wallets, psychics: psychics, store: store, places: places,
		tracker: tracker, clock: clk, minimumAge: minimumAge, avatar: avatar}
}

func profileView(p *domain.Profile) ProfileView {
	v := ProfileView{
		Nickname: p.Nickname, BirthTime: p.BirthTime, BirthPlaceID: p.BirthPlaceID, BirthPlace: p.BirthPlace,
		BirthLatitude: p.BirthLatitude, BirthLongitude: p.BirthLongitude, BirthTimezone: p.BirthTimezone,
		Location: p.Location, Mood: p.Mood, Theme: p.Theme, AvatarURL: p.AvatarURL,
		NotifyHoroscope: p.NotifyHoroscope, NotifyEvents: p.NotifyEvents, NotifyChart: p.NotifyChart,
	}
	if p.BirthDate != nil {
		v.BirthDate = p.BirthDate.Format("2006-01-02")
	}
	return v
}

// Me returns the signed-in user's account, profile and wallet.
func (s *UserService) Me(ctx context.Context, userID uuid.UUID) (*UserView, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, mapRepoErr(err, "User")
	}
	p, err := s.users.GetProfile(ctx, userID)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, apperror.Internal(err)
		}
		p = &domain.Profile{UserID: userID, Mood: "Empowered", Theme: "Warm Beige"}
	}
	view := &UserView{ID: u.ID, Email: u.Email, Role: string(u.Role), Status: string(u.Status),
		DisplayName: u.DisplayName, Locale: u.Locale, CreatedAt: u.CreatedAt, Profile: profileView(p)}
	if w, err := s.wallets.Get(ctx, userID); err == nil {
		wv := walletView(w)
		view.Wallet = &wv
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.Internal(err)
	}
	if u.Role == domain.RolePsychic {
		if ps, err := s.psychics.GetByUserID(ctx, userID); err == nil {
			view.Psychic = &PsychicSelfView{Slug: ps.Slug, Name: ps.Name, ImageURL: ps.ImageURL,
				Availability: string(ps.Availability), RatePerMinute: ps.RatePerMinute, Listed: ps.Listed}
		}
	}
	return view, nil
}

// UpdateProfile applies a validated partial update (FR-08).
func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, in ProfileUpdate) (*UserView, error) {
	fields := map[string]string{}
	profile := map[string]any{}
	user := map[string]any{}

	if in.DisplayName != nil {
		v := CleanLine(*in.DisplayName)
		if v == "" || len([]rune(v)) > 80 {
			fields["display_name"] = "Use between 1 and 80 characters."
		} else {
			user["display_name"] = v
		}
	}
	if in.Locale != nil {
		user["locale"] = normalizeLocale(*in.Locale)
	}
	if in.Nickname != nil {
		v := CleanLine(*in.Nickname)
		if len([]rune(v)) > 40 {
			fields["nickname"] = "Use at most 40 characters."
		} else {
			profile["nickname"] = v
		}
	}
	if in.BirthDate != nil {
		if strings.TrimSpace(*in.BirthDate) == "" {
			profile["birth_date"] = nil
		} else if d, msg := ParseBirthDate(*in.BirthDate, s.clock.Now(), s.minimumAge); msg != "" {
			fields["birth_date"] = msg
		} else {
			profile["birth_date"] = *d
		}
	}
	if in.BirthTime != nil {
		v := strings.TrimSpace(*in.BirthTime)
		if v != "" {
			if _, err := time.Parse("15:04", v); err != nil || len(v) != 5 {
				fields["birth_time"] = "Use the 24-hour format HH:MM."
			}
		}
		if _, bad := fields["birth_time"]; !bad {
			profile["birth_time"] = v
		}
	}
	if in.BirthPlaceID != nil {
		id := strings.TrimSpace(*in.BirthPlaceID)
		if id == "" {
			profile["birth_place_id"], profile["birth_place"] = "", ""
			profile["birth_latitude"], profile["birth_longitude"], profile["birth_timezone"] = nil, nil, ""
		} else if place, ok := s.lookup(id); !ok {
			fields["birth_place_id"] = "Choose a birth place from the list."
		} else {
			profile["birth_place_id"], profile["birth_place"] = place.ID, place.Name+", "+place.Country
			profile["birth_latitude"], profile["birth_longitude"], profile["birth_timezone"] = place.Latitude, place.Longitude, place.TimeZone
		}
	}
	if in.Location != nil {
		v := CleanLine(*in.Location)
		if len([]rune(v)) > 60 {
			fields["location"] = "Use at most 60 characters."
		} else {
			profile["location"] = v
		}
	}
	if in.Mood != nil {
		if !allowedMoods[*in.Mood] {
			fields["mood"] = "Choose a mood from the list."
		} else {
			profile["mood"] = *in.Mood
		}
	}
	if in.Theme != nil {
		if !allowedThemes[*in.Theme] {
			fields["theme"] = "Choose a theme from the list."
		} else {
			profile["theme"] = *in.Theme
		}
	}
	if in.NotifyHoroscope != nil {
		profile["notify_horoscope"] = *in.NotifyHoroscope
	}
	if in.NotifyEvents != nil {
		profile["notify_events"] = *in.NotifyEvents
	}
	if in.NotifyChart != nil {
		profile["notify_chart"] = *in.NotifyChart
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if len(user) > 0 {
		if err := s.users.Updates(ctx, userID, user); err != nil {
			return nil, mapRepoErr(err, "User")
		}
	}
	if len(profile) > 0 {
		if err := s.users.UpdateProfile(ctx, userID, profile); err != nil {
			return nil, mapRepoErr(err, "Profile")
		}
	}
	keys := make([]string, 0, len(profile)+len(user))
	for k := range profile {
		keys = append(keys, k)
	}
	for k := range user {
		keys = append(keys, k)
	}
	s.tracker.Track(ctx, "profile.updated", map[string]any{"fields": strings.Join(keys, ","), "count": len(keys)})
	return s.Me(ctx, userID)
}

func (s *UserService) lookup(id string) (PlaceInfo, bool) {
	if s.places == nil || len(id) > 80 {
		return PlaceInfo{}, false
	}
	return s.places(id)
}

// Supported avatar upload types (sniffed, never trusted from headers).
var avatarTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true}

// UploadAvatar validates, normalises and stores a profile picture. The image
// is decoded and re-encoded as a square JPEG, which strips EXIF/GPS metadata
// and any payload smuggled in the original file.
func (s *UserService) UploadAvatar(ctx context.Context, userID uuid.UUID, data []byte) (*UserView, error) {
	if int64(len(data)) > s.avatar.MaxBytes {
		return nil, apperror.PayloadTooLarge("Choose an image smaller than 1 MB.")
	}
	if len(data) < 64 {
		return nil, apperror.Validation(map[string]string{"avatar": "The file is not a valid image."})
	}
	if ct := http.DetectContentType(data); !avatarTypes[ct] {
		return nil, apperror.UnsupportedMedia("Use a PNG, JPEG or WebP image.")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, apperror.Validation(map[string]string{"avatar": "The file is not a valid image."})
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > s.avatar.MaxPixels || cfg.Width > 8192 || cfg.Height > 8192 {
		return nil, apperror.Validation(map[string]string{"avatar": "The image dimensions are too large."})
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, apperror.Validation(map[string]string{"avatar": "The file is not a valid image."})
	}
	out, err := squareJPEG(img, s.avatar.Size)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	old, err := s.users.GetProfile(ctx, userID)
	if err != nil {
		return nil, mapRepoErr(err, "Profile")
	}
	key := "avatars/" + userID.String() + "/" + security.MustRandomToken(12) + ".jpg"
	url, err := s.store.Put(ctx, key, "image/jpeg", out, "public, max-age=31536000, immutable")
	if err != nil {
		logger.FromContext(ctx).Error("avatar upload failed", zap.Error(err))
		return nil, apperror.Unavailable(err)
	}
	if err := s.users.UpdateProfile(ctx, userID, map[string]any{"avatar_url": url, "avatar_key": key}); err != nil {
		_ = s.store.Delete(ctx, key)
		return nil, mapRepoErr(err, "Profile")
	}
	if old.AvatarKey != "" {
		warnErr(ctx, "avatar: delete previous object", s.store.Delete(ctx, old.AvatarKey))
	}
	s.tracker.Track(ctx, "profile.avatar_uploaded", map[string]any{"bytes": len(out)})
	return s.Me(ctx, userID)
}

// DeleteAvatar removes the profile picture.
func (s *UserService) DeleteAvatar(ctx context.Context, userID uuid.UUID) (*UserView, error) {
	p, err := s.users.GetProfile(ctx, userID)
	if err != nil {
		return nil, mapRepoErr(err, "Profile")
	}
	if err := s.users.UpdateProfile(ctx, userID, map[string]any{"avatar_url": "", "avatar_key": ""}); err != nil {
		return nil, mapRepoErr(err, "Profile")
	}
	if p.AvatarKey != "" {
		warnErr(ctx, "avatar: delete object", s.store.Delete(ctx, p.AvatarKey))
	}
	return s.Me(ctx, userID)
}

// squareJPEG centre-crops img to a square and scales it to size×size.
func squareJPEG(img image.Image, size int) ([]byte, error) {
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	x0 := b.Min.X + (b.Dx()-side)/2
	y0 := b.Min.Y + (b.Dy()-side)/2
	crop := image.Rect(x0, y0, x0+side, y0+side)
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, crop, draw.Src, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
