package service

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
)

// revisions 5.0: revenue analytics (§24.5).

// RevenueMaxDays is the longest reporting window.
const RevenueMaxDays = 366

// RevenueSettings configure the revenue report.
type RevenueSettings struct {
	Currency string
	// PayoutCentsPerCredit is the value of one advisor-earned credit
	// (STARYIELD_BILLING_PAYOUT_CENTS_PER_CREDIT).
	PayoutCentsPerCredit int64
}

// RevenueService builds the admin revenue report.
type RevenueService struct {
	repo    repository.RevenueRepository
	pricing *PricingService
	clock   clock.Clock
	cfg     RevenueSettings
}

// NewRevenueService wires the revenue report.
func NewRevenueService(repo repository.RevenueRepository, pricing *PricingService, clk clock.Clock, cfg RevenueSettings) *RevenueService {
	if cfg.Currency == "" {
		cfg.Currency = "USD"
	}
	return &RevenueService{repo: repo, pricing: pricing, clock: clk, cfg: cfg}
}

// RevenueTotals are the headline figures of the period (cents unless noted).
type RevenueTotals struct {
	GrossCents                int64 `json:"gross_cents"`
	RefundsCents              int64 `json:"refunds_cents"`
	NetCents                  int64 `json:"net_cents"`
	Purchases                 int64 `json:"purchases"`
	PayingClients             int64 `json:"paying_clients"`
	AvgOrderCents             int64 `json:"avg_order_cents"`
	RecognizedCents           int64 `json:"recognized_cents"`
	DeferredCents             int64 `json:"deferred_cents"`
	AdvisorCostCents          int64 `json:"advisor_cost_cents"`
	ProcessingEstimateCents   int64 `json:"processing_estimate_cents"`
	ContributionEstimateCents int64 `json:"contribution_estimate_cents"`
	RefundRateBps             int64 `json:"refund_rate_bps"`
	Disputes                  int64 `json:"disputes"`
}

// RevenuePoint is one bucket of the series.
type RevenuePoint struct {
	Bucket          string `json:"bucket"`
	GrossCents      int64  `json:"gross_cents"`
	RefundsCents    int64  `json:"refunds_cents"`
	NetCents        int64  `json:"net_cents"`
	RecognizedCents int64  `json:"recognized_cents"`
}

// RevenuePackMix is one pack's share of the period.
type RevenuePackMix struct {
	PackID       uuid.UUID `json:"pack_id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	Purchases    int64     `json:"purchases"`
	GrossCents   int64     `json:"gross_cents"`
	RefundsCents int64     `json:"refunds_cents"`
	ShareBps     int64     `json:"share_bps"`
}

// RevenueReport is the §24.5 report.
type RevenueReport struct {
	From     time.Time        `json:"from"`
	To       time.Time        `json:"to"`
	Interval string           `json:"interval"`
	Currency string           `json:"currency"`
	Totals   RevenueTotals    `json:"totals"`
	Series   []RevenuePoint   `json:"series"`
	Packs    []RevenuePackMix `json:"packs"`
	Welcome  struct {
		Redemptions     int64 `json:"redemptions"`
		UnverifiedCard  int64 `json:"unverified_card"`
		RepeatBuyers30d int64 `json:"repeat_buyers_30d"`
		RepeatRateBps   int64 `json:"repeat_rate_bps"`
	} `json:"welcome_offer"`
	FreeTrial struct {
		SignupsWithTrial int64 `json:"signups_with_trial"`
		Purchased        int64 `json:"purchased"`
		ConversionBps    int64 `json:"conversion_bps"`
	} `json:"free_trial"`
	RefundRequests struct {
		Open               int64    `json:"open"`
		Approved           int64    `json:"approved"`
		Denied             int64    `json:"denied"`
		Credited           int64    `json:"credited"`
		Withdrawn          int64    `json:"withdrawn"`
		MedianHoursResolve *float64 `json:"median_hours_to_resolve"`
	} `json:"refund_requests"`
	Assumptions struct {
		ProcessingFeeBps     int64 `json:"processing_fee_bps"`
		ProcessingFixedCents int64 `json:"processing_fixed_cents"`
	} `json:"assumptions"`
}

// ParseRevenueRange validates the window and the interval of a report
// request: at most 366 days, interval day (default), week or month.
func ParseRevenueRange(fromRaw, toRaw, interval string, now time.Time) (Range, string, error) {
	r, err := ParseRange(fromRaw, toRaw, now)
	if err != nil {
		return r, "", err
	}
	if r.To.Sub(r.From) > RevenueMaxDays*24*time.Hour {
		return r, "", apperror.Validation(map[string]string{"from": "The revenue report is limited to 366 days."})
	}
	if interval == "" {
		interval = "day"
	}
	if _, ok := repository.RevenueUnits[interval]; !ok {
		return r, "", apperror.Validation(map[string]string{"interval": "Use day, week or month."})
	}
	return r, interval, nil
}

func bps(part, whole int64) int64 {
	if whole <= 0 || part <= 0 {
		return 0
	}
	// part ≤ 1e12 and whole ≤ 1e12 in practice; widen anyway to stay exact.
	return mulDivInt(part, 10000, whole)
}

func mulDivInt(a, b, c int64) int64 {
	if c == 0 {
		return 0
	}
	// a×b fits in int64 for the magnitudes of a report (cents ≤ 1e13).
	return a * b / c
}

// revenueBucketStart truncates t to the start of its interval bucket (UTC; weeks start on Monday, like date_trunc).
func revenueBucketStart(t time.Time, interval string) time.Time {
	t = t.UTC()
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	switch interval {
	case "week":
		return d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	return d
}

func revenueNextBucket(t time.Time, interval string) time.Time {
	switch interval {
	case "week":
		return t.AddDate(0, 0, 7)
	case "month":
		return t.AddDate(0, 1, 0)
	}
	return t.AddDate(0, 0, 1)
}

// Report computes the revenue report. Recognized revenue counts the value of
// purchased credits consumed by chats and by custom requests when released;
// deferred revenue is what is still unconsumed at the end of the window (the
// live column when the window ends now or later; for a past end, the lots'
// value now plus what was consumed or clawed back since, minus what was
// restored since). The processing figure and the contribution are estimates
// from the admin assumptions, never profit.
func (s *RevenueService) Report(ctx context.Context, r Range, interval string) (*RevenueReport, error) {
	settings, err := s.pricing.Settings(ctx)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	p := repository.RevenuePeriod{From: r.From, To: r.To, Currency: s.cfg.Currency, Unit: interval}
	live := !r.To.Before(now.Add(-time.Minute))
	var (
		sales      repository.RevenueSales
		gross      []repository.RevenueBucket
		refunds    []repository.RevenueBucket
		recognized []repository.RevenueBucket
		deferred   int64
		advisor    int64
		packs      []repository.RevenuePackRow
		packRef    []repository.RevenuePackRefund
		offer      repository.RevenueOffer
		trial      repository.RevenueTrial
		requests   repository.RevenueRefundRequests
		disputes   int64
	)
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(6)
	g.Go(func() (e error) {
		sales, e = s.repo.Sales(gctx, p, int64(settings.ProcessingFeeBps), int64(settings.ProcessingFixedCents))
		return
	})
	g.Go(func() (e error) { gross, e = s.repo.GrossSeries(gctx, p); return })
	g.Go(func() (e error) { refunds, e = s.repo.RefundSeries(gctx, p); return })
	g.Go(func() (e error) { recognized, e = s.repo.RecognizedSeries(gctx, p); return })
	g.Go(func() (e error) { deferred, e = s.repo.DeferredMillicents(gctx, r.To, live); return })
	g.Go(func() (e error) { advisor, e = s.repo.AdvisorNetCredits(gctx, p); return })
	g.Go(func() (e error) { packs, e = s.repo.Packs(gctx, p); return })
	g.Go(func() (e error) { packRef, e = s.repo.PackRefunds(gctx, p); return })
	g.Go(func() (e error) { offer, e = s.repo.WelcomeOffer(gctx, p); return })
	g.Go(func() (e error) { trial, e = s.repo.FreeTrial(gctx, p); return })
	g.Go(func() (e error) { requests, e = s.repo.RefundRequests(gctx, p); return })
	g.Go(func() (e error) { disputes, e = s.repo.Disputes(gctx, p); return })
	if err := g.Wait(); err != nil {
		return nil, apperror.Internal(err)
	}

	out := &RevenueReport{From: r.From, To: r.To, Interval: interval, Currency: s.cfg.Currency}
	t := &out.Totals
	t.GrossCents, t.Purchases, t.PayingClients = sales.GrossCents, sales.Purchases, sales.PayingClients
	if sales.Purchases > 0 {
		t.AvgOrderCents = sales.GrossCents / sales.Purchases
	}
	var recognizedMilli int64
	for _, b := range refunds {
		t.RefundsCents += b.Amount
	}
	for _, b := range recognized {
		recognizedMilli += b.Amount
	}
	t.NetCents = t.GrossCents - t.RefundsCents
	t.RecognizedCents = recognizedMilli / 1000
	t.DeferredCents = deferred / 1000
	t.AdvisorCostCents = advisor * s.cfg.PayoutCentsPerCredit
	t.ProcessingEstimateCents = sales.ProcessingCents
	t.ContributionEstimateCents = t.NetCents - t.AdvisorCostCents - t.ProcessingEstimateCents
	t.RefundRateBps = bps(t.RefundsCents, t.GrossCents)
	t.Disputes = disputes

	// Series: every bucket of the window, empty ones included.
	type cell struct{ gross, refunds, recognizedMilli int64 }
	cells := map[time.Time]*cell{}
	at := func(b time.Time) *cell {
		k := revenueBucketStart(b, interval)
		if cells[k] == nil {
			cells[k] = &cell{}
		}
		return cells[k]
	}
	for _, b := range gross {
		at(b.Bucket).gross += b.Amount
	}
	for _, b := range refunds {
		at(b.Bucket).refunds += b.Amount
	}
	for _, b := range recognized {
		at(b.Bucket).recognizedMilli += b.Amount
	}
	out.Series = []RevenuePoint{}
	for b := revenueBucketStart(r.From, interval); b.Before(r.To); b = revenueNextBucket(b, interval) {
		c := cells[b]
		if c == nil {
			c = &cell{}
		}
		out.Series = append(out.Series, RevenuePoint{Bucket: b.Format(time.RFC3339), GrossCents: c.gross, RefundsCents: c.refunds,
			NetCents: c.gross - c.refunds, RecognizedCents: c.recognizedMilli / 1000})
	}

	// Pack mix: packs sold in the window, plus packs that only saw refunds.
	byPack := map[uuid.UUID]*RevenuePackMix{}
	for _, row := range packs {
		byPack[row.PackID] = &RevenuePackMix{PackID: row.PackID, Code: row.Code, Name: row.Name, Purchases: row.Purchases, GrossCents: row.GrossCents,
			ShareBps: bps(row.GrossCents, sales.GrossCents)}
	}
	for _, row := range packRef {
		m := byPack[row.PackID]
		if m == nil {
			m = &RevenuePackMix{PackID: row.PackID, Code: row.Code, Name: row.Name}
			byPack[row.PackID] = m
		}
		m.RefundsCents = row.Cents
	}
	out.Packs = make([]RevenuePackMix, 0, len(byPack))
	for _, m := range byPack {
		out.Packs = append(out.Packs, *m)
	}
	sort.Slice(out.Packs, func(i, j int) bool {
		a, b := out.Packs[i], out.Packs[j]
		if a.GrossCents != b.GrossCents {
			return a.GrossCents > b.GrossCents
		}
		return a.Code < b.Code
	})

	out.Welcome.Redemptions, out.Welcome.UnverifiedCard, out.Welcome.RepeatBuyers30d = offer.Redemptions, offer.UnverifiedCard, offer.RepeatBuyers
	out.Welcome.RepeatRateBps = bps(offer.RepeatBuyers, offer.Redemptions)
	out.FreeTrial.SignupsWithTrial, out.FreeTrial.Purchased = trial.Signups, trial.Purchased
	out.FreeTrial.ConversionBps = bps(trial.Purchased, trial.Signups)
	rq := &out.RefundRequests
	rq.Open, rq.Approved, rq.Denied, rq.Credited, rq.Withdrawn = requests.Open, requests.Approved, requests.Denied, requests.Credited, requests.Withdrawn
	rq.MedianHoursResolve = requests.MedianHours
	out.Assumptions.ProcessingFeeBps = int64(settings.ProcessingFeeBps)
	out.Assumptions.ProcessingFixedCents = int64(settings.ProcessingFixedCents)
	return out, nil
}
