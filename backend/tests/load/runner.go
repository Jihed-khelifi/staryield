//go:build load

// Package load drives a running Staryield API at a constant arrival rate and
// checks latency and error-rate objectives, and verifies that hot queries use
// indexes on a large dataset. It is excluded from normal test runs:
//
//	go run ./cmd/seed -profile=large            # large dataset (see README)
//	STARYIELD_RATE_LIMIT_SCALE=1000 go run ./cmd/api
//	go test -tags load ./tests/load/ -run TestLoad -v
//	go test -tags load ./tests/load/ -run TestExplain -v
package load

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Request performs one call and returns the HTTP status (0 on transport
// failure) and an error for anything the scenario considers a failure.
type Request func(ctx context.Context, c *Client, i int64) (int, error)

// SLO is the objective a scenario must meet.
type SLO struct {
	P95        time.Duration
	P99        time.Duration
	MaxErrRate float64 // e.g. 0.001 for 0.1 %
}

// Scenario is an open-model load: Rate arrivals per second for Duration,
// regardless of how fast the server answers. MaxInFlight bounds concurrency;
// arrivals beyond it are counted as dropped (a sign of saturation).
type Scenario struct {
	Name        string
	Rate        int
	Duration    time.Duration
	MaxInFlight int
	Do          Request
	SLO         SLO
}

// Result summarises a scenario run.
type Result struct {
	Name         string        `json:"name"`
	Rate         int           `json:"rate"`
	Duration     time.Duration `json:"duration"`
	Sent         int64         `json:"sent"`
	OK           int64         `json:"ok"`
	Errors       int64         `json:"errors"`
	Dropped      int64         `json:"dropped"`
	Status       map[int]int64 `json:"status"`
	P50          time.Duration `json:"p50"`
	P90          time.Duration `json:"p90"`
	P95          time.Duration `json:"p95"`
	P99          time.Duration `json:"p99"`
	Max          time.Duration `json:"max"`
	Throughput   float64       `json:"throughput_rps"`
	ErrorSamples []string      `json:"error_samples,omitempty"`
	SLO          SLO           `json:"slo"`
	Violations   []string      `json:"violations,omitempty"`
}

// ErrRate is the share of failed or dropped arrivals.
func (r Result) ErrRate() float64 {
	if r.Sent == 0 {
		// Nothing measured: failures during setup still fail the run.
		if r.Errors+r.Dropped > 0 {
			return 1
		}
		return 0
	}
	return float64(r.Errors+r.Dropped) / float64(r.Sent)
}

// Check compares the result with its SLO and records violations.
func (r *Result) Check() {
	r.Violations = nil
	if r.Sent == 0 {
		r.Violations = append(r.Violations, "no requests were measured")
	}
	if r.SLO.P95 > 0 && r.P95 > r.SLO.P95 {
		r.Violations = append(r.Violations, fmt.Sprintf("p95 %s > %s", r.P95, r.SLO.P95))
	}
	if r.SLO.P99 > 0 && r.P99 > r.SLO.P99 {
		r.Violations = append(r.Violations, fmt.Sprintf("p99 %s > %s", r.P99, r.SLO.P99))
	}
	if er := r.ErrRate(); er > r.SLO.MaxErrRate {
		r.Violations = append(r.Violations, fmt.Sprintf("error rate %.3f%% > %.3f%%", er*100, r.SLO.MaxErrRate*100))
	}
}

// String renders a one-line summary.
func (r Result) String() string {
	return fmt.Sprintf("%-28s sent=%-6d ok=%-6d err=%-4d drop=%-4d p50=%-8s p95=%-8s p99=%-8s max=%-8s rps=%.1f",
		r.Name, r.Sent, r.OK, r.Errors, r.Dropped, round(r.P50), round(r.P95), round(r.P99), round(r.Max), r.Throughput)
}

func round(d time.Duration) time.Duration { return d.Round(100 * time.Microsecond) }

// Run executes a scenario against a pool of clients (round-robin).
func Run(ctx context.Context, s Scenario, clients []*Client) Result {
	if s.MaxInFlight <= 0 {
		s.MaxInFlight = 512
	}
	res := Result{Name: s.Name, Rate: s.Rate, Duration: s.Duration, Status: map[int]int64{}, SLO: s.SLO}
	if s.Rate <= 0 || s.Duration <= 0 || len(clients) == 0 {
		return res
	}
	var (
		mu        sync.Mutex
		latencies = make([]time.Duration, 0, s.Rate*int(s.Duration/time.Second+1))
		samples   []string
		inFlight  = make(chan struct{}, s.MaxInFlight)
		wg        sync.WaitGroup
		sent, ok  atomic.Int64
		errs, drp atomic.Int64
	)
	interval := time.Second / time.Duration(s.Rate)
	start := time.Now()
	deadline := start.Add(s.Duration)
	runCtx, cancel := context.WithTimeout(ctx, s.Duration+30*time.Second)
	defer cancel()

	next := start
	for i := int64(0); ; i++ {
		if next.After(deadline) || runCtx.Err() != nil {
			break
		}
		if d := time.Until(next); d > 0 {
			time.Sleep(d)
		}
		next = next.Add(interval)
		sent.Add(1)
		select {
		case inFlight <- struct{}{}:
		default:
			drp.Add(1)
			continue
		}
		wg.Add(1)
		go func(i int64) {
			defer wg.Done()
			defer func() { <-inFlight }()
			c := clients[int(i)%len(clients)]
			t0 := time.Now()
			status, err := s.Do(runCtx, c, i)
			lat := time.Since(t0)
			mu.Lock()
			latencies = append(latencies, lat)
			res.Status[status]++
			if err != nil && len(samples) < 10 {
				samples = append(samples, err.Error())
			}
			mu.Unlock()
			if err != nil {
				errs.Add(1)
			} else {
				ok.Add(1)
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	res.Sent, res.OK, res.Errors, res.Dropped = sent.Load(), ok.Load(), errs.Load(), drp.Load()
	res.ErrorSamples = samples
	if elapsed > 0 {
		res.Throughput = float64(res.OK) / elapsed.Seconds()
	}
	if len(latencies) > 0 {
		sort.Slice(latencies, func(a, b int) bool { return latencies[a] < latencies[b] })
		res.P50 = percentile(latencies, 0.50)
		res.P90 = percentile(latencies, 0.90)
		res.P95 = percentile(latencies, 0.95)
		res.P99 = percentile(latencies, 0.99)
		res.Max = latencies[len(latencies)-1]
	}
	res.Check()
	return res
}

// percentile uses the nearest-rank method on sorted values.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(p*float64(len(sorted))+0.999999) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

// Summary renders results as an aligned table.
func Summary(results []Result) string {
	var b strings.Builder
	for _, r := range results {
		b.WriteString(r.String())
		if len(r.Violations) > 0 {
			b.WriteString("  ✗ " + strings.Join(r.Violations, "; "))
		} else {
			b.WriteString("  ✓")
		}
		b.WriteString("\n")
	}
	return b.String()
}
