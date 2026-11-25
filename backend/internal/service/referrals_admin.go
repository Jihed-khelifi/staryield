package service

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// Growth: the referral loop's admin console. The console is the only place
// that shows who invited whom.

const (
	msgReferralStatus     = "Choose joined, qualified, granted or rejected."
	msgReferralFlagged    = "Use true or false."
	msgReferralReason     = "Give a reason of 3 to 200 characters."
	msgReferralNotHeld    = "Only a referral held for review can be approved."
	msgReferralNotOpen    = "Only a referral that is joined or qualified can be rejected."
	msgReferralAdminFound = "Referral"
	referralTopInviters   = 10
)

// ReferralParty is a person in the admin list.
type ReferralParty struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// AdminReferral is one referral as the console shows it.
type AdminReferral struct {
	ID            uuid.UUID      `json:"id"`
	Referrer      ReferralParty  `json:"referrer"`
	Referee       *ReferralParty `json:"referee,omitempty"` // absent once the friend's account was erased
	Status        string         `json:"status"`
	RewardMinutes int            `json:"reward_minutes"`
	Flagged       bool           `json:"flagged"`
	Flags         []string       `json:"flags"`
	RejectReason  string         `json:"reject_reason,omitempty"`
	JoinedAt      time.Time      `json:"joined_at"`
	QualifiedAt   *time.Time     `json:"qualified_at,omitempty"`
	GrantedAt     *time.Time     `json:"granted_at,omitempty"`
	DecidedBy     *ReferralParty `json:"decided_by,omitempty"`
	DecidedAt     *time.Time     `json:"decided_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

// views resolves the names of a page of referrals with one query.
func (s *ReferralService) views(ctx context.Context, rows []domain.Referral) ([]AdminReferral, error) {
	ids := make([]uuid.UUID, 0, len(rows)*2)
	for i := range rows {
		ids = append(ids, rows[i].ReferrerID)
		if rows[i].RefereeID != nil {
			ids = append(ids, *rows[i].RefereeID)
		}
		if rows[i].DecidedBy != nil {
			ids = append(ids, *rows[i].DecidedBy)
		}
	}
	names, err := s.d.Repo.Names(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]AdminReferral, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		v := AdminReferral{ID: r.ID, Referrer: ReferralParty{ID: r.ReferrerID, Name: names[r.ReferrerID]}, Status: string(r.Status),
			RewardMinutes: r.RewardMinutes, Flagged: r.Flagged, Flags: append([]string{}, r.Flags...), RejectReason: r.RejectReason,
			JoinedAt: r.JoinedAt, QualifiedAt: r.QualifiedAt, GrantedAt: r.GrantedAt, DecidedAt: r.DecidedAt, CreatedAt: r.CreatedAt}
		if r.RefereeID != nil {
			v.Referee = &ReferralParty{ID: *r.RefereeID, Name: names[*r.RefereeID]}
		}
		if r.DecidedBy != nil {
			v.DecidedBy = &ReferralParty{ID: *r.DecidedBy, Name: names[*r.DecidedBy]}
		}
		out = append(out, v)
	}
	return out, nil
}

// List returns one page of referrals, newest first, optionally filtered by
// status and by whether they are held for review.
func (s *ReferralService) List(ctx context.Context, status string, flagged *bool, cursor string, limit int) (Page[AdminReferral], error) {
	fields := map[string]string{}
	if status != "" && !domain.ReferralStatus(status).Valid() {
		fields["status"] = msgReferralStatus
	}
	if len(fields) > 0 {
		return Page[AdminReferral]{}, apperror.Validation(fields)
	}
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[AdminReferral]{}, err
	}
	limit = min(max(limit, 1), 100)
	rows, err := s.d.Repo.List(ctx, repository.ReferralFilter{Status: domain.ReferralStatus(status), Flagged: flagged, Cursor: c, Limit: limit + 1})
	if err != nil {
		return Page[AdminReferral]{}, apperror.Internal(err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	items, err := s.views(ctx, rows)
	if err != nil {
		return Page[AdminReferral]{}, err
	}
	return Page[AdminReferral]{Items: items, NextCursor: next}, nil
}

// ReferralTopInviter is one of the inviters with the most rewarded friends.
type ReferralTopInviter struct {
	User      ReferralParty `json:"user"`
	Qualified int64         `json:"qualified"`
	Minutes   int64         `json:"minutes"`
}

// ReferralSummary is GET /admin/referrals/summary.
type ReferralSummary struct {
	Program ReferralProgram `json:"program"`
	// Counts by status.
	Joined    int64 `json:"joined"`
	Qualified int64 `json:"qualified"`
	Granted   int64 `json:"granted"`
	Rejected  int64 `json:"rejected"`
	// Flagged counts referrals held for review (joined or qualified and flagged).
	Flagged        int64 `json:"flagged"`
	PendingMinutes int64 `json:"pending_minutes"`
	GrantedMinutes int64 `json:"granted_minutes"`
	// ReadyRewards and ReadyMinutes are what the launch (or the next run of the
	// grant job) will add: qualified, not held for review.
	ReadyRewards int64                `json:"ready_rewards"`
	ReadyMinutes int64                `json:"ready_minutes"`
	TopInviters  []ReferralTopInviter `json:"top_inviters"`
}

// Summary returns the counters of the program.
func (s *ReferralService) Summary(ctx context.Context) (*ReferralSummary, error) {
	totals, err := s.d.Repo.Totals(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := &ReferralSummary{Program: s.d.Program(ctx), TopInviters: []ReferralTopInviter{}}
	for _, t := range totals {
		switch t.Status {
		case domain.ReferralJoined:
			out.Joined = t.Count
			out.Flagged += t.Flagged
		case domain.ReferralQualified:
			out.Qualified, out.PendingMinutes = t.Count, t.Minutes
			out.Flagged += t.Flagged
		case domain.ReferralGranted:
			out.Granted, out.GrantedMinutes = t.Count, t.Minutes
		case domain.ReferralRejected:
			out.Rejected = t.Count
		}
	}
	if out.ReadyRewards, out.ReadyMinutes, err = s.d.Repo.PendingGrants(ctx); err != nil {
		return nil, apperror.Internal(err)
	}
	top, err := s.d.Repo.TopInviters(ctx, referralTopInviters)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	ids := make([]uuid.UUID, 0, len(top))
	for _, t := range top {
		ids = append(ids, t.ReferrerID)
	}
	names, err := s.d.Repo.Names(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, t := range top {
		out.TopInviters = append(out.TopInviters, ReferralTopInviter{User: ReferralParty{ID: t.ReferrerID, Name: names[t.ReferrerID]},
			Qualified: t.Qualified, Minutes: t.Minutes})
	}
	return out, nil
}

// decide runs fn on the locked referral in one transaction and returns it.
func (s *ReferralService) decide(ctx context.Context, id uuid.UUID, fn func(r *domain.Referral, now time.Time) error) (*domain.Referral, error) {
	var out *domain.Referral
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		r, err := s.d.Repo.ForUpdate(ctx, id)
		if err != nil {
			return err
		}
		now := s.d.Clock.Now()
		if err := fn(r, now); err != nil {
			return err
		}
		admin := actor(ctx)
		r.DecidedBy, r.DecidedAt = admin, &now
		out = r
		return s.d.Repo.Save(ctx, r)
	})
	if err != nil {
		return nil, mapRepoErr(err, msgReferralAdminFound)
	}
	return out, nil
}

func (s *ReferralService) view(ctx context.Context, r *domain.Referral) (*AdminReferral, error) {
	v, err := s.views(ctx, []domain.Referral{*r})
	if err != nil {
		return nil, err
	}
	return &v[0], nil
}

// Approve releases a referral held for review: a qualified one is granted at
// the next run (or announced as pending before launch), a joined one is
// rewarded automatically once it qualifies.
func (s *ReferralService) Approve(ctx context.Context, id uuid.UUID) (*AdminReferral, error) {
	r, err := s.decide(ctx, id, func(r *domain.Referral, _ time.Time) error {
		if !r.Flagged || (r.Status != domain.ReferralJoined && r.Status != domain.ReferralQualified) {
			return apperror.InvalidState(msgReferralNotHeld)
		}
		r.Flagged = false
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.record(ctx, "referral.approved", "referral", r.ID.String(), map[string]any{"status": string(r.Status), "referrer_id": r.ReferrerID})
	if r.Status == domain.ReferralQualified {
		s.afterQualified(ctx, r, s.d.Program(ctx))
	}
	return s.view(ctx, r)
}

// Reject closes a joined or qualified referral with the administrator's reason.
func (s *ReferralService) Reject(ctx context.Context, id uuid.UUID, reason string) (*AdminReferral, error) {
	reason = CleanLine(reason)
	if n := utf8.RuneCountInString(reason); n < 3 || n > domain.ReferralRejectReasonMaxLen {
		return nil, apperror.Validation(map[string]string{"reason": msgReferralReason})
	}
	r, err := s.decide(ctx, id, func(r *domain.Referral, _ time.Time) error {
		if r.Status != domain.ReferralJoined && r.Status != domain.ReferralQualified {
			return apperror.InvalidState(msgReferralNotOpen)
		}
		r.Status, r.RejectReason = domain.ReferralRejected, reason
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.record(ctx, "referral.rejected", "referral", r.ID.String(), map[string]any{"referrer_id": r.ReferrerID, "reason": reason})
	return s.view(ctx, r)
}

// ReferralLaunchResult is POST /admin/referrals/launch.
type ReferralLaunchResult struct {
	LaunchedAt time.Time `json:"launched_at"`
	// Rewards and Minutes are what the grant job is about to add (qualified,
	// not held for review); the rest follows as friends qualify.
	Rewards int64 `json:"rewards"`
	Minutes int64 `json:"minutes"`
}

// Launch stamps the launch date once (a typed confirmation is enforced; a
// second launch is a 409) and wakes the grant job.
func (s *ReferralService) Launch(ctx context.Context, confirmation string) (*ReferralLaunchResult, error) {
	set, err := s.d.Launch(ctx, confirmation)
	if err != nil {
		return nil, err
	}
	s.signal()
	rewards, minutes, err := s.d.Repo.PendingGrants(ctx)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		warnErr(ctx, "referrals: count the rewards of the launch", err)
	}
	out := &ReferralLaunchResult{Rewards: rewards, Minutes: minutes}
	if set.LaunchedAt != nil {
		out.LaunchedAt = *set.LaunchedAt
	}
	return out, nil
}
