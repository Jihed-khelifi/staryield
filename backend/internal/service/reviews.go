package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/analytics"
	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// ReviewAuthor is the public identity of a reviewer (first name only).
type ReviewAuthor struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// ReviewView is a rating as shown publicly and to its participants.
type ReviewView struct {
	ID        uuid.UUID    `json:"id"`
	SessionID *uuid.UUID   `json:"session_id,omitempty"`
	Rating    int          `json:"rating"`
	Comment   string       `json:"comment"`
	Status    string       `json:"status,omitempty"`
	Author    ReviewAuthor `json:"author"`
	CreatedAt time.Time    `json:"created_at"`
}

// AdminReviewView adds moderation context.
type AdminReviewView struct {
	ReviewView
	Client         ParticipantRef `json:"client"`
	Psychic        ParticipantRef `json:"psychic"`
	ModerationNote string         `json:"moderation_note,omitempty"`
}

// ParticipantRef names a user in admin views.
type ParticipantRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug,omitempty"`
}

// ReviewService lets clients rate advisors after a session.
type ReviewService struct {
	tx       database.TxManager
	reviews  repository.ReviewRepository
	chat     repository.ChatRepository
	users    repository.UserRepository
	psychics repository.PsychicRepository
	audit    repository.AuditRepository
	notify   Notifier
	tracker  analytics.Tracker
	clock    clock.Clock
	window   time.Duration
	ipSecret string
}

// NewReviewService wires the review service.
func NewReviewService(tx database.TxManager, reviews repository.ReviewRepository, chat repository.ChatRepository, users repository.UserRepository,
	psychics repository.PsychicRepository, audit repository.AuditRepository, notify Notifier, tracker analytics.Tracker, clk clock.Clock,
	window time.Duration, ipSecret string) *ReviewService {
	if notify == nil {
		notify = NoopNotifier{}
	}
	return &ReviewService{tx: tx, reviews: reviews, chat: chat, users: users, psychics: psychics, audit: audit, notify: notify,
		tracker: tracker, clock: clk, window: window, ipSecret: ipSecret}
}

// firstName returns a reviewer's public name: nickname, else the first word
// of the display name (never the e-mail address).
func firstName(nickname, display string) string {
	if n := strings.TrimSpace(nickname); n != "" {
		return truncateString(n, 40)
	}
	parts := strings.Fields(display)
	if len(parts) == 0 {
		return "Client"
	}
	return truncateString(parts[0], 40)
}

func (s *ReviewService) authors(ctx context.Context, rows []domain.Review) map[uuid.UUID]ReviewAuthor {
	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ClientID)
	}
	out := map[uuid.UUID]ReviewAuthor{}
	users, _ := s.users.GetByIDs(ctx, ids)
	profiles, _ := s.users.GetProfiles(ctx, ids)
	for _, u := range users {
		p := profiles[u.ID]
		name := firstName(p.Nickname, u.DisplayName)
		if u.Status == domain.UserDeleted {
			name = "Client"
		}
		out[u.ID] = ReviewAuthor{Name: name, AvatarURL: p.AvatarURL}
	}
	return out
}

func reviewView(r *domain.Review, a ReviewAuthor, withSession bool) ReviewView {
	v := ReviewView{ID: r.ID, Rating: r.Rating, Comment: r.Comment, Author: a, CreatedAt: r.CreatedAt}
	if withSession {
		id := r.SessionID
		v.SessionID = &id
		v.Status = string(r.Status)
	}
	return v
}

// Eligibility explains whether a client may rate a session.
func (s *ReviewService) eligible(sess *domain.Session, clientID uuid.UUID, now time.Time) error {
	if sess.ClientID != clientID {
		return apperror.NotFound("Session")
	}
	if sess.StartedAt == nil || (sess.Status != domain.SessionEnded && sess.Status != domain.SessionSettled) {
		return apperror.InvalidState("You can rate a reading once it has ended.")
	}
	if sess.EndedAt != nil && now.Sub(*sess.EndedAt) > s.window {
		return apperror.InvalidState("The rating period for this reading has closed.")
	}
	return nil
}

// CanReview reports whether the client may still rate the session.
func (s *ReviewService) CanReview(ctx context.Context, sess *domain.Session, clientID uuid.UUID) (bool, *ReviewView) {
	if r, err := s.reviews.GetBySession(ctx, sess.ID); err == nil {
		v := reviewView(r, s.authors(ctx, []domain.Review{*r})[r.ClientID], true)
		return false, &v
	}
	return s.eligible(sess, clientID, s.clock.Now()) == nil, nil
}

// Submit records a client's rating of a finished session.
func (s *ReviewService) Submit(ctx context.Context, clientID, sessionID uuid.UUID, rating int, comment string) (*ReviewView, error) {
	fields := map[string]string{}
	if rating < 1 || rating > 5 {
		fields["rating"] = "Choose between 1 and 5 stars."
	}
	comment = CleanText(comment)
	if utf8.RuneCountInString(comment) > 1000 {
		fields["comment"] = "Use at most 1000 characters."
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	sess, err := s.chat.GetSession(ctx, sessionID)
	if err != nil {
		return nil, mapRepoErr(err, "Session")
	}
	now := s.clock.Now()
	if err := s.eligible(sess, clientID, now); err != nil {
		return nil, err
	}
	r := &domain.Review{ID: domain.NewID(), SessionID: sess.ID, ClientID: clientID, PsychicID: sess.PsychicID, Rating: rating,
		Comment: comment, Status: domain.ReviewVisible, CreatedAt: now, UpdatedAt: now}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.reviews.Create(ctx, r); err != nil {
			return err
		}
		return s.reviews.AdjustPsychicRating(ctx, sess.PsychicID, float64(rating), 1)
	})
	if errors.Is(err, repository.ErrDuplicate) {
		return nil, apperror.Conflict(apperror.CodeConflict, "You have already rated this reading.")
	}
	if err != nil {
		return nil, mapRepoErr(err, "Review")
	}
	s.tracker.Track(ctx, "review.submitted", map[string]any{"session_id": sess.ID, "psychic_id": sess.PsychicID, "rating": rating,
		"has_comment": comment != ""})
	s.notify.Notify(ctx, NotificationSpec{Kind: NotifyReviewReceived, Data: map[string]any{"review_id": r.ID, "rating": rating},
		Link: "/profile/analytics"}, sess.PsychicID)
	v := reviewView(r, s.authors(ctx, []domain.Review{*r})[clientID], true)
	return &v, nil
}

func (s *ReviewService) page(ctx context.Context, f repository.ReviewFilter, cursor string, withSession bool) (Page[ReviewView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[ReviewView]{}, err
	}
	f.Cursor = c
	rows, err := s.reviews.List(ctx, f)
	if err != nil {
		return Page[ReviewView]{}, apperror.Internal(err)
	}
	authors := s.authors(ctx, rows)
	page := Page[ReviewView]{Items: make([]ReviewView, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, reviewView(&rows[i], authors[rows[i].ClientID], withSession))
	}
	if len(rows) > 0 && len(rows) == clampPage(f.Limit, 20, 200) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

// Public lists an advisor's visible reviews by slug.
func (s *ReviewService) Public(ctx context.Context, slug, cursor string, limit int) (Page[ReviewView], error) {
	p, err := s.psychics.GetBySlug(ctx, slug)
	if err != nil {
		return Page[ReviewView]{}, mapRepoErr(err, "Advisor")
	}
	if ok, err := s.psychics.IsPublic(ctx, p.UserID); err != nil || !ok {
		return Page[ReviewView]{}, apperror.NotFound("Advisor")
	}
	return s.page(ctx, repository.ReviewFilter{PsychicID: &p.UserID, Status: domain.ReviewVisible, Limit: clampPage(limit, 10, 50)}, cursor, false)
}

// ForPsychic lists the caller's own visible reviews.
func (s *ReviewService) ForPsychic(ctx context.Context, psychicID uuid.UUID, cursor string, limit int) (Page[ReviewView], error) {
	return s.page(ctx, repository.ReviewFilter{PsychicID: &psychicID, Status: domain.ReviewVisible, Limit: clampPage(limit, 20, 100)}, cursor, true)
}

// AdminList lists reviews for moderation.
func (s *ReviewService) AdminList(ctx context.Context, f repository.ReviewFilter, cursor string) (Page[AdminReviewView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[AdminReviewView]{}, err
	}
	if f.Status != "" && f.Status != domain.ReviewVisible && f.Status != domain.ReviewHidden {
		return Page[AdminReviewView]{}, apperror.Validation(map[string]string{"status": "Use visible or hidden."})
	}
	f.Cursor = c
	f.Limit = clampPage(f.Limit, 50, 200)
	rows, err := s.reviews.List(ctx, f)
	if err != nil {
		return Page[AdminReviewView]{}, apperror.Internal(err)
	}
	page := Page[AdminReviewView]{Items: make([]AdminReviewView, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, s.adminView(ctx, &rows[i]))
	}
	if len(rows) > 0 && len(rows) == f.Limit {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

func (s *ReviewService) adminView(ctx context.Context, r *domain.Review) AdminReviewView {
	authors := s.authors(ctx, []domain.Review{*r})
	v := AdminReviewView{ReviewView: reviewView(r, authors[r.ClientID], true), ModerationNote: r.ModerationNote,
		Client: ParticipantRef{ID: r.ClientID, Name: authors[r.ClientID].Name}, Psychic: ParticipantRef{ID: r.PsychicID}}
	if u, err := s.users.GetByID(ctx, r.ClientID); err == nil {
		v.Client.Name = u.DisplayName
	}
	if p, err := s.psychics.GetByUserID(ctx, r.PsychicID); err == nil {
		v.Psychic.Name, v.Psychic.Slug = p.Name, p.Slug
	}
	return v
}

// Moderate hides or restores a review; the advisor's rating is adjusted.
func (s *ReviewService) Moderate(ctx context.Context, id uuid.UUID, status domain.ReviewStatus, note string) (*AdminReviewView, error) {
	if status != domain.ReviewVisible && status != domain.ReviewHidden {
		return nil, apperror.Validation(map[string]string{"status": "Use visible or hidden."})
	}
	note = CleanLine(note)
	if utf8.RuneCountInString(note) > 300 {
		return nil, apperror.Validation(map[string]string{"note": "Use at most 300 characters."})
	}
	var r *domain.Review
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if r, err = s.reviews.GetForUpdate(ctx, id); err != nil {
			return err
		}
		if r.Status == status {
			return nil
		}
		if err := s.reviews.Update(ctx, id, map[string]any{"status": status, "moderation_note": note}); err != nil {
			return err
		}
		delta := float64(r.Rating)
		count := 1
		if status == domain.ReviewHidden {
			delta, count = -delta, -1
		}
		r.Status, r.ModerationNote = status, note
		return s.reviews.AdjustPsychicRating(ctx, r.PsychicID, delta, count)
	})
	if err != nil {
		return nil, mapRepoErr(err, "Review")
	}
	recordAudit(ctx, s.audit, s.clock, s.ipSecret, "review."+string(status), "review", id.String(), map[string]any{"note": note, "rating": r.Rating})
	v := s.adminView(ctx, r)
	return &v, nil
}
