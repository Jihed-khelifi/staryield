package service

import (
	"context"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// Directory limits.
const (
	catalogTTL         = 10 * time.Second
	browseDefaultSize  = 24
	browseMaxSize      = 48
	featuredDefault    = 3
	featuredMax        = 6
	featuredSkillCount = 3
	// ratingPrior is how many virtual reviews at the platform mean an
	// advisor's rating is blended with, so one 5-star review does not beat
	// hundreds of 4.9s ("most rated" means rated well by many).
	ratingPrior = 20.0
)

// AdvisorCard is an advisor as shown in directory and home cards.
type AdvisorCard struct {
	Slug            string   `json:"slug"`
	Name            string   `json:"name"`
	Headline        string   `json:"headline"`
	ImageURL        string   `json:"image_url"`
	Zodiac          string   `json:"zodiac"`
	Skills          []string `json:"skills"`
	Rating          float64  `json:"rating"`
	ReviewsCount    int      `json:"reviews_count"`
	ReadingsCount   int      `json:"readings_count"`
	YearsExperience int      `json:"years_experience"`
	RatePerMinute   int64    `json:"rate_per_minute"`
	Status          string   `json:"status"`
	Featured        bool     `json:"featured"`
}

// BrowseQuery filters the directory.
type BrowseQuery struct {
	Page     int
	PageSize int
	Status   string
	Search   string
	Skill    string
}

// BrowsePage is one numbered directory page.
type BrowsePage struct {
	Items      []AdvisorCard `json:"items"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	Total      int           `json:"total"`
	TotalPages int           `json:"total_pages"`
}

// FeaturedAdvisor is a home-page card with its call to action: "chat" when
// the advisor can take a session now, otherwise "waitlist".
type FeaturedAdvisor struct {
	AdvisorCard
	CTA string `json:"cta"`
}

// FeaturedList is the home page selection.
type FeaturedList struct {
	Items       []FeaturedAdvisor `json:"items"`
	GeneratedAt time.Time         `json:"generated_at"`
}

type catalogSnap struct {
	cards      []AdvisorCard // curated directory order
	lastOnline []*time.Time  // parallel to cards
	pos        map[uuid.UUID]int
	mean       float64 // mean rating of reviewed advisors
	builtAt    time.Time
	// rank is the featured order, computed once per snapshot on first use
	// (every status patch makes a new snapshot with a fresh cache).
	rank *rankCache
}

type rankCache struct {
	once sync.Once
	idx  []int
}

// AdvisorCatalog serves the public directory and the home selection from a
// short-lived in-memory snapshot, so traffic never fans out into per-request
// presence lookups or open-session scans. One rebuild runs at a time.
// Status changes published on this instance are patched into the snapshot
// in place (no rebuild); profile and listing edits mark it stale; anything
// else (other instances, ratings, expired presence) is picked up within the
// TTL.
type AdvisorCatalog struct {
	repo     repository.CatalogRepository
	statuses StatusLookup
	clock    clock.Clock
	ttl      time.Duration

	snap  atomic.Pointer[catalogSnap]
	stale atomic.Bool
	// gen counts in-place patches, so a rebuild that raced one is redone.
	gen   atomic.Uint64
	group singleflight.Group
}

// NewAdvisorCatalog wires the catalogue.
func NewAdvisorCatalog(repo repository.CatalogRepository, statuses StatusLookup, clk clock.Clock) *AdvisorCatalog {
	return &AdvisorCatalog{repo: repo, statuses: statuses, clock: clk, ttl: catalogTTL}
}

// MarkStale forces the next read to rebuild (directory data changed).
func (c *AdvisorCatalog) MarkStale() { c.stale.Store(true) }

// SetStatus patches one advisor's freshly published status into the
// snapshot without a rebuild. Advisors not in the snapshot (unlisted, or
// created since) are left to MarkStale and the TTL.
func (c *AdvisorCatalog) SetStatus(psychicID uuid.UUID, st domain.PresenceStatus) {
	c.gen.Add(1)
	now := c.clock.Now()
	for {
		cur := c.snap.Load()
		if cur == nil {
			return
		}
		i, ok := cur.pos[psychicID]
		if !ok || cur.cards[i].Status == string(st) {
			return
		}
		next := *cur
		next.cards = slices.Clone(cur.cards)
		next.lastOnline = slices.Clone(cur.lastOnline)
		// Same rule as the presence history: last online is the latest
		// moment the advisor was online or busy.
		if countsAsOnline(domain.PresenceStatus(cur.cards[i].Status)) || countsAsOnline(st) {
			next.lastOnline[i] = &now
		}
		next.cards[i].Status = string(st)
		next.rank = &rankCache{}
		if c.snap.CompareAndSwap(cur, &next) {
			return
		}
	}
}

func (c *AdvisorCatalog) current(ctx context.Context) (*catalogSnap, error) {
	snap := c.snap.Load()
	if snap != nil && !c.stale.Load() && c.clock.Now().Sub(snap.builtAt) < c.ttl {
		return snap, nil
	}
	v, err, _ := c.group.Do("catalog", func() (any, error) {
		// A request that is cancelled must not fail every caller waiting
		// on the shared rebuild.
		bctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return c.build(bctx)
	})
	if err != nil {
		if snap != nil {
			warnErr(ctx, "catalog: rebuild failed, serving previous snapshot", err)
			return snap, nil
		}
		return nil, apperror.Internal(err)
	}
	return v.(*catalogSnap), nil
}

func (c *AdvisorCatalog) build(ctx context.Context) (*catalogSnap, error) {
	// Clear first: a change during the build marks it stale again.
	c.stale.Store(false)
	gen := c.gen.Load()
	rows, err := c.repo.ListedCards(ctx)
	if err != nil {
		c.stale.Store(true)
		return nil, err
	}
	list := make([]domain.Psychic, len(rows))
	for i, r := range rows {
		list[i] = domain.Psychic{UserID: r.UserID, Availability: r.Availability}
	}
	st, err := c.statuses(ctx, list)
	if err != nil {
		c.stale.Store(true)
		return nil, err
	}
	snap := &catalogSnap{cards: make([]AdvisorCard, len(rows)), lastOnline: make([]*time.Time, len(rows)),
		pos: make(map[uuid.UUID]int, len(rows)), builtAt: c.clock.Now(), rank: &rankCache{}}
	var sum float64
	var reviewed int
	for i, r := range rows {
		skills := []string(r.Skills)
		if skills == nil {
			skills = []string{}
		}
		status := st[r.UserID]
		if status == "" {
			status = domain.PresenceOffline
		}
		snap.cards[i] = AdvisorCard{Slug: r.Slug, Name: r.Name, Headline: r.Headline, ImageURL: r.ImageURL, Zodiac: r.Zodiac,
			Skills: skills, Rating: r.Rating, ReviewsCount: r.ReviewsCount, ReadingsCount: r.ReadingsCount,
			YearsExperience: r.YearsExperience, RatePerMinute: r.RatePerMinute, Status: string(status), Featured: r.Featured}
		snap.lastOnline[i] = r.LastOnlineAt
		snap.pos[r.UserID] = i
		if r.ReviewsCount > 0 {
			sum += r.Rating
			reviewed++
		}
	}
	snap.mean = 4.5
	if reviewed > 0 {
		snap.mean = sum / float64(reviewed)
	}
	c.snap.Store(snap)
	if c.gen.Load() != gen {
		// A status was patched into the previous snapshot while this one
		// was read; it may predate that change.
		c.stale.Store(true)
	}
	return snap, nil
}

var browseStatuses = map[string]bool{"online": true, "busy": true, "away": true, "offline": true}

// fold lower-cases for case-insensitive matching.
func fold(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// Browse returns a numbered page of the directory.
func (c *AdvisorCatalog) Browse(ctx context.Context, q BrowseQuery) (*BrowsePage, error) {
	fields := map[string]string{}
	if q.Page == 0 {
		q.Page = 1
	}
	if q.PageSize == 0 {
		q.PageSize = browseDefaultSize
	}
	if q.Page < 1 || q.Page > 10000 {
		fields["page"] = "Use a page number of 1 or more."
	}
	if q.PageSize < 1 || q.PageSize > browseMaxSize {
		fields["page_size"] = "Use a page size between 1 and 48."
	}
	q.Status = fold(q.Status)
	if q.Status != "" && !browseStatuses[q.Status] {
		fields["status"] = "Choose online, busy, away or offline."
	}
	q.Search = CleanLine(q.Search)
	if utf8.RuneCountInString(q.Search) > 60 {
		fields["q"] = "Use at most 60 characters."
	}
	q.Skill = CleanLine(q.Skill)
	if utf8.RuneCountInString(q.Skill) > 40 {
		fields["skill"] = "Use at most 40 characters."
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	snap, err := c.current(ctx)
	if err != nil {
		return nil, err
	}
	search, skill := fold(q.Search), fold(q.Skill)
	matched := make([]AdvisorCard, 0, len(snap.cards))
	for _, card := range snap.cards {
		if q.Status != "" && card.Status != q.Status {
			continue
		}
		if search != "" && !strings.Contains(fold(card.Name), search) && !strings.Contains(fold(card.Headline), search) {
			continue
		}
		if skill != "" && !hasSkill(card.Skills, skill) {
			continue
		}
		matched = append(matched, card)
	}
	page := &BrowsePage{Page: q.Page, PageSize: q.PageSize, Total: len(matched), Items: []AdvisorCard{}}
	page.TotalPages = (page.Total + q.PageSize - 1) / q.PageSize
	if start := (q.Page - 1) * q.PageSize; start < len(matched) {
		page.Items = matched[start:min(start+q.PageSize, len(matched))]
	}
	return page, nil
}

func hasSkill(skills []string, want string) bool {
	for _, s := range skills {
		if fold(s) == want {
			return true
		}
	}
	return false
}

// statusTier orders advisors by how soon a client can talk to them.
func statusTier(status string) int {
	switch status {
	case string(domain.PresenceOnline):
		return 0
	case string(domain.PresenceBusy):
		return 1
	case string(domain.PresenceAway):
		return 2
	}
	return 3
}

// weightedRating blends an advisor's rating with the platform mean.
func weightedRating(rating float64, reviews int, mean float64) float64 {
	v := float64(reviews)
	return (v*rating + ratingPrior*mean) / (v + ratingPrior)
}

// ranked returns the snapshot's advisors in featured order: available now
// first, then the best rated by many reviews, then the most recently online.
func (snap *catalogSnap) ranked() []int {
	snap.rank.once.Do(func() {
		idx := make([]int, len(snap.cards))
		for i := range idx {
			idx[i] = i
		}
		sort.SliceStable(idx, func(a, b int) bool {
			x, y := snap.cards[idx[a]], snap.cards[idx[b]]
			if tx, ty := statusTier(x.Status), statusTier(y.Status); tx != ty {
				return tx < ty
			}
			if wx, wy := weightedRating(x.Rating, x.ReviewsCount, snap.mean), weightedRating(y.Rating, y.ReviewsCount, snap.mean); wx != wy {
				return wx > wy
			}
			if x.ReviewsCount != y.ReviewsCount {
				return x.ReviewsCount > y.ReviewsCount
			}
			lx, ly := snap.lastOnline[idx[a]], snap.lastOnline[idx[b]]
			switch {
			case lx != nil && ly == nil:
				return true
			case lx == nil && ly != nil:
				return false
			case lx != nil && ly != nil && !lx.Equal(*ly):
				return lx.After(*ly)
			}
			return x.Name < y.Name
		})
		snap.rank.idx = idx
	})
	return snap.rank.idx
}

// Featured returns the home page selection: advisors available now first,
// then the best rated by many reviews, then the most recently online.
func (c *AdvisorCatalog) Featured(ctx context.Context, limit int) (*FeaturedList, error) {
	if limit == 0 {
		limit = featuredDefault
	}
	if limit < 1 || limit > featuredMax {
		return nil, apperror.Validation(map[string]string{"limit": "Use a number between 1 and 6."})
	}
	snap, err := c.current(ctx)
	if err != nil {
		return nil, err
	}
	idx := snap.ranked()
	out := &FeaturedList{Items: make([]FeaturedAdvisor, 0, min(limit, len(idx))), GeneratedAt: snap.builtAt.UTC().Truncate(time.Second)}
	for _, i := range idx[:min(limit, len(idx))] {
		card := snap.cards[i]
		if len(card.Skills) > featuredSkillCount {
			card.Skills = card.Skills[:featuredSkillCount]
		}
		cta := "waitlist"
		if card.Status == string(domain.PresenceOnline) {
			cta = "chat"
		}
		out.Items = append(out.Items, FeaturedAdvisor{AdvisorCard: card, CTA: cta})
	}
	return out, nil
}
