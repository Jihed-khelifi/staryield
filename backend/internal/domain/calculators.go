package domain

import (
	"time"

	"github.com/google/uuid"
)

// Calculator kinds measured by the visitor roll-up (revisions 3.0, §17).
const (
	CalcNatalChart  = "natal-chart"
	CalcPathOfLife  = "path-of-life"
	CalcAstrology   = "astrology"
	CalculatorsPath = "calculators"
)

// CalculatorKinds lists the three calculators in display order.
var CalculatorKinds = []string{CalcNatalChart, CalcPathOfLife, CalcAstrology}

// CalculatorLocales lists the locales of the site in display order.
var CalculatorLocales = []string{"en", "fr", "es"}

// ValidCalculatorKind reports whether k is one of the three calculators.
func ValidCalculatorKind(k string) bool {
	for _, v := range CalculatorKinds {
		if v == k {
			return true
		}
	}
	return false
}

// ValidCalculatorLocale reports whether l is a site locale.
func ValidCalculatorLocale(l string) bool {
	for _, v := range CalculatorLocales {
		if v == l {
			return true
		}
	}
	return false
}

// VisitorEventType says what a visitor event contributes to the roll-up.
type VisitorEventType string

// Visitor event types.
const (
	VisitView     VisitorEventType = "view"     // calculator page view
	VisitStart    VisitorEventType = "start"    // calculator_started
	VisitComplete VisitorEventType = "complete" // calculator_completed
	VisitLanding  VisitorEventType = "landing"  // campaign_landing (first touch)
	VisitLink     VisitorEventType = "link"     // authenticated event: identify the visitor
)

// VisitorEvent is one validated browser event fed to the roll-up.
type VisitorEvent struct {
	Type        VisitorEventType
	AnonymousID string
	At          time.Time
	Locale      string // en | fr | es
	Kind        string // calculator kind (view, start, complete)
	SignedIn    bool
	UserID      *uuid.UUID
	// Landing data (VisitLanding only). UTM values are already cleaned.
	UTMSource, UTMMedium, UTMCampaign, UTMContent, UTMTerm string
	ClickID                                                string // raw, validated id; resolved when the batch is stored
	// ReferrerHost is the host of the referrer, never a path.
	ReferrerHost string
}

// VisitorLanding is the first-touch data of a visitor.
type VisitorLanding struct {
	At                                                     time.Time
	UTMSource, UTMMedium, UTMCampaign, UTMContent, UTMTerm string
	ClickID                                                string
	// Set by the store when the click id or utm_campaign resolves.
	ResolvedClick *uuid.UUID
	LinkID        *uuid.UUID
	CampaignID    *uuid.UUID
}

// Empty reports whether the landing carries no attribution at all.
func (l VisitorLanding) Empty() bool {
	return l.UTMSource == "" && l.UTMMedium == "" && l.UTMCampaign == "" && l.UTMContent == "" && l.UTMTerm == "" && l.ClickID == ""
}

// VisitorUpsert is the coalesced state of one visitor within a batch.
type VisitorUpsert struct {
	AnonymousID  string
	FirstSeen    time.Time
	LastSeen     time.Time
	FirstLocale  string
	Landing      *VisitorLanding
	ReferrerHost string
	UserID       *uuid.UUID
	LinkedAt     time.Time
	// Touched is false for visitors only identified by an authenticated
	// event: they get linked when they already exist, but never created.
	Touched bool
}

// VisitKey identifies one calculator_visits row.
type VisitKey struct {
	Day         time.Time // UTC date
	AnonymousID string
	Kind        string
	Locale      string
}

// VisitCounts are the additive counters of one calculator_visits row.
type VisitCounts struct {
	Views, Started, Completed int
	SignedIn                  bool
}

// VisitorBatch is a coalesced set of visitor events: every key appears once,
// so a multi-row INSERT … ON CONFLICT never touches a row twice.
type VisitorBatch struct {
	Visitors map[string]*VisitorUpsert
	Visits   map[VisitKey]*VisitCounts
}

// NewVisitorBatch returns an empty batch.
func NewVisitorBatch() *VisitorBatch {
	return &VisitorBatch{Visitors: map[string]*VisitorUpsert{}, Visits: map[VisitKey]*VisitCounts{}}
}

// Empty reports whether the batch holds nothing to store.
func (b *VisitorBatch) Empty() bool { return len(b.Visitors) == 0 && len(b.Visits) == 0 }

// Add coalesces one event into the batch.
func (b *VisitorBatch) Add(e VisitorEvent) {
	at := e.At.UTC()
	v := b.Visitors[e.AnonymousID]
	if v == nil {
		v = &VisitorUpsert{AnonymousID: e.AnonymousID, FirstSeen: at, LastSeen: at, FirstLocale: e.Locale}
		b.Visitors[e.AnonymousID] = v
	}
	if e.Type != VisitLink {
		if !v.Touched || at.Before(v.FirstSeen) {
			v.FirstSeen, v.FirstLocale = at, e.Locale
		}
		if !v.Touched || at.After(v.LastSeen) {
			v.LastSeen = at
		}
		v.Touched = true
	}
	if v.ReferrerHost == "" {
		v.ReferrerHost = e.ReferrerHost
	}
	if e.UserID != nil && v.UserID == nil {
		id := *e.UserID
		v.UserID, v.LinkedAt = &id, at
	}
	switch e.Type {
	case VisitLanding:
		l := VisitorLanding{At: at, UTMSource: e.UTMSource, UTMMedium: e.UTMMedium, UTMCampaign: e.UTMCampaign,
			UTMContent: e.UTMContent, UTMTerm: e.UTMTerm, ClickID: e.ClickID}
		if !l.Empty() && (v.Landing == nil || at.Before(v.Landing.At)) {
			v.Landing = &l
		}
	case VisitView, VisitStart, VisitComplete:
		k := VisitKey{Day: time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC), AnonymousID: e.AnonymousID, Kind: e.Kind, Locale: e.Locale}
		c := b.Visits[k]
		if c == nil {
			c = &VisitCounts{}
			b.Visits[k] = c
		}
		switch e.Type {
		case VisitView:
			c.Views++
		case VisitStart:
			c.Started++
		case VisitComplete:
			c.Completed++
		}
		c.SignedIn = c.SignedIn || e.SignedIn
	}
}

// ClickResolution is a short-link click resolved to its link and campaign.
type ClickResolution struct {
	ClickID                                                uuid.UUID
	LinkID                                                 uuid.UUID
	CampaignID                                             *uuid.UUID
	UTMSource, UTMMedium, UTMCampaign, UTMContent, UTMTerm string
}

// ResolveLanding fills a landing's link, campaign and missing UTM values from
// the short-link click it carries (resolved by the store) or, failing that,
// from the campaign whose slug equals utm_campaign. Unknown clicks and slugs
// are ignored: the visitor keeps only the (already cleaned) UTM values.
func ResolveLanding(l *VisitorLanding, click *ClickResolution, campaignBySlug map[string]uuid.UUID) {
	if click != nil {
		id := click.ClickID
		l.ResolvedClick, l.LinkID, l.CampaignID = &id, &click.LinkID, click.CampaignID
		if l.UTMSource == "" {
			l.UTMSource = click.UTMSource
		}
		if l.UTMMedium == "" {
			l.UTMMedium = click.UTMMedium
		}
		if l.UTMCampaign == "" {
			l.UTMCampaign = click.UTMCampaign
		}
		if l.UTMContent == "" {
			l.UTMContent = click.UTMContent
		}
		if l.UTMTerm == "" {
			l.UTMTerm = click.UTMTerm
		}
	}
	if l.CampaignID == nil && l.UTMCampaign != "" {
		if id, ok := campaignBySlug[toLower(l.UTMCampaign)]; ok {
			l.CampaignID = &id
		}
	}
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
